# A Sinatra/Flask-style framework for Go?

`lambada-web` kept coming up short against `scandalous/web.rb`'s Sinatra
readability -- see [issue #17](https://github.com/woodie/lambada/issues/17)
(closed, won't fix) and `docs/COWORK.md`'s session entries for how that
comparison started and how it was actually resolved. This doc keeps the
research behind that decision -- the existing framework landscape, and why
prior attempts at exactly this idea didn't stick -- since it's useful
reference for evaluating the same question again without re-deriving it
from scratch.

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
expressing something close to Sinatra's ergonomics; the gap that remains
isn't a compiler limitation, as the failure-mode research below makes
clear. chi deliberately stays a thin router on top of plain
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
Go's static-typing culture rejects implicit, receiver-swapping,
reflection-driven magic, and this wasn't just a theoretical preference:
the market acted on it. Martini's own
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

**Thin, non-magical routers didn't fail from a flaw -- they solved a
problem that stopped existing.** [Pat](https://github.com/bmizerany/pat) (built by Blake
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
of a namespace that's already crowded and mostly abandoned. A narrow,
app-specific primitive -- something like resolving an untrusted filename
against a base directory before doing something custom with it -- doesn't
have this problem: stdlib has no reason to ever grow a general version of
it, since it's shaped by what a specific app does next, not a generic HTTP
routing or file-serving concern the way `http.ServeMux`/`http.FileServerFS`
are. That's a structurally different bet than Pat, Traffic, Goji, or
Martini made -- the one place a small, honestly-scoped library wouldn't be
racing the standard library to a fight it's already been losing for a
decade, even if a given app never ends up needing one of its own (see
issue #17).

## How this resolved for lambada

Closed as won't-fix in [issue #17](https://github.com/woodie/lambada/issues/17):
`lambada-web`'s actual gaps closed with stdlib alone (`http.FileServerFS`
for static files) plus ordinary function extraction (`scanFilesListing`,
`scanFilesPath`), no new dependency or library required. `lambada-web` is
still the only Go HTTP service in this account, so there was never a
second consumer to justify a shared package -- same reasoning as issue
#6's nginx discussion. See `docs/COWORK.md`'s session entries for the
implementation details.
