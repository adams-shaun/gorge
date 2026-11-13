package dirac

import (
	"fmt"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// effectLimitTable is the vanilla table plus a card whose program grants a free
// play permission with a MayPlayLimit cap of 1 (a Flameskull-style Effect
// grant), so prepareGen arms the limit walk.
func effectLimitTable() *kop.Table {
	t := v.NewTable()
	t.Exprs = []kop.Expr{}
	t.Cards = append(t.Cards, kop.CardDef{
		Name:  "Test Grant",
		Types: kop.TSorcery,
		Abilities: []kop.Ability{{Kind: kop.AbSpell, Prog: []kop.Instr{
			{Op: kop.OpMayPlay, Obj: kop.RefRemembered, A: kop.Const(kop.DurYourNextTurn),
				B: kop.Const(kop.MayPlayFree | 1<<kop.MayPlayLimitShift), C: int16(kop.ZoneExile)},
		}}},
	})
	return t
}

// TestEffectLimitPerGrant: an Effect-delivered MayPlayLimit$ grant counts
// per grant and per turn, not per player. Two grants of
// limit 1 each hold two exiled cards: casting one card leaves the other
// grant's cards castable, a second cast through the spent grant is refused,
// and the spent grant is open again once a turn has passed.
func TestEffectLimitPerGrant(t *testing.T) {
	sc := newScenTable(t, effectLimitTable(), Config{NoElide: true})
	s, k := sc.s, sc.k
	if !k.gen.anyMayLimit {
		t.Fatal("prepareGen did not arm the limit walk")
	}
	var o [4]Obj
	for i := range o {
		o[i] = sc.obj(v.ThinkTwice, 0)
		s.objs[o[i]].zone = kop.ZoneExile
	}
	grant := func(a, b Obj) {
		s.g.rem[0], s.g.rem[1], s.g.nrem = a, b, 2
		in := kop.Instr{Op: kop.OpMayPlay, Obj: kop.RefRemembered, A: kop.Const(kop.DurYourNextTurn),
			B: kop.Const(kop.MayPlayFree | 1<<kop.MayPlayLimitShift), C: int16(kop.ZoneExile)}
		if ok, done := s.execGen(k, &in); !ok || !done {
			t.Fatalf("OpMayPlay not handled (%v, %v)", ok, done)
		}
	}
	grant(o[0], o[1])
	grant(o[2], o[3])
	if s.g.nmay != 4 {
		t.Fatalf("%d records, want 4", s.g.nmay)
	}
	offered := func() (got [4]bool) {
		var d Decision
		var av avail
		s.priorityGen(k, &d, 0, true, &av, false)
		for i := range o {
			got[i] = hasMayCast(&d, o[i])
		}
		return
	}
	cast := func(x Obj) {
		*s.bld() = build{e: stackEntry{kind: seSpell, obj: x, card: v.ThinkTwice, ctrl: 0}}
		s.castFlagsGen(k, x, castMayPlay|castFree)
		s.objs[x].zone = kop.ZoneStack
		s.objs[x].ver++
	}
	if got := offered(); got != [4]bool{true, true, true, true} {
		t.Fatalf("fresh grants offer %v, want all four", got)
	}
	cast(o[0])
	if got := offered(); got != [4]bool{false, false, true, true} {
		t.Fatalf("after one cast through grant 1 the offers are %v, want only grant 2's cards", got)
	}
	cast(o[2])
	if got := offered(); got != [4]bool{} {
		t.Fatalf("both grants spent, offers %v, want none", got)
	}
	// a record that outlives the turn (until your next turn) is open again
	s.cleanupGen(k)
	if got := offered(); got != [4]bool{false, true, false, true} {
		t.Fatalf("next turn the offers are %v, want the unplayed card of each grant", got)
	}
	cast(o[1])
	if got := offered(); got != [4]bool{false, false, false, true} {
		t.Fatalf("after the new turn's cast through grant 1 the offers are %v, want grant 2's card only", got)
	}
}

// effectLimitProgScen is a scenario whose stack holds a test sorcery running prog,
// resolving for player 0; both seats hold two Plains.
func effectLimitProgScen(t *testing.T, prog ...kop.Instr) *scen {
	t.Helper()
	deck := []cardset.Entry{{Name: "Plains", Count: 20}}
	set := cardsettest.Set(t, "fdn")
	tbl, _, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	src := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Kf28", Types: kop.TSorcery,
		Abilities: []kop.Ability{{Kind: kop.AbSpell, Prog: prog}}})
	sc := newScenTable(t, tbl, Config{NoElide: true})
	o := sc.obj(src, 0)
	sc.s.objs[o].zone = kop.ZoneStack
	sc.s.push(stackEntry{kind: seSpell, obj: o, ctrl: 0, card: src, trigObj: NoObj})
	sc.s.res.e = stackEntry{kind: seSpell, obj: o, ctrl: 0, card: src, trigObj: NoObj}
	sc.s.res.pc = 0
	return sc
}

