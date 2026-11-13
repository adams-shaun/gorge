SHELL      := /bin/bash
BIN_DIR    := bin
# Go tooling must never see web/node_modules (stray .go files from
# transitive npm deps) or .worktrees (sibling task checkouts): go's ./...
# skips dot-directories and stops at web/go.mod, but find/gofmt do not.
#
# .ds4 and .superpowers are pruned for the same reason: both are gitignored
# local scratch trees with no tracked .go file. Without the prune `make lint`
# fails on the formatting of files nobody can commit, and GO_SRC picks them up
# as build dependencies.
GO_FILES   := find . \( -name node_modules -o -name .worktrees -o -name .ds4 -o -name .superpowers \) -prune -o -type f -name '*.go' -print
GO_SRC     := $(shell $(GO_FILES) 2>/dev/null) go.mod

# Resource caps for every Go invocation below. `go test ./...` defaults -p to
# GOMAXPROCS, so the full tree can hold that many test binaries resident at
# once, which has run a machine out of memory. -p bounds
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
# -trimpath: without it each worktree's absolute path
# keys every compile, so no worktree reuses another's build cache.
export GOFLAGS ?= -p=2 -trimpath
GO_TEST_FLAGS ?= -p=2

# seat-check's two structural test runs. systemd-run bounds them when the machine
# offers it; otherwise they fall back to the plain env caps (and -p 1 limits
# them to one package binary at a time).
SEAT_CAP ?= $(shell systemd-run --user --scope -q true >/dev/null 2>&1 && echo 'systemd-run --user --scope -q -p MemoryMax=4G -p CPUQuota=400%')

# Where forgec puts the fetched corpus and the IR compiled from it. Never
# committed — the scripts are GPL-3.0.
CARDS_DIR  ?= .cards
# gorge's fork of Card-Forge/forge: its `gorge` branch is upstream plus
# script corrections against the printed card, each also sent upstream.
FORGE_REPO ?= https://github.com/adams-shaun/forge.git
# Pinned to the lock's commit; a corpus bump is a deliberate change, not a side effect of Forge's master moving.
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
# 07d4f918d = Card-Forge master 2026-11-02, which holds those fixes and scripts Star Trek (TRK, TRC).
# e5904606 = 07d4f918d + the fork's Star Trek scripts (16 upstream PRs, 55 authored against the printed cards, a Romulan Bird-of-Prey fix).
# 66757752dc = e5904606 + H.M.S. Bounty (Count$YourExtraTurns, the extra-turn counter and its script).
# 21f2538a76 = the fork's gorge-trek3: Card-Forge master 6264f0292f (2026-11-03) + the open Star Trek PR heads
# (Card-Forge/forge#12365 through #12561) + the fork's authored Star Trek scripts, H.M.S. Bounty and four script fixes sent to those PRs.
# df3c93c8e4 = the fork's gorge-trek4: Card-Forge master e7020aa13c (2026-11-04) + the open Star Trek PR heads of 2026-11-04
# (Card-Forge/forge#12365 through #12561) + the fork's own Star Trek PRs with their review fixes (Card-Forge/forge#12666 through
# #12673: the authored scripts, H.M.S. Bounty and EndCombatPhase's Optional$) + Dr. McCoy's ActiveZones$.
# 64afbdf3a6 = the fork's gorge-trek5: df3c93c8e4 + Card-Forge master da66fb1ac4 (2026-11-04), whose one card change
# is The Duras Sisters' printed 3/2 (it was 1/1).
# 84ce1603d9 = the fork's gorge-trek6: 64afbdf3a6 + two Oracle fixes XMage's TRK pass found (2026-11-04): 70,000
# Light-Years from Home puts the basic land into its controller's hand, and Captain Michael Burnham is a Human Officer.
# 418ba5215f = the fork's gorge-trek7: 84ce1603d9 + reviewer fixes to Card-Forge/forge#12668/#12669 (2026-11-07): ABILITY trigger
# text and ValidTgtsDesc$ (Tal Shiar Operative, Wesley Crusher, Sela), Tal Shiar Analyst's Defined$ Opponent, Sarek's PlayMain1.
FORGE_REF  ?= 418ba5215f63ca660f830468d3c66dc6886a6e5e

