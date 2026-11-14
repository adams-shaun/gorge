package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestEnchantParkTrimsEnteredCards: a found-card move onto the battlefield that
// enters a non-Aura card and then parks at an Aura's raGenEnchant ask leaves
// only the cards still to move in the look, as every sibling park does. Before,
// the answered re-run began at the card that had already entered and left it
// from a slot already detached ("kernel: place into unsupported zone").
func TestEnchantParkTrimsEnteredCards(t *testing.T) {
	deck := []cardset.Entry{{Name: "Witness Protection", Count: 1}, {Name: "Banner of Kinship", Count: 1}, {Name: "Eager Trufflesnout", Count: 2}, {Name: "Plains", Count: 16}}
	set := cardsettest.Set(t, "fdn")
	for _, e := range deck {
		if i, ok := set.Lookup(e.Name); !ok || !set.Supported()[i] {
			t.Skipf("%s not in the FDN table", e.Name)
		}
	}
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	src := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Mover", Types: kop.TSorcery, Abilities: []kop.Ability{
		{Kind: kop.AbSpell, Prog: []kop.Instr{{Op: kop.OpSearchEx}}}}})

	for _, tail := range []bool{false, true} {
		sc := newScenTable(t, tbl, Config{NoElide: true})
		s := sc.s
		a, b := sc.hand(id["Plains"], 0), sc.hand(id["Witness Protection"], 0)
		look := []Obj{a, b}
		var c Obj = NoObj
		if tail {
			c = sc.hand(id["Plains"], 0)
			look = append(look, c)
		}
		h1, h2 := sc.perm(id["Eager Trufflesnout"], 0), sc.perm(id["Eager Trufflesnout"], 1)
		s.res = resCtx{e: stackEntry{kind: seSpell, obj: NoObj, card: src, ctrl: 0, trigObj: NoObj},
			active: true, lastTok: NoObj, eachObj: NoObj}
		s.res.e.tgt = noTargets
		s.ctl = ctlResolve
		s.g.pickP, s.g.pickZ, s.g.pickF = 0, uint8(kop.ZoneBattlefield), 0
		copy(s.resArm().look[:], look)
		s.g.nlook = uint8(len(look))
		if !s.handAuraArmed(sc.k) || s.objs[b].zone != kop.ZoneHand || s.objs[a].zone != kop.ZoneHand {
			t.Fatalf("tail=%v: want the OpSearchEx armed and the cards in the hand", tail)
		}

		s.moveFoundAns(sc.k, NoSlot)
		if s.res.ask != raGenEnchant {
			t.Fatalf("tail=%v: ask %d, want the enchant ask", tail, s.res.ask)
		}
		if !sc.onBF(a) || s.objs[b].zone != kop.ZoneHand {
			t.Fatalf("tail=%v: after the park A on bf %v, B zone %d; want A entered and B waiting in the hand", tail, sc.onBF(a), s.objs[b].zone)
		}
		if tail && s.objs[c].zone != kop.ZoneHand {
			t.Fatalf("after the park the card behind the Aura is in zone %d, want the hand", s.objs[c].zone)
		}
		if want := uint8(len(look) - 1); s.g.nlook != want || s.resArm().look[0] != b {
			t.Fatalf("tail=%v: nlook %d look[0] %d, want %d starting at the parked Aura %d", tail, s.g.nlook, s.resArm().look[0], want, b)
		}
		aSlot, host := sc.slot(a), sc.slot(h2)
		if sc.slot(h1) == host {
			t.Fatal("the two hosts share a slot")
		}

		s.res.ask = raNone
		s.moveFoundAns(sc.k, host)
		if !sc.onBF(b) || s.perms[sc.slot(b)].attached != host {
			t.Fatalf("tail=%v: Aura on bf %v attached %d, want on the battlefield attached to %d", tail, sc.onBF(b), s.perms[sc.slot(b)].attached, host)
		}
		if !sc.onBF(a) || sc.slot(a) != aSlot {
			t.Fatalf("tail=%v: A on bf %v slot %d, want still in slot %d", tail, sc.onBF(a), sc.slot(a), aSlot)
		}
		if att := s.perms[sc.slot(a)].attached; att != NoSlot {
			t.Fatalf("tail=%v: A attached to %d, want unattached", tail, att)
		}
		if tail && !sc.onBF(c) {
			t.Fatalf("the card behind the Aura is in zone %d, want the battlefield", s.objs[c].zone)
		}
		if tail {
			if att := s.perms[sc.slot(c)].attached; att != NoSlot {
				t.Fatalf("tail=%v: the trailing land attached to %d, want unattached", tail, att)
			}
		}
		if s.g.nlook != 0 {
			t.Fatalf("tail=%v: nlook %d after the answer, want 0", tail, s.g.nlook)
		}
	}

	// A second parked Aura in the tail: the answered re-run moves the first
	// Aura and then parks again on the next look card. The answered arm must
	// leave that second ask posed (not resume past it) and run the search's
	// tail only after the last answer.
	sc := newScenTable(t, tbl, Config{NoElide: true})
	s := sc.s
	a := sc.hand(id["Plains"], 0)
	b1 := sc.hand(id["Witness Protection"], 0)
	b2 := sc.hand(id["Witness Protection"], 0)
	h1, h2 := sc.perm(id["Eager Trufflesnout"], 0), sc.perm(id["Eager Trufflesnout"], 1)
	sp := sc.obj(src, 0)
	s.objs[sp].zone = kop.ZoneStack
	s.sp = 1
	s.res = resCtx{e: stackEntry{kind: seSpell, obj: sp, card: src, ctrl: 0, trigObj: NoObj},
		active: true, lastTok: NoObj, eachObj: NoObj}
	s.res.e.tgt = noTargets
	s.ctl = ctlResolve
	s.g.pickP, s.g.pickZ, s.g.pickF = 0, uint8(kop.ZoneBattlefield), 0
	copy(s.resArm().look[:], []Obj{a, b1, b2})
	s.g.nlook = 3
	if !s.handAuraArmed(sc.k) || s.objs[b1].zone != kop.ZoneHand || s.objs[b2].zone != kop.ZoneHand {
		t.Fatal("two Auras: want the OpSearchEx armed and both Auras in the hand")
	}
	if h1 == h2 || sc.slot(h1) == sc.slot(h2) {
		t.Fatal("the two hosts are not distinct")
	}

	s.moveFoundAns(sc.k, NoSlot)
	if s.res.ask != raGenEnchant || s.g.nlook != 2 || s.resArm().look[0] != b1 {
		t.Fatalf("two Auras: after the park ask %d nlook %d look[0] %d, want the ask with look starting at the first Aura %d",
			s.res.ask, s.g.nlook, s.resArm().look[0], b1)
	}
	draws := s.rng.Draws
	s.applyResolveGen(sc.k, MakeAction(AkChoose, TCode(h2), 0, 0))
	if s.res.ask != raGenEnchant {
		t.Fatalf("two Auras: ask %d after the first answer, want the second Aura's ask (not dropped)", s.res.ask)
	}
	if !sc.onBF(b1) || s.perms[sc.slot(b1)].attached != sc.slot(h2) {
		t.Fatalf("two Auras: first Aura on bf %v attached %d, want attached to %d",
			sc.onBF(b1), s.perms[sc.slot(b1)].attached, sc.slot(h2))
	}
	if s.g.nlook != 1 || s.resArm().look[0] != b2 || s.objs[b2].zone != kop.ZoneHand {
		t.Fatalf("two Auras: nlook %d look[0] %d second Aura zone %d, want 1 starting at the parked second Aura %d in the hand",
			s.g.nlook, s.resArm().look[0], s.objs[b2].zone, b2)
	}
	if !sc.onBF(a) {
		t.Fatal("two Auras: the already-entered Plains left the battlefield")
	}
	if s.rng.Draws != draws {
		t.Fatalf("two Auras: the search tail ran before the second ask was answered (draws %d -> %d)", draws, s.rng.Draws)
	}

	s.applyResolveGen(sc.k, MakeAction(AkChoose, TCode(h1), 0, 0))
	if !sc.onBF(b2) || s.perms[sc.slot(b2)].attached != sc.slot(h1) {
		t.Fatalf("two Auras: second Aura on bf %v attached %d, want attached to %d",
			sc.onBF(b2), s.perms[sc.slot(b2)].attached, sc.slot(h1))
	}
	if s.g.nlook != 0 || s.res.ask != raNone || s.res.left != 0 || s.res.pc == 0 {
		t.Fatalf("two Auras: after the second answer nlook %d ask %d left %d pc %d, want the look empty and the search resumed",
			s.g.nlook, s.res.ask, s.res.left, s.res.pc)
	}
	if s.rng.Draws == draws {
		t.Fatal("two Auras: the search tail did not run after the second answer")
	}

	// A queued as-enters ask inside the answered re-run: after the enchant
	// answer, the re-run enters Banner of Kinship (a type election) with a
	// trailing land still in the look. moveFoundAns parks that election by
	// setting res.left = tailSearchEnter; the answer arm must leave the marker
	// alone instead of overwriting it with tailSearch, or the trailing land
	// never moves.
	{
		sc := newScenTable(t, tbl, Config{NoElide: true})
		s := sc.s
		a := sc.hand(id["Plains"], 0)
		b := sc.hand(id["Witness Protection"], 0)
		c := sc.hand(id["Banner of Kinship"], 0)
		d := sc.hand(id["Plains"], 0)
		h1 := sc.perm(id["Eager Trufflesnout"], 0)
		h2 := sc.perm(id["Eager Trufflesnout"], 1)
		sp := sc.obj(src, 0)
		s.objs[sp].zone = kop.ZoneStack
		s.sp = 1
		s.res = resCtx{e: stackEntry{kind: seSpell, obj: sp, card: src, ctrl: 0, trigObj: NoObj},
			active: true, lastTok: NoObj, eachObj: NoObj}
		s.res.e.tgt = noTargets
		s.ctl = ctlResolve
		s.g.pickP, s.g.pickZ, s.g.pickF = 0, uint8(kop.ZoneBattlefield), 0
		copy(s.resArm().look[:], []Obj{a, b, c, d})
		s.g.nlook = 4
		if !s.handAuraArmed(sc.k) || s.objs[b].zone != kop.ZoneHand {
			t.Fatal("as-enters tail: want the OpSearchEx armed and the Aura in the hand")
		}
		for _, o := range []Obj{a, c, d} {
			if s.objs[o].zone != kop.ZoneHand {
				t.Fatalf("as-enters tail: card %d is in zone %d, want the hand", o, s.objs[o].zone)
			}
		}
		if h1 == h2 || sc.slot(h1) == sc.slot(h2) || s.objs[c].card != id["Banner of Kinship"] {
			t.Fatal("as-enters tail: the hosts or the queued-ask card are not distinct")
		}

		s.moveFoundAns(sc.k, NoSlot)
		if s.res.ask != raGenEnchant || s.g.nlook != 3 || s.resArm().look[0] != b {
			t.Fatalf("as-enters tail: after the park ask %d nlook %d look[0] %d, want the ask with look starting at the Aura %d",
				s.res.ask, s.g.nlook, s.resArm().look[0], b)
		}
		aSlot := sc.slot(a)
		s.applyResolveGen(sc.k, MakeAction(AkChoose, TCode(h2), 0, 0))
		if s.res.ask != raGenEnterAsk || s.res.left != tailSearchEnter {
			t.Fatalf("as-enters tail: ask %d left %d after the enchant answer, want the queued election with tailSearchEnter kept",
				s.res.ask, s.res.left)
		}
		if s.g.nlook != 1 || s.resArm().look[0] != d || s.objs[d].zone != kop.ZoneHand {
			t.Fatalf("as-enters tail: nlook %d look[0] %d trailing land zone %d, want 1 starting at %d in the hand",
				s.g.nlook, s.resArm().look[0], s.objs[d].zone, d)
		}
		if !sc.onBF(b) || s.perms[sc.slot(b)].attached != sc.slot(h2) || !sc.onBF(c) {
			t.Fatalf("as-enters tail: Aura on bf %v attached %d, Banner on bf %v; want attached to %d and the Banner entered",
				sc.onBF(b), s.perms[sc.slot(b)].attached, sc.onBF(c), sc.slot(h2))
		}
		if !sc.onBF(a) || sc.slot(a) != aSlot {
			t.Fatalf("as-enters tail: the leading Plains on bf %v slot %d, want still in slot %d", sc.onBF(a), sc.slot(a), aSlot)
		}

		s.applyResolveGen(sc.k, MakeAction(AkChoose, 0, 0, 0))
		if !sc.onBF(d) || s.perms[sc.slot(d)].attached != NoSlot {
			t.Fatalf("as-enters tail: trailing land on bf %v attached %d, want entered unattached",
				sc.onBF(d), s.perms[sc.slot(d)].attached)
		}
		if s.g.nlook != 0 || s.res.left != 0 || s.res.ask != raNone {
			t.Fatalf("as-enters tail: after the election nlook %d left %d ask %d, want the look empty and the tail run",
				s.g.nlook, s.res.left, s.res.ask)
		}
		if !sc.onBF(b) || !sc.onBF(c) || !sc.onBF(a) {
			t.Fatal("as-enters tail: an already-entered card left the battlefield")
		}
	}
}

