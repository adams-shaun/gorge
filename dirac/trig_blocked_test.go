package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestBecomesBlocked: a become-blocked trigger (gorge Mode$ AttackerBlocked)
// fires once per distinct blocked attacker matching it (an attacker
// double-blocked triggers once; an unblocked one not at all), and the
// ValidBlocker$ Card.Self form only for the attacker the source blocks.
func TestBecomesBlocked(t *testing.T) {
	deck := []cardset.Entry{{Name: "Plains", Count: 20}}
	set := cardsettest.Set(t, "fdn")
	tbl, _, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	gain := []kop.Instr{{Op: kop.OpGainLife, Obj: kop.RefController, A: kop.Const(1)}}
	atkr, wall, bear := CardID(len(tbl.Cards)), CardID(len(tbl.Cards)+1), CardID(len(tbl.Cards)+2)
	tbl.Cards = append(tbl.Cards,
		kop.CardDef{Name: "Test Blocked", Types: kop.TCreature, Power: 0, Toughness: 4,
			Abilities: []kop.Ability{{Kind: kop.AbTriggered, Trig: kop.Trigger{Event: kop.TrigBecomesBlocked, Flags: kop.TfSelf}, Prog: gain}}},
		kop.CardDef{Name: "Test Wall", Types: kop.TCreature, Power: 0, Toughness: 4,
			Abilities: []kop.Ability{{Kind: kop.AbTriggered, Trig: kop.Trigger{Event: kop.TrigBecomesBlocked, Flags2: kop.Tf2BlockedBySelf}, Prog: gain}}},
		kop.CardDef{Name: "Test Bear", Types: kop.TCreature, Power: 2, Toughness: 4})
	sc := newScenTable(t, tbl, Config{NoElide: true})
	a1, a2 := sc.perm(atkr, 0), sc.perm(bear, 0)
	w1, w2, b1 := sc.perm(wall, 1), sc.perm(wall, 1), sc.perm(bear, 1)
	sc.combat(t, []Obj{a1, a2}, [][2]Obj{{w1, a1}, {b1, a1}})
	_ = w2
	// a1 blocked (twice): its trigger once (+1); a2 (a Bear) unblocked deals 2.
	// w1 blocks a1: +1 to seat 1; w2 blocks nothing.
	if l0, l1 := sc.s.pl[0].life, sc.s.pl[1].life; l0 != 21 || l1 != 19 {
		t.Fatalf("life %d / %d, want 21 / 19", l0, l1)
	}
}

// TestBlocksTrigger: a blocks trigger (gorge Mode$ Blocks) fires once per
// declared pair whose blocker matches (TfSelf: the listener blocks), and
// a global "whenever a creature blocks" once per blocking creature.
func TestBlocksTrigger(t *testing.T) {
	deck := []cardset.Entry{{Name: "Plains", Count: 20}}
	set := cardsettest.Set(t, "fdn")
	tbl, _, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	gain := []kop.Instr{{Op: kop.OpGainLife, Obj: kop.RefController, A: kop.Const(1)}}
	wall, watcher, bear := CardID(len(tbl.Cards)), CardID(len(tbl.Cards)+1), CardID(len(tbl.Cards)+2)
	tbl.Cards = append(tbl.Cards,
		kop.CardDef{Name: "Test Wall", Types: kop.TCreature, Power: 0, Toughness: 4,
			Abilities: []kop.Ability{{Kind: kop.AbTriggered, Trig: kop.Trigger{Event: kop.TrigBlocks, Flags: kop.TfSelf}, Prog: gain}}},
		kop.CardDef{Name: "Test Watcher", Types: kop.TEnchantment,
			Abilities: []kop.Ability{{Kind: kop.AbTriggered, Trig: kop.Trigger{Event: kop.TrigBlocks}, Prog: gain}}},
		kop.CardDef{Name: "Test Bear", Types: kop.TCreature, Power: 2, Toughness: 4})
	sc := newScenTable(t, tbl, Config{NoElide: true})
	a1, a2 := sc.perm(bear, 0), sc.perm(bear, 0)
	sc.perm(watcher, 0)
	w1, w2, b1 := sc.perm(wall, 1), sc.perm(wall, 1), sc.perm(bear, 1)
	_ = w2
	sc.combat(t, []Obj{a1, a2}, [][2]Obj{{w1, a1}, {b1, a1}})
	// w1 blocks (its own trigger; w2 blocks nothing) and the watcher sees
	// two blocking creatures: three triggers, each with the attacker a1 as
	// its trigger object (gorge TriggerCard); the watcher's two wait on
	// their controller's order.
	if sc.s.npend != 3 {
		t.Fatalf("%d pending triggers, want 3", sc.s.npend)
	}
	for i := 0; i < int(sc.s.npend); i++ {
		if p := sc.s.pend[i]; p.trigObj != a1 {
			t.Fatalf("pending %d: trigger object %d, want the attacker %d", i, p.trigObj, a1)
		}
	}
}

