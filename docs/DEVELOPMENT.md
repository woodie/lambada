# Development

## Layout

```
*.go                Go source for lambada, the SMTP server (package main)
src/main.ts         the scan page's code (TypeScript), built by `npm run build`
public/             what nginx serves at /: index.html, style.css, and main.js,
                    the minified build of src/main.ts (committed)
service/            lambada.service (systemd) and lambada.nginx.conf
spec/javascript/    scan page tests (Vitest, jsdom)
spec/webdav/        tests of the real nginx config plus a real lambada
scripts/            dev helpers: `make seed` and `npm run serve`
```

`lambada` defaults to `./attachments` (created automatically), so a plain
`make build && ./lambada` from the repo root works with no setup. Under
systemd, `LAMBADA_ATTACHMENTS_DIR=/srv/lambada/attachments` points it at the
folder nginx serves.

## Configuration

| Setting           | Default         | Notes                                     |
|-------------------|-----------------|-------------------------------------------|
| `attachmentDir`   | `./attachments` | Override with `LAMBADA_ATTACHMENTS_DIR`   |
| `listenAddr`      | `0.0.0.0:2525`  | Port 25 is redirected here by iptables    |
| `maxFileAge`      | `24h`           | Applies to scans only (see below)         |
| `MaxMessageBytes` | `25 MB`         | Matches nginx's `client_max_body_size`    |

Everything but the folder is a package-level `var` in `main.go` or
`attachments.go`; edit and rebuild to change it. `LOG_QUIET`, set to any
value, silences logging (used by `npm run check`).

Cleanup runs each time a scan arrives and deletes files older than
`maxFileAge` whose names match `^[0-9]{10}\.pdf$`, the names lambada gives
scans. Everything else in the folder (uploads, Finder's hidden files,
folders) is left alone. nginx refuses uploads with a scan's name, so an
upload is never cleaned up and never replaces a scan.

## WebDAV (nginx)

`service/lambada.nginx.conf` serves the folder at the root, with no login:

- `PROPFIND` (`Depth: 1`) lists it: name, size and modified date per file
- `GET` downloads, `PUT` uploads, `DELETE` removes a file
- `LOCK`/`UNLOCK`, so Finder treats the folder as writable
- refused (403): deleting the folder itself, uploading into a subfolder or
  over a scan's name, creating folders, copying and moving, and changing the
  page's files
- a browser's plain `GET /` gets the scan page from `/srv/lambada/public/`,
  served at `/.lambada/` (hidden, so never listed)

nginx runs as `www-data`, so the folder needs to be writable by other users;
`make install` does `chmod o+w` on it. Uploaded files are created readable
by everyone.

Check the listing on the Pi:

```bash
curl -X PROPFIND -H "Depth: 1" http://localhost/
```

## Linting and testing

```bash
npm install            # first time only

npm run check          # eslint, tsc, vitest, golangci-lint, go test, html-validate
npm run build          # src/main.ts -> public/main.js
npm run test           # vitest, then go test rendered by gorderly
npm run lint-js        # eslint --fix (neostandard, see eslint.config.js)
npm run typecheck      # tsc --noEmit
npm run test-js        # vitest with coverage (src/ must stay at 100%)
npm run lint-go        # golangci-lint run --fix
npm run test-go        # go test -v ./... | gorderly -fv
npm run lint-html      # html-validate public/
make test-webdav       # the nginx config, in Docker (see below)
```

