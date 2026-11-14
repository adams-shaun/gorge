package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestGravePickAuraWaitsForItsHost: CR 303.4f/g reach every non-cast Aura
// entry, not just the hand move. A hidden pick from a graveyard onto the
// battlefield that picks an Aura must park before the move (the card waits
// in the graveyard at the raGenEnchant ask) so the resolution -- and the
// spell on the stack -- stays suspended until its controller answers what it
// enchants. It used to enter unattached, and the CR 704.5m check swept it.
func TestGravePickAuraWaitsForItsHost(t *testing.T) {
	deck := []cardset.Entry{{Name: "Witness Protection", Count: 1}, {Name: "Eager Trufflesnout", Count: 2}, {Name: "Plains", Count: 17}}
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
	tbl.Filters = append(tbl.Filters, kop.Filter{Types: kop.TEnchantment})
	f := kop.FilterID(len(tbl.Filters) - 1)
	// The Graveyard,Exile hidden pick (graveexile): Filter.Zone is ZoneNone
	// and SrGraveExile rides C>>3, as kgen lowers Fetch Quest's DBChangeZone.
	pick := kop.Instr{Op: kop.OpChooseMove, Obj: kop.RefController, A: kop.Const(1), F: f,
		C: int16(kop.ZoneBattlefield) | int16(kop.SrGraveExile)<<3}
	src := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Fetch", Types: kop.TSorcery, Abilities: []kop.Ability{
		{Kind: kop.AbSpell, Prog: []kop.Instr{pick}}}})

	sc := newScenTable(t, tbl, Config{NoElide: true})
	s := sc.s
	aura := sc.obj(id["Witness Protection"], 0)
	s.place(aura, kop.ZoneGraveyard, false)
	h1, h2 := sc.perm(id["Eager Trufflesnout"], 0), sc.perm(id["Eager Trufflesnout"], 1)
	sp := sc.obj(src, 0)
	s.objs[sp].zone = kop.ZoneStack
	s.sp = 1
	s.res = resCtx{e: stackEntry{kind: seSpell, obj: sp, card: src, ctrl: 0, trigObj: NoObj},
		active: true, lastTok: NoObj, eachObj: NoObj}
	s.res.e.tgt = noTargets
	s.ctl = ctlResolve
	if s.startSearch(sc.k, &pick) || s.res.ask != raGenSearch {
		t.Fatalf("ask %d, want the hidden graveyard pick", s.res.ask)
	}

	// Answer the pick with the Aura: two legal hosts, so the entry parks.
	s.applyResolveGen(sc.k, MakeAction(AkChoose, TCode(aura), 0, 0))
	if s.res.ask != raGenEnchant {
		t.Fatalf("ask %d after the pick, want the Aura's enchant ask", s.res.ask)
	}
	if s.objs[aura].zone != kop.ZoneGraveyard {
		t.Fatalf("Aura zone %d during the ask, want the graveyard (CR 303.4f: it chooses as it enters)", s.objs[aura].zone)
	}
	if s.objs[sp].zone != kop.ZoneStack {
		t.Fatalf("spell zone %d during the ask, want the stack: the resolution must stay suspended", s.objs[sp].zone)
	}

	// Answer the enchant ask: the Aura enters attached, then the resolution
	// finishes and the spell leaves the stack.
	s.applyResolveGen(sc.k, MakeAction(AkChoose, TCode(h1), 0, 0))
	if !sc.onBF(aura) {
		t.Fatalf("Aura zone %d after the answer, want the battlefield", s.objs[aura].zone)
	}
	if got, want := s.perms[sc.slot(aura)].attached, sc.slot(h1); got != want {
		t.Fatalf("Aura attached to %d, want the chosen host %d (%d is the other legal host)", got, want, sc.slot(h2))
	}
	if s.objs[sp].zone != kop.ZoneGraveyard {
		t.Fatalf("spell zone %d after the answer, want its rest zone: the resolution finishes with the answer", s.objs[sp].zone)
	}
}
