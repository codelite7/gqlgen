package public

import aliasinternal "github.com/99designs/gqlgen/codegen/config/testdata/aliasroot/internal"

type Operation = aliasinternal.Operation

type Entity struct {
	Operation         aliasinternal.Operation
	OptionalOperation *aliasinternal.Operation
}
