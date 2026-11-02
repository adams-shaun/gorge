package kgen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestIncubatorTokenFaces: an Incubate carrier appends the Incubator token
// and its back face to the pool; the token lowers as a transforming
// double-faced token (FaceTransform), its back face beside it.
func TestIncubatorTokenFaces(t *testing.T) {
	cardsDir := ratchetCards(t)
	list := filepath.Join(t.TempDir(), "cards.txt")
	if err := os.WriteFile(list, []byte("Ichor Drinker\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := loadCardList(list, cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	lw := lowerPool(p)
	if len(lw.cards) != 1 {
		t.Fatalf("%d cards, want 1", len(lw.cards))
	}
	c := lw.cards[0]
	for _, r := range c.Refusals {
		t.Fatalf("Ichor Drinker refused: %s", r.Reason)
	}
	const key = "incubator_c_0_0_a_phyrexian"
	want := map[string]bool{key: true, faceKey(key, 1): true}
	for _, k := range c.Tokens {
		delete(want, k)
	}
	if len(want) != 0 {
		t.Fatalf("Ichor Drinker tokens %v, missing %v", c.Tokens, want)
	}
	seen := map[string]*lCard{}
	for _, tok := range lw.tokens {
		seen[tok.Name] = tok
	}
	front, back := seen[key], seen[faceKey(key, 1)]
	if front == nil || back == nil {
		t.Fatalf("lowered tokens %v, want the Incubator token and its back", lw.tokens)
	}
	if !front.Token || front.FaceMode != kop.FaceTransform || front.Face != 0 || len(front.Faces) != 2 {
		t.Fatalf("front token=%v face=%d mode=%d faces=%v, want a transforming token", front.Token, front.Face, front.FaceMode, front.Faces)
	}
	if !back.Token || back.FaceMode != kop.FaceTransform || back.Face != 1 {
		t.Fatalf("back token=%v face=%d mode=%d, want face 1 of a transforming token", back.Token, back.Face, back.FaceMode)
	}
}

// TestBattleTransformLifted (CR 310): a transforming
// double-faced Battle lowers (defense + protector + defeat ask) instead of
// being refused by the old CR 310 card-level check, and its faces link as a
// transforming DFC.
func TestBattleTransformLifted(t *testing.T) {
	cards := lowerNamed(t, []string{"Invasion of Zendikar", "Dion, Bahamut's Dominant"})
	if len(cards) != 2 {
		t.Fatalf("%d cards, want 2", len(cards))
	}
	byName := map[string]*lCard{}
	for _, c := range cards {
		byName[c.Name] = c
	}
	inv, dion := byName["Invasion of Zendikar"], byName["Dion, Bahamut's Dominant"]
	if inv == nil || dion == nil {
		t.Fatalf("lowered %v", cards)
	}
	for _, r := range inv.Refusals {
		if strings.HasPrefix(r.Reason, "battle (CR 310)") {
			t.Fatalf("Invasion of Zendikar still refused as a battle: %s", r.Reason)
		}
	}
	if inv.FaceMode != kop.FaceTransform || len(inv.Faces) != 2 {
		t.Fatalf("Invasion of Zendikar: layout %d faces %v, want a transforming DFC", inv.FaceMode, inv.Faces)
	}
	for _, r := range dion.Refusals {
		t.Fatalf("Dion refused: %s", r.Reason)
	}
	if dion.FaceMode != kop.FaceTransform || len(dion.Faces) != 2 {
		t.Fatalf("Dion: layout %d faces %v, want a transforming DFC", dion.FaceMode, dion.Faces)
	}
}
