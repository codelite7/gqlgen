package shardruntime

import (
	"bytes"
	"context"
	"fmt"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/vektah/gqlparser/v2/ast"

	"github.com/99designs/gqlgen/graphql"
)

func TestFieldRegistry(t *testing.T) {
	resetFieldRegistryForTest()

	h := func(context.Context, ObjectExecutionContext, graphql.CollectedField, any) graphql.Marshaler {
		return graphql.Null
	}

	if got, ok := LookupField("scope", "Query", "name"); ok || got != nil {
		t.Fatalf("unexpected field handler before registration: handler=%v ok=%v", got, ok)
	}

	RegisterField("scope", "Query", "name", h)

	got, ok := LookupField("scope", "Query", "name")
	if !ok {
		t.Fatal("expected registered field handler")
	}
	if got == nil {
		t.Fatal("expected non-nil field handler")
	}

	if got, ok := LookupField("scope", "Query", "missing"); ok || got != nil {
		t.Fatalf("unexpected field handler for missing field: handler=%v ok=%v", got, ok)
	}
	if got, ok := LookupField("scope", "Mutation", "name"); ok || got != nil {
		t.Fatalf("unexpected field handler for missing object: handler=%v ok=%v", got, ok)
	}
	if got, ok := LookupField("other-scope", "Query", "name"); ok || got != nil {
		t.Fatalf("unexpected field handler for missing scope: handler=%v ok=%v", got, ok)
	}

	defer func() {
		recovered := recover()
		if recovered == nil {
			t.Fatal("expected duplicate registration panic")
		}
		msg, ok := recovered.(string)
		if !ok {
			t.Fatalf("expected panic string, got %T", recovered)
		}
		expected := "duplicate field shard handler registration: scope:Query:name"
		if msg != expected {
			t.Fatalf("unexpected panic message: got %q want %q", msg, expected)
		}
	}()

	RegisterField("scope", "Query", "name", h)
}

func TestFieldLookupSnapshotIsBuiltLazily(t *testing.T) {
	resetFieldRegistryForTest()

	h := func(context.Context, ObjectExecutionContext, graphql.CollectedField, any) graphql.Marshaler {
		return graphql.Null
	}

	const total = 16
	for i := range total {
		RegisterField("scope", "Query", fmt.Sprintf("field_%03d", i), h)
	}

	if got := len(loadFieldLookupSnapshot()); got != 0 {
		t.Fatalf("unexpected eager field snapshot size: got %d want 0", got)
	}

	got, ok := LookupField("scope", "Query", "field_000")
	if !ok || got == nil {
		t.Fatal("expected lookup to resolve registered field after lazy snapshot build")
	}

	if got := len(loadFieldLookupSnapshot()); got != total {
		t.Fatalf("unexpected rebuilt field snapshot size: got %d want %d", got, total)
	}
}

func TestStreamFieldRegistry(t *testing.T) {
	resetStreamFieldRegistryForTest()

	h := func(context.Context, ObjectExecutionContext, graphql.CollectedField, any) func(context.Context) graphql.Marshaler {
		return func(context.Context) graphql.Marshaler {
			return graphql.Null
		}
	}

	if got, ok := LookupStreamField("scope", "Query", "name"); ok || got != nil {
		t.Fatalf("unexpected stream field handler before registration: handler=%v ok=%v", got, ok)
	}

	RegisterStreamField("scope", "Query", "name", h)

	got, ok := LookupStreamField("scope", "Query", "name")
	if !ok {
		t.Fatal("expected registered stream field handler")
	}
	if got == nil {
		t.Fatal("expected non-nil stream field handler")
	}

	if got, ok := LookupStreamField("scope", "Query", "missing"); ok || got != nil {
		t.Fatalf("unexpected stream field handler for missing field: handler=%v ok=%v", got, ok)
	}
	if got, ok := LookupStreamField("scope", "Mutation", "name"); ok || got != nil {
		t.Fatalf("unexpected stream field handler for missing object: handler=%v ok=%v", got, ok)
	}
	if got, ok := LookupStreamField("other-scope", "Query", "name"); ok || got != nil {
		t.Fatalf("unexpected stream field handler for missing scope: handler=%v ok=%v", got, ok)
	}

	defer func() {
		recovered := recover()
		if recovered == nil {
			t.Fatal("expected duplicate registration panic")
		}
		msg, ok := recovered.(string)
		if !ok {
			t.Fatalf("expected panic string, got %T", recovered)
		}
		expected := "duplicate stream field shard handler registration: scope:Query:name"
		if msg != expected {
			t.Fatalf("unexpected panic message: got %q want %q", msg, expected)
		}
	}()

	RegisterStreamField("scope", "Query", "name", h)
}

func TestComplexityRegistry(t *testing.T) {
	resetComplexityRegistryForTest()

	h := func(context.Context, ObjectExecutionContext, int, map[string]any) (int, bool) {
		return 42, true
	}

	if got, ok := LookupComplexity("scope", "Query", "name"); ok || got != nil {
		t.Fatalf("unexpected complexity handler before registration: handler=%v ok=%v", got, ok)
	}

	RegisterComplexity("scope", "Query", "name", h)

	got, ok := LookupComplexity("scope", "Query", "name")
	if !ok {
		t.Fatal("expected registered complexity handler")
	}
	if got == nil {
		t.Fatal("expected non-nil complexity handler")
	}

	if got, ok := LookupComplexity("scope", "Query", "missing"); ok || got != nil {
		t.Fatalf("unexpected complexity handler for missing field: handler=%v ok=%v", got, ok)
	}
	if got, ok := LookupComplexity("scope", "Mutation", "name"); ok || got != nil {
		t.Fatalf("unexpected complexity handler for missing object: handler=%v ok=%v", got, ok)
	}
	if got, ok := LookupComplexity("other-scope", "Query", "name"); ok || got != nil {
		t.Fatalf("unexpected complexity handler for missing scope: handler=%v ok=%v", got, ok)
	}

	defer func() {
		recovered := recover()
		if recovered == nil {
			t.Fatal("expected duplicate registration panic")
		}
		msg, ok := recovered.(string)
		if !ok {
			t.Fatalf("expected panic string, got %T", recovered)
		}
		expected := "duplicate complexity shard handler registration: scope:Query:name"
		if msg != expected {
			t.Fatalf("unexpected panic message: got %q want %q", msg, expected)
		}
	}()

	RegisterComplexity("scope", "Query", "name", h)
}

func TestInputUnmarshalRegistryDeterministicOrder(t *testing.T) {
	resetInputUnmarshalRegistryForTest()

	type marker struct{ id string }
	inputB := &marker{id: "B"}
	inputA := &marker{id: "A"}
	inputC := &marker{id: "C"}

	if got := ListInputUnmarshalers("scope", nil); got != nil {
		t.Fatalf("unexpected input unmarshalers before registration: %v", got)
	}

	RegisterInputUnmarshaler("scope", "InputB", inputB)
	RegisterInputUnmarshaler("scope", "InputA", inputA)
	RegisterInputUnmarshaler("scope", "InputC", inputC)
	RegisterInputUnmarshaler("other-scope", "InputA", &marker{id: "other"})

	got := ListInputUnmarshalers("scope", nil)
	if len(got) != 3 {
		t.Fatalf("unexpected number of input unmarshalers: got %d want %d", len(got), 3)
	}

	if got[0] != inputA {
		t.Fatalf("unexpected first input unmarshaler: got %v want %v", got[0], inputA)
	}
	if got[1] != inputB {
		t.Fatalf("unexpected second input unmarshaler: got %v want %v", got[1], inputB)
	}
	if got[2] != inputC {
		t.Fatalf("unexpected third input unmarshaler: got %v want %v", got[2], inputC)
	}

	if got := ListInputUnmarshalers("missing-scope", nil); got != nil {
		t.Fatalf("unexpected input unmarshalers for missing scope: %v", got)
	}

	defer func() {
		recovered := recover()
		if recovered == nil {
			t.Fatal("expected duplicate registration panic")
		}
		msg, ok := recovered.(string)
		if !ok {
			t.Fatalf("expected panic string, got %T", recovered)
		}
		expected := "duplicate input unmarshaler registration: scope:InputA"
		if msg != expected {
			t.Fatalf("unexpected panic message: got %q want %q", msg, expected)
		}
	}()

	RegisterInputUnmarshaler("scope", "InputA", &marker{id: "dup"})
}

