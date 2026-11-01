package draftlab

import "testing"

// TestLabCard: Lab.Card resolves a name against the lab's corpus registry;
// an unknown name has no card.
func TestLabCard(t *testing.T) {
	l := testLab(t)
	c, ok := l.Card("Island")
	if !ok {
		t.Fatal("Card(Island): the corpus carries no Island")
	}
	if c == nil || len(c.Faces) == 0 || c.Faces[0].Name != "Island" {
		t.Fatalf("Card(Island) = %+v, want the Island card", c)
	}
	if _, ok := l.Card("Not A Real Card Name"); ok {
		t.Error("Card resolved a name the corpus does not carry")
	}
}