// TestDiscardZero: a NumCards$ that resolves to 0 discards nothing and
// poses no ask (gorge discardBounds keeps zero at zero), in the unfiltered,
// ChooseOpp, each-player and Optional$ arms; AnyNumber$ still asks (its
// bounds are 0..eligible and NumCards$ is unread).
func TestDiscardZero(t *testing.T) {
	cases := []struct {
		name string
		in   kop.Instr
		asks bool
	}{
		{"unfiltered", kop.Instr{Op: kop.OpDiscard, Obj: kop.RefController, A: kop.Const(0), C: kop.ChooseSelf}, false},
		{"choose-opp", kop.Instr{Op: kop.OpDiscard, Obj: kop.RefOpponent, A: kop.Const(0), C: kop.ChooseOpp}, false},
		{"each-player", kop.Instr{Op: kop.OpDiscard, Obj: kop.RefEachPlayer, A: kop.Const(0), C: kop.ChooseSelf}, false},
		{"optional", kop.Instr{Op: kop.OpDiscard, Obj: kop.RefController, A: kop.Const(0), C: kop.ChooseSelf | kop.OptAsk}, false},
		{"any-number", kop.Instr{Op: kop.OpDiscard, Obj: kop.RefController, A: kop.Const(0), C: kop.ChooseSelf | kop.OptAsk | kop.OptAny}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			sc := effectLimitProgScen(t, c.in)
			for p := uint8(0); p < 2; p++ {
				sc.hand(v.Plains, p)
				sc.hand(v.Plains, p)
			}
			sc.s.runProg(sc.k)
			asked := sc.s.res.ask != raNone
			if asked != c.asks {
				t.Fatalf("ask pending %v, want %v", asked, c.asks)
			}
			if sc.s.pl[0].handN != 2 || sc.s.pl[1].handN != 2 {
				t.Fatalf("hands %d/%d, want 2/2 (nothing discarded)", sc.s.pl[0].handN, sc.s.pl[1].handN)
			}
		})
	}
}

// TestFetchShuffleOwed: a Defined$ library fetch is not a search, so it
// shuffles only when the script owes one (gorge zone_search.go
// implicitShuffle): a Traveling Botanist fetch (no flag) draws no shuffle
// RNG, a Guidelight Pathmaker fetch (BncShuffleOwed / RetShuffleOwed) does.
func TestFetchShuffleOwed(t *testing.T) {
	for _, tc := range []struct {
		name  string
		in    kop.Instr
		owed  bool
		check func(*State, Obj) bool
	}{
		{"bounce", kop.Instr{Op: kop.OpBounce, Obj: kop.RefRemembered, C: kop.BncFromLibrary}, false,
			func(s *State, o Obj) bool { return s.objs[o].zone == kop.ZoneHand }},
		{"bounce-owed", kop.Instr{Op: kop.OpBounce, Obj: kop.RefRemembered, C: kop.BncFromLibrary | kop.BncShuffleOwed}, true,
			func(s *State, o Obj) bool { return s.objs[o].zone == kop.ZoneHand }},
		{"return", kop.Instr{Op: kop.OpReturnEx, Obj: kop.RefRemembered, B: kop.Const(kop.RetFromLibrary), C: int16(kop.ZoneBattlefield)}, false,
			func(s *State, o Obj) bool { return s.objs[o].zone == kop.ZoneBattlefield }},
		{"return-owed", kop.Instr{Op: kop.OpReturnEx, Obj: kop.RefRemembered, B: kop.Const(kop.RetFromLibrary | kop.RetShuffleOwed), C: int16(kop.ZoneBattlefield)}, true,
			func(s *State, o Obj) bool { return s.objs[o].zone == kop.ZoneBattlefield }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sc, id := newRemParamsScen(t)
			s := sc.s
			s.res.e.obj = sc.obj(id["Plains"], 0)
			pl := &s.pl[0]
			a := pl.lib[pl.libLo]
			s.g.rem[0], s.g.nrem = a, 1
			before := s.rng.Draws
			in := tc.in
			s.execGen(sc.k, &in)
			if !tc.check(s, a) {
				t.Fatalf("the fetched card is in zone %d", s.objs[a].zone)
			}
			if drew := s.rng.Draws != before; drew != tc.owed {
				t.Fatalf("shuffle RNG drawn %v, want %v", drew, tc.owed)
			}
		})
	}
}

