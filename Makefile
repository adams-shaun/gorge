SHELL      := /bin/bash
BIN_DIR    := bin
# Go tooling must never see web/node_modules (stray .go files from
# transitive npm deps) or .worktrees (sibling task checkouts): go's ./...
# skips dot-directories and stops at web/go.mod, but find/gofmt do not.
GO_FILES   := find . \( -name node_modules -o -name .worktrees \) -prune -o -type f -name '*.go' -print
GO_SRC     := $(shell $(GO_FILES) 2>/dev/null) go.mod

# Resource caps for every Go invocation below. `go test ./...` defaults -p to
# GOMAXPROCS (32 on this box), so the full tree can hold 32 test binaries
# resident at once; with a second worktree gating at the same time and the
# local agent fleet on the machine, that OOMed and locked the box. -p bounds
# how many package binaries run concurrently, and GOMEMLIMIT gives each one's
# GC a soft ceiling to collect against instead of growing until the kernel
# steps in (host/ peaks at 8.5G unbounded, 5.2G under this limit, and
# still finishes inside its 30s budget at 24.8s -- 4GiB thrashes it to 33.5s).
#
# -parallel and GOMAXPROCS are deliberately left at their defaults: capping
# them to 2 was measured and bought only ~1.4G more headroom while doubling
# rules/ (4.6s -> 11.0s) and pushing every package past its recorded budget.
#
# GOMEMLIMIT bounds the Go HEAP only. The race detector's shadow memory sits
# outside it, so -race stays one package at a time, and never in two
# worktrees at once.
#
# Both ?= so a caller can raise them for a deliberate one-off measurement.
export GOMEMLIMIT ?= 5GiB
GO_TEST_FLAGS ?= -p=2

# Where forgec puts the fetched corpus and the IR compiled from it. Never
# committed — the scripts are GPL-3.0.
CARDS_DIR  ?= .cards
# Pinned to the lock's commit for M2r; a corpus bump is a deliberate,
# ledgered change, not a side effect of Forge's master moving.
FORGE_REF  ?= 95f04e8a04c8925fa97cb226fc3341cabcc90a53

.PHONY: help
help:
	@echo "mtgcore targets:"
	@echo "  make build          — compile forgec, mtgsim and gorged"
	@echo "  make fetch-cards    — fetch Forge cardsfolder + tokenscripts at FORGE_REF into $(CARDS_DIR)"
	@echo "  make compile-cards  — compile the fetched corpus into the IR cache"
	@echo "  make report         — print card coverage against implemented primitives"
	@echo "  make sim            — build mtgsim and play 20 verified 4-seat games"
	@echo "  make gorged         — run the M2a table server (browser client at the addr)"
	@echo "  make deploy-demo    — rebuild and (re)serve the demo on :8080 public / :8081 omniscient"
	@echo "                        (PACE=1.5s / TABLES / SEATS / FORMATS / SEED override the defaults)"
	@echo "  make stop-demo      — stop every running gorged, start nothing"
	@echo "  make gentypes       — regenerate web/src/protocol.ts from package protocol"
	@echo "  make web            — npm ci and build the spectator client into cmd/gorged/webdist"
	@echo "  make web-dev        — run the Vite dev server for web/"
	@echo "  make test-web       — run web/'s Vitest suite"
	@echo "  make lint-web       — svelte-check and eslint over web/"
	@echo "  make test lint cover"
	@echo "  make conformance    — run the CR 601 suite (KNOWN-RED; failing today is the \
expected outcome — see the target's comment)"
	@echo "  NOTE: make test-web / npm test needs Node >=22 (vitest 5); see web/README.md"

.PHONY: build
build: $(BIN_DIR)/forgec $(BIN_DIR)/mtgsim $(BIN_DIR)/gorged

$(BIN_DIR)/forgec: $(GO_SRC)
	@mkdir -p $(BIN_DIR)
	CGO_ENABLED=0 go build -o $@ ./cmd/forgec

$(BIN_DIR)/mtgsim: $(GO_SRC)
	@mkdir -p $(BIN_DIR)
	CGO_ENABLED=0 go build -o $@ ./cmd/mtgsim

$(BIN_DIR)/gorged: $(GO_SRC)
	@mkdir -p $(BIN_DIR)
	CGO_ENABLED=0 go build -o $@ ./cmd/gorged

.PHONY: gorged
# gorged runs the M2a table server: perpetual bot tables served to a browser
# at the listen address. make web builds the Svelte client it embeds first.
gorged: $(BIN_DIR)/gorged
	$(BIN_DIR)/gorged -decks internal/testutil/decks -tables 4 -seats 4 -pace 1.5s -format commander,constructed

