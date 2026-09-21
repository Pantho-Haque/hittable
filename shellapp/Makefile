# Reinstall the `hittable` command after making changes.
# Builds a native binary for this machine and installs it to ~/.local/bin plus
# wherever an existing `hittable` is found on PATH.
ARCH ?= $(shell uname -m | sed 's/x86_64/amd64/')
BIN  := $(HOME)/.local/bin/hittable

.PHONY: install build test uninstall

build:
	GOARCH=$(ARCH) go build -o hittable ./cmd/hittable

# Replace by unlink+copy, never overwrite in place: a running hittable keeps
# its old inode and macOS does not kill the new binary over a stale
# code-signature cache. Ad-hoc re-sign on macOS for the same reason.
install: build
	@mkdir -p $(dir $(BIN))
	@for p in $(BIN) $$(which -a hittable 2>/dev/null | sort -u); do \
		rm -f "$$p"; cp hittable "$$p"; \
		if command -v codesign >/dev/null 2>&1; then codesign -s - --force "$$p" 2>/dev/null; fi; \
	done; true
	@hash -r 2>/dev/null || true
	@echo "installed: $$(which hittable) ($(ARCH))"

test:
	go vet ./... && go test ./... -timeout 300s

uninstall:
	rm -f $(BIN) $(HOME)/go/bin/hittable
