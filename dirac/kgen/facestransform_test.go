package kgen

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// TestGenBattleRefused (CR 310): every MOM Siege is a
// transforming DFC whose front is a Battle. With battlesAccepted the 15
// shapes kgen reads lower; the other 21 are refused by their NEXT blocker
// (a front-face effect or a back-face shape), never by the generic faces
// class and never by "battle (CR 310)".
func TestGenBattleRefused(t *testing.T) {
	cardsDir := ratchetCards(t)
	names, err := readCardList(filepath.Join(testSetlistDir, "MOM.txt"))
	if err != nil {
		t.Fatal(err)
	}
	p, err := loadCardNames(names, cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	// The build-probe upper bound: the 15 MOM Sieges whose
	// front and back kgen reads exactly.
	lifted := map[string]bool{
		"Invasion of Azgol": true, "Invasion of Belenon": true,
		"Invasion of Dominaria": true, "Invasion of Ergamon": true,
		"Invasion of Innistrad": true, "Invasion of Ixalan": true,
		"Invasion of Kaladesh": true, "Invasion of Karsus": true,
		"Invasion of Kylem": true, "Invasion of Mercadia": true,
		"Invasion of Moag": true, "Invasion of Muraganda": true,
		"Invasion of Tolvada": true, "Invasion of Xerex": true,
		"Invasion of Zendikar": true,
		"Invasion of Vryn":     true, // its back face copies a spell
		"Invasion of Theros":   true, // Demigod, a creature type outside the pool
		// Pyre of the World Tree's Discard<1/Land> activated cost
		// (discard-filtered)
		"Invasion of Kaldheim": true,
		// its back face's Origin$ Hand Permanent.YouOwn move
		// (CR 303.4f/g)
		"Invasion of Shandalar": true,
		// its back face's upkeep damage gated on the opponent's hand size
		// and on its not being your turn (Prickle Faeries)
		"Invasion of Eldraine": true,
		// its back face's AffectedZone$ Stack Convoke grant (Invasion of
		// Segovia lowers to a StackKw static)
		"Invasion of Segovia": true,
		// its enters trigger's target, a non-Elf creature with power X or
		// less (X the lands you control): PredPowerGEV PredN 2
		"Invasion of Lorwyn": true,
	}
	var battles []string
	for _, n := range names {
		c := p.byName[n]
		if c.AlternateMode != "DoubleFaced" {
			continue
		}
		for _, f := range c.Faces {
			if f == nil {
				continue
			}
			for _, ty := range f.Types {
				if ty == "Battle" {
					battles = append(battles, n)
				}
			}
		}
	}
	if len(battles) != 36 {
		t.Fatalf("MOM has %d Battle DFCs, want 36", len(battles))
	}
	want := map[string]bool{}
	for _, n := range battles {
		want[n] = true
	}
	lw := lowerPool(p)
	if _, _, _, err := buildPool(lw); err != nil {
		t.Fatal(err)
	}
	seen, got := 0, map[string]bool{}
	for _, c := range lw.cards {
		if !want[c.Name] {
			continue
		}
		seen++
		if len(c.Refusals) == 0 {
			got[c.Name] = true
			continue
		}
		for _, r := range c.Refusals {
			if strings.HasPrefix(r.Reason, "multi-face card") {
				t.Errorf("%s refused as a multi-face card, want its next blocker (%v)", c.Name, c.Refusals)
			}
			if strings.HasPrefix(r.Reason, "battle (CR 310)") {
				t.Errorf("%s still refused as an unread battle shape (%v)", c.Name, c.Refusals)
			}
		}
	}
	if seen != len(battles) {
		t.Errorf("saw %d of %d Battle DFCs", seen, len(battles))
	}
	for n := range lifted {
		if !got[n] {
			t.Errorf("%s did not lower; the lift regressed", n)
		}
	}
	for n := range got {
		if !lifted[n] {
			t.Errorf("%s lowered but is not in the lifted set", n)
		}
	}
}

// TestGenTransformedShapes: the three clean
// transforming shapes lower — SetState Transform Self (Dowsing Device),
// ChangeZone Transformed$ True from exile or graveyard (The Modern Age,
// Dion), and a Mode$ Transformed trigger (Norn's Inquisitor) — and the
// front/back faces link as a transforming layout.
func TestGenTransformedShapes(t *testing.T) {
	cards := lowerNamed(t, []string{"Dowsing Device", "The Modern Age", "Dion, Bahamut's Dominant", "Norn's Inquisitor"})
	if len(cards) != 4 {
		t.Fatalf("%d cards, want 4", len(cards))
	}
	byName := map[string]*lCard{}
	for _, c := range cards {
		byName[c.Name] = c
		for _, r := range c.Refusals {
			t.Errorf("%s refused: %s", c.Name, r.Reason)
		}
		if c.Name == "Norn's Inquisitor" {
			continue // single-faced: only the trigger is in scope
		}
		if c.FaceMode != kop.FaceTransform || len(c.Faces) != 2 || c.Faces[1] != faceKey(c.Name, 1) {
			t.Errorf("%s: layout %d faces %v, want a transforming DFC", c.Name, c.FaceMode, c.Faces)
		}
	}
	if c := byName["Dowsing Device"]; !hasInstr(c, "Transform", rSelf, "") {
		t.Errorf("Dowsing Device: no Transform Self")
	}
	if c := byName["The Modern Age"]; !hasInstr(c, "Return", rRemembered, "Transformed") || !hasInstr(c, "Exile", rSelf, "Remember") {
		t.Errorf("The Modern Age: no exile-then-return-transformed")
	}
	if c := byName["Dion, Bahamut's Dominant"]; !hasInstr(c, "Return", rRemembered, "Transformed") || !hasInstr(c, "Exile", rSelf, "Remember") {
		t.Errorf("Dion: no exile-then-return-transformed")
	}
	if c := byName["Norn's Inquisitor"]; !hasTrigger(c, "Transformed") {
		t.Errorf("Norn's Inquisitor: no Mode$ Transformed trigger")
	}
}

// syntheticTDFC builds a two-face DoubleFaced card whose front carries one
// activated ability with api and params, for the refused Transformed$ shapes.
func syntheticTDFC(api string, params map[string]string) *cards.Card {
	front := &cards.Face{
		Name: "Synthetic Front", ManaCost: "1 G", Types: []string{"Creature"},
		Abilities: []*cards.SA{{Kind: "AB", API: api, Params: params}},
	}
	back := &cards.Face{Name: "Synthetic Back", ManaCost: "no cost", Types: []string{"Creature"}}
	return &cards.Card{Faces: []*cards.Face{front, back}, AlternateMode: cards.AlternateDoubleFaced}
}

// syntheticSingleFace is the same card without a second face: a
// Transformed$ return on it must refuse (the layout cannot turn).
func syntheticSingleFace(params map[string]string) *cards.Card {
	front := &cards.Face{
		Name: "Synthetic Front", ManaCost: "1 G", Types: []string{"Creature"},
		Abilities: []*cards.SA{{Kind: "AB", API: "ChangeZone", Params: params}},
	}
	return &cards.Card{Faces: []*cards.Face{front}}
}

func lowerSynthetic(c *cards.Card) *lCard {
	p := &pool{byName: map[string]*cards.Card{"Synthetic": c}, nDecks: map[string]int{}, reg: &cards.Registry{}}
	return lowerCard(p, "Synthetic", c, false)
}

// TestGenTransformedRefusals: every Transformed$
// shape the kernel cannot carry refuses with a named reason, never lowering
// the flag silently. The accepted shapes are the plain battlefield Return
// of Self or of this chain's own Remembered self-exile.
func TestGenTransformedRefusals(t *testing.T) {
	for _, tc := range []struct {
		name string
		card *cards.Card
		want string
	}{
		{"ChangeZoneAll", syntheticTDFC("ChangeZoneAll", map[string]string{
			"Cost": "1", "Origin": "Graveyard", "Destination": "Battlefield", "Transformed": "True",
		}), "ChangeZoneAll Transformed$"},
		{"search", syntheticTDFC("Search", map[string]string{
			"Cost": "1", "Transformed": "True",
		}), "api Search"},
		{"destination", syntheticTDFC("ChangeZone", map[string]string{
			"Cost": "1", "Defined": "Self", "Origin": "Graveyard", "Destination": "Hand", "Transformed": "True",
		}), "Transformed$ to Hand"},
		{"origin", syntheticTDFC("ChangeZone", map[string]string{
			"Cost": "1", "Defined": "Self", "Origin": "Hand", "Destination": "Battlefield", "Transformed": "True",
		}), "Transformed$ from Hand"},
		{"remembered without self-exile", syntheticTDFC("ChangeZone", map[string]string{
			"Cost": "1", "Defined": "Remembered", "Origin": "Exile", "Destination": "Battlefield", "Transformed": "True",
		}), "Defined$ Remembered without this chain's own self-exile"},
		{"delayed remembered", syntheticTDFC("ChangeZone", map[string]string{
			"Cost": "1", "Defined": "DelayTriggerRemembered", "Origin": "Graveyard", "Destination": "Battlefield", "Transformed": "True",
		}), `Defined$ "DelayTriggerRemembered"`},
		{"not a transforming layout", syntheticSingleFace(map[string]string{
			"Cost": "1", "Defined": "Self", "Origin": "Graveyard", "Destination": "Battlefield", "Transformed": "True",
		}), "layout the kernel does not turn"},
	} {
		c := lowerSynthetic(tc.card)
		found := false
		for _, r := range c.Refusals {
			if strings.Contains(r.Reason, tc.want) {
				found = true
			}
		}
		if !found {
			t.Errorf("%s: refusals %v, want %q", tc.name, c.Refusals, tc.want)
		}
	}
}

// hasInstr reports whether c has an instruction with op and object kind,
// whose CS contains the fragment (empty matches any CS).
func hasInstr(c *lCard, op string, obj refKind, cs string) bool {
	for _, a := range c.Abilities {
		for _, in := range a.Prog {
			if in.Op == op && in.Obj.Kind == obj && (cs == "" || strings.Contains(in.CS, cs)) {
				return true
			}
		}
	}
	return false
}

// hasTrigger reports whether c has a triggered ability with event.
func hasTrigger(c *lCard, event string) bool {
	for _, a := range c.Abilities {
		if a.Trigger != nil && a.Trigger.Event == event {
			return true
		}
	}
	return false
}