func TestInputUnmarshalMap(t *testing.T) {
	resetInputUnmarshalRegistryForTest()

	type inputA struct{ Value string }
	type inputB struct{ Value string }

	fnA := func(context.Context, any) (inputA, error) { return inputA{}, nil }
	fnB := func(context.Context, any) (inputB, error) { return inputB{}, nil }

	RegisterInputUnmarshaler("scope", "InputA", fnA)
	RegisterInputUnmarshaler("scope", "InputB", fnB)

	inputMap := InputUnmarshalMap("scope", nil)
	if len(inputMap) != 2 {
		t.Fatalf("unexpected number of input unmarshalers in map: got %d want %d", len(inputMap), 2)
	}

	if _, ok := inputMap[reflect.TypeFor[inputA]()]; !ok {
		t.Fatal("missing inputA unmarshaler in map")
	}
	if _, ok := inputMap[reflect.TypeFor[inputB]()]; !ok {
		t.Fatal("missing inputB unmarshaler in map")
	}

	missingScope := InputUnmarshalMap("missing-scope", nil)
	if len(missingScope) != 0 {
		t.Fatalf(
			"expected empty input unmarshaler map for missing scope, got %d entries",
			len(missingScope),
		)
	}
}

func TestCodecMarshalRegistry(t *testing.T) {
	resetCodecMarshalRegistryForTest()

	h := func(context.Context, ObjectExecutionContext, ast.SelectionSet, any) graphql.Marshaler {
		return graphql.Null
	}

	if got, ok := LookupCodecMarshal("scope", "marshalFoo"); ok || got != nil {
		t.Fatalf("unexpected codec marshal handler before registration: handler=%v ok=%v", got, ok)
	}

	RegisterCodecMarshal("scope", "marshalFoo", h)

	got, ok := LookupCodecMarshal("scope", "marshalFoo")
	if !ok {
		t.Fatal("expected registered codec marshal handler")
	}
	if got == nil {
		t.Fatal("expected non-nil codec marshal handler")
	}

	if got, ok := LookupCodecMarshal("scope", "marshalMissing"); ok || got != nil {
		t.Fatalf("unexpected codec marshal handler for missing func: handler=%v ok=%v", got, ok)
	}
	if got, ok := LookupCodecMarshal("other-scope", "marshalFoo"); ok || got != nil {
		t.Fatalf("unexpected codec marshal handler for missing scope: handler=%v ok=%v", got, ok)
	}

	defer func() {
		recovered := recover()
		if recovered == nil {
			t.Fatal("expected duplicate registration panic")
		}
		msg, ok := recovered.(string)
		if !ok {
			t.Fatalf("expected panic string, got %T", recovered)
		}
		expected := "duplicate codec marshal handler registration: scope:marshalFoo"
		if msg != expected {
			t.Fatalf("unexpected panic message: got %q want %q", msg, expected)
		}
	}()

	RegisterCodecMarshal("scope", "marshalFoo", h)
}

func TestCodecUnmarshalRegistry(t *testing.T) {
	resetCodecUnmarshalRegistryForTest()

	h := func(context.Context, ObjectExecutionContext, any) (any, error) {
		return nil, nil
	}

	if got, ok := LookupCodecUnmarshal("scope", "unmarshalBar"); ok || got != nil {
		t.Fatalf(
			"unexpected codec unmarshal handler before registration: handler=%v ok=%v",
			got,
			ok,
		)
	}

	RegisterCodecUnmarshal("scope", "unmarshalBar", h)

	got, ok := LookupCodecUnmarshal("scope", "unmarshalBar")
	if !ok {
		t.Fatal("expected registered codec unmarshal handler")
	}
	if got == nil {
		t.Fatal("expected non-nil codec unmarshal handler")
	}

	if got, ok := LookupCodecUnmarshal("scope", "unmarshalMissing"); ok || got != nil {
		t.Fatalf("unexpected codec unmarshal handler for missing func: handler=%v ok=%v", got, ok)
	}
	if got, ok := LookupCodecUnmarshal("other-scope", "unmarshalBar"); ok || got != nil {
		t.Fatalf("unexpected codec unmarshal handler for missing scope: handler=%v ok=%v", got, ok)
	}

	defer func() {
		recovered := recover()
		if recovered == nil {
			t.Fatal("expected duplicate registration panic")
		}
		msg, ok := recovered.(string)
		if !ok {
			t.Fatalf("expected panic string, got %T", recovered)
		}
		expected := "duplicate codec unmarshal handler registration: scope:unmarshalBar"
		if msg != expected {
			t.Fatalf("unexpected panic message: got %q want %q", msg, expected)
		}
	}()

	RegisterCodecUnmarshal("scope", "unmarshalBar", h)
}

func TestRegistryDuplicatePanics(t *testing.T) {
	t.Run("object", func(t *testing.T) {
		resetObjectRegistryForTest()

		h := func(context.Context, ObjectExecutionContext, ast.SelectionSet, any) graphql.Marshaler {
			return graphql.Null
		}

		RegisterObject(
			"scope",
			"Query",
			func(ctx context.Context, ec ObjectExecutionContext, sel ast.SelectionSet, obj any) graphql.Marshaler {
				return h(ctx, ec, sel, obj)
			},
		)
		assertDuplicateRegistrationPanic(
			t,
			"duplicate object shard handler registration: scope:Query",
			func() {
				RegisterObject(
					"scope",
					"Query",
					func(ctx context.Context, ec ObjectExecutionContext, sel ast.SelectionSet, obj any) graphql.Marshaler {
						return h(ctx, ec, sel, obj)
					},
				)
			},
		)
	})

	t.Run("stream object", func(t *testing.T) {
		resetStreamObjectRegistryForTest()

		h := func(context.Context, ObjectExecutionContext, ast.SelectionSet) func(context.Context) graphql.Marshaler {
			return func(context.Context) graphql.Marshaler {
				return graphql.Null
			}
		}

		RegisterStreamObject(
			"scope",
			"Query",
			func(ctx context.Context, ec ObjectExecutionContext, sel ast.SelectionSet) func(context.Context) graphql.Marshaler {
				return h(ctx, ec, sel)
			},
		)
		assertDuplicateRegistrationPanic(
			t,
			"duplicate stream object shard handler registration: scope:Query",
			func() {
				RegisterStreamObject(
					"scope",
					"Query",
					func(ctx context.Context, ec ObjectExecutionContext, sel ast.SelectionSet) func(context.Context) graphql.Marshaler {
						return h(ctx, ec, sel)
					},
				)
			},
		)
	})

	t.Run("field", func(t *testing.T) {
		resetFieldRegistryForTest()

		h := func(context.Context, ObjectExecutionContext, graphql.CollectedField, any) graphql.Marshaler {
			return graphql.Null
		}

		RegisterField("scope", "Query", "name", h)
		assertDuplicateRegistrationPanic(
			t,
			"duplicate field shard handler registration: scope:Query:name",
			func() {
				RegisterField("scope", "Query", "name", h)
			},
		)
	})

	t.Run("stream field", func(t *testing.T) {
		resetStreamFieldRegistryForTest()

		h := func(context.Context, ObjectExecutionContext, graphql.CollectedField, any) func(context.Context) graphql.Marshaler {
			return func(context.Context) graphql.Marshaler {
				return graphql.Null
			}
		}

		RegisterStreamField("scope", "Query", "name", h)
		assertDuplicateRegistrationPanic(
			t,
			"duplicate stream field shard handler registration: scope:Query:name",
			func() {
				RegisterStreamField("scope", "Query", "name", h)
			},
		)
	})

	t.Run("complexity", func(t *testing.T) {
		resetComplexityRegistryForTest()

		h := func(context.Context, ObjectExecutionContext, int, map[string]any) (int, bool) {
			return 42, true
		}

		RegisterComplexity("scope", "Query", "name", h)
		assertDuplicateRegistrationPanic(
			t,
			"duplicate complexity shard handler registration: scope:Query:name",
			func() {
				RegisterComplexity("scope", "Query", "name", h)
			},
		)
	})

	t.Run("input unmarshaler", func(t *testing.T) {
		resetInputUnmarshalRegistryForTest()

		type marker struct{ id string }

		RegisterInputUnmarshaler("scope", "InputA", &marker{id: "A"})
		assertDuplicateRegistrationPanic(
			t,
			"duplicate input unmarshaler registration: scope:InputA",
			func() {
				RegisterInputUnmarshaler("scope", "InputA", &marker{id: "dup"})
			},
		)
	})

	t.Run("codec marshal", func(t *testing.T) {
		resetCodecMarshalRegistryForTest()

		h := func(context.Context, ObjectExecutionContext, ast.SelectionSet, any) graphql.Marshaler {
			return graphql.Null
		}

		RegisterCodecMarshal("scope", "marshalFoo", h)
		assertDuplicateRegistrationPanic(
			t,
			"duplicate codec marshal handler registration: scope:marshalFoo",
			func() {
				RegisterCodecMarshal("scope", "marshalFoo", h)
			},
		)
	})

	t.Run("codec unmarshal", func(t *testing.T) {
		resetCodecUnmarshalRegistryForTest()

		h := func(context.Context, ObjectExecutionContext, any) (any, error) {
			return nil, nil
		}

		RegisterCodecUnmarshal("scope", "unmarshalBar", h)
		assertDuplicateRegistrationPanic(
			t,
			"duplicate codec unmarshal handler registration: scope:unmarshalBar",
			func() {
				RegisterCodecUnmarshal("scope", "unmarshalBar", h)
			},
		)
	})
}

