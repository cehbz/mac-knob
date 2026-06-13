# Build, sign, and install the spacekit binaries.
#
# Accessibility/Screen Recording grants are bound to a binary's code
# signature. Ad-hoc signatures change cdhash on every rebuild, which orphans
# the grant and makes macOS re-prompt. Signing with a stable self-signed
# identity (pulled from 1Password at build time, see scripts/sign.sh) keys the
# grant on identifier + certificate instead, so it survives rebuilds.
#
# One-time setup: scripts/gen-signing-cert.sh, store the .p12 in 1Password
# (see README "Stable signing").

CODESIGN_IDENTITY ?= spacekit
PREFIX ?= $(HOME)/bin
BINS := spaceswitch spacekeeper

# Per-machine signing references (vault/item names) live in a gitignored
# local file so the public Makefile stays generic. See README "Stable signing".
-include signing.local.mk

export CODESIGN_IDENTITY OP_P12_REF OP_P12PW_REF

.PHONY: all build sign install test clean restore-agent-install restore-agent-uninstall save-agent-install save-agent-uninstall

all: install

build:
	go build -o spaceswitch ./cmd/spaceswitch
	go build -o spacekeeper ./cmd/spacekeeper

sign: build
	./scripts/sign.sh $(BINS)

install: sign
	mkdir -p $(PREFIX)
	for b in $(BINS); do cp $$b $(PREFIX)/$$b; done
	@# Restart the daemon if loaded, so the running binary matches what was granted.
	@launchctl kickstart -k gui/$$(id -u)/bz.ceh.spaceswitch 2>/dev/null && echo "restarted spaceswitch daemon" || true
	@echo "installed to $(PREFIX)"

test:
	go test ./...

# Optional login agent that runs `spacekeeper restore` after a settle delay.
RESTORE_AGENT := bz.ceh.spacekeeper-restore
restore-agent-install:
	cp dist/$(RESTORE_AGENT).plist $(HOME)/Library/LaunchAgents/$(RESTORE_AGENT).plist
	launchctl bootout gui/$$(id -u)/$(RESTORE_AGENT) 2>/dev/null || true
	launchctl bootstrap gui/$$(id -u) $(HOME)/Library/LaunchAgents/$(RESTORE_AGENT).plist
	@echo "login restore enabled"

restore-agent-uninstall:
	launchctl bootout gui/$$(id -u)/$(RESTORE_AGENT) 2>/dev/null || true
	rm -f $(HOME)/Library/LaunchAgents/$(RESTORE_AGENT).plist
	@echo "login restore disabled"

# Optional agent that snapshots the layout into history every few minutes.
SAVE_AGENT := bz.ceh.spacekeeper-save
save-agent-install:
	cp dist/$(SAVE_AGENT).plist $(HOME)/Library/LaunchAgents/$(SAVE_AGENT).plist
	launchctl bootout gui/$$(id -u)/$(SAVE_AGENT) 2>/dev/null || true
	launchctl bootstrap gui/$$(id -u) $(HOME)/Library/LaunchAgents/$(SAVE_AGENT).plist
	@echo "periodic snapshots enabled"

save-agent-uninstall:
	launchctl bootout gui/$$(id -u)/$(SAVE_AGENT) 2>/dev/null || true
	rm -f $(HOME)/Library/LaunchAgents/$(SAVE_AGENT).plist
	@echo "periodic snapshots disabled"

clean:
	rm -f $(BINS)
