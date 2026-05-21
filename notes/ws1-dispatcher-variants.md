# WS1 Task 0 — Object Dispatcher Template Variants (Findings)

Investigation done against gqlgen fork at gemini-pinned SHA `df17c4febbc5` (= PR #7 merge, head of fork master on github; local `master` ref is stale at `3ea039bc`). WS1 implementer should branch off `df17c4febbc5`.

Source templates inspected:
- `codegen/object.gotpl` (140 lines) — single-file mode dispatcher
- `codegen/split_shard_.gotpl` (186 lines) — split-packages mode dispatcher
- `codegen/split_fields_.gotpl` (124 lines) — split-packages per-field worker functions
- `codegen/field.gotpl` (per-field bespoke functions in single-file mode — head sampled, full file ~300+ lines)

Supporting Go sources:
- `codegen/object.go:159-176` — `Object.IsConcurrent()`, `Object.InvalidsIncrement(varName)`
- `codegen/field.go:600-605` — `Field.IsConcurrent()` = `!DisableConcurrency && (MethodHasContext || IsResolver)`

Smoke-gen entrypoint (relevant to Task 4): `codegen/testdata/gqlgen.go`, invoked as
`go run ../../../testdata/gqlgen.go -config gqlgen.yml [-stub stub.go]`.
The plan's reference to `testserver/main.go generate` is incorrect — there is no such file. Existing testservers use `//go:generate go run ../../../testdata/gqlgen.go -config gqlgen.yml -stub stub.go`.

---

## Per-Field Dispatch Variants (single-file `object.gotpl`)

The dispatcher function is `_<Type>(ctx, [ec,] sel, [obj]) graphql.Marshaler`. Function signature is parametrized at the template level by `$.FuncReceiver` / `$.ECFuncParam` / `$.ECDot` / `$.ECArg` (these implement `use_function_syntax_for_execution_context: true`). Inside the function:

1. `CollectFields(ec.OperationContext, sel, <type>Implementors)` builds the field list.
2. For Root types, `ctx = graphql.WithFieldContext(ctx, &graphql.FieldContext{Object: <name>})` once.
3. `out := graphql.NewFieldSet(fields)` and `deferred := map[string]*graphql.FieldSet{}`.
4. `for i, field := range fields { ... switch field.Name { ... } }` — the dispatcher's hot loop.
5. After the loop: `out.Dispatch(ctx)`, invalids→Null check, atomic increment to `ec.Deferred`, then iterate `deferred` calling `ec.ProcessDeferredGroup(...)`. Return `out`.

The switch has FIVE distinct case shapes:

### Variant 0 — `__typename` (always emitted, before the `range` over fields)

```go
case "__typename":
    out.Values[i] = graphql.MarshalString("<TypeName>")
```

No invalids tracking. Doesn't appear in the field handler table — should be handled inline by the generic dispatcher (fast path on `field.Name == "__typename"`).

### Variant A — concurrent, non-Root (most common for resolver fields)

Conditions: `$field.IsConcurrent` is true AND `not $object.Root`.

Emits:
- `field := field` (closure capture shadow).
- `innerFunc := func(ctx, fs *graphql.FieldSet) (res graphql.Marshaler) { ... }` containing:
  - Panic recover (gated on `$.Config.OmitPanicHandler` = false).
  - `res = {{$.ECDot}}_{{$object.Name}}_{{$field.Name}}(ctx, {{$.ECArg}}field, obj)` — calls the bespoke per-field function.
  - If `$field.TypeReference.GQL.NonNull`: `if res == graphql.Null { {{ $object.InvalidsIncrement "fs" }} }`.
- Runtime deferrable check:
  - `if field.Deferrable != nil { ... add to deferred map ...; out.Values[i] = graphql.Null; continue }`
- Otherwise: `out.Concurrently(i, func(ctx) graphql.Marshaler { return innerFunc(ctx, out) })`.

### Variant B — concurrent, Root

Same as Variant A, but no Deferrable branch, and the goroutine dispatch wraps in root-resolver middleware:
- `rrm := func(ctx) graphql.Marshaler { return ec.OperationContext.RootResolverMiddleware(ctx, func(ctx) graphql.Marshaler { return innerFunc(ctx, out) }) }`
- `out.Concurrently(i, func(ctx) graphql.Marshaler { return rrm(innerCtx) })` where `innerCtx` was set per-iteration earlier in the loop.

### Variant C — non-concurrent, Root

- `out.Values[i] = ec.OperationContext.RootResolverMiddleware(innerCtx, func(ctx) (res graphql.Marshaler) { return {{$.ECDot}}_{{$object.Name}}_{{$field.Name}}(ctx, {{$.ECArg}}field) })`
- If non-null: `if out.Values[i] == graphql.Null { {{ $object.InvalidsIncrement "out" }} }`

### Variant D — non-concurrent, non-Root (the simplest)

- `out.Values[i] = {{$.ECDot}}_{{$object.Name}}_{{$field.Name}}(ctx, {{$.ECArg}}field, obj)`
- If non-null: `if out.Values[i] == graphql.Null { {{ $object.InvalidsIncrement "out" }} }`

---

## Template-Data Inputs That Drive Each Variant

Per-field (`$field` = `*codegen.Field`):

| Input | Type | Use in dispatcher |
|---|---|---|
| `$field.Name` | string | case label and call name |
| `$field.IsConcurrent` | bool (template-time) | selects Variant A/B (true) vs C/D (false). Determined by `Field.IsConcurrent()` in `codegen/field.go:600`: `!Object.DisableConcurrency && (MethodHasContext || IsResolver)`. |
| `$field.TypeReference.GQL.NonNull` | bool | gates invalids increment |
| `$field.Deferrable` | runtime — NOT template-time | `field.Deferrable` is a runtime check against the resolved `graphql.CollectedField`. Only checked in concurrent + non-Root code paths. Does NOT need to be in the per-type table; it stays inside the generic dispatcher's concurrent-non-Root branch. |

Per-object (`$object` = `*codegen.Object`):

| Input | Type | Use in dispatcher |
|---|---|---|
| `$object.Name` | string | function name and Implementors var |
| `$object.Root` | bool | RRM wrapping, innerCtx init, no-deferrable variant |
| `$object.Stream` | bool | function-level — stream mode returns `func(ctx) graphql.Marshaler` instead of `graphql.Marshaler`. Out of scope for WS1's normal dispatcher (handled separately). |
| `$object.Reference` / `$object.Implementors` | type metadata | function signature and Implementors slice |
| `$object.IsConcurrent` | bool | drives `InvalidsIncrement` — if true, emits `atomic.AddUint32(&X.Invalids, 1)`; else `X.Invalids++`. Per-object computed from whether ANY field is concurrent (`codegen/object.go:159`). |
| `$object.InvalidsIncrement(varName)` | string (Go statement) | currently substituted directly in template — needs replacing with a runtime branch if we genericize. |

Global (`$.Config`, `$.FuncReceiver` / `$.ECFuncParam` / `$.ECDot` / `$.ECArg`):

| Input | Use |
|---|---|
| `$.Config.OmitPanicHandler` | gates the `defer recover()` block in concurrent paths |
| `$.FuncReceiver` / `$.ECFuncParam` / `$.ECDot` / `$.ECArg` | implements the function-syntax-for-execution-context flag. In modern single-file, `$.FuncReceiver=""`, `$.ECFuncParam="ec ec.executionContext, "`, `$.ECDot="ec."`, `$.ECArg="ec, "`. |
| `$.Config.Exec.Layout` | top-level guard: `{{ if ne .Config.Exec.Layout "split-packages" }}` — this entire template only emits in non-split-packages mode. |

---

## Comparison with Split-Packages (`split_shard_.gotpl`)

The split-packages variant emits a near-identical loop, with three structural differences:

1. **Function signature** uses concrete `shardruntime.ObjectExecutionContext` interface: `func _<Name>(ctx context.Context, ec shardruntime.ObjectExecutionContext, sel ast.SelectionSet, obj any)`. Single-file uses concrete `*executionContext` directly via the function-syntax shim.

2. **Resolver call uses `ec.ResolveField(...)`** instead of `ec._<Object>_<field>(...)`. The shard's `init()` block at the top registers per-field `__splitField_*` handlers via `shardruntime.RegisterObject(splitScope, ...)`. At runtime `ec.ResolveField(ctx, typeName, fieldName, field, obj)` looks up the registered handler and invokes it. The handler itself (in `split_fields_.gotpl`) wraps `graphql.ResolveField[any](...)` with the directive closure, fieldContext provider, marshaler, etc.

3. **`InvalidsIncrement` is inlined** rather than emitted via the helper — `{{- if $object.IsConcurrent }} atomic.AddUint32(&out.Invalids, 1) {{- else }} out.Invalids++ {{- end }}`.

The split-packages dispatcher also handles deferred fields, panic recover, and concurrent dispatch the same way as single-file.

---

## Answers to the Three Open Questions (Plan §WS1 §Open Questions)

### Q1: Where does directive-wrapping happen?

**Answer:** Inside the per-field bespoke function, NOT in the dispatcher.

- In single-file mode, the dispatcher calls `{{$.ECDot}}_{{$object.Name}}_{{$field.Name}}(ctx, ...)`. That bespoke function (emitted by `codegen/field.gotpl`) calls `graphql.ResolveField(ctx, ec.OperationContext, field, fieldContextProvider, resolver, directiveWrapper, marshaler)`. The `directiveWrapper` closure handles all directive chaining including `_fieldMiddleware` for any FIELD-location directives. Field-level directives are also expanded inline if `$field.HasDirectives`.

- In split-packages mode, the dispatcher calls `ec.ResolveField(ctx, "<Type>", "<Field>", field, obj)` which looks up a registered `__splitField_<Type>_<Field>` (from `split_fields_.gotpl`) and invokes it. That function calls `graphql.ResolveField[any](...)` with `__splitDirectives_<Type>_<Field>(next)` as its directive closure.

**Implication for WS1:** The generic `DispatchObject` does NOT need to do anything about directives. The per-type field handler's `Resolve` closure just calls the existing bespoke per-field function (single-file) or `ec.ResolveField` (split-packages, but split-packages is out of scope for WS1's first cut — see "Scope" below). All directive wrapping is preserved verbatim.