// enchantParkFix: a two-card library search onto the battlefield whose first
// found card is a land and whose second is an Aura with two legal hosts.
// kgen never lowers such a search (it refuses a multi-card battlefield search
// whose filter may admit an Aura), but a search world re-deals the hidden
// objects' cards (State.DealHidden): a picked library land can be an Aura by
// the time the pick resolves, so the kernel must survive the pose.
type enchantParkFix struct {
	sc          *scen
	land, aura  Obj
	land2       Obj
	bear1, bear Obj
}

func newEnchantParkFix(t *testing.T) *enchantParkFix {
	t.Helper()
	deck := []cardset.Entry{{Name: "Pacifism", Count: 4}, {Name: "Plains", Count: 12}, {Name: "Grizzly Bears", Count: 4}}
	set := cardsettest.Set(t, "handaura")
	tbl, _, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	var pac, pl, bears CardID = 255, 255, 255
	for i := range tbl.Cards {
		switch tbl.Cards[i].Name {
		case "Pacifism":
			pac = CardID(i)
		case "Plains":
			pl = CardID(i)
		case "Grizzly Bears":
			bears = CardID(i)
		}
	}
	tbl.Filters = append(tbl.Filters, kop.Filter{Types: kop.TLand | kop.TEnchantment, Zone: kop.ZoneLibrary})
	any := kop.FilterID(len(tbl.Filters) - 1)
	spell := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Fetcher", Types: kop.TSorcery, Abilities: []kop.Ability{
		{Kind: kop.AbSpell, Prog: []kop.Instr{{Op: kop.OpSearchEx, Obj: kop.RefController, A: kop.Const(3), F: any, C: int16(kop.ZoneBattlefield)},
			{Op: kop.OpSearchEx, Obj: kop.RefController, A: kop.Const(1), F: any, C: int16(kop.ZoneHand)}}}}})
	sc := newScenTableLib(t, tbl, pl, Config{NoElide: true})
	if sc.k.gen == nil {
		t.Fatal("the fixture table has no generated state")
	}
	s := sc.s
	m := &enchantParkFix{sc: sc}
	m.land = sc.obj(pl, 0)
	m.aura = sc.obj(pac, 0)
	m.land2 = sc.obj(pl, 0)
	for _, o := range []Obj{m.land, m.aura, m.land2} {
		s.objs[o].zone = kop.ZoneLibrary
		s.pl[0].libTop(o)
	}
	m.bear1 = sc.perm(bears, 1)
	m.bear = sc.perm(bears, 0)
	s.res = resCtx{e: stackEntry{kind: seSpell, obj: NoObj, card: spell, ctrl: 0, trigObj: NoObj}, active: true, lastTok: NoObj, eachObj: NoObj}
	s.res.e.tgt = noTargets
	s.ctl = ctlResolve
	return m
}

