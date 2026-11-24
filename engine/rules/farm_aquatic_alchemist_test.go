package rules

import (
	"fmt"
	"testing"

	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestFarmAquaticAlchemist pins the WOE farm window (WOE-s334, FX|G, field
// stack) where gorge triggered Aquatic Alchemist ("Whenever you cast your
// first instant or sorcery spell each turn") on the turn's SECOND instant or
// sorcery, while Forge and XMage put only the spell. CR 715.3b: "While on the
// stack as an Adventure, the spell has only its alternative characteristics",
// so an Adventure cast earlier this turn (Bubble Up, a sorcery) was the first
// instant or sorcery spell, even after it resolved and the card sits in exile
// with its creature face (CR 715.4 describes the card, not the cast). CR
// 603.2: the trigger event is the cast as it was made.
func TestFarmAquaticAlchemist(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	sc := `{"name":"farm aquatic alchemist","turn":1,"cr":["715.3b","715.4","603.2"],"why":"inline",` +
		`"setup":{"p0":{"battlefield":["Aquatic Alchemist"],"hand":["Aquatic Alchemist","Shock"],` +
		`"graveyard":["Lightning Bolt"],"life":20},"p1":{"life":20}},"steps":[` +
		`{"op":"cast","seat":0,"card":"p0:Bubble Up","mana":"UUU","targets":["p0:Lightning Bolt"]},` +
		`{"op":"resolve"},` +
		`{"op":"cast","seat":0,"card":"p0:Shock","mana":"R","targets":["p1"]}]}`
	res, err := RunOracleScenarioJSON(reg, []byte(sc))
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if len(res.Fails) != 0 {
		t.Fatalf("scenario failed: %v", res.Fails)
	}
	stackAt := func(checkpoint string) []string {
		t.Helper()
		for i := range res.Snapshots {
			if res.Snapshots[i].Checkpoint == checkpoint {
				var got []string
				for _, s := range res.Snapshots[i].Stack {
					got = append(got, s.Kind+":"+s.Source)
				}
				return got
			}
		}
		t.Fatalf("no %q snapshot", checkpoint)
		return nil
	}
	// Precondition: the Adventure cast itself is an instant-or-sorcery cast
	// the Alchemist sees, so the second cast's count has a first to see.
	first := stackAt("step 0 (cast)")
	if len(first) != 2 || first[0] != "ability:p0:Aquatic Alchemist" {
		t.Fatalf("precondition: stack after the Bubble Up cast = %v, want the Alchemist trigger above the spell (top first)", first)
	}
	if got, want := stackAt("step 2 (cast)"), []string{"spell:p0:Shock"}; fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("stack after Shock = %v, want %v (Forge and XMage: Bubble Up was the first instant or sorcery)", got, want)
	}
}

// TestFarmAquaticAlchemistOmen is the class sibling: an Omen cast (CR 720.3,
// Claim Territory, a sorcery while on the stack) that resolved and was
// shuffled into its library as Bloomvine Regent (CR 720.4) was still the
// turn's first sorcery spell, so the Alchemist does not trigger on Shock.
func TestFarmAquaticAlchemistOmen(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	sc := `{"name":"farm aquatic alchemist omen","turn":1,"cr":["720.3","720.4","603.2"],"why":"inline",` +
		`"setup":{"p0":{"battlefield":["Aquatic Alchemist"],"hand":["Bloomvine Regent","Shock"],` +
		`"library":["Forest","Forest","Island"],"life":20},"p1":{"life":20}},"steps":[` +
		`{"op":"cast","seat":0,"card":"p0:Claim Territory","mana":"GGG"},` +
		`{"op":"resolve"},` +
		`{"op":"cast","seat":0,"card":"p0:Shock","mana":"R","targets":["p1"]}]}`
	res, err := RunOracleScenarioJSON(reg, []byte(sc))
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if len(res.Fails) != 0 {
		t.Fatalf("scenario failed: %v", res.Fails)
	}
	stackAt := func(checkpoint string) []string {
		t.Helper()
		for i := range res.Snapshots {
			if res.Snapshots[i].Checkpoint == checkpoint {
				var got []string
				for _, s := range res.Snapshots[i].Stack {
					got = append(got, s.Kind+":"+s.Source)
				}
				return got
			}
		}
		t.Fatalf("no %q snapshot", checkpoint)
		return nil
	}
	if first := stackAt("step 0 (cast)"); len(first) != 2 || first[0] != "ability:p0:Aquatic Alchemist" {
		t.Fatalf("precondition: stack after the Claim Territory cast = %v, want the Alchemist trigger above the spell (top first)", first)
	}
	if got, want := stackAt("step 2 (cast)"), []string{"spell:p0:Shock"}; fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("stack after Shock = %v, want %v (Claim Territory was the first sorcery)", got, want)
	}
}
