# Picking up lambada in a new Cowork session

Context for whoever opens this repo cold, with none of the prior conversation history.

## What this is

Lambada is two minimal Go services for a scanner/printer that requires an open relay to e-mail out scans. Run them on a Raspberry Pi to receive scans by e-mail and browse or download them from your home network.

'lambada-mta' is a minimal SMTP server. The scanner emails scans to the Pi over SMTP; lambada-mta receives the message, decodes the attachment, and saves it with an epoch-based filename (e.g. 1779867473.pdf) to attachments/. Files older than 24 hours are cleaned up on each incoming message.

'lambada-web' serves a listing of those files over HTTP: a page with download links, human-readable file sizes, and "time ago" timestamps, plus a GET /scans.json API used by the zouk Mac client. This is a Go port of the scandalous project's Sinatra web server.

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
building an OAuth2 proxy + nginx + DDNS + a hole in the home firewall to
expose scandalous-web safely to the outside, a JSON API got hacked into
scandalous-web in about five minutes, and zouk -- a small native Mac
client that talks to that API directly over the LAN -- came together in
a couple of hours. No HTTPS, no public exposure, no Samba: zouk plus a
lightweight backend answered all three of those complaints at once.

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
  justify). Two symptoms reported together under one title, root-caused
  separately this session:
  1. `sudo systemctl status lambada-mta lambada-web` hung the shell,
     needing Ctrl-C. That's just `less` paging interactively over SSH,
     not an app bug. Fixed by adding `--no-pager` to the status commands
     in `README.md`/`docs/DEVELOPMENT.md`.
  2. The actual "properly background" complaint, clarified by woodie as:
     the client can't connect, like the server doesn't close connections
     properly. Root cause: `cmd/lambada-web/main.go`'s `main()` called
     bare `http.ListenAndServe(listenAddr, newMux())`, which builds a
     zero-value `http.Server` -- every one of `ReadTimeout`,
     `ReadHeaderTimeout`, `WriteTimeout`, and `IdleTimeout` defaults to 0,
     i.e. "wait forever." A client that opens a keep-alive connection and
     goes quiet (a laptop sleeping mid-request, a flaky Wi-Fi hop, zouk
     reconnecting without cleanly closing the old socket) ties up a
     goroutine and a file descriptor on the Pi forever. `Restart=always`
     in `lambada-web.service` never fires to clear this because the
     process never actually crashes -- it just silently accumulates
     leaked connections for as long as it's been running, until new
     clients can't get in at all, even though systemd reports it "active"
     the entire time. Manually killing it and re-backgrounding with
     `setsid nohup ./lambada-web ... & disown` "fixed" it only because
     that reset the leak count to zero -- not because manual
     backgrounding is mechanically different from systemd's
     `Type=simple`; it isn't. This is the class of bug the Ruby/Puma
     predecessor (`~/workspace/scandalous`) never hit: Puma sets sane
     connection timeouts itself, bare `net/http.ListenAndServe` doesn't.
     Fixed by extracting `newServer(addr, handler) *http.Server` in
     `cmd/lambada-web/main.go` with explicit timeouts, with a regression
     test in `main_test.go` asserting every timeout is nonzero so a
     future edit can't silently revert to the zero-value server.
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
- **The timeline confirms the leak theory and explains the asymmetry.**
  systemd started `lambada-web.service` at 03:55:24, logged it listening
  fine, then woodie stopped it by hand at 04:28:53 -- just 33 minutes
  later -- and switched to the manual `setsid nohup ... & disown`
  workaround at 04:53, which was still running fine ~10 hours later when
  the output was pasted. Same binary, same leak, wildly different time to
  failure: that gap is consistent with systemd's per-service file
  descriptor ceiling (`LimitNOFILE`) being much lower than whatever an
  interactive login shell hands a manually-backgrounded process, so the
  exact same zero-timeout leak burns through systemd's tighter ceiling
  fast and the shell's looser one slowly. Fixed: both `service/*.service`
  files now set `LimitNOFILE=65536` as a backstop alongside the real fix
  (`newServer()`'s timeouts). See the "Verifying or resetting the
  iptables redirect"-adjacent note in `docs/DEVELOPMENT.md` for the
  `/proc/<pid>/fd` count check that would confirm this empirically on the
  live box -- not yet run, since this session has no shell on `rackspace`.
- `lambada-mta.service` was unaffected throughout (enabled, active, PID
  matched `ps aux`) -- the asymmetry is specific to `lambada-web`'s HTTP
  keep-alive handling, not a systemd-wide problem.

## Next up

- Confirm on real hardware: `go build ./...`, `go test ./...` (or
  `ginkgo -r -v`), then a soak test of `lambada-web` under systemd over a
  longer stretch (hours, with a handful of clients connecting and going
  quiet) to confirm the timeout fix actually keeps it responsive --
  unlike the pager fix, this one can't be fully confirmed by a quick
  smoke test, since the original bug only showed up after the process had
  been up for a while.
- Consider whether `lambada-web` should also handle `SIGTERM` for a
  graceful `Shutdown()` instead of letting systemd hard-kill it on
  `restart`/`stop` -- not done this session, flagged in case a restart
  ever drops an in-flight download.
