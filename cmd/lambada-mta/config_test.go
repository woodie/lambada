package main

import (
	"testing"

	. "github.com/woodie/expect"
)

// Allow all tests in this package to use lowercase expect()
func expect[T any](got T, t testing.TB) Expectation[T] { return Expect(got, t) }

// Improve readability with vars set for structural functions and lifecycle hooks
/*
func TestCalculator(t *testing.T) {
	spec.Run(t, "Calculator", func(t *testing.T, describe spec.G, it spec.S) {

		context, before, after := describe, it.Before, it.After // HERE

		var calculator *Calculator
		before(func() { calculator = NewCalculator() })
		after(func() { calculator.Clear() })

		describe("#add", func() {
			context("with positive numbers", func() {
				it("returns the correct sum", func() {
					expect(calculator.Add(2, 3), t).To(Equal(5))
				})
			})
		})
	})
} */
