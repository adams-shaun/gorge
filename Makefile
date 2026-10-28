SHELL      := /bin/bash
BIN_DIR    := bin
# Go tooling must never see web/node_modules (stray .go files from
# transitive npm deps) or .worktrees (sibling task checkouts): go's ./...
# skips dot-directories and stops at web/go.mod, but find/gofmt do not.
#
# .ds4 and .superpowers are pruned for the same reason: both are agent scratch
# trees, neither carries a single TRACKED .go file (verified: `git ls-files
# '.ds4/**/*.go' '.superpowers/**/*.go'` is empty), and both are gitignored.
# Without the prune `make lint` fails on the formatting of files nobody can
# commit -- an archived agent's scratch main.go was enough to turn the lane
# red -- and GO_SRC picks them up as build dependencies, so dropping a stray
# .go file into a scratch directory needlessly rebuilds every binary.
GO_FILES   := find . \( -name node_modules -o -name .worktrees -o -name .ds4 -o -name .superpowers \) -prune -o -type f -name '*.go' -print
GO_SRC     := $(shell $(GO_FILES) 2>/dev/null) go.mod

# Resource caps for every Go invocation below. `go test ./...` defaults -p to
# GOMAXPROCS (32 on this box), so the full tree can hold 32 test binaries
# resident at once; with a second worktree gating at the same time and the
# local agent fleet on the machine, that OOMed and locked the box. -p bounds
# how many package binaries run concurrently, and GOMEMLIMIT gives each one's
# GC a soft ceiling to collect against instead of growing until the kernel
# steps in (server/host/ peaks at 8.5G unbounded, 5.2G under this limit, and
# still finishes inside its 30s budget at 24.8s -- 4GiB thrashes it to 33.5s).
#
# -parallel and GOMAXPROCS are deliberately left at their defaults: capping
# them to 2 was measured and bought only ~1.4G more headroom while doubling
# engine/rules/ (4.6s -> 11.0s) and pushing every package past its recorded budget.
#
# GOMEMLIMIT bounds the Go HEAP only. The race detector's shadow memory sits
# outside it, so -race stays one package at a time, and never in two
# worktrees at once.
#
# Both ?= so a caller can raise them for a deliberate one-off measurement.
export GOMEMLIMIT ?= 5GiB
# -trimpath (operator, 2026-10-05): without it each worktree's absolute path
# keys every compile, so no worktree reuses another's build cache.
export GOFLAGS ?= -p=2 -trimpath
GO_TEST_FLAGS ?= -p=2

# seat-check's two structural test runs. systemd-run bounds them when this box
# offers it; otherwise they fall back to the plain env caps (and -p 1 limits
# them to one package binary at a time).
SEAT_CAP ?= $(shell systemd-run --user --scope -q true >/dev/null 2>&1 && echo 'systemd-run --user --scope -q -p MemoryMax=4G -p CPUQuota=400%')

# Where forgec puts the fetched corpus and the IR compiled from it. Never
# committed — the scripts are GPL-3.0.
CARDS_DIR  ?= .cards
# gorge's fork of Card-Forge/forge: its `gorge` branch is upstream plus
# script corrections against the printed card, each also sent upstream.
FORGE_REPO ?= https://github.com/adams-shaun/forge.git
# Pinned to the lock's commit for M2r; a corpus bump is a deliberate,
# ledgered change, not a side effect of Forge's master moving.
# 25a608a4f = fb4d80911 (Card-Forge master 2026-10-02) + Card-Forge/forge#12153
# (Bard's Company P/T, Voice of Victory type, Yip Yip! target) + #12160
# (Demonic Pact's discard mode targets an opponent).
# 1dc038bc6 = the adams-shaun/forge branch `gorge` (2026-10-12): adds 764b1e3a3
# (g_1_1_elf token is Elf, not Elf Warrior; Card-Forge/forge#12242/#12244) and
# 1dc038bc6 (Bullseye, Death Dealer, Ghostly Dancers ETB Origin$ Any |
# Destination$ Battlefield; Reckless Detective UnlessResolveSubs$ WhenPaid;
# Card-Forge/forge#12243/#12245).
# 098b05531 = 1dc038bc6 + The Earth Crystal targets only creatures you control
# (ValidTgts$ Creature.YouCtrl; Card-Forge/forge#12255/#12256).
# 76fd7691a = 098b05531 + 8 card-script fixes against the printed card: "up to
# one target" (#12316); P/T, loyalty, cost, types and keywords for 18 cards
# (#12318); three Sorceries (#12319); Kutzil's Flanker and Sarkhan's
# Dragonfire amounts (#12320); exact/required target counts (#12321);
# planeswalker/battle targets (#12322); Dream Beavers' Defined$ and
# Tresserhorn's Lord's ValidTgts$ (#12323), all Card-Forge/forge.
# 91402ac51 = 76fd7691a + 53ab12ae9 (wires unreachable SVars: Your Mistake Is My
# Triumph, Disciple of Perdition, Last Rites) + 91402ac51 (Mandatory$ True on 48
# ChooseCard lines of 44 cards whose Oracle says "choose"; Card-Forge/forge#12481/#12482).
FORGE_REF  ?= 91402ac511046c0b7ed0bb691a8a446ca7dfc873

