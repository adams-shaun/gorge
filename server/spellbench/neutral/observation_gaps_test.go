package neutral

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/server/spellbench/protocol"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestPlainObservationRebuildFillsCarriedFacts pins the rebuild facts:
// the observation's turn (round is derived from it), pool, land-drop count,
// and the printed facts the registry the session already holds supplies. Every
// asserted field is one the plain v2 observation carries or the registry
// knows; a field the observation does not carry (spells cast, available mana,
// potential actions, ...) is deliberately not invented here.
func TestPlainObservationRebuildFillsCarriedFacts(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	b := NewBuilder(reg)
	for _, name := range []string{"Mother of Runes", "Urza's Mine", "Delver of Secrets", "Kavu Scout", "Kinsbaile Skirmisher"} {
		if r, err := Resolve(reg, name); err != nil || r.Card == nil {
			t.Fatalf("setup: %s is not in the corpus: %v", name, err)
		}
	}
	mother, mine := "Mother of Runes", "Urza's Mine"
	back, scout, kinsbaile := "Insectile Aberration", "Kavu Scout", "Kinsbaile Skirmisher"
	token := "Goblin"
	hand := func(name *string) protocol.ObjectRecord {
		return protocol.ObjectRecord{ObjectRef: protocol.ObjectRef{ObjectID: "h-" + *name,
			CardName: name, OwnerSeat: "p0", ControllerSeat: "p0", Zone: "hand"},
			Characteristics: &protocol.Characteristics{Supertypes: []string{}, Types: []string{"creature"}, Subtypes: []string{}}}
	}
	// Urza's Mine's printed type line carries punctuation the observation's
	// type vocabulary folds away; the rebuild must recover it from the face.
	mineRec := hand(&mine)
	mineRec.Characteristics.Types, mineRec.Characteristics.Subtypes = []string{"land"}, []string{"urzas_mine"}
	// Insectile Aberration is Delver of Secrets' back face: the observed name
	// is the back face, the printed front face is the registry card's.
	delver := hand(&back)
	delver.Characteristics.Types = []string{"creature"}
	delver.Characteristics.Subtypes = []string{"human", "insect"}
	kin := hand(&kinsbaile)
	kin.Characteristics.Types = []string{"creature"}
	kin.Characteristics.Subtypes = []string{"kithkin", "soldier"}
	scoutRec := protocol.ObjectRecord{ObjectRef: protocol.ObjectRef{ObjectID: "g-scout", CardName: &scout,
		OwnerSeat: "p0", ControllerSeat: "p0", Zone: "graveyard"},
		Characteristics: &protocol.Characteristics{Supertypes: []string{}, Types: []string{"creature"}, Subtypes: []string{"kavu", "scout"}}}
	tokenRec := protocol.ObjectRecord{ObjectRef: protocol.ObjectRef{ObjectID: "b-token", CardName: &token,
		OwnerSeat: "p0", ControllerSeat: "p0", Zone: "battlefield"}, Token: true,
		Permanent: &protocol.Permanent{SummoningSick: true}}
	obs := &protocol.Observation{Viewer: "p0", Turn: 5, PhaseStep: "precombat_main",
		Players: [2]protocol.PlayerObs{{
			Seat: "p0", Life: 20, ManaPool: protocol.ManaPool{W: 2}, LandsPlayedThisTurn: 1,
			Hand:        []protocol.ObjectRecord{hand(&mother), mineRec, delver, kin},
			Graveyard:   []protocol.ObjectRecord{scoutRec},
			Battlefield: []protocol.ObjectRecord{tokenRec},
		}, {Seat: "p1", Life: 20}}}
	// The preconditions the assertions depend on: the objects sit in the zones
	// the rebuild reads, and the token does not name a registry card.
	if got := len(obs.Players[0].Hand); got != 4 {
		t.Fatalf("setup: hand has %d cards, want 4", got)
	}
	if got := len(obs.Players[0].Graveyard); got != 1 || obs.Players[0].Graveyard[0].Permanent != nil {
		t.Fatalf("setup: graveyard record is not a bare off-battlefield card")
	}
	if len(obs.Players[0].Battlefield) != 1 || !obs.Players[0].Battlefield[0].Token {
		t.Fatalf("setup: battlefield token is missing")
	}

	v, err := b.View(obs)
	if err != nil {
		t.Fatal(err)
	}
	// One round is two turns in the v2 observation's two-seat profile: turn 5
	// opens round 3. A zero or turn-echoed round is wrong.
	if v.Round != 3 {
		t.Errorf("Round = %d, want 3 for turn 5", v.Round)
	}
	p0 := v.Players[0]
	// Native's poolView keeps only the colours holding mana; six zero keys is
	// the bug the census measured on every decision.
	if len(p0.Pool) != 1 || p0.Pool["W"] != 2 {
		t.Errorf("Pool = %v, want only W:2", p0.Pool)
	}
	if !p0.LandDropSpent {
		t.Error("LandDropSpent = false, want true after one land played")
	}
	got := p0.Hand
	if got[0].ColourIdentity != cards.ColourMask(cards.ColourWhite) {
		t.Errorf("Mother of Runes ColourIdentity = %#x, want white", got[0].ColourIdentity)
	}
	if got[0].PrintedPT != "1/1" {
		t.Errorf("Mother of Runes PrintedPT = %q, want 1/1", got[0].PrintedPT)
	}
	if !slices.Equal(got[0].AbilityCosts, []string{"T"}) {
		t.Errorf("Mother of Runes AbilityCosts = %v, want [T]", got[0].AbilityCosts)
	}
	if got[1].Types != "Land Urza's Mine" {
		t.Errorf("Urza's Mine Types = %q, want the printed punctuation", got[1].Types)
	}
	if got[2].Name != "Insectile Aberration" || got[2].CardName != "Delver of Secrets" {
		t.Errorf("Delver back face = name %q card_name %q, want Insectile Aberration / Delver of Secrets",
			got[2].Name, got[2].CardName)
	}
	if got[3].PlayMain1 != "TRUE" {
		t.Errorf("Kinsbaile Skirmisher PlayMain1 = %q, want TRUE", got[3].PlayMain1)
	}
	gy := p0.Graveyard[0]
	if !gy.RemoveDeck {
		t.Error("Kavu Scout RemoveDeck = false, want true (AI:RemoveDeck)")
	}
	if gy.SummonSick {
		t.Error("graveyard SummonSick = true; the rebuild must not invent a battlefield transient")
	}
	if gy.AbilityCosts != nil {
		t.Errorf("graveyard AbilityCosts = %v, want nil (native projects them only off the battlefield and own hand)", gy.AbilityCosts)
	}
	bf := p0.Battlefield[0]
	if !bf.IsToken {
		t.Error("battlefield token IsToken = false")
	}
	if !bf.SummonSick {
		t.Error("battlefield token SummonSick = false, want true from its permanent record")
	}
}
