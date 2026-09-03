package codegen

import (
	"go/token"
	"go/types"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/vektah/gqlparser/v2/ast"
)

func TestObjectInvalidsIncrement(t *testing.T) {
	// non-concurrent: all fields are non-resolver, non-method-with-context
	sequential := &Object{Definition: &ast.Definition{Name: "Query"}}
	sequential.Fields = []*Field{
		{FieldDefinition: &ast.FieldDefinition{Name: "foo"}, Object: sequential},
	}
	assert.Equal(t, "out.Invalids++", sequential.InvalidsIncrement("out"))
	assert.Equal(t, "fs.Invalids++", sequential.InvalidsIncrement("fs"))

	// concurrent: at least one resolver field
	obj := &Object{Definition: &ast.Definition{Name: "User"}}
	obj.Fields = []*Field{
		{
			FieldDefinition: &ast.FieldDefinition{Name: "name"},
			IsResolver:      true,
			Object:          obj,
		},
	}
	assert.Equal(t, "atomic.AddUint32(&out.Invalids, 1)", obj.InvalidsIncrement("out"))
	assert.Equal(t, "atomic.AddUint32(&fs.Invalids, 1)", obj.InvalidsIncrement("fs"))
}

func TestObjectInvalidsIncrement_DisableConcurrency(t *testing.T) {
	// DisableConcurrency=true makes IsConcurrent() false even with resolver fields
	obj := &Object{
		Definition:         &ast.Definition{Name: "Mutation"},
		DisableConcurrency: true,
	}
	obj.Fields = []*Field{
		{
			FieldDefinition: &ast.FieldDefinition{Name: "createUser"},
			IsResolver:      true,
			Object:          obj,
		},
	}
	assert.Equal(t, "out.Invalids++", obj.InvalidsIncrement("out"))
}

func TestObjectHasUnmarshal(t *testing.T) {
	mk := func(method string) *Object {
		pkg := types.NewPackage("example.com/x", "x")
		named := types.NewNamed(types.NewTypeName(token.NoPos, pkg, "In", nil), types.NewStruct(nil, nil), nil)
		if method != "" {
			sig := types.NewSignatureType(types.NewVar(token.NoPos, pkg, "i", types.NewPointer(named)), nil, nil, nil, nil, false)
			named.AddMethod(types.NewFunc(token.NoPos, pkg, method, sig))
		}
		return &Object{Definition: &ast.Definition{Name: "In", Kind: ast.InputObject}, Type: named}
	}
	assert.False(t, mk("").HasUnmarshal())
	assert.True(t, mk("UnmarshalGQL").HasUnmarshal())
	assert.True(t, mk("UnmarshalGQLContext").HasUnmarshal())
	assert.False(t, mk("MarshalGQL").HasUnmarshal())
}
