package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// A modal double-faced spell cast as its back face has only that face's
// characteristics on the stack (CR 712.8, 712.11c), so the snapshot labels it
// by the back face's name, as Forge does, not by the card's front face.
func TestOracleModalBackFaceSpellStackLabelIsBackFace(t *testing.T) {
	res, _ := runFixtureScenario(t, `{"name":"mdfc-back","setup":{"p0":{"hand":["Monica Rambeau"]},"p1":{}},"steps":[{"op":"cast","seat":0,"card":"p0:Monica Rambeau","mana":"WWRRR","cast_mode":"modal_spell"}]}`)
	var snap *OracleSnapshot
	for i := range res.Snapshots {
		if len(res.Snapshots[i].Stack) > 0 {
			snap = &res.Snapshots[i]
		}
	}
	if snap == nil {
		t.Fatal("no checkpoint has the back-face spell on the stack")
	}
	if got := snap.Stack[0].Source; got != "p0:Photon, Living Light" {
		t.Errorf("stack source = %q, want p0:Photon, Living Light", got)
	}
}

// The front face cast from the same card keeps the setup ref's name.
func TestOracleModalFrontFaceSpellStackLabelKeepsRef(t *testing.T) {
	res, _ := runFixtureScenario(t, `{"name":"mdfc-front","setup":{"p0":{"hand":["Monica Rambeau"]},"p1":{}},"steps":[{"op":"cast","seat":0,"card":"p0:Monica Rambeau","mana":"CCW"}]}`)
	var snap *OracleSnapshot
	for i := range res.Snapshots {
		if len(res.Snapshots[i].Stack) > 0 {
			snap = &res.Snapshots[i]
		}
	}
	if snap == nil {
		t.Fatal("no checkpoint has the front-face spell on the stack")
	}
	if got := snap.Stack[0].Source; got != "p0:Monica Rambeau" {
		t.Errorf("stack source = %q, want p0:Monica Rambeau", got)
	}
}

// Census: every modal double-faced card in the corpus, on the stack as its
// back face, is labelled by the back face's name under the setup ref's seat
// prefix and ordinal; on the stack as its front face it keeps the setup ref.
func TestOracleModalSpellStackLabelCensus(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	n := 0
	for _, c := range reg.AllCards() {
		if c == nil || c.AlternateMode != "Modal" || len(c.Faces) != 2 || c.Faces[1] == nil || c.Faces[1].Name == "" {
			continue
		}
		n++
		ref := "p1:" + c.Faces[0].Name + "#2"
		back := &state.Object{Card: c, FaceIdx: 1, Zone: state.ZStack}
		if got, want := stackSourceRef(ref, back), "p1:"+c.Faces[1].Name+"#2"; got != want {
			t.Errorf("%s: back-face label %q, want %q", c.Faces[0].Name, got, want)
		}
		front := &state.Object{Card: c, FaceIdx: 0, Zone: state.ZStack}
		if got := stackSourceRef(ref, front); got != ref {
			t.Errorf("%s: front-face label %q, want %q", c.Faces[0].Name, got, ref)
		}
	}
	if n < 20 {
		t.Fatalf("only %d modal double-faced cards in the corpus; census is vacuous (corpus missing?)", n)
	}
}
