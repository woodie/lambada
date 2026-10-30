# Why the tests use `justBeforeEach`

lambada's tests follow one pattern. Each `context` sets up one scenario in a
`beforeEach`, and the function under test runs once, in a `justBeforeEach`
on the parent `describe`. The `it` blocks only check the result.

```go
describe("processAttachments", func() {
	var raw string
	var err error

	justBeforeEach(func() {
		msg, _ := mail.ReadMessage(strings.NewReader(raw))
		err = processAttachments(msg)
	})

	context("when the message is not multipart", func() {
		beforeEach(func() { raw = plainMessage })

		it("returns no error", func() { expect(err, t).To(Succeed()) })
	})

	context("when the message has an attachment", func() {
		beforeEach(func() { raw = multipartMessage })

		it("saves one file", func() { /* ... */ })
	})
})
```

## Why `beforeEach` isn't enough

Hooks run from the outside in. A `beforeEach` on the `describe` runs before
the `beforeEach` in each nested `context`. If the call to
`processAttachments` were in a `beforeEach`, it would run before `raw` was
set, so every context would test an empty message.

`justBeforeEach` runs after every `beforeEach` at every level, right before
the `it`. The scenario is fully set up by the time the function runs.

Swapping any of the suite's four `justBeforeEach` calls for `beforeEach`
fails tests:

| `describe`           | Fails with `beforeEach` because                   |
|----------------------|---------------------------------------------------|
| `checkAttachmentDir` | it runs before the context picks the path         |
| `cleanupOldFiles`    | it runs before the context ages the files         |
| `processAttachments` | it runs before the context picks the message      |
| `saveAttachment`     | it runs before the context picks the path         |

## Why not call the function in each context instead

That works too, but repeats the call in every context. With `justBeforeEach`:

- The call is written once, so a change to its arguments is one edit.
- A new case is a new `context` with one `beforeEach` that says what's
  different. It can't forget to make the call.
- Each `it` reads as a plain statement of what's true in that scenario.

## Where it comes from

`justBeforeEach` is Ginkgo's `JustBeforeEach`. Go's
[`woodie/spec`](https://github.com/woodie/spec) adds it as
`it.JustBeforeEach`, and the suite aliases it on its first line:

```go
describe, beforeEach, justBeforeEach := context, it.BeforeEach, it.JustBeforeEach
```

The scan page's tests (`spec/javascript`) use the same pattern, with
`justBeforeEach` from [`penultimate`](https://www.npmjs.com/package/penultimate).
