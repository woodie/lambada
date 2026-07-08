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
"Sandbox limitation" above), so `go.mod`/`go.sum` are hand-edited to the
best approximation; run `go mod tidy` locally to let Go regenerate
`go.sum` and confirm resolution, then `go test ./...` to confirm the
updated `main_test.go` fixture (`"15 hours ago"`, no `about`) passes
against the real module.
