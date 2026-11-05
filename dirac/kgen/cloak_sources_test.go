package kgen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// kcloakSourceList lowers the two MKM cloak-source carriers against the
// corpus and returns them by name.
func kcloakSourceList(t *testing.T) map[string]*lCard {
	t.Helper()
	list := filepath.Join(t.TempDir(), "cards.txt")
	const listText = "Vannifar, Evolved Enigma\nEtrata, Deadly Fugitive\nGrizzly Bears\n"
	if err := os.WriteFile(list, []byte(listText), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := loadCardList(list, ratchetCards(t))
	if err != nil {
		t.Fatal(err)
	}
	lw := lowerPool(p)
	byName := map[string]*lCard{}
	for _, c := range lw.cards {
		byName[c.Name] = c
	}
	for _, n := range []string{"Vannifar, Evolved Enigma", "Etrata, Deadly Fugitive"} {
		if byName[n] == nil {
			t.Fatalf("%s not lowered", n)
		}
	}
	return byName
}

// TestKgenCloakSourcesLower: Vannifar's combat trigger's Charm lowers its
// cloak mode as a ChooseCard hand pick (gorge's face_down_choice) followed by
// an OpCloak over the chosen card (CfChosen), and its counter mode stays; the
// Etrata trigger's ValidLibrary cloak lowers as an OpCloak over the damaged
// player's library top (CfCloakCtrl, the cloaker is the resolving
// controller), while Etrata's card still refuses on its TurnFaceUp static
// grant.
func TestKgenCloakSourcesLower(t *testing.T) {
	byName := kcloakSourceList(t)

	v := byName["Vannifar, Evolved Enigma"]
	if len(v.Refusals) != 0 {
		t.Fatalf("Vannifar refused: %q", lowerRefusals(v))
	}
	var pick, cloak, counters bool
	for _, a := range v.Abilities {
		for _, in := range a.Prog {
			switch {
			case in.Op == "ChooseCard" && in.CS == "FaceDownPick" && in.Obj.Kind == rYou && in.C == zHand && in.F != nil:
				pick = true
			case in.Op == "Cloak" && in.C&kop.CfChosen != 0 && in.A.Kind == vConst && in.A.N == 1:
				cloak = true
			case in.Op == "PutCounterAll":
				counters = true
			}
		}
	}
	if !pick || !cloak || !counters {
		t.Fatalf("Vannifar programs lack the cloak pick (%v), the chosen cloak (%v) or the counter mode (%v)", pick, cloak, counters)
	}

	e := byName["Etrata, Deadly Fugitive"]
	if len(e.Refusals) == 0 {
		t.Fatal("Etrata no longer refuses; the brief's next-refusal stop needs revisiting")
	}
	if !strings.Contains(e.Refusals[0].Reason, "TurnFaceUp") {
		t.Fatalf("Etrata's first refusal is %q, want the TurnFaceUp static grant", e.Refusals[0].Reason)
	}
	var libCloak bool
	for _, a := range e.Abilities {
		for _, in := range a.Prog {
			if in.Op == "Cloak" && in.Obj.Kind == rTriggeredPlayer && in.C&kop.CfCloakCtrl != 0 &&
				in.C&kop.CfChosen == 0 && in.A.Kind == vConst && in.A.N == 1 {
				libCloak = true
			}
		}
	}
	if !libCloak {
		t.Fatal("Etrata's trigger lacks the ValidLibrary cloak (obj=triggeredplayer, CfCloakCtrl)")
	}
}
