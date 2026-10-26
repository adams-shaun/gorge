# Standard-program set card lists

One card name per line (names only, no script text). Read by
`TestEveryStandardSetParamsAreRead` (`paramcensus_sets_test.go`). Generated
from gorge-compliance's XMage manifests; names the corpus lacks are dropped
here, so a list never names a card the corpus cannot load:

    python3 -c 'import json,sys; print("\n".join(sorted({c["name"] for c in json.load(open(sys.argv[1]))["cards"]})))' \
      ~/projects/gorge-compliance/oracle/compliance/manifests/<SET>.json > <SET>.txt

Dropped names: DMU Tura Kennerud, Skyknight; HOB Dain Ironfoot, Dain's
Company, Dain, Lord of the Iron Hills, Fili the Pathfinder, Gloin the Mighty,
Kili the Resourceful, Oin the Brave, Thror's Map; LCI Bartolome del Presidio;
MSH Mjolnir, Hammer of Thor; SPM Arana, Heart of the Spider; TMT Bespoke Bo.
