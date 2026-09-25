package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/state"
)

// TestAttachedToPlayerWordRecognised pins the flipped `AttachedTo You` pin.
// The word was deliberately UNKNOWN while state.Object.AttachedTo was an
// ObjID and a player attachment had no representation; the
// AttachedPlayer/HasAttachedPlayer pair (written only by events.Attach's
// player branch) landed since, so `Curse.AttachedTo You` -- Lynde, Cheerful
// Tormentor's ChooseCard pool and Witchbane Orb's DestroyAll -- is now a real,
// modelable read and must be recognised rather than refused. The old
// recognise-and-inert objection is answered directly: a candidate matches iff
// it is a battlefield Aura the spec's You seat enchants, which is exactly the
// card text's meaning, so recognition cannot let a card through to a silently
// never-firing test.
//
// The match shape mirrors Player.EnchantedBy: o.HasAttachedPlayer &&
// o.AttachedPlayer == <referent> && o.Zone == ZBattlefield.
func TestAttachedToPlayerWordRecognised(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	g := state.NewGame([]string{"you", "them"})
	bear := corpusObject(t, reg, g, "Grizzly Bears")

	// Three distinct Auras: enchanted to seat 0, enchanted to seat 1, and
	// attached to a creature as an object (the negative control).
	curseOnYou := corpusObject(t, reg, g, "Curse of the Pierced Heart")
	curseOnYou.HasAttachedPlayer = true
	curseOnYou.AttachedPlayer = 0
	curseOnThem := corpusObject(t, reg, g, "Curse of the Pierced Heart")
	curseOnThem.HasAttachedPlayer = true
	curseOnThem.AttachedPlayer = 1
	auraOnBear := corpusObject(t, reg, g, "Unholy Strength")
	auraOnBear.AttachedTo = bear.ID

	// Precondition that makes the negative cases meaningful: the two curses
	// differ only in their enchanted seat, and the creature-attached Aura is
	// genuinely attached to a DIFFERENT kind of bearer.
	if curseOnYou.AttachedPlayer == curseOnThem.AttachedPlayer {
		t.Fatal("precondition: the two curses must enchant different seats")
	}
	if auraOnBear.AttachedTo == 0 {
		t.Fatal("precondition: the creature Aura must be object-attached")
	}

	spec := "Curse.AttachedTo You"
	if un := UnknownPredicates(spec); len(un) != 0 {
		t.Fatalf("UnknownPredicates(%q) = %v, want empty (recognised player-referent word)", spec, un)
	}
	if !MatchesObjectCtx(g, spec, curseOnYou, SpecContext{You: 0}) {
		t.Errorf("%s must match a Curse attached to seat 0", spec)
	}
	if MatchesObjectCtx(g, spec, curseOnThem, SpecContext{You: 0}) {
		t.Errorf("%s must not match a Curse attached to seat 1", spec)
	}
	if MatchesObjectCtx(g, spec, auraOnBear, SpecContext{You: 0}) {
		t.Errorf("%s must not match a creature-attached Aura", spec)
	}

	// The referent is the perspective seat: from seat 1's point of view the
	// other curse is the match and the first is not.
	if !MatchesObjectCtx(g, spec, curseOnThem, SpecContext{You: 1}) {
		t.Errorf("%s must match a Curse attached to seat 1 when You == 1", spec)
	}
	if MatchesObjectCtx(g, spec, curseOnYou, SpecContext{You: 1}) {
		t.Errorf("%s must not match a Curse attached to seat 0 when You == 1", spec)
	}

	// Unbound: a SpecContext whose You is not a seat of this game. The read
	// fails closed (matchPositive ok=false) like the Targeted referents --
	// never a silent false the leading '!' could invert.
	for _, spec := range []string{"Curse.AttachedTo You", "Curse.!AttachedTo You"} {
		if MatchesObjectCtx(g, spec, curseOnYou, SpecContext{You: 5}) {
			t.Errorf("%s with an unbound You must match nothing", spec)
		}
		if _, ok := matchPositive(g, spec, curseOnYou, SpecContext{You: 5}); ok {
			t.Errorf("%s with an unbound You must be unbound (ok=false), got bound", spec)
		}
	}

	// The '!' negation inverts only when bound, and only for the seat it
	// names: a creature-attached Aura and a Curse attached to seat 1 are both
	// "not attached to YOU" (seat 0), while the Curse attached to seat 0 is
	// not. The base is widened to Aura so the creature-attached control has a
	// base of its own (Unholy Strength is an Aura, not a Curse).
	neg := "Aura.!AttachedTo You"
	if MatchesObjectCtx(g, neg, curseOnYou, SpecContext{You: 0}) {
		t.Errorf("%s must not match a Curse attached to seat 0 (it IS attached to you)", neg)
	}
	if !MatchesObjectCtx(g, neg, curseOnThem, SpecContext{You: 0}) {
		t.Errorf("%s must match a Curse attached to seat 1", neg)
	}
	if !MatchesObjectCtx(g, neg, auraOnBear, SpecContext{You: 0}) {
		t.Errorf("%s must match a creature-attached Aura", neg)
	}
}

// TestAttachedToPlayerWordLeavesThePlaneswalkerYouSpellingAlone pins the other
// corpus spelling of the token `You`: one card literally prints
// `Types:Legendary Planeswalker You`, which is a TYPE position, not an
// `AttachedTo` referent. Recognising `AttachedTo You` must not have widened
// the type-word path into accepting a bare `You` base as a type.
func TestAttachedToPlayerWordLeavesThePlaneswalkerYouSpellingAlone(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	g := state.NewGame([]string{"you"})
	bear := corpusObject(t, reg, g, "Grizzly Bears")
	if MatchesObjectCtx(g, "Creature.You", bear, SpecContext{You: 0}) {
		t.Error("a bare You base is not a type word and must not match a creature")
	}
}
