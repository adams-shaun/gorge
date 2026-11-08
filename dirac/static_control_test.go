package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// CR 613 layer 2 scenarios: layer 2 control records and durations
// (gorge engine/rules/control.go RegisterControl / expireControl,
// control_static.go reconcileControlStatics). FDN has no permanent or
// source-bound steal and no control static, so the table here is FDN's
// with Witness Protection's statics replaced by one StatControl static
// over its own "enchanted creature" filter (a synthetic Control Magic).

type l5scen struct {
	sc *scen
	id map[string]CardID
}

func newL5(t *testing.T) *l5scen {
	deck := []cardset.Entry{{Name: "Witness Protection", Count: 1}, {Name: "Ambush Wolf", Count: 2},
		{Name: "Burglar Rat", Count: 1}, {Name: "Plains", Count: 10}}
	fdnSet := cardsettest.Set(t, "fdn")
	for _, e := range deck {
		if i, ok := fdnSet.Lookup(e.Name); !ok || !fdnSet.Supported()[i] {
			t.Skipf("%s not in the FDN table", e.Name)
		}
	}
	tbl, da, _, err := fdnSet.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	l := &l5scen{id: map[string]CardID{}}
	for _, c := range da {
		l.id[tbl.Cards[c].Name] = c
	}
	cs := append([]kop.CardDef(nil), tbl.Cards...)
	wp := &cs[l.id["Witness Protection"]]
	var f kop.FilterID
	var abs []kop.Ability
	for _, a := range wp.Abilities {
		if a.Kind == kop.AbStatic && a.Static.Kind == kop.StatOverride {
			f = a.Static.Filter
			continue
		}
		abs = append(abs, a)
	}
	if f == 0 {
		t.Fatal("Witness Protection has no override static")
	}
	wp.Abilities = append(abs, kop.Ability{Kind: kop.AbStatic, Static: kop.Static{Kind: kop.StatControl, Filter: f}})
	t2 := *tbl
	t2.Cards = cs
	l.sc = newScenTable(t, &t2, Config{NoElide: true})
	if !l.sc.k.gen.ctrlSBA || !l.sc.k.gen.has(l.id["Witness Protection"], gfCtrlStat) {
		t.Fatal("a StatControl static leaves the control gates off")
	}
	return l
}

func (l *l5scen) ctrl(o Obj) uint8 { return l.sc.s.perms[l.sc.slot(o)].ctrl }

func (l *l5scen) gain(o Obj, to, dur uint8, src Obj) {
	s := l.sc.s
	var ver uint8
	if src != NoObj {
		ver = s.objs[src].ver
	}
	if !s.gainControl(l.sc.slot(o), to, to, dur, src, ver, false) {
		panic("pool full")
	}
}

// TestL5TwoControlDurations: two control changes with different
// durations; one expires (the layer 2 scenario). The newest surviving
// record decides; an expired newest record hands the object to the
// survivor's controller, not to its owner (CR 613.7, gorge expireControl).
func TestL5TwoControlDurations(t *testing.T) {
	// a permanent steal by p1, then p0 takes it back until end of turn:
	// at cleanup it goes back to p1
	l := newL5(t)
	w := l.sc.perm(l.id["Ambush Wolf"], 0)
	l.gain(w, 1, kop.DurPermanent, NoObj)
	l.gain(w, 0, kop.DurEOT, NoObj)
	if l.ctrl(w) != 0 {
		t.Fatalf("after the EOT steal back: controller %d, want 0", l.ctrl(w))
	}
	l.sc.s.ctrlCleanup(l.sc.k)
	if l.ctrl(w) != 1 {
		t.Fatalf("after cleanup: controller %d, want 1 (the permanent record survives)", l.ctrl(w))
	}
	if l.sc.s.ctrlSBA(l.sc.k); l.ctrl(w) != 1 {
		t.Fatalf("a later check moved it: controller %d", l.ctrl(w))
	}

	// an EOT steal by p1, then p1 gains it permanently (no visible move,
	// still a record): the EOT end moves nothing
	l = newL5(t)
	w = l.sc.perm(l.id["Ambush Wolf"], 0)
	l.gain(w, 1, kop.DurEOT, NoObj)
	l.gain(w, 1, kop.DurPermanent, NoObj)
	l.sc.s.ctrlCleanup(l.sc.k)
	if l.ctrl(w) != 1 {
		t.Fatalf("EOT then permanent: controller %d after cleanup, want 1", l.ctrl(w))
	}
	if n := l.records(); n != 1 {
		t.Fatalf("%d control records after cleanup, want the permanent one", n)
	}
}

