# Configuration

## Dependencies

- [`github.com/emersion/go-smtp`](https://github.com/emersion/go-smtp) — SMTP server primitives

## Quick Start

```bash
# Fetch dependencies
go mod tidy

# Run (listens on :2525 by default)
go run main.go
```

## Build

```
install github.com/emersion/go-smtp@latest
go build
```

Attachments land in `./attachments/` (created automatically).

## Testing with curl / swaks

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

## Configuration

| Constant          | Default        | Description                    |
|-------------------|----------------|--------------------------------|
| `attachmentDir`   | `./attachments`| Where attachments are written  |
| `listenAddr`      | `0.0.0.0:2525` | TCP address to listen on       |
| `MaxMessageBytes` | `25 MB`        | Maximum accepted message size  |

## Logging
```
2026/05/27 00:35:50 SMTP open relay listening on 0.0.0.0:2525 (attachments -> ./attachments)
2026/05/27 00:37:51 New connection from myprinter
2026/05/27 00:37:51 Cleanup removed old file: attachments/.DS_Store
2026/05/27 00:37:51 Cleanup removed old file: attachments/1779741215.pdf
2026/05/27 00:37:51 Cleanup removed old file: attachments/1779741236.pdf
2026/05/27 00:37:51 Cleanup removed old file: attachments/1779744713.pdf
2026/05/27 00:37:53 Receiving message
2026/05/27 00:37:56 Write 268809 bytes to attachments/1779867473.pdf
2026/05/27 00:37:56 Saved attachment: attachments/1779867473.pdf
```
