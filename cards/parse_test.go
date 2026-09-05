package cards

import "testing"

const boltSrc = `Name:Lightning Bolt
ManaCost:R
Types:Instant
A:SP$ DealDamage | ValidTgts$ Any | NumDmg$ 3 | SpellDescription$ CARDNAME deals 3 damage to any target.
Oracle:Lightning Bolt deals 3 damage to any target.
`

func TestParseSimpleSpell(t *testing.T) {
	c, diags := ParseBytes("l/lightning_bolt.txt", []byte(boltSrc))
	if len(diags) != 0 {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	if len(c.Faces) != 1 {
		t.Fatalf("faces = %d, want 1", len(c.Faces))
	}
	f := c.Faces[0]
	if f.Name != "Lightning Bolt" {
		t.Errorf("Name = %q", f.Name)
	}
	if f.ManaCost != "R" {
		t.Errorf("ManaCost = %q", f.ManaCost)
	}
	if len(f.Types) != 1 || f.Types[0] != "Instant" {
		t.Errorf("Types = %v", f.Types)
	}
	if len(f.Abilities) != 1 {
		t.Fatalf("Abilities = %d, want 1", len(f.Abilities))
	}
	a := f.Abilities[0]
	if a.Kind != "SP" || a.API != "DealDamage" {
		t.Errorf("ability head = %s$ %s", a.Kind, a.API)
	}
	if a.Params["NumDmg"] != "3" {
		t.Errorf("NumDmg = %q", a.Params["NumDmg"])
	}
	if a.Params["ValidTgts"] != "Any" {
		t.Errorf("ValidTgts = %q", a.Params["ValidTgts"])
	}
	// The head token must not survive as a parameter.
	if _, ok := a.Params["SP"]; ok {
		t.Error("SP leaked into Params")
	}
}

const dfcSrc = `Name:Front Face
ManaCost:1 U
Types:Creature Human
PT:1/1
K:Flying
T:Mode$ ChangesZone | Origin$ Any | Destination$ Battlefield | ValidCard$ Card.Self | Execute$ TrigDraw
SVar:TrigDraw:DB$ Draw | NumCards$ 1
Oracle:Flying
ALTERNATE
Name:Back Face
Types:Creature Horror
PT:3/2
S:Mode$ Continuous | Affected$ Creature.Other | AddPower$ 1
Oracle:Other creatures get +1/+0.
`

func TestParseMultiFaceAndLineKinds(t *testing.T) {
	c, diags := ParseBytes("f/front_face.txt", []byte(dfcSrc))
	if len(diags) != 0 {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	if len(c.Faces) != 2 {
		t.Fatalf("faces = %d, want 2", len(c.Faces))
	}
	front, back := c.Faces[0], c.Faces[1]
	if front.PT != "1/1" || back.PT != "3/2" {
		t.Errorf("PT front=%q back=%q", front.PT, back.PT)
	}
	if len(front.Keywords) != 1 || front.Keywords[0] != "Flying" {
		t.Errorf("front keywords = %v", front.Keywords)
	}
	if len(front.Triggers) != 1 || front.Triggers[0].Mode != "ChangesZone" {
		t.Errorf("front triggers = %v", front.Triggers)
	}
	if front.SVars["TrigDraw"] != "DB$ Draw | NumCards$ 1" {
		t.Errorf("SVar = %q", front.SVars["TrigDraw"])
	}
	if len(back.Statics) != 1 || back.Statics[0].Mode != "Continuous" {
		t.Errorf("back statics = %v", back.Statics)
	}
	if back.Statics[0].Params["AddPower"] != "1" {
		t.Errorf("AddPower = %q", back.Statics[0].Params["AddPower"])
	}
	// Faces do not share state.
	if len(back.Keywords) != 0 {
		t.Errorf("keywords leaked to back face: %v", back.Keywords)
	}
}

func TestParseIgnoresCommentsAndBlanks(t *testing.T) {
	src := "# leading comment\nName:X\n\nTypes:Land\n# trailing\nOracle:\n"
	c, diags := ParseBytes("x.txt", []byte(src))
	if len(diags) != 0 {
		t.Fatalf("diags = %v", diags)
	}
	if c.Faces[0].Name != "X" || len(c.Faces[0].Types) != 1 {
		t.Fatalf("face = %+v", c.Faces[0])
	}
}

func TestParseReportsUnkeyedLine(t *testing.T) {
	_, diags := ParseBytes("x.txt", []byte("Name:X\nthis line has no key\n"))
	if len(diags) != 1 {
		t.Fatalf("diags = %v, want 1", diags)
	}
}

// TestTriggerCarriesOptionalDecider pins the one field Task 27 depends on
// reaching cards.Trigger. Established from .cards/cardsfolder rather than
// guessed (Ruling T12-a's precedent): on a T: line, optionality is spelled
// OptionalDecider$ and only that -- 1496 T: lines carry it, and the bare
// Optional$ form -- 1143 occurrences by the anchored pattern
// (^|[^A-Za-z])Optional$, all on SVar:, A:, R: and S: lines -- never appears
// on one. The anchor matters: the unanchored substring counts 1199 because
// Forge also has RevealOptional$, ChoiceOptional$ and RepeatOptional$.
// parseParams already collects every Key$ value on the line, so this needs
// no parser change; the test is here so a future change to parseParams
// cannot silently drop it.
func TestTriggerCarriesOptionalDecider(t *testing.T) {
	src := "Name:X\nTypes:Enchantment\n" +
		"T:Mode$ Phase | Phase$ Upkeep | OptionalDecider$ TriggeredCardController | " +
		"Execute$ Trig | TriggerDescription$ you may do it\n" +
		"SVar:Trig:DB$ GainLife | LifeAmount$ 1\nOracle:x\n"
	c, d := ParseBytes("t.txt", []byte(src))
	if len(d) != 0 {
		t.Fatalf("diags: %v", d)
	}
	tr := c.Faces[0].Triggers
	if len(tr) != 1 {
		t.Fatalf("triggers = %d, want 1", len(tr))
	}
	if got := tr[0].Params["OptionalDecider"]; got != "TriggeredCardController" {
		t.Fatalf("OptionalDecider = %q, want TriggeredCardController", got)
	}
	// A trigger with no OptionalDecider$ must read as mandatory, not as an
	// empty-string decider.
	c2, _ := ParseBytes("t.txt", []byte("Name:Y\nTypes:Enchantment\n"+
		"T:Mode$ Phase | Phase$ Upkeep | Execute$ Trig\nSVar:Trig:DB$ GainLife\nOracle:x\n"))
	if _, ok := c2.Faces[0].Triggers[0].Params["OptionalDecider"]; ok {
		t.Fatal("a mandatory trigger reported an OptionalDecider")
	}
}