func TestRegistryConcurrentAccess(t *testing.T) {
	t.Run("field", func(t *testing.T) {
		resetFieldRegistryForTest()

		h := func(context.Context, ObjectExecutionContext, graphql.CollectedField, any) graphql.Marshaler {
			return graphql.Null
		}

		const total = 128
		const readers = 8

		errCh := make(chan error, total+readers)
		reportErr := func(err error) {
			select {
			case errCh <- err:
			default:
			}
		}

		start := make(chan struct{})
		var writersWG sync.WaitGroup
		var readersWG sync.WaitGroup
		var writesDone atomic.Bool

		for i := range total {
			writersWG.Add(1)
			go func(i int) {
				defer writersWG.Done()
				<-start
				RegisterField("scope", "Query", fmt.Sprintf("field_%03d", i), h)
			}(i)
		}

		for range readers {
			readersWG.Go(func() {
				<-start
				for !writesDone.Load() {
					for i := range total {
						handler, ok := LookupField("scope", "Query", fmt.Sprintf("field_%03d", i))
						if ok && handler == nil {
							reportErr(fmt.Errorf("nil field handler for registered key %d", i))
						}
					}
				}
			})
		}

		close(start)
		writersWG.Wait()
		writesDone.Store(true)
		readersWG.Wait()

		close(errCh)
		for err := range errCh {
			t.Fatal(err)
		}

		for i := range total {
			handler, ok := LookupField("scope", "Query", fmt.Sprintf("field_%03d", i))
			if !ok || handler == nil {
				t.Fatalf("missing registered field handler for key %d", i)
			}
		}
	})

	t.Run("stream field", func(t *testing.T) {
		resetStreamFieldRegistryForTest()

		h := func(context.Context, ObjectExecutionContext, graphql.CollectedField, any) func(context.Context) graphql.Marshaler {
			return func(context.Context) graphql.Marshaler {
				return graphql.Null
			}
		}

		const total = 128
		const readers = 8

		errCh := make(chan error, total+readers)
		reportErr := func(err error) {
			select {
			case errCh <- err:
			default:
			}
		}

		start := make(chan struct{})
		var writersWG sync.WaitGroup
		var readersWG sync.WaitGroup
		var writesDone atomic.Bool

		for i := range total {
			writersWG.Add(1)
			go func(i int) {
				defer writersWG.Done()
				<-start
				RegisterStreamField("scope", "Query", fmt.Sprintf("stream_field_%03d", i), h)
			}(i)
		}

		for range readers {
			readersWG.Go(func() {
				<-start
				for !writesDone.Load() {
					for i := range total {
						handler, ok := LookupStreamField(
							"scope",
							"Query",
							fmt.Sprintf("stream_field_%03d", i),
						)
						if ok && handler == nil {
							reportErr(
								fmt.Errorf("nil stream field handler for registered key %d", i),
							)
						}
					}
				}
			})
		}

		close(start)
		writersWG.Wait()
		writesDone.Store(true)
		readersWG.Wait()

		close(errCh)
		for err := range errCh {
			t.Fatal(err)
		}

		for i := range total {
			handler, ok := LookupStreamField("scope", "Query", fmt.Sprintf("stream_field_%03d", i))
			if !ok || handler == nil {
				t.Fatalf("missing registered stream field handler for key %d", i)
			}
		}
	})

	t.Run("complexity", func(t *testing.T) {
		resetComplexityRegistryForTest()

		h := func(context.Context, ObjectExecutionContext, int, map[string]any) (int, bool) {
			return 42, true
		}

		const total = 128
		const readers = 8

		errCh := make(chan error, total+readers)
		reportErr := func(err error) {
			select {
			case errCh <- err:
			default:
			}
		}

		start := make(chan struct{})
		var writersWG sync.WaitGroup
		var readersWG sync.WaitGroup
		var writesDone atomic.Bool

		for i := range total {
			writersWG.Add(1)
			go func(i int) {
				defer writersWG.Done()
				<-start
				RegisterComplexity("scope", "Query", fmt.Sprintf("complexity_%03d", i), h)
			}(i)
		}

		for range readers {
			readersWG.Go(func() {
				<-start
				for !writesDone.Load() {
					for i := range total {
						handler, ok := LookupComplexity(
							"scope",
							"Query",
							fmt.Sprintf("complexity_%03d", i),
						)
						if ok && handler == nil {
							reportErr(fmt.Errorf("nil complexity handler for registered key %d", i))
						}
					}
				}
			})
		}

		close(start)
		writersWG.Wait()
		writesDone.Store(true)
		readersWG.Wait()

		close(errCh)
		for err := range errCh {
			t.Fatal(err)
		}

		for i := range total {
			handler, ok := LookupComplexity("scope", "Query", fmt.Sprintf("complexity_%03d", i))
			if !ok || handler == nil {
				t.Fatalf("missing registered complexity handler for key %d", i)
			}
		}
	})

	t.Run("input unmarshaler", func(t *testing.T) {
		resetInputUnmarshalRegistryForTest()

		type marker struct{ id string }

		const total = 128
		const readers = 8

		errCh := make(chan error, total+readers)
		reportErr := func(err error) {
			select {
			case errCh <- err:
			default:
			}
		}

		start := make(chan struct{})
		var writersWG sync.WaitGroup
		var readersWG sync.WaitGroup
		var writesDone atomic.Bool

		for i := range total {
			writersWG.Add(1)
			go func(i int) {
				defer writersWG.Done()
				<-start
				RegisterInputUnmarshaler(
					"scope",
					fmt.Sprintf("Input_%03d", i),
					&marker{id: fmt.Sprintf("%03d", i)},
				)
			}(i)
		}

		for range readers {
			readersWG.Go(func() {
				<-start
				for !writesDone.Load() {
					unmarshalers := ListInputUnmarshalers("scope", nil)
					if len(unmarshalers) > total {
						reportErr(
							fmt.Errorf(
								"unexpected input unmarshaler count: got %d want <= %d",
								len(unmarshalers),
								total,
							),
						)
					}
					for i, unmarshaler := range unmarshalers {
						if unmarshaler == nil {
							reportErr(fmt.Errorf("nil input unmarshaler at index %d", i))
						}
					}
				}
			})
		}

		close(start)
		writersWG.Wait()
		writesDone.Store(true)
		readersWG.Wait()

		close(errCh)
		for err := range errCh {
			t.Fatal(err)
		}

		unmarshalers := ListInputUnmarshalers("scope", nil)
		if len(unmarshalers) != total {
			t.Fatalf(
				"unexpected number of registered input unmarshalers: got %d want %d",
				len(unmarshalers),
				total,
			)
		}
		for i, unmarshaler := range unmarshalers {
			if unmarshaler == nil {
				t.Fatalf("nil input unmarshaler at index %d after registration", i)
			}
		}
	})

	t.Run("codec marshal", func(t *testing.T) {
		resetCodecMarshalRegistryForTest()

		h := func(context.Context, ObjectExecutionContext, ast.SelectionSet, any) graphql.Marshaler {
			return graphql.Null
		}

		const total = 128
		const readers = 8

		errCh := make(chan error, total+readers)
		reportErr := func(err error) {
			select {
			case errCh <- err:
			default:
			}
		}

		start := make(chan struct{})
		var writersWG sync.WaitGroup
		var readersWG sync.WaitGroup
		var writesDone atomic.Bool

		for i := range total {
			writersWG.Add(1)
			go func(i int) {
				defer writersWG.Done()
				<-start
				RegisterCodecMarshal("scope", fmt.Sprintf("marshal_%03d", i), h)
			}(i)
		}

		for range readers {
			readersWG.Go(func() {
				<-start
				for !writesDone.Load() {
					for i := range total {
						handler, ok := LookupCodecMarshal("scope", fmt.Sprintf("marshal_%03d", i))
						if ok && handler == nil {
							reportErr(
								fmt.Errorf("nil codec marshal handler for registered key %d", i),
							)
						}
					}
				}
			})
		}

		close(start)
		writersWG.Wait()
		writesDone.Store(true)
		readersWG.Wait()

		close(errCh)
		for err := range errCh {
			t.Fatal(err)
		}

		for i := range total {
			handler, ok := LookupCodecMarshal("scope", fmt.Sprintf("marshal_%03d", i))
			if !ok || handler == nil {
				t.Fatalf("missing registered codec marshal handler for key %d", i)
			}
		}
	})

	t.Run("codec unmarshal", func(t *testing.T) {
		resetCodecUnmarshalRegistryForTest()

		h := func(context.Context, ObjectExecutionContext, any) (any, error) {
			return nil, nil
		}

		const total = 128
		const readers = 8

		errCh := make(chan error, total+readers)
		reportErr := func(err error) {
			select {
			case errCh <- err:
			default:
			}
		}

		start := make(chan struct{})
		var writersWG sync.WaitGroup
		var readersWG sync.WaitGroup
		var writesDone atomic.Bool

		for i := range total {
			writersWG.Add(1)
			go func(i int) {
				defer writersWG.Done()
				<-start
				RegisterCodecUnmarshal("scope", fmt.Sprintf("unmarshal_%03d", i), h)
			}(i)
		}

		for range readers {
			readersWG.Go(func() {
				<-start
				for !writesDone.Load() {
					for i := range total {
						handler, ok := LookupCodecUnmarshal(
							"scope",
							fmt.Sprintf("unmarshal_%03d", i),
						)
						if ok && handler == nil {
							reportErr(
								fmt.Errorf("nil codec unmarshal handler for registered key %d", i),
							)
						}
					}
				}
			})
		}

		close(start)
		writersWG.Wait()
		writesDone.Store(true)
		readersWG.Wait()

		close(errCh)
		for err := range errCh {
			t.Fatal(err)
		}

		for i := range total {
			handler, ok := LookupCodecUnmarshal("scope", fmt.Sprintf("unmarshal_%03d", i))
			if !ok || handler == nil {
				t.Fatalf("missing registered codec unmarshal handler for key %d", i)
			}
		}
	})
}

