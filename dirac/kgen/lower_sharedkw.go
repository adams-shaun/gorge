package kgen

import (
	"math/bits"
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// sharedKwKeys are the Mode$ Continuous parameters a SharedKeywordsZone$ static
// may carry: the keyword list, the zone and the restriction, and the plain
// affected / description keys. Any other effect (a P/T grant, a condition)
// beside the shared list would be copied into every per-keyword static, so it
// refuses.
var sharedKwKeys = map[string]bool{
	"Mode": true, "Affected": true, "AffectedDefined": true, "EffectZone": true, "AddKeyword": true,
	"SharedKeywordsZone": true, "SharedRestrictions": true, "Description": true,
}

// sharedKwModelled is the spellings of the protection, hexproof and trample
// families a printed creature may carry for the shared grant to be exact: the
// grant is the sharing card's OWN spelling (gorge effects.SharedKeywordSpellings),
// and the kernel grants only these (a creature's protection from white, blue or
// green has no KwMask bit; pumpKeywords refuses it).
var sharedKwModelled = map[string]bool{
	"Protection from black": true, "Protection from red": true, "Protection from everything": true,
	"Hexproof": true, "Trample": true,
}

// sharedKwProt is the protection family's modelled spellings, with the colours' N from
// kop.ExileKwProtN: each is its own static, gated on
// a matching exiled card printing that very spelling.
var sharedKwProt = []struct {
	kw string // the spelling the static grants
	n  int    // ExStaticExileKw's N
}{
	{"Protection from black", kop.ExileKwProtN(kop.ColB)},
	{"Protection from red", kop.ExileKwProtN(kop.ColR)},
	{"Protection from everything", bits.TrailingZeros32(uint32(kop.KwProtectionAll))},
}

// sharedKeywordStatics lowers a Mode$ Continuous static with
// SharedKeywordsZone$ Exile (Rayami, First of the Fallen: "as long as an
// exiled creature card with a blood counter on it has flying, Rayami has
// flying. The same is true for ..."): one Continuous static per keyword of
// AddKeyword$, each gated on some exiled card matching SharedRestrictions$
// printing that keyword (ExStaticExileKw >= 1). Gorge's statSharedKeywords
// reads the printed keywords of every exiled card matching a restriction and
// grants the card's own spelling for a family word (Protection, Hexproof,
// Trample), so the family words lower only when no creature in the pool prints
// a spelling the kernel cannot grant (poolSharesOnlyModelled); the
// battlefield, graveyard and other zones, a comma list of restrictions, a
// source-relative restriction and a condition beside the list refuse.
func (lc *lowerCtx) sharedKeywordStatics(sp *params, st *lStatic) ([]lStatic, error) {
	for k := range sp.m {
		if !sharedKwKeys[k] {
			return nil, refuse("SharedKeywordsZone$ beside %s$", k)
		}
	}
	zone := sp.str("SharedKeywordsZone")
	if zone != "Exile" {
		return nil, refuse("SharedKeywordsZone %q", zone)
	}
	spec := sp.str("SharedRestrictions")
	if spec == "" {
		spec = "Card"
	}
	if strings.Contains(spec, ",") || strings.Contains(spec, "ExiledWith") {
		return nil, refuse("SharedRestrictions %q", spec)
	}
	if st.Cond.Kind != cNone {
		return nil, refuse("SharedKeywordsZone$ with a condition")
	}
	f, err := lc.filter(spec)
	if err != nil {
		return nil, err
	}
	for _, c := range f.Clauses {
		if c.Flags&(fSelf|fOther) != 0 {
			return nil, refuse("SharedRestrictions %q is source-relative", spec)
		}
	}
	list := cards.SplitKeywordList(sp.str("AddKeyword"))
	if len(list) == 0 {
		return nil, refuse("SharedKeywordsZone$ with no AddKeyword$")
	}
	type grant struct {
		kw string // the AddKeyword$ spelling the static grants
		n  int    // ExStaticExileKw's N
	}
	var grants []grant
	famPool := false
	for _, w := range list {
		switch w {
		case "Protection":
			famPool = true
			for _, p := range sharedKwProt {
				grants = append(grants, grant{p.kw, p.n})
			}
		case "Hexproof", "Trample":
			famPool = true
			grants = append(grants, grant{w, bits.TrailingZeros32(uint32(kwBits[w]))})
		default:
			if !simpleKeywords[w] || kwBits[w] == 0 || strings.HasPrefix(w, "Protection") {
				return nil, refuse("keyword %q", w)
			}
			grants = append(grants, grant{w, bits.TrailingZeros32(uint32(kwBits[w]))})
		}
	}
	if famPool {
		if err := lc.poolSharesOnlyModelled(); err != nil {
			return nil, err
		}
	}
	for k := range sp.m {
		sp.consume(k)
	}
	var out []lStatic
	for _, g := range grants {
		m := map[string]string{}
		for k, v := range sp.m {
			if k != "SharedKeywordsZone" && k != "SharedRestrictions" {
				m[k] = v
			}
		}
		m["AddKeyword"] = g.kw
		c := *st
		c.Cond = lCond{Kind: cValue, A: lValue{Kind: vExileKw, F: f, N: g.n}, Op: opGE, B: cst(1)}
		sts, err := lc.continuous(newParams(m), &c)
		if err != nil {
			return nil, err
		}
		if sts == nil {
			sts = []lStatic{c}
		}
		out = append(out, sts...)
	}
	return out, nil
}

// poolSharesOnlyModelled refuses the family words of a shared-keyword grant
// when some creature card of the pool prints a protection, hexproof or trample
// spelling the kernel cannot grant (protection from white, blue or green,
// "Protection:<spec>", "Hexproof from <quality>", "Trample over
// planeswalkers"): exiled with the rider's counter, that card would share its
// own spelling in gorge and nothing in the kernel. The pool is the table's
// cards (lc.p.names, sorted), so the grant lowers where it is exact and is
// refused, never approximated, where it is not.
func (lc *lowerCtx) poolSharesOnlyModelled() error {
	if lc.p == nil {
		return refuse("SharedKeywordsZone$ family keyword with no pool to read")
	}
	for _, n := range lc.p.names {
		c := lc.p.byName[n]
		if c == nil {
			continue
		}
		for _, f := range c.Faces {
			if !f.IsCreature() {
				continue
			}
			for _, k := range f.Keywords {
				if sharedKwFamily(k) != "" && !sharedKwModelled[k] {
					return refuse("SharedKeywordsZone$ family keyword: %s prints %q, which the kernel cannot grant", n, k)
				}
			}
		}
	}
	return nil
}

// sharedKwFamily is the family word of a printed keyword spelling (gorge
// effects.sharedKeywordFamily): "" for a keyword that is its own spelling.
func sharedKwFamily(kw string) string {
	if strings.HasPrefix(kw, "Protection") {
		return "Protection"
	}
	switch h := cards.KeywordHead(kw); h {
	case "Landwalk", "Hexproof", "Trample":
		return h
	}
	return ""
}
