# Build, sign, and install the mac-knob binaries.
#
# Accessibility/Screen Recording grants are bound to a binary's code
# signature. Ad-hoc signatures change cdhash on every rebuild, which orphans
# the grant and makes macOS re-prompt. Signing with a stable self-signed
# identity (pulled from 1Password at build time, see scripts/sign.sh) keys the
# grant on identifier + certificate instead, so it survives rebuilds.
#
# One-time setup: scripts/gen-signing-cert.sh, store the .p12 in 1Password
# (see README "Stable signing").

CODESIGN_IDENTITY ?= mac-knob
PREFIX ?= $(HOME)/bin
BINS := spaceswitch spacekeeper

# Per-machine signing references (vault/item names) live in a gitignored
# local file so the public Makefile stays generic. See README "Stable signing".
-include signing.local.mk

export CODESIGN_IDENTITY OP_P12_REF OP_P12PW_REF

.PHONY: all build sign install test clean

all: install

build:
	go build -o spaceswitch ./cmd/spaceswitch
	go build -o spacekeeper ./cmd/spacekeeper

sign: build
	./scripts/sign.sh $(BINS)

install: sign
	mkdir -p $(PREFIX)
	for b in $(BINS); do cp $$b $(PREFIX)/$$b; done
	@echo "installed to $(PREFIX)"

test:
	go test ./...

clean:
	rm -f $(BINS)
