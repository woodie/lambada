package main

import (
	"testing"

	. "github.com/woodie/expect"
)

// Allow all tests in this package to use lowercase expect()
func expect[T any](got T, t testing.TB) Expectation[T] { return Expect(got, t) }

// Improve readability with structural functions and lifecycle hooks.
// Parameter is named context spec.G (not describe) -- only add
// `describe := context` if a file actually calls describe(...), which
// most files in this package do (main_test.go, scanfiles_test.go,
// server_test.go, middleware_test.go all group by feature/function).
// it's hook methods are called qualified: it.BeforeEach/it.AfterEach/it.JustBeforeEach.
// https://gist.github.com/woodie/35ee3fc2bea01b775b95b3fe5e950a05#file-example-go-L3