func assertDuplicateRegistrationPanic(t *testing.T, expected string, register func()) {
	t.Helper()

	defer func() {
		recovered := recover()
		if recovered == nil {
			t.Fatal("expected duplicate registration panic")
		}
		msg, ok := recovered.(string)
		if !ok {
			t.Fatalf("expected panic string, got %T", recovered)
		}
		if msg != expected {
			t.Fatalf("unexpected panic message: got %q want %q", msg, expected)
		}
	}()

	register()
}

func resetObjectRegistryForTest() {
	mu.Lock()
	defer mu.Unlock()

	objectByScope = map[string]map[string]ObjectHandler{}
	resetObjectLookupSnapshotForTest()
}

func resetStreamObjectRegistryForTest() {
	mu.Lock()
	defer mu.Unlock()

	streamByScope = map[string]map[string]StreamObjectHandler{}
	resetStreamObjectLookupSnapshotForTest()
}

func resetFieldRegistryForTest() {
	mu.Lock()
	defer mu.Unlock()

	fieldByScope = map[string]map[string]map[string]FieldHandler{}
	resetFieldLookupSnapshotForTest()
}

func resetStreamFieldRegistryForTest() {
	mu.Lock()
	defer mu.Unlock()

	streamFieldByScope = map[string]map[string]map[string]StreamFieldHandler{}
	resetStreamFieldLookupSnapshotForTest()
}

func resetComplexityRegistryForTest() {
	mu.Lock()
	defer mu.Unlock()

	complexityByScope = map[string]map[string]map[string]ComplexityHandler{}
	resetComplexityLookupSnapshotForTest()
}

func resetInputUnmarshalRegistryForTest() {
	mu.Lock()
	defer mu.Unlock()

	inputUnmarshalByScope = map[string]map[string]any{}
	resetInputUnmarshalLookupSnapshotForTest()
}

func resetCodecMarshalRegistryForTest() {
	mu.Lock()
	defer mu.Unlock()

	codecMarshalByScope = map[string]map[string]CodecMarshalHandler{}
	resetCodecMarshalLookupSnapshotForTest()
}

func resetCodecUnmarshalRegistryForTest() {
	mu.Lock()
	defer mu.Unlock()

	codecUnmarshalByScope = map[string]map[string]CodecUnmarshalHandler{}
	resetCodecUnmarshalLookupSnapshotForTest()
}

func resetFieldContextRegistryForTest() {
	mu.Lock()
	defer mu.Unlock()

	fieldContextByScope = map[string]map[string]FieldContextHandler{}
	resetFieldContextLookupSnapshotForTest()
}

func resetResolverInvokerRegistryForTest() {
	mu.Lock()
	defer mu.Unlock()

	resolverInvokerByScope = map[string]map[string]ResolverInvokerHandler{}
	resetResolverInvokerLookupSnapshotForTest()
}

func resetArgsRegistryForTest() {
	mu.Lock()
	defer mu.Unlock()

	argsByScope = map[string]map[string]ArgsHandler{}
	resetArgsLookupSnapshotForTest()
}

func TestFieldContextRegistry(t *testing.T) {
	resetFieldContextRegistryForTest()

	h := func(context.Context, ObjectExecutionContext, graphql.CollectedField) (*graphql.FieldContext, error) {
		return &graphql.FieldContext{}, nil
	}

	if got, ok := LookupFieldContext("scope", "Query", "name"); ok || got != nil {
		t.Fatalf("unexpected field context handler before registration: handler=%v ok=%v", got, ok)
	}

	RegisterFieldContext("scope", "Query", "name", h)

	got, ok := LookupFieldContext("scope", "Query", "name")
	if !ok {
		t.Fatal("expected registered field context handler")
	}
	if got == nil {
		t.Fatal("expected non-nil field context handler")
	}

	if got, ok := LookupFieldContext("scope", "Query", "missing"); ok || got != nil {
		t.Fatalf("unexpected field context handler for missing field: handler=%v ok=%v", got, ok)
	}
	if got, ok := LookupFieldContext("scope", "Mutation", "name"); ok || got != nil {
		t.Fatalf("unexpected field context handler for missing object: handler=%v ok=%v", got, ok)
	}
	if got, ok := LookupFieldContext("other-scope", "Query", "name"); ok || got != nil {
		t.Fatalf("unexpected field context handler for missing scope: handler=%v ok=%v", got, ok)
	}

	defer func() {
		recovered := recover()
		if recovered == nil {
			t.Fatal("expected duplicate registration panic")
		}
		msg, ok := recovered.(string)
		if !ok {
			t.Fatalf("expected panic string, got %T", recovered)
		}
		expected := "duplicate field context handler registration: scope:Query:name"
		if msg != expected {
			t.Fatalf("unexpected panic message: got %q want %q", msg, expected)
		}
	}()

	RegisterFieldContext("scope", "Query", "name", h)
}

