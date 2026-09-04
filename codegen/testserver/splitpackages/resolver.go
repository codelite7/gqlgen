package splitpackages

// THIS CODE WILL BE UPDATED WITH SCHEMA CHANGES. PREVIOUS IMPLEMENTATION FOR SCHEMA CHANGES WILL BE KEPT IN THE COMMENT SECTION. IMPLEMENTATION FOR UNCHANGED SCHEMA WILL BE KEPT.

import (
	"context"

	"github.com/99designs/gqlgen/codegen/testserver/splitpackages/model"
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

// DirectiveArg is the resolver for the directiveArg field.
func (r *queryResolver) DirectiveArg(ctx context.Context, arg string) (*string, error) {
	panic("not implemented")
}

// DirectiveNullableArg is the resolver for the directiveNullableArg field.
func (r *queryResolver) DirectiveNullableArg(ctx context.Context, arg *int, arg2 *int, arg3 *string) (*string, error) {
	panic("not implemented")
}

// DirectiveSingleNullableArg is the resolver for the directiveSingleNullableArg field.
func (r *queryResolver) DirectiveSingleNullableArg(ctx context.Context, arg1 *string) (*string, error) {
	panic("not implemented")
}

// DirectiveInputNullable is the resolver for the directiveInputNullable field.
func (r *queryResolver) DirectiveInputNullable(ctx context.Context, arg *model.InputDirectives) (*string, error) {
	panic("not implemented")
}

// DirectiveInput is the resolver for the directiveInput field.
func (r *queryResolver) DirectiveInput(ctx context.Context, arg model.InputDirectives) (*string, error) {
	panic("not implemented")
}

// DirectiveInputType is the resolver for the directiveInputType field.
func (r *queryResolver) DirectiveInputType(ctx context.Context, arg model.InnerInput) (*string, error) {
	panic("not implemented")
}

// DirectiveInputOuter is the resolver for the directiveInputOuter field.
func (r *queryResolver) DirectiveInputOuter(ctx context.Context, arg model.OuterWrapperInput) (*string, error) {
	panic("not implemented")
}

// DirectiveInputWithArgs is the resolver for the directiveInputWithArgs field.
func (r *queryResolver) DirectiveInputWithArgs(ctx context.Context, arg model.InputDirectivesWithArgs) (*string, error) {
	panic("not implemented")
}

// DirectiveObject is the resolver for the directiveObject field.
func (r *queryResolver) DirectiveObject(ctx context.Context) (*model.ObjectDirectives, error) {
	panic("not implemented")
}

// DirectiveObjectWithCustomGoModel is the resolver for the directiveObjectWithCustomGoModel field.
func (r *queryResolver) DirectiveObjectWithCustomGoModel(ctx context.Context) (*model.ObjectDirectivesWithCustomGoModel, error) {
	panic("not implemented")
}

// DirectiveFieldDef is the resolver for the directiveFieldDef field.
func (r *queryResolver) DirectiveFieldDef(ctx context.Context, ret string) (string, error) {
	panic("not implemented")
}

// DirectiveField is the resolver for the directiveField field.
func (r *queryResolver) DirectiveField(ctx context.Context) (*string, error) {
	panic("not implemented")
}

// DirectiveDouble is the resolver for the directiveDouble field.
func (r *queryResolver) DirectiveDouble(ctx context.Context) (*string, error) {
	panic("not implemented")
}

// DirectiveUnimplemented is the resolver for the directiveUnimplemented field.
func (r *queryResolver) DirectiveUnimplemented(ctx context.Context) (*string, error) {
	panic("not implemented")
}

// GoodbyeFromExtras is the resolver for the goodbyeFromExtras field.
func (r *queryResolver) GoodbyeFromExtras(ctx context.Context, name string) (string, error) {
	panic("not implemented")
}

// DirectiveArg is the resolver for the directiveArg field.
func (r *subscriptionResolver) DirectiveArg(ctx context.Context, arg string) (<-chan *string, error) {
	panic("not implemented")
}

// DirectiveNullableArg is the resolver for the directiveNullableArg field.
func (r *subscriptionResolver) DirectiveNullableArg(ctx context.Context, arg *int, arg2 *int, arg3 *string) (<-chan *string, error) {
	panic("not implemented")
}

// DirectiveDouble is the resolver for the directiveDouble field.
func (r *subscriptionResolver) DirectiveDouble(ctx context.Context) (<-chan *string, error) {
	panic("not implemented")
}

// DirectiveUnimplemented is the resolver for the directiveUnimplemented field.
func (r *subscriptionResolver) DirectiveUnimplemented(ctx context.Context) (<-chan *string, error) {
	panic("not implemented")
}

// Mutation returns MutationResolver implementation.
func (r *Resolver) Mutation() MutationResolver { return &mutationResolver{r} }

// Query returns QueryResolver implementation.
func (r *Resolver) Query() QueryResolver { return &queryResolver{r} }

// Subscription returns SubscriptionResolver implementation.
func (r *Resolver) Subscription() SubscriptionResolver { return &subscriptionResolver{r} }

type mutationResolver struct{ *Resolver }
type queryResolver struct{ *Resolver }
type subscriptionResolver struct{ *Resolver }
