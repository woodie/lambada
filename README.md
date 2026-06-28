# Lambada scan server

[![go.mod version](https://img.shields.io/github/go-mod/go-version/woodie/lambada)](https://github.com/woodie/lambada)
[![CI](https://github.com/woodie/lambada/actions/workflows/go.yml/badge.svg)](https://github.com/woodie/lambada/actions/workflows/go.yml)
[![Release](https://img.shields.io/github/v/release/woodie/lambada.svg)](https://github.com/woodie/lambada/releases/latest)
[![License](https://img.shields.io/github/license/woodie/lambada.svg)](LICENSE)

Have an old Scanner/Printer that requires an open relay to e-mail out scans?
Now you can serve up scanned documents on your home network.

<img width="193" height="226" alt="printer" src="https://github.com/user-attachments/assets/a1d7f795-6e4b-43ca-91a9-1d915b28fedc" />
<img width="161" height="117" alt="piv1" src="https://github.com/user-attachments/assets/d4d1104a-7512-4310-a699-df8a36704b9b" /> &nbsp;
<img width="292" height="181" alt="listing" src="https://github.com/user-attachments/assets/5c7a480d-249d-4637-ae91-e07db638f35b" />
<br>
<br>
A lot of perfectly good scanners and printers end up in a landfill the
day their cloud-email feature stops working -- usually because the auth
method it depends on (an open relay, a now-revoked app password) quietly
breaks somewhere upstream, and the manufacturer has no reason to fix
firmware on hardware that old. If your printer can still email a scan
to *somewhere*, lambada plus an SMTP open relay it trusts (its own,
deliberately -- see Installation) is enough to keep it useful for years
past the cloud feature's death, on hardware as small as a Pi Zero.

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
clunkier on a Pi than the alternatives above.

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

# Redirect port 25 -> 2525 so lambada-mta can run as a non-root user.
# lambada-web doesn't need this -- it's reachable directly on 0.0.0.0:8080.
sudo iptables -t nat -A PREROUTING -p tcp --dport 25 -j REDIRECT --to-port 2525

# Install and start both services
sudo cp service/lambada-mta.service service/lambada-web.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now lambada-mta lambada-web
sudo systemctl status lambada-mta lambada-web --no-pager

# Scan something and watch the logs
sudo tail -f /var/log/syslog
```

At this point `lambada-web` is already reachable on the LAN at
`http://<pi>:8080/` -- nginx below is entirely optional.

### Optional: front lambada-web with nginx on port 80

Skip this unless you specifically want `lambada-web` reachable on plain
`http://<pi>/` (port 80) instead of `:8080`:

```bash
sudo apt-get update
sudo apt-get install -y nginx
sudo cp service/lambada-web.nginx.conf /etc/nginx/sites-available/lambada-web
sudo ln -sf /etc/nginx/sites-available/lambada-web /etc/nginx/sites-enabled/lambada-web
sudo rm -f /etc/nginx/sites-enabled/default
sudo nginx -t && sudo systemctl reload nginx

# Drop lambada-web to loopback-only now that nginx is fronting it
sudo systemctl edit lambada-web
#   [Service]
#   Environment=LAMBADA_WEB_LISTEN_ADDR=127.0.0.1:8080
sudo systemctl restart lambada-web
```

See [DEVELOPMENT.md](docs/DEVELOPMENT.md) for testing, configuration
details, and how to roll the nginx step back.
