package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestShieldPrepOffOnFrozenPools: FDN and BLB place no SHIELD counter and
// have no StYouHexproof static, so the shield arms never run there.
func TestShieldPrepOffOnFrozenPools(t *testing.T) {
	blbSet := cardsettest.Set(t, "blb")
	fdnSet := cardsettest.Set(t, "fdn")
	for _, f := range []func() (*kop.Table, error){fdnSet.Pool, blbSet.Pool} {
		tb, err := f()
		if err != nil {
			t.Fatal(err)
		}
		g := genPrep{replBits: make([]uint32, len(tb.Cards))}
		prepShield(tb, &g)
		if g.anyRepl&rbShield != 0 || g.anyYouHex {
			t.Fatal("rbShield / anyYouHex on a frozen pool")
		}
	}
}

func shieldScen(t *testing.T, names ...string) (*scen, map[string]CardID) {
	t.Helper()
	deck := []cardset.Entry{{Name: "Mountain", Count: 8}, {Name: "Swamp", Count: 8}}
	kshieldSet := cardsettest.Set(t, "shield")
	for _, n := range names {
		if i, ok := kshieldSet.Lookup(n); !ok || !kshieldSet.Supported()[i] {
			t.Fatalf("%s not supported in the shield table", n)
		}
		deck = append(deck, cardset.Entry{Name: n, Count: 1})
	}
	tbl, da, _, err := kshieldSet.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	var lib [20]CardID
	for i := range lib {
		lib[i] = id["Mountain"]
	}
	s := new(State)
	s.Init(tbl, lib[:], lib[:], 1, Config{NoElide: true})
	for p := uint8(0); p < 2; p++ {
		pls := &s.pl[p]
		for pls.handN > 0 {
			o := pls.hand[0]
			s.detach(o)
			s.objs[o].zone = kop.ZoneLibrary
			pls.libBottom(o)
		}
	}
	s.turn, s.active, s.step, s.prio, s.passes, s.ctl = 2, 0, kop.StepMain1, 0, 0, ctlPriority
	return &scen{s: s, k: s.k()}, id
}

// castAt casts hand card o at target object tgt and resolves it.
func (sc *scen) castAt(t *testing.T, o, tgt Obj) {
	t.Helper()
	sc.castCard(t, o)
	if d := sc.s.Step(); d.Kind != DkPriority {
		sc.s.Apply(MakeAction(AkTarget, 0, TCode(tgt), 0))
	}
	sc.resolveAll(t)
}

// TestShieldCounterReplaces: Rhox Pummeler enters with a SHIELD counter
// (and has trample while it does); Shock is prevented whole and spends it,
// a second Shock lands. Swooping Protector's counter replaces Murder (it
// stays, untapped: not regeneration), a second Murder destroys it.
func TestShieldCounterReplaces(t *testing.T) {
	sc, id := shieldScen(t, "Rhox Pummeler", "Swooping Protector", "Shock", "Murder")
	if sc.k.gen.anyRepl&rbShield == 0 {
		t.Fatal("rbShield unset with SHIELD placers in the table")
	}
	sc.lands(0, id["Mountain"], 4)
	sc.lands(0, id["Swamp"], 6)
	rhox := sc.perm(id["Rhox Pummeler"], 1)
	if n := sc.s.Counters(rhox, kop.CtrShield); n != 1 {
		t.Fatalf("Rhox Pummeler enters with %d SHIELD, want 1", n)
	}
	if sc.s.kw(sc.k, sc.slot(rhox))&kop.KwTrample == 0 {
		t.Fatal("no trample with a shield counter")
	}
	sc.castAt(t, sc.hand(id["Shock"], 0), rhox)
	if n, d := sc.s.Counters(rhox, kop.CtrShield), sc.s.perms[sc.slot(rhox)].damage; n != 0 || d != 0 {
		t.Fatalf("after Shock: %d SHIELD, %d damage; want 0, 0", n, d)
	}
	if sc.s.kw(sc.k, sc.slot(rhox))&kop.KwTrample != 0 {
		t.Fatal("trample without a shield counter")
	}
	sc.castAt(t, sc.hand(id["Shock"], 0), rhox)
	if d := sc.s.perms[sc.slot(rhox)].damage; d != 2 {
		t.Fatalf("second Shock dealt %d, want 2", d)
	}
	bird := sc.perm(id["Swooping Protector"], 1)
	sc.castAt(t, sc.hand(id["Murder"], 0), bird)
	if !sc.onBF(bird) || sc.s.Counters(bird, kop.CtrShield) != 0 || sc.s.perms[sc.slot(bird)].flags&pfTapped != 0 {
		t.Fatal("Murder over a shield counter: want the Protector on the battlefield, untapped, its counter spent")
	}
	sc.castAt(t, sc.hand(id["Murder"], 0), bird)
	if sc.onBF(bird) {
		t.Fatal("second Murder: the Protector survived")
	}
}

