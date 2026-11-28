package kgen

import (
	"errors"

	"github.com/adams-shaun/gorge/engine/cardc/forge"
	"github.com/adams-shaun/gorge/engine/cardc/ir"
	"github.com/adams-shaun/gorge/engine/cards"
)

// filterReader selects how a pool reads filter texts: the string reader
// (parseFilter, the default) or the typed interface (forge.LowerFilter mapped
// by lFilterFromIR), a comparison path whose tables must not change.
type filterReader uint8

const (
	filterReadString filterReader = iota // the default
	filterReadTyped
)

// defaultFilterReader is the reader every pool this process loads starts
// with: the -filter-ir flag (main.go) sets it to filterReadTyped.
var defaultFilterReader = filterReadString

// filterStats counts a typed-reader pool's filter reads: typed is a read the
// typed path answered; fallbackFrontEnd one the front end refused
// (refusedBy, indexed by forge.RefusalKind); fallbackReader one whose
// ir.Filter has a node lFilterFromIR does not map. Both fallbacks re-read
// the text with the string reader.
type filterStats struct {
	typed, fallbackFrontEnd, fallbackReader int
	refusedBy                               [16]int
}

// errNoTypedReading marks an ir.Filter node the typed reader has no mapping
// for; the read falls back to the string reader.
var errNoTypedReading = errors.New("kgen: no typed reading")

func init() {
	// kgen is the first binary whose cards answer FilterOf: the slot holds
	// the front end's reading of the parameter's text.
	cards.RegisterFilterBinder("forge", func(text string) (*ir.Filter, string) {
		f, r := forge.LowerFilter(text)
		if r != nil {
			return nil, r.Kind.String()
		}
		return f, ""
	})
}

// parseFilterText reads one filter text with the pool's reader.
func (lc *lowerCtx) parseFilterText(expr string) (*lFilter, error) {
	if lc.p.filterReader != filterReadTyped {
		return parseFilter(expr, lc.known, lc.p.isTypeWord)
	}
	return readFilterTyped(expr, lc.known, lc.p.isTypeWord, &lc.p.filterStats)
}

// readFilterTyped is the typed branch of parseFilterText: the front end's
// reading mapped by lFilterFromIR, the string reader where either has none
// (counted in st). A capacity refusal is the typed reader's decision, but its
// text is the string reader's (which names the alternative), so a refused
// card's coverage reason reads the same on both paths.
func readFilterTyped(expr string, knownSub, isType func(string) bool, st *filterStats) (*lFilter, error) {
	irf, ref := forge.LowerFilter(expr)
	if ref != nil {
		st.fallbackFrontEnd++
		if int(ref.Kind) < len(st.refusedBy) {
			st.refusedBy[ref.Kind]++
		}
		return parseFilter(expr, knownSub, isType)
	}
	lf, err := lFilterFromIR(irf, knownSub)
	if errors.Is(err, errNoTypedReading) {
		st.fallbackReader++
		return parseFilter(expr, knownSub, isType)
	}
	if err != nil {
		if _, serr := parseFilter(expr, knownSub, isType); serr != nil {
			return nil, serr
		}
		return nil, err
	}
	st.typed++
	return lf, nil
}

// lFilterFromIR maps the front end's reading of a filter to the lFilter the
// string reader builds for the same text: one clause per object alternative,
// the player alternatives in Players. A node outside the mapping is
// errNoTypedReading; a clause setter's capacity refusal passes through.
func lFilterFromIR(f *ir.Filter, knownSub func(string) bool) (*lFilter, error) {
	if f == nil {
		return nil, errNoTypedReading
	}
	alts := []*ir.Filter{f}
	if f.Op == ir.FAny && !isAnyTargetIR(f) {
		alts = f.Args
	}
	out := &lFilter{}
	for _, alt := range alts {
		if pl, ok, err := playerAltFromIR(alt); ok || err != nil {
			if err != nil {
				return nil, err
			}
			out.Players |= pl
			continue
		}
		if c, ok := anyTargetAltFromIR(alt); ok {
			out.Players |= plBoth
			out.Clauses = append(out.Clauses, c)
			continue
		}
		c, err := clauseFromIR(alt, knownSub)
		if err != nil {
			return nil, err
		}
		out.Clauses = append(out.Clauses, c)
	}
	return out, nil
}

