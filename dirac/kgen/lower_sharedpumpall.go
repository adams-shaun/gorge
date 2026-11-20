package kgen

import (
	"math/bits"
	"sort"
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// Internal filter words of the SharedKeywordsZone$ PumpAll lowering. They are
// not Forge spellings: the lowering builds the clause directly, and the words
// exist so the same presence read can be written in a filter string.
const (
	sharedLandwalkWord = "hasKeywordLandwalk:"
	sharedProtWord     = "hasKeywordProtection:"
)

// sharedPumpAll lowers a keyword-only PumpAll with SharedKeywordsZone$
// Battlefield ("creatures you control gain flying until end of turn if a
// creature you control has flying", Odric, Lunarch Marshal; Concerted Effort).
// gorge's sharedPumpGrant (engine/effects/pump_params.go) narrows the KW$
// list at resolution: a keyword is granted only when a card in the zone
// matching SharedRestrictions$ has it in its derived keywords. This lowering
// emits one conditional grant per keyword instead: a SkipIf over a
// battlefield-presence word, then the grant. The reads are simultaneous in
// gorge (a grant this PumpAll makes never makes a later keyword shared), and
// one grant per keyword keeps that: the condition of keyword k is read before
// k is granted, and no later condition reads k.
//
// The Landwalk and Protection words stand for the sharing card's own
// spellings, so each lowers to one conditional grant per spelling present in
// the pool. A spelling the filter grammar cannot lower is not shared (the
// kernel has no read for it; the ticket's cards use only modelled ones).
func (lc *lowerCtx) sharedPumpAll(ps *params, f *lFilter) (bool, error) {
	zone, ok := ps.get("SharedKeywordsZone")
	if !ok {
		return false, nil
	}
	zone = strings.TrimSpace(zone)
	if zone != "Battlefield" {
		return true, refuse("PumpAll SharedKeywordsZone %q", zone)
	}
	for _, k := range [...]string{"Duration", "ValidTgts", "NumAtt", "NumDef", "Defined", "RememberPumped", "PumpZone", "AtEOT"} {
		if _, ok := ps.get(k); ok {
			return true, refuse("PumpAll SharedKeywordsZone$ with %s$", k)
		}
	}
	spec := ps.str("SharedRestrictions")
	if spec == "" {
		spec = "Card"
	}
	restrict, err := lc.filter(spec)
	if err != nil {
		return true, err
	}
	if restrict.Players != 0 || len(restrict.Clauses) == 0 {
		return true, refuse("PumpAll SharedRestrictions$ %q: not an object filter", spec)
	}
	kw, ok := ps.get("KW")
	if !ok {
		return true, refuse("PumpAll SharedKeywordsZone$ without KW$")
	}
	for _, term := range strings.Split(kw, "&") {
		term = strings.TrimSpace(term)
		if term == "" {
			continue
		}
		if err := lc.sharedKeywordGrant(restrict, f, term); err != nil {
			return true, err
		}
	}
	return true, nil
}

// sharedKeywordGrant emits one conditional grant for a KW$ term.
func (lc *lowerCtx) sharedKeywordGrant(restrict, targets *lFilter, term string) error {
	switch {
	case term == "Skulk":
		return lc.sharedGuard(restrict, lClause{CmpStat: cmpWord7, CmpN: int(kop.W7WithSkulk)}, func() error {
			lc.emit(lInstr{Op: "KwMark", F: targets, C: kop.KwMarkSkulk})
			return nil
		})
	case strings.HasPrefix(term, "Landwalk"):
		return lc.sharedLandwalk(restrict, targets, term)
	case strings.HasPrefix(term, "Protection"):
		return lc.sharedProtection(restrict, targets, term)
	}
	ks, err := pumpKeywords(term)
	if err != nil {
		return err
	}
	if len(ks) != 1 {
		return refuse("PumpAll KW$ %q: one keyword per shared grant", term)
	}
	bit, ok := kwBits[term]
	if !ok {
		return refuse("keyword %q", term)
	}
	cond := lClause{CmpStat: cmpHasKw, CmpN: bits.TrailingZeros32(uint32(bit))}
	return lc.sharedGuard(restrict, cond, func() error {
		in := lInstr{Op: "PumpAll", F: targets, CS: ks[0]}
		if err := pumpAllHighKw(&in, false); err != nil {
			return err
		}
		lc.emit(in)
		return nil
	})
}

// sharedLandwalk emits one conditional landwalk grant per spelling: the KW$
// family word expands to every "Landwalk:<spec>" the pool's cards print or
// grant, and an explicit spelling is itself. The grant is an end-of-turn
// override record carrying the land filter (kop.Override.Landwalk), read the
// way a printed landwalk is.
func (lc *lowerCtx) sharedLandwalk(restrict, targets *lFilter, term string) error {
	terms := []string{term}
	if term == "Landwalk" {
		terms = lc.poolSharedTerms("Landwalk")
	}
	for _, t := range terms {
		head, param, ok := strings.Cut(t, ":")
		if !ok || head != "Landwalk" {
			continue
		}
		lf, err := lc.landwalkFilter(t, param)
		if err != nil {
			continue // a spelling the kernel cannot read is not shared
		}
		lw := lf
		if err := lc.sharedGuard(restrict, lClause{CmpStat: cmpHasLandwalk, CmpFilter: lw}, func() error {
			lc.sharedOverrideAll(targets, &lOverride{Landwalk: lw}, "shared landwalk until end of turn")
			return nil
		}); err != nil {
			return err
		}
	}
	return nil
}

// sharedProtection emits one conditional protection grant per spelling: the
// KW$ family word expands to every Protection spelling the pool prints or
// grants, and an explicit spelling is itself. Colours are the kernel's own
// protection reads (black and red keyword bits, white/blue/green quality
// clauses, "each color" and "everything" their own forms); a spec-shaped
// quality is an end-of-turn override record carrying its source filter
// (kop.Override.ProtFrom).
func (lc *lowerCtx) sharedProtection(restrict, targets *lFilter, term string) error {
	terms := []string{term}
	if term == "Protection" {
		terms = lc.poolSharedTerms("Protection")
	}
	for _, t := range terms {
		if err := lc.sharedProtectionTerm(restrict, targets, t); err != nil {
			return err
		}
	}
	return nil
}

func (lc *lowerCtx) sharedProtectionTerm(restrict, targets *lFilter, term string) error {
	if term == "Protection from everything" {
		return lc.sharedGuard(restrict, lClause{CmpStat: cmpHasKw, CmpN: bits.TrailingZeros32(uint32(kop.KwProtectionAll))}, func() error {
			in := lInstr{Op: "PumpAll", F: targets, CS: "Protection from everything"}
			if err := pumpAllHighKw(&in, false); err != nil {
				return err
			}
			lc.emit(in)
			return nil
		})
	}
	if term == "Protection from each color" {
		return lc.sharedGuard(restrict, lClause{CmpStat: cmpHasProtEachColor}, func() error {
			lc.emit(lInstr{Op: "ColorGrant", F: targets, A: cst(0), C: 12, Note: "shared protection from each colour until end of turn"})
			return nil
		})
	}
	if colour, ok := strings.CutPrefix(term, "Protection from "); ok {
		if letters, ok := protColourLetters[colour]; ok {
			// every colour spelling (black and red included) is a colour
			// clause on the override record: the kernel enforces it at the
			// block, target and damage sites, and its projection maps black
			// and red to their keyword bits (grantedProtColKw). The bit grant
			// is not used here because the kernel's block gate reads only the
			// printed colours and the quality records, not a granted bit.
			mask := colorMask(letters)
			qf := &lFilter{Clauses: []lClause{{Colors: letters}}}
			return lc.sharedGuard(restrict, lClause{CmpStat: cmpHasProtColor, CmpN: int(mask)}, func() error {
				lc.sharedOverrideAll(targets, &lOverride{ProtFrom: qf}, "shared protection until end of turn")
				return nil
			})
		}
	}
	var qf *lFilter
	var err error
	if rest, ok := strings.CutPrefix(term, "Protection:"); ok {
		spec, _, _ := strings.Cut(rest, ":")
		qf, err = lc.qualityFilter(strings.TrimSpace(spec))
	} else if rest, ok := strings.CutPrefix(term, "Protection from "); ok {
		qf, err = lc.qualityFilter(rest)
	} else {
		// a pseudo or unmodelled spelling ("ProtectionColor"): not shared
		return nil
	}
	if err != nil {
		// a quality the filter grammar cannot read (a player quality, odd or
		// even, a chosen player) is not shared
		return nil
	}
	return lc.sharedGuard(restrict, lClause{CmpStat: cmpHasProtQuality, CmpFilter: qf}, func() error {
		lc.sharedOverrideAll(targets, &lOverride{ProtFrom: qf}, "shared protection until end of turn")
		return nil
	})
}

// sharedGuard wraps one grant in a SkipIf over "a battlefield permanent
// matching restrict also matches kw". The condition reads the state before
// the grant, which is gorge's simultaneous read. lFilter clauses are OR
// alternatives, so the keyword joins every restriction clause (A or B) AND K
// = (A AND K) or (B AND K); a restriction clause that already carries a
// compared statistic refuses rather than widen.
func (lc *lowerCtx) sharedGuard(restrict *lFilter, kw lClause, grant func() error) error {
	cond := &lFilter{Players: restrict.Players}
	for _, c := range restrict.Clauses {
		if c.CmpStat != cmpNone {
			return refuse("SharedRestrictions$ clause beside a compared statistic")
		}
		c.CmpStat, c.CmpN, c.CmpFilter = kw.CmpStat, kw.CmpN, kw.CmpFilter
		cond.Clauses = append(cond.Clauses, c)
	}
	return lc.guardBlock(lCond{Kind: cPresent, F: cond, Op: opGE, A: cst(1)}, grant)
}

// sharedOverrideAll emits an end-of-turn OverrideAll record over f.
func (lc *lowerCtx) sharedOverrideAll(f *lFilter, o *lOverride, note string) {
	lc.out.overrides = append(lc.out.overrides, o)
	lc.emit(lInstr{Op: "OverrideAll", F: f, CS: "EOT", C: len(lc.out.overrides) - 1, Note: note})
}

// poolSharedTerms returns every Landwalk or Protection spelling the pool's
// cards print or grant, sorted. The pool is the table's card list, so a
// spelling no card of the pool can have is not emitted. Printed keywords,
// a Continuous static's AddKeyword$, a Pump/PumpAll's KW$ and an Animate's
// Keywords$ are read.
func (lc *lowerCtx) poolSharedTerms(family string) []string {
	if lc.p == nil {
		return nil
	}
	seen := map[string]bool{}
	add := func(s string) {
		s = strings.TrimSpace(s)
		if s == "" {
			return
		}
		if family == "Landwalk" {
			if !strings.HasPrefix(s, "Landwalk:") {
				return
			}
		} else if !strings.HasPrefix(s, "Protection from ") && !strings.HasPrefix(s, "Protection:") {
			return
		}
		seen[s] = true
	}
	for _, n := range lc.p.names {
		c := lc.p.byName[n]
		if c == nil {
			continue
		}
		for _, face := range c.Faces {
			for _, k := range face.Keywords {
				add(k)
			}
			for _, sa := range face.Abilities {
				for _, key := range [...]string{"KW", "AddKeyword", "Keywords"} {
					v, ok := sa.Params[key]
					if !ok {
						continue
					}
					for _, t := range strings.Split(v, "&") {
						add(t)
					}
				}
			}
			for _, st := range face.Statics {
				v, ok := st.Params["AddKeyword"]
				if !ok {
					continue
				}
				for _, t := range strings.Split(v, "&") {
					add(t)
				}
			}
		}
	}
	out := make([]string, 0, len(seen))
	for s := range seen {
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}
