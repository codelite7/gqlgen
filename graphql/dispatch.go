package graphql

import (
	"context"
	"math"
	"strconv"
	"sync/atomic"

	"github.com/vektah/gqlparser/v2/ast"
)

// ObjectExecutionContext is the runtime surface required by DispatchObject.
//
// In single-file mode the generated *executionContext type satisfies this
// interface via methods promoted from the embedded *ExecutionContextState
// (which itself embeds *OperationContext for Error/Recover). split-packages
// implements the same interface plus additional methods (ResolveField, codec
// helpers) that DispatchObject does not require.
type ObjectExecutionContext interface {
	GetOperationContext() *OperationContext
	Error(ctx context.Context, err error)
	Recover(ctx context.Context, r any) error
	AddDeferred(delta int32)
	ProcessDeferredGroup(dg DeferredGroup)
}

// FieldHandler is the per-field entry in a per-object handler table emitted by
// the generic-dispatcher codegen mode. The dispatcher consults the table at
// runtime to dispatch each selected field; the Resolve closure is a one-line
// trampoline into the per-field bespoke `_<Object>_<field>` function that
// continues to hold directive wrapping, field-context lookup, resolver call
// and marshaling — only the per-type switch is collapsed.
type FieldHandler struct {
	Name         string
	NonNull      bool
	Concurrent   bool
	RecoverPanic bool
	Resolve      func(
		ctx context.Context,
		ec ObjectExecutionContext,
		field CollectedField,
		obj any,
	) Marshaler
}

// DispatchObject is the generic per-object dispatcher. It replaces the giant
// per-type switch emitted by codegen/object.gotpl when
// `use_generic_dispatcher: true` is set in the gqlgen config.
//
// The mechanism is the same one that paid for use_function_syntax_for_-
// execution_context: collapse many per-type AST-distinct dispatchers into a
// single generic body so the compiler holds the helper's type-checked AST
// once instead of hundreds of near-identical copies.
//
// Semantics mirror object.gotpl's non-stream, non-split-packages branch. The
// per-field bespoke `_<Object>_<field>` functions are still emitted and still
// hold all per-field uniqueness (directives, field context, resolver call,
// marshaler); DispatchObject just orchestrates the loop, deferrable handling,
// concurrent dispatch and Null/Invalids bookkeeping.
func DispatchObject(
	ctx context.Context,
	ec ObjectExecutionContext,
	sel ast.SelectionSet,
	obj any,
	typeName string,
	implementors []string,
	handlers []FieldHandler,
	root bool,
) Marshaler {
	opCtx := ec.GetOperationContext()
	fields := CollectFields(opCtx, sel, implementors)
	if root {
		ctx = WithFieldContext(ctx, &FieldContext{Object: typeName})
	}

	out := NewFieldSet(fields)
	deferred := make(map[string]*FieldSet)

	for i, field := range fields {
		// __typename fast-path. Doesn't appear in the handler table.
		if field.Name == "__typename" {
			out.Values[i] = MarshalString(typeName)
			continue
		}

		var rootInnerCtx context.Context
		if root {
			rootInnerCtx = WithRootFieldContext(ctx, &RootFieldContext{
				Object: field.Name,
				Field:  field,
			})
		}

		handler := findHandler(handlers, field.Name)
		if handler == nil {
			panic("unknown field " + strconv.Quote(field.Name))
		}

		if handler.Concurrent {
			handlerCopy := *handler
			fieldCopy := field
			innerFunc := func(ctx context.Context, fs *FieldSet) (res Marshaler) {
				if handlerCopy.RecoverPanic {
					defer func() {
						if r := recover(); r != nil {
							ec.Error(ctx, ec.Recover(ctx, r))
						}
					}()
				}
				res = handlerCopy.Resolve(ctx, ec, fieldCopy, obj)
				if handlerCopy.NonNull && res == Null {
					atomic.AddUint32(&fs.Invalids, 1)
				}
				return res
			}

			if root {
				rrm := func(ctx context.Context) Marshaler {
					return opCtx.RootResolverMiddleware(ctx, func(ctx context.Context) Marshaler {
						return innerFunc(ctx, out)
					})
				}
				rootCtx := rootInnerCtx
				out.Concurrently(i, func(ctx context.Context) Marshaler {
					return rrm(rootCtx)
				})
				continue
			}

			// non-Root concurrent — honor @defer at runtime
			if fieldCopy.Deferrable != nil {
				dfs, ok := deferred[fieldCopy.Deferrable.Label]
				di := 0
				if ok {
					dfs.AddField(fieldCopy)
					di = len(dfs.Values) - 1
				} else {
					dfs = NewFieldSet([]CollectedField{fieldCopy})
					deferred[fieldCopy.Deferrable.Label] = dfs
				}
				dfs.Concurrently(di, func(ctx context.Context) Marshaler {
					return innerFunc(ctx, dfs)
				})
				out.Values[i] = Null
				continue
			}

			out.Concurrently(i, func(ctx context.Context) Marshaler {
				return innerFunc(ctx, out)
			})
			continue
		}

		// non-concurrent path
		if root {
			handlerCopy := *handler
			fieldCopy := field
			out.Values[i] = opCtx.RootResolverMiddleware(
				rootInnerCtx,
				func(ctx context.Context) Marshaler {
					return handlerCopy.Resolve(ctx, ec, fieldCopy, obj)
				},
			)
		} else {
			out.Values[i] = handler.Resolve(ctx, ec, field, obj)
		}
		if handler.NonNull && out.Values[i] == Null {
			atomic.AddUint32(&out.Invalids, 1)
		}
	}

	out.Dispatch(ctx)
	if out.Invalids > 0 {
		return Null
	}

	ec.AddDeferred(int32(min(len(deferred), math.MaxInt32)))

	for label, dfs := range deferred {
		ec.ProcessDeferredGroup(DeferredGroup{
			Label:    label,
			Path:     GetPath(ctx),
			FieldSet: dfs,
			Context:  ctx,
		})
	}

	return out
}

// findHandler returns a pointer to the handler matching name, or nil. Linear
// scan is intentional — most types have a small number of fields and a slice
// scan beats map overhead at typical sizes; switch to a built-once
// map[string]int only if profiling shows it matters for very wide types.
func findHandler(handlers []FieldHandler, name string) *FieldHandler {
	for i := range handlers {
		if handlers[i].Name == name {
			return &handlers[i]
		}
	}
	return nil
}
