PREFIX?=/usr/local
ATTACHMENTS_DIR?=/srv/lambada/attachments
WWW_DIR?=/srv/lambada/www

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
# (not root, even when $(SUDO) was needed to create it), since lambada-mta/
# -web run as whichever user `User=` in the service files names, which is
# expected to be the same account that ran `make install`. See
# docs/DEVELOPMENT.md's "systemd Services" section.
OWNER:=$(shell id -un)
GROUP:=$(shell id -gn)

.PHONY: all
all: build

.PHONY: build
build:
	$(GO) build -o lambada-mta ./cmd/lambada-mta
	$(GO) build -o lambada-web ./cmd/lambada-web

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

.PHONY: install
install: build
	$(SUDO) $(MKDIR) $(PREFIX)/bin
	$(SUDO) $(CP) lambada-mta lambada-web $(PREFIX)/bin/
	$(SUDO) $(MKDIR) $(ATTACHMENTS_DIR)
	$(SUDO) $(CHOWN) $(OWNER):$(GROUP) $(ATTACHMENTS_DIR)

# The scan page nginx serves at / (service/lambada.nginx.conf). Plain files,
# so this only copies www/. The folder is chowned like ATTACHMENTS_DIR, so
# updating the page later needs no sudo.
.PHONY: install-www
install-www:
	$(SUDO) $(MKDIR) $(WWW_DIR)
	$(SUDO) $(CHOWN) $(OWNER):$(GROUP) $(WWW_DIR)
	$(CP) -R www/. $(WWW_DIR)/

# Refreshes www/lib/ from node_modules after `npm install`. The copies are
# committed, so installing on the Pi needs no npm.
.PHONY: vendor
vendor:
	$(MKDIR) www/lib/humane
	$(CP) node_modules/humane-ts/dist/*.js node_modules/humane-ts/LICENSE www/lib/humane/

.PHONY: uninstall
uninstall:
	$(SUDO) $(RM) $(PREFIX)/bin/lambada-mta $(PREFIX)/bin/lambada-web

.PHONY: clean
clean:
	$(RM) lambada-mta lambada-web
