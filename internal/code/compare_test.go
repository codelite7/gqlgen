package code

import (
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCompatibleTypes_InternalAliasNamed(t *testing.T) {
	internalPkg := types.NewPackage(
		"github.com/org/repo/ent/gen/internal/contacthistory",
		"contacthistory",
	)
	internalTypeName := types.NewTypeName(token.NoPos, internalPkg, "Operation", nil)
	internalNamed := types.NewNamed(internalTypeName, types.Typ[types.String], nil)

	aliasPkg := types.NewPackage(
		"github.com/org/repo/ent/gen/contacthistory",
		"contacthistory",
	)
	aliasTypeName := types.NewTypeName(token.NoPos, aliasPkg, "Operation", nil)
	alias := types.NewAlias(aliasTypeName, internalNamed)

	require.NoError(t, CompatibleTypes(alias, internalNamed))
	require.NoError(t, CompatibleTypes(internalNamed, alias))
}

func TestCompatibleTypes_InternalAliasNamedDifferentName(t *testing.T) {
	internalPkg := types.NewPackage(
		"github.com/org/repo/ent/gen/internal/contacthistory",
		"contacthistory",
	)
	internalTypeName := types.NewTypeName(token.NoPos, internalPkg, "Operation", nil)
	internalNamed := types.NewNamed(internalTypeName, types.Typ[types.String], nil)

	aliasPkg := types.NewPackage(
		"github.com/org/repo/ent/gen/contacthistory",
		"contacthistory",
	)
	aliasTypeName := types.NewTypeName(token.NoPos, aliasPkg, "OperationAlias", nil)
	alias := types.NewAlias(aliasTypeName, internalNamed)

	require.NoError(t, CompatibleTypes(alias, internalNamed))
	require.NoError(t, CompatibleTypes(internalNamed, alias))
}

func TestCompatibleTypes_InternalNonAliasNamed(t *testing.T) {
	internalPkg := types.NewPackage(
		"github.com/org/repo/ent/gen/internal/contacthistory",
		"contacthistory",
	)
	internalTypeName := types.NewTypeName(token.NoPos, internalPkg, "Operation", nil)
	internalNamed := types.NewNamed(internalTypeName, types.Typ[types.String], nil)

	publicPkg := types.NewPackage(
		"github.com/org/repo/ent/gen/contacthistory",
		"contacthistory",
	)
	publicTypeName := types.NewTypeName(token.NoPos, publicPkg, "Operation", nil)
	publicNamed := types.NewNamed(publicTypeName, types.Typ[types.String], nil)

	require.Error(t, CompatibleTypes(publicNamed, internalNamed))
	require.Error(t, CompatibleTypes(internalNamed, publicNamed))
}

func TestCompatibleTypes(t *testing.T) {
	valid := []struct {
		expected string
		actual   string
	}{
		{"string", "string"},
		{"*string", "string"},
		{"string", "*string"},
		{"*string", "*string"},
		{"[]string", "[]string"},
		{"*[]string", "[]string"},
		{"*[]string", "[]*string"},
		{"*[]*[]*[]string", "[][][]string"},
		{"map[string]any", "map[string]any"},
		{"map[string]string", "map[string]string"},
		{"Bar", "Bar"},
		{"any", "any"},
		{"interface{Foo() bool}", "interface{Foo() bool}"},
		{"struct{Foo bool}", "struct{Foo bool}"},
	}

	for _, tc := range valid {
		t.Run(tc.expected+"="+tc.actual, func(t *testing.T) {
			expectedType := parseTypeStr(t, tc.expected)
			actualType := parseTypeStr(t, tc.actual)
			require.NoError(t, CompatibleTypes(expectedType, actualType))
		})
	}

	invalid := []struct {
		expected string
		actual   string
	}{
		{"string", "int"},
		{"*string", "[]string"},
		{"[]string", "[][]string"},
		{"Bar", "Baz"},
		{"map[string]any", "map[string]string"},
		{"map[string]string", "[]string"},
		{"interface{Foo() bool}", "any"},
		{"struct{Foo bool}", "struct{Bar bool}"},
	}

	for _, tc := range invalid {
		t.Run(tc.expected+"!="+tc.actual, func(t *testing.T) {
			expectedType := parseTypeStr(t, tc.expected)
			actualType := parseTypeStr(t, tc.actual)
			require.Error(t, CompatibleTypes(expectedType, actualType))
		})
	}
}

func parseTypeStr(t *testing.T, s string) types.Type {
	t.Helper()

	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "test.go", `package test
		type Bar string
		type Baz string

		type Foo struct {
			Field `+s+`
		}
	`, 0)
	require.NoError(t, err)

	conf := types.Config{Importer: importer.Default()}
	pkg, err := conf.Check("test", fset, []*ast.File{f}, nil)
	require.NoError(t, err)

	return pkg.Scope().Lookup("Foo").Type().(*types.Named).Underlying().(*types.Struct).Field(0).
		Type()
}