// TestSearchEnchantParkKeepsEnteredCardOutOfTheRerun: the land enters, the Aura
// parks at the enchant ask, and the answer enters the Aura attached to its host
// without moving the land again (the look list kept the entered land and
// moveFoundAns left it from the battlefield into "place into unsupported zone").
func TestSearchEnchantParkKeepsEnteredCardOutOfTheRerun(t *testing.T) {
	m := newEnchantParkFix(t)
	s, k := m.sc.s, m.sc.k
	in := &s.prog(k)[0]
	if s.startSearch(k, in) || s.res.ask != raGenSearch {
		t.Fatalf("ask %d, want the search pick", s.res.ask)
	}
	s.applyResolveGen(k, MakeAction(AkChoose, TCode(m.land), 0, 0))
	s.applyResolveGen(k, MakeAction(AkChoose, TCode(m.aura), 0, 0))
	s.applyResolveGen(k, MakeAction(AkChoose, TCode(m.land2), 0, 0))
	// the precondition the assertion below depends on: the land entered, the
	// Aura waits in the library for the host ask
	if s.res.ask != raGenEnchant {
		t.Fatalf("ask %d, want the Aura's enchant ask %d", s.res.ask, raGenEnchant)
	}
	if s.objs[m.land].zone != kop.ZoneBattlefield || s.objs[m.aura].zone != kop.ZoneLibrary || s.objs[m.land2].zone != kop.ZoneLibrary {
		t.Fatalf("zones %d %d %d, want the first land entered and the Aura and the last land still in the library",
			s.objs[m.land].zone, s.objs[m.aura].zone, s.objs[m.land2].zone)
	}
	s.applyResolveGen(k, MakeAction(AkChoose, TCode(m.bear), 0, 0))
	if s.res.pc != 1 || s.res.ask != raGenSearch {
		t.Fatalf("pc %d ask %d, want the program resumed at the next search's pick", s.res.pc, s.res.ask)
	}
	for _, o := range []Obj{m.land, m.aura, m.land2} {
		if s.objs[o].zone != kop.ZoneBattlefield {
			t.Fatalf("object %d is in zone %d, want the battlefield", o, s.objs[o].zone)
		}
	}
	if got, want := s.perms[s.objs[m.aura].slot].attached, s.objs[m.bear].slot; got != want {
		t.Fatalf("the Aura is attached to slot %d, want its answered host %d", got, want)
	}
	if got := s.perms[s.objs[m.land2].slot].attached; got == s.objs[m.bear].slot {
		t.Fatalf("the land found after the Aura is attached to the Aura's host (slot %d)", got)
	}
}
