package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// newPreventKWScen builds a scenario over the preventkw fixture pool: the
// printed K:Prevent carriers and the creatures and burn the tests play
// against them. The pool plays unverified cards (AllowUnverified): the
// keyword's card-def facts are what is under test, not the evidence sweep.
func newPreventKWScen(t *testing.T, deck []cardset.Entry) (*scen, map[string]CardID) {
	t.Helper()
	set, err := cardset.ByName("preventkw")
	if err != nil {
		t.Skip(err)
	}
	set = cardset.AllowUnverified(set)
	for _, e := range deck {
		if _, ok := set.Lookup(e.Name); !ok {
			t.Fatalf("%s not in the preventkw pool", e.Name)
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
	return newScenTableLib(t, tbl, id["Plains"], Config{NoElide: true}), id
}

// TestPreventKeywordCombatToAndBy: Fog Bank ("Prevent all combat damage that
// would be dealt to and dealt by CARDNAME.") blocks a 3/3. The blocker is
// dealt nothing, and it deals nothing back: assignment still happens (the
// attacker assigns lethal and the blocker assigns its power), then the whole
// event is prevented.
func TestPreventKeywordCombatToAndBy(t *testing.T) {
	sc, id := newPreventKWScen(t, []cardset.Entry{
		{Name: "Fog Bank", Count: 1}, {Name: "Hill Giant", Count: 1}, {Name: "Plains", Count: 8},
	})
	fb := sc.perm(id["Fog Bank"], 1)
	giant := sc.perm(id["Hill Giant"], 0)
	sc.combat(t, []Obj{giant}, [][2]Obj{{fb, giant}})
	if d := sc.s.perms[sc.slot(fb)].damage; d != 0 {
		t.Fatalf("Fog Bank damage = %d, want 0 (combat damage to it is prevented)", d)
	}
	if d := sc.s.perms[sc.slot(giant)].damage; d != 0 {
		t.Fatalf("Hill Giant damage = %d, want 0 (combat damage by Fog Bank is prevented)", d)
	}
	if !sc.onBF(fb) || !sc.onBF(giant) {
		t.Fatalf("Fog Bank alive=%v, Hill Giant alive=%v; want both alive (no damage was dealt)", sc.onBF(fb), sc.onBF(giant))
	}
}

// TestPreventKeywordCombatToOnly: Guard Gomazoa ("Prevent all combat damage
// that would be dealt to CARDNAME.") blocks the 3/3. It is dealt nothing but
// deals its 1 power back: the sentence names only the "to" direction.
func TestPreventKeywordCombatToOnly(t *testing.T) {
	sc, id := newPreventKWScen(t, []cardset.Entry{
		{Name: "Guard Gomazoa", Count: 1}, {Name: "Hill Giant", Count: 1}, {Name: "Plains", Count: 8},
	})
	gom := sc.perm(id["Guard Gomazoa"], 1)
	giant := sc.perm(id["Hill Giant"], 0)
	sc.combat(t, []Obj{giant}, [][2]Obj{{gom, giant}})
	if d := sc.s.perms[sc.slot(gom)].damage; d != 0 {
		t.Fatalf("Guard Gomazoa damage = %d, want 0 (combat damage to it is prevented)", d)
	}
	if d := sc.s.perms[sc.slot(giant)].damage; d != 1 {
		t.Fatalf("Hill Giant damage = %d, want 1 (Guard Gomazoa's 1 power is dealt back)", d)
	}
}

// TestPreventKeywordAllDamageTo: Dawn Elemental ("Prevent all damage that
// would be dealt to CARDNAME.") takes no noncombat damage either, and the
// same burn still lands on another creature. Fog Bank's combat-only sentence
// keeps a noncombat hit, which pins the two scopes apart.
func TestPreventKeywordAllDamageTo(t *testing.T) {
	sc, id := newPreventKWScen(t, []cardset.Entry{
		{Name: "Dawn Elemental", Count: 1}, {Name: "Fog Bank", Count: 1}, {Name: "Grizzly Bears", Count: 1},
		{Name: "Shock", Count: 1}, {Name: "Plains", Count: 8},
	})
	dawn := sc.perm(id["Dawn Elemental"], 0)
	fog := sc.perm(id["Fog Bank"], 0)
	bear := sc.perm(id["Grizzly Bears"], 0)
	shock := id["Shock"]
	if n := sc.s.replDamage(sc.k, 0, NoSlot, shock, uint8(dawn), false, 3); n != 0 {
		t.Fatalf("Dawn Elemental took %d of a 3-damage burn, want 0 (all damage to it is prevented)", n)
	}
	if n := sc.s.replDamage(sc.k, 0, NoSlot, shock, uint8(fog), false, 3); n != 3 {
		t.Fatalf("Fog Bank took %d of a 3-damage burn, want 3 (its sentence is combat-scoped)", n)
	}
	if n := sc.s.replDamage(sc.k, 0, NoSlot, shock, uint8(bear), false, 3); n != 3 {
		t.Fatalf("Grizzly Bears took %d of a 3-damage burn, want 3 (no prevention)", n)
	}
}

// TestPreventKeywordLostAbilities: the printed sentence is the card's face
// replacement, so it is gone when the carrier has lost all abilities (gorge
// replacementFace's printedAbilitiesLost; the kernel genLost): a Fog Bank
// under the loss takes the attacker's damage, and a Dawn Elemental under it
// takes the burn.
func TestPreventKeywordLostAbilities(t *testing.T) {
	sc, id := newPreventKWScen(t, []cardset.Entry{
		{Name: "Fog Bank", Count: 1}, {Name: "Hill Giant", Count: 1}, {Name: "Plains", Count: 8},
	})
	fb := sc.perm(id["Fog Bank"], 1)
	giant := sc.perm(id["Hill Giant"], 0)
	sc.s.perms[sc.slot(fb)].ovr.flags |= ovLoseAll
	sc.combat(t, []Obj{giant}, [][2]Obj{{fb, giant}})
	if sc.onBF(fb) {
		t.Fatal("Fog Bank alive; want it dealt the attacker's 3 (0/2, prevention gone with the loss)")
	}
	if d := sc.s.perms[sc.slot(giant)].damage; d != 0 {
		t.Fatalf("Hill Giant damage = %d, want 0 (Fog Bank's 0 power is dealt back)", d)
	}

	sc2, id2 := newPreventKWScen(t, []cardset.Entry{
		{Name: "Dawn Elemental", Count: 1}, {Name: "Shock", Count: 1}, {Name: "Plains", Count: 8},
	})
	dawn := sc2.perm(id2["Dawn Elemental"], 0)
	sc2.s.perms[sc2.slot(dawn)].ovr.flags |= ovLoseAll
	if n := sc2.s.replDamage(sc2.k, 0, NoSlot, id2["Shock"], uint8(dawn), false, 3); n != 3 {
		t.Fatalf("Dawn Elemental took %d of a 3-damage burn, want 3 (it lost all abilities: no prevention)", n)
	}
}

// preventKWAsk returns the next decision: the CR 616.1 order ask when a
// second replacement competes with the carrier's printed prevention. The
// Shock is already cast and targeted; priority is passed only while it is on
// the stack.
func preventKWAsk(t *testing.T, sc *scen, _ Obj) Decision {
	t.Helper()
	s := sc.s
	for i := 0; i < 12; i++ {
		d := s.Step()
		switch {
		case d.Kind == DkChoose && d.Sub == ChGenReplOrder:
			return d
		case d.Kind == DkPriority && s.sp > 0:
			s.Apply(MakeAction(AkPass, 0, 0, 0))
		case d.Kind != DkPriority && d.N == 1:
			s.Apply(d.Options[0])
		default:
			t.Fatalf("unexpected decision kind %d sub %d player %d opts %v", d.Kind, d.Sub, d.Player, d.Options[:d.N])
		}
	}
	t.Fatal("no order ask after Shock resolved against the shielded carrier")
	return Decision{}
}

// passPriority passes the priority holder's priority until the other player
// holds it (castCard offers only the holder's casts: the Shock here is cast
// by the nonactive player, so the active player passes first).
func passPriority(t *testing.T, sc *scen, player uint8) {
	t.Helper()
	for i := 0; i < 8; i++ {
		d := sc.s.Step()
		if d.Kind == DkPriority && d.Player == player {
			return // it stays pending for the caller's Step
		}
		if d.Kind != DkPriority {
			t.Fatalf("decision kind %d sub %d player %d, want priority for player %d", d.Kind, d.Sub, d.Player, player)
		}
		sc.s.Apply(MakeAction(AkPass, 0, 0, 0))
	}
	t.Fatalf("priority never reached player %d", player)
}

// TestPreventKeywordOrderAsk: a shield counter on Dawn Elemental and a burn
// spell both match the same noncombat damage ("Prevent all damage" and CR
// 122.1c), so the Elemental's controller is asked which applies: picking the
// printed prevention first keeps the counter, picking the shield spends it;
// with the counter gone the printed prevention alone applies unasked.
func TestPreventKeywordOrderAsk(t *testing.T) {
	sc, id := newPreventKWScen(t, []cardset.Entry{
		{Name: "Dawn Elemental", Count: 1}, {Name: "Boon of Safety", Count: 1}, {Name: "Shock", Count: 1},
		{Name: "Plains", Count: 8}, {Name: "Mountain", Count: 4},
	})
	s := sc.s
	dawn := sc.perm(id["Dawn Elemental"], 0)
	sc.lands(0, id["Plains"], 2)
	sc.lands(1, id["Mountain"], 3)
	sc.castCard(t, sc.hand(id["Boon of Safety"], 0))
	sc.do(t, AkTarget, 0xff, uint8(dawn))
	sc.resolveAll(t)
	if n := s.Counters(dawn, kop.CtrShield); n != 1 {
		t.Fatalf("Dawn Elemental holds %d SHIELD after Boon of Safety, want 1", n)
	}
	// the ask has the printed prevention and the shield, both the
	// Elemental's own object (gorge collects the printed line in its scan
	// and appends the shield last, so the modes run 0, 1)
	passPriority(t, sc, 1)
	sc.castCard(t, sc.hand(id["Shock"], 1))
	sc.do(t, AkTarget, 0xff, uint8(dawn))
	d := preventKWAsk(t, sc, dawn)
	if d.Player != 0 || d.N != 2 {
		t.Fatalf("order ask player %d n %d opts %v, want the controller's two-option ask", d.Player, d.N, d.Options[:d.N])
	}
	var opts [2]Action
	for i := 0; i < int(d.N); i++ {
		a := d.Options[i]
		if a.Source() != TCode(uint8(dawn)) {
			t.Fatalf("option %d source %d, want the Elemental's own replacement", i, a.Source())
		}
		opts[a.Mode()] = a
	}
	s.Apply(opts[0]) // the printed prevention first: the counter stays
	sc.resolveAll(t)
	if n, dmg := s.Counters(dawn, kop.CtrShield), s.perms[sc.slot(dawn)].damage; n != 1 || dmg != 0 {
		t.Fatalf("printed prevention picked: %d SHIELD, %d damage; want 1, 0", n, dmg)
	}

	passPriority(t, sc, 1)
	sc.castCard(t, sc.hand(id["Shock"], 1))
	sc.do(t, AkTarget, 0xff, uint8(dawn))
	d = preventKWAsk(t, sc, dawn)
	for i := 0; i < int(d.N); i++ {
		opts[d.Options[i].Mode()] = d.Options[i]
	}
	s.Apply(opts[1]) // the shield first: it is spent
	sc.resolveAll(t)
	if n, dmg := s.Counters(dawn, kop.CtrShield), s.perms[sc.slot(dawn)].damage; n != 0 || dmg != 0 {
		t.Fatalf("shield picked: %d SHIELD, %d damage; want 0, 0", n, dmg)
	}

	passPriority(t, sc, 1)
	sc.castCard(t, sc.hand(id["Shock"], 1))
	sc.do(t, AkTarget, 0xff, uint8(dawn))
	for i := 0; i < 12; i++ {
		d := sc.s.Step()
		if d.Kind == DkChoose && d.Sub == ChGenReplOrder {
			t.Fatal("an order ask over the printed prevention alone (one candidate)")
		}
		if d.Kind == DkPriority && s.sp == 0 {
			break
		}
		if d.Kind == DkPriority {
			s.Apply(MakeAction(AkPass, 0, 0, 0))
			continue
		}
		s.Apply(d.Options[0])
	}
	if dmg := s.perms[sc.slot(dawn)].damage; dmg != 0 {
		t.Fatalf("third Shock dealt %d, want 0 (the printed prevention still applies)", dmg)
	}
}
