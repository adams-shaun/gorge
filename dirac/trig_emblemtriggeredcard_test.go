package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestEmblemTriggeredCardObjectCapture: a TriggeredCard capture (an Effect's
// RememberObjects$ TriggeredCard) remembers the object the parent trigger
// named, which can differ from the object of the emblem's own object event
// (Lucius: the imprinted creature leaves, the remembered card is Lucius). The
// object-event emblem hook must match its trigger filter, not the remembered
// set, hand the remembered object to the trigger as its object, and drop a
// Once emblem after it first fires. A Targeted capture still gates on the
// remembered set.
func TestEmblemTriggeredCardObjectCapture(t *testing.T) {
	deck := []cardset.Entry{{Name: "Plains", Count: 20}}
	set := cardsettest.Set(t, "fdn")
	tbl, _, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	gain := []kop.Instr{{Op: kop.OpGainLife, Obj: kop.RefController, A: kop.Const(1)}}
	trig := kop.Ability{Kind: kop.AbTriggered, Trig: kop.Trigger{Event: kop.TrigETB, Zone: kop.ZoneCommand}, Prog: gain}
	latent := trig
	latent.AFlags |= kop.AfLatent
	base := kop.Emblem{Abilities: []kop.Ability{trig}}
	triggered, targeted, once := base, base, base
	triggered.Capture = kop.RefTriggerObj
	targeted.Capture = kop.RefTarget0
	once.Capture, once.Once = kop.RefTriggerObj, true
	baseEmb := len(tbl.Emblems)
	tbl.Emblems = append(tbl.Emblems, triggered, targeted, once)
	// g.emb stores C+1 (OpCreateEmblem's embRec.idx), so the three appended
	// table entries are remembered as their index + 1.
	embTriggered, embTargeted, embOnce := uint8(baseEmb+1), uint8(baseEmb+2), uint8(baseEmb+3)
	spell, bear := CardID(len(tbl.Cards)), CardID(len(tbl.Cards)+1)
	tbl.Cards = append(tbl.Cards,
		kop.CardDef{Name: "Test Sorcery", Types: kop.TSorcery, Abilities: []kop.Ability{latent}},
		kop.CardDef{Name: "Test Bear", Types: kop.TCreature, Power: 2, Toughness: 2})
	sc := newScenTable(t, tbl, Config{NoElide: true})
	if sc.k.gen == nil || sc.k.gen.embObjEv&(1<<kop.TrigETB) == 0 {
		t.Fatal("precondition: the table's emblem does not register the ETB hook")
	}
	captured := sc.perm(bear, 0) // the parent trigger's object
	eventObj := sc.perm(bear, 1) // the emblem trigger's event object
	if captured == eventObj || !sc.onBF(captured) || !sc.onBF(eventObj) {
		t.Fatal("precondition: the two objects must differ and be on the battlefield")
	}

	// arm puts one emblem on seat 0 remembering num; fire queues one ETB
	// event for eventObj and returns how many triggers it queued.
	arm := func(emb uint8, num Obj) {
		sc.s.g.emb[0][0], sc.s.g.embC[0][0], sc.s.g.embO[0][0] = emb, spell, captured
		sc.s.g.nemb[0] = 1
		sc.s.embSetNum(0, 0, objWord(num)+1)
	}
	fire := func() int {
		sc.s.npend = 0
		sc.s.fireObj(sc.k, kop.TrigETB, sc.slot(eventObj), eventObj, 1, 0)
		return int(sc.s.npend)
	}

	// TriggeredCard: the event object need not be the remembered object; the
	// remembered object is the trigger object the body reads.
	arm(embTriggered, captured)
	if n := fire(); n != 1 {
		t.Fatalf("TriggeredCard capture, a different event object: %d triggers, want 1", n)
	} else if got := sc.s.pend[0].trigObj; got != captured {
		t.Errorf("TriggeredCard capture trigger object %v, want the remembered %v (not the event object %v)", got, captured, eventObj)
	}
	// Targeted: the trigger still fires only for the remembered object.
	arm(embTargeted, captured)
	if n := fire(); n != 0 {
		t.Errorf("Targeted capture, a different event object: %d triggers, want 0", n)
	}
	arm(embTargeted, eventObj)
	if n := fire(); n != 1 {
		t.Fatalf("Targeted capture, the remembered event object: %d triggers, want 1", n)
	} else if got := sc.s.pend[0].trigObj; got != eventObj {
		t.Errorf("Targeted capture trigger object %v, want %v", got, eventObj)
	}
	// Once: the TriggeredCard emblem is dropped after it first fires.
	arm(embOnce, captured)
	if n := fire(); n != 1 {
		t.Fatalf("Once TriggeredCard capture, first event: %d triggers, want 1", n)
	}
	if sc.s.g.nemb[0] != 0 {
		t.Fatalf("%d emblems after the Once trigger fired, want 0", sc.s.g.nemb[0])
	}
	if n := fire(); n != 0 {
		t.Errorf("Once TriggeredCard capture, a second event: %d triggers, want 0", n)
	}
}