.PHONY: help
help:
	@echo "mtgcore targets:"
	@echo "  make build          — compile forgec, mtgsim and gorged"
	@echo "  make fetch-cards    — fetch Forge cardsfolder + tokenscripts at FORGE_REF into $(CARDS_DIR)"
	@echo "  make compile-cards  — compile the fetched corpus into the IR cache"
	@echo "  make report         — print card coverage against implemented primitives"
	@echo "  make dirac-tables   — write dirac's binary card tables under the corpus (gitignored, keyed by corpus and kgen sources)"
	@echo "  make dirac-slices   — play dirac against gorge on the committed FDN and BLB slices, both surfaces"
	@echo "  make dirac-mp-check — vet, test and lockstep-play dirac's four-seat-slot size class (build tag dirac_mp)"
	@echo "  make test-one PKG=./p RUN=TestX — one capped, corpus-checked test run with one verdict line (debugging)"
	@echo "  make test-targeted  — the packages your diff vs BASE touches plus their test reverse deps (the inner loop)"
	@echo "  make test-smart     — the landing gate's steps, fail-fast, on a committed tree (precheck is an alias)"
	@echo "  make fresh-clone-check — build, vet and test dirac with no corpus: tests needing tables skip, none fail"
	@echo "  make enginebench-pair BASE=main CAND=. — paired engine-speed comparison (cmd/enginebench)"
	@echo "  make enginebench-profile [REV=.]       — CPU/alloc profiles of the bench rows, tops printed"
	@echo "  make enginebench-verify [REV=.]        — memo-verify-mode smoke over the bench rows"
	@echo "  make sim            — build mtgsim and play 20 verified 4-seat games"
	@echo "  make draftsim       — build draftsim and play 2 verified FDN draft events"
	@echo "  make pr [PR_BASE=main] [DRY=1] — open/refresh a GitHub PR with a generated change overview"
	@echo "  make gorged         — run the table server (browser client at the addr)"
	@echo "  make deploy-demo    — build the client and gorged, then (re)serve the demo (scripts/deploy-demo.sh; deploy/demo.env)"
	@echo "  make stop-demo      — stop the demo's servers, start nothing"
	@echo "  make demo-image     — build the demo's container images (deploy/docker; no card scripts inside)"
	@echo "  make demo-up        — build and start the demo in containers: nginx in front of gorged (deploy/docker/.env)"
	@echo "  make demo-down      — stop the demo's containers (volumes kept)"
	@echo "  make prewarm-art    — fill the demo's card-art cache from its decks"
	@echo "  make gentypes       — regenerate web/src/protocol.ts from package protocol"
	@echo "  make web            — npm ci and build the spectator client into server/gorged/webdist"
	@echo "  make web-dev        — run the Vite dev server for web/"
	@echo "  make test-web       — run web/'s Vitest suite"
	@echo "  make lint-web       — svelte-check and eslint over web/"
	@echo "  make smoke          — headless-browser smoke gate vs two real gorged servers (public+omniscient); fails on any browser error or a hung loading state"
	@echo "  make test lint cover"
	@echo "  make test-wires     — the SpellBench and ManaBrew interface tests (build tags spellbench, manabrew; not part of make test)"
	@echo "  make seat-check     — fast structural gate: gofmt, gentypes -check, archtest, codeshape"
	@echo "  make conformance    — run the CR 601/733 conformance suites (see the target's comment)"
	@echo "  make wire-matrix    — run every engine / wire / consumer cell of the commit tier (docs/site/architecture/wires.md)"
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
# gorged runs the table server: perpetual bot tables served to a browser
# at the listen address. make web builds the Svelte client it embeds first.
# -vsbot arms the landing page's play-a-human-vs-bot entry point; without it
# the server is spectator-only, exactly as before the feature existed.
gorged: $(BIN_DIR)/gorged
	$(BIN_DIR)/gorged -decks testkit/testutil/decks -tables 4 -seats 4 -pace 1.5s -format commander,constructed -vsbot

