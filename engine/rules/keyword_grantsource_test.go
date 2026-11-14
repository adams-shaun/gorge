package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestKeywordGrantSourceNamesTheGrantingStatic: Jandor, Fortuned Traveler's
// static grants "Saddle:2" to a Beast, and KeywordGrantSource names Jandor as
// the grantor; the Beast grants itself nothing, and a line no static grants
// has no source.
func TestKeywordGrantSourceNamesTheGrantingStatic(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	jandor := lookup(t, reg, "Jandor, Fortuned Traveler")
	beast, diags := cards.ParseBytes("fixture", []byte("Name:Test Beast\nTypes:Creature Beast\nPT:2/2\nOracle:x\n"))
	if len(diags) != 0 {
		t.Fatalf("fixture parse: %+v", diags)
	}
	e := corpusEngine(t, reg, []*cards.Card{jandor, beast}, nil)
	jid := moveByName(t, e, 0, "Jandor, Fortuned Traveler", state.ZBattlefield)
	bid := moveByName(t, e, 0, "Test Beast", state.ZBattlefield)
	if param, ok := e.derivedKeywordParam(bid, "Saddle"); !ok || param != "2" {
		t.Fatalf("precondition: derived Saddle grant = (%q, %v), want (\"2\", true)", param, ok)
	}
	if got := e.KeywordGrantSource(bid, "Saddle:2"); got != jid {
		t.Fatalf("KeywordGrantSource(beast, Saddle:2) = %d, want Jandor %d", got, jid)
	}
	if got := e.KeywordGrantSource(bid, "Saddle:3"); got != 0 {
		t.Fatalf("KeywordGrantSource(beast, Saddle:3) = %d, want 0 (no static grants it)", got)
	}
	if got := e.KeywordGrantSource(jid, "Saddle:2"); got != 0 {
		t.Fatalf("KeywordGrantSource(jandor, Saddle:2) = %d, want 0 (a Human is no Mount candidate)", got)
	}
}
