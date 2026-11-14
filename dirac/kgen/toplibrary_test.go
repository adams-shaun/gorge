package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
)

// topLibChain lowers a synthetic sorcery whose root is a PeekAndReveal with
// the SubAbility `sub` (the else branch after an optional reveal) and returns
// the refusal reasons and the emitted program as Op[:CS][ C=n] strings.
func topLibChain(t *testing.T, sub string) (refusals, ops []string) {
	t.Helper()
	root := "SP$ PeekAndReveal | Cost$ B | PeekAmount$ 1 | RevealValid$ Land | RevealOptional$ True | RememberRevealed$ True"
	svars := map[string]string{"DBX": sub}
	r := cards.ResolveSVar(map[string]string{"R": root}, "R")
	if r == nil {
		t.Fatalf("root %q did not resolve", root)
	}
	r.Params["SubAbility"] = "DBX"
	r.Sub = cards.ResolveSVar(svars, "DBX")
	if r.Sub == nil {
		t.Fatalf("sub %q did not resolve", sub)
	}
	face := &cards.Face{Name: "Synthetic", ManaCost: "B", Types: []string{"Sorcery"}, SVars: svars, Abilities: []*cards.SA{r}}
	card := &cards.Card{Faces: []*cards.Face{face}}
	p := &pool{byName: map[string]*cards.Card{"Synthetic": card}, nDecks: map[string]int{}, reg: &cards.Registry{}}
	lc := lowerCard(p, "Synthetic", card, false)
	for _, rf := range lc.Refusals {
		refusals = append(refusals, rf.Reason)
	}
	for _, a := range lc.Abilities {
		for _, in := range a.Prog {
			ops = append(ops, in.Op)
			if in.Op == "ToLibrary" {
				ops[len(ops)-1] += " C=" + string(rune('0'+in.C))
			}
		}
	}
	return
}

// TestGenTopLibraryOptional: the Optional$ True ChangeZone Defined$
// TopOfLibrary Origin$ Library move lowers to an OpMayAsk
// (MayGuardLibraryTop) over the one move, to the hand, the graveyard or the
// bottom of the library; without Optional$ it lowers to the move behind a
// library-size skip (no ask); every other shape is refused with a named
// reason.
func TestGenTopLibraryOptional(t *testing.T) {
	cond := " | ConditionDefined$ Remembered | ConditionPresent$ Card | ConditionCompare$ EQ0"
	base := "DB$ ChangeZone | Defined$ TopOfLibrary | Origin$ Library | Optional$ True"
	for _, tc := range []struct{ name, sub, want string }{
		{"graveyard", base + " | Destination$ Graveyard" + cond, "Peek SkipIf MayAsk ToGraveyard"},
		{"bottom", base + " | Destination$ Library | LibraryPosition$ -1" + cond, "Peek SkipIf MayAsk ToLibrary C=1"},
		{"hand", base + " | Destination$ Hand", "Peek MayAsk Return"},
		{"no Optional", "DB$ ChangeZone | Defined$ TopOfLibrary | Origin$ Library | Destination$ Graveyard", "Peek SkipIf ToGraveyard"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			refused, ops := topLibChain(t, tc.sub)
			if len(refused) != 0 {
				t.Fatalf("refused: %q", refused)
			}
			if got := strings.Join(ops, " "); got != tc.want {
				t.Errorf("program %q, want %q", got, tc.want)
			}
		})
	}
	for _, tc := range []struct{ name, sub, want string }{
		{"exile", base + " | Destination$ Exile", "Destination$ other than the Hand, Graveyard or the bottom"},
		{"top of the library", base + " | Destination$ Library", "Destination$ other than the Hand, Graveyard or the bottom"},
		{"graveyard with a position", base + " | Destination$ Graveyard | LibraryPosition$ 0", "Destination$ other than the Hand, Graveyard or the bottom"},
		{"library position 0", base + " | Destination$ Library | LibraryPosition$ 0", "Destination$ other than the Hand, Graveyard or the bottom"},
		{"Optional False", "DB$ ChangeZone | Defined$ TopOfLibrary | Origin$ Library | Destination$ Graveyard | Optional$ False", "Optional$"},
		{"origin hand", "DB$ ChangeZone | Defined$ TopOfLibrary | Origin$ Hand | Destination$ Graveyard | Optional$ True", "Origin$ other than Library"},
		{"Chooser", base + " | Destination$ Graveyard | Chooser$ Opponent", "Chooser$"},
		{"ChangeType", base + " | Destination$ Graveyard | ChangeType$ Creature", "ChangeType$"},
		{"ValidTgts", base + " | Destination$ Graveyard | ValidTgts$ Player", "ValidTgts$"},
		{"Mandatory", base + " | Destination$ Graveyard | Mandatory$ True", "Mandatory$"},
		{"Shuffle", base + " | Destination$ Library | LibraryPosition$ -1 | Shuffle$ True", "Shuffle$"},
		{"RememberChanged", base + " | Destination$ Graveyard | RememberChanged$ True", "RememberChanged$"},
		{"DefinedPlayer", base + " | Destination$ Graveyard | DefinedPlayer$ Opponent", "DefinedPlayer$"},
	} {
		t.Run("refuses "+tc.name, func(t *testing.T) {
			refused, ops := topLibChain(t, tc.sub)
			kdefWant(t, refused, tc.want)
			for _, o := range ops {
				if strings.HasPrefix(o, "MayAsk") || strings.HasPrefix(o, "ToGraveyard") {
					t.Errorf("a refused move left a partial program: %q", ops)
				}
			}
		})
	}
}
