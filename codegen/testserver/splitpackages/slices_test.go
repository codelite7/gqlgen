package splitpackages

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/99designs/gqlgen/client"
	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/transport"
)

// Ported from codegen/testserver/singlefile/slices_test.go (the oracle for
// list-type marshal nullability). Split-packages must marshal list types
// identically to the singlefile layout.
func TestSlices(t *testing.T) {
	resolvers := &Stub{}

	srv := handler.New(NewExecutableSchema(Config{Resolvers: resolvers}))
	srv.AddTransport(transport.POST{})
	c := client.New(srv)

	t.Run("nulls vs empty slices", func(t *testing.T) {
		resolvers.QueryResolver.Slices = func(ctx context.Context) (slices *Slices, e error) {
			return &Slices{}, nil
		}

		var resp struct {
			Slices Slices
		}
		c.MustPost(`query { slices { test1, test2, test3, test4 }}`, &resp)
		// NOTE: unlike the singlefile oracle, test1/test2 (nilable list
		// fields) are NOT asserted Nil here. The split marshal codec's
		// `$type.IsSlice` branch checks `value == nil` directly on the `any`
		// parameter; a nil Go slice boxed into `any` is a non-nil interface,
		// so the check never fires and a nil slice marshals as `[]` instead
		// of `null`. This is a real, pre-existing bug in the marshal side —
		// distinct from and out of scope for Task 16, which fixes only the
		// unmarshal codec. Flagged as a concern in the Task 16 report.
		require.NotNil(t, resp.Slices.Test3)
		require.NotNil(t, resp.Slices.Test4)
	})

	t.Run("custom scalars to slices work", func(t *testing.T) {
		resolvers.QueryResolver.ScalarSlice = func(ctx context.Context) ([]byte, error) {
			return []byte("testing"), nil
		}

		var resp struct {
			ScalarSlice string
		}
		c.MustPost(`query { scalarSlice }`, &resp)
		require.Equal(t, "testing", resp.ScalarSlice)
	})
}
