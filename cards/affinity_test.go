package cards

import "testing"

// The Affinity keyword expansion (CR 702.41a): K:Affinity:<spec> mints ONE
// self-scoped ReduceCost cost static, priced by a Count$Valid <spec>.YouCtrl
// SVar the ordinary cost-static machinery resolves. The fixtures embed the
// real corpus scripts' keyword lines (never a committed .txt).

// TestAffinityExpandsToSelfReduceCostStatic pins the minted static's exact
// shape on Frogmite's real keyword line: one Static, Mode$ ReduceCost,
// ValidCard$ Card.Self, Type$ Spell, EffectZone$ All, Amount$ the __kw
// SVar, and the SVar body counting the battlefield spec with the YouCtrl
// qualifier INSIDE it (Count$Valid's "Valid" head is battlefield-wide). The
// dot-less spec joins with '.'; a spec that already carries a dot would
// need '+', pinned by the Junk Winder test in rules/affinity_test.go.
func TestAffinityExpandsToSelfReduceCostStatic(t *testing.T) {
	f := expanded(t, "Name:Frogmite\nManaCost:4\nTypes:Artifact Creature Frog\nPT:2/2\nK:Affinity:Artifact\nOracle:x\n")
	if len(f.Statics) != 1 {
		t.Fatalf("want exactly one minted static, got %+v", f.Statics)
	}
	st := f.Statics[0]
	if st.Mode != "ReduceCost" {
		t.Fatalf("Mode = %q, want ReduceCost", st.Mode)
	}
	for key, want := range map[string]string{
		"ValidCard":  "Card.Self",
		"Type":       "Spell",
		"EffectZone": "All",
	} {
		if st.Params[key] != want {
			t.Fatalf("%s = %q, want %q (%+v)", key, st.Params[key], want, st.Params)
		}
	}
	amount := st.Params["Amount"]
	if len(amount) < len("__kwAffinity0") || amount[:len("__kwAffinity")] != "__kwAffinity" {
		t.Fatalf("Amount = %q, want an __kw SVar name", amount)
	}
	body, ok := f.SVars[amount]
	if !ok {
		t.Fatalf("SVar %q missing (have %v)", amount, f.SVars)
	}
	if body != "Count$Valid Artifact.YouCtrl" {
		t.Fatalf("SVar body = %q, want Count$Valid Artifact.YouCtrl", body)
	}
	if st.Params["KeywordLine"] != "Affinity:Artifact" {
		t.Fatalf("KeywordLine = %q, want the full keyword line", st.Params["KeywordLine"])
	}
	if _, has := st.Params["Color"]; has {
		t.Fatal("Color$ must not be set: affinity reduces generic only")
	}
	if _, has := st.Params["Relative"]; has {
		t.Fatal("Relative$ must not be set: the affinity amount is X-independent")
	}
}

// TestAffinityDropsTrailingDescription covers the 3 corpus lines whose
// keyword line carries a display field after a second colon
// ("K:Affinity:Land.Snow:snow land"): the spec is everything up to the
// SECOND colon, the tail is display text only.
func TestAffinityDropsTrailingDescription(t *testing.T) {
	f := expanded(t, "Name:Snowy\nManaCost:2\nTypes:Snow Creature\nPT:2/2\nK:Affinity:Land.Snow:snow land\nOracle:x\n")
	if len(f.Statics) != 1 {
		t.Fatalf("%+v", f.Statics)
	}
	amount := f.Statics[0].Params["Amount"]
	body, ok := f.SVars[amount]
	if !ok {
		t.Fatalf("SVar %q missing", amount)
	}
	if body != "Count$Valid Land.Snow+YouCtrl" {
		t.Fatalf("SVar body = %q, want Count$Valid Land.Snow+YouCtrl (no display text)", body)
	}
}

// TestAffinityExpansionIsIdempotent pins the registry re-link hazard: a
// second Link() (cards/registry.go re-runs f.link() on faces cached before
// a newly added expansion) must not append a second reduction static --
// that would double the discount, replay-visibly.
func TestAffinityExpansionIsIdempotent(t *testing.T) {
	c, _ := ParseBytes("k.txt", []byte("Name:C\nManaCost:4\nTypes:Artifact Creature\nPT:2/2\nK:Affinity:Artifact\nOracle:x\n"))
	c.Link()
	n := len(c.Faces[0].Statics)
	if n != 1 {
		t.Fatalf("first Link produced %d statics, want 1", n)
	}
	c.Link()
	if len(c.Faces[0].Statics) != n {
		t.Fatal("a second Link expanded K:Affinity again (double reduction)")
	}
}
