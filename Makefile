.PHONY: build test benchmark lint install plugin-test demo release release-check workflow-lint
PREFIX ?= $(HOME)/.local
VERSION ?= $(shell cat VERSION)
build:
	go build -trimpath -ldflags '-s -w -X main.version=$(VERSION)' -o bin/tokenslim ./cmd/tokenslim
	mkdir -p integrations/codex/tokenslim/bin
	cp bin/tokenslim integrations/codex/tokenslim/bin/tokenslim
test:
	go test -race ./...
benchmark:
	go test ./internal/compressor ./internal/engine -run '^$$' -bench . -benchmem
lint:
	go vet ./...
	test -z "$$(gofmt -l cmd internal)"
install: build
	install -d "$(PREFIX)/bin"
	install -m 755 bin/tokenslim "$(PREFIX)/bin/tokenslim"
plugin-test: build
	go test ./internal/hook -v
	python3 scenarios/run.py --check
demo: build
	python3 scenarios/run.py --check
release:
	VERSION="$(VERSION)" sh scripts/release.sh

release-check: release
	python3 scripts/validate-release.py --packages
workflow-lint:
	go run github.com/rhysd/actionlint/cmd/actionlint@v1.7.7 -shellcheck= .github/workflows/*.yml
