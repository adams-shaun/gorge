package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// The mid-resolution card-name choice (gorge effNameCard, engine/effects/
// cardflow.go; Demonic Consultation's `SP$ NameCard | Defined$ You |
// SubAbility$ DBExile`, Cheering Fanatic's trigger-body `DB$ NameCard |
// Defined$ You | SubAbility$ DBEffect`): the resolving controller chooses the
// name of one real card of the matchup and the answer lands on the resolving
// source as Object.ChosenName. The kernel lowers it to OpExt ExtNameCard (B.N = 0
// every nameable card of the matchup, 1 the nonland ones), the ask the
// ChGenName decision over Table.NameCardsAny (Table.NameCards) ascending,
// gorge's sorted name list over the matchup's narrowed name universe -- the
// oracle narrows the universe to the matchup's own cards whenever one of
// them asks (oracle NameUniverseFor, holdsNameChooserN), so both sides offer
// the same names in the same order (dirac/repl_namecard.go). The read beside
// it is the NamedCard filter property (kop.PredNamedCard, a DigUntil's Valid$
// or a cost static's ValidCard$), the clear gorge's `DB$ Cleanup |
// ClearNamedCard$ True` (Cheering Fanatic's next attack asks anew).
//
// Only the plain ask lowers. gorge's other NameCard parametrizations -- an
// AtRandom$ draw (h.Rand over the eligible names), a ChooseFromList$ list, a
// ChooseFromDefinedCards$ Remembered intersection, ExcludeChosen$ -- ask
// differently and refuse by name, and so does a ValidCards$ the kernel's two
// name sets cannot express. A ValidDescription$ beside an absent ValidCards$
// is gorge's SAFETY fallback (effects descriptionSpecTab): the description
// forms the table maps narrow an otherwise-unrestricted offer, so they refuse
// rather than offer the whole universe.

// nameCardMid lowers a chain's mid-resolution NameCard node.
func (lc *lowerCtx) nameCardMid(ps *params) error {
	// gorge effNameCard never reads Defined$: the ask is always the
	// resolving controller's, whatever the referent says
	ps.consume("Defined")
	for _, k := range [...]string{"AtRandom", "ChooseFromList", "ChooseFromDefinedCards", "ExcludeChosen"} {
		if v, ok := ps.get(k); ok {
			return refuse("NameCard %s$ %q", k, v)
		}
	}
	kind := 0
	switch spec := ps.str("ValidCards"); spec {
	case "":
		if d := ps.str("ValidDescription"); nameDescNarrows(d) {
			return refuse("NameCard ValidDescription$ %q: it narrows the offer as gorge's description fallback does", d)
		}
	case "Card.nonLand":
		kind = 1
	default:
		return refuse("NameCard ValidCards$ %q", spec)
	}
	lc.emit(lInstr{Op: "NameCard", B: cst(kind)})
	return nil
}

// nameDescNarrows reports whether a NameCard ValidDescription$ is one of the
// description forms gorge's fallback table maps to a predicate
// (effects descriptionSpecTab, engine/effects/namecard.go) -- the narrowing
// an absent ValidCards$ would lower the offer with. A description outside the
// table is unrestricted, as Forge's is.
func nameDescNarrows(d string) bool {
	switch strings.ToLower(strings.TrimSpace(d)) {
	case "nonland", "creature", "creature card", "artifact", "artifact card", "land", "land card",
		"nonbasic land", "card other than a basic land", "nonartifact, nonland", "noncreature, nonland":
		return true
	}
	return false
}

// buildNameCard is OpExt ExtNameCard.
func buildNameCard(ki *kop.Instr, mode int) {
	ki.Op, ki.A, ki.B = kop.OpExt, kop.Const(int(kop.ExtNameCard)), kop.Const(mode)
}

// clearNamedCard lowers a Cleanup node's ClearNamedCard$ (gorge `DB$ Cleanup |
// ClearNamedCard$ True`, Cheering Fanatic's DBCleanup): the source's chosen
// card name goes, so the next ask asks anew; a host-bound emblem's frozen
// capture keeps its own (gorge state.ContinuousEffect.ChosenName).
func (lc *lowerCtx) clearNamedCard(ps *params) error {
	v, ok := ps.get("ClearNamedCard")
	if !ok {
		return nil
	}
	if v != "True" {
		return refuse("Cleanup ClearNamedCard$ %q", v)
	}
	lc.emit(lInstr{Op: "ClearNamedCard"})
	return nil
}

// namedCardPred turns the clause's NamedCard property into kf's Pred (CR
// 201.4): the candidate shares the reading source's chosen card name,
// kop.PredNamedCard, the clause's one Pred slot (kernel filter_negations.go).
// The reads the kernel makes pass the reading source as the match context: a
// dig's reveal walk (the resolving source), a cost static's spell match (the
// static's source, or its emblem's frozen capture), and a SpellCast trigger's
// filter (the listener's own chosen byte, kernel castFilterOK).
func namedCardPred(kf *kop.Filter, c *lClause) error {
	if c.Flags&fNamedCard == 0 {
		return nil
	}
	if kf.Pred != kop.PredNone {
		return refuse("NamedCard beside another predicate")
	}
	kf.Pred = kop.PredNamedCard
	c.Flags &^= fNamedCard // consumed as the Pred, no kop flag of its own
	return nil
}
