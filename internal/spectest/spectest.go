// Package spectest is lambada's spec_helper equivalent: every test file gets describe/context/it/before/after as plain parameters, no per-file alias line at all.
package spectest

import (
	"testing"

	"github.com/sclevine/spec"
)

// Run wraps spec.Run, pre-binding before/after/context via spec.Aliases.
func Run(t *testing.T, name string, f func(t *testing.T, describe, context spec.Describe, it spec.S, before, after func(func()))) {
	t.Helper()
	spec.Run(t, name, func(t *testing.T, describe spec.Describe, it spec.S) {
		before, after, context := spec.Aliases(describe, it)
		f(t, describe, context, it, before, after)
	})
}
