package singlefile

import (
	"context"
	"fmt"
	"slices"
)

// HybridInput is the Go binding for the HybridInput input object. It decodes
// itself with UnmarshalGQLContext, so gqlgen emits a hybrid unmarshaler: this
// method sees the plain and defaulted fields, while generated code keeps
// running the @toUpper field directive and the `resolved` field resolver.
type HybridInput struct {
	Plain       string
	WithDefault string
	Directed    string
	Resolved    string

	// SawKeys records, sorted, the keys the hybrid body handed to
	// UnmarshalGQLContext. Not a schema field.
	SawKeys []string
}

func (i *HybridInput) UnmarshalGQLContext(_ context.Context, v any) error {
	m, ok := v.(map[string]any)
	if !ok {
		return fmt.Errorf("HybridInput: expected map[string]any, got %T", v)
	}
	for k := range m {
		i.SawKeys = append(i.SawKeys, k)
	}
	slices.Sort(i.SawKeys)
	i.Plain, _ = m["plain"].(string)
	i.WithDefault, _ = m["withDefault"].(string)
	i.Directed, _ = m["directed"].(string)
	i.Resolved, _ = m["resolved"].(string)
	return nil
}
