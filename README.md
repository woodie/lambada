# SMTP Attachment Server

A minimal SMTP server written in Go that accepts incoming email and saves
all attachments to the local filesystem.

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
go build -buildvcs=false
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
| `listenAddr`      | `:2525`        | TCP address to listen on       |
| `MaxMessageBytes` | 25 MB          | Maximum accepted message size  |

## Adding TLS (Production)

```go
import "crypto/tls"

cert, _ := tls.LoadX509KeyPair("cert.pem", "key.pem")
s.TLSConfig = &tls.Config{Certificates: []tls.Certificate{cert}}
s.AllowInsecureAuth = false
// Use s.ListenAndServeTLS() instead of s.ListenAndServe()
```

## Adding Authentication

Replace the body of `AuthPlain` with real credential checks:

```go
func (s *Session) AuthPlain(username, password string) error {
    if username != "user" || password != "secret" {
        return errors.New("invalid credentials")
    }
    return nil
}
```
