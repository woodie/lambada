# Comments

Rationale, history, and design notes that used to live as multi-line
comments in the source. Organized by file, then by the type, property, or
function each note is attached to. The source itself now carries at most
one short line at any given spot -- anything longer that would previously
have been a package-doc paragraph or a multi-line `//` note lives here
instead. When a code location kept its own one-line comment, it's noted
below so this stays a complete map of "why," not a duplicate of what's
already readable in the file.

## cmd/lambada-mta/attachments.go

### `package main` (package doc)
Kept a one-line comment in place: "Attachments parses incoming mail, saves attachments to disk, and cleans up old ones."

Full history: Go port of scandalous's `lib/scan_files.rb` (specifically the cleanup/detach side of the `ScanFiles` class) plus the MIME parsing `mta.rb` gets for free from Ruby's `mail` gem. Kept in its own file/test file the same way the Ruby original does: `main.go`'s SMTP `Backend`/`Session` call into this file, but nothing in `attachments.go` knows about `net/smtp`.

### `attachmentDir`, `maxFileAge` (var)
Kept a one-line comment in place: "attachmentDir and maxFileAge are overridden via LAMBADA_ATTACHMENTS_DIR; lambada-web must agree on the same directory."

Full history: Go port of scandalous's `ScanFiles::SCAN_FOLDER` / `ScanFiles::ONE_DAY_AGO`. `attachmentDir` defaults to a relative path ("./attachments") so a plain `go build && ./lambada-mta` from a checkout works with no setup. Under systemd, `LAMBADA_ATTACHMENTS_DIR` overrides it to the shared production location (`/srv/lambada/attachments` -- see `service/lambada-mta.service` and `docs/DEVELOPMENT.md`'s Configuration section). `lambada-web` honors the same variable for the same directory, since both binaries have to agree on where attachments live.

### `envOr` (func)
Kept a one-line comment in place: "envOr returns the named environment variable, or fallback if unset/empty."

Full history: identical helper to `lambada-web`'s `envOr`, duplicated rather than shared -- the two binaries don't have a common internal package to put it in.

## cmd/lambada-mta/main.go

### `package main` (package doc)
Kept a one-line comment in place: "Command lambada-mta is a tiny open-relay SMTP server that saves attachments to disk."

Full history: any mail sent to it gets its attachments saved to disk by `attachments.go` and is otherwise discarded. Go port of scandalous's `mta.rb`, split the same way: the "work" lives in `attachments.go`, and `main.go` is just the SMTP wiring (`mta.rb`'s `MidiSmtpServer` subclass).

### `init` (func)
Kept a one-line comment in place: "LAMBADA_QUIET, if set, silences all logging (see package.json's check script)."

Full history: same mechanism as `cmd/lambada-web/main.go`'s `init` -- see that entry below for the full rationale (both binaries honor the variable identically).

## cmd/lambada-web/main.go

### `package main` (package doc)
Kept a one-line comment in place: "Command lambada-web serves a listing of scans plus a JSON API for the zouk Mac client."

Full history: serves a listing of the scans Lambada (or its Ruby predecessor, scandalous) has saved, plus a small JSON API for the zouk Mac client. Go port of scandalous's `web.rb` + `lib/scan_files.rb`, split the same way: the "work" lives in `scanfiles.go` (`ScanFiles`) and `server.go` (`Server`), and `main.go` is just the HTTP wiring (`web.rb`).

### `scanDir`, `listenAddr` (var)
Kept a one-line comment in place: "scanDir and listenAddr are overridden via LAMBADA_ATTACHMENTS_DIR and LAMBADA_WEB_LISTEN_ADDR; see docs/DEVELOPMENT.md."

