package dirac

import (
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// restrSourceScen: the restrictedsource pool's table with the named cards (and ten Plains)
// in the deck; the scenario starts with every hand and the battlefield
// empty.
func restrSourceScen(t *testing.T, names ...string) (sc *scen, id map[string]CardID) {
	t.Helper()
	deck := []cardset.Entry{{Name: "Plains", Count: 10}}
	set := cardsettest.Set(t, "restrictedsource")
	for _, n := range names {
		if i, ok := set.Lookup(n); !ok || !set.Supported()[i] {
			t.Fatalf("%s not in the restrictedsource table", n)
		}
		deck = append(deck, cardset.Entry{Name: n, Count: 1})
	}
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id = map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	for i := range tbl.Cards {
		if c := &tbl.Cards[i]; c.Types&kop.TToken != 0 && c.Types&kop.TArtifact != 0 && len(c.Abilities) == 1 && c.Abilities[0].Kind == kop.AbMana && c.Abilities[0].ManaRestrict != 0 {
			id["Powerstone Token"] = CardID(i) // (a token def has no name)
		}
	}
	sc = newScenTableLib(t, tbl, id["Plains"], Config{NoElide: true})
	return sc, id
}

// restrSourceChose sets the creature type the Courtyard c chose.
func restrSourceChose(t *testing.T, sc *scen, c Obj, typ string) {
	t.Helper()
	if d := sc.s.Step(); d.Kind == DkChoose { // its as-enters creature type: answered, then set
		sc.s.Apply(d.Options[0])
	}
	for i, n := range sc.k.t.Subtypes {
		if n == typ {
			sc.s.perms[sc.slot(c)].chosen = objWord(uint8(i))
			return
		}
	}
	t.Fatalf("no subtype %s in the table", typ)
}

func actOffered(d *Decision, o Obj) bool {
	for _, a := range d.Options[:d.N] {
		if a.Kind() == AkActivate && a.Source() == TCode(uint8(o)) {
			return true
		}
	}
	return false
}

// TestRestrictedSourceAdmission (gorge; CR 106.6): an
// untapped restricted-mana source is a planner unit for exactly the
// payments its restriction admits: offered and planned (the plan taps it
// at the cast), never for the others.
func TestRestrictedSourceAdmission(t *testing.T) {
	if m := os.Getenv("KF24_MUT"); m != "" { // TestRestrictedSourceAdmissionMutation's child
		DebugRestrictedSource = m[0] - '0'
		defer func() { DebugRestrictedSource = 0 }()
	}
	// (a) Lupinflower Village's {W}: a creature spell, not a noncreature one
	sc, id := restrSourceScen(t, "Lupinflower Village", "Mountain", "Savannah Lions", "Pacifism", "Grizzly Bears")
	sc.perm(id["Lupinflower Village"], 0)
	sc.perm(id["Mountain"], 0)
	sc.perm(id["Grizzly Bears"], 1)
	lions, paci := sc.hand(id["Savannah Lions"], 0), sc.hand(id["Pacifism"], 0)
	d := sc.s.Step()
	if !castOffered(&d, lions) || castOffered(&d, paci) {
		t.Fatalf("(a) Savannah Lions offered %v, Pacifism %v; want the creature on the Village's {W} only", castOffered(&d, lions), castOffered(&d, paci))
	}
	sc.s.Apply(MakeAction(AkCast, TCode(lions), 0, 0))
	sc.s.Step()
	if sc.s.sp != 1 || sc.s.rbAny(0) || sc.s.pl[0].pool != ([kop.NumManaKinds]uint8{}) {
		t.Fatalf("(a) after the Lions: stack %d restricted %v pool %v, want the Lions on the stack and nothing floating", sc.s.sp, sc.s.rb[0], sc.s.pl[0].pool)
	}

	// (b) Giada's {W}: an Angel spell, not a non-Angel
	sc, id = restrSourceScen(t, "Giada, Font of Hope", "Forest", "Dazzling Angel", "Savannah Lions")
	sc.perm(id["Giada, Font of Hope"], 0)
	sc.perm(id["Forest"], 0)
	sc.perm(id["Forest"], 0)
	angel, lions := sc.hand(id["Dazzling Angel"], 0), sc.hand(id["Savannah Lions"], 0)
	d = sc.s.Step()
	if !castOffered(&d, angel) || castOffered(&d, lions) {
		t.Fatalf("(b) Dazzling Angel offered %v, Savannah Lions %v; want the Angel on Giada's {W} only", castOffered(&d, angel), castOffered(&d, lions))
	}

	// (c) Secluded Courtyard: any colour for a creature of its type, and for
	// an activated ability of a creature of that type; {C} otherwise
	for _, typ := range []string{"Goblin", "Cat"} {
		sc, id = restrSourceScen(t, "Secluded Courtyard", "Mogg Fanatic", "Savannah Lions")
		court := sc.perm(id["Secluded Courtyard"], 0)
		restrSourceChose(t, sc, court, typ)
		fan := sc.hand(id["Mogg Fanatic"], 0)
		d = sc.s.Step()
		if castOffered(&d, fan) != (typ == "Goblin") {
			t.Fatalf("(c) Mogg Fanatic offered %v with %s chosen", castOffered(&d, fan), typ)
		}
	}
	for _, typ := range []string{"Dragon", "Goblin"} {
		sc, id = restrSourceScen(t, "Secluded Courtyard", "Furnace Whelp", "Mogg Fanatic")
		court := sc.perm(id["Secluded Courtyard"], 0)
		restrSourceChose(t, sc, court, typ)
		whelp := sc.perm(id["Furnace Whelp"], 0)
		d = sc.s.Step()
		if actOffered(&d, whelp) != (typ == "Dragon") {
			t.Fatalf("(c) Furnace Whelp's {R} activation offered %v with %s chosen", actOffered(&d, whelp), typ)
		}
	}

	// (d) a Powerstone: an artifact spell, an activated ability and any
	// non-spell payment (echo, cumulative upkeep, a triggered cost), never
	// a nonartifact spell
	sc, id = restrSourceScen(t, "Swamp", "Wayfarer's Bauble", "Walking Corpse", "Pestilent Cauldron", "Thran Spider")
	sc.perm(id["Powerstone Token"], 0)
	sc.perm(id["Swamp"], 0)
	bauble, corpse := sc.hand(id["Wayfarer's Bauble"], 0), sc.hand(id["Walking Corpse"], 0)
	caul := sc.perm(id["Pestilent Cauldron"], 0)
	d = sc.s.Step()
	if !castOffered(&d, bauble) || castOffered(&d, corpse) || !actOffered(&d, caul) {
		t.Fatalf("(d) Bauble offered %v, Walking Corpse %v, Cauldron's activation %v; want the artifact and the activation, not the Corpse",
			castOffered(&d, bauble), castOffered(&d, corpse), actOffered(&d, caul))
	}
	k := sc.k
	var ps Slot
	for m := sc.s.ctrlM[0]; m.any(); m = m.rest() {
		if sc.s.cardAt(m.first()) == id["Powerstone Token"] {
			ps = m.first()
		}
	}
	ab := &k.t.Cards[sc.s.cardAt(ps)].Abilities[k.gen.restrRecOf(sc.s.cardAt(ps)).ab]
	for _, c := range []struct {
		d    payDesc
		want bool
	}{{payDesc{class: pdSpell, card: id["Wayfarer's Bauble"], src: NoSlot}, true}, {payDesc{class: pdSpell, card: id["Walking Corpse"], src: NoSlot}, false},
		{payDesc{class: pdAct, card: id["Pestilent Cauldron"], src: NoSlot}, true}, {payDesc{class: pdOther, src: NoSlot}, true}} {
		if got := sc.s.restrSrcAdmits(k, 0, ps, ab, &c.d); got != c.want {
			t.Errorf("(d) Powerstone admits payment class %d (card %d): %v, want %v", c.d.class, c.d.card, got, c.want)
		}
	}

	// (e) Helga's X mana of one colour: a creature spell with mana value 4+
	// or {X}, not a cheaper one; what the payment leaves floats restricted
	sc, id = restrSourceScen(t, "Helga, Skittish Seer", "Mountain", "Walking Ballista", "Grizzly Bears", "Hill Giant")
	helga := sc.perm(id["Helga, Skittish Seer"], 0)
	sc.perm(id["Mountain"], 0)
	sc.s.perms[sc.slot(helga)].addCtrN(sc.s, int(kop.CtrP1P1), 2) // power 3: three mana of one colour
	ball, bears, giant := sc.hand(id["Walking Ballista"], 0), sc.hand(id["Grizzly Bears"], 0), sc.hand(id["Hill Giant"], 0)
	d = sc.s.Step()
	if !castOffered(&d, ball) || !castOffered(&d, giant) || castOffered(&d, bears) {
		t.Fatalf("(e) Walking Ballista offered %v, Hill Giant %v, Grizzly Bears %v; want the X creature and the mana value 4, not the 2",
			castOffered(&d, ball), castOffered(&d, giant), castOffered(&d, bears))
	}
	sc.s.Apply(MakeAction(AkCast, TCode(ball), 0, 0))
	d = sc.s.Step()
	if d.Kind != DkChoose || d.Sub != ChX {
		t.Fatalf("(e) the Ballista's X ask: kind %d sub %d", d.Kind, d.Sub)
	}
	x1 := MakeAction(AkChoose, 0, 0, 1)
	if !hasOption(&d, x1) || d.N != 3 { // Helga's 3 and the Mountain: X = 0..2
		t.Fatalf("(e) X ask options %v, want 0..2", d.Options[:d.N])
	}
	sc.s.Apply(x1)
	sc.s.Step()
	if sc.s.sp != 1 || sc.s.restrN(0) != 1 || sc.s.pl[0].pool != ([kop.NumManaKinds]uint8{}) {
		t.Fatalf("(e) after the Ballista (X = 1): stack %d restricted %v pool %v, want one restricted mana floating", sc.s.sp, sc.s.rb[0], sc.s.pl[0].pool)
	}
}

// TestRestrictedSourceAdmissionMutation reruns TestRestrictedSourceAdmission in a
// child process under each admission mutation (DebugRestrictedSource 1: every
// restricted source admitted for every payment; 2: none): the child must
// fail.
func TestRestrictedSourceAdmissionMutation(t *testing.T) {
	if os.Getenv("KF24_MUT") != "" {
		t.Skip("the child")
	}
	cardsettest.Require(t, "restrictedsource") // a child that skips would read as a mutation not caught
	for _, m := range []string{"1", "2"} {
		cmd := exec.Command(os.Args[0], "-test.run", "^TestRestrictedSourceAdmission$")
		cmd.Env = append(os.Environ(), "KF24_MUT="+m)
		out, err := cmd.CombinedOutput()
		if err == nil {
			t.Errorf("mutation %s: TestRestrictedSourceAdmission passed:\n%s", m, out)
			continue
		}
		t.Logf("mutation %s fails as it must: %s", m, firstFail(out))
	}
}

// firstFail is the first failure line of a test run's output.
func firstFail(out []byte) string {
	for _, l := range strings.Split(string(out), "\n") {
		if strings.Contains(l, "mana_restrictedsource_test.go:") {
			return strings.TrimSpace(l)
		}
	}
	return "(no failure line)"
}

// actModeOffered reports whether d offers activation mode of permanent o.
func actModeOffered(d *Decision, o Obj, mode uint8) bool {
	for _, a := range d.Options[:d.N] {
		if a.Kind() == AkActivate && a.Source() == TCode(uint8(o)) && a.Mode() == mode {
			return true
		}
	}
	return false
}

// TestRestrictedAnyOneColourSourcePaysOneColour (CR 106.1a, 605.3b):
// Shang-Chi's restricted "{T}: Add two mana of any one color" makes both
// mana one chosen colour, not two independent any-colour mana. {2}{R}{R}{G}{G}
// therefore needs one colour twice from Shang-Chi and the other from two
// lands: with one Forest, one Mountain and two Islands it is unpayable, and a
// second Forest pays it.
func TestRestrictedAnyOneColourSourcePaysOneColour(t *testing.T) {
	sc, id := stdMshScen(t, "Bruce Banner", "Shang-Chi, Master of Kung Fu", "Forest", "Mountain", "Island")
	bruce := sc.perm(id["Bruce Banner"], 0)
	shang := sc.perm(id["Shang-Chi, Master of Kung Fu"], 0)
	sc.perm(id["Forest"], 0)
	sc.perm(id["Mountain"], 0)
	sc.perm(id["Island"], 0)
	sc.perm(id["Island"], 0)
	if z := sc.s.objs[bruce].zone; z != kop.ZoneBattlefield {
		t.Fatalf("Bruce Banner is in zone %v, not the battlefield", z)
	}
	if sc.s.perms[sc.slot(shang)].flags&pfTapped != 0 {
		t.Fatal("Shang-Chi is tapped; its mana ability is not available")
	}
	d := sc.s.Step()
	if actModeOffered(&d, bruce, 1) {
		t.Fatal("Bruce Banner's transform offered by Shang-Chi (RR or GG), Forest, Mountain, Island, Island; want it withheld")
	}

	// Control: the second Forest pays GG beside Shang-Chi's RR, and the
	// Mountain and one Island pay the {2}.
	sc, id = stdMshScen(t, "Bruce Banner", "Shang-Chi, Master of Kung Fu", "Forest", "Mountain", "Island")
	bruce = sc.perm(id["Bruce Banner"], 0)
	sc.perm(id["Shang-Chi, Master of Kung Fu"], 0)
	sc.perm(id["Forest"], 0)
	sc.perm(id["Forest"], 0)
	sc.perm(id["Mountain"], 0)
	sc.perm(id["Island"], 0)
	d = sc.s.Step()
	if !actModeOffered(&d, bruce, 1) {
		t.Fatal("Bruce Banner's transform withheld by Shang-Chi RR, two Forests GG, Mountain and Island; want it offered")
	}
}
