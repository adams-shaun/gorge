package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// soulTransferOffered seats Soul Transfer in the active seat 0's hand in its
// main phase with {B}{B}{B} and Grizzly Bears (the only creature) on seat 0's
// battlefield, Dense Foliage on seat 1's when foliage is set, and reports whether the cast is
// offered. Neither graveyard holds a creature or planeswalker card, so the
// exile mode is the only mode that could be chosen.
func soulTransferOffered(t *testing.T, foliage bool) bool {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	dense := lookup(t, reg, "Dense Foliage")
	soul := lookup(t, reg, "Soul Transfer")
	bears := lookup(t, reg, "Grizzly Bears")
	e, cfg := corpusEngineCfg(t, reg, []*cards.Card{soul, bears}, []*cards.Card{dense})
	moveByName(t, e, 0, "Grizzly Bears", state.ZBattlefield)
	soulID := moveByName(t, e, 0, "Soul Transfer", state.ZHand)
	if foliage {
		cantTargetOnBoard(t, e, moveByName(t, e, 1, "Dense Foliage", state.ZBattlefield))
	}
	addMana(t, e, 0, "BBB")
	e.askPriority(0)
	offered := castOffered(e, soulID)
	replayCheck(t, e, cfg)
	return offered
}

// CR 700.2a with CR 601.2c: a mode with no legal target can't be chosen, so
// a charm with no choosable mode can't be cast. Dense Foliage's ValidSA$
// Spell shields every creature from Soul Transfer's exile mode, a DB link the
// offer census judges while the card is still in hand: that link is the
// spell's, not an unclassified body the restriction fails open for.
func TestCantTargetValidSASpellReadsCharmModeInHand(t *testing.T) {
	if !soulTransferOffered(t, false) {
		t.Fatal("precondition: Soul Transfer is not castable with Grizzly Bears to exile")
	}
	if soulTransferOffered(t, true) {
		t.Fatal("Soul Transfer was offered with its only target shielded by Dense Foliage (CR 700.2a)")
	}
}