// playerAltFromIR maps a player alternative: Player (both), Opponent and
// Player.Opponent / Player.Other (an opponent), You (you). ok is false for an
// object alternative; any other player alternative is errNoTypedReading (the
// string reader reads its head as unknown).
func playerAltFromIR(alt *ir.Filter) (uint8, bool, error) {
	if alt.Subject != ir.SubjPlayer || alt.Zones != 0 {
		return 0, false, nil
	}
	if alt.Op == ir.FPred {
		switch {
		case alt.Pred == ir.PAny && alt.Ref == nil:
			return plBoth, true, nil
		case alt.Pred == ir.POpponentOf && isRefKind(alt.Ref, ir.RefYou):
			return plOpp, true, nil
		case alt.Pred == ir.PPlayerIs && isRefKind(alt.Ref, ir.RefYou):
			return plYou, true, nil
		}
		return 0, true, errNoTypedReading
	}
	if alt.Op == ir.FAll && len(alt.Args) == 2 && isPredNode(alt.Args[0], ir.PAny) && alt.Args[0].Ref == nil {
		q := alt.Args[1]
		if isPredNode(q, ir.POpponentOf) && isRefKind(q.Ref, ir.RefYou) {
			return plOpp, true, nil // Player.Opponent
		}
		if q.Op == ir.FNot && len(q.Args) == 1 && isPredNode(q.Args[0], ir.PIs) && isRefKind(q.Args[0].Ref, ir.RefSelf) {
			return plOpp, true, nil // Player.Other: gorge reads Other as Opponent
		}
	}
	return 0, true, errNoTypedReading
}

// isAnyTargetIR reports whether f is the front end's "any target" node: a
// creature, planeswalker or battle on the battlefield, or a player.
func isAnyTargetIR(f *ir.Filter) bool {
	if f == nil || f.Op != ir.FAny || len(f.Args) != 4 {
		return false
	}
	for i, w := range [...]string{"Creature", "Planeswalker", "Battle"} {
		a := f.Args[i]
		if !isPredNode(a, ir.PCardType) || a.Word != w || a.Subject != ir.SubjPermanent ||
			a.Zones != ir.MakeZoneSet(ir.ZoneBattlefield) || a.Ref != nil {
			return false
		}
	}
	p := f.Args[3]
	return isPredNode(p, ir.PAny) && p.Subject == ir.SubjPlayer && p.Ref == nil && p.Zones == 0
}

// anyTargetAltFromIR maps the Any alternative and Any.wasDealtDamageThisTurn
// to the clause parseFilter builds for them (the players ride in Players).
func anyTargetAltFromIR(alt *ir.Filter) (lClause, bool) {
	if isAnyTargetIR(alt) {
		return lClause{Types: tCreature | tPlaneswalker}, true
	}
	if alt.Op == ir.FAll && len(alt.Args) == 2 && isAnyTargetIR(alt.Args[0]) {
		q := alt.Args[1]
		if isPredNode(q, ir.PDealtDamageThisTurn) && q.Ref == nil && q.Zones == 0 {
			return lClause{Types: tCreature | tPlaneswalker, Flags: fWasDamaged}, true
		}
	}
	return lClause{}, false
}

func isPredNode(n *ir.Filter, p ir.Pred) bool { return n != nil && n.Op == ir.FPred && n.Pred == p }

func isRefKind(r *ir.Ref, k ir.RefKind) bool {
	return r != nil && r.Kind == k && r.Of == nil && r.Filter == nil
}

// typedSpellingDistinctions are the nodes whose Forge spellings the string
// reader tells apart and the interface reads as one, where mapping the most
// common spelling changed a table: lFilterFromIR returns errNoTypedReading
// for each, so the string reader reads them.
var typedSpellingDistinctions = []string{
	"EnchantedBy / EquippedBy / AttachedBy (positive): the string reader builds fEnchantedBy for one and fEquippedBy for the other two, " +
		"and later checks tell the flags apart (edh's table and Sigiled Sword of Valeron changed); the negation (fNotAttachedBy) stays typed",
}

// typedCollapsedSpellings are the nodes the interface reads as one while the
// string reader tells the spellings apart, each mapped to the corpus's most
// common spelling; fdn's and blb's tables come out the same
// (TestTypedFilterTablesByteEqual logs them).
var typedCollapsedSpellings = []string{
	"untapped / !tapped -> untapped",
	"withoutFlying / !withFlying -> withoutFlying",
	"YouDontCtrl / !YouCtrl -> YouDontCtrl",
	"faceUp / !faceDown -> faceUp",
	"hasKeywordFlash / withFlash -> hasKeywordFlash",
	"Spell.<x> / Spell.<x>+inZoneStack -> Spell.<x> (the Spell base carries the stack zone; the string reader's redundant " +
		"inZoneStack compare is dropped; one corpus text, which no pool lowers)",
}
