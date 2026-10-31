package rules

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/rules/chars"
	"github.com/adams-shaun/gorge/engine/rules/pay"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// CR 205.1a: subtypes correlated with a removed card type remain if they are
// also subtypes of a card type the object has; otherwise they are removed. An
// effect that sets subtypes replaces the existing ones from that set.
func TestCardTypeLossKeepsSubtypesOfRemainingTypes(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		aura, permanent string
		want            []string
	}{
		{"Imprisoned in the Moon", "Forest", []string{"Basic", "Forest", "Land"}},
		{"Imprisoned in the Moon", "Grizzly Bears", []string{"Land"}},
		{"Imprisoned in the Moon", "Dryad Arbor", []string{"Forest", "Land"}},
		{"Imprisoned in the Moon", "Urza's Saga", []string{"Land", "Urza's"}},
		{"Imprisoned in the Moon", "The Belligerent and Useless Island", []string{"Island", "Land", "Legendary"}},
		{"Darksteel Mutation", "Grizzly Bears", []string{"Artifact", "Creature", "Insect"}},
		{"Darksteel Mutation", "Dryad Arbor", []string{"Artifact", "Creature", "Insect"}},
		{"Darksteel Mutation", "Lizard Blades", []string{"Artifact", "Creature", "Equipment", "Insect"}},
		{"One with the Stars", "Honden of Seeing Winds", []string{"Enchantment", "Legendary", "Shrine"}},
		{"One with the Stars", "Enduring Courage", []string{"Enchantment"}},
		{"Minimus Containment", "Bonesplitter", []string{"Artifact", "Treasure"}},
		{"Swift Reconfiguration", "Lizard Blades", []string{"Artifact", "Vehicle"}},
	} {
		t.Run(tc.aura+" on "+tc.permanent, func(t *testing.T) {
			t.Parallel()
			reg := testutil.CorpusRegistry(t)
			e, _ := landTypesEngine(t, reg,
				[]*cards.Card{lookup(t, reg, tc.aura), lookup(t, reg, tc.permanent)}, nil)
			id := moveByName(t, e, 0, tc.permanent, state.ZBattlefield)
			lossAttach(t, e, tc.aura, 0, id)
			got := slices.Sorted(slices.Values(e.Derived(id).Types))
			if !slices.Equal(got, tc.want) {
				t.Fatalf("types = %v, want %v", got, tc.want)
			}
		})
	}
}

// A land creature that is also an artifact keeps only its land subtype.
func TestImprisonedInTheMoonOnArtifactCreatureLand(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, _ := landTypesEngine(t, reg,
		[]*cards.Card{lookup(t, reg, "Imprisoned in the Moon"), lookup(t, reg, "Dryad Arbor")}, nil)
	id := moveByName(t, e, 0, "Dryad Arbor", state.ZBattlefield)
	e.AddContinuous(state.ContinuousEffect{Source: id, Affects: "Card.Self", Layer: state.LType,
		AddTypes: []string{"Artifact", "Equipment"}})
	lossAttach(t, e, "Imprisoned in the Moon", 0, id)
	got := slices.Sorted(slices.Values(e.Derived(id).Types))
	if want := []string{"Forest", "Land"}; !slices.Equal(got, want) {
		t.Fatalf("types = %v, want %v", got, want)
	}
}

// The land type stays but the mana ability it carries (CR 305.6) arrives in
// layer 4, so the layer-6 removal takes it: the land offers only the granted
// {C}.
func TestImprisonedLandHasNoIntrinsicManaAbility(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, cfg := landTypesEngine(t, reg,
		[]*cards.Card{lookup(t, reg, "Imprisoned in the Moon"), lookup(t, reg, "Forest")}, nil)
	id := moveByName(t, e, 0, "Forest", state.ZBattlefield)
	if mas := e.availableManaAbilities(0, id); !pay.ManaAbilitiesProduce(mas, "G") {
		t.Fatal("precondition: Forest does not tap for {G}")
	}
	lossAttach(t, e, "Imprisoned in the Moon", 0, id)
	mas := e.availableManaAbilities(0, id)
	if pay.ManaAbilitiesProduce(mas, "G") {
		t.Fatal("Forest still taps for {G} after losing all other abilities")
	}
	if !pay.ManaAbilitiesProduce(mas, "C") {
		t.Fatal("Forest does not tap for the granted {C}")
	}
	replayCheck(t, e, cfg)
}

// Titania's Song makes a noncreature artifact an artifact creature, so an
// Equipment stays an Equipment and, being a creature, falls off (CR 301.5c,
// 704.5q).
func TestTitaniasSongKeepsArtifactSubtypes(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, _ := landTypesEngine(t, reg, []*cards.Card{lookup(t, reg, "Titania's Song"),
		lookup(t, reg, "Bonesplitter"), lookup(t, reg, "Grizzly Bears")}, nil)
	bears := moveByName(t, e, 0, "Grizzly Bears", state.ZBattlefield)
	blade := moveByName(t, e, 0, "Bonesplitter", state.ZBattlefield)
	e.emit(events.Event{Kind: events.Attach, Obj: blade, IDs: []state.ObjID{bears}})
	moveByName(t, e, 0, "Titania's Song", state.ZBattlefield)
	e.pending = nil
	e.Advance()
	got := slices.Sorted(slices.Values(e.Derived(blade).Types))
	if want := []string{"Artifact", "Creature", "Equipment"}; !slices.Equal(got, want) {
		t.Fatalf("types = %v, want %v", got, want)
	}
	if to := e.G.Obj(blade).AttachedTo; to != 0 {
		t.Fatalf("an Equipment that is a creature is still attached to %d", to)
	}
}

// The word lists behind the CR 205.1a rule cover the corpus: every subtype a
// face prints belongs to one of that face's card types.
func TestPrintedSubtypesBelongToAPrintedCardType(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	modelled := []string{"Artifact", "Creature", "Enchantment", "Land"}
	reg.Universe().EachFaceTypes(func(types []string) {
		var keep []effects.SubtypesKept
		for _, ty := range types {
			if slices.Contains(modelled, ty) {
				keep = append(keep, effects.SubtypesKeptBy([]string{ty}))
			} else if effects.CardTypeWord(ty) {
				return // a type no RemoveCardTypes$ effect adds
			}
		}
		if len(keep) == 0 {
			return
		}
		for _, w := range types {
			// Host is an un-set word that behaves as a supertype.
			if effects.CardTypeWord(w) || chars.IsSupertype(w) || w == "Host" {
				continue
			}
			if !slices.ContainsFunc(keep, func(k effects.SubtypesKept) bool { return k.Has(w) }) {
				t.Errorf("%q on %v belongs to none of its card types", w, types)
			}
		}
	})
}
