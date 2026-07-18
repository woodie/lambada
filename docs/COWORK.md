# Picking up lambada in a new Cowork session

Context for whoever opens this repo cold, with none of the prior conversation history.
Cross-project conventions (git locks, sandbox toolchain) are in `~/workspace/woodie/docs/COWORK.md`.

## What this is

Lambada is two minimal Go services for a scanner/printer that requires an open relay to e-mail out scans. Run them on a Raspberry Pi to receive scans by e-mail and browse or download them from your home network.

'lambada-mta' is a minimal SMTP server. The scanner emails scans to the Pi over SMTP; lambada-mta receives the message, decodes the attachment, and saves it with an epoch-based filename (e.g. 1779867473.pdf) to attachments/. Files older than 24 hours are cleaned up on each incoming message.

'lambada-web' serves a listing of those files over HTTP: a page with download links, human-readable file sizes, and "time ago" timestamps, plus a GET /files.json API used by the zouk Mac client. This is a Go port of the scandalous project's Sinatra web server.

Read `README.md` first for the user-facing description and API
contract. This doc is about *how the project got here* and *how to keep
working on it consistently*.

## Why Go, and what it costs

The short version of how lambada exists at all: `scandalous`
(Ruby/Sinatra + Puma) was hand-built after a previous postfix +
Gmail-with-an-app-specific-password setup stopped working once Google
killed that auth method. Actually getting scans off it turned out to be
its own saga -- [zouk's README](https://github.com/woodie/zouk) lays out
why: downloading over plain HTTP means browser warnings about unsafe
files, setting up HTTPS on a home network is "an absolute pain," and
Samba -- the obvious third option -- is slow and awkward, especially on
a Pi. lambada itself actually started as a Samba-friendly MTA for that
exact reason, before discovering Samba on a Pi performs too poorly to
be worth it.

The fix that actually worked wasn't more infrastructure. Rather than
building a Certbot + oauth2-proxy + nginx + DDNS + a hole in the home
firewall to expose scandalous-web safely to the outside, a JSON API got
hacked into scandalous-web in about five minutes, and zouk -- a small
native Mac client that talks to that API directly over the LAN -- came
together in a couple of hours. No HTTPS, no public exposure, no Samba:
zouk plus a lightweight backend answered all three of those complaints
at once.

Porting that backend from Ruby to Go (lambada) wasn't because anything
was wrong with scandalous-web -- there wasn't. It was that only one
specific Ruby patch version (3.1.2p20) actually ran on the Pi, and
chasing that kind of version pinning forever felt worse than just
rewriting the thing. So: "what could possibly go wrong?" Issue #2 is the
literal answer -- Puma absorbs a whole class of HTTP-server footgun that
Go's stdlib hands back to whoever writes `main()`, and lambada walked
straight into it. That's not an argument against Go, just the honest
price of leaving a mature framework's defaults behind. The result today:
real production use by the family, who are, by all accounts, thrilled
with how much easier this is than what came before.

## Sandbox limitation -- read this before touching Go code

There is **no Go toolchain in the Cowork sandbox** (confirmed: no `go`
binary, no `sudo` to install one, and the outbound network proxy blocks
`go.dev`/`dl.google.com`/Ubuntu's package mirrors -- only a short
allowlist, which happens to include `github.com` itself, is reachable).
Every `cmd/lambada-mta` / `cmd/lambada-web` edit in this project has been
made by inspection only. Don't claim a Go change builds or passes tests
without the user confirming `go build ./...`, `go test ./...`, or
`ginkgo -r -v` on a machine that actually has Go (their Mac, the Pi,
wherever) -- just make the edit and ask for that confirmation. Same
situation as zouk's missing Swift toolchain, different language.

## Current state

`main` was one commit ahead of `origin/main` (a docs cleanup: added this
file, moved `DEVELOPMENT.md` into `docs/`) before this session added more
-- check `git status`/`git diff` before assuming HEAD reflects the latest
code, same caveat as zouk.

Two issues were filed against the repo, both about running `lambada-web`
on the Pi in production, and both addressed this session:

- **[#1, "Basic iptables configuration information"](https://github.com/woodie/lambada/issues/1)**
  -- not a code bug, a docs gap. README/DEVELOPMENT.md showed how to *add*
  the port 25->2525 and 80->8080 `PREROUTING` redirects but never how to
  verify they're actually in place, or how to reset them once repeated
  setup attempts turn them into a "hot mess" (woodie's words, after
  cleaning it up by hand). Fixed by adding a "Verifying or resetting the
  iptables redirect" subsection to `docs/DEVELOPMENT.md` with the
  `iptables -t nat -L PREROUTING -n -v` check and the flush-and-reapply
  commands from the issue. Worth remembering for next time: the redirect
  is a bare `iptables` command with nothing persisting it (no
  `iptables-persistent`/`netfilter-persistent`), so it's gone after every
  reboot unless reapplied or saved.
- **[#2, "Properly background lambada-web"](https://github.com/woodie/lambada/issues/2)**
  -- a previous session got as far as the `systemctl` pager hang and
  stopped there; the actual connection-handling bug went undiagnosed,
  and woodie ended up living with the `setsid nohup ... & disown`
  workaround rather than getting a real fix (or giving up on lambada-web
  and going back to scandalous-web, which it wasn't quite bad enough to
  justify). Two symptoms reported together under one title, diagnosed
  separately this session -- with very different confidence levels:
  1. `sudo systemctl status lambada-mta lambada-web` hung the shell,
     needing Ctrl-C. That's just `less` paging interactively over SSH,
     not an app bug -- confirmed, not a theory. Fixed by adding
     `--no-pager` to the status commands in `README.md`/`docs/DEVELOPMENT.md`.
  2. The actual "properly background" complaint, clarified by woodie as:
     the client can't connect, like the server doesn't close connections
     properly. What's verifiably true about the code, just from reading
     it: `cmd/lambada-web/main.go`'s `main()` called bare
     `http.ListenAndServe(listenAddr, newMux())`, which builds a
     zero-value `http.Server` -- every one of `ReadTimeout`,
     `ReadHeaderTimeout`, `WriteTimeout`, and `IdleTimeout` defaults to 0,
     i.e. "wait forever." From there it's a theory, not a confirmed root
     cause: a client that opens a keep-alive connection and goes quiet (a
     laptop sleeping mid-request, a flaky Wi-Fi hop, zouk reconnecting
     without cleanly closing the old socket) *could* tie up a goroutine
     and a file descriptor on the Pi indefinitely, which *would* explain
     why `Restart=always` in `lambada-web.service` never fired (the
     process never actually crashes) and why manually killing and
     re-backgrounding with `setsid nohup ./lambada-web ... & disown`
     seemed to "fix" it. Nobody ever pulled an actual `/proc/<pid>/fd`
     count from the Pi while the symptom was happening, though, and a
     later investigation into a related-looking hang ([issue #5]
     (https://github.com/woodie/lambada/issues/5), below) found fd counts
     at rest were normal and couldn't reproduce a leak -- so treat this as
     the leading explanation, not settled fact. Either way, a zero-timeout
     `http.Server` is bad practice on its own merits regardless of whether
     it explains this specific incident, and that's the part actually
     fixed: extracted `newServer(addr, handler) *http.Server` (originally
     in `cmd/lambada-web/main.go`, now its own `server.go`) with explicit
     timeouts, with a regression test (`server_test.go`) asserting every
     timeout is nonzero so a future edit can't silently revert to the
     zero-value server.
  - Alongside #2's fix, also simplified both `service/*.service` files'
    `ExecStart` from `/usr/bin/env ./<binary>` to the binary's absolute
    path -- one less moving part in the supervision chain, matching how
    `scandalous-mta.service`'s `ExecStart=/usr/bin/ruby mta.rb` does it
    directly rather than through `env`. Not the root cause of #2 on its
    own, just a minor hardening made alongside it.

### Live evidence from the Pi (rackspace), same session

woodie pasted real `ps aux` / `systemctl status --no-pager` output partway
through this session. Two things it caught, both fixed:

- **`service/*.service` says `User=pi` / `/home/pi/workspace/lambada` --
  on purpose, don't "fix" this again.** `pi` is the default Raspberry Pi
  OS user; the checked-in files are a template for whoever else clones
  this repo, most of whom will actually be running as `pi`. This
  project's own Pi happens to run as `woodie` instead (unlike
  `scandalous`, which is woodie-personal and was never meant for anyone
  else to deploy, so hardcoding `User=woodie` there is fine). Mid-session
  this got edited to `User=woodie` directly in the repo before that
  distinction was caught -- reverted. If you're deploying on a Pi where
  the user isn't `pi`, edit your local copy of the service file after
  `cp`-ing it to `/etc/systemd/system/`, and don't commit that edit back.
- **The timeline is consistent with the leak theory, though it doesn't
  confirm it.** systemd started `lambada-web.service` at 03:55:24, logged
  it listening fine, then woodie stopped it by hand at 04:28:53 -- just 33
  minutes later -- and switched to the manual `setsid nohup ... & disown`
  workaround at 04:53, which was still running fine ~10 hours later when
  the output was pasted. Same binary, two very different times to
  failure: *if* the zero-timeout theory is right, that gap fits systemd's
  per-service file descriptor ceiling (`LimitNOFILE`) being much lower
  than whatever an interactive login shell hands a manually-backgrounded
  process -- the same leak would burn through systemd's tighter ceiling
  fast and the shell's looser one slowly. But it's equally consistent
  with a one-off network blip, something specific to that boot, or plain
  bad luck, and issue #5's later investigation into a similar-looking
  hang found fd counts normal at rest and couldn't pin down a leak either
  -- so treat this as corroborating, not conclusive. Fixed regardless:
  both `service/*.service` files now set `LimitNOFILE=65536` as a
  backstop alongside the timeout fix (`newServer()` in
  `cmd/lambada-web/server.go`). See the "Verifying or resetting the
  iptables redirect"-adjacent note in `docs/DEVELOPMENT.md` for the
  `/proc/<pid>/fd` count check that would actually confirm or rule this
  out on the live box -- not yet run, since this session has no shell on
  `rackspace`.
- `lambada-mta.service` was unaffected throughout (enabled, active, PID
  matched `ps aux`) -- the asymmetry is specific to `lambada-web`'s HTTP
  keep-alive handling, not a systemd-wide problem.

## Comment philosophy

Code comments explain current, non-obvious behavior -- not how the code
got here. Refactor history, rejected libraries, "this used to hand-roll
X" -- that belongs in this file, not in `main.go`. Picking a well-known
library to do an obvious thing (formatting bytes, formatting durations)
shouldn't need a comment defending the choice.

`humanSize`/`timeAgo` went through three shapes before landing:

1. The initial Ruby->Go port hand-rolled Rails-clone helpers
   (`humanSize`, `timeAgoInWords`) directly in `main.go`, feeding a
   `toViewData`/`viewScan` layer that pre-formatted each scan before
   handing it to the template -- a port artifact, not a deliberate
   choice.
2. Looked for a byte-for-byte match of Rails' `number_to_human_size`/
   `distance_of_time_in_words` (`gonumbers`) -- rejected, it has real
   bugs at small `n`. No library matches Rails' exact wording *and* is
   bug-free, so simplicity won: swapped in `github.com/dustin/go-humanize`
   (`Bytes`) and `github.com/justincampbell/timeago` (`FromDuration`),
   still wired through `toViewData`. Confirmed via `ginkgo -r -v`
   (37/37 passing).
3. Final shape: deleted `toViewData`/`viewScan` entirely.
   `listing.html.tmpl` now calls `humanSize`/`timeAgo` directly via
   `template.FuncMap`, the same way `scandalous/views/listing.erb` calls
   `number_to_human_size`/`time_ago_in_words` inline rather than
   pre-formatting in Ruby. One deliberate deviation from the ERB: Go's
   `timeAgo` takes an explicit `now time.Time` (threaded into the
   template as `{{$now := .Now}}`) instead of calling `time.Now()`
   internally, to keep age calculations dependency-injectable and
   testable.
4. Undid that one deviation. `timeNow` (`time.Now` directly) is now a
   `listingTemplate` FuncMap entry, and the template calls
   `{{timeAgo .Time timeNow}}` -- matching `listing.erb`'s
   `time_ago_in_words` exactly, which reaches for `Time.now` implicitly
   rather than taking it as an argument. `listingData` drops its `Now`
   field. The dependency-injectable property step 3 called out as
   deliberate was never actually exercised: the one render test asserts
   a loose "less than a minute ago" substring, robust to whatever clock
   reaches the template, real or fixed. A future test wanting an exact
   age string would need `handleIndex` to accept an injected
   template/clock regardless of how `timeNow`'s value arrives, so the
   `.Now` plumbing wasn't earning its keep. Commit `cccb5b1`.
5. Switched `timeAgo` from `timeago.FromDuration(now.Sub(t).Abs())` to
   `timeago.FromTime` directly ([issue #15]
   (https://github.com/woodie/lambada/issues/15)). `FromDuration` takes
   a bare duration, so the caller has to normalize the sign with
   `.Abs()` -- which throws away direction: a future mtime (clock skew,
   a malformed server timestamp) would render identically to a past one.
   `FromTime` takes the two timestamps itself and picks "ago" or "from
   now" accordingly, so no caller-side guard is needed. The wrinkle
   flagged when this was first scoped -- `FromTime` self-appends "
   ago"/" from now", where `FromDuration` returns bare text and the
   template appended its own " ago" -- is resolved by no longer
   double-appending: the template calls `{{timeAgo .Time}}` and prints
   the result as-is. `timeNow` and the explicit `now` parameter are gone
   -- `timeAgo` is back to a single argument, `timeago.FromTime` reaching
   for the real clock itself, same as step 4's reasoning for why the
   dependency-injectable version wasn't earning its keep. Existing
   render-test assertions ("less than a minute ago", "about 15 hours
   ago", etc.) were unaffected -- `FromTime`'s output for a past time is
   byte-identical to the old `FromDuration` + template-appended-"ago"
   text. Added one new case proving the actual fix: a file with a future
   mtime now renders "3 minutes from now" instead of colliding into "3
   minutes ago".

   woodie's brief for this change, for the record: don't fight the
   library appending "ago" itself; prefer whatever can be called
   straight from the template without a caller-side future/past guard;
   accept minor wording differences from the Ruby/Rails original; and
   lean toward keeping Go and zouk's Swift implementation
   (`ScanEntry.timeAgo(relativeTo:)` in
   `zouk/Sources/ZoukKit/ScanEntry.swift`) similar. Swift's version
   still takes an explicit `relativeTo:` and strips the formatter's own
   trailing " ago" rather than embracing it -- a deliberate difference,
   not an oversight: zouk's `RelativeDateTimeFormatter` has no built-in
   "less than a minute" bucket the way `FromDuration` does, so
   `ScanEntry.timeAgo` needs its own `< 30`-second clamp either way, and
   at that point keeping the explicit clock and controlling the suffix
   itself cost it nothing extra. Go's `FromTime` has no such gap to
   paper over, so the more direct one-argument call was the simpler
   choice there. Not applied to `scandalous`'s Ruby (still
   `distance_of_time_in_words(from_time, Time.now).abs`-based, matching
   real Rails behavior, future-mtime handling included) -- untouched
   since nothing prompted revisiting it this session.
6. Swapped `github.com/dustin/go-humanize`'s `Bytes` for
   `github.com/c2h5oh/datasize`'s `ByteSize.HumanReadable()`. Trigger:
   `main_test.go` asserted `"80 kB"` while `scandalous/spec/web_spec.rb`'s
   equivalent case (same 79992-byte fixture) asserts `"78.1 KB"` --
   `go-humanize`'s `Bytes` is SI (1000-based, lowercase `kB`), while
   Rails' `number_to_human_size` is 1024-based but labels it `KB`, not
   `KiB`. `go-humanize`'s `IBytes` is 1024-based but keeps the IEC
   `KiB`/`MiB` labels, and its default 2-significant-digit rounding drops
   the decimal once the integer part hits two digits (`IBytes(79992)` ->
   `"78 KiB"`, not `"78.1 KiB"`) -- `BytesN`/`IBytesN` can force a third
   digit back in, but neither exposes swapping the unit strings without
   hand-rolling the formatter. `datasize.ByteSize.HumanReadable()` does
   both natively: 1024-based division with `KB`/`MB`/... labels and a
   fixed one-decimal format, which happens to reproduce
   `number_to_human_size`'s output exactly for this fixture. Confirmed by
   arithmetic, not a test run (sandbox limitation, as above): 79992 / 1024
   = 78 remainder 120, formatted as `"78.1 KB"`. `go.mod`/`go.sum` updated
   to drop `dustin/go-humanize` and add `c2h5oh/datasize`
   (`v0.0.0-20231215233829-aa82cc1e6500`, its only published pseudo-version
   -- the repo has no semver tags); `go.sum`'s hash lines for the new
   module aren't filled in here since the sandbox can't compute them --
   run `go mod tidy` before building.
7. Compared `main_test.go`, `scandalous/spec/web_spec.rb`, and
   `zouk/Tests/ZoukKitTests/ScanEntrySpec.swift` side by side this
   session. Two wording gaps turned up: Go/Ruby render `"about 15 hours
   ago"` for the hour bucket, `RelativeDateTimeFormatter` (no "about"
   concept) renders plain `"15 hours ago"`; and the future case reads
   three different ways -- `"3 minutes from now"` (Go), `"in 3 minutes"`
   (Swift), Ruby's buggy `"3 minutes ago"`. Confirmed intentional, not
   bugs to converge -- this **supersedes** step 5's "lean toward keeping
   Go and zouk's Swift implementation similar," which in hindsight read
   as a wording-parity goal it never actually achieved. The real rule:
   pick whichever library plugs in with the least resistance (no
   hand-rolled future/past guard), and let each project's spec document
   that library's actual output. Cross-language prose matching isn't a
   goal -- three specs describing three libraries' real behavior is the
   point, not a bug. No code changes made.
8. Replaced `c2h5oh/datasize` (step 6) with a hand-rolled `humanSize`.
   Trigger: woodie compared the new `"220.6 KB"` output against a real
   file's Finder size (`"226 KB"`) and found they didn't match --
   `datasize` is 1024-based, Finder is 1000-based, and both happen to
   label the unit `KB`. This means step 6's fix was itself wrong: it
   matched `scandalous`'s `number_to_human_size` output (`"78.1 KB"`),
   but that's *also* 1024-based under a `KB` label (`number_to_human_size(1234)
   # => "1.21 KB"` is Rails' own documented example), so step 6 matched
   the wrong reference. zouk's `ByteCountFormatter(.file)` -- confirmed
   1000-based, matching Finder -- was the one implementation that had it
   right the whole time. No published Go or Ruby library ships
   1000-based math under capitalized `KB`/`MB` labels: the technically
   correct-per-SI libraries (`go-humanize`, `docker/go-units`) use
   1000-based math but lowercase `kB`; the libraries that capitalize it
   pair it with 1024-based math (`datasize`, `IBytes`'s `KiB`). Rails'
   `number_to_human_size` dropped its `:prefix` option in Rails 5 and
   has no built-in way back to 1000-based math (see
   [rails/rails#40054](https://github.com/rails/rails/issues/40054) --
   a private-API monkey-patch is possible but wasn't used). Rather than
   post-process a library's output or monkey-patch Rails internals,
   `humanSize` in `main.go` is now a small hand-rolled function (no
   external dependency -- `go.mod`/`go.sum` drop `c2h5oh/datasize`
   entirely, nothing replaces it), and `scandalous`'s `web.rb` grew the
   Ruby equivalent (`human_size`, in a `helpers do` block, replacing
   `number_to_human_size` and the now-unused
   `ActionView::Helpers::NumberHelper` include). Both round to 2
   significant digits the same way `go-humanize`'s `Bytes()` always did
   -- only the base (1000, unchanged) and the label case (`KB`, not
   `kB`) needed fixing. `main_test.go`'s and `web_spec.rb`'s shared
   79992-byte fixture assertion goes back to `"80 KB"` -- the original
   pre-step-6 number was right all along, only the case was wrong.

## This session: nginx in front of lambada-web (issues #5, #6), on a feature branch

Two new issues, filed this session, prompted by woodie noticing zouk's
launch sometimes hangs on the running-dog screen until an unrelated
browser `GET /` unsticks it -- not reproducible with the old Ruby/Puma
`scandalous-web`:

- **[#5](https://github.com/woodie/lambada/issues/5)** -- the bug report
  itself, with a repro procedure for next time (watch `/proc/<pid>/fd`
  live, trigger the hang, hit `GET /` from a browser, see whether the fd
  count moves before or after). Investigated this session: fd count at
  rest was a normal 8-9 (not a leak), the single iptables redirect rule
  per port was clean (not issue #1 recurring), no shared lock between
  `handleIndex`/`handleScansJSON`. Root cause **not confirmed** -- the bug
  stopped reproducing mid-session before the live capture could happen.
- **[#6](https://github.com/woodie/lambada/issues/6)** -- woodie's own
  "try nginx in front of lambada" proposal, written up after talking
  through whether a "smart person" suggesting nginx for X/Y/Z reasons
  actually holds up for a homelab system handling a couple of connections
  a week. It does, for two reasons that don't depend on
  scaling/TLS/rate-limiting theater: nginx terminates the client
  connection with its own decades-tuned timeout/keep-alive handling
  before lambada-web ever sees it, and it lets the iptables port 80->8080
  `PREROUTING` redirect (already flagged as fragile in issue #1) go away
  entirely. The honest caveat, also from #6: this insulates against the
  suspected cause of #5 rather than confirming what it actually was.

Decided to try it anyway, with two safety nets woodie asked for
explicitly:

1. **A no-rebuild switch, not a hardcoded address.** `lambada-web`'s
   `listenAddr` still defaults to `0.0.0.0:8080` -- the same direct-expose
   setup it's always had, so a fresh clone works whether or not nginx is
   installed. `LAMBADA_WEB_LISTEN_ADDR` is the opt-*in*: set it to
   `127.0.0.1:8080` via a `systemctl edit lambada-web` override once nginx
   is actually fronting it, switching lambada-web to loopback-only without
   touching code or rebuilding. (Originally shipped the other way around --
   defaulting to `127.0.0.1:8080` with the env var as the opt-*out* back to
   direct -- but woodie wanted the no-nginx case to keep working out of the
   box rather than silently going unreachable for anyone who skips the
   nginx section, so the default/override roles got swapped before this PR
   merged.) The narrower fix would have been to just hardcode whichever
   address and call the iptables line dead; woodie wanted rollback to not
   require a rebuild either direction, so this is the one deliberate
   exception to "no flags or env vars" in `docs/DEVELOPMENT.md`'s
   Configuration section.
2. **A feature branch, not straight to `main`.** All of this -- the
   `main.go` change, the new `service/lambada-web.nginx.conf`, and the
   README.md/DEVELOPMENT.md doc updates -- is on `nginx-reverse-proxy`,
   not `main`, specifically so it can be tried on the Pi and abandoned
   without a revert if it doesn't earn its keep.

`docs/DEVELOPMENT.md` gained a "Reverse proxy (nginx)" section (install +
verify) and a "Rolling back to the iptables redirect" subsection under it;
the old "port 25 and 80 redirected via iptables" text is now port-25-only,
since lambada-web no longer needs a redirect by default.

### A `.git` gotcha hit while doing this

Running `git checkout -b` from the Cowork sandbox's bash mount against
this same working copy raced with something else touching `.git` at the
same time (most likely woodie's own Terminal on the actual Mac, since the
folder is a live two-way mount, not a copy) and left `.git/index.lock`,
`.git/packed-refs.lock.{x,x2,stale.<ts>}`, and
`.git/refs/tags/0.3.0.lock.{x,x2,stale.<ts>}` behind -- conflict-renamed
copies of git's own lockfiles, not names git itself would ever create.
The `refs/tags/0.3.0.lock.*` ones are the nastier kind: anything that
walks `refs/tags/*` (`git fsck`, `git tag`, `git push --tags`) tries to
parse them as refs and fails with `invalid sha1 pointer`. None of it
touched real history -- `main` and `nginx-reverse-proxy` both still point
at `7f71178` -- but the sandbox couldn't delete the stray files itself
(`Operation not permitted` on every one), so woodie cleaned them up by
hand on the Mac. **Takeaway for next time:** don't run git commands from
both sides (sandbox bash and a local Terminal) against the same mounted
repo at once.

## This session: `/files.json` + `url`→`path` field rename (breaking API change)

woodie noticed the JSON field was never a URL -- `/download/<name>` is a
server-relative path, not a URL -- and decided to fix the misnomer now
rather than carry it forward, plus rename the endpoint itself from
`/scans.json` to `/files.json` (the API just shares files; "scans" was an
unnecessary narrowing, and woodie wants room to expand the API later).
`scandalous` started this rename first (the original, hand-written-in-a-
few-minutes Ruby implementation), with the `url`→`path` field rename
already landed there as an uncommitted local change before this session;
this session's job was the endpoint rename in `scandalous` plus mirroring
both renames into `lambada-web` to keep the two backends in sync, since
`zouk` is meant to talk to either one. Deliberately untouched: every
method/function name (`scans_json`, `handleScansJSON`, `toScansJSON`,
`fetchScans()` on the zouk side) -- only the wire shape (route path, JSON
key) changed, not the code's internal vocabulary for it.

Changed here: `main.go`'s route registration (`GET /files.json`, still
calling `handleScansJSON` -- name intentionally not changed), `scanfiles.go`'s
`scanJSON.URL`/`json:"url"` field renamed to `Path`/`json:"path"`, and the
corresponding test/doc/README references. Made by inspection only per the
sandbox limitation above, then **confirmed on real hardware**: woodie ran
`ginkgo-fd -r` on their Mac -- every suite green (Attachments, Lambada WEB
including `GET /files.json`, Server, ScanFiles including `toScansJSON`).
Committed as `602430c` ("Rename /scans.json to /files.json; rename url
field to path") and tagged `2.0.0` (a major bump, not minor, since lambada
passed 1.0 -- strict semver for a breaking wire-contract change). `zouk`
has to land its half of this too (or stay backward-compatible) since an
old `zouk` build pointed at a freshly rebuilt `lambada-web` would 404 on
`/scans.json` and fail to decode a listing missing the `url` key it
expects -- there's no in-between state where both old and new
clients/servers interoperate. See zouk's own `docs/COWORK.md` for that
side of the same change (not yet committed/tagged as of this writing).

Deployed to the production Pi (`rackspace`) and verified this session:
`curl http://localhost:8080/files.json` returns the new `path` key,
`curl -i http://localhost:8080/scans.json` 404s as expected. One wrinkle
hit along the way, worth recording: the Pi's checkout was sitting on a
stale `install-as-service` branch (2 commits ahead of where `main` was
*then*, but `main` had since gained the nginx-reverse-proxy and
better-templates work -- including the equivalent systemd/`make install`
setup that branch was trying to add). No merge was needed; `main` already
superseded it, so the fix was just `git checkout main && git pull`
(picking up `602430c`/`2.0.0` once pushed), `go build`, `make install`,
`systemctl restart`. `install-as-service` itself is now dead weight on the
remote -- worth deleting next time someone's doing branch cleanup.

## This session: filled in `main_test.go`'s `timeAgo` cases

`cmd/lambada-web/main_test.go`'s "with files can be older" block (four
age cases: just now, 3 minutes, 15 hours, 30 hours) was a stub --
`var time // FIXME` and empty `BeforeEach()`s -- left that way pending
this pass. Filled in to mirror `scandalous/spec/web_spec.rb`'s
equivalent block, with one necessary deviation: Ruby's lazy
`let(:time)`, overridden per nested `context`, doesn't translate
directly to Ginkgo, since Ginkgo runs outer `BeforeEach`s before inner
ones -- a single shared outer `before { File.utime(time, ...) }` would
fire before the inner context's `time` override existed. Used a
`setFileAge(age time.Duration)` closure instead, called from each
inner `BeforeEach` with `os.Chtimes`.

Also caught and fixed a pre-existing bug in the stub while filling it
in: the "three minutes ago" case asserted `"less than 3 minutes"`.
Reading `github.com/justincampbell/timeago`'s `FromDuration` source
confirmed 3 minutes actually renders as `"3 minutes ago"` (matching
what `web_spec.rb` itself asserts) -- both specs' `it` descriptions say
"less than 3 minutes ago", which is a pre-existing mislabel in the
Ruby original, kept as-is (with a comment) rather than "fixed", per
the one-for-one mirroring goal.

Made by inspection only per the sandbox limitation above, then
**confirmed on real hardware**: woodie ran `ginkgo-fd cmd/lambada-web`
on their Mac -- 22/22 specs passing, including all four new age cases.

## Next up

- A longer soak test of `lambada-web` under systemd (hours, with a
  handful of clients connecting and going quiet) to confirm the timeout
  fix from issue #2 actually keeps it responsive over time, not just at
  a quick smoke-test scale -- the original bug only showed up after the
  process had been up for a while.
- Consider whether `lambada-web` should also handle `SIGTERM` for a
  graceful `Shutdown()` instead of letting systemd hard-kill it on
  `restart`/`stop` -- not done this session, flagged in case a restart
  ever drops an in-flight download.
- A "what's not conventional Go here" pass turned up a few small,
  unacted-on items: `scanDir`/`listenAddr` (web) and
  `attachmentDir`/`listenAddr`/`maxFileAge` (mta) are hardcoded package
  vars rather than `flag`-parsed -- the one place the port is actually
  more rigid than the Ruby version, since `scandalous/config/puma.rb`'s
  bind address is editable without a rebuild. (`listenAddr` (web)
  partially addressed this on the `nginx-reverse-proxy` branch -- see
  above -- via a single `LAMBADA_WEB_LISTEN_ADDR` env override, not a
  general flags system. `scanDir` (web) and all three `mta` vars are
  still hardcoded.) Smaller ones: `sort.Slice`
  in `listing()` could be `slices.SortFunc` now that `go.mod` declares
  `go 1.26.3`, and `filepath.Glob`'s error in `listing()` could be
  wrapped with `%w` plus the directory path for a more useful log line.
  None of this blocks anything.
- Ginkgo/Gomega (RSpec-style `Describe`/`Context`/`It`) is the one
  clearly non-default-for-Go choice in the test suite, picked
  deliberately so `main_test.go` mirrors `scan_files_spec.rb`/
  `web_spec.rb`'s structure one-for-one -- flagged here for visibility,
  not because it needs reconsidering.

## This session: swapped humanSize/timeAgo for the humane gem

The hand-rolled `humanSize` (step 8 above) and
`github.com/justincampbell/timeago`-backed `timeAgo` are both replaced
by [`humane`](https://github.com/woodie/humane)
(`humane.SizeFormatter`/`humane.NewTimeFormatter`), a small module
extracted from this exact logic so `lambada` and `scandalous` (via
[`humane-ruby`](https://github.com/woodie/humane-ruby)) share one
implementation instead of two drifting copies. `humanSize`'s output is
unchanged -- `humane.SizeFormatter.Format` is the same rounding
approach, just moved out of this repo. `timeAgo`'s wording changes
slightly: no more "about" prefix on the hour bucket (`"15 hours ago"`,
not `"about 15 hours ago"`), matching Finder/zouk and dropping the one
place `timeago`'s wording diverged from them. The direction-aware
behavior from issue #15 (step 5) carries over unchanged --
`humane.TimeFormatter.Format(t, relativeTo)` takes both times
explicitly, so `timeAgo` in `main.go` wraps it in a one-line closure
supplying `time.Now()` as `relativeTo`, matching `listing.html.tmpl`'s
existing single-argument `{{timeAgo $file.Time}}` call.

`go.mod` drops `justincampbell/timeago` and its indirect
`justincampbell/bigduration` entirely, adding `github.com/woodie/humane
v0.1.0` in their place. Made in the sandbox (no Go toolchain -- see
"Sandbox limitation" above), so `go.mod`/`go.sum` were hand-edited to the
best approximation, then **confirmed on real hardware**: woodie ran `go
mod tidy` (which downloaded `humane v0.1.0` and regenerated `go.sum`)
and `go test ./...` -- 44/44 passing, including the updated `"15 hours
ago"` fixture. Also fixed a leftover non-English `It` description
(`"displays a future"` -> `"displays 3 minutes from now"`) to match
`scandalous`'s equivalent spec wording. Tagged and released as `2.2.0`
(`docs/releases/2.2.0.md`), pushed to GitHub.

## This session: bumped `humane` to v0.2.0 (future wording change)

Revisited step 7's "not a bug, three specs describing three libraries'
real behavior" conclusion above -- that reasoning predated `humane`
existing at all (it was about the raw pre-extraction libraries:
`justincampbell/timeago`, Rails' buggy `time_ago_in_words`, and
`RelativeDateTimeFormatter`). Once `humane` was extracted specifically to
*model* `RelativeDateTimeFormatter`, shipping wording that knowingly
diverged from what that API actually outputs (symmetric `"X from now"`
vs. the real `"in X"`) undercut the library's own stated premise. Fixed
upstream in `humane` `v0.2.0` -- see `humane`'s own `docs/COWORK.md` for
the full reasoning -- and propagated here: `go.mod` bumped to
`github.com/woodie/humane v0.2.0`, `main.go`'s doc comment on
`listingTemplate` updated to say `"in 3 minutes"` instead of `"3 minutes
from now"`, and `main_test.go`'s "when files can be newer" case updated
to match. `humane-ruby`/`scandalous` got the same treatment in parallel.

Made in the sandbox (no Go toolchain -- see "Sandbox limitation" above)
and **not yet confirmed on real hardware** -- `go.mod` was hand-edited to
`v0.2.0` before that tag was pushed to GitHub, so `go mod tidy` will need
to actually fetch it once `humane`'s `v0.2.0` tag is pushed and `go.sum`
regenerated. Tag/push order matters here: push `humane`'s `v0.2.0` tag
before running `go mod tidy`/`go test ./...` on this repo, or `go mod
tidy` will fail to resolve the new version.

## This session: JS linting/testing, golangci-lint, `LAMBADA_QUIET`, v2.4.0

`scandalous` got a JS lint/test setup first (`standard` + `vitest` for its
`public/script.js`), then woodie asked for the same structure in `lambada`.
`cmd/lambada-web/templates/listing.html.tmpl` turned out to have the exact
same `deleteFile` confirm/fetch/DELETE logic inline (a Go port artifact --
semicolons, no `standard` style yet, unlike scandalous's already-extracted
copy). Extracted it to `cmd/lambada-web/static/script.js`, embedded via a
new `//go:embed static/script.js` var + `handleScript` + `GET /script.js`
route (mirroring `handleStyle`/`style.css`), with a Ginkgo test for the new
route in `main_test.go`.

Two more renames piggybacked on the same session, both internal-only
(neither touches `/files.json`'s wire format, which is a separate
`toScansJSON` struct): `cmd/lambada-web/templates/` moved to `views/`
(matching scandalous's `views/` naming), and `listingData`'s `Scans` field
is now `Listing`.

JS tooling: `standard` (zero-config) + `vitest` + `sinon`.
`spec/javascript/script.spec.js` exercises `deleteFile` via Node's `vm`
module rather than jsdom -- jsdom's `window.location.reload` turned out to
be non-configurable and unstubbable with sinon, and since `script.js` only
ever touches `confirm`/`fetch`/`location`/`alert` (no real DOM), a bare
`vm.createContext` sandbox is both sufficient and one less dependency.
`spec/javascript/setup.js` aliases `context = describe` to match the
RSpec-style `describe`/`context`/`it` hierarchy documented in the
account-wide `docs/COWORK.md`.

`golangci-lint` was added too -- this repo's first Go linter ever, default
linter set, no `.golangci.yml` -- wired into a brand-new
`.github/workflows/ci.yml` (`lambada` had no CI at all before this
session) alongside the JS job.

`package.json` gained `lint-js`/`test-js`/`lint-go`/`test-go` plus a
`check` script running all four in sequence, stopping at the first
failure. `check` uses compact output (`vitest run --reporter=dot`, plain
`ginkgo -r` instead of `ginkgo-fd -r`'s documentation-style verbosity) so
running everything together stays readable -- except `ginkgo -r` was still
drowning its own dots in every handler's `log.Printf` output (woodie
pasted a real run showing this). Fixed with `LAMBADA_QUIET`: both
binaries' `init()` now check it and redirect the `log` package's output to
`io.Discard` when set to any non-empty value. Not a production knob --
`check` sets `LAMBADA_QUIET=1` only for its own `ginkgo -r` step;
standalone `npm run test-go` (`ginkgo-fd -r`) runs without it, so full
logging is still there when actually debugging a suite.

Verification: `standard`/`vitest` confirmed directly in the sandbox (all 9
`script.spec.js` cases passing). `golangci-lint`/`ginkgo` can't run in the
sandbox -- no Go toolchain, and no network access to install
`golangci-lint` (`gem install`/`go install`-style fetches are blocked the
same way `humane`'s v0.2.0 tag fetch was, above) -- so all Go-side changes
were made by inspection only, per the usual caveat. **Confirmed on real
hardware this session** regardless: woodie pasted `npm run check` output
twice, first showing the log-noise problem, then (post-`LAMBADA_QUIET`) a
clean run -- `golangci-lint run` "0 issues", `ginkgo -r` "Lambada MTA Suite
- 21/21 specs" and "Lambada Web Suite - 24/24 specs", both PASS.

Tagged `2.4.0` (`docs/releases/2.4.0.md`), six commits (script
extraction+rename, `LAMBADA_QUIET`, JS tooling, CI, docs, release notes).
Not pushed from the sandbox -- confirmed no SSH key or agent for
`git@github.com` is available there (`ssh -T git@github.com` fails DNS
resolution directly; `git ls-remote origin` fails host key verification
even routed through the sandbox's proxy) -- woodie pushes `main` plus the
`2.4.0` tag from their Mac. The same setup was mirrored into `scandalous`
in parallel this session (also tagged `2.4.0`); scandalous doesn't keep a
`docs/COWORK.md`, so nothing about it is recorded there beyond its own
commit history.

One git gotcha worth remembering: scandalous's `DEVELOPMENT.md` ->
`docs/DEVELOPMENT.md` move (done by woodie locally, before this session)
was already sitting *staged* in git's index, not just modified in the
working tree. Committing a specific, unrelated set of files with `git add
<paths>` doesn't exclude an already-staged file from the same commit --
`git commit` includes everything currently in the index. Worth checking
`git status` for pre-existing staged changes before assuming a targeted
`git add`/`git commit` pair will produce the commit you expect.

## This session: bumped `humane` to v0.3.0 (`CollapseMinute` -> `IncludeSeconds`)

Unlike the `v0.2.0` bump above, this one needed zero source changes here.
`main.go` only ever calls `humane.NewTimeFormatter()` and
`humane.SizeFormatter{}` -- no `CollapseMinute`/`IncludeSeconds` field is
ever set explicitly -- so the rename's polarity inversion doesn't touch this
repo's behavior at all, and `main_test.go` already asserted the `v0.2.0`
asymmetric wording (`"in 3 minutes"`), so there was nothing stale to update
there either. `scandalous` came out the same way in parallel -- see its own
commit history (no `docs/COWORK.md` there).

Only change: `go.mod`'s `github.com/woodie/humane` requirement bumped
`v0.2.0` -> `v0.3.0`, hand-edited the same way the `v0.2.0` bump was.
Different situation this time though -- `humane`'s `v0.3.0` tag is already
pushed and released (unlike last time, where the tag didn't exist yet), so
`go mod tidy`/`go get github.com/woodie/humane@v0.3.0` should resolve
cleanly on the first try rather than needing the tag/push-order dance
documented above. `go.sum` deliberately left untouched -- hand-computing
its checksums isn't safe to fake; needs a real `go get`/`go mod tidy` run
to regenerate correctly. Made in the sandbox (no Go toolchain); not yet
confirmed on real hardware.

## This session: bumped `humane` to v0.4.0, opted into `Approximate`

`humane`'s `v0.4.0` adds `TimeFormatter.Approximate` (prefixes "about"/"in
about" onto buckets of an hour or larger -- see `humane/docs/releases/v0.4.0.md`).
Unlike the `v0.3.0` bump, this one *does* need a source change: `main.go`'s
module-level `timeFormatter` var changed from `humane.NewTimeFormatter()` to
`humane.TimeFormatter{Approximate: true}` (still `IncludeSeconds: false` via
the zero value, so that behavior is unchanged -- only `Approximate` is new).

This listing page is a static server render with no live refresh -- the
same case `scandalous`'s `time_ago` opted into `approximate: true` for, and
the whole reason the option exists. `main_test.go`'s "fifteen hours ago"/
"thirty hours ago" contexts updated to expect `"about 15 hours ago"`/
`"about 1 day ago"`; everything under an hour (`"3 minutes ago"`, `"less
than a minute ago"`, `"in 3 minutes"`) is untouched, since `Approximate`
only touches hour-plus buckets.

`go.mod`'s `github.com/woodie/humane` requirement bumped `v0.3.0` -> `v0.4.0`,
hand-edited the same way prior bumps were. `go.sum` deliberately left
untouched again -- needs a real `go get`/`go mod tidy` run. Made in the
sandbox (no Go toolchain); not yet confirmed on real hardware. `humane`'s
own `v0.4.0` is committed locally but not yet tagged/released as of this
writing -- confirm that first (see `humane/docs/COWORK.md` "Next up"),
then `go get github.com/woodie/humane@v0.4.0` here.

## This session: adopting `humane` v0.9.0's full API rethink

`humane` dropped its instantiated-formatter shape entirely in `v0.9.0` --
`humane.SizeFormatter{}`/`humane.TimeFormatter{Approximate: true}` and
their `.Format` methods are gone, replaced by package-level `humane.HumanSize`/
`humane.TimeAgo` functions (see `humane/docs/COWORK.md`'s own `v0.9.0` entry
for the full cross-repo rationale). `main.go`'s module-level `sizeFormatter`/
`timeFormatter` vars are gone with them -- there was no per-instance state to
hold once configuration moved to a per-call argument, so the vars were
ceremony this rewrite no longer needs. `listingTemplate`'s `FuncMap` now
references `humane.HumanSize` directly and wraps `humane.TimeAgo(&t,
time.Now())` inline.

`humane.TimeAgo`'s `Approximate` now defaults to `true` (was `false`,
requiring the explicit `TimeFormatter{Approximate: true}` this file used to
set) -- matches what this app already opted into, so the rendered listing is
unchanged. `main_test.go`'s existing assertions (`"less than a minute ago"`,
`"in 3 minutes"`, the `"80 KB"` fixture) all stay under the hour-scale
`Approximate` boundary or are otherwise untouched by the rounding-rule
correction in `HumanSize` (see `humane`'s `docs/COMMENTS.md`), so nothing
needed updating there.

`humane` `v0.9.0` is tagged, pushed, and released
([release](https://github.com/woodie/humane/releases/tag/v0.9.0)) -- the
temporary `replace github.com/woodie/humane => ../humane` directive is
removed from `go.mod`; the `require` line's existing `v0.9.0` now resolves
to the real published module. Confirmed for real via `go mod tidy` (pulled
the real published module) + `npm run check` on woodie's Mac -- JS lint,
9/9 JS tests, `golangci-lint` (0 issues), and both Ginkgo suites (42/42)
all green. Tagged, pushed, and released as `lambada` `2.7.0`:
https://github.com/woodie/lambada/releases/tag/2.7.0. Deployed to the Pi
via `git pull` + `go mod tidy` + `make install` + `systemctl restart` --
confirmed live against a real file (`"226 KB"`, `"1 minute ago"` on the
listing page).

## This session: html-validate, then closing out issue #17 without a new framework

Two threads, same session:

**HTML fragment linting.** `scandalous` and `lambada` both wanted a way to
lint/check their HTML views. `herb` (the HTML+ERB toolchain) was the
starting point, but it only understands Ruby's `<% %>` delimiters, not
Go's `{{ }}` -- so it can't parse `views/listing.html.tmpl` at all. Since
one consistent tool across both repos mattered more than ERB-specific
smarts, both repos got `html-validate` (offline, npm, explicit
fragment/incomplete-template support) instead: `npm run lint-html`,
folded into `check`. Along the way, both templates turned out to share
the same latent bug: the delete button's `onclick` attribute nested a
double-quoted interpolated string inside a double-quoted HTML attribute,
which breaks a template-agnostic parser's raw-source quote counting (it
has no `{{ }}`/`<% %>` awareness). Fixed in both by swapping the outer
attribute to single quotes -- Go's `html/template` was never actually
confused by the original quoting (it lexes `{{ }}` actions before any
HTML tokenization), but the linter needed it regardless. Also added the
missing `lang="en"` on `<html>` in both.

**Issue #17 ("Static helper for Go web").** woodie's ask went further than
the issue itself: use Sinatra (`scandalous/web.rb`) as the model for "push
work into testable libraries, keep route handlers thin," and see whether
Go needs a whole Sinatra/Flask-style framework to get there, or whether
stdlib plus ordinary refactoring already does it.

Two stdlib/Go-idiom fixes closed most of the actual boilerplate, no new
framework or even a local helper needed:

- `http.FileServerFS` (stdlib, Go 1.22+ -- this repo is on 1.26.3) replaces
  `handleStyle`/`handleScript` entirely. `//go:embed static/style.css` +
  `//go:embed static/script.js` + two handlers became one `//go:embed
  static` + `mux.Handle("GET /", http.FileServerFS(sub))` -- the same
  "static files just work" behavior Sinatra gives `scandalous/public/` for
  free, achieved with zero new dependencies. Confirmed `"GET /{$}"` (exact
  root, already used for the index) and `"GET /"` (everything else) don't
  conflict -- Go's route-precedence rules are built for exactly this
  combination.
- Deduped two blocks of copy-pasted glue that had nothing to do with
  routing sugar: `handleIndex`/`handleScansJSON` had an identical
  five-line "fetch the listing or 500" block (extracted to
  `scanListingOrFail`, which takes `w` and writes the 500 itself on
  failure, so callers just do `if !ok { return }`), and
  `handleDownload`/`handleDelete` had an identical "sanitize the filename,
  stat the file, 404 if missing" block (extracted to `scanFilePath`).
  `scanFilePath` went through two shapes in the same session: first as a
  pure `string -> (string, bool)` function with callers each repeating
  `writeFileNotFound(w)` themselves, then folded that call inside
  `scanFilePath` (taking `w` directly) once woodie pointed out the
  repeated call -- now it matches `scanListingOrFail`'s shape exactly,
  and both pairs of handlers reduce to the identical `if !ok { return }`
  after the helper call. (`sanitizeFilename` was also inlined directly
  into `scanFilePath` per woodie's request, since it had exactly one
  caller left after the dedup.) One deliberate, called-out behavior
  tweak: the two failure modes (bad filename vs. missing file) used to
  produce two different 404 bodies (`http.NotFound`'s default page vs. a
  hand-written "File not found") -- now both go through `writeFileNotFound`
  for one consistent body. `main_test.go` only ever asserted status codes
  for these paths, not body text, so nothing broke.

Net: `handleStyle`/`handleScript` gone, ~24 lines of duplicated glue gone,
zero new dependencies, zero new abstraction layer. Made by inspection
first (no Go toolchain in the sandbox), then **confirmed on real
hardware**: woodie ran `npm run check` -- JS lint clean, 9/9 JS tests,
`golangci-lint` 0 issues, both Ginkgo suites green (MTA 21/21, Web
21/21), and the new `lint-html` step clean too.

Whether a real, published Sinatra/Flask-style Go framework is still worth
building is an open question, being discussed live rather than decided
this session -- worth a look at whatever `docs/COWORK.md` entry (if any)
follows this one for how it landed. The two fixes above account for
everything concretely wrong in `main.go` today; what's left (a `Render`
helper for the one template call, a `JSON` helper for the one JSON route,
route-declaration sugar) is thinner than what motivated a framework in
the first place, and issue #6/#17 already established this account's
default posture on building generic-sounding abstractions ahead of a
second real consumer.

## This session: adopted `humane` v0.9.3 (`TimeAgo` is now one-argument)

`humane` `v0.9.3` renames the old two-argument `TimeAgo` to `DistanceInTime`
and adds a new one-argument `TimeAgo(at, opts...)` supplying `time.Now()`
internally -- see `humane`'s own `docs/COWORK.md` `v0.9.3` entry. This is
exactly the shape `listingTemplate`'s `timeAgo` FuncMap closure was already
hand-rolling (`return humane.TimeAgo(&t, time.Now())`), so the fix is a
one-line simplification: `return humane.TimeAgo(&t)`. The closure itself
stays -- still needed to take the address of the template's by-value
`time.Time` -- just drops the now-redundant `time.Now()` argument.

`go.mod` bumped `v0.9.0` -> `v0.9.3` via a real `go get
github.com/woodie/humane@v0.9.3` + `go mod tidy` on woodie's Mac (this
session's edit only hand-touched the version string in the sandbox, per
usual). Confirmed for real via `npm run check` -- JS lint clean, 9/9 JS
tests, `golangci-lint` 0 issues, both Ginkgo suites green (MTA 21/21, Web
21/21). Not yet tagged/released as `lambada`, and not yet deployed to the
Pi -- woodie was away from home this session, so the Pi side is a
deliberately separate follow-up once back on the same network.

## This session: three CI fixes (golangci-lint-action, duplicate workflows, vitest ERESOLVE)

woodie reported a real failing GitHub Actions run
(https://github.com/woodie/lambada/actions) and this session worked
through it one failure at a time as each fix uncovered the next.

**`golangci-lint` couldn't load its config.** `can't load config: the Go
language version (go1.24) used to build golangci-lint is lower than the
targeted Go version (1.26.3)`. `ci.yml`'s `golangci-lint` job pinned
`golangci/golangci-lint-action@v6` with `version: latest` -- but `v6` only
resolves the latest golangci-lint **v1.x** release, a line that's stopped
tracking new Go versions and was last built with go1.24. Bumped the
action to `@v9` (`v7+` targets golangci-lint v2, whose releases track
current Go) -- `version: latest` unchanged. Confirmed by reading
`golangci-lint-action`'s own README (`Compatibility` section) rather than
on real hardware, since golangci-lint itself can't run in the sandbox (no
Go toolchain, per "Sandbox limitation" above).

**Two workflows fired on every push.** `.github/workflows/go.yml` -- a
leftover GitHub-scaffolded "Go" template (`actions/checkout@v4` +
`actions/setup-go@v4` with a hardcoded `go-version: '1.26.3'`, then `go
build`/`go test ./...`) -- triggered on the same `push`/`pull_request`
(`main`) events as `ci.yml`, so every push kicked off two separate
workflow runs. Not a pure duplicate, though: `ci.yml` only ran
`golangci-lint` + the JS checks and never actually built or tested the Go
code -- `go.yml` was the only place `go test ./...` (which runs the
Ginkgo suites via the stdlib test runner, no `ginkgo` CLI needed) actually
happened in CI. Folded it into `ci.yml` as a new `go-test` job, matching
the `golangci-lint` job's `actions/checkout@v5` + `actions/setup-go@v5`
(`go-version-file: go.mod`) pattern instead of `go.yml`'s stale pinned
versions, then deleted `go.yml`. `README.md`'s CI badge (pointed at the
now-deleted `go.yml`) updated to `ci.yml` as a follow-up woodie caught
separately. One workflow file now, one run per push/PR, same lint/build/
test/JS coverage as before.

**`npm install` hit `ERESOLVE` on a fresh CI run.** `html-validate@11.5.6`
carries an *optional* peerDependency on `vitest ^3.0.0 || ^4.0.1` (for a
Vitest/Jest matcher neither repo uses), while `package.json` pinned
`vitest ^2.1.9`. Neither `lambada` nor `scandalous` commits a
`package-lock.json` (see the `javascript` job's comment: "no
package-lock.json committed, so npm ci isn't available"), so every CI run
does a fresh, unconstrained `npm install` -- and once vitest 2.x lands in
the tree, npm's resolver flags it as conflicting with that peer, even
though the peer is optional. Bumped `vitest` to `^4.1.10` (latest stable;
`5.0.0` is still beta) in both repos. Unlike the Go-side fixes above, this
one **was** confirmed directly in the sandbox -- Node/npm is available
here even without a Go toolchain: clean `npm install`, 9/9
`script.spec.js` tests passing, `standard` lint clean, in both `lambada`
and `scandalous`. `scandalous` doesn't keep its own `docs/COWORK.md`
(per the note above), so this is the record of that side of the fix too.

All four fixes committed locally to `lambada`'s `main` (golangci-lint-action
bump, vitest bump, workflow consolidation, badge fix -- 4 commits ahead of
`origin/main` as of this writing) and to `scandalous`'s `main` (vitest bump
only, 1 commit ahead). Not pushed from the sandbox, per the usual "Pushing"
rule in the account-wide `docs/COWORK.md` -- woodie pushes both from his
Mac.

## This session: migrating `cmd/lambada-web`'s tests off Ginkgo/Gomega

Supersedes the "Next up" note above ("Ginkgo/Gomega... flagged here for
visibility, not because it needs reconsidering") -- it is being
reconsidered now, prompted by the same `sclevine/spec` evaluation done for
`gorderly` (see that repo's own `docs/COWORK.md`, "Test-writing convention"
section) landing on `~/workspace/spec` (the account's own fork, which added
`Describe`/`it.Context()`/`it.T()`/`Var[T]` this session) plus a new
`~/workspace/expect` matcher module, once real usage here -- not just
`gorderly`'s own tests -- showed what a Gomega replacement actually needs to
cover.

All five `cmd/lambada-web` test files rewritten:

- `middleware_test.go`, `server_test.go` -- direct `Describe`/`It`/`Expect`
  translations. `server_test.go` needed explicit generic type arguments
  twice (`expect.BeIdenticalTo[http.Handler](mux)`,
  `expect.BeNumerically[time.Duration](">", 0)`) since Go's generic
  inference doesn't look at how `.To(...)`'s result is used afterward, only
  at the arguments given -- `mux`'s own type (`*http.ServeMux`) and the
  untyped `0` don't match `got`'s type (`http.Handler`, `time.Duration`)
  closely enough to infer on their own.
- `scanfiles_test.go` -- `GinkgoT().TempDir()` became `it.T().TempDir()`
  inside `it.Before`, the concrete case that justified adding `S.T()` to the
  `spec` fork in the first place (mirrors `GinkgoT()`'s own reason for
  existing). `DescribeTable`/`Entry` (the invalid-filename cases) became a
  plain `for _, tc := range cases { it(tc.name, func(){...}) }` loop --
  `it` is just a func value in `spec`, so calling it in a loop needs no
  table-DSL equivalent at all.
- `main_test.go` -- the deep-nesting case (`Describe` > `Context` > nested
  `Context` > `It`, five levels at its worst). Each top-level
  `describe(...)` block declares its own `context := describe` alias before
  nesting -- same `G` value under two names, matching this account's
  Quick/Kotest `describe`/`context` convention (see `next-caltrain-swift`/
  `next-caltrain-kotlin`'s `GoodTimesSpec` files) even though `spec` itself
  has no separate `Context` type. `Skip("running as root...")` inside
  `BeforeEach` became `it.T().Skip(...)` inside `it.Before` -- the other
  real `S.T()` use case, alongside `TempDir` above.
- `main_suite_test.go` deleted outright. Its only job was Ginkgo's
  `RunSpecs` entry point; `spec.Run` needs no shared suite registration
  across files, so each file's own `func TestXxx(t *testing.T)` is a
  complete, independent entry point on its own. `TestLambadaWeb` (the name
  `main_suite_test.go` used) moved into `main_test.go` directly rather than
  inventing a new name.

`go.mod`: dropped `github.com/onsi/ginkgo/v2`/`github.com/onsi/gomega`,
added `github.com/sclevine/spec v1.4.0` (real upstream tag, overridden
locally) and a placeholder `github.com/woodie/expect
v0.0.0-00010101000000-000000000000`, both via `replace` directives pointing
at `../spec`/`../expect` for now. Left every indirect dependency
(`go-logr/logr`, `slim-sprig`, `go-cmp`, `pprof`, `go.yaml.in/yaml`,
`golang.org/x/net`) untouched rather than guessing by inspection which are
still needed transitively by `go-smtp`/`humane` once Ginkgo/Gomega no
longer pull them in -- that's exactly what a real `go mod tidy` resolves
correctly, and hand-pruning risks getting it wrong.

Made entirely by inspection -- no Go toolchain in this sandbox, same
limitation flagged throughout this file. **Not yet confirmed on real
hardware.** On your Mac, in order:

```
cd ~/workspace/lambada
go mod tidy
go test -v ./cmd/lambada-web/...
```

Once that's green: tag and push `~/workspace/spec` and `~/workspace/expect`
(each has its own "Verification" notes in its own `docs/COWORK.md`), then
swap the two `replace` lines in `go.mod` for real published versions and
run `go mod tidy` again for a real `go.sum` -- the same tag-then-bump-then-
`go mod tidy` sequence this file has used for every `humane` version bump
above, just for two new modules instead of one.

`package.json`'s `test`/`test-go`/`check` scripts updated too: `ginkgo-fd -r`/
`ginkgo -r` replaced with `go test -v ./... | gorderly -fd` (verbose) and
plain `go test ./...` (the `check` script's terse-on-success step -- bare
`go test` is already terse, so no `gorderly` piping needed there). Needs
`gorderly` on `PATH` (`go install github.com/woodie/gorderly@latest`,
matching how that repo's own `docs/COWORK.md` describes installing it to
`~/go/bin`) -- not yet confirmed this actually renders lambada's real
`go test -v` output correctly, only `gorderly`'s own suite and the
`spec-demo` translation have been confirmed against it so far.

## This session: migrating `cmd/lambada-mta`'s tests off Ginkgo/Gomega too

Same pattern as `cmd/lambada-web` above, applied to `attachments_test.go`
(one file, one top-level `Describe("Attachments", ...)`) and
`main_suite_test.go` (deleted -- same reasoning as `lambada-web`'s: no
shared `RunSpecs` entry point needed once each package's own
`func TestXxx(t *testing.T)` is independent). Kept the test function named
`TestAttachments`, not `TestLambadaMTA` -- there's only one top-level group
in this package, so `Test<TopLevelName>` (matching every `lambada-web` file)
reads more consistently than reusing the old suite-level name.

This file's real Gomega usage needed two matchers `lambada-web`'s migration
never touched: `BeADirectory()` (`checkAttachmentDir`'s tests check a real
directory, not just an existing path) and `Panic()` (`Expect(func(){...}).NotTo(Panic())`,
checking `checkAttachmentDir` doesn't panic on a pre-existing directory or a
symlink) -- both added to `~/workspace/expect` this session (see its own
`docs/COWORK.md`). Every `BeNil()` call site in this file turned out to be
checking an `error` (`Expect(err).To(BeNil())`), so those became
`expect.Succeed()`, not a new general `BeNil` matcher.

Same verification gap as `lambada-web`: no Go toolchain in this sandbox,
not yet run for real. `go mod tidy && go test -v ./cmd/lambada-mta/...`
covers this package once you're back on your Mac -- same command block as
`lambada-web`'s, since both packages share one `go.mod`.

## This session: naming the context alias, dropping `it.` off Before/After

Two readability follow-ups, prompted directly by reviewing the migration
above.

`context := describe` (used at the top of every nested group across both
packages) is now `context := describe.AsContext()` -- a one-line, no-op
method added to `~/workspace/spec` (`func (g G) AsContext() G { return g }`)
purely so the alias explains itself instead of reading like a bare, easy-to-
miss assignment.

`it.Before(...)`/`it.After(...)` shortened to bare `before(...)`/`after(...)`
via a Go method value, declared once per file right where `it` first comes
into scope: `before, after := it.Before, it.After` (or just `before :=
it.Before` in files with no `After` calls -- Go won't compile an unused
`after`). No `spec` change needed for this one; method values already do
the job. The `it(...)` call itself (declaring a spec) is unchanged -- only
its `Before`/`After` methods got the shorthand, since dropping `it` from
`it(...)` entirely would need Ginkgo-style ambient global state, which is
exactly the design `spec` (and this fork) deliberately avoids. Applied to
every already-migrated file: `scanfiles_test.go`/`main_test.go`
(`lambada-web`) and `attachments_test.go` (`lambada-mta`).

Follow-up correction, same session: `main_test.go`/`attachments_test.go`
originally redeclared `context := describe.AsContext()` inside every
top-level `describe(...)` block, mirroring Ginkgo's per-block feel too
literally. Unnecessary -- `describe`/`context`/`it`/`before`/`after` are
the same five values for the entire file regardless of nesting depth, and
Go closures already see every enclosing scope's locals at any depth. Moved
all five declarations (well, three: `describe`/`it` come from `spec.Run`'s
own callback params) to one spot at the very top of each file's
`spec.Run(...)` body; every nested block now just uses them via ordinary
closure capture, no re-declaration anywhere. This is the actual fix for
"can we hide these so they just work always" -- within `spec`'s
no-global-state design, one declaration per file is as automatic as it
gets without changing `G`/`S`'s function signatures to inject fresh
`describe`/`context`/`it`/`before`/`after` values into every nested
closure's own parameters, which would be a much larger, likely
non-upstreamable redesign, not attempted here.