### Q2: Async/concurrent field resolution

**Answer:** `$field.IsConcurrent` is a **template-time** boolean, derived from the field's resolver shape (`MethodHasContext || IsResolver`, minus the `DisableConcurrency` opt-out). So at codegen time we know which fields need goroutine dispatch.

The per-type field-handler table needs `Concurrent bool` per entry. The generic `DispatchObject` switches on it:

- Non-concurrent → `out.Values[i] = handler.Resolve(ctx, ec, field, obj)` (with optional RRM wrapping for Root types — see Q below on Root vs non-Root).
- Concurrent → `out.Concurrently(i, func(ctx) graphql.Marshaler { return handler.Resolve(ctx, ec, field, obj) })`.

The Deferrable check (`field.Deferrable != nil`) is RUNTIME (it reads from the resolved `graphql.CollectedField`), and it's always-and-only paired with concurrent + non-Root. The generic dispatcher can keep this branch internal — it doesn't need to leak into the per-type table. The same goes for the panic recover wrapper and the root-resolver middleware: both are conditions of the dispatcher path, not per-field facts (except that `IsConcurrent` decides which path).

The cleanest factoring for WS1:
- Field handler table per type: `[]FieldHandler{{Name, NonNull, Concurrent, Resolve}}` (one slice).
- `DispatchObject` takes the type's `Root bool` and `ConcurrentObject bool` as separate args (or as a `TypeDescriptor` struct alongside the slice).
- Dispatcher internal branches on `Root`, `Concurrent`, and `field.Deferrable` to choose RRM / goroutine / deferred-set path.

