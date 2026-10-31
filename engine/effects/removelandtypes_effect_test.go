package effects

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// RemoveLandTypes$ on an Effect-delivered / StaticEffect$ Continuous body
// (Orcish Farmer, Cyclopean Tomb, Ultima, Origin of Oblivion) must reach
// state.ContinuousEffect.RemoveLandTypes, the same field the printed-static
// scanner (engine/rules/layers.go) sets, instead of falling to the unimplemented Note.

func TestEffectDeliveredRemoveLandTypesRegisters(t *testing.T) {
	h, c := fixtureHost(t)
	c.SVars = map[string]string{
		"Animate": "Mode$ Continuous | Affected$ Card.IsRemembered | AddType$ Swamp | RemoveLandTypes$ True | Description$ Fixture.",
	}
	Resolve(h, c, sa(t, "DB$ Effect | StaticAbilities$ Animate | Duration$ Permanent"))
	var got *state.ContinuousEffect
	for i := range h.continuous {
		if h.continuous[i].Layer == state.LType {
			got = &h.continuous[i]
		}
	}
	if got == nil {
		t.Fatalf("continuous = %+v, want a layer-4 type effect", h.continuous)
	}
	if !got.RemoveLandTypes || len(got.AddTypes) != 1 || got.AddTypes[0] != "Swamp" {
		t.Fatalf("type effect = %+v, want RemoveLandTypes + AddTypes [Swamp]", *got)
	}
	for _, n := range notesOf(h) {
		if strings.Contains(n, "unimplemented") || strings.Contains(n, "not implemented") {
			t.Fatalf("unexpected unimplemented Note: %q", n)
		}
	}
}

// The artifact and enchantment subtype strips reach the layer-4 effect through
// an Effect-delivered body too.
func TestEffectDeliveredRemoveSubtypeSetsRegister(t *testing.T) {
	h, c := fixtureHost(t)
	c.SVars = map[string]string{
		"Animate": "Mode$ Continuous | Affected$ Card.IsRemembered | AddType$ Artifact | RemoveArtifactTypes$ True | RemoveEnchantmentTypes$ True | Description$ Fixture.",
	}
	Resolve(h, c, sa(t, "DB$ Effect | StaticAbilities$ Animate | Duration$ Permanent"))
	var got *state.ContinuousEffect
	for i := range h.continuous {
		if h.continuous[i].Layer == state.LType {
			got = &h.continuous[i]
		}
	}
	if got == nil || !got.RemoveArtifactTypes || !got.RemoveEnchantmentTypes {
		t.Fatalf("type effect = %+v, want both subtype strips", got)
	}
	for _, n := range notesOf(h) {
		if strings.Contains(n, "unimplemented") || strings.Contains(n, "not implemented") {
			t.Fatalf("unexpected unimplemented Note: %q", n)
		}
	}
}

// TestRemoveLandTypesAloneStillEmits: a strip-only body (Ultima's blight
// static) must emit the layer-4 effect without an AddType$.
func TestRemoveLandTypesAloneStillEmits(t *testing.T) {
	g, affects, ok := parseStaticEffectGrant(map[string]string{
		"Mode": "Continuous", "Affected": "Card.IsRemembered", "RemoveLandTypes": "True",
	}, false)
	if !ok || !g.removeLandTypes || affects != "Card.IsRemembered" {
		t.Fatalf("grant = %+v affects %q ok %v, want removeLandTypes", g, affects, ok)
	}
	if !effectStaticGrantReadableKeys.Has("RemoveLandTypes") {
		t.Fatal("RemoveLandTypes missing from effectStaticGrantReadableKeys")
	}
}

// TestRemoveLandTypesCensus is the corpus ratchet: every Mode$ Continuous
// SVar body naming RemoveLandTypes$ True must parse with removeLandTypes set,
// so a card cannot silently regress to the unread-key Note.
func TestRemoveLandTypesCensus(t *testing.T) {
	reg, err := cards.SharedCorpus(filepath.Join("..", "..", ".cards"))
	if err != nil {
		t.Skipf("corpus unavailable: %v", err)
	}
	const wantAtLeast = 3 // Orcish Farmer, Cyclopean Tomb, Ultima
	n := 0
	for _, c := range reg.AllCards() {
		for _, f := range c.Faces {
			for name, body := range f.SVars {
				if !strings.Contains(body, "RemoveLandTypes$") || !strings.Contains(body, "Mode$ Continuous") {
					continue
				}
				params := map[string]string{}
				for seg := range strings.SplitSeq(body, "|") {
					if k, v, ok := strings.Cut(seg, "$"); ok {
						params[strings.TrimSpace(k)] = strings.TrimSpace(v)
					}
				}
				g, _, ok := parseStaticEffectGrant(params, false)
				if !ok || !g.removeLandTypes {
					t.Errorf("%s SVar %s: RemoveLandTypes$ not carried (%+v)", f.Name, name, g)
				}
				n++
			}
		}
	}
	if n < wantAtLeast {
		t.Fatalf("census found %d bodies, want >= %d (corpus missing?)", n, wantAtLeast)
	}
	t.Logf("RemoveLandTypes$ Continuous SVar bodies: %d", n)
}