func TestResolverInvokerRegistry(t *testing.T) {
	resetResolverInvokerRegistryForTest()

	h := func(context.Context, ObjectExecutionContext, any) (any, error) {
		return "resolved", nil
	}

	if got, ok := LookupResolverInvoker("scope", "Query", "name"); ok || got != nil {
		t.Fatalf(
			"unexpected resolver invoker handler before registration: handler=%v ok=%v",
			got,
			ok,
		)
	}

	RegisterResolverInvoker("scope", "Query", "name", h)

	got, ok := LookupResolverInvoker("scope", "Query", "name")
	if !ok {
		t.Fatal("expected registered resolver invoker handler")
	}
	if got == nil {
		t.Fatal("expected non-nil resolver invoker handler")
	}

	result, err := got(context.Background(), nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "resolved" {
		t.Fatalf("unexpected result: got %v want %q", result, "resolved")
	}

	if got, ok := LookupResolverInvoker("scope", "Query", "missing"); ok || got != nil {
		t.Fatalf("unexpected resolver invoker handler for missing field: handler=%v ok=%v", got, ok)
	}
	if got, ok := LookupResolverInvoker("scope", "Mutation", "name"); ok || got != nil {
		t.Fatalf(
			"unexpected resolver invoker handler for missing object: handler=%v ok=%v",
			got,
			ok,
		)
	}
	if got, ok := LookupResolverInvoker("other-scope", "Query", "name"); ok || got != nil {
		t.Fatalf("unexpected resolver invoker handler for missing scope: handler=%v ok=%v", got, ok)
	}

	defer func() {
		recovered := recover()
		if recovered == nil {
			t.Fatal("expected duplicate registration panic")
		}
		msg, ok := recovered.(string)
		if !ok {
			t.Fatalf("expected panic string, got %T", recovered)
		}
		expected := "duplicate resolver invoker handler registration: scope:Query:name"
		if msg != expected {
			t.Fatalf("unexpected panic message: got %q want %q", msg, expected)
		}
	}()

	RegisterResolverInvoker("scope", "Query", "name", h)
}

func TestArgsRegistry(t *testing.T) {
	resetArgsRegistryForTest()

	h := func(context.Context, ObjectExecutionContext, map[string]any) (map[string]any, error) {
		return map[string]any{"parsed": true}, nil
	}

	if got, ok := LookupArgs("scope", "Query.name"); ok || got != nil {
		t.Fatalf("unexpected args handler before registration: handler=%v ok=%v", got, ok)
	}

	RegisterArgs("scope", "Query.name", h)

	got, ok := LookupArgs("scope", "Query.name")
	if !ok {
		t.Fatal("expected registered args handler")
	}
	if got == nil {
		t.Fatal("expected non-nil args handler")
	}

	result, err := got(context.Background(), nil, map[string]any{"input": "value"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result["parsed"] != true {
		t.Fatalf("unexpected result: got %v", result)
	}

	if got, ok := LookupArgs("scope", "Query.missing"); ok || got != nil {
		t.Fatalf("unexpected args handler for missing key: handler=%v ok=%v", got, ok)
	}
	if got, ok := LookupArgs("other-scope", "Query.name"); ok || got != nil {
		t.Fatalf("unexpected args handler for missing scope: handler=%v ok=%v", got, ok)
	}

	defer func() {
		recovered := recover()
		if recovered == nil {
			t.Fatal("expected duplicate registration panic")
		}
		msg, ok := recovered.(string)
		if !ok {
			t.Fatalf("expected panic string, got %T", recovered)
		}
		expected := "duplicate args handler registration: scope:Query.name"
		if msg != expected {
			t.Fatalf("unexpected panic message: got %q want %q", msg, expected)
		}
	}()

	RegisterArgs("scope", "Query.name", h)
}

// fakeEC is a test-only implementation of ObjectExecutionContext.
type fakeEC struct {
	fieldContextHandlers map[string]FieldContextHandler
}

// fakeECWithOpCtx embeds fakeEC and overrides GetOperationContext to return a
// non-nil OperationContext with empty Variables, satisfying the args path in
// buildFieldContext which calls ec.GetOperationContext().Variables.
type fakeECWithOpCtx struct {
	fakeEC
}

func (f *fakeECWithOpCtx) GetOperationContext() *graphql.OperationContext {
	return &graphql.OperationContext{
		Variables: map[string]any{},
		ResolverMiddleware: func(ctx context.Context, next graphql.Resolver) (res any, err error) {
			return next(ctx)
		},
	}
}

func (f *fakeEC) GetOperationContext() *graphql.OperationContext { return nil }
func (f *fakeEC) MarshalCodec(context.Context, string, ast.SelectionSet, any) graphql.Marshaler {
	return graphql.Null
}
func (f *fakeEC) UnmarshalCodec(context.Context, string, any) (any, error) { return nil, nil }
func (f *fakeEC) ParseFieldArgs(context.Context, string, map[string]any) (map[string]any, error) {
	return nil, nil
}

func (f *fakeEC) ResolveField(
	context.Context,
	string,
	string,
	graphql.CollectedField,
	any,
) graphql.Marshaler {
	return graphql.Null
}

func (f *fakeEC) ResolveStreamField(
	context.Context,
	string,
	string,
	graphql.CollectedField,
	any,
) func(context.Context) graphql.Marshaler {
	return func(context.Context) graphql.Marshaler { return graphql.Null }
}

func (f *fakeEC) InvokeResolver(context.Context, string, string, any) (any, error) {
	return nil, nil
}

func (f *fakeEC) LookupFieldContextHandler(obj, field string) (FieldContextHandler, bool) {
	h, ok := f.fieldContextHandlers[obj+"."+field]
	return h, ok
}
func (f *fakeEC) ProcessDeferredGroup(graphql.DeferredGroup) {}
func (f *fakeEC) AddDeferred(int32)                          {}
func (f *fakeEC) Error(context.Context, error)               {}
func (f *fakeEC) Recover(_ context.Context, r any) error     { return fmt.Errorf("%v", r) }

func TestMakeChildResolver_Scalar(t *testing.T) {
	ec := &fakeEC{}
	lookup := &ObjectChildLookup{TypeName: "UUID", Kind: ast.Scalar}

	childFn := makeChildResolver(ec, lookup)
	_, err := childFn(context.Background(), graphql.CollectedField{})
	if err == nil {
		t.Fatal("expected error for scalar Child resolution")
	}
	if got := err.Error(); got != "field of type UUID does not have child fields" {
		t.Fatalf("unexpected error: got %q", got)
	}
}

func TestMakeChildResolver_NonObjectComposite(t *testing.T) {
	ec := &fakeEC{}
	lookup := &ObjectChildLookup{TypeName: "MyInput", Kind: ast.InputObject}

	childFn := makeChildResolver(ec, lookup)
	_, err := childFn(context.Background(), graphql.CollectedField{})
	if err == nil {
		t.Fatal("expected error for input-object Child resolution")
	}
	if got := err.Error(); got != "FieldContext.Child cannot be called on type INPUT_OBJECT" {
		t.Fatalf("unexpected error: got %q", got)
	}
}

func TestMakeChildResolver_ObjectKnownField(t *testing.T) {
	ec := &fakeEC{
		fieldContextHandlers: map[string]FieldContextHandler{
			"Escrow.id": func(_ context.Context, _ ObjectExecutionContext, _ graphql.CollectedField) (*graphql.FieldContext, error) {
				return &graphql.FieldContext{
					Object: "Escrow",
					Field:  graphql.CollectedField{Field: &ast.Field{Name: "id"}},
				}, nil
			},
		},
	}
	lookup := &ObjectChildLookup{
		TypeName: "Escrow",
		Kind:     ast.Object,
		Children: []string{"id", "address"},
	}

	childFn := makeChildResolver(ec, lookup)
	fc, err := childFn(context.Background(), graphql.CollectedField{Field: &ast.Field{Name: "id"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fc == nil || fc.Object != "Escrow" {
		t.Fatalf("unexpected FieldContext: %+v", fc)
	}
}

func TestMakeChildResolver_ObjectUnknownField(t *testing.T) {
	ec := &fakeEC{}
	lookup := &ObjectChildLookup{TypeName: "Escrow", Kind: ast.Object, Children: []string{"id"}}

	childFn := makeChildResolver(ec, lookup)
	_, err := childFn(
		context.Background(),
		graphql.CollectedField{Field: &ast.Field{Name: "nonexistent"}},
	)
	if err == nil {
		t.Fatal("expected error for unknown child field")
	}
	expected := `no field named "nonexistent" was found under type Escrow`
	if got := err.Error(); got != expected {
		t.Fatalf("unexpected error: got %q want %q", got, expected)
	}
}

func TestMakeChildResolver_NilRet(t *testing.T) {
	ec := &fakeEC{}

	childFn := makeChildResolver(ec, nil)
	_, err := childFn(context.Background(), graphql.CollectedField{})
	if err == nil {
		t.Fatal("expected error for nil return-type lookup")
	}
	if got := err.Error(); got != "no return type information for field" {
		t.Fatalf("unexpected error: got %q", got)
	}
}

func TestMakeChildResolver_ObjectHandlerMissing(t *testing.T) {
	ec := &fakeEC{}
	lookup := &ObjectChildLookup{TypeName: "Escrow", Kind: ast.Object, Children: []string{"id"}}

	childFn := makeChildResolver(ec, lookup)
	_, err := childFn(context.Background(), graphql.CollectedField{Field: &ast.Field{Name: "id"}})
	if err == nil {
		t.Fatal("expected error when no FieldContext handler is registered")
	}
	if got := err.Error(); got != "no field context handler for Escrow.id" {
		t.Fatalf("unexpected error: got %q", got)
	}
}

func TestRegisterFieldDef_BasicRegistration(t *testing.T) {
	resetFieldRegistryForTest()
	resetFieldContextRegistryForTest()

	def := FieldDef{
		Resolve: func(ctx context.Context, _ ObjectExecutionContext, _ uint16, obj any) (any, error) {
			return "value", nil
		},
		ReturnType:   &ObjectChildLookup{TypeName: "String", Kind: ast.Scalar},
		MarshalCodec: "marshalNString",
		NonNull:      true,
		PanicHandled: true,
	}

	RegisterFieldDef("scope-x", "MyObj", "myField", def)

	if _, ok := LookupField("scope-x", "MyObj", "myField"); !ok {
		t.Fatal("expected RegisterFieldDef to register a FieldHandler")
	}
	if _, ok := LookupFieldContext("scope-x", "MyObj", "myField"); !ok {
		t.Fatal("expected RegisterFieldDef to register a FieldContextHandler")
	}
}

func TestBuildFieldContext_NoArgs(t *testing.T) {
	resetFieldRegistryForTest()
	resetFieldContextRegistryForTest()
	resetArgsRegistryForTest()

	ec := &fakeEC{
		fieldContextHandlers: map[string]FieldContextHandler{},
	}
	def := &FieldDef{
		IsMethod:   true,
		IsResolver: false,
		ReturnType: &ObjectChildLookup{TypeName: "String", Kind: ast.Scalar},
	}
	cf := graphql.CollectedField{Field: &ast.Field{Name: "id"}}

	fc, err := buildFieldContext(context.Background(), ec, def, "scope", "Escrow", cf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fc.Object != "Escrow" {
		t.Fatalf("unexpected Object: got %q want Escrow", fc.Object)
	}
	if fc.IsMethod != true || fc.IsResolver != false {
		t.Fatalf("unexpected flags: IsMethod=%v IsResolver=%v", fc.IsMethod, fc.IsResolver)
	}
	if fc.Child == nil {
		t.Fatal("expected non-nil Child resolver")
	}
}

func TestBuildFieldContext_ArgsPath(t *testing.T) {
	resetFieldRegistryForTest()
	resetFieldContextRegistryForTest()
	resetArgsRegistryForTest()

	RegisterArgs(
		"scope",
		"EscrowQueryArgs",
		func(_ context.Context, _ ObjectExecutionContext, raw map[string]any) (map[string]any, error) {
			return map[string]any{"id": raw["id"]}, nil
		},
	)

	ec := &fakeECWithOpCtx{}
	def := &FieldDef{
		IsMethod: true,
		ReturnType: &ObjectChildLookup{
			TypeName: "Escrow",
			Kind:     ast.Object,
			Children: []string{"id"},
		},
		ArgsKey: "EscrowQueryArgs",
	}
	// Build a synthetic field with a Definition so ArgumentMap can walk the arg defs.
	cf := graphql.CollectedField{
		Field: &ast.Field{
			Name: "escrow",
			Arguments: ast.ArgumentList{
				{Name: "id", Value: &ast.Value{Raw: "abc", Kind: ast.StringValue}},
			},
			Definition: &ast.FieldDefinition{
				Name: "escrow",
				Arguments: ast.ArgumentDefinitionList{
					{Name: "id"},
				},
			},
		},
	}

	fc, err := buildFieldContext(context.Background(), ec, def, "scope", "Query", cf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fc.Args["id"] != "abc" {
		t.Fatalf("unexpected args: %#v", fc.Args)
	}
}

func TestBuildFieldContext_ArgsPath_MissingArgsHandler(t *testing.T) {
	resetFieldRegistryForTest()
	resetFieldContextRegistryForTest()
	resetArgsRegistryForTest()

	ec := &fakeECWithOpCtx{}
	def := &FieldDef{
		ReturnType: &ObjectChildLookup{
			TypeName: "Escrow",
			Kind:     ast.Object,
			Children: []string{"id"},
		},
		ArgsKey: "MissingHandler",
	}
	cf := graphql.CollectedField{Field: &ast.Field{Name: "escrow"}}

	_, err := buildFieldContext(context.Background(), ec, def, "scope", "Query", cf)
	if err == nil {
		t.Fatal("expected error for missing args handler")
	}
	if err.Error() != `no args handler for "MissingHandler"` {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestResolveFromDef_CallsResolve(t *testing.T) {
	resetFieldRegistryForTest()
	resetFieldContextRegistryForTest()
	resetArgsRegistryForTest()
	resetCodecMarshalRegistryForTest()

	called := false
	RegisterCodecMarshal(
		"scope",
		"marshalNString",
		func(_ context.Context, _ ObjectExecutionContext, _ ast.SelectionSet, v any) graphql.Marshaler {
			return graphql.MarshalString(v.(string))
		},
	)

	def := FieldDef{
		Resolve: func(_ context.Context, _ ObjectExecutionContext, _ uint16, obj any) (any, error) {
			called = true
			return obj.(string) + "_resolved", nil
		},
		ReturnType:   &ObjectChildLookup{TypeName: "String", Kind: ast.Scalar},
		MarshalCodec: "marshalNString",
		NonNull:      true,
		PanicHandled: true,
	}

	RegisterFieldDef("scope", "Query", "name", def)

	h, ok := LookupField("scope", "Query", "name")
	if !ok {
		t.Fatal("missing handler")
	}
	ec := &fakeECWithOpCtx{}
	cf := graphql.CollectedField{Field: &ast.Field{Name: "name"}}

	m := h(context.Background(), ec, cf, "hello")
	if m == graphql.Null {
		t.Fatal("expected non-null marshaler from resolveFromDef")
	}
	if !called {
		t.Fatal("Resolve was not invoked")
	}
}

// TestResolveFromDef_Adapter exercises the per-object adapter dispatch shape:
// a single shared Resolve func switches on fieldIdx, and two FieldDef values
// differing only by FieldIdx (0 and 1) route through it to distinct results.
// This is the data shape Tasks 2+ emit; here we hand-write the switch to verify
// resolveFromDef passes def.FieldIdx through to the adapter.
func TestResolveFromDef_Adapter(t *testing.T) {
	resetCodecMarshalRegistryForTest()

	// One shared adapter for the (hypothetical) object: idx 0 -> string field,
	// idx 1 -> int field. Mirrors the per-object adapter Task 2 will emit.
	adapter := func(_ context.Context, _ ObjectExecutionContext, fieldIdx uint16, obj any) (any, error) {
		switch fieldIdx {
		case 0:
			return "name-value", nil
		case 1:
			return 42, nil
		default:
			return nil, fmt.Errorf("unexpected fieldIdx %d", fieldIdx)
		}
	}

	ec := &fakeECWithOpCtx{}
	ctx := context.Background()

	marshalString := func(_ context.Context, _ ObjectExecutionContext, _ ast.SelectionSet, v any) graphql.Marshaler {
		return graphql.MarshalString(v.(string))
	}
	marshalInt := func(_ context.Context, _ ObjectExecutionContext, _ ast.SelectionSet, v any) graphql.Marshaler {
		return graphql.MarshalInt(v.(int))
	}

	cases := []struct {
		name      string
		fieldIdx  uint16
		fieldName string
		marshalFn CodecMarshalHandler
		want      string
	}{
		{name: "idx0 string field", fieldIdx: 0, fieldName: "name", marshalFn: marshalString, want: `"name-value"`},
		{name: "idx1 int field", fieldIdx: 1, fieldName: "age", marshalFn: marshalInt, want: `42`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			def := &FieldDef{
				Resolve:      adapter,
				FieldIdx:     tc.fieldIdx,
				ReturnType:   &ObjectChildLookup{TypeName: "String", Kind: ast.Scalar},
				MarshalCodec: "irrelevant",
				NonNull:      true,
				PanicHandled: true,
				// fakeEC.MarshalCodec returns graphql.Null, so observe the resolved
				// value through marshalFn (in-package access to the unexported field).
				marshalFn: tc.marshalFn,
			}
			cf := graphql.CollectedField{Field: &ast.Field{Name: tc.fieldName}}

			m := resolveFromDef(ctx, ec, def, "scope", "MyObj", cf, "ignored-obj")
			if m == graphql.Null {
				t.Fatalf("expected non-null marshaler for fieldIdx %d", tc.fieldIdx)
			}
			var buf bytes.Buffer
			m.MarshalGQL(&buf)
			if got := buf.String(); got != tc.want {
				t.Fatalf("fieldIdx %d: got %s want %s", tc.fieldIdx, got, tc.want)
			}
		})
	}
}

func TestBuildFieldContext_ArgsPath_PanicRecovered(t *testing.T) {
	resetFieldRegistryForTest()
	resetFieldContextRegistryForTest()
	resetArgsRegistryForTest()

	RegisterArgs(
		"scope",
		"PanickingArgs",
		func(_ context.Context, _ ObjectExecutionContext, _ map[string]any) (map[string]any, error) {
			panic("boom")
		},
	)

	ec := &fakeECWithOpCtx{}
	def := &FieldDef{
		ReturnType: &ObjectChildLookup{
			TypeName: "Escrow",
			Kind:     ast.Object,
			Children: []string{"id"},
		},
		ArgsKey: "PanickingArgs",
	}
	cf := graphql.CollectedField{Field: &ast.Field{Name: "escrow"}}

	_, err := buildFieldContext(context.Background(), ec, def, "scope", "Query", cf)
	if err == nil {
		t.Fatal("expected error from recovered panic")
	}
	if err.Error() != "boom" {
		t.Fatalf("expected recovered panic message, got %v", err)
	}
}

// --- ShardDescriptor / SchemaDescriptor aggregation + dispatch tests ---

// TestBuildSchema_CrossShardMerge proves BuildSchema merges field defs from
// multiple shards that each contribute to the SAME object, and that each
// shard's own FieldIdx + adapter is preserved on the merged def (the per-shard
// index contract). Shard A contributes Query.alpha (adapterA, FieldIdx 0);
// shard B contributes Query.zeta (adapterB, FieldIdx 0). Identical FieldIdx
// values from different shards must remain bound to their own adapter.
func TestBuildSchema_CrossShardMerge(t *testing.T) {
	adapterA := func(_ context.Context, _ ObjectExecutionContext, _ uint16, _ any) (any, error) {
		return "from-A", nil
	}
	adapterB := func(_ context.Context, _ ObjectExecutionContext, _ uint16, _ any) (any, error) {
		return "from-B", nil
	}

	shardA := ShardDescriptor{
		Scope: "scope",
		Objects: []ObjectFieldDefs{
			{
				Object: "Query",
				Fields: []NamedFieldDef{
					{Name: "alpha", Def: FieldDef{Resolve: adapterA, FieldIdx: 0}},
				},
			},
		},
	}
	shardB := ShardDescriptor{
		Scope: "scope",
		Objects: []ObjectFieldDefs{
			{
				Object: "Query",
				Fields: []NamedFieldDef{
					{Name: "zeta", Def: FieldDef{Resolve: adapterB, FieldIdx: 0}},
				},
			},
		},
	}

	s := BuildSchema(shardA, shardB)
	if s.scope != "scope" {
		t.Fatalf("unexpected scope: got %q want %q", s.scope, "scope")
	}

	defAlpha, ok := s.Field("Query", "alpha")
	if !ok {
		t.Fatal("expected Query.alpha after merge")
	}
	defZeta, ok := s.Field("Query", "zeta")
	if !ok {
		t.Fatal("expected Query.zeta after merge")
	}

	gotA, err := defAlpha.Resolve(context.Background(), nil, defAlpha.FieldIdx, nil)
	if err != nil {
		t.Fatalf("unexpected error from alpha adapter: %v", err)
	}
	if gotA != "from-A" {
		t.Fatalf("alpha routed to wrong adapter: got %v want %q", gotA, "from-A")
	}
	gotB, err := defZeta.Resolve(context.Background(), nil, defZeta.FieldIdx, nil)
	if err != nil {
		t.Fatalf("unexpected error from zeta adapter: %v", err)
	}
	if gotB != "from-B" {
		t.Fatalf("zeta routed to wrong adapter: got %v want %q", gotB, "from-B")
	}
}

// TestSchemaDescriptor_FieldBinarySearch verifies the binary-search lookup:
// the per-object names slice is sorted ascending, present field names resolve
// to the matching def, and absent names / objects return false.
func TestSchemaDescriptor_FieldBinarySearch(t *testing.T) {
	mk := func(name string, idx uint16) NamedFieldDef {
		return NamedFieldDef{
			Name: name,
			Def: FieldDef{
				FieldIdx: idx,
				Resolve: func(_ context.Context, _ ObjectExecutionContext, fieldIdx uint16, _ any) (any, error) {
					return fieldIdx, nil
				},
			},
		}
	}

	shard := ShardDescriptor{
		Scope: "scope",
		Objects: []ObjectFieldDefs{
			{
				Object: "Query",
				// Intentionally unsorted input to prove BuildSchema sorts.
				Fields: []NamedFieldDef{
					mk("delta", 3),
					mk("alpha", 0),
					mk("charlie", 2),
					mk("bravo", 1),
				},
			},
		},
	}

	s := BuildSchema(shard)

	idx := s.objects["Query"]
	if idx == nil {
		t.Fatal("expected objectFieldIndex for Query")
	}
	wantNames := []string{"alpha", "bravo", "charlie", "delta"}
	if len(idx.names) != len(wantNames) {
		t.Fatalf("unexpected names length: got %d want %d", len(idx.names), len(wantNames))
	}
	for i, n := range wantNames {
		if idx.names[i] != n {
			t.Fatalf("names not sorted at %d: got %q want %q (%v)", i, idx.names[i], n, idx.names)
		}
	}

	for wantIdx, name := range wantNames {
		def, ok := s.Field("Query", name)
		if !ok {
			t.Fatalf("expected to find Query.%s", name)
		}
		if int(def.FieldIdx) != wantIdx {
			t.Fatalf("Query.%s mapped to wrong def: FieldIdx got %d want %d", name, def.FieldIdx, wantIdx)
		}
	}

	if def, ok := s.Field("Query", "missing"); ok || def != nil {
		t.Fatalf("expected miss for absent field: def=%v ok=%v", def, ok)
	}
	if def, ok := s.Field("Mutation", "alpha"); ok || def != nil {
		t.Fatalf("expected miss for absent object: def=%v ok=%v", def, ok)
	}
}

// TestSchemaDescriptor_ResolveFieldDispatch exercises the exported ResolveField
// dispatch over a shared adapter that switches on fieldIdx, mirroring
// TestResolveFromDef_Adapter. The cached marshalFn (set via the codec registry
// before BuildSchema) renders the resolved value, so the produced JSON proves
// the right def (and thus the right FieldIdx) was dispatched. A non-tautology
// guard confirms a deliberately wrong FieldIdx would produce different output.
func TestSchemaDescriptor_ResolveFieldDispatch(t *testing.T) {
	resetCodecMarshalRegistryForTest()

	adapter := func(_ context.Context, _ ObjectExecutionContext, fieldIdx uint16, _ any) (any, error) {
		switch fieldIdx {
		case 0:
			return "name-value", nil
		case 1:
			return 42, nil
		default:
			return nil, fmt.Errorf("unexpected fieldIdx %d", fieldIdx)
		}
	}

	RegisterCodecMarshal(
		"scope",
		"marshalString",
		func(_ context.Context, _ ObjectExecutionContext, _ ast.SelectionSet, v any) graphql.Marshaler {
			return graphql.MarshalString(v.(string))
		},
	)
	RegisterCodecMarshal(
		"scope",
		"marshalInt",
		func(_ context.Context, _ ObjectExecutionContext, _ ast.SelectionSet, v any) graphql.Marshaler {
			return graphql.MarshalInt(v.(int))
		},
	)

	shard := ShardDescriptor{
		Scope: "scope",
		Objects: []ObjectFieldDefs{
			{
				Object: "MyObj",
				Fields: []NamedFieldDef{
					{Name: "name", Def: FieldDef{
						Resolve:      adapter,
						FieldIdx:     0,
						ReturnType:   &ObjectChildLookup{TypeName: "String", Kind: ast.Scalar},
						MarshalCodec: "marshalString",
						NonNull:      true,
						PanicHandled: true,
					}},
					{Name: "age", Def: FieldDef{
						Resolve:      adapter,
						FieldIdx:     1,
						ReturnType:   &ObjectChildLookup{TypeName: "Int", Kind: ast.Scalar},
						MarshalCodec: "marshalInt",
						NonNull:      true,
						PanicHandled: true,
					}},
				},
			},
		},
	}

	s := BuildSchema(shard)
	ec := &fakeECWithOpCtx{}
	ctx := context.Background()

	cases := []struct {
		fieldName string
		want      string
	}{
		{fieldName: "name", want: `"name-value"`},
		{fieldName: "age", want: `42`},
	}
	for _, tc := range cases {
		t.Run(tc.fieldName, func(t *testing.T) {
			cf := graphql.CollectedField{Field: &ast.Field{Name: tc.fieldName}}
			m, ok := s.ResolveField(ctx, ec, "MyObj", tc.fieldName, cf, "ignored-obj")
			if !ok {
				t.Fatalf("expected ResolveField hit for MyObj.%s", tc.fieldName)
			}
			if m == graphql.Null {
				t.Fatalf("expected non-null marshaler for MyObj.%s", tc.fieldName)
			}
			var buf bytes.Buffer
			m.MarshalGQL(&buf)
			if got := buf.String(); got != tc.want {
				t.Fatalf("MyObj.%s: got %s want %s", tc.fieldName, got, tc.want)
			}
		})
	}

	// Miss path.
	if m, ok := s.ResolveField(ctx, ec, "MyObj", "missing", graphql.CollectedField{Field: &ast.Field{Name: "missing"}}, nil); ok || m != nil {
		t.Fatalf("expected ResolveField miss for absent field: m=%v ok=%v", m, ok)
	}

	// Non-tautology guard: the "name" def routed to FieldIdx 1's marshaller
	// (int) over a string value would panic/produce different output, so the
	// fact that "name" rendered a quoted string confirms FieldIdx 0 dispatched.
	nameDef, _ := s.Field("MyObj", "name")
	if nameDef.FieldIdx != 0 {
		t.Fatalf("name def has wrong FieldIdx: got %d want 0", nameDef.FieldIdx)
	}
	ageDef, _ := s.Field("MyObj", "age")
	if ageDef.FieldIdx != 1 {
		t.Fatalf("age def has wrong FieldIdx: got %d want 1", ageDef.FieldIdx)
	}
}

// TestBuildSchema_MarshalFnCaching asserts BuildSchema replicates
// RegisterFieldDef's marshalFn caching: a codec registered BEFORE BuildSchema
// is cached on the stored def copy (observable because fakeEC.MarshalCodec
// returns graphql.Null, so a non-null marshal proves the cached fn ran). An
// unregistered codec leaves marshalFn nil (falls back to ec.MarshalCodec).
func TestBuildSchema_MarshalFnCaching(t *testing.T) {
	resetCodecMarshalRegistryForTest()

	RegisterCodecMarshal(
		"scope",
		"marshalCached",
		func(_ context.Context, _ ObjectExecutionContext, _ ast.SelectionSet, v any) graphql.Marshaler {
			return graphql.MarshalString("cached:" + v.(string))
		},
	)

	shard := ShardDescriptor{
		Scope: "scope",
		Objects: []ObjectFieldDefs{
			{
				Object: "MyObj",
				Fields: []NamedFieldDef{
					{Name: "cached", Def: FieldDef{
						Resolve: func(_ context.Context, _ ObjectExecutionContext, _ uint16, _ any) (any, error) {
							return "v", nil
						},
						ReturnType:   &ObjectChildLookup{TypeName: "String", Kind: ast.Scalar},
						MarshalCodec: "marshalCached",
						NonNull:      true,
						PanicHandled: true,
					}},
					{Name: "uncached", Def: FieldDef{
						Resolve: func(_ context.Context, _ ObjectExecutionContext, _ uint16, _ any) (any, error) {
							return "v", nil
						},
						ReturnType:   &ObjectChildLookup{TypeName: "String", Kind: ast.Scalar},
						MarshalCodec: "marshalNotRegistered",
						NonNull:      true,
						PanicHandled: true,
					}},
				},
			},
		},
	}

	s := BuildSchema(shard)

	cachedDef, ok := s.Field("MyObj", "cached")
	if !ok {
		t.Fatal("expected MyObj.cached")
	}
	if cachedDef.marshalFn == nil {
		t.Fatal("expected cached marshalFn for registered codec")
	}

	uncachedDef, ok := s.Field("MyObj", "uncached")
	if !ok {
		t.Fatal("expected MyObj.uncached")
	}
	if uncachedDef.marshalFn != nil {
		t.Fatal("expected nil marshalFn for unregistered codec (falls back to ec.MarshalCodec)")
	}

	// Observable: cached path renders through the cached fn (non-null), while
	// uncached path falls back to fakeEC.MarshalCodec which returns graphql.Null.
	ec := &fakeECWithOpCtx{}
	ctx := context.Background()

	cf := graphql.CollectedField{Field: &ast.Field{Name: "cached"}}
	m, ok := s.ResolveField(ctx, ec, "MyObj", "cached", cf, nil)
	if !ok {
		t.Fatal("expected ResolveField hit for cached")
	}
	var buf bytes.Buffer
	m.MarshalGQL(&buf)
	if got := buf.String(); got != `"cached:v"` {
		t.Fatalf("cached marshal output: got %s want %q", got, `"cached:v"`)
	}
}

// TestSchemaDescriptor_FieldContextHandler verifies the exported
// FieldContextHandler returns a closure that builds a *graphql.FieldContext
// via buildFieldContext with the correct Object and IsMethod/IsResolver flags
// from the FieldDef. Mirrors TestBuildFieldContext_NoArgs.
func TestSchemaDescriptor_FieldContextHandler(t *testing.T) {
	shard := ShardDescriptor{
		Scope: "scope",
		Objects: []ObjectFieldDefs{
			{
				Object: "Escrow",
				Fields: []NamedFieldDef{
					{Name: "id", Def: FieldDef{
						IsMethod:   true,
						IsResolver: false,
						ReturnType: &ObjectChildLookup{TypeName: "String", Kind: ast.Scalar},
					}},
				},
			},
		},
	}

	s := BuildSchema(shard)

	handler, ok := s.FieldContextHandler("Escrow", "id")
	if !ok {
		t.Fatal("expected FieldContextHandler for Escrow.id")
	}
	if handler == nil {
		t.Fatal("expected non-nil FieldContextHandler")
	}

	ec := &fakeEC{fieldContextHandlers: map[string]FieldContextHandler{}}
	cf := graphql.CollectedField{Field: &ast.Field{Name: "id"}}
	fc, err := handler(context.Background(), ec, cf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fc.Object != "Escrow" {
		t.Fatalf("unexpected Object: got %q want Escrow", fc.Object)
	}
	if fc.IsMethod != true || fc.IsResolver != false {
		t.Fatalf("unexpected flags: IsMethod=%v IsResolver=%v", fc.IsMethod, fc.IsResolver)
	}
	if fc.Child == nil {
		t.Fatal("expected non-nil Child resolver")
	}

	if h, ok := s.FieldContextHandler("Escrow", "missing"); ok || h != nil {
		t.Fatalf("expected miss for absent field context handler: h=%v ok=%v", h, ok)
	}
}

func TestRegisterStreamFieldDef(t *testing.T) {
	resetFieldRegistryForTest()
	resetStreamFieldRegistryForTest()
	resetFieldContextRegistryForTest()
	resetCodecMarshalRegistryForTest()

	RegisterCodecMarshal(
		"scope",
		"marshalNChat",
		func(_ context.Context, _ ObjectExecutionContext, _ ast.SelectionSet, _ any) graphql.Marshaler {
			return graphql.MarshalString("chat")
		},
	)

	def := StreamFieldDef{
		Resolve: func(_ context.Context, _ ObjectExecutionContext, _ any) (any, error) {
			return make(<-chan string), nil
		},
		ReturnType: &ObjectChildLookup{
			TypeName: "Chat",
			Kind:     ast.Object,
			Children: []string{"id"},
		},
		MarshalCodec: "marshalNChat",
		NonNull:      true,
		PanicHandled: true,
	}
	RegisterStreamFieldDef("scope", "Subscription", "chat", def)

	if _, ok := LookupStreamField("scope", "Subscription", "chat"); !ok {
		t.Fatal("expected stream field handler registered")
	}
	if _, ok := LookupFieldContext("scope", "Subscription", "chat"); !ok {
		t.Fatal("expected field context handler registered (shared with non-streaming)")
	}
}