.PHONY: help
help:
	@echo "mtgcore targets:"
	@echo "  make build          — compile forgec, mtgsim and gorged"
	@echo "  make fetch-cards    — fetch Forge cardsfolder + tokenscripts at FORGE_REF into $(CARDS_DIR)"
	@echo "  make compile-cards  — compile the fetched corpus into the IR cache"
	@echo "  make report         — print card coverage against implemented primitives"
	@echo "  make enginebench-pair BASE=main CAND=. — paired engine-speed comparison (cmd/enginebench)"
	@echo "  make enginebench-profile [REV=.]       — CPU/alloc profiles of the bench rows, tops printed"
	@echo "  make enginebench-verify [REV=.]        — memo-verify-mode smoke over the bench rows"
	@echo "  make sim            — build mtgsim and play 20 verified 4-seat games"
	@echo "  make pr [PR_BASE=main] [DRY=1] — open/refresh a GitHub PR with a generated change overview"
	@echo "  make gorged         — run the M2a table server (browser client at the addr)"
	@echo "  make gentypes       — regenerate web/src/protocol.ts from package protocol"
	@echo "  make web            — npm ci and build the spectator client into server/gorged/webdist"
	@echo "  make web-dev        — run the Vite dev server for web/"
	@echo "  make test-web       — run web/'s Vitest suite"
	@echo "  make lint-web       — svelte-check and eslint over web/"
	@echo "  make smoke          — headless-browser smoke gate vs two real gorged servers (public+omniscient); fails on any browser error or a hung loading state"
	@echo "  make test lint cover"
	@echo "  make seat-check     — structural seat gate: gofmt, gentypes -check, archtest, codeshape"
	@echo "  make conformance    — run the CR 601/733 conformance suites (see the target's comment)"
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
# -vsbot arms the landing page's play-a-human-vs-bot entry point; without it
# the server is spectator-only, exactly as before the feature existed.
gorged: $(BIN_DIR)/gorged
	$(BIN_DIR)/gorged -decks testkit/testutil/decks -tables 4 -seats 4 -pace 1.5s -format commander,constructed -vsbot

.PHONY: sim
sim: $(BIN_DIR)/mtgsim
	$(BIN_DIR)/mtgsim -seats 4 -games 20 -verify

.PHONY: fetch-cards
# A worktree's corpus tests open the main checkout's .cards, which is outside
# that worktree's module root, so `go test` does not hash it: a cached pass
# would survive a corpus change. Drop the test cache when the pin moves.
fetch-cards: $(BIN_DIR)/forgec
	@before=$$(cat $(CARDS_DIR)/cards.lock 2>/dev/null); \
	$(BIN_DIR)/forgec fetch -dir $(CARDS_DIR) -repo $(FORGE_REPO) -ref $(FORGE_REF) && \
	if [ "$$before" != "$$(cat $(CARDS_DIR)/cards.lock 2>/dev/null)" ]; then go clean -testcache; echo "corpus changed: go test cache cleared"; fi

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
# GOGC=200: rules.test measured 104 s vs 112 s CPU (28.3 s vs 30.4 s wall) for 1.2-1.4 GB vs 0.9 GB RSS.
test:
	GOGC=200 go test $(GO_TEST_FLAGS) ./...

