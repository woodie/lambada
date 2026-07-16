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

## Why prior Sinatra/Flask-for-Go attempts didn't stick

Before naming or building anything, worth being honest that "Sinatra/Flask
for Go" isn't a new idea -- it's been tried several times, and every
attempt either stalled or got absorbed into something else. Two distinct
failure modes show up, and they matter for what a new attempt should (and
shouldn't) be.

**Magic-heavy frameworks lost to the Go community's own culture, not to a
missing feature.** [Martini](https://github.com/go-martini/martini) --
explicitly "inspired by express and sinatra" -- used reflection-based
dependency injection to wire handler arguments together. The criticism
that killed it was specific: that reflection "moved important behavior out
of sight, making control flow hard to trace when something misbehaved."
That's the same argument, from the other direction, as "why doesn't Go
have Sinatra's `instance_eval` trick" above -- Go's static-typing culture
rejects implicit, receiver-swapping, reflection-driven magic, and this
wasn't just a theoretical preference: the market acted on it. Martini's own
creator reacted to the criticism by building
[Negroni](https://github.com/urfave/negroni) (same idea, no reflection),
and then [Gin](https://gin-gonic.com/) ate the remaining space entirely by
offering "Martini's API, 30-40x faster, explicit `*gin.Context` instead of
reflection." [Beego](https://github.com/beego/beego),
[Buffalo](https://github.com/gobuffalo/buffalo), and
[Revel](https://revel.github.io/) failed the same way at a larger scale --
full Rails-style stacks (ORM, generators, asset pipelines,
convention-over-configuration) that the Go community's preference for
small composable pieces over one big opinionated framework never really
embraced. All three show real, visible community decline through
2025-2026.

**Thin, non-magical routers (the ones actually closest to what "bolero"
would be) didn't fail from a flaw -- they solved a problem that stopped
existing.** [Pat](https://github.com/bmizerany/pat) (built by Blake
Mizerany, one of Sinatra's actual co-creators), [Traffic](https://github.com/pilu/traffic),
and [web.go](https://github.com/hoisie/web) all existed to give Go's old
`http.ServeMux` the expressive route-pattern matching it never had (named
params, method dispatch) -- a real gap when they were built.
[Goji](https://github.com/zenazn/goji) pitched itself explicitly as "of
the Sinatra and Flask school of web framework design," and even it split
into two incompatible major versions and stalled. Go 1.22 closed the
underlying gap directly in the standard library: `http.ServeMux` now does
method + wildcard pattern matching natively. That's the exact same
mechanism, not a coincidental parallel, that let this session delete
`handleStyle`/`handleScript` via `http.FileServerFS` -- stdlib keeps
absorbing exactly the kind of gap these frameworks existed to paper over,
and Gin/Echo/chi already won whatever mindshare was left over once that
happened.

**The takeaway for anything new in this space:** a pitch of "a nicer
router" is fighting a trend that's actively erasing that category, on top
of a namespace that's already crowded and mostly abandoned. `SafeJoin`
(the "resolve an untrusted filename against a base directory" primitive
from below) doesn't have this problem -- stdlib has no reason to ever grow
a general version of it, because it's shaped by what a specific app does
next, not a generic HTTP routing or file-serving concern the way
`http.ServeMux`/`http.FileServerFS` are. That's a structurally different
bet than Pat, Traffic, Goji, or Martini made, and it's the one place a
small, honestly-scoped library wouldn't be racing the standard library to
a fight it's already been losing for a decade.

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

## Closing the gap further: inline route closures instead of named handlers

A later session pushed the same de-duplication principle one step further,
prompted by comparing `main.go` directly against `web.rb` line for line:
`web.rb`'s `get "/" do ... end` blocks put a route's logic right at its
declaration, with a one-line comment above each (`# Route to list all
available files`) -- `main.go`'s named handler functions
(`handleIndex`/`handleScansJSON`/`handleDownload`/`handleDelete`), by
contrast, separated a route's *registration* (in `newMux()`) from its
*logic* (the function body, defined elsewhere in the file), which is
exactly the indirection Sinatra's block syntax avoids.

`handleIndex`/`handleScansJSON`/`handleDownload`/`handleDelete` are gone
now. Each one is an anonymous `func(w http.ResponseWriter, r *http.Request)
{...}` literal passed directly to `mux.HandleFunc`/`mux.Handle` inside
`newMux()`, with the same one-line `// Route to ...` comment `web.rb` uses
above each block. `scanListingOrFail` and `scanFilePath` (the shared logic
extracted in the fix above) are untouched -- this only collapses the thin
per-route glue, not the real work. Confirmed safe by inspection and then
`npm run check`: `main_test.go` only ever exercises routes through
`newMux()`, never by the handler functions' names, so nothing to update
there.

One follow-up closed a specific readability gap `web.rb`'s `DELETE` route
has that the Go version initially didn't: `web.rb`'s delete route shows
both outcomes explicitly (`status 204` / `status 404`) in an `if/else`
right there in the block. Go's `DELETE` closure only showed
`w.WriteHeader(http.StatusNoContent)` -- the 404 path was invisible at the
call site, hidden inside `scanFilePath`, which writes it internally rather
than returning it to the caller. Reverting that (having each handler write
its own `writeFileNotFound(w)`) was rejected -- it's exactly the duplication
`docs/COWORK.md`'s "html-validate, then closing out issue #17" entry
deliberately removed. Instead `scanFilePath` was renamed
`scanFilePathOr404`, mirroring `scanListingOrFail`'s existing
"OrFail"-style naming: the literal status code is now visible at every call
site through the function name alone, with no added lines and no
duplicated filename-resolution logic between `GET`/`DELETE`.

The one route that doesn't fit the "anonymous func per request" shape
cleanly is the static-file route: `mux.Handle` takes an `http.Handler`
value, not a `func(w, r)`, and `fs.Sub(staticFS, "static")` only needs to
run once, not per request. It's registered as an immediately-invoked
`func() http.Handler { ... }()` literal instead, so the one-time setup
still lives at the route's own registration site rather than being hoisted
above the other four routes (its original position, and the position that
prompted this whole thread) -- `newMux()` now reads top-to-bottom as five
self-contained route blocks, matching `web.rb`'s shape as closely as Go's
type system allows.

## The open idea: `bolero`, a helper library rather than a framework

`lambada` being "absurdly simple" is an advantage here, not just a
limitation -- it's small enough to prototype against without much at
stake, and it's already hit two concrete, repeatable patterns (static
serving, untrusted-filename resolution) that a future second Go service in
this account would almost certainly hit again.

Given the history above, the shape worth building isn't a framework or a
router at all -- that ground is both crowded and, per the "thin routers"
failure mode, being actively eroded by stdlib itself. `bolero` (working
name, continuing the `lambada`/`zouk` dance-name lineage) would instead be
a small, focused **helper library**: a handful of tested primitives an app
imports, not a framework an app is rewritten around. No new handler
signature, no context object, no required migration -- just functions that
remove boilerplate `net/http` code already has to write by hand, verified
once instead of per app:

1. **One-line static file configuration** -- `bolero.Static(mux, "/",
   assetsFS)`, matching what Gin/Echo/Fiber already offer as a method on
   their own router, built on `http.FileServerFS` under the hood but
   usable directly against a plain `*http.ServeMux`.
2. **`bolero.SafeJoin(baseDir, untrustedName string) (path string, ok
   bool)`** -- the "resolve an untrusted filename against a base
   directory" primitive no existing framework ships, for the reason
   covered above: it's specific to "small apps that hand out files by
   name," not general HTTP routing. A library's test suite for this one
   primitive would need to cover, once, for every consumer: `../`
   traversal, absolute paths, embedded path separators, empty/`.`/`..`
   names, and symlink escapes -- exactly the cases `scanFilePath` handles
   today, just verified in one place instead of reinvented per app.

Both of these keep the library's actual surface area small and grounded in
patterns that have already shown up twice (`static/` serving, plus
whatever a second Go service's equivalent of `/download/{filename}` turns
out to be), and neither one requires an app to adopt a new routing
paradigm to get the benefit -- the whole point, per the failure analysis
above, is staying out of the "nicer router" fight entirely.

## Open questions

1. Is a second real Go HTTP service actually likely, or is this still a
   single-consumer problem? The library's case gets much stronger once
   there's a second app to prove `SafeJoin`/`Static` against.
2. If we prototype this against `lambada-web` now (before a second
   consumer exists), do we accept it stays a one-consumer package for a
   while, the same tradeoff issue #17 originally flagged for a much
   smaller local helper -- just deliberately taken this time, with eyes
   open?
3. `bolero` is the working name (continuing `lambada`/`zouk`'s dance-name
   lineage; checked against the frameworks surveyed above and the broader
   Go web-framework namespace, no collision found) -- repo and final
   naming still to be confirmed before any code exists.

Nothing above commits to building this. It's the standing frame for
picking the conversation back up, so the next session doesn't have to
re-derive "why not just use Gin," "why did Pat/Martini/Buffalo not stick,"
or "what would actually be worth building" from scratch.