### Q3: `out.Invalids` tracking

**Answer:** Two orthogonal facts:

- **Per-field:** `$field.TypeReference.GQL.NonNull` — only non-null fields increment `Invalids` when resolved to `graphql.Null`. Stored as `NonNull bool` in the field-handler table.
- **Per-object:** `$object.IsConcurrent` — drives whether the increment must be atomic. Stored as `ConcurrentObject bool` on the per-type descriptor (or constant-folded by emitting a separate generic `dispatchObjectConcurrent[T]` / `dispatchObjectSequential[T]` pair, but that doubles the helper count and only saves one atomic-vs-non-atomic branch per Null result — likely not worth it; just always use atomic, the cost is negligible for the rare invalids path).

---

## Scope Note for WS1 Implementation

The plan's stated goal is to replace the per-type `_<Type>` dispatcher with a generic `DispatchObject`. Three scoping decisions follow from this investigation:

1. **WS1 should target single-file mode only.** Split-packages dispatcher already delegates real work to `ec.ResolveField` (which is itself a generic dispatch), so the marginal savings from genericizing the split-packages `_<Name>` shells are small (~30 lines per type). The big win is in single-file where each `_<Type>` is the full switch. The `use_generic_dispatcher: true` flag should gate just the single-file template path. (Split-packages is also the slower mode per the bench we just ran — it's not the deployment target.)

2. **The per-field bespoke functions `_<Object>_<field>` stay.** WS1 does NOT delete them. They hold the field-context lookup, directive wrapping, resolver call, marshaling — all the per-field uniqueness. WS1 only collapses the SWITCH in `_<Type>`. The handler table's `Resolve` closure is a one-liner that calls into the bespoke function:
   ```go
   {
     Name: "name", NonNull: true, Concurrent: false,
     Resolve: func(ctx context.Context, ec graphql.ObjectExecutionContext, field graphql.CollectedField, obj any) graphql.Marshaler {
       return ec._User_name(ctx, field, obj)
     },
   },
   ```
   (Or, if function syntax is on: `return _User_name(ctx, ec, field, obj)`.)

3. **`graphql.ObjectExecutionContext` interface.** The fork already has `shardruntime.ObjectExecutionContext` used by split-packages, but that interface lives in the wrong package for cross-mode reuse. WS1 should define a new interface in the `graphql/` package with exactly the methods `DispatchObject` needs:
   ```go
   type ObjectExecutionContext interface {
       GetOperationContext() *OperationContext
       Error(ctx context.Context, err error)
       Recover(ctx context.Context, r any) error
   }
   ```
   The single-file `*executionContext` will need `GetOperationContext()` added (today it accesses `.OperationContext` as a field). Trivial method addition. Split-packages' `shardruntime.ObjectExecutionContext` already has these methods; if needed, it can embed `graphql.ObjectExecutionContext` for free interop.

   Important: the interface should NOT include `ResolveField` — that's split-packages-specific. Single-file's resolver dispatch goes through the bespoke `_<Object>_<field>` function directly, not through any interface method.

---

## Estimated Codegen Output Shape (Per Type)

Before (current single-file `_User` with 155 fields ≈ 3,077 lines):

```go
var userImplementors = []string{"User"}

func _User(ctx context.Context, ec ec.executionContext, sel ast.SelectionSet, obj *gen.User) graphql.Marshaler {
    fields := graphql.CollectFields(ec.OperationContext, sel, userImplementors)
    out := graphql.NewFieldSet(fields)
    deferred := make(map[string]*graphql.FieldSet)
    for i, field := range fields {
        switch field.Name {
        case "__typename":
            out.Values[i] = graphql.MarshalString("User")
        case "id":
            field := field
            innerFunc := func(ctx context.Context, fs *graphql.FieldSet) (res graphql.Marshaler) {
                defer func() { if r := recover(); r != nil { ec.Error(ctx, ec.Recover(ctx, r)) } }()
                res = ec._User_id(ctx, ec, field, obj)
                if res == graphql.Null { atomic.AddUint32(&fs.Invalids, 1) }
                return res
            }
            if field.Deferrable != nil { /* ... 8 lines ... */ }
            out.Concurrently(i, func(ctx context.Context) graphql.Marshaler { return innerFunc(ctx, out) })
        case "name":
            /* same shape, ~20 lines */
        // ... 153 more cases ...
        default:
            panic("unknown field " + strconv.Quote(field.Name))
        }
    }
    out.Dispatch(ctx)
    if out.Invalids > 0 { return graphql.Null }
    atomic.AddInt32(&ec.Deferred, int32(min(len(deferred), math.MaxInt32)))
    for label, dfs := range deferred {
        ec.ProcessDeferredGroup(graphql.DeferredGroup{Label: label, Path: graphql.GetPath(ctx), FieldSet: dfs, Context: ctx})
    }
    return out
}
```

After (target with `use_generic_dispatcher: true`):

```go
var userImplementors = []string{"User"}

var userFieldHandlers = []graphql.FieldHandler{
    {Name: "id", NonNull: true, Concurrent: true, Resolve: func(ctx context.Context, ec graphql.ObjectExecutionContext, field graphql.CollectedField, obj any) graphql.Marshaler { return _User_id(ctx, ec.(*executionContext), field, obj.(*gen.User)) }},
    {Name: "name", NonNull: true, Concurrent: false, Resolve: func(ctx context.Context, ec graphql.ObjectExecutionContext, field graphql.CollectedField, obj any) graphql.Marshaler { return _User_name(ctx, ec.(*executionContext), field, obj.(*gen.User)) }},
    // ... 153 more entries, 1 line each ...
}

func _User(ctx context.Context, ec ec.executionContext, sel ast.SelectionSet, obj *gen.User) graphql.Marshaler {
    return graphql.DispatchObject(ctx, ec, sel, obj, "User", userImplementors, userFieldHandlers, false /* Root */)
}
```

Per-type line count goes from ~(60 + 20*N) for N fields to ~(6 + 1*N). For User (N=155): 3,160 → 161 lines, a 95% reduction. The compiler still type-checks each handler closure, but they all share the same body shape (one call into a per-type-stable bespoke function), so the AST-shape diversity drops dramatically — the same mechanism that paid for `use_function_syntax_for_execution_context: true`.

Whether this beats the abandonment threshold (≥10% on BUILD cold OR GQLGEN cold OR GQLGEN warm wall, OR ≥10% memory on either) is what Task 8's bench will tell us.

---

## Smoke-Gen Entrypoint Correction

Plan §Task 4 §Step 3 says:
```bash
go run ../../../testserver/main.go generate
```

This file doesn't exist. Correct invocation is:
```bash
cd codegen/testdata/object_generic
go run ../../gqlgen.go -config gqlgen.yml -stub stub.go
```

(Note: `../../gqlgen.go` because `object_generic` is two levels under `codegen/testdata/`.)

The WS1 implementer should update the test fixture invocation accordingly. Alternatively, since gqlgen fixtures use `//go:generate` blocks in `*_test.go` files, add a `generated_test.go` with `//go:generate go run ../../../testdata/gqlgen.go -config gqlgen.yml -stub stub.go` at the top and use `go generate ./...`.
