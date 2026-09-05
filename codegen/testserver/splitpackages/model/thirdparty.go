package model

import (
	"fmt"
	"io"
	"strconv"

	"github.com/99designs/gqlgen/graphql"
)

type ThirdParty struct {
	str string
}

// String exposes the unexported value for tests outside this package (the
// split-packages layout keeps models in their own package, unlike singlefile
// where ThirdParty and its tests share a package and can reach tp.str directly).
func (tp ThirdParty) String() string {
	return tp.str
}

func MarshalThirdParty(tp ThirdParty) graphql.Marshaler {
	return graphql.WriterFunc(func(w io.Writer) {
		_, err := io.WriteString(w, strconv.Quote(tp.str))
		if err != nil {
			panic(err)
		}
	})
}

func UnmarshalThirdParty(input any) (ThirdParty, error) {
	switch input := input.(type) {
	case string:
		return ThirdParty{str: input}, nil
	default:
		return ThirdParty{}, fmt.Errorf("unknown type for input: %s", input)
	}
}
