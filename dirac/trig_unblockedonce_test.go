package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestUnblockedOnceBigListener: a Tf2UnblockedOnce line (gorge Mode$
// AttackerUnblockedOnce, Coveted Jewel) fires once per combat however big
// its listener is. An animated Jewel (March of the Machines makes it a 6/6)
// at least as large as every unblocked attacker still fires in gorge; a flag
// bit shared with an event-blind gate (Tf2Evolve, read by objTrigGen for
// every event) would compare the attacker against the listener and drop it.
func TestUnblockedOnceBigListener(t *testing.T) {
	deck := []cardset.Entry{{Name: "Plains", Count: 20}}
	set := cardsettest.Set(t, "fdn")
	tbl, _, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	gain := []kop.Instr{{Op: kop.OpGainLife, Obj: kop.RefController, A: kop.Const(1)}}
	jewel, bear := CardID(len(tbl.Cards)), CardID(len(tbl.Cards)+1)
	tbl.Cards = append(tbl.Cards,
		kop.CardDef{Name: "Test Jewel", Types: kop.TCreature | kop.TArtifact, Power: 6, Toughness: 6, Abilities: []kop.Ability{
			{Kind: kop.AbTriggered, Trig: kop.Trigger{Event: kop.TrigBecomesBlocked, Flags2: kop.Tf2UnblockedOnce}, Prog: gain}}},
		kop.CardDef{Name: "Test Bear", Types: kop.TCreature, Power: 1, Toughness: 4})
	sc := newScenTable(t, tbl, Config{NoElide: true})
	if !sc.k.gen.anyUnbOnce {
		t.Fatal("genPrep.anyUnbOnce not set")
	}
	sc.perm(jewel, 1)
	a1, a2 := sc.perm(bear, 0), sc.perm(bear, 0)
	// the precondition the Evolve gate would test: the listener is at least
	// as big as each attacker in both power and toughness
	if j, b := tbl.Cards[jewel], tbl.Cards[bear]; j.Power < b.Power || j.Toughness < b.Toughness {
		t.Fatalf("listener %d/%d smaller than the attacker %d/%d", j.Power, j.Toughness, b.Power, b.Toughness)
	}
	before := DebugUnblockedOnceFires
	sc.passUntil(t, kop.StepAttackers)
	sc.do(t, AkAttack, TCode(a1), scAny)
	sc.do(t, AkAttack, TCode(a2), scAny)
	sc.do(t, AkDone, scAny, scAny)
	sc.passUntil(t, kop.StepBlockers)
	sc.do(t, AkDone, scAny, scAny)
	if got := int(sc.s.npend) + int(sc.s.sp); got != 1 {
		t.Fatalf("%d triggers for two unblocked attackers into a 6/6 listener, want 1", got)
	}
	if got := DebugUnblockedOnceFires - before; got != 1 {
		t.Fatalf("fireUnblockedOnce queued %d, want 1", got)
	}
}

// TestUnblockedOnceBitIsEventGated: Tf2UnblockedOnce shares its bit with
// Tf2SrcInPlay (TrigFlags2 is full), so only a BecomesBlocked line carrying
// it turns on the once-per-combat walk; a DamageDone SrcInPlay line or an
// Evolve ETB line does not.
func TestUnblockedOnceBitIsEventGated(t *testing.T) {
	if kop.Tf2UnblockedOnce&(kop.Tf2Evolve|kop.Tf2YourTurn|kop.Tf2OppTurn|kop.Tf2CastFromHand|kop.Tf2ResolvedLimit|kop.Tf2XBound|kop.Tf2SpentBound|kop.Tf2Convoked) != 0 {
		t.Fatal("Tf2UnblockedOnce shares a bit an event-blind trigger path reads")
	}
	line := func(ev kop.TrigEvent, f kop.TrigFlags2) *kop.Table {
		return &kop.Table{Cards: []kop.CardDef{{Name: "X", Abilities: []kop.Ability{
			{Kind: kop.AbTriggered, Trig: kop.Trigger{Event: ev, Flags2: f}}}}}}
	}
	for _, c := range []struct {
		ev   kop.TrigEvent
		f    kop.TrigFlags2
		want bool
	}{
		{kop.TrigBecomesBlocked, kop.Tf2UnblockedOnce, true},
		{kop.TrigDamageDone, kop.Tf2SrcInPlay, false},
		{kop.TrigETB, kop.Tf2Evolve, false},
	} {
		if _, once := tableUnblocked(line(c.ev, c.f)); once != c.want {
			t.Errorf("event %d flags %#x: once = %v, want %v", c.ev, c.f, once, c.want)
		}
	}
}
