package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestFaceLayouts (C5ab): a modal DFC with a land back face
// lowers its front as the card and its back as a linked face entry; a meld
// pair's result-less half lowers as a single-faced card; a transforming DFC
// lowers; every other multi-face layout is refused
// (a nonland modal back lowers, an Adventure or split card
// with facecastlayouts_test.go). A Battle face lowers since (CR 310),
// not as an unknown layout.
func TestFaceLayouts(t *testing.T) {
	accept := map[string]bool{"Akoum Warrior": true, "Cragcrown Pathway": true, "Midnight Scavengers": true, "Bruna, the Fading Light": true, "Dion, Bahamut's Dominant": true, "Invasion of Zendikar": true}
	refuse := map[string]bool{"Graf Rats": true}
	var names []string
	for n := range accept {
		names = append(names, n)
	}
	for n := range refuse {
		names = append(names, n)
	}
	seen := 0
	for _, c := range lowerNamed(t, names) {
		multi := false
		for _, r := range c.Refusals {
			multi = multi || strings.HasPrefix(r.Reason, "multi-face card")
		}
		switch {
		case accept[c.Name]:
			seen++
			if len(c.Refusals) > 0 {
				t.Errorf("%s refused: %v", c.Name, c.Refusals)
			}
		case refuse[c.Name]:
			seen++
			if !multi {
				t.Errorf("%s: not refused as a multi-face card (%v)", c.Name, c.Refusals)
			}
		}
		switch c.Name {
		case "Akoum Warrior":
			if len(c.Faces) != 2 || c.Faces[1] != faceKey("Akoum Warrior", 1) || c.Face != 0 || len(c.Tokens) != 1 {
				t.Errorf("Akoum Warrior faces %v tokens %v", c.Faces, c.Tokens)
			}
		case "Midnight Scavengers":
			if len(c.Faces) != 0 {
				t.Errorf("Midnight Scavengers has faces %v", c.Faces)
			}
		case "Dion, Bahamut's Dominant":
			if c.FaceMode != kop.FaceTransform || len(c.Faces) != 2 || c.Faces[1] != faceKey(c.Name, 1) {
				t.Errorf("Dion: layout %d faces %v", c.FaceMode, c.Faces)
			}
		}
	}
	if seen != len(names) {
		t.Errorf("lowered %d of %d cards", seen, len(names))
	}
}
