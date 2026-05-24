# Gold-Standard Split-Packages Codegen — Per-Object Adapters + Pure Data Tables

> **For agentic workers:** This is a fork-side (`codelite7/gqlgen`) codegen plan. Execution happens in the fork; the maintainer pushes (fork pushes are blocked in the gemini devenv). Steps use checkbox (`- [ ]`) syntax. REQUIRED SUB-SKILL when executing: `superpowers:subagent-driven-development` (recommended) or `superpowers:executing-plans`.

**Goal:** Collapse the per-field `FieldDef.Resolve` closures in split-packages output into `O(objects)` per-object adapter functions + pure-data field rows, minimizing generated-Go **AST/function count** to cut codegen time and compile wall-time / peak RSS.

**Architecture:** Keep the pinned split-packages data-driven dispatch. Replace the per-field resolver **closure** (one of ~4 `splitFieldAccess` shapes emitted per field today) with **(a)** one generated per-object adapter `__resolveField_<T>(ctx, ec, fieldIdx, obj, …)` — a single `switch fieldIdx` that does the typed access — and **(b)** a `FieldDef` reduced to **pure data** (no closure): an adapter reference + field index, plus codec id, nullability bit, directive pointer, args key. Field dispatch resolves name→index once per object (sorted array / perfect hash). Explicit generated wiring (`SchemaDesc` aggregation) replaces scattered `init()` registration. The per-object adapter is the **irreducible typed-Go floor**; everything else is data + the existing shared `ResolveField[any]` executor.

