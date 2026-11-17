package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// TestTapXScriptStallPropagates: the tap-another search's only payment path is
// Thermopod's "Sacrifice a creature: Add {R}" (kop.SacAnyManaAb, no {T}), and
// the sacrificed creature dies under two exile-instead replacements (two
// Samurai of the Pale Curtain, themselves the only candidates), which latches
// StallReplLeave in the fork (dieZone). The search reports the stall to its
// caller instead of plain "no payment", and the offer walk (scriptOffers)
// declares it on the real state where it drops the play (capFull at the drop —
// grantCap ran before scriptOffers, legal.go, so the drop is final; the search
// itself declares nothing): the game draws as the unmodelled stall instead of
// silently losing the cast gorge offers (dr-mbc-baaa pair 103, decision 254;
// the stand-in cast is Abrade, {1}{R}: past the printed census — the Plains'
// {W} pays the {1} but no printed source makes the {R} — as Eesha is; the
// search's gap unit is Thermopod's deferred sacrifice mana).
// The matchup holds the four cards alone, so the table's prep carries no
// initStall.
func TestTapXScriptStallPropagates(t *testing.T) {
	set := cardsettest.Set(t, "mbc")
	names := []string{"Thermopod", "Samurai of the Pale Curtain", "Abrade", "Plains"}
	deck := []cardset.Entry{}
	for _, n := range names {
		i, ok := set.Lookup(n)
		if !ok {
			t.Fatalf("%s not in the mbc pool", n)
		}
		if !set.Supported()[i] {
			t.Fatalf("%s not supported in the mbc table", n)
		}
		deck = append(deck, cardset.Entry{Name: n, Count: 1})
	}
	for i := range deck {
		if deck[i].Name == "Samurai of the Pale Curtain" {
			deck[i].Count = 2 // the two competing exile-instead replacements
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
	sc := newScenTableLib(t, tbl, id["Plains"], Config{NoElide: true})
	sc.perm(id["Thermopod"], 0)
	sc.perm(id["Samurai of the Pale Curtain"], 0)
	sc.perm(id["Samurai of the Pale Curtain"], 0)
	sc.perm(id["Plains"], 0) // the {1} of the cast: the printed census holds the {W} alone, the search's gap unit the {R}
	abrade := sc.hand(id["Abrade"], 0)
	s, k := sc.s, sc.k
	a := MakeAction(AkCast, TCode(abrade), 0, 0)
	if s.Stalled() {
		t.Fatalf("precondition: the board alone latched %s", StallName(s.StallReason()))
	}
	// the search: its only line stalls, the search reports the stall and
	// declares nothing on the real state (the declaration is the drop site's)
	found, stall := s.tapXScript(k, a, 0, false)
	if found {
		t.Fatal("the search found a payment path, want its only line to stall")
	}
	if stall != StallReplLeave {
		t.Fatalf("the search reported the failed line as plain no-payment: stall %s, want %s",
			StallName(stall), StallName(StallReplLeave))
	}
	if s.Stalled() {
		t.Fatalf("the search declared the stall itself (want the drop site's): %s", StallName(s.StallReason()))
	}
	// the offer walk: the play is dropped and the latched stall declared on the
	// real state, not silently swallowed
	var d, capSpent Decision
	s.scriptOffers(k, &d, &capSpent, 0)
	if hasOption(&d, a) {
		t.Fatal("the offer walk kept the play whose only payment path stalls")
	}
	if !s.Stalled() || s.StallReason() != StallReplLeave {
		t.Fatalf("the offer walk dropped the play silently: stalled %v reason %s, want stalled %s",
			s.Stalled(), StallName(s.StallReason()), StallName(StallReplLeave))
	}
}

// tapXGrantReplScen: seat 0 holds a sacrifice-a-creature mana source (a
// Thermopod shape, kop.SacAnyManaAb) and two exile-instead replacements (a
// Samurai of the Pale Curtain shape, kop.ReplExileOnLeave over creatures), and
// a static grants its land "{T}: Add two mana of any one color". The test cast
// (Test Ember, {R}{R}) is past the printed census (the pod's one {R}) but the
// granted land pays it, so the tap-another search's sacrifice lines — every
// one a StallReplLeave in the fork — are not the play's only route.
func tapXGrantReplScen(t *testing.T) (*scen, Obj) {
	t.Helper()
	tbl := *v.Table
	tbl.Conds = []kop.Cond{{}} // a generated table (prepareGen's gate)
	tbl.Cards = append([]kop.CardDef(nil), v.Table.Cards...)
	tbl.Filters = append([]kop.Filter(nil), v.Table.Filters...)
	tbl.Filters = append(tbl.Filters, kop.Filter{Types: kop.TCreature, Zone: kop.ZoneBattlefield})
	creatures := kop.FilterID(len(tbl.Filters) - 1)
	tbl.Filters = append(tbl.Filters, kop.Filter{Types: kop.TLand, Ctrl: kop.CtrlYou, Zone: kop.ZoneBattlefield})
	lands := kop.FilterID(len(tbl.Filters) - 1)
	plain := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Land", Types: kop.TLand})
	rite := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Two-Colour Rite", Types: kop.TEnchantment, Abilities: []kop.Ability{
		{Kind: kop.AbStatic, Static: kop.Static{Kind: kop.StatContinuous, Filter: lands, Grant: 2}},
		{Kind: kop.AbMana, AFlags: kop.AfLatent, Cost: kop.Cost{Flags: kop.CostTap}, ManaColors: kop.ColAny, ManaN: 2},
	}})
	curtain := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Pale Curtain", Types: kop.TCreature, Power: 1, Toughness: 1, Abilities: []kop.Ability{
		{Kind: kop.AbReplacement, Repl: kop.Repl{Kind: kop.ReplExileOnLeave, Filter: creatures, C: 1}},
	}})
	pod := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Pod", Types: kop.TCreature, Power: 1, Toughness: 1, Abilities: []kop.Ability{
		{Kind: kop.AbMana, Cost: kop.Cost{Flags: kop.CostSac, N: 1, Filter: creatures}, ManaColors: kop.ColR, ManaN: 1},
	}})
	ember := CardID(len(tbl.Cards))
	emberCost := kop.ManaCost{}
	emberCost.Pips[kop.R] = 2
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Ember", Types: kop.TInstant, Cost: emberCost})
	sc := newScenTable(t, &tbl, Config{NoElide: true})
	sc.perm(rite, 0)
	land := sc.perm(plain, 0)
	if sc.s.grantIdx(sc.k, sc.slot(land)) < 0 {
		t.Fatal("precondition: the two-colour grant does not reach the land")
	}
	sc.perm(curtain, 0)
	sc.perm(curtain, 0)
	podO := sc.perm(pod, 0)
	if !sc.onBF(podO) {
		t.Fatal("precondition: the sacrifice source is not on the battlefield")
	}
	emberO := sc.hand(ember, 0)
	return sc, emberO
}

