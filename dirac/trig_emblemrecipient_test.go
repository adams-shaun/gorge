package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestEffectEOTEmblemRecipientCapture: an until-end-of-turn DamageDone emblem
// whose capture is the damage RECIPIENT (Arcbond: RememberObjects$ Targeted
// over ValidTarget$ Card.IsRemembered) fires only for the captured object as
// the damaged permanent, queues the captured object as its trigger object
// (the body's DamageSource$ Remembered), never for a player, and stops when
// the captured object leaves the battlefield (ExileOnMoved$ Battlefield).
// The source-capture branch keeps its old scope.
func TestEffectEOTEmblemRecipientCapture(t *testing.T) {
	deck := []cardset.Entry{{Name: "Plains", Count: 20}}
	set := cardsettest.Set(t, "fdn")
	tbl, _, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	gain := []kop.Instr{{Op: kop.OpGainLife, Obj: kop.RefController, A: kop.Const(1)}}
	// the recipient filter of the stripped ValidTarget$ Card.IsRemembered:
	// an object-only filter, as the lowering leaves it
	tbl.Filters = append(tbl.Filters, kop.Filter{Types: kop.TCreature, Zone: kop.ZoneBattlefield})
	f2 := kop.FilterID(len(tbl.Filters) - 1)
	// the recipient-captured DamageDoneOnce line lowers to a DamageDone
	// ability with the batch/per-referent flags; the hook must still fire it
	// once per Damage event (gorge's delayed path never latches the batch)
	recip := kop.Ability{Kind: kop.AbTriggered, Trig: kop.Trigger{Event: kop.TrigDamageDone, Zone: kop.ZoneCommand, Filter2: f2, Flags2: kop.Tf2Batch | kop.Tf2PerReferent}, Prog: gain}
	src := recip
	src.Trig.Filter2, src.Trig.Flags2 = 0, 0
	latent := recip
	latent.AFlags |= kop.AfLatent
	latentSrc := src
	latentSrc.AFlags |= kop.AfLatent
	base := kop.Emblem{UntilEOT: true}
	withRecipient, withSource := base, base
	withRecipient.Abilities, withRecipient.Capture, withRecipient.CaptureLapse = []kop.Ability{latent}, kop.RefTarget0, true
	withSource.Abilities, withSource.Capture = []kop.Ability{latentSrc}, kop.RefTarget0
	tbl.Emblems = append(tbl.Emblems, withRecipient, withSource)
	embRecipient, embSource := uint8(len(tbl.Emblems)-1), uint8(len(tbl.Emblems))
	spell, bear := CardID(len(tbl.Cards)), CardID(len(tbl.Cards)+1)
	tbl.Cards = append(tbl.Cards,
		kop.CardDef{Name: "Test Instant", Types: kop.TInstant, Abilities: []kop.Ability{latent}},
		kop.CardDef{Name: "Test Bear", Types: kop.TCreature, Power: 2, Toughness: 2})
	sc := newScenTable(t, tbl, Config{NoElide: true})
	if sc.k.gen == nil || sc.k.gen.embCombatEv&(1<<kop.TrigDamageDone) == 0 {
		t.Fatal("precondition: the table's emblem does not register the DamageDone hook")
	}
	srcObj := sc.perm(bear, 0) // seat 0's creature deals the damage
	hurt := sc.perm(bear, 1)   // seat 1's creature is the damaged recipient
	other := sc.perm(bear, 0)
	if !sc.onBF(srcObj) || !sc.onBF(hurt) || !sc.onBF(other) {
		t.Fatal("precondition: the damage source and both creatures must be on the battlefield")
	}
	if srcObj == hurt || hurt == other {
		t.Fatal("precondition: the test's objects must differ")
	}

	// arm puts one emblem on seat 0 with capture byte num and fires one
	// damage event from src to rcp; it returns the queued count and the
	// queued trigger object.
	arm := func(emb uint8, num objWord, rcp TCode, toPlayer bool) (int, Obj) {
		sc.s.g.emb[0][0], sc.s.g.embC[0][0], sc.s.g.embO[0][0] = emb, spell, srcObj
		sc.s.g.nemb[0] = 1
		sc.s.embSetNum(0, 0, num)
		sc.s.npend = 0
		var latch dmgLatch
		sc.s.fireEmblemsDmg(sc.k, sc.slot(srcObj), 2, toPlayer, false, rcp, false, &latch, nil)
		if sc.s.npend == 0 {
			return 0, NoObj
		}
		return int(sc.s.npend), sc.s.pend[0].trigObj
	}

	// recipient capture: only the damaged object itself, queued as the
	// trigger object the body's Remembered reads
	if n, tobj := arm(embRecipient, objWord(hurt)+1, TCode(hurt), false); n != 1 || tobj != hurt {
		t.Errorf("recipient capture, the remembered damaged object: %d triggers, trigger object %v, want 1 and %v", n, tobj, hurt)
	}
	if n, _ := arm(embRecipient, objWord(other)+1, TCode(hurt), false); n != 0 {
		t.Errorf("recipient capture, another captured object: %d triggers, want 0", n)
	}
	if n, _ := arm(embRecipient, 0, TCode(hurt), false); n != 0 {
		t.Errorf("recipient capture with no remembered object: %d triggers, want 0", n)
	}
	if n, _ := arm(embRecipient, objWord(hurt)+1, PlayerCode(1), true); n != 0 {
		t.Errorf("recipient capture, damage to a player: %d triggers, want 0", n)
	}
	// a DamageDoneOnce line (Batch|PerReferent) still queues once per Damage
	// EVENT with that event's own amount: two events to the captured object
	// queue two triggers, an event to another creature queues none
	sc.s.g.emb[0][0], sc.s.g.embC[0][0], sc.s.g.embO[0][0] = embRecipient, spell, srcObj
	sc.s.g.nemb[0] = 1
	sc.s.embSetNum(0, 0, objWord(hurt)+1)
	sc.s.npend = 0
	var latch dmgLatch
	sc.s.fireEmblemsDmg(sc.k, sc.slot(srcObj), 2, false, false, TCode(hurt), false, &latch, nil)
	sc.s.fireEmblemsDmg(sc.k, sc.slot(srcObj), 2, false, false, TCode(other), false, &latch, nil)
	sc.s.fireEmblemsDmg(sc.k, sc.slot(srcObj), 3, false, false, TCode(hurt), false, &latch, nil)
	if sc.s.npend != 2 {
		t.Fatalf("two Damage events to the captured object plus one to another creature: %d triggers, want 2", sc.s.npend)
	}
	if sc.s.pend[0].trigAmt != 2 || sc.s.pend[1].trigAmt != 3 {
		t.Errorf("per-event amounts %d, %d, want 2, 3", sc.s.pend[0].trigAmt, sc.s.pend[1].trigAmt)
	}
	// the capture lapses when the captured object leaves (ExileOnMoved)
	sc.s.g.emb[0][0], sc.s.g.embC[0][0], sc.s.g.embO[0][0] = embRecipient, spell, srcObj
	sc.s.g.nemb[0] = 1
	sc.s.embSetNum(0, 0, objWord(hurt)+1)
	sc.s.embCaptureLapse(sc.k, hurt)
	sc.s.npend = 0
	sc.s.fireEmblemsDmg(sc.k, sc.slot(srcObj), 2, false, false, TCode(hurt), false, &latch, nil)
	if sc.s.npend != 0 {
		t.Errorf("recipient capture after the captured object left: %d triggers, want 0", sc.s.npend)
	}
	// source capture keeps its old scope: the remembered source, whatever
	// the recipient
	if n, tobj := arm(embSource, objWord(srcObj)+1, TCode(hurt), false); n != 1 || tobj != srcObj {
		t.Errorf("source capture, the remembered source: %d triggers, trigger object %v, want 1 and %v", n, tobj, srcObj)
	}
	if n, _ := arm(embSource, objWord(other)+1, TCode(hurt), false); n != 0 {
		t.Errorf("source capture, another source: %d triggers, want 0", n)
	}
}
