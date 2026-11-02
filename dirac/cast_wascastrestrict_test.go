package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// kwrScen: the wascastspell pool table with p0's battlefield from board
// (each name once, untapped, not sick) and the restricted ability's filter of
// Mm'menon's granted mana.
func kwrScen(t *testing.T, board ...string) (sc *scen, id map[string]CardID, f kop.FilterID) {
	t.Helper()
	set := cardsettest.Set(t, "wascastspell")
	tbl, err := set.Pool()
	if err != nil {
		t.Fatal(err)
	}
	id = map[string]CardID{}
	for c := range tbl.Cards {
		if _, ok := id[tbl.Cards[c].Name]; !ok {
			id[tbl.Cards[c].Name] = CardID(c)
		}
	}
	sc = newScenTableLib(t, tbl, id["Plains"], Config{NoElide: true})
	for _, n := range board {
		c, ok := id[n]
		if !ok {
			t.Fatalf("%s not in the wascastspell table", n)
		}
		sc.perm(c, 0)
	}
	for _, a := range tbl.Cards[id["Mm'menon, the Right Hand"]].Abilities {
		if a.Kind == kop.AbMana && a.AFlags&kop.AfLatent != 0 {
			f = a.ManaRestrict
		}
	}
	if f == 0 || sc.k.gen == nil || sc.k.gen.restrF[0] != f && sc.k.gen.restrF[1] != f { // (Interdimensional Web Watch's restriction is registered too)
		t.Fatalf("Mm'menon's granted restriction %d is not the registered one %v", f, sc.k.gen.restrF)
	}
	return sc, id, f
}

// kwrSet sets restriction f's PredWasCastSpell origin word (and type mask)
// for a test, returning the restore.
func kwrSet(k *prep, f kop.FilterID, n int16, types kop.TypeMask) func() {
	old := k.t.Filters[f]
	k.t.Filters[f].Pred, k.t.Filters[f].PredN, k.t.Filters[f].Types = kop.PredWasCastSpell, n, types
	return func() { k.t.Filters[f] = old }
}

// TestWascastrestrictOrigin: the restriction matchers read the cast's origin
// as gorge's castProvenanceAdmitsPending does (restrSpendable, the floating
// poolR read; restrSpellMatch, the restricted planner source's read). An
// offer (cast false) reads every family from the object's current zone, as the
// charge will (gorge castProvenanceAdmitsPending); the payment (cast true) reads
// the pushed entry honestly. A
// type predicate beside the word still applies once the word is stripped.
func TestWascastrestrictOrigin(t *testing.T) {
	sc, id, f := kwrScen(t, "Mm'menon, the Right Hand")
	s, k := sc.s, sc.k
	shock, giant := id["Shock"], id["Hill Giant"]
	if !s.pl[0].addRestr(k, &s.poolRx[0], f, int(kop.U), 1) {
		t.Fatal("restricted mana did not float")
	}
	zones := []kop.Zone{kop.ZoneHand, kop.ZoneExile, kop.ZoneGraveyard, kop.ZoneLibrary}
	for _, c := range []struct {
		name  string
		n     int16
		types kop.TypeMask
		card  CardID
		cast  bool
		want  func(z kop.Zone) bool
	}{
		{"exile offer", 3, 0, shock, false, func(z kop.Zone) bool { return z == kop.ZoneExile }},
		{"exile payment", 3, 0, shock, true, func(z kop.Zone) bool { return z == kop.ZoneExile }},
		{"not hand offer", 16 | 1, 0, shock, false, func(z kop.Zone) bool { return z != kop.ZoneHand }},
		{"not hand payment", 16 | 1, 0, shock, true, func(z kop.Zone) bool { return z != kop.ZoneHand }},
		{"hand offer", 1, 0, shock, false, func(z kop.Zone) bool { return z == kop.ZoneHand }},
		{"hand payment", 1, 0, shock, true, func(z kop.Zone) bool { return z == kop.ZoneHand }},
		{"cast+not hand payment", 4, 0, shock, true, func(z kop.Zone) bool { return z != kop.ZoneHand }},
		{"creature from exile: an instant", 3, kop.TCreature, shock, false, func(kop.Zone) bool { return false }},
		{"creature from exile: a creature", 3, kop.TCreature, giant, false, func(z kop.Zone) bool { return z == kop.ZoneExile }},
	} {
		restore := kwrSet(k, f, c.n, c.types)
		for _, z := range zones {
			og := restrOrigin{from: z, cast: c.cast, byYou: true}
			if got := s.restrSpendable(k, 0, c.card, og); got != c.want(z) {
				t.Errorf("%s from zone %d: restrSpendable %v, want %v", c.name, z, got, c.want(z))
			}
			if got := s.restrSpellMatch(k, 0, NoSlot, f, c.card, og); got != c.want(z) {
				t.Errorf("%s from zone %d: restrSpellMatch %v, want %v", c.name, z, got, c.want(z))
			}
		}
		restore()
	}

	// the zero origin fails closed; the mutation makes every restriction hold
	restore := kwrSet(k, f, 3, 0)
	defer restore()
	if s.restrSpendable(k, 0, shock, restrOrigin{}) {
		t.Errorf("the zero origin admitted an exile restriction")
	}
	DebugRestrProvIgnored = true
	defer func() { DebugRestrProvIgnored = false }()
	if !s.restrSpendable(k, 0, shock, restrOrigin{from: kop.ZoneHand}) {
		t.Errorf("DebugRestrProvIgnored did not admit a hand cast")
	}
}

