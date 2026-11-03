# Standard-program set card lists

One card name per line (names only, no script text). Read by
`TestEveryStandardSetParamsAreRead` (`paramcensus_sets_test.go`). Generated
from gorge-compliance's XMage manifests, so a list never names a card the
corpus cannot load:

    python3 -c 'import json,sys; print("\n".join(sorted({c["name"] for c in json.load(open(sys.argv[1]))["cards"]})))' \
      <gorge-compliance checkout>/oracle/compliance/manifests/<SET>.json > <SET>.txt

Names the corpus lacks are dropped; a name that differs from Forge's only by a
diacritic resolves (`cards.NormalizeName` folds it), so XMage's "Dain Ironfoot"
is Forge's "Dáin Ironfoot" and stays in its list.

TRK and TRC have no XMage manifest. Their lists are the names in the `[cards]`
section of Forge's edition files that have a script at the pin, so they grow with
the corpus until Forge has scripted the whole set.
