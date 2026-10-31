package fake_test

import (
	"context"
	"errors"
	"fmt"

	"github.com/powerman/mockery-templates/fake"
)

var (
	errNotFound = errors.New("not found")
	errDiskFull = errors.New("disk full")
)

// memStore is a real in-memory implementation of fake.Store.
type memStore map[string]string

func (s memStore) Get(_ context.Context, key string) (string, error) {
	value, ok := s[key]
	if !ok {
		return "", errNotFound
	}
	return value, nil
}

func (s memStore) Set(_ context.Context, key, value string) error {
	s[key] = value
	return nil
}

func ExampleFakeStore() {
	ctx := context.Background()
	var store fake.FakeStore // The zero value works like NewFakeStore().
	store.SetFunc.SetDefaultReturn(errDiskFull)

	fmt.Println(store.Set(ctx, "key", "value"))
	// Output:
	// disk full
}

func ExampleNewFakeStore() {
	ctx := context.Background()
	store := fake.NewFakeStore()
	store.GetFunc.SetDefaultReturn("default", nil)

	fmt.Println(store.Get(ctx, "key"))
	// Output:
	// default <nil>
}

func ExampleNewFakeStore_unexpectedCall() {
	ctx := context.Background()
	store := fake.NewFakeStore()
	store.GetFunc.PushReturn("once", nil)

	fmt.Println(store.Get(ctx, "key"))

	defer func() { fmt.Println("panic:", recover()) }()
	fmt.Println(store.Get(ctx, "key")) // The queue is drained and there is no default hook.
	// Output:
	// once <nil>
	// panic: unexpected invocation of FakeStore.Get
}

func ExampleNewFakeStoreFrom() {
	ctx := context.Background()
	store := fake.NewFakeStoreFrom(make(memStore))
	store.SetFunc.PushReturn(errDiskFull) // Only the next call fails.

	fmt.Println(store.Set(ctx, "key", "one"))
	fmt.Println(store.Set(ctx, "key", "two"))
	fmt.Println(store.Get(ctx, "key"))
	// Output:
	// disk full
	// <nil>
	// two <nil>
}

func ExampleFakeStoreGetFunc_PushReturn() {
	ctx := context.Background()
	store := fake.NewFakeStore()
	store.GetFunc.SetDefaultReturn("", errNotFound)
	store.GetFunc.PushReturn("first", nil)
	store.GetFunc.PushHook(func(_ context.Context, key string) (string, error) {
		return "second for " + key, nil
	})

	for range 4 {
		value, err := store.Get(ctx, "key")
		fmt.Printf("%q %v\n", value, err)
	}
	// Output:
	// "first" <nil>
	// "second for key" <nil>
	// "" not found
	// "" not found
}

func ExampleFakeStoreGetFunc_History() {
	ctx := context.Background()
	store := fake.NewFakeStoreFrom(memStore{"a": "1"})
	_, _ = store.Get(ctx, "a")
	_, _ = store.Get(ctx, "b")

	for _, call := range store.GetFunc.History() {
		fmt.Printf("Get(%q) = %q, %v\n", call.Key, call.S, call.Err)
	}
	// Output:
	// Get("a") = "1", <nil>
	// Get("b") = "", not found
}
