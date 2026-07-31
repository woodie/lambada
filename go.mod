module lambada

go 1.26.3

require (
	github.com/emersion/go-smtp v0.24.0
	github.com/sclevine/spec v1.4.0
	github.com/woodie/expect v0.3.0
	github.com/woodie/humane v0.9.4
)

require github.com/emersion/go-sasl v0.0.0-20241020182733-b788ff22d5a6 // indirect

// woodie/spec is a fork of sclevine/spec adding BeforeEach/AfterEach/
// JustBeforeEach (Before/After deprecated, not removed). Module path is
// unchanged from upstream, so this is a replace, not a version bump.
replace github.com/sclevine/spec => github.com/woodie/spec v0.2.0