# conformance runs the CR 601 and CR 733 conformance suites. I-2
# (mandatory-target feasibility), I-7 (targets before payment), and the CR 733.1
# illegal-cast reversal are fixed and asserted by the ordinary suite; the
# historical opt-in guard and GORGE_CR_CONFORMANCE switch were removed after
# the following fixes landed:
#
#   38846fb2  withhold casts without mandatory targets (I-2)
#   22ea5da0  make the cast proposal a transaction (I-7)
#   81ade672  undo as-enters choice on a mana abort (CR 733.1)
#
# This remains an explicit lane because it is a focused CR audit rather than a
# default full-suite target. Use -run TestCR, not TestCR601: the 733 reversal
# tests are named TestCR733* and a narrower pattern silently drops them.
.PHONY: conformance
conformance:
	@echo "== CR 601/733 conformance: all audited leaves fixed; ordinary-suite assertions =="
	go test $(GO_TEST_FLAGS) -count=1 ./engine/rules -run TestCR -v

# The Oracle-text card audit (scenarios written from printed card text and
# the CR, docs/design/2026-09-27-oracle-text-card-audit.md) and the XMage
# ports live in gorge-compliance (github.com/adams-shaun/gorge-compliance,
# `make oracle-audit` there). gorge keeps the runner and its contract check:
# go test ./engine/rules -run 'TestOracleRunnerContract|TestOracleScenarioShapeCheck'.

# gc-gate budgets the share of consumed CPU a package's tests spend collecting
# garbage. GC_PROCS pins GOMAXPROCS so the figure is a property of the code
# rather than of the box that ran it — idle-mark CPU scales with core count,
# and it is real CPU taken from everything else on a shared machine, so the
# budget counts it. GC_BUDGET is a fraction of user+sys, NOT the percentage
# gctrace prints (that one divides by every core the process could have used
# and reads ~1% however badly the code allocates).
GC_PKG    ?= ./server/host
GC_PROCS  ?= 32
GC_BUDGET ?= 0.30
.PHONY: gc-gate
gc-gate:
	go run ./cmd/gcgate -pkg $(GC_PKG) -maxprocs $(GC_PROCS) -budget $(GC_BUDGET)

# fuzz runs the opt-in fuzz tests (build tag `fuzz`): the rules seed-fuzz
# acceptance sweep, the ManaBrew decoder fuzz target's seed corpus, and the
# ManaBrew response translator fuzz target. They are excluded from `make
# test` and CI; run by hand or at night.
.PHONY: fuzz
fuzz:
	go test $(GO_TEST_FLAGS) -tags fuzz,manabrew -count=1 -run 'Fuzz' ./engine/rules ./server/protocol/manabrew ./server/manabrew

# test-manabrew runs the ManaBrew wire/translator tests (build tag `manabrew`).
# ManaBrew work is deprioritised (operator, 2026-10-02), so these test files
# are excluded from `go test ./...`, `make test` and CI.
.PHONY: test-manabrew
test-manabrew:
	go test $(GO_TEST_FLAGS) -tags manabrew -count=1 ./server/protocol/manabrew ./server/manabrew/... ./server/host/manabrewhttp ./cmd/gorged ./cmd/cardfuzz

# Engine-speed benchmarking (cmd/enginebench; scripts in scripts/enginebench-*.sh,
# workload decks committed in cmd/enginebench/testdata/decks). Every run is
# capped in a systemd scope when one is available (and serialised on
# HEAVY_LOCK only if you set it); binaries, results and profiles go to
# BENCH_DIR (rebuildable scratch). Throughput is per CPU-second so a loaded
# box still compares.
#   make enginebench-pair BASE=main CAND=.     paired base-vs-candidate rows
#   make enginebench-profile [REV=.]           CPU + allocation profiles, tops printed
#   make enginebench-verify [REV=.]            memo-verify-mode smoke (cache changes)
# Knobs: ROWS="-row random -pair A;-row sampler" REPS=3 SECS=10 GOGC=...
# BENCH_DIR HEAVY_LOCK (see scripts/enginebench-lib.sh).
BASE ?= main
CAND ?= .
REV  ?= .
.PHONY: enginebench-pair enginebench-profile enginebench-verify
enginebench-pair:
	scripts/enginebench-pair.sh $(BASE) $(CAND)
enginebench-profile:
	scripts/enginebench-profile.sh $(REV)
enginebench-verify:
	scripts/enginebench-verify.sh $(REV)

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
	@# REFUSE to install through a symlink. Task worktrees may get web/node_modules
	@# as a symlink to the main checkout's install (four-yay's
	@# ops/scripts/agent-worktree.sh --web moved there 2026-10-16), so
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

