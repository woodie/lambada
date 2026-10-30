package main

import (
	"testing"

	. "github.com/woodie/expect"
)

// Allow all tests in this package to use lowercase expect()
func expect[T any](got T, t testing.TB) Expectation[T] { return Expect(got, t) }

// Improve readability with structural functions and lifecycle hooks.
// Right below spec.Run, alias only what the suite calls:
// describe, beforeEach, justBeforeEach := context, it.BeforeEach, it.JustBeforeEach
// https://gist.github.com/woodie/35ee3fc2bea01b775b95b3fe5e950a05#file-example-go-L3
