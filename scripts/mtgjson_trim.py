#!/usr/bin/env python3
"""mtgjson_trim.py -- regenerate draft/limited/setdata/mtgjson/<SET>.json.

Usage (operator; needs network, writes nothing but the output file):

    D=$(mktemp -d)   # an empty directory for the downloads
    for f in Meta.json SET.json.gz SPG.json.gz; do curl -sSLo "$D/$f" https://mtgjson.com/api/v5/$f; done
    gunzip "$D"/*.gz
    python3 -I scripts/mtgjson_trim.py "$D" SET draft/limited/setdata/mtgjson/SET.json

It keeps each set's "play" and "play-arena" booster configs and the cards
they reference (the set's own cards plus SPG special guests), with the
MTGJSON version and MIT notice recorded in the file (see LICENSE-MTGJSON.txt).
Then run `go generate ./draft/limited/setdata/...` to rebuild the specs.
"""
import json,sys
D=sys.argv[1]; code=sys.argv[2]; out=sys.argv[3]
meta=json.load(open(D+'/Meta.json'))['data']
s=json.load(open(f'{D}/{code}.json'))['data']
spg=json.load(open(f'{D}/SPG.json'))['data']
cards={c['uuid']:c for c in s['cards']}; cards.update({c['uuid']:c for c in spg['cards']})
def card(u):
    c=cards[u]
    r={"uuid":u,"name":c['name'],"set":c['setCode'],"number":c['number'],"rarity":c['rarity'],"colors":"".join(c.get('colors',[]))}
    if c.get('faceName'): r["face_name"]=c['faceName']
    return r
res={"set":code,"name":s['name'],
     "source":f"MTGJSON {meta['version']} ({meta['date']}), https://mtgjson.com/api/v5/{code}.json booster configs, trimmed to the play and play-arena configs and the cards they reference",
     "license":"MIT License, Copyright (c) 2018-Present Zachary Halpern (MTGJSON). Permission is hereby granted, free of charge, to any person obtaining a copy of this software and associated documentation files, to deal in the Software without restriction, subject to the condition that the copyright notice and this permission notice are included in all copies or substantial portions. THE SOFTWARE IS PROVIDED AS IS, WITHOUT WARRANTY OF ANY KIND.",
     "configs":{}}
missing=0
for key in ('play','play-arena'):
    cfg=s['booster'].get(key)
    if not cfg: continue
    sheets={}
    for sn,sh in cfg['sheets'].items():
        cl=[]
        for u,w in sorted(sh['cards'].items(), key=lambda kv:(cards[kv[0]]['setCode'],cards[kv[0]]['number'],kv[0]) if kv[0] in cards else ('~','~',kv[0])):
            if u not in cards: missing+=1; continue
            d=card(u); d["weight"]=w; cl.append(d)
        e={"total_weight":sh['totalWeight'],"foil":sh.get('foil',False),"cards":cl}
        for f,j in (('balanceColors','balance_colors'),('allowDuplicates','allow_duplicates'),('fixed','fixed')):
            if sh.get(f) is not None: e[j]=sh[f]
        sheets[sn]=e
    res["configs"][key]={"source_set_codes":cfg.get('sourceSetCodes',[]),"total_weight":cfg['boostersTotalWeight'],
        "variants":[{"weight":b['weight'],"contents":dict(sorted(b['contents'].items()))} for b in cfg['boosters']],"sheets":dict(sorted(sheets.items()))}
json.dump(res,open(out,'w'),indent=1,ensure_ascii=False); open(out,'a').write("\n")
print(code,"missing uuids:",missing, "configs:",list(res["configs"]))
