package cards

import "testing"

const chainSrc = `Name:Chained
ManaCost:2 R
Types:Sorcery
A:SP$ DealDamage | ValidTgts$ Any | NumDmg$ 2 | SubAbility$ DBDraw
SVar:DBDraw:DB$ Draw | NumCards$ 1 | SubAbility$ DBLife
SVar:DBLife:DB$ GainLife | LifeAmount$ 2
Oracle:x
`

func TestLinkBuildsSubAbilityChain(t *testing.T) {
	c, diags := ParseBytes("c/chained.txt", []byte(chainSrc))
	diags = append(diags, c.Link()...)
	if len(diags) != 0 {
		t.Fatalf("diags = %v", diags)
	}
	a := c.Faces[0].Abilities[0]
	if a.API != "DealDamage" {
		t.Fatalf("head API = %s", a.API)
	}
	if a.Sub == nil || a.Sub.API != "Draw" {
		t.Fatalf("first sub = %+v", a.Sub)
	}
	if a.Sub.Sub == nil || a.Sub.Sub.API != "GainLife" {
		t.Fatalf("second sub = %+v", a.Sub.Sub)
	}
	if a.Sub.Sub.Sub != nil {
		t.Fatal("chain should terminate")
	}
}

const trigSrc = `Name:Trigger Card
ManaCost:1 W
Types:Creature Human
PT:1/1
T:Mode$ ChangesZone | Origin$ Any | Destination$ Battlefield | ValidCard$ Card.Self | Execute$ TrigGain
SVar:TrigGain:DB$ GainLife | LifeAmount$ 3
R:Event$ Moved | Destination$ Graveyard | ValidCard$ Card.Self | ReplaceWith$ RepExile
SVar:RepExile:DB$ ChangeZone | Origin$ Graveyard | Destination$ Exile | Defined$ Self
Oracle:x
`

func TestLinkResolvesTriggerAndReplacement(t *testing.T) {
	c, diags := ParseBytes("t/trigger_card.txt", []byte(trigSrc))
	diags = append(diags, c.Link()...)
	if len(diags) != 0 {
		t.Fatalf("diags = %v", diags)
	}
	f := c.Faces[0]
	if f.Triggers[0].Effect == nil || f.Triggers[0].Effect.API != "GainLife" {
		t.Fatalf("trigger effect = %+v", f.Triggers[0].Effect)
	}
	if f.Repls[0].With == nil || f.Repls[0].With.API != "ChangeZone" {
		t.Fatalf("replacement effect = %+v", f.Repls[0].With)
	}
}

func TestLinkReportsMissingSVarAndSurvivesCycles(t *testing.T) {
	missing := "Name:M\nTypes:Sorcery\nA:SP$ Draw | NumCards$ 1 | SubAbility$ Nope\nOracle:x\n"
	c, _ := ParseBytes("m.txt", []byte(missing))
	if d := c.Link(); len(d) != 1 {
		t.Fatalf("diags = %v, want 1 unresolved-SVar", d)
	}

	cyclic := "Name:C\nTypes:Sorcery\nA:SP$ Draw | SubAbility$ A\nSVar:A:DB$ Draw | SubAbility$ B\nSVar:B:DB$ Draw | SubAbility$ A\nOracle:x\n"
	c2, _ := ParseBytes("c.txt", []byte(cyclic))
	c2.Link()
	// Link should return without hanging; maxSVarDepth caps the walk.
}
