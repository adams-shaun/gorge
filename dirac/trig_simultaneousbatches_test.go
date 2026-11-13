package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// Four simultaneity rules the kernel's trigger queue must share with gorge
// (the trigger-order menu lists the same pending triggers): each test injects
// the watcher it needs into the fdn table, so none depends on a printed card.

type simFix struct {
	sc                                        *scen
	etb, sac, taps, legend, saga, bear, plain CardID
}

func newSimFix(t *testing.T) *simFix {
	t.Helper()
	deck := []cardset.Entry{{Name: "Plains", Count: 20}}
	set := cardsettest.Set(t, "fdn")
	tbl, _, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	tbl.Filters = append(tbl.Filters,
		kop.Filter{Types: kop.TCreature, Owner: kop.CtrlYou, Flags: kop.FOther},
		kop.Filter{Types: kop.TEnchantment, Owner: kop.CtrlYou})
	otherCreature, yourEnchantment := kop.FilterID(len(tbl.Filters)-2), kop.FilterID(len(tbl.Filters)-1)
	gain := []kop.Instr{{Op: kop.OpGainLife, Obj: kop.RefController, A: kop.Const(1)}}
	watch := func(name string, types kop.TypeMask, ev kop.TrigEvent, f kop.FilterID) kop.CardDef {
		return kop.CardDef{Name: name, Types: types, Power: 1, Toughness: 1, Abilities: []kop.Ability{{Kind: kop.AbTriggered,
			Trig: kop.Trigger{Event: ev, Filter: f}, Prog: gain}}}
	}
	n := CardID(len(tbl.Cards))
	f := &simFix{etb: n, sac: n + 1, taps: n + 2, legend: n + 3, saga: n + 4, bear: n + 5}
	tbl.Cards = append(tbl.Cards,
		watch("Sim Entry Watcher", kop.TCreature, kop.TrigETB, otherCreature),
		watch("Sim Sacrifice Watcher", kop.TCreature, kop.TrigSacrificed, otherCreature),
		watch("Sim Tap Watcher", kop.TCreature, kop.TrigTaps, otherCreature),
		watch("Sim Legend", kop.TCreature|kop.TLegendary, kop.TrigLeaves, yourEnchantment),
		kop.CardDef{Name: "Sim Saga", Types: kop.TEnchantment, SagaN: 3},
		kop.CardDef{Name: "Sim Bear", Types: kop.TCreature, Power: 2, Toughness: 2})
	f.sc = newScenTable(t, tbl, Config{NoElide: true})
	return f
}

// pendingBy counts the queued triggers whose source is o.
func (f *simFix) pendingBy(o Obj) (n int) {
	for i := 0; i < int(f.sc.s.npend); i++ {
		if f.sc.s.pend[i].obj == o {
			n++
		}
	}
	return n
}

// TestEncoreGroupSacrificeIsSimultaneous: the end-step trigger that sacrifices
// a group of tokens (Encore's copies) sacrifices them together, so each
// watcher's "whenever you sacrifice another creature" sees the other one go
// (CR 603.10a): one trigger per token, not one only for the token sacrificed
// second.
func TestEncoreGroupSacrificeIsSimultaneous(t *testing.T) {
	f := newSimFix(t)
	sc := f.sc
	s := sc.s
	a, b := sc.perm(f.sac, 0), sc.perm(f.sac, 0)
	if !s.subs[kop.TrigSacrificed].any() {
		t.Fatal("nothing listens for sacrifices: the test sees nothing")
	}
	// a's end-step trigger is on the stack; b joined it as a follower. The
	// entry carries a's registration version, as fireDelayed enqueues it.
	s.dly[0] = delayed{act: uint8(kop.OpSacrifice), ctrl: 0, obj: b, ver: s.objs[b].ver, when: dlyGrpFollow, creator: a}
	s.ndly = 1
	s.npend = 0
	s.resolveSacUntracked(sc.k, &stackEntry{ctrl: 0, obj: a, trigObj: NoObj, srcVer: s.objs[a].ver})
	if s.objs[a].zone == kop.ZoneBattlefield || s.objs[b].zone == kop.ZoneBattlefield {
		t.Fatalf("both tokens must be sacrificed: zones %v %v", s.objs[a].zone, s.objs[b].zone)
	}
	if f.pendingBy(a) != 1 || f.pendingBy(b) != 1 || s.npend != 2 {
		t.Fatalf("pending Sacrificed triggers: a %d, b %d, total %d; want one each", f.pendingBy(a), f.pendingBy(b), s.npend)
	}
}

