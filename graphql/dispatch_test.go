package graphql_test

import (
	"bytes"
	"context"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/vektah/gqlparser/v2/ast"

	"github.com/99designs/gqlgen/graphql"
)

// testObjectExecutionContext is a minimal stand-in for the generated
// *executionContext that implements graphql.ObjectExecutionContext.
type testObjectExecutionContext struct {
	opCtx           *graphql.OperationContext
	errs            []error
	deferred        int32
	deferredGroups  []graphql.DeferredGroup
	recoverPassthru bool
}

func newTestObjectExecutionContext() *testObjectExecutionContext {
	opCtx := &graphql.OperationContext{
		ResolverMiddleware: func(ctx context.Context, next graphql.Resolver) (any, error) {
			return next(ctx)
		},
		RootResolverMiddleware: func(ctx context.Context, next graphql.RootResolver) graphql.Marshaler {
			return next(ctx)
		},
		RecoverFunc: graphql.DefaultRecover,
	}
	return &testObjectExecutionContext{opCtx: opCtx}
}

func (e *testObjectExecutionContext) GetOperationContext() *graphql.OperationContext {
	return e.opCtx
}

func (e *testObjectExecutionContext) Error(_ context.Context, err error) {
	e.errs = append(e.errs, err)
}

func (e *testObjectExecutionContext) Recover(_ context.Context, r any) error {
	if e.recoverPassthru {
		if err, ok := r.(error); ok {
			return err
		}
	}
	return nil
}

func (e *testObjectExecutionContext) AddDeferred(delta int32) {
	atomic.AddInt32(&e.deferred, delta)
}

func (e *testObjectExecutionContext) ProcessDeferredGroup(dg graphql.DeferredGroup) {
	e.deferredGroups = append(e.deferredGroups, dg)
}

// withOpCtx returns ctx with the test's operation context attached.
func (e *testObjectExecutionContext) withOpCtx(ctx context.Context) context.Context {
	return graphql.WithOperationContext(ctx, e.opCtx)
}

// makeFieldSel returns an ast.SelectionSet with a single field of the given name
// whose type is `String!` (non-null) or `String` based on nonNull.
func makeFieldSel(name string, nonNull bool) ast.SelectionSet {
	var t *ast.Type
	if nonNull {
		t = ast.NonNullNamedType("String", nil)
	} else {
		t = ast.NamedType("String", nil)
	}
	return ast.SelectionSet{
		&ast.Field{
			Name:  name,
			Alias: name,
			Definition: &ast.FieldDefinition{
				Name: name,
				Type: t,
			},
		},
	}
}

// marshalToBytes calls MarshalGQL on the given marshaler and returns the bytes.
func marshalToBytes(m graphql.Marshaler) []byte {
	buf := &bytes.Buffer{}
	m.MarshalGQL(buf)
	return buf.Bytes()
}

type testUser struct {
	Name string
}

func TestDispatchObject_BasicScalar(t *testing.T) {
	ec := newTestObjectExecutionContext()
	ctx := ec.withOpCtx(context.Background())

	sel := makeFieldSel("name", true)
	user := &testUser{Name: "alice"}

	handlers := []graphql.FieldHandler{
		{
			Name:       "name",
			NonNull:    true,
			Concurrent: false,
			Resolve: func(ctx context.Context, _ graphql.ObjectExecutionContext, _ graphql.CollectedField, obj any) graphql.Marshaler {
				return graphql.MarshalString(obj.(*testUser).Name)
			},
		},
	}

	res := graphql.DispatchObject(
		ctx, ec, sel, user, "User", []string{"User"}, handlers, false,
	)
	require.NotNil(t, res)
	require.Equal(t, `{"name":"alice"}`, string(marshalToBytes(res)))
}
