# Lambada scan server

[![go.mod version](https://img.shields.io/github/go-mod/go-version/woodie/lambada)](https://github.com/woodie/lambada)
[![CI](https://github.com/woodie/lambada/actions/workflows/ci.yml/badge.svg)](https://github.com/woodie/lambada/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/woodie/lambada.svg)](https://github.com/woodie/lambada/releases/latest)
[![License](https://img.shields.io/github/license/woodie/lambada.svg)](LICENSE)

Have an old Scanner/Printer that requires an open relay to e-mail out scans?
Now you can serve up scanned documents on your home network.

<img width="193" height="226" alt="printer" 
     src="https://github.com/user-attachments/assets/a1d7f795-6e4b-43ca-91a9-1d915b28fedc" /><img
     width="161" height="117" alt="piv1" 
     src="https://github.com/user-attachments/assets/d4d1104a-7512-4310-a699-df8a36704b9b" />&nbsp;&nbsp;<img
     width="345" height="188" alt="listing" src="docs/listing.png" />
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

Two pieces share one folder, `/srv/lambada/attachments`:

- **`lambada`** -- a minimal SMTP server, the one thing this repo builds.
  The scanner emails a scan to the Pi; `lambada` decodes the attachment and
  saves it with an epoch-based name (e.g. `1779867473.pdf`). Each time a
  scan arrives, scans older than 24 hours are deleted. Only files named like
  that (ten digits, `.pdf`) are ever deleted; anything else stays until
  someone deletes it.
- **nginx** -- serves the folder over WebDAV at `http://<pi>/`, using
  [`service/lambada.nginx.conf`](service/lambada.nginx.conf). A browser
  gets a page listing each file like Finder's list view (name, size, age),
  with download and delete. WebDAV clients get the folder itself: list, download, upload
  and delete.

Ways to get at the scans:

- **A browser:** `http://<pi>/`. Download, delete, and upload with the
  "upload files" button or by dropping files on the page. Browsers flag
  downloads over plain HTTP, so expect an extra "Keep" click.
- **Finder:** Go → Connect to Server…, `http://<pi>/`, connect as Guest.
  Drag a file into the window to upload it, for example to move files to or
  from another computer on your network.
- **[zouk](https://github.com/woodie/zouk)** (Mac) or
  **[huck](https://github.com/woodie/huck)**, or any other WebDAV client.

Uploads go in the top-level folder only (no subfolders), up to 25 MB each,
and can't use a scan's name.

## Installation

On the Pi, with `go` installed:

```bash
sudo apt-get update
sudo apt-get install -y git nginx libnginx-mod-http-dav-ext iptables-persistent

mkdir -p ~/workspace && cd ~/workspace
git clone https://github.com/woodie/lambada.git
cd lambada

# Build lambada, install it to /usr/local/bin, and set up /srv/lambada
# (prompts for sudo only if those locations aren't already writable)
make install

# Redirect port 25 -> 2525 so lambada can run as a non-root user
sudo iptables -t nat -A PREROUTING -p tcp --dport 25 -j REDIRECT --to-port 2525
sudo netfilter-persistent save

# Start lambada (edit User= first if you don't log in as pi)
sudo cp service/lambada.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now lambada
sudo systemctl status lambada --no-pager

# Serve the folder with nginx on port 80
sudo cp service/lambada.nginx.conf /etc/nginx/sites-available/lambada
sudo ln -sf /etc/nginx/sites-available/lambada /etc/nginx/sites-enabled/lambada
sudo rm -f /etc/nginx/sites-enabled/default
sudo nginx -t && sudo systemctl reload nginx
```

Point the scanner's email settings at the Pi, scan something, and open
`http://<pi>/`. See [DEVELOPMENT.md](docs/DEVELOPMENT.md) for updating,
testing and troubleshooting.
