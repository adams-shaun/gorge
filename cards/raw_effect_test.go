package cards

import (
	"reflect"
	"testing"
)

func TestPrimitivesFollowEffectSVarRawChildren(t *testing.T) {
	src := `Name:Raw Effect Fixture
Types:Enchantment
A:AB$ Effect | Triggers$ T MissingTrigger WrongStatic | StaticAbilities$ S WrongReplacement | ReplacementEffects$ R MissingReplacement WrongTrigger
SVar:T:Mode$ UnsupportedTrigger
SVar:S:Mode$ UnsupportedStaticA,UnsupportedStaticB
SVar:R:Event$ UnsupportedReplacement
SVar:WrongStatic:Event$ WrongEvent
SVar:WrongReplacement:Event$ WrongEvent
SVar:WrongTrigger:Mode$ WrongMode
Oracle:x
`
	c, _ := ParseBytes("raw_effect.txt", []byte(src))
	c.Link()
	got := c.Primitives()
	want := []string{"api:Effect", "repl:UnsupportedReplacement", "stat:UnsupportedStaticA", "stat:UnsupportedStaticB", "trig:UnsupportedTrigger"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Primitives() = %v, want %v", got, want)
	}
}
