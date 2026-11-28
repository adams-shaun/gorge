# The typed card interface's make targets, included last from the Makefile.
MTGISH_REF ?= 4f6512cfb08500617b6304ae1b5baf131f158a12
OGRAM_TAG ?= faces-oracle-20261112220158-s193-g0.5.0-r1

.PHONY: mtgish-fetch ogram-fetch

# Fetch the pinned mtgish data into $(CARDS_DIR)/mtgish/<ref>/, sha256-checked.
mtgish-fetch:
	@MTGISH_REF=$(MTGISH_REF) bash scripts/cardc-mtgish-fetch.sh

# Fetch the pinned oracle-grammar release into $(CARDS_DIR)/ogram/<tag>/, sha256-checked.
ogram-fetch:
	@OGRAM_TAG=$(OGRAM_TAG) bash scripts/cardc-ogram-fetch.sh
