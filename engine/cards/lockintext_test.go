package cards

import (
	"strings"
	"testing"
)

// allSunsDawnLockInTextSrc is All Suns' Dawn's printed expansion (the five
// per-colour ChangeZone links, each marked LockInText$ True) plus two
// unlocked siblings so the test can prove the lock is per ability, per SVar
// body and per static rather than a blanket no-op.
const allSunsDawnLockInTextSrc = "Name:All Suns' Dawn\n" +
	"ManaCost:4 G\n" +
	"Types:Sorcery\n" +
	"A:SP$ ChangeZone | Origin$ Graveyard | Destination$ Hand | ValidTgts$ Card.White+YouOwn | TargetMin$ 0 | TargetMax$ 1 | LockInText$ True | SubAbility$ ReturnU\n" +
	"A:SP$ Pump | ValidTgts$ Creature.White+YouOwn | NumAtt$ +1\n" +
	"SVar:ReturnU:DB$ ChangeZone | Origin$ Graveyard | Destination$ Hand | ValidTgts$ Card.Blue+YouOwn | TargetMin$ 0 | TargetMax$ 1 | LockInText$ True | SubAbility$ ReturnB\n" +
	"SVar:ReturnB:DB$ ChangeZone | Origin$ Graveyard | Destination$ Hand | ValidTgts$ Card.Black+YouOwn | TargetMin$ 0 | TargetMax$ 1 | LockInText$ True | SubAbility$ ReturnW\n" +
	"SVar:ReturnW:DB$ ChangeZone | Origin$ Graveyard | Destination$ Hand | ValidTgts$ Card.White+YouOwn | TargetMin$ 0 | TargetMax$ 1 | LockInText$ True\n" +
	"SVar:UnlockedPump:DB$ Pump | ValidTgts$ Creature.White+YouOwn | NumAtt$ +1\n" +
	"S:Mode$ Continuous | Affected$ Creature.White | LockInText$ True\n" +
	"S:Mode$ Continuous | Affected$ Creature.White\n" +
	"Oracle:For each color, return up to one target card of that color from your graveyard to your hand.\n"

// TestWithTextChangesSkipsLockInTextAbility pins Forge's per-ability
// LockInText$ lock (AbilityUtils.applyAbilityTextChangeEffects): a text change
// that replaces White with Blue leaves every LockInText$ ability, SVar body
// and static verbatim, while an unlocked sibling on the same face is still
// rewritten. All Suns' Dawn's per-colour filter chain is the real carrier.
func TestWithTextChangesSkipsLockInTextAbility(t *testing.T) {
	t.Parallel()
	c, diags := ParseBytes("all_suns_dawn.txt", []byte(allSunsDawnLockInTextSrc))
	if len(diags) != 0 {
		t.Fatalf("parse diagnostics: %v", diags)
	}
	if d := c.Link(); len(d) != 0 {
		t.Fatalf("link diagnostics: %v", d)
	}
	f := c.Faces[0]
	if len(f.Abilities) != 2 {
		t.Fatalf("precondition: fixture has %d abilities, want 2", len(f.Abilities))
	}
	if f.Abilities[0].Params["LockInText"] != "True" || f.Abilities[0].Sub == nil {
		t.Fatalf("precondition: first ability must carry LockInText$ and a linked sub: %+v", f.Abilities[0])
	}
	if _, locked := f.Abilities[1].Params["LockInText"]; locked {
		t.Fatal("precondition: sibling ability must not carry LockInText$")
	}

	got := f.WithTextChanges([]TextSub{{From: "White", To: "Blue"}})
	if got == f {
		t.Fatal("WithTextChanges returned the original face")
	}

	// The locked inline ability is verbatim, including its chained links
	// that each carry their own lock.
	if v := got.Abilities[0].Params["ValidTgts"]; v != "Card.White+YouOwn" {
		t.Errorf("LockInText$ ability ValidTgts = %q, want unchanged %q", v, "Card.White+YouOwn")
	}
	if got.Abilities[0].Line != f.Abilities[0].Line {
		t.Errorf("LockInText$ ability Line = %q, want unchanged %q", got.Abilities[0].Line, f.Abilities[0].Line)
	}
	if sub := got.Abilities[0].Sub; sub == nil || sub.Params["ValidTgts"] != "Card.Blue+YouOwn" {
		t.Errorf("locked link's blue sub = %+v, want unchanged Card.Blue+YouOwn", sub)
	} else if subSub := sub.Sub; subSub == nil || subSub.Params["ValidTgts"] != "Card.Black+YouOwn" {
		t.Errorf("locked link's black sub = %+v, want unchanged Card.Black+YouOwn", subSub)
	}

	// An unlocked sibling ability on the same face is still rewritten: the
	// lock is per ability, not a face-wide short circuit.
	if v := got.Abilities[1].Params["ValidTgts"]; v != "Creature.Blue+YouOwn" {
		t.Errorf("unlocked sibling ValidTgts = %q, want rewritten %q", v, "Creature.Blue+YouOwn")
	}

	// The raw SVar bodies are locked too, so a later compile cannot read the
	// changed word; an unlocked body still changes.
	if v := got.SVars["ReturnW"]; !strings.Contains(v, "Card.White+YouOwn") {
		t.Errorf("LockInText$ SVar body = %q, want the verbatim Card.White+YouOwn", v)
	}
	if v := got.SVars["UnlockedPump"]; !strings.Contains(v, "Creature.Blue+YouOwn") {
		t.Errorf("unlocked SVar body = %q, want rewritten Creature.Blue+YouOwn", v)
	}

	// The static line carries the same lock (Mirrodin Besieged's Continuous
	// static is the corpus carrier).
	if len(got.Statics) != 2 {
		t.Fatalf("precondition: fixture has %d statics, want 2", len(got.Statics))
	}
	if v := got.Statics[0].Params["Affected"]; v != "Creature.White" {
		t.Errorf("LockInText$ static Affected = %q, want unchanged %q", v, "Creature.White")
	}
	if v := got.Statics[1].Params["Affected"]; v != "Creature.Blue" {
		t.Errorf("unlocked static Affected = %q, want rewritten %q", v, "Creature.Blue")
	}
}
