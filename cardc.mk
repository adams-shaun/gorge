# The card compiler's make targets, included last from the Makefile.
MTGISH_REF ?= 1553384c9a970a02514b337db9c86697313530e8

.PHONY: mtgish-fetch cardc-check

# Fetch the pinned mtgish data into $(CARDS_DIR)/mtgish/<ref>/, sha256-checked.
mtgish-fetch:
	@MTGISH_REF=$(MTGISH_REF) bash scripts/cardc-mtgish-fetch.sh

cardc-check:
	@$(MAKE) test-targeted BASE=origin/main

SLICE ?= fdn
.PHONY: cardc-lockstep
# Every FDN pair (or the Legacy acceptance seats) through cardc lockstep, in chunks of 100.
cardc-lockstep:
	@if [ "$(SLICE)" = legacy ]; then for s in 2 4 6 8; do go run ./cmd/cardc lockstep -slice legacy -seats $$s || exit 1; done; \
	else for f in $$(seq 0 100 1400); do go run ./cmd/cardc lockstep -slice fdn -from $$f -n 100 || exit 1; done; fi
