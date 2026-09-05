SHELL      := /bin/bash
BIN_DIR    := bin
# node_modules is pruned the same way dot-directories (.worktrees, .git) are
# implicitly skipped by go tooling: web/node_modules ships stray .go files
# from transitive npm deps that Go tooling must never see.
GO_FILES   := find . -name node_modules -prune -o -type f -name '*.go' -print
GO_SRC     := $(shell $(GO_FILES) 2>/dev/null) go.mod

# Where forgec puts the fetched corpus and the IR compiled from it. Never
# committed — the scripts are GPL-3.0.
CARDS_DIR  ?= .cards
FORGE_REF  ?= master

.PHONY: help
help:
	@echo "mtgcore targets:"
	@echo "  make build          — compile forgec and mtgsim"
	@echo "  make fetch-cards    — fetch Forge cardsfolder at FORGE_REF into $(CARDS_DIR)"
	@echo "  make compile-cards  — compile the fetched corpus into the IR cache"
	@echo "  make report         — print card coverage against implemented primitives"
	@echo "  make sim            — build mtgsim and play 20 verified 4-seat games"
	@echo "  make gentypes       — regenerate web/src/protocol.ts from package protocol"
	@echo "  make web            — npm ci and build the spectator client into cmd/gorged/webdist"
	@echo "  make web-dev        — run the Vite dev server for web/"
	@echo "  make test-web       — run web/'s Vitest suite"
	@echo "  make lint-web       — svelte-check and eslint over web/"
	@echo "  make test lint cover"
	@echo "  NOTE: make test-web / npm test needs Node >=22 (vitest 5); see web/README.md"

.PHONY: build
build: $(BIN_DIR)/forgec $(BIN_DIR)/mtgsim

$(BIN_DIR)/forgec: $(GO_SRC)
	@mkdir -p $(BIN_DIR)
	CGO_ENABLED=0 go build -o $@ ./cmd/forgec

$(BIN_DIR)/mtgsim: $(GO_SRC)
	@mkdir -p $(BIN_DIR)
	CGO_ENABLED=0 go build -o $@ ./cmd/mtgsim

.PHONY: sim
sim: $(BIN_DIR)/mtgsim
	$(BIN_DIR)/mtgsim -seats 4 -games 20 -verify

.PHONY: fetch-cards
fetch-cards: $(BIN_DIR)/forgec
	$(BIN_DIR)/forgec fetch -dir $(CARDS_DIR) -ref $(FORGE_REF)

.PHONY: compile-cards
compile-cards: $(BIN_DIR)/forgec
	$(BIN_DIR)/forgec compile -dir $(CARDS_DIR)

.PHONY: report
report: $(BIN_DIR)/forgec
	$(BIN_DIR)/forgec report -dir $(CARDS_DIR)

# gentypes regenerates web/src/protocol.ts from package protocol's structs
# (internal/tsgen does the reflection); lint below runs it with -check so a
# stale committed file fails the build instead of drifting from the server.
.PHONY: gentypes
gentypes:
	go run ./cmd/gentypes -o web/src/protocol.ts

.PHONY: test
test:
	go test ./...

COVER_OUT  ?= coverage.out
COVER_HTML ?= coverage.html
.PHONY: cover cover-html
cover:
	go test -covermode=atomic -coverprofile=$(COVER_OUT) -coverpkg=./... ./...
	@go tool cover -func=$(COVER_OUT) | tail -1

cover-html: cover
	go tool cover -html=$(COVER_OUT) -o $(COVER_HTML)

.PHONY: tidy
tidy:
	go mod tidy

# npm's own install fingerprint; a clean checkout has no web/node_modules,
# so lint-web/test-web/web (and web-dev) install first instead of dying with
# "eslint: not found" / "vite: not found".
web/node_modules/.package-lock.json: web/package-lock.json
	cd web && npm ci

.PHONY: web web-dev test-web lint-web
web: web/node_modules/.package-lock.json
	cd web && npm run build

web-dev: web/node_modules/.package-lock.json
	cd web && npm run dev

test-web: web/node_modules/.package-lock.json
	cd web && npm run test

lint-web: web/node_modules/.package-lock.json
	cd web && npm run check && npm run lint

.PHONY: lint
lint: lint-web
	@out=$$(gofmt -l $$($(GO_FILES)) 2>&1); \
		if [ -n "$$out" ]; then \
			echo "gofmt: files need formatting:"; echo "$$out"; exit 1; \
		fi
	go vet ./...
	go run ./cmd/gentypes -check

.PHONY: clean
clean:
	rm -rf $(BIN_DIR) $(COVER_OUT) $(COVER_HTML)

# clean-cards is destructive and separate from clean: refetching the corpus is
# a multi-minute network operation.
.PHONY: clean-cards
clean-cards:
	rm -rf $(CARDS_DIR)
