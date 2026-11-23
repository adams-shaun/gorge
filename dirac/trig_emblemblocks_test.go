package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestEffectEOTEmblemBlocksFires: an until-end-of-turn emblem with a TrigBlocks
// trigger (Benefactor's Draught: ValidCard$ Creature.OppCtrl) queues once per
// declared (attacker, blocker) pair whose BLOCKER matches the filter for the
// emblem's owner, with the blocker as its trigger object and the blocker's
// controller as the trigger player; the cleanup drops it; and with
// DebugIgnoreBlocksEmblem it queues nothing.
func TestEffectEOTEmblemBlocksFires(t *testing.T) {
	deck := []cardset.Entry{{Name: "Plains", Count: 20}}
	set := cardsettest.Set(t, "fdn")
	tbl, _, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	tbl.Filters = append(tbl.Filters, kop.Filter{Types: kop.TCreature, Ctrl: kop.CtrlOpp})
	opp := kop.FilterID(len(tbl.Filters) - 1)
	draw := []kop.Instr{{Op: kop.OpDraw, Obj: kop.RefController, A: kop.Const(1)}}
	trig := kop.Ability{Kind: kop.AbTriggered, Trig: kop.Trigger{Event: kop.TrigBlocks, Zone: kop.ZoneCommand, Filter: opp}, Prog: draw}
	latent := trig
	latent.AFlags |= kop.AfLatent
	tbl.Emblems = append(tbl.Emblems, kop.Emblem{UntilEOT: true, Abilities: []kop.Ability{trig}})
	emb := uint8(len(tbl.Emblems)) // emb holds the index + 1
	spell, bear := CardID(len(tbl.Cards)), CardID(len(tbl.Cards)+1)
	tbl.Cards = append(tbl.Cards,
		kop.CardDef{Name: "Test Instant", Types: kop.TInstant, Abilities: []kop.Ability{latent}},
		kop.CardDef{Name: "Test Bear", Types: kop.TCreature, Power: 2, Toughness: 2})
	sc := newScenTable(t, tbl, Config{NoElide: true})
	s, k := sc.s, sc.k
	if k.gen == nil || k.gen.embCombatEv&(1<<kop.TrigBlocks) == 0 {
		t.Fatal("precondition: the table's emblem does not register the Blocks hook")
	}
	atk0, atk1 := sc.perm(bear, 0), sc.perm(bear, 1)
	blkA, blkB := sc.perm(bear, 1), sc.perm(bear, 1) // seat 1 blocks seat 0's attacker
	own := sc.perm(bear, 0)                          // seat 0 blocks seat 1's attacker
	if blkA == blkB {
		t.Fatal("precondition: the two blockers must be distinct objects")
	}
	// declare puts blockers on the attacker's slot in the given order.
	declare := func(active uint8, atk Obj, blockers ...Obj) {
		for _, o := range []Obj{atk0, atk1, blkA, blkB, own} {
			p := &s.perms[sc.slot(o)]
			p.flags &^= pfBlocking
			p.blocking = NoSlot
		}
		s.active = active
		s.blockSeq = 0
		for _, b := range blockers {
			p := &s.perms[sc.slot(b)]
			p.flags |= pfBlocking
			p.blocking = sc.slot(atk)
			s.blockSeq++
			p.bord = objWord(s.blockSeq)
		}
	}
	own0 := func() {
		s.g.emb[0][0], s.g.embC[0][0], s.g.embO[0][0], s.g.nemb[0] = emb, spell, atk0, 1 // owned by seat 0
		s.g.embN[0][0], s.g.embT[0][0] = 0, 1
		s.npend = 0
	}

	own0()
	declare(0, atk0, blkA, blkB)
	var ps [2 * MaxPerm]blkPair
	if n := s.blockPairs(&ps); n != 2 {
		t.Fatalf("precondition: %d declared pairs, want 2", n)
	}
	s.fireBlocksGen(k)
	if s.npend != 2 {
		t.Fatalf("two opposing blockers: %d triggers queued, want 2 (one per pair)", s.npend)
	}
	for i, want := range []Obj{blkA, blkB} {
		if p := s.pend[i]; p.trigObj != want || p.trigPl != 1 || p.ctrl != 0 || p.card != spell {
			t.Errorf("trigger %d: object %d player %d controller %d card %d, want blocker %d player 1 controller 0 card %d",
				i, p.trigObj, p.trigPl, p.ctrl, p.card, want, spell)
		}
	}

	// the owner's own creature blocking (seat 1 attacks): Creature.OppCtrl misses it
	own0()
	declare(1, atk1, own)
	if n := s.blockPairs(&ps); n != 1 {
		t.Fatalf("precondition: %d declared pairs, want 1", n)
	}
	s.fireBlocksGen(k)
	if s.npend != 0 {
		t.Errorf("the emblem owner's own blocker: %d triggers queued, want 0", s.npend)
	}

	// mutation: the hook off queues nothing for the matching blockers
	own0()
	declare(0, atk0, blkA, blkB)
	DebugIgnoreBlocksEmblem = true
	s.fireBlocksGen(k)
	DebugIgnoreBlocksEmblem = false
	if s.npend != 0 {
		t.Errorf("DebugIgnoreBlocksEmblem: %d triggers queued, want 0", s.npend)
	}

	// the cleanup drops the emblem: a later declaration fires nothing
	own0()
	s.cleanupGen(k)
	if s.g.nemb[0] != 0 {
		t.Fatalf("precondition: %d emblems left after the cleanup", s.g.nemb[0])
	}
	declare(0, atk0, blkA, blkB)
	s.npend = 0
	s.fireBlocksGen(k)
	if s.npend != 0 {
		t.Errorf("after the cleanup drop: %d triggers queued, want 0", s.npend)
	}
}
