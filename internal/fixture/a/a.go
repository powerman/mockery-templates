// Package a provides a type whose name clashes with a type in package b.
package a

// Thing has the same name as b.Thing.
type Thing struct{}
