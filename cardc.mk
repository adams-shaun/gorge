# The typed card interface's make targets, included last from the Makefile.
MTGISH_REF ?= 1553384c9a970a02514b337db9c86697313530e8

.PHONY: mtgish-fetch

# Fetch the pinned mtgish data into $(CARDS_DIR)/mtgish/<ref>/, sha256-checked.
mtgish-fetch:
	@MTGISH_REF=$(MTGISH_REF) bash scripts/cardc-mtgish-fetch.sh
