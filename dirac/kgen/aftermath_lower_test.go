package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestAftermathLayout (CR 702.127a): a split card whose right half carries
// aftermath lowers as kop.FaceSplit, the right half's face entry marked
// Aftermath and the front not (the kernel casts that half only from the
// graveyard, dirac/face_aftermath.go). Dusk // Dawn is the plain shape and
// Start // Finish the half with an additional sacrifice cost. Commit //
// Memory's layout is accepted too: what refuses it is its left half's own
// spell-or-permanent tuck, never the multi-face gate.
func TestAftermathLayout(t *testing.T) {
	names := []string{"Dusk", "Start", "Commit"}
	lw := lowerNamedPool(t, names)
	fronts := map[string]*lCard{}
	for _, c := range lw.cards {
		fronts[c.Name] = c
	}
	faces := map[string]*lCard{}
	for _, c := range lw.tokens {
		faces[c.Name] = c
	}
	for _, name := range []string{"Dusk", "Start"} {
		c := fronts[name]
		if c == nil {
			t.Fatalf("%s: not lowered", name)
		}
		if len(c.Refusals) != 0 {
			t.Errorf("%s refused: %v", name, c.Refusals)
		}
		if c.FaceMode != kop.FaceSplit || len(c.Faces) != 2 || c.Faces[1] != faceKey(name, 1) || c.Aftermath {
			t.Errorf("%s: layout %d faces %v front aftermath %v", name, c.FaceMode, c.Faces, c.Aftermath)
		}
		f := faces[faceKey(name, 1)]
		if f == nil || len(f.Refusals) != 0 || !f.Aftermath || f.Face != 1 || f.FaceMode != kop.FaceSplit {
			t.Errorf("%s: aftermath face entry %+v", name, f)
		}
	}
	commit := fronts["Commit"]
	if commit == nil {
		t.Fatal("Commit: not lowered")
	}
	var got []string
	for _, r := range commit.Refusals {
		got = append(got, r.Reason)
	}
	all := strings.Join(got, " | ")
	if len(got) == 0 || strings.Contains(all, "multi-face card") || !strings.Contains(all, "ChangeZone") {
		t.Errorf("Commit: refusals %q, want its own ChangeZone and no multi-face refusal", all)
	}
	if f := faces[faceKey("Commit", 1)]; f == nil || !f.Aftermath || len(f.Refusals) != 0 {
		t.Errorf("Commit: Memory's face entry %+v", f)
	}
}

// TestAftermathLeftHalfRefuses: aftermath printed on a split card's left
// half is no shape gorge reads (its aftermathAlternateFace looks at the
// right half only), so the card stays refused at the layout. The probe
// card is invented: no line repeats a Forge script.
func TestAftermathLeftHalfRefuses(t *testing.T) {
	src := strings.Join([]string{
		"Name:Probe Leftward",
		"ManaCost:1 W",
		"Types:Sorcery",
		"K:Aftermath",
		"A:SP$ GainLife | LifeAmount$ 7 | SpellDescription$ The leftward probe gains seven.",
		"AlternateMode:Split",
		"Oracle:Leftward probe text.",
		"",
		"ALTERNATE",
		"",
		"Name:Probe Rightward",
		"ManaCost:1 B",
		"Types:Sorcery",
		"A:SP$ GainLife | LifeAmount$ 9 | SpellDescription$ The rightward probe gains nine.",
		"Oracle:Rightward probe text.",
	}, "\n")
	c := lowerScriptCard(t, "Probe Leftward", src)
	var got []string
	for _, r := range c.Refusals {
		got = append(got, r.Reason)
	}
	if !strings.Contains(strings.Join(got, " | "), "multi-face card") {
		t.Errorf("left-half aftermath: refusals %v, want the multi-face refusal", got)
	}
	// The same card with the keyword on its right half lowers: the gate
	// above is the half the keyword sits on, nothing else.
	moved := strings.Replace(strings.Replace(src, "K:Aftermath\n", "", 1), "Types:Sorcery\nA:SP$ GainLife | LifeAmount$ 9", "Types:Sorcery\nK:Aftermath\nA:SP$ GainLife | LifeAmount$ 9", 1)
	if moved == src || strings.Count(moved, "K:Aftermath") != 1 {
		t.Fatal("probe rewrite did not move the keyword")
	}
	if r := lowerScriptCard(t, "Probe Leftward", moved); len(r.Refusals) != 0 || r.FaceMode != kop.FaceSplit {
		t.Errorf("right-half aftermath: refusals %v layout %d", r.Refusals, r.FaceMode)
	}
}
