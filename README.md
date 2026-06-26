# Lambada scan exporter

[![go.mod version](https://img.shields.io/github/go-mod/go-version/woodie/lambada)](https://github.com/woodie/lambada)
[![CI](https://github.com/woodie/lambada/actions/workflows/go.yml/badge.svg)](https://github.com/woodie/lambada/actions/workflows/go.yml)
[![Release](https://img.shields.io/github/v/release/woodie/lambada.svg)](https://github.com/woodie/lambada/releases/latest)
[![License](https://img.shields.io/github/license/woodie/lambada.svg)](LICENSE)

Two minimal Go services for a scanner/printer that requires an open relay to
e-mail out scans. Run them on a **Raspberry Pi** to receive scans by e-mail
and browse or download them from your home network.

<img width="697" height="358" alt="flow" src="https://github.com/user-attachments/assets/3844ed47-9741-4017-afd2-7c778b765d1a" />

## Why this exists

A lot of perfectly good scanners and printers end up in a landfill the
day their cloud-email feature stops working -- usually because the auth
method it depends on (an open relay, a now-revoked app password) quietly
breaks somewhere upstream, and the manufacturer has no reason to fix
firmware on hardware that old. If your printer can still email a scan
to *somewhere*, lambada plus an SMTP open relay it trusts (its own,
deliberately -- see Installation) is enough to keep it useful for years
past the cloud feature's death, on hardware as small as a Pi Zero.

If this is useful enough that someone wants it running on Windows
instead of a Mac, that's a good problem to have -- [open an
issue](https://github.com/woodie/zouk/issues).

## How it works

This repo builds two independent binaries that share the same
`attachments/` directory:

- **`lambada-mta`** -- a minimal SMTP server. The scanner emails scans to
  the Pi over SMTP; `lambada-mta` receives the message, decodes the
  attachment, and saves it with an epoch-based filename (e.g.
  `1779867473.pdf`) to `attachments/`. Files older than 24 hours are
  cleaned up on each incoming message.
- **`lambada-web`** -- serves a listing of those files over HTTP: a page
  with download links, human-readable file sizes, and "time ago"
  timestamps, plus a `GET /scans.json` API used by the
  [zouk](https://github.com/woodie/zouk) Mac client. This is a Go port of
  the [scandalous](https://github.com/woodie/scandalous) project's Sinatra
  web server.

The recommended way to get scans onto your Mac is `lambada-web`'s
listing page -- either through the [zouk](https://github.com/woodie/zouk)
client, or just a plain web browser pointed at the Pi (browsers flag
the download as unsafe over plain HTTP, so expect an extra "Keep"
click to confirm it anyway). Samba is also supported, as a **legacy
option** for anyone who'd rather mount a network share than use either
of those -- link `attachments/` to a Samba share (see Installation
below) and skip running `lambada-web` entirely, since the two services
don't depend on each other. It works, but it's noticeably slower and
clunkier on a Pi than the alternatives above; we kept it around for
people who want to try it, not because we'd recommend it.

<img width="292" height="181" alt="listing" src="https://github.com/user-attachments/assets/5c7a480d-249d-4637-ae91-e07db638f35b" />

## Installation

Make sure you have `git` and `go` installed on the Pi, then...

```bash
# Pull down the project
mkdir ~/workspace
cd ~/workspace
git clone git@github.com:woodie/lambada.git
cd lambada

# Optional, legacy: link the attachments folder to a Samba share instead of
# (or in addition to) browsing it through lambada-web. Most people should
# just use the zouk client and skip this.
ln -s /srv/samba/public attachments

# Fetch dependencies and build both binaries
go mod tidy
go build -o lambada-mta ./cmd/lambada-mta
go build -o lambada-web ./cmd/lambada-web

# Redirect port 25 -> 2525 and port 80 -> 8080 so both services can run as
# a non-root user
sudo iptables -t nat -A PREROUTING -p tcp --dport 25 -j REDIRECT --to-port 2525
sudo iptables -t nat -A PREROUTING -p tcp --dport 80 -j REDIRECT --to-port 8080

# Install and start both services
sudo cp service/lambada-mta.service service/lambada-web.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now lambada-mta lambada-web
sudo systemctl status lambada-mta lambada-web --no-pager

# Scan something and watch the logs
sudo tail -f /var/log/syslog
```

See [DEVELOPMENT.md](docs/DEVELOPMENT.md) for testing and configuration details.
