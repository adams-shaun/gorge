package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
)

// Jace, Cunning Castaway and Vigorous Charge lockstep
// (dirac/kgen/lower_zone.go emblem: an end-of-turn Effect spelled
// Duration$ EndOfTurn over Mode$ DamageDoneOnce, and a Targeted capture over
// Mode$ DamageDealtOnce; kernel fireEmblemsDmgFrom). Each deck is vanilla 2/2
// attackers beside eight copies of one card, so its emblem is live in combat.
func effDamageOnceDecks(card, land string) [2][]cardset.Entry {
	a := []cardset.Entry{
		{Name: card, Count: 8}, {Name: "Grizzly Bears", Count: 14}, {Name: land, Count: 9}, {Name: "Plains", Count: 9},
	}
	return [2][]cardset.Entry{a, append([]cardset.Entry(nil), a...)}
}

var effDamageOnceCards = []struct{ card, land string }{
	{"Jace, Cunning Castaway", "Island"}, {"Vigorous Charge", "Forest"},
}

func TestEffDamageOnceLockstep(t *testing.T) {
	for _, c := range effDamageOnceCards {
		st := &dirac.EffEOTTrigStats{}
		dirac.DebugEffEOTTrigStats = st
		lockstepRun(t, "effdamageonce", effDamageOnceDecks(c.card, c.land), 300, true, nil)
		dirac.DebugEffEOTTrigStats = nil
		if st.DmgFires == 0 {
			t.Fatalf("%s: no damage emblem trigger was queued in the games; the lockstep proves nothing", c.card)
		}
		t.Logf("%s: damage emblem fires %d", c.card, st.DmgFires)
	}
}

// TestEffDamageOnceLockstepMutation: with the emblem's trigger handed to the
// other seat (Jace's draw and discard) or its damage amount zeroed (the
// life-gain bodies), each card's lockstep diverges on some seed.
func TestEffDamageOnceLockstepMutation(t *testing.T) {
	for _, c := range effDamageOnceCards {
		dirac.DebugEffEOTTrigOppOwner, dirac.DebugInnerLightNoAmount = c.card == "Jace, Cunning Castaway", c.card != "Jace, Cunning Castaway"
		div := lockstepRun(t, "effdamageonce", effDamageOnceDecks(c.card, c.land), 300, false, nil)
		dirac.DebugEffEOTTrigOppOwner, dirac.DebugInnerLightNoAmount = false, false
		if div == 0 {
			t.Errorf("%s: mutating the damage emblem left the lockstep clean", c.card)
		}
	}
}
