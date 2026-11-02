# Set card lists (acceptance ratchets)

One card name per line. `dirac/kgen/ratchet_test.go` lowers each list with kgen
in-process and pins the accepted count: a fall fails, a
rise must be re-pinned in the same commit.

- `<SET>.txt`: the set's card names, from gorge-compliance's XMage manifest
  `oracle/compliance/manifests/<SET>.json`:

  ```sh
  python3 -c 'import json,sys; print("\n".join(sorted({c["name"] for c in json.load(open(sys.argv[1]))["cards"]})))' \
    <gorge-compliance checkout>/oracle/compliance/manifests/BLB.json > BLB.txt
  ```

- `ALL` is not a tracked list. It is every card in the corpus, the first `Name:`
  of each script, byte-sorted and unique. It is extracted from the GPL-3.0 corpus,
  so kgen's tests and `-census-std` write it to `.cards/kgen/ALL.txt` when they
  need it (`dirac/kgen/setlistall.go`). The same list by hand:

  ```sh
  find -L .cards/cardsfolder -name '*.txt' -exec awk '/^Name:/{sub(/^Name:/,"");sub(/\r$/,"");print;nextfile}' {} + | LC_ALL=C sort -u
  ```

TRK and TRC have no XMage manifest. Their lists are the names in the `[cards]` section of
Forge's edition files (`Star Trek.txt`, `Star Trek Commander.txt`) that have a script at the
pin, so they grow with the corpus until Forge has scripted the whole set.

## Names dropped from the manifest lists

`kgen -card-list` refuses a list naming a card the corpus lacks, so these
manifest names are removed from their lists (the XMage name differs from
Forge's by a diacritic, or the corpus has no script at the pin). Re-check
them after a `FORGE_REF` bump:

| set | dropped |
|---|---|
| DMU | Tura Kennerud, Skyknight |
| HOB | Dain Ironfoot; Dain's Company; Dain, Lord of the Iron Hills; Fili the Pathfinder; Gloin the Mighty; Kili the Resourceful; Oin the Brave; Thror's Map |
| LCI | Bartolome del Presidio |
| MSH | Mjolnir, Hammer of Thor |
| SPM | Arana, Heart of the Spider |
| TMT | Bespoke Bo |

## Synthetic evidence pools

`make dirac-tables` turns each list named in `../standard.txt`, and the corpus outside them, into a
synthetic evidence pool's table under `.cards/kernel-synth-tables/`. These lists are the only committed input:
see `dirac/docs/verification.md`. Editing a list re-keys the card tables.
