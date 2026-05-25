//go:generate rm -rf internal/gqlgenexec
//go:generate rm -f resolver.go
//go:generate go run ../../../testdata/gqlgen.go -config gqlgen.yml -stub stub.go

package splitpackages

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/99designs/gqlgen/client"
	"github.com/99designs/gqlgen/graphql/handler"
)

func newStubServer(t *testing.T) *client.Client {
	t.Helper()
	resolvers := &Stub{}
	resolvers.QueryResolver.Hello = func(ctx context.Context, name string) (string, error) {
		return "Hello " + name, nil
	}
	resolvers.QueryResolver.GoodbyeFromExtras = func(ctx context.Context, name string) (string, error) {
		return "Goodbye " + name, nil
	}
	resolvers.MutationResolver.Greet = func(ctx context.Context, name string) (string, error) {
		return "Greetings " + name, nil
	}
	resolvers.MutationResolver.PingFromExtras = func(ctx context.Context) (string, error) {
		return "pong", nil
	}
	srv := handler.NewDefaultServer(NewExecutableSchema(Config{Resolvers: resolvers}))
	return client.New(srv)
}

func TestSplitPackagesLayout(t *testing.T) {
	c := newStubServer(t)

	var resp struct {
		Hello string
	}
	c.MustPost(`query { hello(name:"Ada") }`, &resp)
	require.Equal(t, "Hello Ada", resp.Hello)
}

// TestMultiFileRootFields exercises the partitioner's root-type slicing: Query
// and Mutation are extended across schema.graphql and extras.graphql, so
// fields from each .graphql file must register on their own shard while the
// gateway dispatcher routes correctly across them.
func TestMultiFileRootFields(t *testing.T) {
	c := newStubServer(t)

	t.Run("query fields from both files resolve", func(t *testing.T) {
		var resp struct {
			Hello   string
			Goodbye string `json:"goodbyeFromExtras"`
		}
		c.MustPost(`query { hello(name:"Ada") goodbyeFromExtras(name:"Ada") }`, &resp)
		require.Equal(t, "Hello Ada", resp.Hello)
		require.Equal(t, "Goodbye Ada", resp.Goodbye)
	})

	t.Run("mutation fields from both files resolve", func(t *testing.T) {
		var resp struct {
			Greet string
			Ping  string `json:"pingFromExtras"`
		}
		c.MustPost(`mutation { greet(name:"Ada") pingFromExtras }`, &resp)
		require.Equal(t, "Greetings Ada", resp.Greet)
		require.Equal(t, "pong", resp.Ping)
	})
}

// TestRootTypeShardDistribution asserts the partitioner places each root
// field in the shard for its declaring .graphql file. Without root-type
// slicing, the alphabetically-first file's shard absorbs every root field
// (the agent_licensing pattern observed in production).
func TestRootTypeShardDistribution(t *testing.T) {
	shardsDir := filepath.Join("internal", "gqlgenexec", "shards")

	want := map[string]struct{}{
		"Query.hello":             {}, // schema.graphql
		"Mutation.greet":          {}, // schema.graphql
		"Query.goodbyeFromExtras": {}, // extras.graphql
		"Mutation.pingFromExtras": {}, // extras.graphql
	}
	got := map[string]string{} // "Query.hello" -> shard pkg dir

	entries, err := os.ReadDir(shardsDir)
	require.NoError(t, err)
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		reg := filepath.Join(shardsDir, e.Name(), "register.generated.go")
		body, err := os.ReadFile(reg)
		if err != nil {
			continue
		}
		for _, root := range []string{"Query", "Mutation"} {
			// Non-stream fields are now pure data in var ShardDesc, emitted as
			// ShardFieldDef literals: {Object: "Query", Name: "hello", Def: ...}.
			marker := `{Object: "` + root + `", Name: "`
			rest := string(body)
			for {
				i := strings.Index(rest, marker)
				if i < 0 {
					break
				}
				rest = rest[i+len(marker):]
				before, _, ok := strings.Cut(rest, `"`)
				if !ok {
					break
				}
				name := before
				if name == "__schema" || name == "__type" {
					continue
				}
				got[root+"."+name] = e.Name()
			}
		}
	}

	for key := range want {
		require.Contains(t, got, key, "field %s should be registered by some shard", key)
	}

	schemaShard := got["Query.hello"]
	extrasShard := got["Query.goodbyeFromExtras"]
	require.NotEmpty(t, schemaShard, "schema.graphql shard not found")
	require.NotEmpty(t, extrasShard, "extras.graphql shard not found")
	require.NotEqual(t, schemaShard, extrasShard,
		"schema.graphql and extras.graphql fields must live in different shards")
	require.Equal(t, schemaShard, got["Mutation.greet"],
		"Mutation.greet (declared in schema.graphql) must live with Query.hello")
	require.Equal(
		t,
		extrasShard,
		got["Mutation.pingFromExtras"],
		"Mutation.pingFromExtras (declared in extras.graphql) must live with Query.goodbyeFromExtras",
	)
}

func TestSplitPackagesCodecCompile(t *testing.T) {
	schema := NewExecutableSchema(Config{Resolvers: &Stub{}})
	require.NotNil(t, schema)
}

func TestSplitPackagesCompiles(t *testing.T) {
	schema := NewExecutableSchema(Config{Resolvers: &Stub{}})
	require.NotNil(t, schema)
}

func TestSplitComplexityParity(t *testing.T) {
	t.Run("uses configured complexity handler", func(t *testing.T) {
		schema := NewExecutableSchema(Config{
			Resolvers: &Stub{},
			Complexity: ComplexityRoot{
				Query: struct {
					GoodbyeFromExtras func(childComplexity int, name string) int
					Hello             func(childComplexity int, name string) int
				}{
					Hello: func(childComplexity int, name string) int { return childComplexity + len(name) },
				},
			},
		})

		value, ok := schema.Complexity(
			context.Background(),
			"Query",
			"hello",
			4,
			map[string]any{"name": "Ada"},
		)
		require.True(t, ok)
		require.Equal(t, 7, value)
	})

	t.Run("returns false when complexity function is unset", func(t *testing.T) {
		schema := NewExecutableSchema(Config{Resolvers: &Stub{}})

		value, ok := schema.Complexity(
			context.Background(),
			"Query",
			"hello",
			2,
			map[string]any{"name": "Ada"},
		)
		require.False(t, ok)
		require.Equal(t, 0, value)
	})

	t.Run("returns false on argument parse failure", func(t *testing.T) {
		schema := NewExecutableSchema(Config{
			Resolvers: &Stub{},
			Complexity: ComplexityRoot{
				Query: struct {
					GoodbyeFromExtras func(childComplexity int, name string) int
					Hello             func(childComplexity int, name string) int
				}{
					Hello: func(childComplexity int, name string) int { return childComplexity },
				},
			},
		})

		value, ok := schema.Complexity(
			context.Background(),
			"Query",
			"hello",
			3,
			map[string]any{"name": []int{1}},
		)
		require.False(t, ok)
		require.Equal(t, 0, value)
	})
}
