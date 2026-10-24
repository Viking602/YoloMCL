SHELL := bash
.DEFAULT_GOAL := help
.PHONY: help build check fmt release verify publish-secrets

BUILD_ARGS ?=
CHECK_ARGS ?=
RELEASE_ARGS ?=
TAG ?=
export TAG

help:
	@printf '%s\n' \
	  'make build                         Build the launcher into .cache/' \
	  'make build BUILD_ARGS="--release"  Build without the UI inspector' \
	  'make check                         Format, architecture, static analysis and tests' \
	  'make check CHECK_ARGS="--cross"    Also compile all six targets' \
	  'make fmt                           Fix formatting and check imports' \
	  'make release TAG=v1.0.0            Build and sign a tagged release' \
	  'make verify TAG=v1.0.0             Verify all six release targets' \
	  'make publish-secrets               Configure GitHub Actions publisher secrets'

build:
	@bash scripts/build.sh $(BUILD_ARGS)

check:
	@bash scripts/check.sh $(CHECK_ARGS)

fmt:
	@bash scripts/check.sh --fix --format-only

release:
	@bash scripts/release.sh $(RELEASE_ARGS)

verify:
	@bash scripts/release.sh --verify $(RELEASE_ARGS)

publish-secrets:
	@bash scripts/publish-secrets.sh
