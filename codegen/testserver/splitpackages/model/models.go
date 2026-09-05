package model

type ObjectDirectivesWithCustomGoModel struct {
	NullableText string // not *string, but schema is `String @toNull` type.
}