func (l *l5scen) records() int {
	n := 0
	for i := 0; i < int(l.sc.s.g.nfx); i++ {
		if l.sc.s.g.fx[i].kind == fxControl {
			n++
		}
	}
	return n
}

// TestL5WhileSource: "for as long as you control <source>" (LoseControl$
// LeavesPlay,LoseControl) ends at the state-based check after the source
// leaves, or after its controller changes; "for as long as <source>
// remains on the battlefield" (LeavesPlay) survives the source's change of
// controller.
func TestL5WhileSource(t *testing.T) {
	for _, tc := range []struct {
		dur      uint8
		leave    bool
		wantBack bool
	}{
		{kop.DurWhileControlSource, true, true},
		{kop.DurWhileControlSource, false, true},
		{kop.DurWhileSource, true, true},
		{kop.DurWhileSource, false, false},
	} {
		l := newL5(t)
		k := l.sc.k
		w := l.sc.perm(l.id["Ambush Wolf"], 0)
		src := l.sc.perm(l.id["Burglar Rat"], 1)
		l.gain(w, 1, tc.dur, src)
		if l.ctrl(w) != 1 {
			t.Fatalf("%+v: steal did not move control", tc)
		}
		if tc.leave {
			l.sc.s.removePerm(k, l.sc.slot(src), kop.ZoneGraveyard, false)
		} else {
			l.gain(src, 0, kop.DurPermanent, NoObj)
		}
		l.sc.s.sba(k)
		if back := l.ctrl(w) == 0; back != tc.wantBack {
			t.Fatalf("%+v: wolf controller %d", tc, l.ctrl(w))
		}
	}
}

// TestL5UntilEndOfYourNextTurn: LoseControl$ UntilTheEndOfYourNextTurn
// made on its controller's turn survives that turn's cleanup and ends at
// the cleanup of their next turn; made on the opponent's turn it ends at
// the controller's next cleanup (gorge untilTurn = nextTurnFor(You)).
func TestL5UntilEndOfYourNextTurn(t *testing.T) {
	for _, made := range []uint8{1, 0} {
		l := newL5(t)
		s := l.sc.s
		w := l.sc.perm(l.id["Ambush Wolf"], 0)
		s.active = made
		l.gain(w, 1, kop.DurYourNextTurn, NoObj)
		turns := []uint8{made}
		if made == 1 {
			turns = append(turns, 0)
		}
		for _, a := range turns {
			s.active = a
			s.ctrlCleanup(l.sc.k)
			if l.ctrl(w) != 1 {
				t.Fatalf("made on p%d's turn: ended at p%d's cleanup", made, a)
			}
		}
		s.active = 1
		s.ctrlCleanup(l.sc.k)
		if l.ctrl(w) != 0 {
			t.Fatalf("made on p%d's turn: still p%d's after p1's next cleanup", made, l.ctrl(w))
		}
	}
}

// TestL5ControlStatic: an Aura's "You control enchanted creature" takes
// the creature at the state-based check, a newer until-EOT steal on top
// returns it to the Aura's controller (not its owner), and the Aura
// leaving returns it to its owner (gorge reconcileControlStatics +
// staticGrantLive).
func TestL5ControlStatic(t *testing.T) {
	l := newL5(t)
	s, k := l.sc.s, l.sc.k
	w := l.sc.perm(l.id["Ambush Wolf"], 0)
	a := l.sc.perm(l.id["Witness Protection"], 1)
	s.attachTo(k, l.sc.slot(a), l.sc.slot(w))
	s.sba(k)
	if l.ctrl(w) != 1 {
		t.Fatalf("enchanted: controller %d, want the Aura's 1", l.ctrl(w))
	}
	l.gain(w, 0, kop.DurEOT, NoObj)
	if s.sba(k); l.ctrl(w) != 0 {
		t.Fatalf("the standing static took back a newer steal: controller %d", l.ctrl(w))
	}
	s.ctrlCleanup(k)
	if l.ctrl(w) != 1 {
		t.Fatalf("after the EOT steal ended: controller %d, want 1", l.ctrl(w))
	}
	s.sba(k)
	if n := l.records(); n != 1 {
		t.Fatalf("%d control records, want the static's one", n)
	}
	s.removePerm(k, l.sc.slot(a), kop.ZoneGraveyard, false)
	s.sba(k)
	if l.ctrl(w) != 0 {
		t.Fatalf("Aura gone: controller %d, want the owner 0", l.ctrl(w))
	}
	if n := l.records(); n != 0 {
		t.Fatalf("%d control records left", n)
	}
}