// TestRememberDamagedDealt: RememberDamaged$ remembers an object only when
// damage above 0 was actually dealt to it, and the exile-if-dies of
// ReplaceDyingDefined$ Remembered (DmgExileIfDealt) applies to the same
// recipients only (gorge, CR 120.1). A creature with protection from
// everything takes none of it; Targeted (no DmgExileIfDealt) keeps marking.
func TestRememberDamagedDealt(t *testing.T) {
	tbl := v.NewTable()
	tbl.Exprs = []kop.Expr{}
	prot := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Protected", Types: kop.TCreature, Power: 2, Toughness: 2,
		Keywords: kop.KwProtectionAll})
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Damage", Types: kop.TSorcery,
		Abilities: []kop.Ability{{Kind: kop.AbSpell, Prog: []kop.Instr{
			{Op: kop.OpDealDamageEx, Obj: kop.RefTarget0, A: kop.Const(2),
				C: kop.DmgRemember | kop.DmgExileIfDiesThisTurn | kop.DmgExileIfDealt},
		}}}})
	sc := newScenTable(t, tbl, Config{NoElide: true})
	s, k := sc.s, sc.k
	pr := sc.perm(prot, 1)
	bear := sc.perm(v.GrizzlyBears, 1)
	s.res.e = stackEntry{kind: seSpell, ctrl: 0, card: prot + 1, trigObj: NoObj, obj: NoObj}
	for i := range s.res.e.tgt {
		s.res.e.tgt[i] = uint8(NoTarget)
	}
	deal := func(flags int16, x Obj) {
		s.res.e.tgt[0] = uint8(x)
		in := kop.Instr{Op: kop.OpDealDamageEx, Obj: kop.RefTarget0, A: kop.Const(2), C: flags}
		s.execDamageEx(k, &in)
	}
	all := int16(kop.DmgRemember | kop.DmgExileIfDiesThisTurn | kop.DmgExileIfDealt)
	deal(all, pr)
	if s.g.nrem != 0 || s.mSl(k, mechExileEx, s.objs[pr].slot) {
		t.Fatalf("a fully prevented recipient was remembered (%d) or marked to be exiled", s.g.nrem)
	}
	deal(all, bear)
	if s.g.nrem != 1 || s.g.rem[0] != bear || !s.mSl(k, mechExileEx, s.objs[bear].slot) {
		t.Fatalf("the damaged recipient: remembered %d, marked %v", s.g.nrem, s.mSl(k, mechExileEx, s.objs[bear].slot))
	}
	// Targeted: every target is marked, dealt or not
	s.mSlZero(k, mechExileEx)
	s.g.nrem = 0
	deal(int16(kop.DmgExileIfDiesThisTurn), pr)
	if !s.mSl(k, mechExileEx, s.objs[pr].slot) {
		t.Fatal("ReplaceDyingDefined$ Targeted no longer marks a prevented target")
	}
}

