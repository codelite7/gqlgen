package singlefile

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/99designs/gqlgen/client"
	"github.com/99designs/gqlgen/graphql"
	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/transport"
)

// HybridInput's Go type has UnmarshalGQLContext, so gqlgen generates a hybrid
// unmarshaler: the method decodes the plain and defaulted fields, generated code
// still runs the @toUpper field directive and the `resolved` field resolver.
func TestHybridInputUnmarshaler(t *testing.T) {
	describe := func(in *HybridInput) string {
		if in == nil {
			return "<nil>"
		}
		return fmt.Sprintf("plain=%q default=%q directed=%q resolved=%q saw=%s",
			in.Plain, in.WithDefault, in.Directed, in.Resolved, strings.Join(in.SawKeys, ","))
	}

	resolvers := &Stub{}
	resolvers.QueryResolver.HybridInput = func(ctx context.Context, arg HybridInput) (string, error) {
		return describe(&arg), nil
	}
	resolvers.QueryResolver.HybridInputNullable = func(ctx context.Context, arg *HybridInput) (string, error) {
		return describe(arg), nil
	}
	resolvers.HybridInputResolver.Resolved = func(ctx context.Context, obj *HybridInput, data string) error {
		obj.Resolved = "resolver(" + data + ")"
		return nil
	}

	srv := handler.New(NewExecutableSchema(Config{
		Resolvers: resolvers,
		Directives: DirectiveRoot{
			ToUpper: func(ctx context.Context, obj any, next graphql.Resolver) (any, error) {
				res, err := next(ctx)
				if err != nil {
					return nil, err
				}
				return strings.ToUpper(res.(string)), nil
			},
		},
	}))
	srv.AddTransport(transport.POST{})
	c := client.New(srv)

	t.Run("method decodes plain fields, directive and resolver fields are withheld from it", func(t *testing.T) {
		var resp struct{ HybridInput string }
		c.MustPost(`{ hybridInput(arg: {plain: "p", withDefault: "explicit", directed: "up", resolved: "r"}) }`, &resp)
		require.Equal(t,
			`plain="p" default="explicit" directed="UP" resolved="resolver(r)" saw=plain,withDefault`,
			resp.HybridInput)
	})

	t.Run("default is applied and reaches the method", func(t *testing.T) {
		var resp struct{ HybridInput string }
		c.MustPost(`{ hybridInput(arg: {plain: "p", directed: "up", resolved: "r"}) }`, &resp)
		require.Equal(t,
			`plain="p" default="fromDefault" directed="UP" resolved="resolver(r)" saw=plain,withDefault`,
			resp.HybridInput)
	})

	t.Run("explicit null input is not an error", func(t *testing.T) {
		var resp struct{ HybridInputNullable string }
		c.MustPost(`{ hybridInputNullable(arg: null) }`, &resp)
		require.Equal(t, "<nil>", resp.HybridInputNullable)
	})

	// A non-map value cannot reach the unmarshaler through a query (gqlparser
	// rejects it while coercing variables), but it can through
	// graphql.UnmarshalInputFromContext — the path ent's gqlcollections uses when
	// it re-feeds an already-typed input. That must error, not panic.
	t.Run("non-map input is an error, not a panic", func(t *testing.T) {
		defer func(orig func(context.Context, HybridInput) (string, error)) {
			resolvers.QueryResolver.HybridInput = orig
		}(resolvers.QueryResolver.HybridInput)
		resolvers.QueryResolver.HybridInput = func(ctx context.Context, arg HybridInput) (string, error) {
			var out HybridInput
			return "", graphql.UnmarshalInputFromContext(ctx, 42, &out)
		}

		var resp struct{ HybridInput string }
		err := c.Post(`{ hybridInput(arg: {plain: "p", directed: "up", resolved: "r"}) }`, &resp)
		require.ErrorContains(t, err, "unmarshalInputHybridInput: expected map[string]any, got int")
	})
}
