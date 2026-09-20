package cards

import (
	"reflect"
	"testing"
)

func TestPrimitivesCoverEveryLineKind(t *testing.T) {
	src := `Name:Kitchen Sink
ManaCost:2 U
Types:Creature Bird
PT:1/1
K:Flying
T:Mode$ ChangesZone | Destination$ Battlefield | ValidCard$ Card.Self | Execute$ TrigDraw
SVar:TrigDraw:DB$ Draw | NumCards$ 1 | SubAbility$ DBLife
SVar:DBLife:DB$ GainLife | LifeAmount$ 1
S:Mode$ Continuous | Affected$ Creature.Other | AddPower$ 1
R:Event$ Moved | Destination$ Graveyard | ValidCard$ Card.Self | ReplaceWith$ RepX
SVar:RepX:DB$ ChangeZone | Origin$ Graveyard | Destination$ Exile
A:AB$ Tap | Cost$ T | ValidTgts$ Creature
Oracle:x
`
	c, _ := ParseBytes("k/kitchen_sink.txt", []byte(src))
	c.Link()
	got := c.Primitives()
	want := []string{
		"api:ChangeZone", "api:Draw", "api:GainLife", "api:Tap",
		"kw:Flying", "repl:Moved", "stat:Continuous", "trig:ChangesZone",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Primitives()\n got %v\nwant %v", got, want)
	}
}

func TestPrimitivesAreSortedAndDeduplicated(t *testing.T) {
	src := "Name:D\nTypes:Sorcery\nA:SP$ Draw | NumCards$ 1 | SubAbility$ X\nSVar:X:DB$ Draw | NumCards$ 1\nOracle:x\n"
	c, _ := ParseBytes("d.txt", []byte(src))
	c.Link()
	if got := c.Primitives(); !reflect.DeepEqual(got, []string{"api:Draw"}) {
		t.Fatalf("got %v", got)
	}
}

// TestPrimitivesFollowSVarNamingParams pins the reachability of a primitive
// that only an SVar-naming PARAMETER names, never a SubAbility$ chain: the
// Torment of Hailfire shape (Repeat's RepeatSubAbility$) and the Vision,
// Synthezoid Avenger shape (Charm's Choices$). A Sub-only walk reports
// neither, so the card reads as fully supported while half its behaviour is
// an unregistered API. The probe API names are arbitrary: Primitives does
// not know what is registered.
func TestPrimitivesFollowSVarNamingParams(t *testing.T) {
	src := `Name:PrimFixtures
Types:Sorcery
A:SP$ Repeat | RepeatNum$ 2 | RepeatSubAbility$ DBRepeatBody
A:SP$ Charm | CharmNum$ 1 | Choices$ DBPutCounter,DBPhaseOut
SVar:DBRepeatBody:DB$ GenericChoice | Choices$ DBSac
SVar:DBSac:DB$ LoseLife | LifeAmount$ 3
SVar:DBPutCounter:DB$ PutCounter | CounterType$ P1P1
SVar:DBPhaseOut:DB$ CustomUnsupportedProbe
SVar:CountOnly:Count$Valid Creature.YouCtrl
Oracle:x
`
	c, _ := ParseBytes("p/prim_fixtures.txt", []byte(src))
	c.Link()
	got := c.Primitives()
	want := []string{
		"api:Charm", "api:CustomUnsupportedProbe", "api:GenericChoice",
		"api:LoseLife", "api:PutCounter", "api:Repeat",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Primitives()\n got %v\nwant %v", got, want)
	}
}
