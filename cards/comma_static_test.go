package cards

// Comma-separated `S:Mode$ A,B` statics: one structural home at parse time.
//
// The corpus prints compound statics as one S: line naming several modes over
// the same parameters — Pacifism's "S:Mode$ CantAttack,CantBlock | ValidCard$
// Creature.EnchantedBy" is the canonical carrier (58 raw corpus lines of the
// exact CantAttack,CantBlock shape, 87 comma-mode rows overall). Every
// Static.Mode consumer compares the field against ONE literal mode
// (primitive.go's "stat:" token, rules' activeStatics/staticEffects,
// face.go's CDA reads, compiled_catalog.go's staticModeCode), so the raw
// comma list was a single opaque name: the card measured unsupported AND the
// static was dead at runtime. The fix splits at the two construction points —
// parse.go's printed-S: case and ParseStaticLines for the SVar-bodied route —
// so every downstream consumer sees a single mode for free.
//
// These tests pin the split on the Primitives()/Unsupported surface and on
// the face's own Static list.

import (
	"reflect"
	"strings"
	"testing"
)

const commaModeFixture = `Name:Pacifier
ManaCost:1 W
Types:Enchantment Aura
S:Mode$ CantAttack,CantBlock | ValidCard$ Card.Self
Oracle:x
`

// TestPrimitivesSplitCommaModeList: a compound S: line yields BOTH
// stat:CantAttack and stat:CantBlock in Primitives(), and never the combined
// "stat:CantAttack,CantBlock" token.
func TestPrimitivesSplitCommaModeList(t *testing.T) {
	c, _ := ParseBytes("p/pacifier.txt", []byte(commaModeFixture))
	c.Link()
	got := c.Primitives()
	want := []string{"stat:CantAttack", "stat:CantBlock"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Primitives()\n got %v\nwant %v", got, want)
	}
}

// TestFaceStaticsSplitShareParams: the face carries one Static per mode, in
// line order, all sharing the same Params map — which still holds the full
// Mode$ text (no static consumer reads Params["Mode"], but nothing may lose
// it either).
func TestFaceStaticsSplitShareParams(t *testing.T) {
	c, _ := ParseBytes("p/pacifier.txt", []byte(commaModeFixture))
	f := c.Faces[0]
	if len(f.Statics) != 2 {
		t.Fatalf("face has %d statics, want one per mode:\n%+v", len(f.Statics), f.Statics)
	}
	if f.Statics[0].Mode != "CantAttack" || f.Statics[1].Mode != "CantBlock" {
		t.Fatalf("static modes = %q, %q; want CantAttack, CantBlock", f.Statics[0].Mode, f.Statics[1].Mode)
	}
	for i, st := range f.Statics {
		if st.Params["ValidCard"] != "Card.Self" {
			t.Fatalf("static %d lost the shared params: %+v", i, st.Params)
		}
		if st.Params["Mode"] != "CantAttack,CantBlock" {
			t.Fatalf("static %d Params[Mode] = %q, want the full original text", i, st.Params["Mode"])
		}
	}
}

// TestUnsupportedNeverReportsACommaToken: Registry.Unsupported names single
// modes, never the combined token — with both halves in the support map the
// card is fully supported, and without them the missing entries are the two
// split tokens.
func TestUnsupportedNeverReportsACommaToken(t *testing.T) {
	c, _ := ParseBytes("p/pacifier.txt", []byte(commaModeFixture))
	c.Link()
	r := NewRegistry()
	r.Add(c)
	supported := map[string]bool{"stat:CantAttack": true, "stat:CantBlock": true}
	if miss := r.Unsupported(c, supported); len(miss) != 0 {
		t.Fatalf("Unsupported with both halves supported = %v, want empty", miss)
	}
	miss := r.Unsupported(c, map[string]bool{"stat:CantAttack": true})
	if len(miss) != 1 || miss[0] != "stat:CantBlock" {
		t.Fatalf("Unsupported = %v, want exactly [stat:CantBlock]", miss)
	}
	for _, m := range miss {
		if strings.Contains(m, ",") {
			t.Fatalf("Unsupported reports a comma token: %v", miss)
		}
	}
}

