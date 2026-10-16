# Lambada scan server

[![go.mod version](https://img.shields.io/github/go-mod/go-version/woodie/lambada)](https://github.com/woodie/lambada)
[![CI](https://github.com/woodie/lambada/actions/workflows/ci.yml/badge.svg)](https://github.com/woodie/lambada/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/woodie/lambada.svg)](https://github.com/woodie/lambada/releases/latest)
[![License](https://img.shields.io/github/license/woodie/lambada.svg)](LICENSE)

Have an old scanner/printer that can only send scans by e-mail? lambada
gives it somewhere to send them, and serves the scans on your home network.

<img width="193" height="226" alt="A scanner" src="docs/hp_scanner.png" /><img
     width="161" height="117" alt="A Raspberry Pi" src="docs/raspberry_pi.png" />&nbsp;&nbsp;<img
     width="345" height="188" alt="The file list in a browser" src="docs/listing.png" />
<br>
<br>
Plenty of good scanners and printers get thrown out once scan-to-e-mail
stops working. Mail services now want a secure sign-in that old firmware
can't do, and manufacturers don't update firmware that old. lambada accepts
mail without a sign-in, on your home network only, so a scanner that can
still send a scan by e-mail stays useful for years. It runs fine on an old
Raspberry Pi, even a Pi Zero.

## How it works

lambada has two parts that share one folder, `/srv/lambada/attachments`:

- **`lambada`**, the one program this repo builds, is a small mail server.
  The scanner e-mails a scan to it, and lambada saves the attached file.
  Each time a scan arrives, scans older than a day are deleted. Files you
  upload stay until someone deletes them.
- **nginx** serves the folder over WebDAV, using
  [`service/lambada.nginx.conf`](service/lambada.nginx.conf). A browser gets
  a page listing every file with its name, size and age, where you can
  download, upload and delete. Finder, File Explorer and other WebDAV apps
  can open the folder itself.

See [USAGE.md](docs/USAGE.md) for an overview of WebDAV clients to fetch scans.

## Installation

These steps are for Raspberry Pi OS, or another Debian-based Linux, with
Go 1.26 or newer installed. Run them on the server:

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

## Scanner configuration

Open the scanner's own settings page (usually its IP address in a browser)
and find its e-mail or SMTP settings:

- Turn on sending e-mail directly from the scanner.
- Set the **SMTP Gateway** (or SMTP server) to your server's IP address.
- Set the **SMTP Port** to `25`.
- Leave login, SSL and TLS off. lambada accepts mail from anyone on your
  network, so no account is needed.
- The sender doesn't matter: the "From" address, display name and default
  subject can be anything.
- Add a contact to scan to, such as "Scans" at `scans@example.com`. The
  address is never used: the e-mail is never delivered anywhere, because
  lambada saves the attached scan and throws the rest of the message away.
  So any address will do, and naming the contact after the server (for
  example "Scans: 10.0.1.111") makes it easy to pick on the scanner's
  screen.

Scan something to e-mail, then open `http://<server>/` to see it. See
[DEVELOPMENT.md](docs/DEVELOPMENT.md) for updating, testing and
troubleshooting.
