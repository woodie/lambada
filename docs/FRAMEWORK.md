# A Sinatra/Flask-style framework for Go?

`lambada-web` keeps coming up short against `scandalous/web.rb`'s Sinatra
readability -- see [issue #17](https://github.com/woodie/lambada/issues/17)
and the "This session: html-validate, then closing out issue #17 without a
new framework" entry in `docs/COWORK.md` for how that comparison started.
This doc is the living answer to "so should we actually build a framework,"
kept separate from `docs/COWORK.md`'s session-by-session journal since it's
a standing design question, not a one-time decision.

## Existing Go web frameworks

None of these are hypothetical -- they're mature, widely used, and would
solve real parts of the gap. Worth naming clearly what each one provides,
since "just use an existing framework" is the obvious first question before
inventing anything new.

| Framework | Static files | Templates | Context object | Built on |
|---|---|---|---|---|
| [Gin](https://gin-gonic.com/) | `router.Static("/assets", "./public")` -- one line | Built-in (`c.HTML`) | `*gin.Context` | `net/http` |
| [Echo](https://echo.labstack.com/) | `e.Static("/assets", "public")` -- one line | Built-in renderer interface | `echo.Context` | `net/http` |
| [Fiber](https://gofiber.io/) | `app.Static("/", "./public")` -- one line | Built-in | `*fiber.Ctx` | `fasthttp`, not `net/http` |
| [chi](https://github.com/go-chi/chi) | Not built in -- you wrap `http.FileServer` yourself | Not built in | Plain `http.Handler`, just adds routing/middleware | `net/http` |
| Beego / Buffalo | Yes, plus ORM, asset pipeline, generators | Yes | Yes | `net/http` |

Gin, Echo, and Fiber all genuinely give you Sinatra's "static files just
work" one-liner and a single context object with `.Param()`/`.JSON()`/
`.Render()` methods -- proof that Go's type system has no problem
expressing something close to Sinatra's ergonomics; see the "Why doesn't Go
have Sinatra" discussion logged below for why the *gap that remains* isn't
a compiler limitation. chi deliberately stays a thin router on top of plain
`http.Handler` and doesn't try to be Sinatra-like at all. Beego and Buffalo
go the other direction -- full Rails-style stacks with ORMs and generators,
several sizes heavier than anything a homelab scan server needs.

Fiber's `fasthttp` foundation is a real practical cost, not just a footnote:
it doesn't implement `http.Handler`, so none of the stdlib middleware,
`httptest`-based tests, or general `net/http` tooling this repo already
uses would carry over untouched. Gin and Echo, by contrast, both sit on top
of `net/http` -- a migration would touch every handler's signature, but
`httptest.NewRequest`/`ResponseRecorder`-based tests (what `main_test.go`
already uses) would keep working the same way.

## Why lambada doesn't use one of these (yet)

`lambada-web` has four routes and two static assets. Adopting Gin or Echo
is a real dependency and a real migration -- every handler's signature
changes from `func(w http.ResponseWriter, r *http.Request)` to
`func(c *gin.Context)`, and every existing `httptest`-based assertion in
`main_test.go` would need rewriting around the framework's own test
helpers. That's a lot of change to buy back four `mux.HandleFunc` lines and
two lines of static-file config -- especially once `http.FileServerFS`
(stdlib, Go 1.22+) closes the static-file gap for free, which it now does
(see `docs/COWORK.md`).

There's also a more specific gap none of these frameworks actually close.
Their `Static()` helpers serve *a whole directory* of files safely --
that's a solved problem, and stdlib's `FileServerFS` solves it the same
way. But `/download/{filename}` and the `DELETE` counterpart aren't
"serve a directory" -- they take an untrusted filename from the URL,
resolve it against `scanDir`, and then do something custom (attach as a
download, or `os.Remove` it). None of Gin/Echo/Fiber/chi ship a
general-purpose "validate this untrusted filename against a base directory
before I do my own thing with it" primitive, because that's inherently
tied to what the app does next -- it's not a router or template-rendering
concern. `scanFilePath` (see below) is genuinely bespoke to this app today,
not a gap any existing framework was leaving on the table.

This matches the account's existing posture on generic-sounding
abstractions ahead of a second real need -- see issue #6's nginx discussion
and issue #17 itself. `lambada-web` is still the only Go HTTP service in
this account.

## What we're doing instead: de-duplication with stdlib

Two concrete fixes landed this session without a new dependency:

- `http.FileServerFS` replaced `handleStyle`/`handleScript` (one handler
  per static file) with one `//go:embed static` + `mux.Handle("GET /",
  http.FileServerFS(sub))`.
- `scanListingOrFail` and `scanFilePath` replaced two blocks of
  copy-pasted glue (an identical "fetch the listing or 500" block in
  `handleIndex`/`handleScansJSON`, and an identical "sanitize the
  filename, stat the file, 404 if missing" block in
  `handleDownload`/`handleDelete`). Both helpers take `w` and write their
  own failure response, so every handler now reduces to the same
  `if !ok { return }` shape after calling one.

This is the same principle Sinatra's `web.rb` follows by keeping
`ScanFiles`/`Humane` out of the route blocks entirely -- push work into a
named, callable unit, keep the handler itself as thin glue. Go doesn't
need a framework to do that; it needs ordinary function extraction. See
`docs/COWORK.md`'s "html-validate, then closing out issue #17" entry for
the full before/after.

## The open idea: lambada as a testbed for a real micro-framework

`lambada` being "absurdly simple" is an advantage here, not just a
limitation -- it's small enough to prototype against without much at
stake, and it's already hit two concrete, repeatable patterns (static
serving, untrusted-filename resolution) that a future second Go service in
this account would almost certainly hit again.

If a framework gets built, its pitch shouldn't be "yet another router" --
Gin/Echo/chi already do that well, and duplicating their routing sugar
buys nothing. The actual differentiator worth building would be:

1. **One-line static file configuration** -- `app.Static("/", assetsFS)`,
   matching what Gin/Echo/Fiber already offer, built on `http.FileServerFS`
   under the hood.
2. **A tested, reusable "resolve an untrusted filename against a base
   directory" primitive** -- something like `app.SafeJoin(baseDir,
   untrustedName string) (path string, ok bool)`. This is the piece no
   existing framework ships, because it's specific to "small apps that
   hand out files by name," not general HTTP routing. If it lived in a
   framework with its own test suite, every consuming app would inherit
   tested traversal defense instead of hand-rolling and re-testing
   `scanFilePath`'s logic from scratch. A framework's test suite for this
   one primitive would need to cover, once, for every consumer: `../`
   traversal, absolute paths, embedded path separators, empty/`.`/`..`
   names, and symlink escapes -- exactly the cases `scanFilePath` handles
   today, just verified in one place instead of reinvented per app.

Both of these keep the framework's actual surface area small and grounded
in patterns that have already shown up twice (`static/` serving, plus
whatever a second Go service's equivalent of `/download/{filename}` turns
out to be) rather than speculative routing/middleware/rendering machinery
copied from Gin's feature list.

## Open questions

1. Is a second real Go HTTP service actually likely, or is this still a
   single-consumer problem? The framework case gets much stronger once
   there's a second app to prove `SafeJoin`/`Static` against.
2. If we prototype this against `lambada-web` now (before a second
   consumer exists), do we accept it stays a one-consumer package for a
   while, the same tradeoff issue #17 originally flagged for a much
   smaller local helper -- just deliberately taken this time, with eyes
   open?
3. Name and repo, if and when this moves from discussion to code -- no
   decision made here.

Nothing above commits to building this. It's the standing frame for
picking the conversation back up, so the next session doesn't have to
re-derive "why not just use Gin" or "what would actually be worth
building" from scratch.