// TestL5TwoControlStatics: two control Auras on one host. Every live static
// registers its own record, in the wanted-set order, and the later Aura's
// record is the latest one, so it controls the host (CR 613.7); the older
// Aura's record stays underneath, so removing the newer one would hand the
// host straight back to the older controller instead of the owner.
func TestL5TwoControlStatics(t *testing.T) {
	l := newL5(t)
	s, k := l.sc.s, l.sc.k
	w := l.sc.perm(l.id["Ambush Wolf"], 0)
	a0 := l.sc.perm(l.id["Witness Protection"], 0)
	a1 := l.sc.perm(l.id["Witness Protection"], 1)
	s.attachTo(k, l.sc.slot(a0), l.sc.slot(w))
	s.attachTo(k, l.sc.slot(a1), l.sc.slot(w))
	s.sba(k)
	// Both Auras register (a0's grant under a1's), and the later one (a1)
	// decides.
	if got := l.ctrl(w); got != 1 {
		t.Fatalf("two control statics: controller %d, want 1 (the later Aura, CR 613.7)", got)
	}
	if got := l.records(); got != 2 {
		t.Fatalf("control records %d, want 2", got)
	}
	s.removePerm(k, l.sc.slot(a0), kop.ZoneGraveyard, false)
	s.sba(k)
	if got := l.ctrl(w); got != 1 {
		t.Fatalf("after the older Aura left: controller %d, want 1", got)
	}
}

// TestL5FourControlStatics: the wanted set is ordered by each player's
// battlefield list, not by slot. A freed slot lets a later Aura enter below
// an older one, so a slot-order scan registers the grants in a different
// order and ctrlExpire later hands the host to a different survivor.
func TestL5FourControlStatics(t *testing.T) {
	l := newL5(t)
	s, k := l.sc.s, l.sc.k
	w := l.sc.perm(l.id["Ambush Wolf"], 1)
	x := l.sc.perm(l.id["Ambush Wolf"], 0)
	y := l.sc.perm(l.id["Ambush Wolf"], 0)
	a0 := l.sc.perm(l.id["Witness Protection"], 0)
	s.removePerm(k, l.sc.slot(y), kop.ZoneGraveyard, false)
	a1 := l.sc.perm(l.id["Witness Protection"], 0)
	a2 := l.sc.perm(l.id["Witness Protection"], 0)
	s.removePerm(k, l.sc.slot(x), kop.ZoneGraveyard, false)
	a3 := l.sc.perm(l.id["Witness Protection"], 1)
	// Slots: w 0, a3 1, a1 2, a0 3, a2 4; battlefield order a0, a1, a2, a3.
	if got := [4]Slot{l.sc.slot(a3), l.sc.slot(a1), l.sc.slot(a0), l.sc.slot(a2)}; got != [4]Slot{1, 2, 3, 4} {
		t.Fatalf("slots %v, want [1 2 3 4]", got)
	}
	for _, a := range []Obj{a3, a0, a1, a2} {
		s.attachTo(k, l.sc.slot(a), l.sc.slot(w))
	}
	s.sba(k)
	// Every Aura registers in battlefield order a0, a1, a2, a3, so a3's
	// record is the latest and it controls the host (CR 613.7). A slot-order
	// walk would end with a0 instead.
	if got := l.ctrl(w); got != 1 {
		t.Fatalf("four control statics: controller %d, want 1 (a3, the latest record)", got)
	}
	if got := l.records(); got != 4 {
		t.Fatalf("control records %d, want 4", got)
	}
	s.removePerm(k, l.sc.slot(a1), kop.ZoneGraveyard, false)
	s.sba(k)
	if got := l.ctrl(w); got != 1 {
		t.Fatalf("after a1 left: controller %d, want 1 (a3's surviving grant)", got)
	}
	if got := l.records(); got != 3 {
		t.Fatalf("control records %d after a1 left, want 3", got)
	}
	s.removePerm(k, l.sc.slot(a2), kop.ZoneGraveyard, false)
	s.sba(k)
	if got := l.ctrl(w); got != 1 {
		t.Fatalf("after a2 left: controller %d, want 1 (a3's surviving grant)", got)
	}
	if got := l.records(); got != 2 {
		t.Fatalf("control records %d after a2 left, want 2", got)
	}
}