// TestYouHexproofWhileShielded: Captain America gives his controller
// hexproof only while he has a shield counter (StYouHexproof under Cond2).
func TestYouHexproofWhileShielded(t *testing.T) {
	sc, id := shieldScen(t, "Captain America, Super-Soldier", "Shock")
	if !sc.k.gen.anyYouHex {
		t.Fatal("anyYouHex unset")
	}
	ca := sc.perm(id["Captain America, Super-Soldier"], 1)
	if !sc.s.playerHexproof(sc.k, 1) || sc.s.playerHexproof(sc.k, 0) {
		t.Fatal("want player 1 (Captain America's controller) hexproof, player 0 not")
	}
	sc.s.spendShield(sc.slot(ca))
	if sc.s.playerHexproof(sc.k, 1) {
		t.Fatal("player 1 hexproof with no shield counter left")
	}
}

// shockToAsk casts Shock from player 0 at code and returns the next
// decision (the CR 616.1 order ask when the shield competes with another
// replacement).
func shockToAsk(t *testing.T, sc *scen, id map[string]CardID, code uint8) Decision {
	t.Helper()
	s := sc.s
	sc.lands(0, id["Mountain"], 1)
	sh := sc.hand(id["Shock"], 0)
	sc.do(t, AkCast, uint8(sh), 0xff)
	for i := 0; i < 8; i++ {
		d := s.Step()
		switch {
		case d.Kind == DkTarget:
			sc.do(t, AkTarget, 0xff, code)
		case d.Kind == DkPriority && s.sp > 0:
			s.Apply(MakeAction(AkPass, 0, 0, 0))
		case d.Kind == DkChoose && d.Sub != ChGenReplOrder:
			s.Apply(d.Options[0])
		default:
			return d
		}
	}
	t.Fatal("no decision after Shock resolved")
	return Decision{}
}