# smoke is the browser smoke gate (Task SG1, extended by ui19): it builds the
# REAL client and the REAL binary, starts THREE gorged servers — one
# -spectator public (the mode that was broken when a null hand spread into
# the board killed every table page, and the one the omniscient-only unit
# fixtures cannot see), one -spectator omniscient, and one SEATED 1v1
# play-vs-bot server (-vsbot -humans 1, Task ui19) — on smoke ports in
# 8090-8099, and drives the headless-browser test at web/e2e against all
# three. It fails on any pageerror, any console.error, any failed request to
# the server's own origin, a page still in its loading state after a bounded
# wait, or a blank page that never mounts. It also asserts the SEATED layout
# invariants ui19 was added for: the seated player's own identity bar does
# not overlap their own hand fan, and the hand fan stays bounded by the
# board (never self-sizing straight off it). scripts/smoke.sh owns
# starting/stopping the set and removing their temp dirs on failure (a leaked
# gorged poisons the next run).
#
# Deliberately NOT part of `make test`. The Go suite must stay runnable with
# no browser and no live servers; threading smoke into the default tree would
# make `make test` need Playwright + three gorged on this shared box. CI and
# the merge gate run it explicitly, exactly like the other opt-in lanes
# (conformance, gc-gate).
.PHONY: smoke
smoke:
	scripts/smoke.sh

.PHONY: lint
lint: lint-web
	@out=$$(gofmt -l $$($(GO_FILES)) 2>&1); \
		if [ -n "$$out" ]; then \
			echo "gofmt: files need formatting:"; echo "$$out"; exit 1; \
		fi
	go vet ./...
	go run ./cmd/gentypes -check

# seat-check is the fast structural gate a seat runs before DONE: formatting,
# the generated wire types, and the two architecture/shape ratchets. It
# deliberately leaves out `go vet ./...` and `lint-web` (make lint), which the
# daemon's gates cover; it must stay a ~10 s check.
.PHONY: seat-check
seat-check:
	@out=$$(gofmt -l $$($(GO_FILES)) 2>&1); \
		if [ -n "$$out" ]; then \
			echo "gofmt: files need formatting:"; echo "$$out"; exit 1; \
		fi
	go run ./cmd/gentypes -check
	$(SEAT_CAP) env GOMAXPROCS=4 GOMEMLIMIT=3GiB go test -timeout 2m -p 1 ./internal/archtest/
	$(SEAT_CAP) env GOMAXPROCS=4 GOMEMLIMIT=3GiB go test -timeout 2m -p 1 ./internal/codeshape/

.PHONY: clean
clean:
	rm -rf $(BIN_DIR) $(COVER_OUT) $(COVER_HTML)

# clean-cards is destructive and separate from clean: refetching the corpus is
# a multi-minute network operation.
.PHONY: clean-cards
clean-cards:
	rm -rf $(CARDS_DIR)

# coverage writes the card-support tables -- .coverage/summary.md and
# .coverage/coverage.md -- into the gitignored .coverage/. They are generated,
# never committed: the `card coverage` workflow publishes them on its run page
# (job summary + artifact). Output is deterministic (no wall clock; every table
# sorted), so two runs over the same corpus and tree are byte-identical.
.PHONY: coverage
coverage: $(BIN_DIR)/forgec
	$(BIN_DIR)/forgec coverage -dir $(CARDS_DIR) -out .coverage

# ---- pull requests ------------------------------------------------------------
# make pr                 open (or refresh) a GitHub PR: current branch -> origin's default branch
#   PR_HEAD=<branch>      branch to propose (default: the checked-out branch)
#   PR_BASE=<branch>      target branch (default: origin's default branch)
#   TITLE="..."           PR title (default: the commit subject, or "<head> → <base>: N commits")
#   SUMMARY="..."         a paragraph placed under the overview table
#   DRAFT=1               open as a draft      DRY=1   print the title and body only
.PHONY: pr
pr: export HEAD = $(PR_HEAD)
pr: export BASE = $(PR_BASE)
pr: export TITLE := $(TITLE)
pr: export SUMMARY := $(SUMMARY)
pr: export DRAFT := $(DRAFT)
pr: export DRY := $(DRY)
pr:
	@bash scripts/open-pr.sh
