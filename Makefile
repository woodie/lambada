PREFIX?=/usr/local
ATTACHMENTS_DIR?=/srv/lambada/attachments
PUBLIC_DIR?=/srv/lambada/public

CP=/bin/cp -f
MKDIR=/bin/mkdir -p
RM=/bin/rm -f
CHOWN=/bin/chown
GO?=go

# Same pattern as xctidy/zouk's Makefiles: ask for sudo once, only for
# install/uninstall, only if $(PREFIX)/bin isn't already writable -- build
# and test never touch anything outside the checkout, so they never need it.
# Reused below for ATTACHMENTS_DIR too: on a fresh system both $(PREFIX)/bin
# and /srv are root-owned, so this one check covers both in the common case.
# If your system happens to differ, just rerun the failing line with sudo.
SUDO:=$(shell d="$(PREFIX)/bin"; while [ ! -d "$$d" ] && [ "$$d" != "/" ]; do d=$$(dirname "$$d"); done; test -w "$$d" && echo "" || echo "sudo")

# Who `make install` is run as -- ATTACHMENTS_DIR gets chowned to this user
# (not root, even when $(SUDO) was needed to create it), since lambada runs as
# whichever user `User=` in service/lambada.service names, which is expected
# to be the same account that ran `make install`.
OWNER:=$(shell id -un)
GROUP:=$(shell id -gn)

.PHONY: all
all: build

.PHONY: build
build:
	$(GO) build -o lambada .

.PHONY: test
test:
	$(GO) test ./...

# Runs spec/webdav against service/lambada.nginx.conf in a Debian container
# with the Pi's nginx packages, so no local nginx is needed. Needs Docker
# (Docker Desktop, OrbStack or Colima). node_modules stays inside the container.
.PHONY: test-webdav
test-webdav:
	docker build -q -t lambada-webdav-test spec/webdav
	docker run --rm -v "$(CURDIR)":/lambada -v /lambada/node_modules -v lambada-webdav-go:/root/go lambada-webdav-test

# Runs lambada's web side in Docker (Dockerfile), set up like the Pi: the scan
# page and WebDAV at http://localhost:8080/, scans from ./scans-dev. public/ is
# mounted live, so page edits show on reload. Ctrl-C stops it. No MTA.
# Needs Docker (Docker Desktop, OrbStack or Colima).
# Adds a few test PDFs to ./scans-dev, dated minutes to days ago.
.PHONY: seed
seed:
	node scripts/seed-scans.mjs scans-dev

.PHONY: docker-run
docker-run:
	$(MKDIR) scans-dev
	docker build -t lambada-nginx .
	docker run --rm -it --init -p 8080:80 \
		-v "$(CURDIR)/scans-dev:/srv/lambada/attachments" \
		-v "$(CURDIR)/public:/srv/lambada/public:ro" \
		lambada-nginx

# Installs the binary, the scans folder and the scan page. The scans folder is
# opened to other users (o+w) so nginx's www-data user can upload and delete.
.PHONY: install
install: build install-public
	$(SUDO) $(MKDIR) $(PREFIX)/bin
	$(SUDO) $(CP) lambada $(PREFIX)/bin/
	$(SUDO) $(MKDIR) $(ATTACHMENTS_DIR)
	$(SUDO) $(CHOWN) $(OWNER):$(GROUP) $(ATTACHMENTS_DIR)
	chmod o+w $(ATTACHMENTS_DIR)

# The scan page nginx serves at / (service/lambada.nginx.conf). Plain files,
# so this only copies public/. The folder is chowned like ATTACHMENTS_DIR, so
# updating the page later needs no sudo.
.PHONY: install-public
install-public:
	$(SUDO) $(MKDIR) $(PUBLIC_DIR)
	$(SUDO) $(CHOWN) $(OWNER):$(GROUP) $(PUBLIC_DIR)
	$(CP) -R public/. $(PUBLIC_DIR)/

.PHONY: uninstall
uninstall:
	$(SUDO) $(RM) $(PREFIX)/bin/lambada

.PHONY: clean
clean:
	$(RM) lambada
