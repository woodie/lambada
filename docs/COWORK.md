# Picking up lambada in a new Cowork session

Cross-project conventions (git locks, pushing, comments, code style, test
structure) are in `~/workspace/woodie/docs/COWORK.md`. Read `README.md` and
`docs/DEVELOPMENT.md` first; this file only adds what they don't say.

## What this is

A scan server for a Raspberry Pi. `lambada` (the Go code at the repo root)
is a tiny SMTP open relay: the scanner emails a scan, lambada saves it to
`/srv/lambada/attachments`. nginx serves that folder over WebDAV, plus a
static scan page for browsers. zouk (Mac) and huck are the WebDAV clients.

History worth knowing in one paragraph: lambada started as a Go port of
`scandalous` (Ruby, now deprecated) with two services, `lambada-mta` and
`lambada-web`, plus a custom `files.json` protocol for zouk. In October
2026 nginx's WebDAV replaced lambada-web and the protocol entirely (#19,
#20, #21). The web server's code is at the `lambada-web-final` tag; git log
has the rest. None of it is needed to work on the current code.

## Design decisions

- **The folder is a general share, not just a scan inbox.** Cleanup deletes
  only scans (`^[0-9]{10}\.pdf$`, older than 24 hours, run when a scan
  arrives); uploads stay until someone deletes them. nginx refuses uploads
  with a scan's name, so the two never overlap (#22).
- **Uploads:** PUT to the top level only, up to 25 MB, the same limit as an
  emailed scan. Replacing an existing upload is allowed (#22 suggested
  create-only), since Finder can PUT the same name more than once while
  copying a file.
- **No login.** Home network only, same as the open relay.
- **The scan page is plain TypeScript, no framework** (`src/main.ts`),
  chosen over Vue and petite-vue to stay basic. Vite (library mode, the
  same setup as `netpress`) bundles it with humane-ts into one minified
  IIFE, `public/main.js`, committed so the Pi needs no npm; CI checks it
  matches `src/`. humane-ts is pinned to a git tag (`#v0.1.0`) until it's on
  npm. Page files are addressed as `/.lambada/...`, since a relative URL
  from `/` would land in the scans folder.
- **Tests:** Go uses `spec` + `expect` (RSpec-style tree; never a literal `/`
  in a test name, since `spec` and `gorderly` treat it as nesting). The
  page's tests (`spec/javascript`) keep `src/main.ts` at 100% coverage.
  Lint is `neostandard` plus `tsc --noEmit`, like humane-ts.
  `spec/webdav` runs the real nginx config with a real lambada.

## Working from the sandbox

There's no Go toolchain in the Cowork sandbox or the linked Mac VM, so Go
edits are made by inspection: hand off `npm run check` (and
`make test-webdav` for nginx changes) and wait for a clean run before
tagging. After changing `go.mod`, `go mod tidy` has to run on the Mac, and
`go.mod` and `go.sum` get committed together.

## Current status

Branch `lambada-service` (on top of `scan-page`, #20): one `lambada` service
(the MTA, moved out of `cmd/`), lambada-web deleted, `www/` renamed
`public/`, the page moved to TypeScript built into `public/main.js`, uploads
enabled, cleanup limited to scans, docs rewritten. Verified in the sandbox
(Go 1.24 tests, page tests, WebDAV tests against real nginx, the built page
in Chromium); not yet on the Mac with Go 1.26 or on the Pi.

Order from here:

1. `npm install`, `npm run check`, `make test-webdav` and `make docker-run`
   on the Mac; then
   one PR from this branch, which includes `scan-page` (closes #20, #21
   and #22).
2. Install on the Pi (DEVELOPMENT.md, "Upgrading from lambada-mta and
   lambada-web") and test in a browser and Finder: list, download, delete,
   drag a file in to upload, and an emailed scan.
3. Then zouk and huck learn PUT.

The Pi currently runs nginx with the pre-upload config and an older
lambada-mta build; port 80 → 8080 redirect already removed, port 25 → 2525
kept.

## Open issues

- #10, distribution: with one service plus an nginx site and static files,
  a `.deb` (GoReleaser + nfpm, a postinst for systemd, nginx and the
  folders) is the leading option. Not started.
