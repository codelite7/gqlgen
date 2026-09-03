package binding

import (
	"context"
	"fmt"
	"io"

	"github.com/99designs/gqlgen/graphql"
)

type Number int

func (e *Number) UnmarshalGQL(v any) error {
	num, err := graphql.UnmarshalInt(v)
	if err != nil {
		return err
	}
	*e = Number(num)
	return nil
}

func (e Number) MarshalGQL(w io.Writer) error {
	fmt.Fprint(w, e)
	return nil
}

type ContextNumber int

func (e *ContextNumber) UnmarshalGQLContext(ctx context.Context, v any) error {
	num, err := graphql.UnmarshalInt(v)
	if err != nil {
		return err
	}
	*e = Number(num)
	return nil
}

func (e ContextNumber) MarshalGQLContext(_ context.Context, w io.Writer) error {
	fmt.Fprint(w, e)
	return nil
}

// ContextInput is an input object that decodes itself; it has no marshal
// method because input objects are never marshaled.
type ContextInput struct {
	Text string
}

func (i *ContextInput) UnmarshalGQLContext(ctx context.Context, v any) error {
	m, ok := v.(map[string]any)
	if !ok {
		return fmt.Errorf("expected map, got %T", v)
	}
	i.Text, _ = m["text"].(string)
	return nil
}
