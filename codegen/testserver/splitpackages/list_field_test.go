package splitpackages

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/99designs/gqlgen/client"
	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/transport"
)

// TestListFieldUnmarshalMatchesUpstream asserts, end to end through
// client.MustPost, that an input object's list field decodes exactly like
// upstream gqlgen (codegen/type.gotpl): omitted -> nil, [] -> a non-nil empty
// slice, [null] -> a one-element slice holding a nil element. This is the
// direct regression test for the split-packages slice codec bug (Task 16).
func TestListFieldUnmarshalMatchesUpstream(t *testing.T) {
	resolvers := &Stub{}
	srv := handler.New(NewExecutableSchema(Config{Resolvers: resolvers}))
	srv.AddTransport(transport.POST{})
	c := client.New(srv)

	resolvers.QueryResolver.InputListField = func(ctx context.Context, arg ListFieldInput) (string, error) {
		if arg.Items == nil {
			return "unset", nil
		}
		if len(arg.Items) == 0 {
			return "empty", nil
		}
		out := make([]string, len(arg.Items))
		for i, s := range arg.Items {
			if s == nil {
				out[i] = "null"
			} else {
				out[i] = *s
			}
		}
		return strings.Join(out, ","), nil
	}

	var resp struct {
		InputListField string
	}

	t.Run("omitted field decodes as nil", func(t *testing.T) {
		c.MustPost(`query { inputListField(arg: {}) }`, &resp)
		require.Equal(t, "unset", resp.InputListField)
	})

	t.Run("explicit empty list decodes as a non-nil empty slice", func(t *testing.T) {
		c.MustPost(`query { inputListField(arg: { items: [] }) }`, &resp)
		require.Equal(t, "empty", resp.InputListField)
	})

	t.Run("a null element decodes as a one-element slice holding nil", func(t *testing.T) {
		c.MustPost(`query { inputListField(arg: { items: [null] }) }`, &resp)
		require.Equal(t, "null", resp.InputListField)
	})

	t.Run("mixed values and null elements are threaded through in order", func(t *testing.T) {
		c.MustPost(`query { inputListField(arg: { items: ["a", null, "b"] }) }`, &resp)
		require.Equal(t, "a,null,b", resp.InputListField)
	})
}
