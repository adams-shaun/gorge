package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestEffectEOTEmblemPlayerCapture: an until-end-of-turn DamageDone emblem
// with CapturePlayer (Great Train Heist's RememberObjects$ TargetedPlayer)
// fires only for damage to the remembered seat, never for damage to a
// permanent, and an emblem with no capture (embN 0 on an uncaptured emblem)
// or an object Capture keeps its old scope.
func TestEffectEOTEmblemPlayerCapture(t *testing.T) {
	deck := []cardset.Entry{{Name: "Plains", Count: 20}}
	set := cardsettest.Set(t, "fdn")
	tbl, _, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	gain := []kop.Instr{{Op: kop.OpGainLife, Obj: kop.RefController, A: kop.Const(1)}}
	trig := kop.Ability{Kind: kop.AbTriggered, Trig: kop.Trigger{Event: kop.TrigDamageDone, Zone: kop.ZoneCommand, Flags: kop.TfToPlayer}, Prog: gain}
	latent := trig
	latent.AFlags |= kop.AfLatent
	base := kop.Emblem{Abilities: []kop.Ability{trig}, UntilEOT: true}
	withPlayer, withObj, withNone := base, base, base
	withPlayer.CapturePlayer = kop.RefTarget0
	withObj.Capture = kop.RefTarget0
	tbl.Emblems = append(tbl.Emblems, withPlayer, withObj, withNone)
	embPlayer, embObj, embNone := uint8(len(tbl.Emblems)-2), uint8(len(tbl.Emblems)-1), uint8(len(tbl.Emblems))
	spell, bear := CardID(len(tbl.Cards)), CardID(len(tbl.Cards)+1)
	tbl.Cards = append(tbl.Cards,
		kop.CardDef{Name: "Test Instant", Types: kop.TInstant, Abilities: []kop.Ability{latent}},
		kop.CardDef{Name: "Test Bear", Types: kop.TCreature, Power: 2, Toughness: 2})
	sc := newScenTable(t, tbl, Config{NoElide: true})
	if sc.k.gen == nil || sc.k.gen.embCombatEv&(1<<kop.TrigDamageDone) == 0 {
		t.Fatal("precondition: the table's emblem does not register the DamageDone hook")
	}
	src := sc.perm(bear, 0) // seat 0's creature deals the damage
	other := sc.perm(bear, 1)
	if !sc.onBF(src) || !sc.onBF(other) {
		t.Fatal("precondition: the damage source and the other creature must be on the battlefield")
	}

	// queued counts the triggers one damage event from src queues for an
	// emblem (kind) whose capture byte is num, owned by seat 0.
	queued := func(emb, num uint8, toPlayer bool, rcp uint8) int {
		sc.s.g.emb[0][0], sc.s.g.embC[0][0], sc.s.g.embO[0][0] = emb, spell, src
		sc.s.g.nemb[0] = 1
		sc.s.embSetNum(0, 0, num)
		sc.s.npend = 0
		var latch dmgLatch
		sc.s.fireEmblemsDmg(sc.k, sc.slot(src), 2, toPlayer, true, TCode(rcp), false, &latch)
		return int(sc.s.npend)
	}
	toP0, toP1 := PlayerCode(0), PlayerCode(1)
	if toP0 == toP1 {
		t.Fatal("precondition: the two recipients must differ")
	}

	// player capture: seat 1 + 1
	if got := queued(embPlayer, 2, true, uint8(toP1)); got != 1 {
		t.Errorf("player capture, damage to the remembered seat: %d triggers, want 1", got)
	}
	if got := queued(embPlayer, 2, true, uint8(toP0)); got != 0 {
		t.Errorf("player capture, damage to the other seat: %d triggers, want 0", got)
	}
	if got := queued(embPlayer, 2, false, uint8(other)); got != 0 {
		t.Errorf("player capture, damage to a permanent: %d triggers, want 0", got)
	}
	// player capture that remembered nobody (embN 0): never fires
	if got := queued(embPlayer, 0, true, uint8(toP1)); got != 0 {
		t.Errorf("player capture with no remembered seat: %d triggers, want 0", got)
	}
	// no capture: any recipient seat fires
	if got := queued(embNone, 0, true, uint8(toP1)); got != 1 {
		t.Errorf("uncaptured emblem, damage to seat 1: %d triggers, want 1", got)
	}
	// object capture: only the remembered source fires, whatever the recipient
	if got := queued(embObj, uint8(src)+1, true, uint8(toP1)); got != 1 {
		t.Errorf("object capture, the remembered source: %d triggers, want 1", got)
	}
	if got := queued(embObj, uint8(other)+1, true, uint8(toP1)); got != 0 {
		t.Errorf("object capture, another source: %d triggers, want 0", got)
	}
}