# deploy-demo refreshes the demo: it builds the web client and gorged from
# GORGE_ROOT (this checkout unless set), stops the demo's running servers by
# their listening sockets and serves the demo again: bot-only tables spectated
# omniscient, play-vs-bot tables public. Stopping the servers aborts every
# in-flight vs-bot game, so whoever owns the demo runs it by hand and nothing
# runs it automatically. Ports, directories, the art cache and the az
# policies' network paths come from the environment or the untracked
# deploy/demo.env (deploy/demo.env.example); scripts/deploy-demo.sh has every
# variable and its rationale, and `scripts/deploy-demo.sh --dry-run` prints
# the resolved configuration and the commands without running them.
# SERVER_MAIN in the same file builds another module's main package instead
# of cmd/gorged, against this checkout.
#
# stop-demo takes the demo down without starting it again, by the same socket
# lookup (never pkill -f). prewarm-art fills the durable card-art cache the
# deploy otherwise fills before it starts any server: a no-op when the cache
# is complete, non-zero if any name failed; by hand it is strict and
# unbounded, where the deploy bounds it and starts its servers whatever the
# fill's exit.
.PHONY: deploy-demo stop-demo prewarm-art
deploy-demo:
	scripts/deploy-demo.sh --build

stop-demo:
	scripts/deploy-demo.sh --stop-only

prewarm-art:
	scripts/deploy-demo.sh --prewarm-only

# The container demo (deploy/docker, docs/site/web/docker.md). The images carry
# no Forge card scripts: gorged fetches them at FORGE_REF into a volume on
# first start. There is no push target; tag and push the image by hand.
DEMO_COMPOSE = FORGE_REF=$(FORGE_REF) docker compose -f deploy/docker/compose.yaml

.PHONY: demo-image demo-up demo-down
demo-image:
	$(DEMO_COMPOSE) build

demo-up:
	$(DEMO_COMPOSE) up -d --build

demo-down:
	$(DEMO_COMPOSE) down

.PHONY: sim
sim: $(BIN_DIR)/mtgsim
	$(BIN_DIR)/mtgsim -seats 4 -games 20 -verify

# draftsim runs two seeded FDN draft events through the in-memory event host
# and verifies each one replays from its log.
.PHONY: draftsim
draftsim:
	go build -trimpath -o $(BIN_DIR)/draftsim ./cmd/draftsim
	$(BIN_DIR)/draftsim -set FDN -seed 1 -events 2 -verify

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

.PHONY: dirac-tables
# Every pool's binary card table under $(CARDS_DIR)/kernel-tables/<key>, and the
# synthetic evidence pools' under $(CARDS_DIR)/kernel-synth-tables/<key>. Exit 3
# means kgen wrote an ids.tsv: the binary embeds the old bytes, so it is rebuilt
# and rerun. Built, not `go run`, because go run exits 1 whatever the program's
# status was, which would hide the 3.
dirac-tables:
	@mkdir -p $(BIN_DIR)
	go build -o $(BIN_DIR)/kgen ./dirac/cmd/kgen && $(BIN_DIR)/kgen -tables; rc=$$?; \
	if [ $$rc -eq 3 ]; then go build -o $(BIN_DIR)/kgen ./dirac/cmd/kgen && $(BIN_DIR)/kgen -tables || exit $$?; elif [ $$rc -ne 0 ]; then exit $$rc; fi; \
	$(BIN_DIR)/kgen -synth-pools

