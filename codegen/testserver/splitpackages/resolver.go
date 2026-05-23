package splitpackages

// THIS CODE WILL BE UPDATED WITH SCHEMA CHANGES. PREVIOUS IMPLEMENTATION FOR SCHEMA CHANGES WILL BE KEPT IN THE COMMENT SECTION. IMPLEMENTATION FOR UNCHANGED SCHEMA WILL BE KEPT.

import (
	"context"
)

type Resolver struct{}

// Greet is the resolver for the greet field.
func (r *mutationResolver) Greet(ctx context.Context, name string) (string, error) {
	panic("not implemented")
}

// PingFromExtras is the resolver for the pingFromExtras field.
func (r *mutationResolver) PingFromExtras(ctx context.Context) (string, error) {
	panic("not implemented")
}

// Hello is the resolver for the hello field.
func (r *queryResolver) Hello(ctx context.Context, name string) (string, error) {
	panic("not implemented")
}

// GoodbyeFromExtras is the resolver for the goodbyeFromExtras field.
func (r *queryResolver) GoodbyeFromExtras(ctx context.Context, name string) (string, error) {
	panic("not implemented")
}

// Mutation returns MutationResolver implementation.
func (r *Resolver) Mutation() MutationResolver { return &mutationResolver{r} }

// Query returns QueryResolver implementation.
func (r *Resolver) Query() QueryResolver { return &queryResolver{r} }

type mutationResolver struct{ *Resolver }
type queryResolver struct{ *Resolver }
