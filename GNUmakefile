# Binary directory consumed by Terraform/OpenTofu dev_overrides. dev_overrides
# points at a directory holding an executable named terraform-provider-motherduck
# (no versioned plugin path), so `build` writes the binary into $(BINDIR).
BINDIR ?= $(CURDIR)/bin
BINARY := terraform-provider-motherduck

.PHONY: build install
build:
	go build -o $(BINDIR)/$(BINARY) .

install: build