// TestPerPairBlockedTrigger: Tf2PerPair (gorge Mode$ AttackerBlockedByCreature)
// fires once per declared (attacker, blocker) pair. The attacker-anchored
// half (Filter Self) fires once per blocker with that blocker as trigger
// object; the blocker-anchored half (Filter2 Self) fires for the attacker
// the listener blocks, with the attacker as trigger object.
func TestPerPairBlockedTrigger(t *testing.T) {
	deck := []cardset.Entry{{Name: "Plains", Count: 20}}
	set := cardsettest.Set(t, "fdn")
	tbl, _, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	tbl.Filters = append(tbl.Filters, kop.Filter{Types: kop.TCreature}, kop.Filter{Flags: kop.FSelf})
	crea, self := kop.FilterID(len(tbl.Filters)-2), kop.FilterID(len(tbl.Filters)-1)
	gain := []kop.Instr{{Op: kop.OpGainLife, Obj: kop.RefController, A: kop.Const(1)}}
	hound, bear := CardID(len(tbl.Cards)), CardID(len(tbl.Cards)+1)
	tbl.Cards = append(tbl.Cards,
		kop.CardDef{Name: "Test Hound", Types: kop.TCreature, Power: 2, Toughness: 4, Abilities: []kop.Ability{
			{Kind: kop.AbTriggered, Trig: kop.Trigger{Event: kop.TrigBecomesBlocked, Filter: crea, Filter2: self, Flags2: kop.Tf2PerPair}, Prog: gain},
			{Kind: kop.AbTriggered, Trig: kop.Trigger{Event: kop.TrigBecomesBlocked, Flags: kop.TfSelf, Filter2: crea, Flags2: kop.Tf2PerPair}, Prog: gain}}},
		kop.CardDef{Name: "Test Bear", Types: kop.TCreature, Power: 2, Toughness: 4})
	sc := newScenTable(t, tbl, Config{NoElide: true})
	h := sc.perm(hound, 0)
	b1, b2 := sc.perm(bear, 1), sc.perm(bear, 1)
	sc.combat(t, []Obj{h}, [][2]Obj{{b1, h}, {b2, h}})
	if sc.s.npend != 2 {
		t.Fatalf("%d pending triggers, want 2 (one per blocker)", sc.s.npend)
	}
	for i, want := range []Obj{b1, b2} {
		if p := sc.s.pend[i]; p.trigObj != want {
			t.Fatalf("pending %d: trigger object %d, want the blocker %d", i, p.trigObj, want)
		}
	}
}

// TestPerPairBlockerAnchored: the blocker-anchored half fires for the
// attacker the hound blocks, with that attacker as its trigger object.
func TestPerPairBlockerAnchored(t *testing.T) {
	deck := []cardset.Entry{{Name: "Plains", Count: 20}}
	set := cardsettest.Set(t, "fdn")
	tbl, _, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	tbl.Filters = append(tbl.Filters, kop.Filter{Types: kop.TCreature}, kop.Filter{Flags: kop.FSelf})
	crea, self := kop.FilterID(len(tbl.Filters)-2), kop.FilterID(len(tbl.Filters)-1)
	kill := []kop.Instr{{Op: kop.OpDealDamage, Obj: kop.RefTriggerObj, A: kop.Const(5)}}
	hound, bear := CardID(len(tbl.Cards)), CardID(len(tbl.Cards)+1)
	tbl.Cards = append(tbl.Cards,
		kop.CardDef{Name: "Test Hound", Types: kop.TCreature, Power: 2, Toughness: 4, Abilities: []kop.Ability{
			{Kind: kop.AbTriggered, Trig: kop.Trigger{Event: kop.TrigBecomesBlocked, Filter: crea, Filter2: self, Flags2: kop.Tf2PerPair}, Prog: kill}}},
		kop.CardDef{Name: "Test Bear", Types: kop.TCreature, Power: 2, Toughness: 4})
	sc := newScenTable(t, tbl, Config{NoElide: true})
	a1, a2 := sc.perm(bear, 0), sc.perm(bear, 0)
	h := sc.perm(hound, 1)
	sc.combat(t, []Obj{a1, a2}, [][2]Obj{{h, a2}})
	// the trigger (5 damage to its trigger object) kills the blocked a2
	if !sc.onBF(a1) || sc.onBF(a2) || !sc.onBF(h) {
		t.Fatalf("a1 %v a2 %v hound %v on the battlefield, want true false true", sc.onBF(a1), sc.onBF(a2), sc.onBF(h))
	}
}