.PHONY: dirac-slices
# dirac against gorge on the committed FDN and BLB slices, both surfaces. A divergence, an
# unmodelled stall or a hang fails; a capacity stall is counted and reported.
dirac-slices: dirac-tables
	go build -trimpath -o $(BIN_DIR)/kfuzz ./dirac/oracle/cmd/kfuzz
	@for set in fdn blb; do for surface in autopay full; do \
		$(BIN_DIR)/kfuzz -kernel $$set -first-pair 0 -pairs 1500 -seed 1 -keep-going -surface $$surface -mulligans 1 -shrink 0 -workers 4 -absolute -v || exit 1; \
	done; done

.PHONY: dirac-mp-check dirac-mp-lockstep
# The dirac_mp size class (four seat slots) vets and plays: its State fits the bar, its layout
# pin holds, two-seat random games run, and it plays in lockstep with gorge (dirac-mp-lockstep).
dirac-mp-check: dirac-tables
	go vet -tags dirac_mp ./dirac/...
	GOMAXPROCS=4 GOMEMLIMIT=3GiB go test -tags dirac_mp -trimpath -failfast -timeout 2m -run 'TestStateSize|TestStateLayoutPinnedMP|TestRandomGames' ./dirac
	$(MAKE) --no-print-directory dirac-mp-lockstep

# The dirac_mp build in lockstep with gorge, absolutely: 4-seat vanilla pods under the random and
# heuristic policies, and the FDN and BLB slices at two seats (the mp build plays a two-seat game
# as the default build does). A divergence, a hang, an unmodelled stall or an error fails; a
# capacity stall is counted. Needs dirac-tables. scripts/dirac-gate.sh runs it as step "mp".
dirac-mp-lockstep:
	go build -trimpath -tags dirac_mp -o $(BIN_DIR)/kfuzz-mp ./dirac/oracle/cmd/kfuzz
	@for pol in random heuristic; do \
		echo "kfuzz-mp: vanilla, 4 seats, policy $$pol"; \
		GOMAXPROCS=4 GOMEMLIMIT=3GiB $(BIN_DIR)/kfuzz-mp -kernel vanilla -seats 4 -pairs 100 -seed 1 -keep-going -policy $$pol -shrink 0 || exit 1; \
	done
	@for set in fdn blb; do \
		echo "kfuzz-mp: $$set, 2 seats, autopay"; \
		GOMAXPROCS=4 GOMEMLIMIT=3GiB $(BIN_DIR)/kfuzz-mp -kernel $$set -first-pair 0 -pairs 200 -seed 1 -keep-going -surface autopay -mulligans 1 -shrink 0 -workers 4 -absolute || exit 1; \
	done

.PHONY: fresh-clone-check
fresh-clone-check:
	bash scripts/fresh-clone-check.sh

# The test targets (scripts/test-targets.sh) replace a raw `go test`: capped, corpus-checked, one verdict line
# `test-<mode>: PASS|FAIL|ENV|EMPTY|BUDGET|BUSY <sha> <pkgs> <seconds>` (exit 0, 1, 3, 4, 5, 75).
#   test-one       PKG=./p RUN=TestX [SKIP= V=1 TIMEOUT=2m TAGS= TESTENV='KGEN_REPIN=1' UPDATE=1]: debugging
#   test-targeted  the packages the diff vs BASE touches plus every package whose tests import them
#   test-smart     the landing gate's steps, fail-fast, on a committed tree; precheck is an alias
# BASE is the branch the change lands on.
BASE ?= land
export BASE
.PHONY: test-one test-targeted test-smart precheck
# $(value …) passes RUN/SKIP/TESTENV verbatim; $(RUN) re-expands and eats $| as a make automatic variable.
test-one:
	@PKG='$(PKG)' RUN='$(value RUN)' SKIP='$(value SKIP)' V='$(V)' TIMEOUT='$(TIMEOUT)' TAGS='$(TAGS)' TESTENV='$(value TESTENV)' UPDATE='$(UPDATE)' bash scripts/test-targets.sh one
test-targeted:
	@bash scripts/test-targets.sh targeted
test-smart:
	@bash scripts/test-targets.sh smart
precheck: test-smart

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