**Tech Stack:** Go (`codelite7/gqlgen` fork); `text/template` codegen (`codegen/split_*.gotpl`); `graphql/executor/shardruntime` runtime; pinned baseline `d5f762356cd5` (PR #12 merge, `feat/split-packages-data-driven-dispatch`).

**Decided end-state (the target this plan terminates at):** *"N closures collapsed to kind+data"* = `O(objects)` typed adapter functions + N pure-data field rows; **zero per-field closures**.

---

## Why this is the floor (decided; do not re-litigate)

Settled by 3-model consensus (gpt-5.2 / gpt-5.1-codex / gpt-5.2-pro, all 8/10) + source verification:

1. **Generics can't help the bulk.** Per-field variation (codec, accessor, nullability, directives) is *value*-level; Go type parameters express *type*-level variation only. `ResolveField[any]` is correct; per-concrete-type instantiation is counterproductive (GCShape stenciling → same body + extra dictionaries). Confirmed: `graphql/resolve_field.go` `ResolveField[T any]`, only ever `[any]`.
2. **The compile floor = emit fewer Go functions/symbols, not "more cleverly typed" Go.** Data is cheaper for `cmd/compile` than code. The remaining bloat after the pin's `__splitField_*` deletion is the **per-field closures** still living on `FieldDef.Resolve`.
3. **The only mandatory typed Go is per-object access.** You cannot do `obj.(*User).Name` without *some* typed code; the minimum is **one adapter per object type** (a switch), not one closure per field. Reflection/unsafe-offset rejected (runtime cost + loss of static type safety; runtime parity is required here).
4. **Split-packages stays** (it hosts the data-driven dispatch and bounds per-package compile memory while output is still large). Shard *count* is a separate knob to dial down later — out of scope here.

---

## Baseline — current pinned state (the "from")

Per field today, `codegen/split_register_.gotpl` emits a `shardruntime.RegisterFieldDef(... FieldDef{ Resolve: func(ctx, ec, obj any)(any,error){ <splitFieldAccess body> }, MarshalCodec, NonNull, Directives, IsMethod, IsResolver, ArgsKey, ReturnType })`. The `Resolve` body is one of ~4 distinct shapes from `codegen/split_fields_.gotpl` `define "splitFieldAccess"`:

- **`.IsResolver`** → `return ec.InvokeResolver(ctx, "<T>", "<F>", obj)` (1 line)
- **`.IsMethod`** → `obj.(*T).Method(args)` with haser / v-ok / no-err / err variants (1–6 lines; args via `fc := graphql.GetFieldContext(ctx)`)
- **`.IsVariable`** → `return obj.(*T).GoField, nil` (+ optional haser) (1–4 lines)
- **`.IsMap`** → `switch v := obj.(T)["<F>"].(type) { … }` (9 lines)

`FieldDef` (`graphql/executor/shardruntime/runtime.go`): `{ Resolve func(ctx, ec, obj any)(any,error); Directives func(ctx, next)Resolver; MarshalCodec string; NonNull, PanicHandled, IsMethod, IsResolver bool; ArgsKey string; ReturnType *ObjectChildLookup }`. The single remaining per-field **closure** is `Resolve`. At ~10k fields that is ~10k distinct closure ASTs the compiler type-checks/SSA-compiles.

Dispatch: `shardruntime.resolveFromDef` looks the `FieldDef` up (per `(object, field)`) and calls `Resolve`.

---

## End-state architecture (the "to")

1. **Per-object adapter** — one generated function per object type, the *only* per-schema typed Go:
   ```go
   // generated, one per object T, in the object's shard package
   func __resolveField_<T>(ctx context.Context, ec shardruntime.ObjectExecutionContext, fieldIdx uint16, obj any) (any, error) {
       o := obj.(*gen.<T>)
       switch fieldIdx {
       case 0: return o.<Field0>, nil                          // IsVariable
       case 1: return o.<Method1>(/* args from fc */)          // IsMethod
       case 2: return ec.InvokeResolver(ctx, "<T>", "<F2>", o) // IsResolver
       // ... one case per field of T (the splitFieldAccess body, verbatim) ...
       default: return nil, fmt.Errorf("unknown field index %d for <T>", fieldIdx)
       }
   }
   ```
   `O(objects)` functions (each a switch) replace `O(fields)` closures. The switch *cases* are the existing `splitFieldAccess` bodies — semantics unchanged.

2. **`FieldDef` becomes pure data** (no closure):
   ```go
   type FieldDef struct {
       Adapter      func(ctx context.Context, ec ObjectExecutionContext, fieldIdx uint16, obj any) (any, error) // points at __resolveField_<T>
       FieldIdx     uint16
       MarshalCodec string
       NonNull      bool
       PanicHandled bool
       Directives   func(ctx context.Context, next graphql.Resolver) graphql.Resolver // ptr to existing __splitDirectives_<T>_<F>, or nil
       ArgsKey      string
       ReturnType   *ObjectChildLookup
   }
   ```
   `Adapter` is **one func value per object** (shared across that object's fields), not per field. (`IsMethod`/`IsResolver` booleans fold into the adapter's switch case; they need not survive on `FieldDef`.)

3. **Field dispatch by index, once per object.** Generate a `name→index` lookup per object (sorted `[]string` + binary search, or a minimal perfect hash for large objects) instead of per-field string-keyed registry hits.

4. **Explicit wiring over `init()` registry.** Each shard exports `var ShardDesc = …`; a single generated root aggregates them into `var SchemaDesc = …` consumed by the executor. Deterministic, testable, no scattered `init()` side effects. (Consensus: `init()`-registration is idiomatic but a smell at 10k entries.)

5. **Unchanged:** the shared `ResolveField[any]` executor; directive wrappers (`__splitDirectives_*`, referenced by pointer); the typed resolver-binding boundary (user resolver interfaces stay compiled Go → static type safety preserved).

---

## Design decisions (resolved up front)

- **Per-object adapter vs per-field func-pointer vs reflection vs unsafe-offset?** Per-object adapter. Per-field func-pointers are still `O(fields)` symbols (no win). Reflection/unsafe trade runtime + type safety, which the "runtime parity required" constraint forbids. The adapter is the minimum typed Go.
- **Keep the `[any]` generic executor or drop to plain `any`?** Keep `ResolveField[any]` as-is (no behavior change; not worth churning). Note for the record: since it's only ever `[any]`, a non-generic `func ResolveField(obj any, …)` is equivalent — but that's a cosmetic follow-up, not this plan.
- **Codecs:** stay as `MarshalCodec string` (data). No change.
- **Directives:** pointer to the already-separate `__splitDirectives_<T>_<F>`; `nil` when none. No per-field wrapper closure.
- **Args:** keep `ArgsKey` (data); the adapter's method case reads args via `graphql.GetFieldContext(ctx)` exactly as the current `splitFieldAccess` does.
- **Stream fields (`__splitStreamField_*`, subscriptions):** mirror with a `__resolveStreamField_<T>` adapter + `StreamFieldDef`. Gated to the same change; ~one extra adapter shape. (If the consumer schema has no split-packages subscriptions, cover it via the test fixture only.)

---

## File Structure (fork files)

- Modify: `graphql/executor/shardruntime/runtime.go` — `FieldDef` shape (closure → adapter+idx); `resolveFromDef` to call `Adapter(ctx, ec, def.FieldIdx, obj)`; optional `ShardDesc`/`SchemaDesc` types + explicit aggregation.
- Modify: `codegen/split_fields_.gotpl` — emit the per-object `__resolveField_<T>` adapter (switch of `splitFieldAccess` cases) instead of inlining `splitFieldAccess` into a per-field closure.
- Modify: `codegen/split_register_.gotpl` — emit pure-data `FieldDef{Adapter: __resolveField_<T>, FieldIdx: N, …}`; emit `name→index` table; emit `ShardDesc`.
- Modify (root aggregation): the split-packages root template that wires shards — emit `SchemaDesc` aggregation; drop per-field `init()` `Register…` calls.
- Create (test fixture): `codegen/testserver/splitpackages_adapters/` — copy of `splitpackages` with the new path, plus `//go:generate` and a parity test harness (mirror of the existing `splitpackages_desctables` pattern from WS3).
- Modify: `codegen/config/exec.go` + `gqlgen.schema.json` — if gated behind a flag during rollout (see Task 2); otherwise this replaces the default split emission.

---

## TDD Task Breakdown

Each task = one commit. Failing test first; minimal implementation; `go test ./... -count=1` (fork) green before commit. Run fork tests from the fork root.

### Task 1 — `FieldDef` data shape + adapter-based `resolveFromDef`

**Files:** Modify `graphql/executor/shardruntime/runtime.go`; Test `graphql/executor/shardruntime/runtime_test.go`.

- [ ] **Step 1: Failing test** — construct a `FieldDef` whose `Adapter` is a hand-written switch over two `FieldIdx` values returning known typed values, with a known `MarshalCodec`; wire a minimal `ObjectExecutionContext` fake; call `resolveFromDef`; assert the returned `Marshaler` produces the expected JSON for each field index.
- [ ] **Step 2: Run** `go test ./graphql/executor/shardruntime/ -run TestResolveFromDef_Adapter -v` → FAIL (`FieldDef` has no `Adapter`/`FieldIdx`).
- [ ] **Step 3: Implement** — change `FieldDef.Resolve` → `Adapter func(ctx, ec ObjectExecutionContext, fieldIdx uint16, obj any)(any,error)` + `FieldIdx uint16`; update `resolveFromDef` to call `def.Adapter(ctx, ec, def.FieldIdx, obj)` inside the existing `ResolveField[any]` wiring.
- [ ] **Step 4: Run** the test → PASS.
- [ ] **Step 5: Commit** — `refactor(shardruntime): FieldDef stores per-object Adapter + FieldIdx (no per-field closure)`.

### Task 2 — Rollout flag `use_field_adapters` (default off)

**Files:** Modify `codegen/config/exec.go`, `gqlgen.schema.json`; Test `codegen/config/exec_test.go`.

- [ ] **Step 1: Failing test** — load a fixture `gqlgen.yml` with `exec.use_field_adapters: true`; assert `cfg.Exec.UseFieldAdapters == true`.
- [ ] **Step 2: Run** `go test ./codegen/config/ -run TestExec_UseFieldAdapters -v` → FAIL.
- [ ] **Step 3: Implement** — add `UseFieldAdapters bool \`yaml:"use_field_adapters,omitempty"\`` to `ExecConfig`; document; add to schema json. Default false so the old path stays until parity is proven.
- [ ] **Step 4: Run** → PASS.
- [ ] **Step 5: Commit** — `feat(codegen/config): add use_field_adapters flag (default off)`.

### Task 3 — Emit per-object `__resolveField_<T>` adapter

**Files:** Modify `codegen/split_fields_.gotpl`; Test via fixture in Task 6 (golden) — here, a focused template-render unit test if the fork has one, else defer assertion to Task 6.

- [ ] **Step 1: Failing test** — golden test (or Task 6 fixture build) asserting that with the flag on, `split_fields_.gotpl` emits exactly one `func __resolveField_<T>(ctx, ec, fieldIdx uint16, obj any)(any,error)` per object containing a `switch fieldIdx` whose cases are the existing `splitFieldAccess` bodies (in stable field order), and emits **no** per-field `Resolve` closure.
- [ ] **Step 2: Run** the fixture build → FAIL (still emits closures).
- [ ] **Step 3: Implement** — under `{{ if $.Config.Exec.UseFieldAdapters }}`, range objects → emit the adapter with `{{- range $i, $field := .Fields }}case {{$i}}: {{ template "splitFieldAccess" $field }}{{- end }}`. Reuse the existing `splitFieldAccess` define unchanged.
- [ ] **Step 4: Run** → PASS.
- [ ] **Step 5: Commit** — `feat(codegen): emit per-object __resolveField_<T> adapter under use_field_adapters`.

### Task 4 — Emit pure-data `FieldDef` + `name→index` table

**Files:** Modify `codegen/split_register_.gotpl`; Test via Task 6 fixture + a runtime dispatch test.

- [ ] **Step 1: Failing test** — assert generated registration emits `FieldDef{Adapter: __resolveField_<T>, FieldIdx: <i>, MarshalCodec: …, NonNull: …, Directives: …, ArgsKey: …}` with **no closure literal**, plus a per-object `name→index` lookup; and a runtime test that resolving `"<T>"."<F>"` returns the same value as the baseline path.
- [ ] **Step 2: Run** → FAIL.
- [ ] **Step 3: Implement** — under the flag, emit data-only `FieldDef` literals referencing the adapter + index; emit a sorted `[]string` name table (binary search) per object; directive field = `__splitDirectives_<T>_<F>` pointer or `nil`.
- [ ] **Step 4: Run** → PASS.
- [ ] **Step 5: Commit** — `feat(codegen): emit pure-data FieldDef + name→index dispatch under use_field_adapters`.

### Task 5 — Explicit `ShardDesc`/`SchemaDesc` wiring (replace per-field init registry)

**Files:** Modify `codegen/split_register_.gotpl` + split-packages root template; Modify `graphql/executor/shardruntime/runtime.go` (aggregation types); Test `graphql/executor/shardruntime/*_test.go`.

- [ ] **Step 1: Failing test** — assert a generated shard exposes `var ShardDesc = ShardDescriptor{…}`, the root exposes `func Schema() *SchemaDescriptor` aggregating shards, and dispatch through `Schema()` returns the same results as the registry path — with **no `init()`** doing per-field `Register…`.
- [ ] **Step 2: Run** → FAIL.
- [ ] **Step 3: Implement** — add `ShardDescriptor`/`SchemaDescriptor` types + aggregation; emit `var ShardDesc` per shard; emit a generated root that imports shards and builds `SchemaDesc`; executor consumes it. Remove per-field `init()` `RegisterFieldDef` emission under the flag.
- [ ] **Step 4: Run** → PASS.
- [ ] **Step 5: Commit** — `feat(codegen): explicit ShardDesc/SchemaDesc wiring (drop per-field init registry) under use_field_adapters`.

### Task 6 — `splitpackages_adapters` fixture + parity tests

**Files:** Create `codegen/testserver/splitpackages_adapters/` (copy of `splitpackages`, `use_field_adapters: true`, `//go:generate`); wire existing splitpackages test suite to run against it.

- [ ] **Step 1: Failing test** — `go test ./codegen/testserver/splitpackages_adapters/...` (fixture added, generator not yet run) → FAIL/no-compile.
- [ ] **Step 2: Run** `go generate ./codegen/testserver/splitpackages_adapters/...` → generates adapters + data tables.
- [ ] **Step 3: Implement** — ensure it compiles; parametrize the shared splitpackages execution suite to run identically against the adapters fixture (introspection, directives, nullability, lists, maps, methods, resolvers, errors).
- [ ] **Step 4: Run** `go test ./codegen/testserver/splitpackages_adapters/... -count=1` → PASS (behavioral parity with `splitpackages`).
- [ ] **Step 5: Commit** — `test(codegen/testserver): add splitpackages_adapters fixture + parity suite`.

### Task 7 — Stream-field adapter (subscriptions)

**Files:** Modify `codegen/split_fields_.gotpl` (+ stream register template); add a subscription to the `splitpackages_adapters` fixture if none exists.

- [ ] **Step 1: Failing test** — fixture with a split-packages subscription field, assert generated code uses `__resolveStreamField_<T>` + `StreamFieldDef`, not a per-field stream closure; runtime parity test for the subscription.
- [ ] **Step 2: Run** → FAIL.
- [ ] **Step 3: Implement** — mirror Tasks 3–4 for `ResolveFieldStream[any]`: `__resolveStreamField_<T>` adapter + `StreamFieldDef` data.
- [ ] **Step 4: Run** → PASS.
- [ ] **Step 5: Commit** — `feat(codegen): per-object stream adapter for split-packages subscriptions`.

### Task 8 — Docs + consumer measurement

**Files:** `codegen/AGENTS.md` (document `use_field_adapters`); this plan's "Measurement" section is the gate.

- [ ] **Step 1:** Document the flag (what it does, default off, relationship to data-driven dispatch).
- [ ] **Step 2:** On the **gemini consumer**: pin the fork to the branch HEAD, set `exec.use_field_adapters: true` in `service-api-go/api-graphql/gqlgen.yml`, run `task generate` then `task validate-go`. Capture: codegen wall time, cold `go build ./src/...` wall + peak RSS, and total generated LOC, vs the current pin.
- [ ] **Step 3: Commit (fork)** — `docs(codegen): document use_field_adapters flag`.

---

## Measurement & success criteria

Compare current pin vs `use_field_adapters: true`, on the gemini consumer:

| Metric | Baseline (pin) | Target |
|---|---|---|
| `task generate-go` wall (forced) | ~2:14 | ↓ (less template work per field) |
| Cold `go build ./src/...` wall | ~1:25 (warm-ish) / cold TBD | ↓ |
| Compile peak RSS | ~3–4.8 GB | ↓ |
| Generated Go LOC (largest entity) | `ent_escrow` ~56k (post data-driven) | ↓↓ (closures → switch cases) |
| GraphQL runtime throughput / latency | baseline | **parity required (±noise)** |

**Adopt** if ≥10% improvement on any of {codegen wall, cold compile wall, peak RSS} **and** runtime parity. **Retract the flag** (keep code, remove from consumer config) if <10% everywhere or any runtime regression. This is the end of the plan — the decided gold-standard structure ("N closures collapsed to kind+data") is reached at Task 6; Tasks 7–8 finish stream parity + prove the win.

## Risks / watch-items

- **Per-object adapter switch size:** very wide objects (hundreds of fields) produce a large switch; still one function (one AST), far cheaper than N closures, but confirm `cmd/compile` handles the largest object's switch well.
- **Args plumbing inside the adapter:** method-with-args cases must read `graphql.GetFieldContext(ctx)` correctly when invoked via the adapter; covered by the method/args parity cases in Task 6.
- **`ec.InvokeResolver` signature** from inside the adapter must match today's call; verify against `shardruntime`.
- **Field order stability:** `FieldIdx` must be assigned deterministically (schema field order) so adapters and data tables agree and codegen stays idempotent (keep the repo's idempotent-codegen invariant).
- **Runtime parity is a gate, not a goal:** indirection changes from closure→adapter must not regress latency; the index dispatch should be ≤ the current registry lookup.

## Non-goals

- No execution-layout change — split-packages is retained (shard-count tuning is a separate, later effort).
- No runtime performance *improvement* targeted (parity only).
- No reflection / `unsafe` field access.
- No move off Go codegen to an embedded-binary-plan interpreter (a further-floor option the panel raised; deliberately out of scope to preserve static type safety + debuggability).