// TestL5ControlChangeLeavesCombat: a permanent that changes controller is
// removed from combat (CR 506.4; gorge changeControl).
func TestL5ControlChangeLeavesCombat(t *testing.T) {
	l := newL5(t)
	s := l.sc.s
	w := l.sc.perm(l.id["Ambush Wolf"], 0)
	sl := l.sc.slot(w)
	s.perms[sl].flags |= pfAttacking
	s.nAttackers = 1
	l.gain(w, 1, kop.DurEOT, NoObj)
	if s.perms[sl].flags&pfAttacking != 0 || s.nAttackers != 0 {
		t.Fatalf("still attacking after the steal (nAttackers %d)", s.nAttackers)
	}
	if s.perms[sl].flags&pfSick == 0 {
		t.Fatal("a new controller's creature is not summoning sick")
	}
}

// TestL5BeastbinderUntilYourNextTurn: Azure Beastbinder's override records
// (loses all abilities, base 2/2) last until its controller's next turn:
// they survive that controller's cleanup and end at the cleanup before
// their next turn (gorge UntilYourNextTurn: UntilTurn = next - 1).
func TestL5BeastbinderUntilYourNextTurn(t *testing.T) {
	deck := []cardset.Entry{{Name: "Azure Beastbinder", Count: 1}, {Name: "Plains", Count: 10}}
	blbSet := cardsettest.Set(t, "blb")
	if i, ok := blbSet.Lookup("Azure Beastbinder"); !ok || !blbSet.Supported()[i] {
		t.Skip("Azure Beastbinder not in the BLB table")
	}
	tbl, da, _, err := blbSet.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	bb := da[0]
	var prog []kop.Instr
	for _, a := range tbl.Cards[bb].Abilities {
		if a.Kind == kop.AbTriggered {
			prog = a.Prog
		}
	}
	if len(prog) != 3 || prog[0].Op != kop.OpOverride || prog[0].A.N != kop.DurUntilYourNextTurn {
		t.Fatalf("Beastbinder's program %+v", prog)
	}
	sc := newScenTable(t, tbl, Config{NoElide: true})
	s, k := sc.s, sc.k
	if !k.gen.anyNTOv {
		t.Fatal("anyNTOv off")
	}
	victim := sc.perm(bb, 1)
	sl := sc.slot(victim)
	s.active = 0
	s.res.e.ctrl = 0
	s.applyOverrideFl(k, sl, int(prog[0].C), bb, s.ovDurFl(prog[0].A.N))
	s.applyOverrideFl(k, sl, int(prog[2].C), bb, s.ovDurFl(prog[2].A.N))
	pt := func() (int, int) { return s.pt(k, sl) }
	if p, tg := pt(); !s.genLost(k, sl) || p != 2 || tg != 2 {
		t.Fatalf("after the trigger: lost %v, %d/%d", s.genLost(k, sl), p, tg)
	}
	s.cleanupGen(k) // p0's own cleanup: p1's turn is next
	if p, _ := pt(); !s.genLost(k, sl) || p != 2 {
		t.Fatal("the effect ended at its controller's own cleanup")
	}
	s.active = 1
	s.cleanupGen(k) // the cleanup before p0's next turn
	if p, tg := pt(); s.genLost(k, sl) || p != 1 || tg != 3 {
		t.Fatalf("after p1's cleanup: lost %v, %d/%d, want 1/3 with its abilities", s.genLost(k, sl), p, tg)
	}
}