# conformance runs the CR 601 and CR 733 conformance suites. Mandatory-target
# feasibility, targets before payment, and the CR 733.1
# illegal-cast reversal are asserted by the ordinary suite.
#
# This remains an explicit lane because it is a focused CR audit rather than a
# default full-suite target. Use -run TestCR, not TestCR601: the 733 reversal
# tests are named TestCR733* and a narrower pattern silently drops them.
.PHONY: conformance
conformance:
	@echo "== CR 601/733 conformance: all audited leaves fixed; ordinary-suite assertions =="
	go test $(GO_TEST_FLAGS) -count=1 ./engine/rules -run TestCR -v

.PHONY: wire-matrix
wire-matrix:
	scripts/wire-matrix.sh commit

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
	go test $(GO_TEST_FLAGS) -tags fuzz -count=1 -run 'Fuzz' ./engine/rules ./server/protocol/manabrew ./server/manabrew

# test-wires runs the SpellBench and ManaBrew interface tests, which sit behind
# the build tags `spellbench` and `manabrew` so they do not gate a merge. The
# nightly CI run takes them too.
WIRE_PKGS = ./server/spellbench/... ./server/host/sbv2http ./dirac/cmd/sbv2dirac/... ./cmd/spellbench-gorge-env ./server/manabrew/... ./server/host/manabrewhttp ./server/protocol/manabrew/...
.PHONY: test-wires
test-wires:
	go test $(GO_TEST_FLAGS) -tags 'spellbench manabrew' $(WIRE_PKGS)

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
# BENCH_DIR HEAVY_LOCK (see scripts/enginebench-lib.sh). BENCH_BASE defaults to main; a BASE on the command
# line still selects the base rev (it also selects the test targets' branch).
BENCH_BASE ?= main
CAND ?= .
REV  ?= .
.PHONY: enginebench-pair enginebench-profile enginebench-verify
enginebench-pair:
	scripts/enginebench-pair.sh $(if $(filter command line environment,$(origin BASE)),$(BASE),$(BENCH_BASE)) $(CAND)
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
	@# as a symlink to the main checkout's install, so
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

# smoke is the browser smoke gate: it builds the
# REAL client and the REAL binary, starts its gorged servers — one
# -spectator public (the mode that was broken when a null hand spread into
# the board killed every table page, and the one the omniscient-only unit
# fixtures cannot see), one -spectator omniscient, one SEATED 1v1
# play-vs-bot server (-vsbot -humans 1), two fixtures, and one server per
# engine offering all three wires — on smoke ports in
# 8090-8099, and drives the headless-browser test at web/e2e against them.
# It fails on any pageerror, any console.error, any failed request to
# the server's own origin, a page still in its loading state after a bounded
# wait, or a blank page that never mounts. It also asserts the SEATED layout
# invariants: the seated player's own identity bar does
# not overlap their own hand fan, and the hand fan stays bounded by the
# board (never self-sizing straight off it). scripts/smoke.sh owns
# starting/stopping the set and removing their temp dirs on failure (a leaked
# gorged poisons the next run).
#
# Deliberately NOT part of `make test`. The Go suite must stay runnable with
# no browser and no live servers; threading smoke into the default tree would
# make `make test` need Playwright + these gorged on this shared box. CI runs it
# as the job browser, exactly like the other opt-in lanes
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

# seat-check is the fast structural gate to run before finishing a change:
# formatting, the generated wire types, and the two architecture/shape ratchets.
# It deliberately leaves out `go vet ./...` and `lint-web` (make lint); it must
# stay a ~10 s check.
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
#   DRAFT=1               open as a draft      DRY=1   print the oracle step, the title and the body only
#   SKIP_ORACLE=1         do not play the whole of dirac/oracle before a PR to main
.PHONY: pr
pr: export HEAD = $(PR_HEAD)
pr: export BASE = $(PR_BASE)
pr: export TITLE := $(TITLE)
pr: export SUMMARY := $(SUMMARY)
pr: export DRAFT := $(DRAFT)
pr: export DRY := $(DRY)
pr:
	@bash scripts/open-pr.sh