// TestTwoExileMarksStall: each turn-scoped "exile instead of dying" effect
// is its own CR 616.1 replacement in gorge, which asks the order when two apply.
// A state-based death is asked (exileOrderAsk); dieZone, every other death, has
// no such ask, so two marks on the creature stall there (StallReplLeave),
// whichever flavour (Targeted or DmgExileIfDealt) placed them. One mark, or
// marks on different creatures, do not, and no mark stalls by being placed.
func TestTwoExileMarksStall(t *testing.T) {
	targeted := int16(kop.DmgExileIfDiesThisTurn)
	dealt := int16(kop.DmgExileIfDiesThisTurn | kop.DmgExileIfDealt)
	for _, tc := range []struct {
		name  string
		first int16
		next  int16
		same  bool
		stall bool
	}{
		{"single", targeted, 0, true, false},
		{"two-targeted", targeted, targeted, true, true},
		{"targeted-then-dealt", targeted, dealt, true, true},
		{"dealt-then-targeted", dealt, targeted, true, true},
		{"two-dealt", dealt, dealt, true, true},
		{"different-slots", targeted, targeted, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tbl := v.NewTable()
			tbl.Exprs = []kop.Expr{}
			spell := CardID(len(tbl.Cards))
			tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Damage", Types: kop.TSorcery,
				Abilities: []kop.Ability{{Kind: kop.AbSpell, Prog: []kop.Instr{
					{Op: kop.OpDealDamageEx, Obj: kop.RefTarget0, A: kop.Const(1), C: dealt},
				}}}})
			sc := newScenTable(t, tbl, Config{NoElide: true})
			s, k := sc.s, sc.k
			a := sc.perm(v.GrizzlyBears, 1)
			b := sc.perm(v.GrizzlyBears, 1)
			s.res.e = stackEntry{kind: seSpell, ctrl: 0, card: spell, trigObj: NoObj, obj: NoObj}
			for i := range s.res.e.tgt {
				s.res.e.tgt[i] = uint8(NoTarget)
			}
			deal := func(flags int16, x Obj) {
				s.res.e.tgt[0] = uint8(x)
				in := kop.Instr{Op: kop.OpDealDamageEx, Obj: kop.RefTarget0, A: kop.Const(1), C: flags}
				s.execDamageEx(k, &in)
			}
			deal(tc.first, a)
			if s.Stalled() {
				t.Fatal("a first mark stalled")
			}
			if tc.next != 0 {
				if tc.same {
					deal(tc.next, a)
				} else {
					deal(tc.next, b)
				}
			}
			if s.Stalled() {
				t.Fatal("a mark stalled")
			}
			if z := s.dieZone(k, s.objs[a].slot); z != kop.ZoneExile {
				t.Fatalf("dieZone = %d, want exile", z)
			}
			if tc.stall {
				if s.StallReason() != StallReplLeave {
					t.Fatalf("stall reason %d, want StallReplLeave", s.StallReason())
				}
				return
			}
			if s.Stalled() {
				t.Fatal("dieZone of one mark stalled")
			}
		})
	}
}

// cloneTemplateTable is the vanilla table plus a template creature whose enters
// program chooses a creature type (an Adaptive Automaton's ask) and a free
// Clone that enters as a copy of any other creature.
func cloneTemplateTable() (t *kop.Table, template, clone CardID) {
	t = v.NewTable()
	t.Exprs = []kop.Expr{}
	t.TypeVocab = []uint8{1, 2, 3}
	t.Filters = append(t.Filters, kop.Filter{Types: kop.TCreature})
	f := kop.FilterID(len(t.Filters) - 1)
	template = CardID(len(t.Cards))
	t.Cards = append(t.Cards, kop.CardDef{Name: "Test Choose Template", Types: kop.TCreature, Power: 2, Toughness: 2,
		Abilities: []kop.Ability{{Kind: kop.AbReplacement, Repl: kop.Repl{Kind: kop.ReplEntersProgram},
			Prog: []kop.Instr{{Op: kop.OpChooseType, Obj: kop.RefController}}}}})
	clone = CardID(len(t.Cards))
	t.Cards = append(t.Cards, kop.CardDef{Name: "Test Clone", Types: kop.TCreature,
		Abilities: []kop.Ability{{Kind: kop.AbReplacement, Repl: kop.Repl{Kind: kop.ReplEntersProgram},
			Prog: []kop.Instr{{Op: kop.OpBecomeCopy, Obj: kop.RefReplaced, C: 1, F: f}}}}})
	return t, template, clone
}