// TestLegendRuleAndSagaSacrificeAreOneBatch: a legend-rule ask and a Saga due
// its sacrifice are in the same state-based check (CR 704.3; gorge
// parkLegendChoice parks the pass's acts with the ask), so the legend the rule
// removes still sees the Saga go (CR 603.10a): "whenever an enchantment you
// control is put into a graveyard" triggers for both, not only the one kept.
func TestLegendRuleAndSagaSacrificeAreOneBatch(t *testing.T) {
	f := newSimFix(t)
	sc := f.sc
	s, k := sc.s, sc.k
	keep, gone := sc.perm(f.legend, 0), sc.perm(f.legend, 0)
	saga := sc.perm(f.saga, 0)
	s.perms[sc.slot(saga)].setCtrN(s, int(kop.CtrLore), 3)
	if !s.sagaDue(k, sc.slot(saga)) {
		t.Fatal("the Saga is not due its sacrifice: the test sees nothing")
	}
	if !s.sba(k) || s.ctl != ctlLegend {
		t.Fatalf("no legend ask posed: ctl %v", s.ctl)
	}
	s.npend = 0
	s.applyLegend(k, MakeAction(AkChoose, TCode(keep), 0, 0))
	if s.objs[saga].zone == kop.ZoneBattlefield || s.objs[gone].zone == kop.ZoneBattlefield {
		t.Fatalf("the legend and the Saga must both leave: zones %v %v", s.objs[gone].zone, s.objs[saga].zone)
	}
	if f.pendingBy(keep) != 1 || f.pendingBy(gone) != 1 {
		t.Fatalf("pending triggers: kept %d, ruled out %d; want one each", f.pendingBy(keep), f.pendingBy(gone))
	}
}

// TestCopyEachTargetCopiesEnterTogether: "for each of them, create a token that's
// a copy" (Dominion Infiltration) makes the copies of its several targets in
// one zone batch (gorge effCopyPermanent, CR 603.6a), so a copy's "whenever
// another creature you control enters" also fires for the copy made after it.
func TestCopyEachTargetCopiesEnterTogether(t *testing.T) {
	f := newSimFix(t)
	sc := f.sc
	s, k := sc.s, sc.k
	src := sc.perm(f.etb, 1)
	s.res = resCtx{e: stackEntry{kind: seSpell, ctrl: 0, obj: src, card: f.etb, trigObj: NoObj}, active: true, lastTok: NoObj, eachObj: NoObj}
	if !s.anyETB(k) {
		t.Fatal("nothing listens for entries: the test sees nothing")
	}
	in := kop.Instr{Op: kop.OpCopyAsTokenEx, Obj: kop.RefSelf, A: kop.Const(1)}
	s.npend = 0
	// the each-target loop's frame (execEachTarget): one frame, one copy per target
	hr := s.holdEntries()
	s.g.inLoop = true
	s.copyTokens(k, &in, f.etb, 1)
	if s.npend != 0 {
		t.Fatalf("the first copy's entry check fired at once (%d pending)", s.npend)
	}
	s.copyTokens(k, &in, f.etb, 1)
	s.g.inLoop = false
	s.holdClose(k, hr)
	if s.npend != 2 {
		t.Fatalf("pending triggers %d, want 2 (each copy sees the other enter)", s.npend)
	}
}

// TestSneakEntryTapFiresNoTaps: a sneaked creature enters tapped and attacking;
// that tapped state is its entry state, not a tapping (CR 603.2e, gorge
// emitTap's entering flag), so a "whenever another creature you control
// becomes tapped" watcher does not trigger.
func TestSneakEntryTapFiresNoTaps(t *testing.T) {
	f := newSimFix(t)
	sc := f.sc
	s, k := sc.s, sc.k
	watcher := sc.perm(f.taps, 0)
	e := sc.perm(f.bear, 0)
	sl := sc.slot(e)
	// precondition: tapping the creature the ordinary way does trigger the watcher
	s.perms[sl].flags |= pfTapped
	s.npend = 0
	s.fireTaps(k, sl)
	if f.pendingBy(watcher) != 1 {
		t.Fatalf("an ordinary tap queued %d watcher triggers, want 1: the test sees nothing", f.pendingBy(watcher))
	}
	s.npend = 0
	s.perms[sl].flags &^= pfTapped
	s.sneakEnter(k, sl, &stackEntry{ctrl: 0, trigObj: NoObj})
	if s.perms[sl].flags&pfTapped == 0 {
		t.Fatal("the sneaked creature did not enter tapped")
	}
	if s.npend != 0 {
		t.Fatalf("entering tapped queued %d triggers, want none", s.npend)
	}
}