// TestTapXScriptStallSparedAGrantedPayer: on the tapXGrantReplScen board the
// tap-another search's sacrifice lines all latch StallReplLeave in the fork,
// but the play is not dropped: tapXScriptOffered's fall-through prices it from
// the granted land (sacXBesideGrant && grantActScript, mana_grantsacx.go), so
// the cast stays offered and no stall is declared on the real state — the
// search's stall is its own, not the game's.
func TestTapXScriptStallSparedAGrantedPayer(t *testing.T) {
	sc, emberO := tapXGrantReplScen(t)
	s, k := sc.s, sc.k
	a := MakeAction(AkCast, TCode(emberO), 0, 0)
	if s.Stalled() {
		t.Fatalf("precondition: the board alone latched %s", StallName(s.StallReason()))
	}
	// precondition: the tap-another search fails on the stalling sacrifice
	// lines while the grant search prices the play
	found, stall := s.tapXScript(k, a, 0, false)
	if found {
		t.Fatal("precondition: the tap-another search paid the cast, want the sacrifice lines to stall")
	}
	if stall != StallReplLeave {
		t.Fatalf("precondition: the failed lines latched %s, want %s", StallName(stall), StallName(StallReplLeave))
	}
	if s.Stalled() {
		t.Fatalf("the search declared the stall though the granted mana pays the cast: %s", StallName(s.StallReason()))
	}
	offered, gstall := s.tapXScriptOffered(k, a, 0)
	if !offered {
		t.Fatal("the offer search gave the cast up: the granted mana pays it (sacXBesideGrant && grantActScript)")
	}
	if gstall != 0 {
		t.Fatalf("the offer search propagated a stall the grant search priced past: %s", StallName(gstall))
	}
	var d, capSpent Decision
	s.scriptOffers(k, &d, &capSpent, 0)
	if !hasOption(&d, a) {
		t.Fatal("the offer walk dropped the cast its granted mana pays")
	}
	if s.Stalled() {
		t.Fatalf("the offer walk latched %s though the cast is offered", StallName(s.StallReason()))
	}
}
