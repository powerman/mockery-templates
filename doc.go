// Package mockerytemplates provides templates for [mockery],
// which generate code from Go interfaces.
// Each template lives in a directory which is also a Go package documenting the generated API:
//
//   - [github.com/powerman/mockery-templates/fake]:
//     stateful fakes with an own evolving API derived from go-mockgen (recommended).
//   - [github.com/powerman/mockery-templates/go-mockgen]:
//     stateful fakes compatible with go-mockgen v2.1.1
//     (frozen, for compatibility and migration).
//
// [mockery]: https://github.com/vektra/mockery
package mockerytemplates