.PHONY: deploy-demo stop-demo
# deploy-demo refreshes the local demo: two servers on 127.0.0.1, public
# spectator on :8080 and omniscient on :8081, each with two Commander and
# two constructed tables so the overview's per-format sections are both
# populated. .githooks/post-merge runs it in the background after a merge
# into main, so what is on :8080 is never older than main.
#
# The binary is rebuilt unconditionally rather than through
# $(BIN_DIR)/gorged: that rule depends on the Go sources, but the client
# reaches the server through //go:embed all:webdist, so a web-only change
# leaves every .go file untouched and the embedded client stale -- which is
# precisely the change a demo exists to show.
deploy-demo: web
	@mkdir -p $(BIN_DIR)
	CGO_ENABLED=0 go build -o $(BIN_DIR)/gorged ./cmd/gorged
	scripts/deploy-demo.sh

# stop-demo takes the demo down without starting it again. Same socket
# lookup as the deploy: never `pkill -f`.
stop-demo:
	scripts/deploy-demo.sh --stop-only

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
	go test $(GO_TEST_FLAGS) ./...

# conformance runs the CR 601 conformance suite (rules/cr601_conformance_test.go)
# as an OPT-IN, KNOWN-RED lane. Those tests are RED BY DESIGN: they pin unfixed
# defects (I-2: cast legality is checked AFTER committing, so an illegal target
# is taken before the cost is paid and only then reversed; I-7: a counterspell
# with only itself on the stack fizzles rather than counter itself, whose
# current assertion pins graveyard card-loss that CR 733.1 says is wrong). They
# are gated behind GORGE_CR_CONFORMANCE=1 precisely because a PASS here means a
# defect got fixed and the corresponding opt-in guard should be REMOVED -- so
# failure TODAY is the expected outcome, not a regression, and a green run is
# the signal to act, not a reason to celebrate.
#
# It is intentionally NOT reachable from `make test`, `go test ./...`, or any
# default gate (there is no CI here): wiring it into the default tree would make
# the tree red by construction, strictly worse than the status quo.
.PHONY: conformance
conformance:
	@echo "== CR 601 conformance: KNOWN-RED lane — failure is the EXPECTED outcome today; a PASS means a defect (I-2/I-7) was fixed and the opt-in guard should be removed =="
	GORGE_CR_CONFORMANCE=1 go test $(GO_TEST_FLAGS) -count=1 ./rules -run TestCR601 -v

# gc-gate budgets the share of consumed CPU a package's tests spend collecting
# garbage. GC_PROCS pins GOMAXPROCS so the figure is a property of the code
# rather than of the box that ran it — idle-mark CPU scales with core count,
# and it is real CPU taken from everything else on a shared machine, so the
# budget counts it. GC_BUDGET is a fraction of user+sys, NOT the percentage
# gctrace prints (that one divides by every core the process could have used
# and reads ~1% however badly the code allocates).
GC_PKG    ?= ./host
GC_PROCS  ?= 32
GC_BUDGET ?= 0.30
.PHONY: gc-gate
gc-gate:
	go run ./cmd/gcgate -pkg $(GC_PKG) -maxprocs $(GC_PROCS) -budget $(GC_BUDGET)

# test-time measures every package's test wall time and records it, plus its
# budget, in each package's TEST_HISTORY.md (Task TT). The pre-commit hook
# enforces the budget on changed packages.
.PHONY: test-time
test-time:
	go run ./cmd/testtime -all

# alloc-gate measures every package's test peak RSS and total allocation and
# records them, plus hard budgets, in each package's ALLOC_HISTORY.md (Task
# A4). The budgets bind: a package over its alloc_budget_mb (total GC work)
# or rss_budget_mb (what actually OOMs a shared box) fails the gate. This is
# the absolute-scale counterpart to gc-gate's ratio budget.
.PHONY: alloc-gate
alloc-gate:
	go run ./cmd/allocgate -all

COVER_OUT  ?= coverage.out
COVER_HTML ?= coverage.html
.PHONY: cover cover-html
cover:
	go test $(GO_TEST_FLAGS) -covermode=atomic -coverprofile=$(COVER_OUT) -coverpkg=./... ./...
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
	@# REFUSE to install through a symlink. Task worktrees get web/node_modules
	@# as a symlink to the main checkout's install (agent-worktree.sh --web), so
	@# ~166MB is shared instead of copied. `npm ci` DELETES node_modules before
	@# installing, and through a symlink that delete lands on the SHARED
	@# directory -- wiping web tooling for the main checkout and every other
	@# worktree at once. It has happened: an agent ran `make web`, this rule
	@# fired because package-lock.json was newer than the marker, and four
	@# concurrent worktrees lost their install. The agent had been told not to
	@# run `npm ci` and did not -- make did. So the guard belongs here, where
	@# the command actually is, not in a brief nobody can enforce.
	@if [ -L web/node_modules ]; then 		echo "make: refusing to run 'npm ci' -- web/node_modules is a SYMLINK to $$(readlink web/node_modules)."; 		echo "      npm ci deletes node_modules first, which would wipe the SHARED install"; 		echo "      used by the main checkout and every other task worktree."; 		echo "      Run 'make web' (or npm ci) in the MAIN checkout instead, then re-run here."; 		exit 1; 	fi
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
