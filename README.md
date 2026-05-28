# Lambada scan exporter

A minimal SMTP server (written in Go) that accepts incoming email and saves attachments.
Run this on a **Raspberry Pi** along with **Samba** to share scans from a scanner that requires an open relay to email scans.

<img width="697" height="358" alt="flow" src="https://github.com/user-attachments/assets/3844ed47-9741-4017-afd2-7c778b765d1a" />

## How it works

The scanner emails scans to the Pi over SMTP. Lambada receives the message, decodes the attachment, and saves it with an epoch-based filename (e.g. `1779867473.pdf`) to a folder shared over the network via Samba. Files older than 24 hours are automatically cleaned up on each incoming message.

## Installation

Make sure you have `git` and `go` installed on the Pi, then...

```bash
# Pull down the project
mkdir ~/workspace
cd ~/workspace
git clone git@github.com:woodie/lambada.git
cd lambada

# Link the attachments folder to Samba's public share
ln -s /srv/samba/public attachments

# Fetch dependencies and build
go mod tidy
go build

# Redirect port 25 to 2525 so the service can run as a non-root user
sudo iptables -t nat -A PREROUTING -p tcp --dport 25 -j REDIRECT --to-port 2525

# Install and start the service
sudo cp service/lambada.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable lambada
sudo systemctl start lambada
sudo systemctl status lambada

# Scan something and watch the logs
sudo tail -f /var/log/syslog
```

See [DEVELOPMENT.md](DEVELOPMENT.md) for testing and configuration details.
