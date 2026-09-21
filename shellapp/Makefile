# Reinstall the `hittable` command after making changes.
# Builds a native binary for this machine and copies it wherever an existing
# `hittable` is found on PATH (plus ~/.local/bin as the default).
ARCH ?= $(shell uname -m | sed 's/x86_64/amd64/')
BIN  := $(HOME)/.local/bin/hittable

.PHONY: install build test uninstall

build:
	GOARCH=$(ARCH) go build -o hittable ./cmd/hittable

install: build
	mkdir -p $(dir $(BIN))
	cp hittable $(BIN)
	@for p in $$(which -a hittable 2>/dev/null | sort -u); do \
		[ "$$p" = "$(BIN)" ] || cp hittable "$$p"; \
	done
	@hash -r 2>/dev/null || true
	@echo "installed: $$(which hittable) ($(ARCH))"

test:
	go vet ./... && go test ./... -timeout 120s

uninstall:
	rm -f $(BIN) $(HOME)/go/bin/hittable