Go needs `golangci-lint` (`brew install golangci-lint`) and `gorderly`
(`go install github.com/woodie/gorderly@latest`, with `$(go env GOPATH)/bin`
on your `PATH`). Go tests use [`spec`](https://github.com/woodie/spec) and
[`expect`](https://github.com/woodie/expect) for an RSpec-style
`describe`/`context`/`it` tree.

`node_modules` holds platform-specific binaries. If you see "Cannot find
module @rollup/rollup-\<platform\>", run `rm -rf node_modules
package-lock.json && npm install`.

### The WebDAV tests

`spec/webdav/webdav.spec.js` runs the real `service/lambada.nginx.conf` (only
its paths and port swapped) plus a real `lambada` against a temp folder:
listing, download, upload, delete, locking, every refused request, Finder's
hidden files, and a scan emailed over SMTP, including which files cleanup
removes. `spec/webdav/fixtures/propfind-pi.xml` is a real listing recorded
from the Pi.

On a Mac, `make test-webdav` runs them in a Debian container with the Pi's
nginx packages (Docker Desktop). On Linux with `nginx` and
`libnginx-mod-http-dav-ext` installed, `npm run test-webdav` runs them
directly, as CI does.

## Working on the scan page

The page is plain TypeScript with no framework: `src/main.ts` reads the
WebDAV listing and renders it like a Finder list (name, size, age, plus a
delete button), and uploads with `PUT`, from the "upload files" button or
files dropped on the page. nginx sends the page's files with
`Cache-Control: no-cache`, so browsers pick up an update on the next load. It checks nginx's two upload rules first (no
scan names, 25 MB at most) so it can say why, and asks before replacing a
file with the same name. It uses
[humane-ts](https://github.com/woodie/humane-ts) for sizes and ages. Vite
bundles it, with humane-ts, into one minified IIFE, `public/main.js`, and
`index.html` calls `Lambada.start()`. After changing `src/` or updating
humane-ts, run `npm run build` and commit `public/main.js`; CI fails if it's
out of date. The Pi only ever gets the built file, so it never needs npm.

Icons are [Bootstrap Icons](https://icons.getbootstrap.com) (MIT, license in
`public/bootstrap-icons-LICENSE.txt`): `src/icons.ts` imports the few SVGs
the page uses (file types and the trash can), and Vite inlines them into
`main.js`, so there's no icon font to download. To show a new file type,
import its `filetype-*.svg` there and add it to the table.

```bash
make seed        # a few test PDFs in ./scans-dev, only if it's empty
make docker-run  # seeds, then the Pi's nginx setup in Docker at http://localhost:8080/
npm run serve    # or a small Node stand-in, no Docker, at http://localhost:8000/
```

`make docker-run` mounts `public/` live, so a rebuild (`npm run build`)
shows on reload.

## Testing lambada with swaks

```bash
brew install swaks   # or: apt install swaks
./lambada &
swaks --server localhost:2525 --from sender@example.com --to rcpt@example.com \
  --header "Subject: Test" --attach @/path/to/file.pdf
```

## Updating the Pi

```bash
cd ~/workspace/lambada
git pull
make install
sudo systemctl restart lambada
# only if service/lambada.nginx.conf changed:
sudo cp service/lambada.nginx.conf /etc/nginx/sites-available/lambada
sudo nginx -t && sudo systemctl reload nginx
```

### Upgrading from lambada-mta and lambada-web (2.x)

Before 3.0, this repo built two services. To move a Pi over, after
`git pull` and `make install`:

```bash
sudo systemctl disable --now lambada-mta lambada-web
sudo rm -f /etc/systemd/system/lambada-mta.service /etc/systemd/system/lambada-web.service
sudo rm -f /usr/local/bin/lambada-mta /usr/local/bin/lambada-web
sudo cp service/lambada.service /etc/systemd/system/   # edit User= if needed
sudo systemctl daemon-reload
sudo systemctl enable --now lambada
sudo cp service/lambada.nginx.conf /etc/nginx/sites-available/lambada
sudo nginx -t && sudo systemctl reload nginx
```

The old web server is still in git, at the `lambada-web-final` tag.

## systemd

`service/lambada.service` is written for the default `pi` user. If you log
in as someone else, edit `User=` in the installed copy (it should be the
account that ran `make install`, which owns the folder). Don't commit that
change.

`systemctl status` pages through `less`, which over SSH looks like a hung
shell; use `--no-pager`, or press `q`.

View the logs with `journalctl -u lambada -f` (or `sudo tail -f
/var/log/syslog`):

```
2026/05/27 00:35:50 SMTP open relay listening on 0.0.0.0:2525
2026/05/27 00:37:51 New connection from myprinter
2026/05/27 00:37:51 Cleanup removed old file: /srv/lambada/attachments/1779741215.pdf
2026/05/27 00:37:53 Receiving message
2026/05/27 00:37:56 Write 268809 bytes to /srv/lambada/attachments/1779867473.pdf
2026/05/27 00:37:56 Saved attachment: /srv/lambada/attachments/1779867473.pdf
```

## The port 25 redirect

`iptables -A` adds a rule each time it runs, and nothing keeps it across a
reboot unless it's saved (`netfilter-persistent save`, from
`iptables-persistent`). Check what's there:

```bash
sudo iptables -t nat -L PREROUTING -n -v
```

There should be exactly one `REDIRECT` line, `tcp dpt:25 redir ports 2525`.
If there are duplicates or leftovers (such as the old `dpt:80 redir ports
8080` from before nginx), reset it:

```bash
sudo iptables -t nat -F PREROUTING
sudo iptables -t nat -A PREROUTING -p tcp --dport 25 -j REDIRECT --to-port 2525
sudo netfilter-persistent save
```
