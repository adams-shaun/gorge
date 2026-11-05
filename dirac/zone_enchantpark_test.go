package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

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
	s.applyResolveGen(k, MakeAction(AkChoose, uint8(m.land), 0, 0))
	s.applyResolveGen(k, MakeAction(AkChoose, uint8(m.aura), 0, 0))
	s.applyResolveGen(k, MakeAction(AkChoose, uint8(m.land2), 0, 0))
	// the precondition the assertion below depends on: the land entered, the
	// Aura waits in the library for the host ask
	if s.res.ask != raGenEnchant {
		t.Fatalf("ask %d, want the Aura's enchant ask %d", s.res.ask, raGenEnchant)
	}
	if s.objs[m.land].zone != kop.ZoneBattlefield || s.objs[m.aura].zone != kop.ZoneLibrary || s.objs[m.land2].zone != kop.ZoneLibrary {
		t.Fatalf("zones %d %d %d, want the first land entered and the Aura and the last land still in the library",
			s.objs[m.land].zone, s.objs[m.aura].zone, s.objs[m.land2].zone)
	}
	s.applyResolveGen(k, MakeAction(AkChoose, uint8(m.bear), 0, 0))
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
