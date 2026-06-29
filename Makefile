PREFIX?=/usr/local
ATTACHMENTS_DIR?=/srv/lambada/attachments

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

.PHONY: install
install: build
	$(SUDO) $(MKDIR) $(PREFIX)/bin
	$(SUDO) $(CP) lambada-mta lambada-web $(PREFIX)/bin/
	$(SUDO) $(MKDIR) $(ATTACHMENTS_DIR)
	$(SUDO) $(CHOWN) $(OWNER):$(GROUP) $(ATTACHMENTS_DIR)

.PHONY: uninstall
uninstall:
	$(SUDO) $(RM) $(PREFIX)/bin/lambada-mta $(PREFIX)/bin/lambada-web

.PHONY: clean
clean:
	$(RM) lambada-mta lambada-web