// TestShieldNoncombatOrder: Crystal Barricade's prevention and the shield
// counter both match noncombat damage to Rhox Pummeler. The controller is
// asked; picking the Barricade first prevents the damage without spending
// the counter, picking the shield first spends it.
func TestShieldNoncombatOrder(t *testing.T) {
	for _, pick := range []string{"Crystal Barricade", "Rhox Pummeler"} {
		sc, id := shieldScen(t, "Rhox Pummeler", "Crystal Barricade", "Shock")
		s := sc.s
		rhox := sc.perm(id["Rhox Pummeler"], 1)
		bar := sc.perm(id["Crystal Barricade"], 1)
		d := shockToAsk(t, sc, id, uint8(rhox))
		if d.Kind != DkChoose || d.Sub != ChGenReplOrder || d.Player != 1 || d.N != 2 {
			t.Fatalf("pick %s: want the controller's two-option order ask, got kind %d sub %d player %d opts %v", pick, d.Kind, d.Sub, d.Player, d.Options[:d.N])
		}
		target := bar
		if pick == "Rhox Pummeler" {
			target = rhox
		}
		found := false
		for i := 0; i < int(d.N); i++ {
			if Obj(d.Options[i].Source()) == target {
				s.Apply(d.Options[i])
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("pick %s: no option for %d (opts %v)", pick, target, d.Options[:d.N])
		}
		s.Step()
		want := 1
		if pick == "Rhox Pummeler" {
			want = 0
		}
		if n, dmg := s.Counters(rhox, kop.CtrShield), s.perms[sc.slot(rhox)].damage; n != want || dmg != 0 {
			t.Fatalf("pick %s: %d SHIELD, %d damage; want %d, 0", pick, n, dmg, want)
		}
	}
}

// TestShieldCombatOrderDoubler: a Twinflame Tyrant doubles the blocker's
// combat damage to a shielded Rhox Pummeler. The order ask is posed with
// the Tyrant and the shield; the shield's prevention spends the counter.
func TestShieldCombatOrderDoubler(t *testing.T) {
	sc, id := shieldScen(t, "Rhox Pummeler", "Twinflame Tyrant", "Grizzly Bears")
	s := sc.s
	rhox := sc.perm(id["Rhox Pummeler"], 1)
	ty := sc.perm(id["Twinflame Tyrant"], 0)
	bear := sc.perm(id["Grizzly Bears"], 0)
	sc.combat(t, []Obj{bear}, [][2]Obj{{rhox, bear}})
	d := s.Step()
	if d.Kind != DkChoose || d.Sub != ChGenReplOrder || d.Player != 1 || d.N != 2 {
		t.Fatalf("want the blocker controller's order ask over the Tyrant and the shield, got kind %d sub %d player %d opts %v", d.Kind, d.Sub, d.Player, d.Options[:d.N])
	}
	var shieldPick *Action
	for i := 0; i < int(d.N); i++ {
		if Obj(d.Options[i].Source()) == rhox {
			shieldPick = &d.Options[i]
		} else if Obj(d.Options[i].Source()) != ty {
			t.Fatalf("option source %d is neither the Tyrant nor the shield", d.Options[i].Source())
		}
	}
	if shieldPick == nil {
		t.Fatal("no shield option")
	}
	s.Apply(*shieldPick)
	s.Step()
	if n, dmg := s.Counters(rhox, kop.CtrShield), s.perms[sc.slot(rhox)].damage; n != 0 || dmg != 0 {
		t.Fatalf("after the answer: %d SHIELD, %d damage; want 0, 0", n, dmg)
	}
}

// drawerScen is shieldScen plus a synthetic 2/20 creature whose combat
// damage to anything gains its controller 5 life (a DamageDone trigger with
// TfSelf|TfCombat), so a prevented event that still fires its trigger is
// visible as life the controller never should have gained.
func drawerScen(t *testing.T) (*scen, map[string]CardID) {
	t.Helper()
	kshieldSet := cardsettest.Set(t, "shield")
	tb, err := kshieldSet.Pool()
	if err != nil {
		t.Fatal(err)
	}
	tbl := *tb
	tbl.Cards = append([]kop.CardDef(nil), tb.Cards...)
	tbl.Cards = append(tbl.Cards, kop.CardDef{
		Name: "Test Damage Drawer", Types: kop.TCreature, Power: 2, Toughness: 20,
		Abilities: []kop.Ability{{Kind: kop.AbTriggered,
			Trig: kop.Trigger{Event: kop.TrigDamageDone, Flags: kop.TfSelf | kop.TfCombat},
			Prog: []kop.Instr{{Op: kop.OpGainLife, Obj: kop.RefController, A: kop.Const(5)}}}},
	})
	id := map[string]CardID{}
	for i := range tbl.Cards {
		id[tbl.Cards[i].Name] = CardID(i)
	}
	var lib [20]CardID
	for i := range lib {
		lib[i] = id["Mountain"]
	}
	s := new(State)
	s.Init(&tbl, lib[:], lib[:], 1, Config{NoElide: true})
	for p := uint8(0); p < 2; p++ {
		pls := &s.pl[p]
		for pls.handN > 0 {
			o := pls.hand[0]
			s.detach(o)
			s.objs[o].zone = kop.ZoneLibrary
			pls.libBottom(o)
		}
	}
	s.turn, s.active, s.step, s.prio, s.passes, s.ctl = 2, 0, kop.StepMain1, 0, 0, ctlPriority
	return &scen{s: s, k: s.k()}, id
}

// runToMain2 answers whatever the damage step asks and passes until the
// game reaches seat 0's main phase 2 with an empty stack.
func runToMain2(t *testing.T, sc *scen) {
	t.Helper()
	s := sc.s
	for i := 0; i < 80; i++ {
		d := s.Step()
		if d.Kind == DkPriority && s.sp == 0 && s.step == kop.StepMain2 {
			return
		}
		if d.Kind == DkPriority {
			s.Apply(MakeAction(AkPass, 0, 0, 0))
			continue
		}
		s.Apply(d.Options[0])
	}
	t.Fatalf("never reached main 2")
}

// TestShieldFrozenPreventionNoDamageTrigger: a frozen combat pass (the
// Tyrant + Fleeting Flight event on the other pair poses the order ask)
// walks the Drawer's event to a shield-only recipient through
// dealCombatEvent. The shield prevents the event, so the Drawer's
// DamageDone trigger must not fire (gorge's shield counter prevents the
// damage event; it does not deal zero damage).
func TestShieldFrozenPreventionNoDamageTrigger(t *testing.T) {
	sc, id := drawerScen(t)
	s := sc.s
	drawer := sc.perm(id["Test Damage Drawer"], 0)
	grizzly := sc.perm(id["Grizzly Bears"], 0)
	rhox := sc.perm(id["Rhox Pummeler"], 1)
	swoop := sc.perm(id["Swooping Protector"], 1)
	sc.perm(id["Twinflame Tyrant"], 1)
	sc.lands(0, id["Plains"], 1)
	sc.castAt(t, sc.hand(id["Fleeting Flight"], 0), grizzly)
	if s.kw(sc.k, sc.slot(grizzly))&kop.KwPreventCombatDamageTo == 0 {
		t.Fatal("Fleeting Flight did not grant the prevention")
	}
	sc.combat(t, []Obj{drawer, grizzly}, [][2]Obj{{rhox, drawer}, {swoop, grizzly}})
	d := s.Step()
	if d.Kind != DkChoose || d.Sub != ChGenReplOrder || d.N != 2 {
		t.Fatalf("want the other pair's two-option order ask, got kind %d sub %d opts %v", d.Kind, d.Sub, d.Options[:d.N])
	}
	s.Apply(d.Options[0])
	runToMain2(t, sc)
	if n := s.Counters(rhox, kop.CtrShield); n != 0 {
		t.Fatalf("the Drawer's event did not spend the shield: %d counters", n)
	}
	if life := s.pl[0].life; life != 20 {
		t.Fatalf("the shield prevented the Drawer's damage but its DamageDone trigger gained life: %d, want 20", life)
	}
}

// TestShieldOrderPreventionNoDamageTrigger: the Drawer's event itself is
// asked (its controller's Twinflame Tyrant doubles it, the recipient's
// shield prevents it). Picking the shield takes the event out of the pass
// through dealAsked, so the DamageDone trigger must not fire.
func TestShieldOrderPreventionNoDamageTrigger(t *testing.T) {
	sc, id := drawerScen(t)
	s := sc.s
	drawer := sc.perm(id["Test Damage Drawer"], 0)
	sc.perm(id["Twinflame Tyrant"], 0)
	rhox := sc.perm(id["Rhox Pummeler"], 1)
	sc.combat(t, []Obj{drawer}, [][2]Obj{{rhox, drawer}})
	d := s.Step()
	if d.Kind != DkChoose || d.Sub != ChGenReplOrder || d.N != 2 {
		t.Fatalf("want the two-option order ask, got kind %d sub %d opts %v", d.Kind, d.Sub, d.Options[:d.N])
	}
	picked := false
	for i := 0; i < int(d.N); i++ {
		if Obj(d.Options[i].Source()) == rhox {
			s.Apply(d.Options[i])
			picked = true
			break
		}
	}
	if !picked {
		t.Fatal("no shield option")
	}
	runToMain2(t, sc)
	if n := s.Counters(rhox, kop.CtrShield); n != 0 {
		t.Fatalf("the shield was not spent: %d counters", n)
	}
	if life := s.pl[0].life; life != 20 {
		t.Fatalf("the shield prevented the Drawer's damage but its DamageDone trigger gained life: %d, want 20", life)
	}
}

// TestShieldCombatOrderRecipientPrevention: Fleeting Flight grants Rhox
// Pummeler the combat-damage prevention. The prevention and the shield are
// both candidates (same source, gorge's scan order: prevention then shield);
// picking the prevention first keeps the counter, picking the shield first
// spends it.
func TestShieldCombatOrderRecipientPrevention(t *testing.T) {
	for _, pick := range []uint8{0, 1} {
		sc, id := shieldScen(t, "Rhox Pummeler", "Grizzly Bears", "Fleeting Flight", "Plains")
		s := sc.s
		rhox := sc.perm(id["Rhox Pummeler"], 1)
		bear := sc.perm(id["Grizzly Bears"], 0)
		sc.lands(0, id["Plains"], 1)
		sc.castAt(t, sc.hand(id["Fleeting Flight"], 0), rhox)
		if s.kw(sc.k, sc.slot(rhox))&kop.KwPreventCombatDamageTo == 0 {
			t.Fatal("Fleeting Flight did not grant the prevention")
		}
		sc.combat(t, []Obj{bear}, [][2]Obj{{rhox, bear}})
		d := s.Step()
		if d.Kind != DkChoose || d.Sub != ChGenReplOrder || d.Player != 1 || d.N != 2 {
			t.Fatalf("pick %d: want the two-option order ask, got kind %d sub %d player %d opts %v", pick, d.Kind, d.Sub, d.Player, d.Options[:d.N])
		}
		for i := 0; i < int(d.N); i++ {
			if d.Options[i].Mode() == pick {
				s.Apply(d.Options[i])
				break
			}
		}
		s.Step()
		want := 1
		if pick == 1 {
			want = 0
		}
		if n, dmg := s.Counters(rhox, kop.CtrShield), s.perms[sc.slot(rhox)].damage; n != want || dmg != 0 {
			t.Fatalf("pick %d: %d SHIELD, %d damage; want %d, 0", pick, n, dmg, want)
		}
	}
}

// TestShieldSpentOnLethalDamage: state-based lethal destruction spends a shield
// counter instead (CR 122.1c, 704.5g), and the damage stays marked, so the next pass
// of the same check destroys the creature with no counter left.
func TestShieldSpentOnLethalDamage(t *testing.T) {
	sc, id := shieldScen(t, "Rhox Pummeler")
	rhox := sc.perm(id["Rhox Pummeler"], 1)
	sl := sc.slot(rhox)
	_, tough := sc.s.pt(sc.k, sl)
	sc.s.perms[sl].damage = uint8(tough)
	sc.s.sba(sc.k)
	if sc.onBF(rhox) {
		t.Fatal("lethal damage marked and no counter left: want it destroyed")
	}
	if n := sc.s.perms[sl].ctrN(sc.s, int(kop.CtrShield)); n != 0 {
		t.Fatalf("%d SHIELD on the departed record, want the counter spent", n)
	}
}