// TestWascastrestrictGrantSearch: the grant prefix search skips a restricted
// granted source the play's origin does not admit, uses one it does, and the
// found prefix floats the mana in poolR, never the ordinary pool; a prefix
// that cannot float it (poolR holds another colour) stalls loudly
// (StallRestrSrc) rather than spilling.
func TestWascastrestrictGrantSearch(t *testing.T) {
	sc, id, f := kwrScen(t, "Mm'menon, the Right Hand", "Ornithopter")
	s, k := sc.s, sc.k
	cost := kop.ManaCost{}
	cost.Pips[kop.U] = 1
	spell := func(z kop.Zone, cast bool) payDesc {
		return payDesc{class: pdSpell, card: id["Shock"], src: NoSlot, og: restrOrigin{from: z, cast: cast, byYou: true}}
	}
	var g grantSrch

	// Spell.!wasCastFromYourHand: the pending read takes the object's zone; a
	// payment-style read admits a cast not from a hand
	if !s.grantSearch(k, 0, &cost, NoSlot, spell(kop.ZoneGraveyard, false), &g) {
		t.Errorf("!wasCastFromYourHand: a pending play from a graveyard did not find the restricted grant")
	}
	if s.grantSearch(k, 0, &cost, NoSlot, spell(kop.ZoneHand, false), &g) {
		t.Errorf("!wasCastFromYourHand: a pending play from a hand found the restricted grant")
	}
	if !s.grantSearch(k, 0, &cost, NoSlot, spell(kop.ZoneGraveyard, true), &g) || g.np != 1 {
		t.Errorf("!wasCastFromYourHand from a graveyard, honest read: found %v steps %d", g.np == 1, g.np)
	}
	if s.grantSearch(k, 0, &cost, NoSlot, spell(kop.ZoneHand, true), &g) {
		t.Errorf("!wasCastFromYourHand: a hand cast found the restricted grant")
	}
	if s.grantSearch(k, 0, &cost, NoSlot, payDesc{class: pdAct, card: id["Shock"], src: NoSlot}, &g) {
		t.Errorf("a spell restriction: an activation found the grant")
	}

	// wasCastFromExile (the origin family is admitted pending): float it
	restore := kwrSet(k, f, 3, 0)
	defer restore()
	if s.grantSearch(k, 0, &cost, NoSlot, spell(kop.ZoneHand, false), &g) {
		t.Errorf("wasCastFromExile: a cast from a hand found the grant")
	}
	if !s.grantSearch(k, 0, &cost, NoSlot, spell(kop.ZoneExile, false), &g) || g.np != 1 || g.path[0].gi < 0 {
		t.Fatalf("wasCastFromExile from exile: found %v steps %d", g.np == 1, g.np)
	}
	s.grantApply(k, &g)
	if pl := &s.pl[0]; pl.pool[kop.U] != 0 || pl.poolR&poolRCnt != 1 || int(pl.poolR>>poolRShift) != int(kop.U) {
		t.Errorf("applied prefix: pool U %d, poolR %#x; want the mana in poolR", pl.pool[kop.U], pl.poolR)
	}
	if s.g.stalled != 0 {
		t.Errorf("a floatable restricted grant stalled %s", StallName(s.g.stalled))
	}

	// the grant's colour is full beside another restricted colour: it cannot
	// float, and never spills into the ordinary pool
	sc2, id2, f2 := kwrScen(t, "Mm'menon, the Right Hand", "Ornithopter")
	s2, k2 := sc2.s, sc2.k
	defer kwrSet(k2, f2, 3, 0)()
	if !s2.pl[0].addRestr(k2, &s2.poolRx[0], f2, int(kop.W), 1) {
		t.Fatal("setup: restricted W did not float")
	}
	s2.poolRx[0] = 7 << (3 * kop.U)
	d := payDesc{class: pdSpell, card: id2["Shock"], src: NoSlot, og: restrOrigin{from: kop.ZoneExile}}
	if !s2.grantSearch(k2, 0, &cost, NoSlot, d, &g) || g.np != 1 {
		t.Fatalf("full colour: found %v steps %d", g.np == 1, g.np)
	}
	s2.grantApply(k2, &g)
	if s2.g.stalled != StallRestrSrc {
		t.Errorf("a grant that cannot float: stalled %q, want restrsrc", StallName(s2.g.stalled))
	}
}
