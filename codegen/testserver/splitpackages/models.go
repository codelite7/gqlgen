package splitpackages

import "github.com/99designs/gqlgen/codegen/testserver/splitpackages/model"

// Aliases so the ported directive test (and stub wiring) can refer to the
// generated model types without qualification. Split-packages layout requires
// models to live outside the exec root package: shards import the model
// package, and the root imports the shards.
type (
	HybridInput                       = model.HybridInput
	InnerInput                        = model.InnerInput
	InputDirectives                   = model.InputDirectives
	ObjectDirectives                  = model.ObjectDirectives
	ObjectDirectivesWithCustomGoModel = model.ObjectDirectivesWithCustomGoModel
	Slices                            = model.Slices
	PtrToSliceContainer               = model.PtrToSliceContainer
	ListFieldInput                    = model.ListFieldInput
)
