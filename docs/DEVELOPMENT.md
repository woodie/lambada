# Development

## Layout

```
cmd/lambada-mta/   SMTP server -- receives scans, saves them to attachments/
cmd/lambada-web/   HTTP server -- lists/serves attachments/, JSON API
attachments/       shared scan storage (gitignored; dev-only default --
                    see LAMBADA_ATTACHMENTS_DIR below for the systemd path)
service/           systemd unit files for both binaries
```

Each binary is self-contained (`cmd/lambada-web` embeds its HTML template
and CSS via `go:embed`). Both default to a relative `./attachments`, so a
plain `go build && ./lambada-mta` (or `-web`) from the repo root just works
with no setup. Under systemd, `LAMBADA_ATTACHMENTS_DIR` overrides this to
the shared production location both services actually run against --
see "Configuration" and "systemd Services" below.

## Dependencies

- [`github.com/emersion/go-smtp`](https://github.com/emersion/go-smtp) -- SMTP server primitives (lambada-mta)
- [`github.com/onsi/ginkgo/v2`](https://github.com/onsi/ginkgo) -- BDD test framework
- [`github.com/onsi/gomega`](https://github.com/onsi/gomega) -- matcher library for Ginkgo

`lambada-web` only uses the standard library (`net/http`, `html/template`,
`embed`).

## Quick Start

```bash
# Fetch dependencies and build both binaries (or `make install` to also
# put them on $PATH at /usr/local/bin -- see "systemd Services" below)
go mod tidy
make build

# Run lambada-mta (listens on 0.0.0.0:2525)
./lambada-mta

# Run lambada-web (listens on 0.0.0.0:8080 by default), in another shell
./lambada-web
```

Both default to `./attachments` (created automatically) and expect to be
run from the repo root for ad hoc/dev use -- see "Configuration" below for
the systemd-managed production path. `0.0.0.0:8080` is reachable from other
machines on the LAN out of the box, no nginx required -- see "Reverse proxy
(nginx)" below for the on-Pi setup, and the `LAMBADA_WEB_LISTEN_ADDR`
override to switch lambada-web to loopback-only once nginx is actually
fronting it.

## Configuration

| Variable          | Binary        | Default         | Description                             |
|--------------------|--------------|-----------------|-----------------------------------------|
| `attachmentDir`    | lambada-mta  | `./attachments` | Override with `LAMBADA_ATTACHMENTS_DIR` |
| `listenAddr`       | lambada-mta  | `0.0.0.0:2525`  | TCP address to listen on                |
| `maxFileAge`       | lambada-mta  | `24h`           | How long to retain attachments          |
| `MaxMessageBytes`  | lambada-mta  | `25 MB`         | Maximum accepted message size           |
| `scanDir`          | lambada-web  | `./attachments` | Override with `LAMBADA_ATTACHMENTS_DIR` |
| `listenAddr`       | lambada-web  | `0.0.0.0:8080`  | TCP address to listen on (see below)    |

The settings above are package-level `var`s, not flags or env vars
(matching the original Ruby `mta.rb`/`web.rb`, which hardcoded ports too)
-- edit `cmd/<binary>/main.go` directly if you need to change them. Two
exceptions, both added for a specific operational need rather than general
configurability:

- `attachmentDir`/`scanDir`, overridden by `LAMBADA_ATTACHMENTS_DIR` (both
  binaries read the same variable, since they have to agree on the
  directory) -- added so a plain dev build keeps using the convenient
  `./attachments` default while systemd points both services at the shared
  production location, `/srv/lambada/attachments`, with no rebuild. See
  "systemd Services" below.
- `lambada-web`'s `listenAddr`, overridden by `LAMBADA_WEB_LISTEN_ADDR` --
  added specifically so the nginx-vs-direct choice further down is a
  one-line, no-rebuild switch rather than a code edit.

## Running Tests

```bash
# Install the Ginkgo CLI (first time only)
go install github.com/onsi/ginkgo/v2/ginkgo@latest
go install github.com/woodie/ginkgo-fd@latest

# Make sure ~/go/bin is on your PATH
export PATH="$PATH:$(go env GOPATH)/bin"

# Run every suite
ginkgo -r -v
ginkgo-fd -r

# Or just one binary's suite
ginkgo -v ./cmd/lambada-mta
ginkgo -v ./cmd/lambada-web

# Try format documentation
ginkgo -fd cmd/*
ginkgo-fd cmd/*

# Plain `go test` works too
go test ./...
```

## Testing with swaks (lambada-mta)

```bash
# Install swaks (Swiss Army Knife for SMTP)
brew install swaks   # macOS
apt install swaks    # Debian/Ubuntu

# Send a test email with an attachment
swaks \
  --server localhost:2525 \
  --from sender@example.com \
  --to rcpt@example.com \
  --header "Subject: Test attachment" \
  --attach @/path/to/file.pdf
```

Or with `curl`:

```bash
curl smtp://localhost:2525 \
  --mail-from sender@example.com \
  --mail-rcpt rcpt@example.com \
  --upload-file email.eml
```

## Testing lambada-web by hand

```bash
curl http://localhost:8080/
curl http://localhost:8080/files.json
curl -OJ http://localhost:8080/download/1234567890.pdf
```

## Logging

On Raspberry Pi, view live logs with:

```bash
sudo tail -f /var/log/syslog
```

Example output:

```
2026/05/27 00:35:50 SMTP open relay listening on 0.0.0.0:2525
2026/05/27 00:37:51 New connection from myprinter
2026/05/27 00:37:51 Cleanup removed old file: attachments/1779741215.pdf
2026/05/27 00:37:53 Receiving message
2026/05/27 00:37:56 Write 268809 bytes to attachments/1779867473.pdf
2026/05/27 00:37:56 Saved attachment: attachments/1779867473.pdf
2026/05/27 00:38:02 lambada-web listening on 127.0.0.1:8080, serving ./attachments
```

## systemd Services

The service files live at `service/lambada-mta.service` and
`service/lambada-web.service`, written for the default Raspberry Pi OS
`pi` user. If your Pi runs as a different user (this project's own Pi runs
as `woodie`, not `pi`), edit `User=` in your local copy after `cp`-ing it
in below -- neither `ExecStart=` nor `Environment=LAMBADA_ATTACHMENTS_DIR=...`
need touching, since `make install` always puts both binaries at
`/usr/local/bin` and provisions `/srv/lambada/attachments` (chowned to
whoever ran `make install`, expected to be the same account as `User=`
here) regardless of which user that is. Don't commit the `User=` change
back, since it'd break the default for everyone else cloning the repo.

Both unit files set `Environment=LAMBADA_ATTACHMENTS_DIR=/srv/lambada/attachments`
-- the shared production location both services read from/write to instead
of the dev-only `./attachments` default (see "Configuration" above). This
also means neither file needs a `WorkingDirectory=` anymore: every path
either binary touches is now either embedded in the binary itself
(`lambada-web`'s template/CSS) or set via this absolute env var, so it no
longer matters where -- or whether -- the repo got checked out on the Pi at
all once `make install` has run.

`make install` (see Quick Start above) has to run first -- it's what puts
both binaries at `/usr/local/bin` and creates `/srv/lambada/attachments`,
which is where `ExecStart=`/`Environment=` in both unit files expect them.
Then, to install both services on Raspberry Pi:

```bash
sudo cp service/lambada-mta.service service/lambada-web.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now lambada-mta lambada-web
sudo systemctl status lambada-mta lambada-web --no-pager
```

`systemctl status` pages its output through `less` by default. Over SSH
that's easy to mistake for a hung shell -- `--no-pager` (above) avoids it;
if you forget, `q` exits the pager (not Ctrl-C).

`systemd(1)` documents `SYSTEMD_PAGER=cat` (e.g. in `~/.bashrc`) as a way to
disable the pager for good. In testing on this project's own Pi it did not
suppress the pager, for reasons not yet diagnosed -- `--no-pager` is the
only thing confirmed to work, so use that.

Both unit files set `LimitNOFILE=65536`, well above systemd's stingier
per-service default. This is a backstop, not a confirmed fix for anything
specific -- the actual fix for
[issue #2](https://github.com/woodie/lambada/issues/2) is `newServer()`'s
explicit timeouts in `cmd/lambada-web/server.go`. The higher ceiling is
there in case the leak theory (see `docs/COWORK.md` -- unconfirmed, not a
documented fact) turns out to be right and something still slips past the
timeouts; *if* so, it would also explain why the original symptom showed
up fast under systemd and slow under a manual background, since the same
leak would hit systemd's tighter ceiling first. To check how many file
descriptors a running instance currently holds (rising over time without
plateauing would indicate a leak):

```bash
sudo ls -la /proc/$(pgrep lambada-web)/fd | wc -l
sudo ss -tanp | grep lambada-web   # -a is required -- without it ss only shows
                                    # ESTABLISHED sockets and silently omits the
                                    # CLOSE-WAIT/FIN-WAIT ones a leak actually piles up in
```

Port 25 is redirected to 2525 via iptables so `lambada-mta` can run as a
non-root user:

```bash
sudo iptables -t nat -A PREROUTING -p tcp --dport 25 -j REDIRECT --to-port 2525
```

`lambada-web` doesn't need a redirect to be reachable -- it listens
directly on `0.0.0.0:8080` by default, no privileged port involved. The
port 80 redirect below is purely optional, for whoever wants `:80` without
installing nginx; skip it if `:8080` is fine, or see "Reverse proxy
(nginx)" (next section) if you'd rather have nginx terminating the
connection on `:80`. It only comes back into play automatically if you
roll nginx back out; see "Rolling back to the iptables redirect."

### Verifying or resetting the iptables redirect

The command above adds to `PREROUTING` -- it doesn't replace what's
already there, and nothing persists the result across a reboot (no
`iptables-persistent`/`netfilter-persistent` installed by default). Re-run
it after every setup attempt or reboot and you'll eventually end up with
duplicate/conflicting rules -- a "hot mess," per
[issue #1](https://github.com/woodie/lambada/issues/1).

Check what's actually there:

```bash
sudo iptables -t nat -L PREROUTING -n -v
```

Correct output has exactly one `REDIRECT` line for port 25 (an extra line
for `dpt:80 redir ports 8080` is expected only if you've rolled back to the
pre-nginx setup below):

```
Chain PREROUTING (policy ACCEPT 123K packets, 25M bytes)
 pkts bytes target     prot opt in     out     source        destination
    1    64 REDIRECT   6    --  *      *       0.0.0.0/0     0.0.0.0/0      tcp dpt:25 redir ports 2525
```

If it looks like a mess (duplicates, wrong ports, leftover rules from
testing), wipe `PREROUTING` and reapply cleanly:

```bash
sudo iptables -t nat -F PREROUTING
sudo iptables -t nat -A PREROUTING -p tcp --dport 25 -j REDIRECT --to-port 2525
sudo netfilter-persistent save   # if installed -- otherwise this resets on reboot anyway
```

## Reverse proxy (nginx)

`lambada-web` listens on `0.0.0.0:8080` by default; nginx in front of it is
optional, not assumed. If you do install it, set
`LAMBADA_WEB_LISTEN_ADDR=127.0.0.1:8080` (below) so lambada-web drops to
loopback-only and nginx is the only thing actually facing the LAN on port
80, proxying to it over a stable local connection
(`service/lambada-web.nginx.conf`). The motivation is tracked in
[issue #5](https://github.com/woodie/lambada/issues/5) (an intermittent
zouk connect hang) and
[issue #6](https://github.com/woodie/lambada/issues/6) (the "try nginx"
proposal) -- worth reading both before deciding whether this is worth it
for your setup; the honest caveat from #6 is that this insulates against
the suspected cause rather than confirming it.

Install:

```bash
sudo apt-get update
sudo apt-get install -y nginx
sudo cp service/lambada-web.nginx.conf /etc/nginx/sites-available/lambada-web
sudo ln -sf /etc/nginx/sites-available/lambada-web /etc/nginx/sites-enabled/lambada-web
sudo rm -f /etc/nginx/sites-enabled/default
sudo nginx -t && sudo systemctl reload nginx

# Switch lambada-web to loopback-only now that nginx is fronting it
sudo systemctl edit lambada-web
#   [Service]
#   Environment=LAMBADA_WEB_LISTEN_ADDR=127.0.0.1:8080
sudo systemctl restart lambada-web
```

Verify nginx is proxying correctly (both should return the listing page):

```bash
curl -I http://localhost/            # via nginx, port 80
curl -I http://127.0.0.1:8080/       # lambada-web directly, bypassing nginx
```

### Rolling back to the iptables redirect

If nginx ends up being more trouble than it's worth, rolling back doesn't
require a rebuild -- just remove the override added above, since
`0.0.0.0:8080` is already lambada-web's default:

```bash
# Stop nginx from claiming port 80
sudo systemctl disable --now nginx

# Remove the LAMBADA_WEB_LISTEN_ADDR override added when nginx went in
sudo rm -rf /etc/systemd/system/lambada-web.service.d
sudo systemctl daemon-reload
sudo systemctl restart lambada-web

# Reapply the old port 80 redirect
sudo iptables -t nat -A PREROUTING -p tcp --dport 80 -j REDIRECT --to-port 8080
```
