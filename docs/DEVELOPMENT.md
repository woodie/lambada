# Development

## Layout

```
cmd/lambada-mta/   SMTP server -- receives scans, saves them to attachments/
cmd/lambada-web/   HTTP server -- lists/serves attachments/, JSON API
attachments/       shared scan storage (gitignored; symlink to Samba if desired)
service/           systemd unit files for both binaries
```

Each binary is self-contained (`cmd/lambada-web` embeds its HTML template
and CSS via `go:embed`), but both expect to be run with the repo root as
their working directory, since each defaults to a relative `./attachments`.

## Dependencies

- [`github.com/emersion/go-smtp`](https://github.com/emersion/go-smtp) -- SMTP server primitives (lambada-mta)
- [`github.com/onsi/ginkgo/v2`](https://github.com/onsi/ginkgo) -- BDD test framework
- [`github.com/onsi/gomega`](https://github.com/onsi/gomega) -- matcher library for Ginkgo

`lambada-web` only uses the standard library (`net/http`, `html/template`,
`embed`).

## Quick Start

```bash
# Fetch dependencies
go mod tidy

# Build and run lambada-mta (listens on 0.0.0.0:2525)
go build -o lambada-mta ./cmd/lambada-mta
./lambada-mta

# Build and run lambada-web (listens on 0.0.0.0:8080), in another shell
go build -o lambada-web ./cmd/lambada-web
./lambada-web
```

Both default to `./attachments` (created automatically) and expect to be
run from the repo root.

## Configuration

| Variable          | Binary        | Default         | Description                                  |
|--------------------|--------------|-----------------|-----------------------------------------------|
| `attachmentDir`    | lambada-mta  | `./attachments` | Where attachments are written                 |
| `listenAddr`       | lambada-mta  | `0.0.0.0:2525`  | TCP address to listen on                      |
| `maxFileAge`       | lambada-mta  | `24h`           | How long to retain attachments                |
| `MaxMessageBytes`  | lambada-mta  | `25 MB`         | Maximum accepted message size                 |
| `scanDir`          | lambada-web  | `./attachments` | Where scans are read from                     |
| `listenAddr`       | lambada-web  | `0.0.0.0:8080`  | TCP address to listen on                      |

The settings above are package-level `var`s, not flags or env vars
(matching the original Ruby `mta.rb`/`web.rb`, which hardcoded ports too)
-- edit `cmd/<binary>/main.go` directly if you need to change them.

## Running Tests

```bash
# Install the Ginkgo CLI (first time only)
go install github.com/onsi/ginkgo/v2/ginkgo@latest

# Make sure ~/go/bin is on your PATH
export PATH="$PATH:$(go env GOPATH)/bin"

# Run every suite
ginkgo -r -v

# Or just one binary's suite
ginkgo -v ./cmd/lambada-mta
ginkgo -v ./cmd/lambada-web

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
curl http://localhost:8080/scans.json
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
2026/05/27 00:38:02 lambada-web listening on 0.0.0.0:8080, serving ./attachments
```

## systemd Services

The service files live at `service/lambada-mta.service` and
`service/lambada-web.service`. To install both on Raspberry Pi:

```bash
sudo cp service/lambada-mta.service service/lambada-web.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now lambada-mta lambada-web
sudo systemctl status lambada-mta lambada-web --no-pager
```

`systemctl status` pages its output through `less` by default. Over SSH
that's easy to mistake for a hung shell -- `--no-pager` (above) avoids it;
if you forget, `q` exits the pager (not Ctrl-C).

Both unit files set `LimitNOFILE=65536`, well above systemd's stingier
per-service default. This is a backstop, not the fix -- the actual fix for
[issue #2](https://github.com/woodie/lambada/issues/2) is `newServer()`'s
explicit timeouts in `cmd/lambada-web/main.go`. But it explains why the bug
showed up fast under systemd and slow under a manual background: both leak
identically, they just hit different ceilings. To check how many file
descriptors a running instance currently holds (rising over time without
plateauing means it's still leaking):

```bash
sudo ls -la /proc/$(pgrep lambada-web)/fd | wc -l
sudo ss -tanp | grep lambada-web   # -a is required -- without it ss only shows
                                    # ESTABLISHED sockets and silently omits the
                                    # CLOSE-WAIT/FIN-WAIT ones a leak actually piles up in
```

Port 25 is redirected to 2525, and port 80 to 8080, via iptables so both
services can run as a non-root user:

```bash
sudo iptables -t nat -A PREROUTING -p tcp --dport 25 -j REDIRECT --to-port 2525
sudo iptables -t nat -A PREROUTING -p tcp --dport 80 -j REDIRECT --to-port 8080
```

### Verifying or resetting the iptables redirect

The commands above add to `PREROUTING` -- they don't replace what's
already there, and nothing persists the result across a reboot (no
`iptables-persistent`/`netfilter-persistent` installed by default). Re-run
them after every setup attempt or reboot and you'll eventually end up with
duplicate/conflicting rules -- a "hot mess," per
[issue #1](https://github.com/woodie/lambada/issues/1).

Check what's actually there:

```bash
sudo iptables -t nat -L PREROUTING -n -v
```

Correct output has exactly one `REDIRECT` line per port:

```
Chain PREROUTING (policy ACCEPT 123K packets, 25M bytes)
 pkts bytes target     prot opt in     out     source        destination
    1    64 REDIRECT   6    --  *      *       0.0.0.0/0     0.0.0.0/0      tcp dpt:25 redir ports 2525
   33  2112 REDIRECT   6    --  *      *       0.0.0.0/0     0.0.0.0/0      tcp dpt:80 redir ports 8080
```

If it looks like a mess (duplicates, wrong ports, leftover rules from
testing), wipe `PREROUTING` and reapply cleanly:

```bash
sudo iptables -t nat -F PREROUTING
sudo iptables -t nat -A PREROUTING -p tcp --dport 25 -j REDIRECT --to-port 2525
sudo iptables -t nat -A PREROUTING -p tcp --dport 80 -j REDIRECT --to-port 8080
sudo netfilter-persistent save   # if installed -- otherwise this resets on reboot anyway
```
