SHELL      := /bin/bash
BIN_DIR    := bin
GO_SRC     := $(shell find . -type f -name '*.go' 2>/dev/null) go.mod

# Where forgec puts the fetched corpus and the IR compiled from it. Never
# committed — the scripts are GPL-3.0.
CARDS_DIR  ?= .cards
# Pinned to the lock's commit for M2r; a corpus bump is a deliberate,
# ledgered change, not a side effect of Forge's master moving.
FORGE_REF  ?= 95f04e8a04c8925fa97cb226fc3341cabcc90a53

.PHONY: help
help:
	@echo "mtgcore targets:"
	@echo "  make build          — compile forgec and mtgsim"
	@echo "  make fetch-cards    — fetch Forge cardsfolder at FORGE_REF into $(CARDS_DIR)"
	@echo "  make compile-cards  — compile the fetched corpus into the IR cache"
	@echo "  make report         — print card coverage against implemented primitives"
	@echo "  make sim            — build mtgsim and play 20 verified 4-seat games"
	@echo "  make test lint cover"

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

.PHONY: lint
lint:
	@out=$$(gofmt -l . 2>&1); \
		if [ -n "$$out" ]; then \
			echo "gofmt: files need formatting:"; echo "$$out"; exit 1; \
		fi
	go vet ./...

.PHONY: clean
clean:
	rm -rf $(BIN_DIR) $(COVER_OUT) $(COVER_HTML)

# clean-cards is destructive and separate from clean: refetching the corpus is
# a multi-minute network operation.
.PHONY: clean-cards
clean-cards:
	rm -rf $(CARDS_DIR)