// TestCompoundLineWithRegisteredAndUnregisteredHalves: a line mixing a
// registered mode with an unregistered one (CantTransform is not registered
// here) reports ONLY the missing half, never the comma token.
func TestCompoundLineWithRegisteredAndUnregisteredHalves(t *testing.T) {
	src := "Name:MT\nTypes:Creature Shapeshifter\nPT:2/2\nS:Mode$ CantAttack,CantBlock,CantTransform | ValidCard$ Card.Self\nOracle:x\n"
	c, _ := ParseBytes("m/mt.txt", []byte(src))
	c.Link()
	got := c.Primitives()
	want := []string{"stat:CantAttack", "stat:CantBlock", "stat:CantTransform"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Primitives()\n got %v\nwant %v", got, want)
	}
	r := NewRegistry()
	r.Add(c)
	miss := r.Unsupported(c, map[string]bool{"stat:CantAttack": true, "stat:CantBlock": true})
	if len(miss) != 1 || miss[0] != "stat:CantTransform" {
		t.Fatalf("Unsupported = %v, want exactly [stat:CantTransform]", miss)
	}
}

// TestParseStaticLinesSplitsSVarBody: the SVar-bodied static route gets the
// same split (ParseStaticLines), and ParseStaticLine itself stays
// behaviour-identical — single entry, whole raw mode text.
func TestParseStaticLinesSplitsSVarBody(t *testing.T) {
	body := "Mode$ CantAttack,CantBlock | ValidCard$ Card.Self"
	inners, ok := ParseStaticLines(body)
	if !ok {
		t.Fatal("ParseStaticLines refused a comma-mode body")
	}
	if len(inners) != 2 || inners[0].Mode != "CantAttack" || inners[1].Mode != "CantBlock" {
		t.Fatalf("ParseStaticLines = %+v, want CantAttack + CantBlock sharing params", inners)
	}
	if inners[0].Params["ValidCard"] != "Card.Self" {
		t.Fatalf("split statics lost the params: %+v", inners[0].Params)
	}
	single, ok := ParseStaticLine(body)
	if !ok || single.Mode != "CantAttack,CantBlock" {
		t.Fatalf("ParseStaticLine = %+v ok=%v; want the raw combined mode unchanged", single, ok)
	}
	if _, ok := ParseStaticLines("SP$ Draw | NumCards$ 1"); ok {
		t.Fatal("ParseStaticLines accepted a body with no Mode$")
	}
	if sts, ok := ParseStaticLines("Mode$ Continuous | Affected$ Card.Self"); !ok || len(sts) != 1 || sts[0].Mode != "Continuous" {
		t.Fatalf("ParseStaticLines on a single-mode body = %+v ok=%v, want one Continuous", sts, ok)
	}
}

// TestResplitRepairsADecodedFace: resplitStatics is the gob-decode route's
// repair — a face carrying a combined token static is resplit to one Static
// per mode sharing params, and an already-split list is returned unchanged
// (same backing slice, no allocation).
func TestResplitRepairsADecodedFace(t *testing.T) {
	params := map[string]string{"Mode": "CantAttack,CantBlock", "ValidCard": "Card.Self"}
	sts := []Static{{Mode: "CantAttack,CantBlock", Params: params}, {Mode: "Continuous", Params: nil}}
	out := resplitStatics(sts)
	if len(out) != 3 || out[0].Mode != "CantAttack" || out[1].Mode != "CantBlock" || out[2].Mode != "Continuous" {
		t.Fatalf("resplitStatics = %+v, want CantAttack, CantBlock, Continuous", out)
	}
	if out[0].Params["ValidCard"] != "Card.Self" || out[1].Params["ValidCard"] != "Card.Self" {
		t.Fatalf("resplit statics lost the shared params")
	}
	plain := []Static{{Mode: "CantBlock", Params: nil}}
	if got := resplitStatics(plain); &got[0] != &plain[0] {
		t.Fatal("a split-free list must pass through unchanged (same backing slice)")
	}
}
