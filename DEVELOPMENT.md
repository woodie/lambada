# Development

## Dependencies

- [`github.com/emersion/go-smtp`](https://github.com/emersion/go-smtp) — SMTP server primitives
- [`github.com/onsi/ginkgo/v2`](https://github.com/onsi/ginkgo) — BDD test framework
- [`github.com/onsi/gomega`](https://github.com/onsi/gomega) — matcher library for Ginkgo

## Quick Start

```bash
# Fetch dependencies
go mod tidy

# Build and run (listens on 0.0.0.0:2525)
go build
./lambada
```

Attachments land in `./attachments/` (created automatically).

## Configuration

| Variable          | Default         | Description                    |
|-------------------|-----------------|--------------------------------|
| `attachmentDir`   | `./attachments` | Where attachments are written  |
| `listenAddr`      | `0.0.0.0:2525`  | TCP address to listen on       |
| `maxFileAge`      | `24h`           | How long to retain attachments |
| `MaxMessageBytes` | `25 MB`         | Maximum accepted message size  |

## Running Tests

```bash
# Install the Ginkgo CLI (first time only)
go install github.com/onsi/ginkgo/v2/ginkgo@latest

# Make sure ~/go/bin is on your PATH
export PATH="$PATH:$(go env GOPATH)/bin"

# Run tests with the standard Go runner
go test -v ./...

# Or run tests with the Ginkgo CLI
ginkgo -v
```

## Testing with swaks

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
```

## systemd Service

The service file lives at `service/lambada.service`. To install on Raspberry Pi:

```bash
sudo cp service/lambada.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable lambada
sudo systemctl start lambada
sudo systemctl status lambada
```

Port 25 is redirected to 2525 via iptables so the service can run as a non-root user:

```bash
sudo iptables -t nat -A PREROUTING -p tcp --dport 25 -j REDIRECT --to-port 2525
```
