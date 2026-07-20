package main

import (
	"testing"

	. "github.com/woodie/expect"
)

// expect is the lowercase call-site alias recommended in expect's own
// README ("Lowercase call sites") -- a one-line generic pass-through
// declared once per test package, since Go's capitalize-to-export rule
// only applies across the package boundary. Keeps every call site here
// reading lowercase alongside describe/context/it/before/after instead of
// standing out as the one capitalized word in the block, with zero loss of
// compile-time type inference. Shared by every _test.go file in this
// package (main_test.go, middleware_test.go, server_test.go,
// scanfiles_test.go).
func expect[T any](got T, t testing.TB) Expectation[T] { return Expect(got, t) }