Full history: `scanDir` defaults to a relative path so a plain `go build && ./lambada-web` from a checkout just works with no setup. Under systemd, `LAMBADA_ATTACHMENTS_DIR` overrides it to the shared production location (`/srv/lambada/attachments`) -- `lambada-mta` honors the same variable for the same directory, since both binaries have to agree on it. `listenAddr` defaults to `0.0.0.0:8080`, the same direct-expose setup `lambada-web` has always used -- nginx (`service/lambada-web.nginx.conf`) is optional, not assumed. Setting `LAMBADA_WEB_LISTEN_ADDR=127.0.0.1:8080` switches to loopback-only without a rebuild, once nginx is actually the thing facing the LAN on port 80 and proxying to `lambada-web` over a stable local connection. See `docs/COWORK.md` for the motivation (the suspected culprit behind an intermittent zouk connect hang, issue #5) and `docs/DEVELOPMENT.md`'s "Reverse proxy (nginx)" section for setup/rollback.

### `init` (func)
Kept a one-line comment in place: "LAMBADA_QUIET, if set, silences all logging (see package.json's check script)."

Full history: `LAMBADA_QUIET` silences all logging (`log.Printf`/`Fatalf`) when set to any non-empty value -- both binaries (`lambada-web` and `lambada-mta`) honor it the same way. Useful for keeping `ginkgo -r`'s output focused on pass/fail dots rather than every handler's log lines (see `check` in `package.json`), without editing every log call individually.

### `envOr` (func)
Kept a one-line comment in place: "envOr returns the named environment variable, or fallback if unset/empty."

Full history: the one and only knob `lambada-web` exposes outside of editing `main.go` directly -- see the `scanDir`/`listenAddr` var entry above.

### `listingTemplate` (var)
Kept a one-line comment in place: "listingTemplate renders listing.html.tmpl, exposing humanSize/timeAgo (humane.HumanSize/TimeAgo) to the template."

Full history: same shape as scandalous's `listing.erb` calling `human_size`/`time_ago_in_words` inline. `humane.TimeAgo` defaults to `Approximate: true` (matching ActionView's own always-on-past-the-hour behavior, see `github.com/woodie/humane` v0.9.0), and is direction-aware -- it renders "in 3 minutes" for a future time instead of requiring the caller to normalize the sign, which would collapse future and past into the same "3 minutes ago" text (see https://github.com/woodie/lambada/issues/15). It already appends its own "ago"/"in " affix, so the template doesn't add one. As of `humane` v0.9.3, `TimeAgo` is a one-argument convenience that supplies `time.Now()` internally (see `humane`'s own `docs/COWORK.md` v0.9.3 entry), so the closure wrapping it here only exists to take the address of the template's by-value `time.Time`.

### `listingData` (struct)
Kept a one-line comment in place: "listingData is what listing.html.tmpl renders."

Full history: just the raw scan listing -- `timeAgo` reaches for the real clock itself (`time.Now()`) to compute each scan's age, the same way Ruby's `time_ago_in_words(from_time)` defaults its `to_time` to `Time.now` rather than taking it as an argument.

### `handleIndex` (func)
Kept a one-line comment in place: "handleIndex serves the scan listing at the exact root path (registered as \"GET /{$}\", not \"GET /\")."

Full history: the `"GET /{$}"` pattern (unlike a bare `"/"`) only matches the exact root path -- everything else falls through to a 404, matching Sinatra's behavior for unmatched routes.

### `sanitizeFilename` (func)
Kept a one-line comment in place: "sanitizeFilename defends against path traversal; ServeMux already blocks \"..\", this is a second layer."

Full history: reduces `raw` to its base name and rejects anything that isn't a plain, single-segment filename. `net/http`'s `ServeMux` already redirects requests containing `".."` path segments to their cleaned equivalent before any handler runs; this is a second, defense-in-depth layer in case that ever changes (e.g. a future router swap) or a filename containing `"/"` or `".."` ends up on disk some other way. Shared by `handleDownload` and `handleDelete`, so there's one place guarding against directory traversal rather than two copies of the same check.

### `handleDelete` (func)
Kept a one-line comment in place: "handleDelete is the DELETE counterpart to handleDownload, on the same \"/download/{filename}\" route."

Full history: registered on the exact same `"/download/{filename}"` resource path `handleDownload` uses (GET fetches it, DELETE removes it: same resource, different verb, rather than a separate RPC-style `"/delete/{filename}"` route). Browsers can't submit an HTML form with `method="DELETE"`, so the trash icon in `listing.html.tmpl` calls this via a small inline `fetch()`, not a `<form>` post.

## cmd/lambada-web/main_test.go

### `get` (func)
Kept a one-line comment in place: "get performs an in-process GET against newMux() without binding a real listener."

Full history: mirrors how the Ruby suite uses `Rack::Test` against `WebApp`.

### `del` (func)
Kept a one-line comment in place: "del performs an in-process DELETE against newMux(); named del, not delete, to avoid shadowing the builtin."

### `Describe("Lambada WEB")`
Kept a one-line comment in place: "Lambada WEB exercises the HTTP routes (scanfiles.go/server.go have their own test files)."

### `Describe("DELETE /download/{filename}")`
Kept a one-line comment in place: "DELETE /download/{filename} is the RESTful counterpart to GET on the same route, not a separate \"/delete\" route."

Full history: same resource path, different verb, rather than a separate `"/delete/{filename}"` route -- see `handleDelete` in `main.go`.

### `Context("with a file")`, under `Describe("GET /files.json")`
Kept a one-line comment in place: "The exact JSON shape is unit-tested in scanfiles_test.go; this just checks the route is wired up."

## cmd/lambada-web/scanfiles.go

### `package main` (package doc)
Kept a one-line comment in place: "ScanFiles reads the scan directory and shapes the result for callers."

Full history: Go port of scandalous's `lib/scan_files.rb` (the `ScanFiles` class), kept in its own file/test file the same way: `main.go`'s HTTP handlers call into this, but nothing in `scanfiles.go` knows about `net/http`.

### `listing` (func)
Kept a one-line comment in place: "listing returns every *.pdf file in dir, newest filename first (epoch filenames sort lexicographically)."

Full history: scan filenames are an epoch timestamp (e.g. `1779867473.pdf`), so a descending lexicographic sort on the name is equivalent to newest-first -- this matches `ScanFiles.listing`'s `sort_by { |h| h[:name] }.reverse` in the Ruby version.

### `listing`, Stat error branch
Kept a one-line comment in place: "File may have vanished between Glob and Stat (e.g. concurrent cleanup) -- skip it."

Full history: a file may have been removed or renamed between `Glob` and `Stat` (e.g. `lambada-mta`'s cleanup running concurrently), so a `Stat` error here is skipped rather than treated as fatal.

### `scanJSON` (struct)
Kept a one-line comment in place: "scanJSON is the /files.json (and zouk) wire shape; Path is a server-relative path, not a URL."

Full history: served at `/files.json` and consumed by the zouk Mac client. `Path` was previously misnamed `url` in this field and in scandalous's matching Ruby shape; both were renamed together as part of the `/files.json` rename so the field name actually describes what it holds (a server-relative download path, not a URL).

### `toScansJSON` (func)
Kept a one-line comment in place: "toScansJSON converts a raw listing to its API shape, pulled out for unit testing without net/http/httptest."

Full history: mirrors Ruby's `ScanFiles.scans_json`.

## cmd/lambada-web/scanfiles_test.go

### `Describe("ScanFiles")`
Kept a one-line comment in place: "ScanFiles exercises listing/toScansJSON, the Go port of Ruby's ScanFiles#listing/#scans_json."

Full history: kept in its own file/test file (`scanfiles.go`/`scanfiles_test.go`) the same way the Ruby version keeps `ScanFiles` out of `web.rb`.

## cmd/lambada-web/server.go

### `package main` (package doc)
Kept a one-line comment in place: "Server is the http.Server lambada-web runs, with issue #2's timeouts applied."

### `readHeaderTimeout`, `readTimeout`, `writeTimeout`, `idleTimeout` (const)
Kept a one-line comment in place: "Nonzero timeouts avoid the zero-value http.Server that could leak idle keep-alive connections indefinitely (suspected cause of issue #2)."

Full history: the bare `http.ListenAndServe(addr, handler)` helper used previously builds a zero-value `http.Server`, and every one of `ReadTimeout`, `ReadHeaderTimeout`, `WriteTimeout`, and `IdleTimeout` defaults to 0 there -- i.e. "wait forever." A client that opens a keep-alive connection and then goes quiet (a laptop sleeping mid-request, a flaky Wi-Fi hop, zouk reconnecting without cleanly closing the old socket) would tie up a goroutine and a file descriptor on the Pi for as long as the process has been running. This is the suspected -- not confirmed, see `docs/COWORK.md` -- cause behind https://github.com/woodie/lambada/issues/2, and these timeouts are the fix either way: a server that actually times out idle connections can't leak them indefinitely.

### `newServer` (func)
Kept a one-line comment in place: "newServer builds lambada-web's http.Server, pulled out of main so it's unit-testable without binding a real listener."

## cmd/lambada-web/server_test.go

### `Describe("Server")`
Kept a one-line comment in place: "Server exercises newServer, the constructor server.go defines."

Full history: including the issue #2 regression check below.

### `It("sets every timeout to a nonzero value")`
Kept a one-line comment in place: "Regression test for issue #2: guards against newServer reverting to a zero-value (all-timeouts-0) http.Server."

Full history: a zero-value `http.Server` (what the old `http.ListenAndServe(addr, handler)` helper built) leaves every timeout at 0, i.e. "never" -- the suspected cause of leaked keep-alive connections piling up until new clients couldn't connect at all (see `server.go`). This test just has to fail loudly if a future edit accidentally drops back to a zero-value server.
