package codegen_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestObjectDispatcherEmitsTable verifies that when use_generic_dispatcher is
// enabled in the gqlgen config, codegen/object.gotpl emits the per-type
// FieldHandler table + DispatchObject call instead of the historical
// `switch field.Name { case "id": ... }` shape.
//
// Fixture lives at codegen/testdata/object_generic/. Regenerate with:
//
//	cd codegen/testdata/object_generic
//	go run ../../../testdata/gqlgen.go -config gqlgen.yml -stub stub.go
func TestObjectDispatcherEmitsTable(t *testing.T) {
	generated := readFixtureGenerated(t)

	// The handler table is declared at package level and populated in init()
	// to break the package-level init cycle that forms via introspection types
	// (__Type -> __Field -> __InputValue -> __Type) — each per-field Resolve
	// closure transitively references other types' tables, and Go's static
	// init-cycle analyzer follows closure bodies. init() runs after var
	// initialization, sidestepping the analyzer.
	require.Contains(t, generated, "var userFieldHandlers []graphql.FieldHandler",
		"expected per-type field handler table declaration for User")
	require.Contains(t, generated, "userFieldHandlers = []graphql.FieldHandler{",
		"expected per-type field handler table population for User (in init())")
	require.Contains(t, generated, "graphql.DispatchObject(",
		"expected DispatchObject call in collapsed dispatcher")

	// The collapsed _User dispatcher must not contain the legacy switch.
	// Other generated functions (childFields_User, fieldContext_*) still use
	// switch field.Name for their own purposes — those are unrelated.
	userBody := extractFuncBody(t, generated, "func _User(")
	require.NotContains(t, userBody, `case "id":`,
		"_User must not contain the legacy switch-style case labels")
	require.NotContains(t, userBody, `case "name":`,
		"_User must not contain the legacy switch-style case labels")
	require.NotContains(t, userBody, "switch field.Name",
		"_User must not contain the legacy switch on field.Name")
	require.Contains(t, userBody, "graphql.DispatchObject(",
		"_User must delegate to graphql.DispatchObject")
}

// extractFuncBody returns the source text of the named function (everything
// between `func <name>...` and the matching closing brace at column 0). It is
// deliberately simple — it assumes the generated source is gofmt-clean, which
// gqlgen always emits.
func extractFuncBody(t *testing.T, src, prefix string) string {
	t.Helper()
	start := strings.Index(src, prefix)
	require.GreaterOrEqual(t, start, 0, "function %q not found", prefix)
	// Find the closing brace that sits at column 0 (a top-level brace).
	rest := src[start:]
	for i := 0; i < len(rest); i++ {
		if rest[i] == '\n' && i+1 < len(rest) && rest[i+1] == '}' {
			return rest[:i+2]
		}
	}
	t.Fatalf("could not find end of function %q", prefix)
	return ""
}

func readFixtureGenerated(t *testing.T) string {
	t.Helper()
	path := filepath.Join("testdata", "object_generic", "graph", "generated.go")
	data, err := os.ReadFile(path)
	require.NoError(t, err, "fixture generated.go must exist; run `go generate` in testdata/object_generic")
	// Sanity: must be a real generated.go (not an empty placeholder).
	require.Greater(t, len(data), 100, "generated.go is suspiciously short")
	require.True(t, strings.Contains(string(data), "package graph"), "generated.go missing package declaration")
	return string(data)
}