// TestCloneTemplateChoice: a permanent entering as a copy is asked the
// template's own as-enters choices (gorge, cast_etbchoice.go
// entryETBChoice: CR 707.2 and 614.12, after the entrant's own ordinals).
// The kernel answers the clone election on the stack, then poses the
// template's creature-type ask as the copy enters, before anyone has
// priority; the answer lands on the copy.
func TestCloneTemplateChoice(t *testing.T) {
	tbl, template, clone := cloneTemplateTable()
	sc := newScenTable(t, tbl, Config{NoElide: true})
	s := sc.s
	tpl := sc.perm(template, 0)
	if d := s.Step(); d.Kind == DkChoose && d.Sub == ChGenType { // the template's own as-enters ask
		s.Apply(d.Options[0])
	}
	cl := sc.hand(clone, 0)
	sc.castCard(t, cl)
	var seq []string
	for i := 0; i < 30; i++ {
		d := s.Step()
		if d.Kind == DkChoose {
			seq = append(seq, fmt.Sprintf("choose/%d", d.Sub))
			a := d.Options[1] // not the template's own answer, so the copy's differs
			if d.Sub == ChGenClone {
				a = MakeAction(AkChoose, TCode(tpl), 0, 0)
			}
			s.Apply(a)
			continue
		}
		if s.sp == 0 {
			break // the Clone resolved: nobody has been given priority yet
		}
		seq = append(seq, fmt.Sprintf("kind/%d", d.Kind))
		for j := 0; j < int(d.N); j++ {
			if d.Options[j].Kind() == AkPass {
				s.Apply(d.Options[j])
				break
			}
		}
	}
	want := fmt.Sprintf("[kind/1 kind/1 choose/%d choose/%d]", ChGenClone, ChGenType)
	if got := fmt.Sprint(seq); got != want {
		t.Fatalf("decision sequence %s, want %s: the clone election, then the template's type ask, before priority", got, want)
	}
	if got, tpls := s.perms[sc.slot(cl)].chosen, s.perms[sc.slot(tpl)].chosen; got == tpls {
		t.Fatalf("the copy's chosen type %d is the template's own %d: the copy's ask did not land on it", got, tpls)
	}
}

// TestSpendGrantSkipsUncapped: spendGrant raises the used count of the
// limited records of one grant only. An uncapped record beside them (its
// until byte is compared exactly by Warp, Plot and Prepared) keeps its
// bytes, even when the grant lost its head mark.
func TestSpendGrantSkipsUncapped(t *testing.T) {
	lim := uint8(1 << kop.MayPlayLimitShift)
	if lim&kop.MayPlayLimitMask == 0 {
		t.Fatal("limit 1 is not inside MayPlayLimitMask")
	}
	var s State
	s.g.may[0] = mayPlay{until: kop.DurEOT, kind: 0}
	s.g.may[1] = mayPlay{until: kop.DurEOT, kind: lim} // orphan: no head mark
	s.g.may[2] = mayPlay{until: kop.DurEOT, kind: 0}
	s.g.may[3] = mayPlay{until: kop.DurEOT | mayUntilHead, kind: lim}
	s.g.may[4] = mayPlay{until: kop.DurEOT, kind: lim}
	s.g.nmay = 5
	s.spendGrant(1)
	s.spendGrant(4)
	want := [5]uint8{kop.DurEOT, kop.DurEOT + mayUntilOne, kop.DurEOT, kop.DurEOT | mayUntilHead + mayUntilOne, kop.DurEOT + mayUntilOne}
	for i, w := range want {
		if got := s.g.may[i].until; got != w {
			t.Errorf("record %d until %#x, want %#x", i, got, w)
		}
	}
}
