package gomockgen_test

import (
	"context"
	"errors"
	"fmt"

	gomockgen "github.com/powerman/mockery-templates/go-mockgen"
)

var (
	errNotFound = errors.New("not found")
	errDiskFull = errors.New("disk full")
)

// memStore is a real in-memory implementation of gomockgen.Store.
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

func ExampleNewMockStore() {
	ctx := context.Background()
	store := gomockgen.NewMockStore()
	store.GetFunc.SetDefaultReturn("default", nil)

	fmt.Println(store.Get(ctx, "key"))
	fmt.Println(store.Set(ctx, "key", "value")) // Not configured: returns zero values.
	// Output:
	// default <nil>
	// <nil>
}

func ExampleNewMockStoreFrom() {
	ctx := context.Background()
	store := gomockgen.NewMockStoreFrom(make(memStore))
	store.SetFunc.PushReturn(errDiskFull) // Only the next call fails.

	fmt.Println(store.Set(ctx, "key", "one"))
	fmt.Println(store.Set(ctx, "key", "two"))
	fmt.Println(store.Get(ctx, "key"))
	// Output:
	// disk full
	// <nil>
	// two <nil>
}

func ExampleStoreGetFunc_PushReturn() {
	ctx := context.Background()
	store := gomockgen.NewStrictMockStore()
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

func ExampleStoreGetFunc_History() {
	ctx := context.Background()
	store := gomockgen.NewMockStoreFrom(memStore{"a": "1"})
	_, _ = store.Get(ctx, "a")
	_, _ = store.Get(ctx, "b")

	for _, call := range store.GetFunc.History() {
		fmt.Printf("Get(%q) = %q, %v\n", call.Arg1, call.Result0, call.Result1)
	}
	// Output:
	// Get("a") = "1", <nil>
	// Get("b") = "", not found
}
