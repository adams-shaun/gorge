package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// pwFullName is the partner a K:Partner with line names: the parameter up to
// a second colon (the short alias after it is a deck hint, not a name).
func pwFullName(f *cards.Face) string {
	for _, k := range f.Keywords {
		if cards.KeywordHead(k) != "Partner with" {
			continue
		}
		_, p, _ := strings.Cut(k, ":")
		name, _, _ := strings.Cut(p, ":")
		return strings.TrimSpace(name)
	}
	return ""
}

// pwSearch returns the lowered Partner-with enters trigger: an ETB Self
// trigger whose program is the one Search of the target player's library.
func pwSearch(lc *lCard) (*lAbility, bool) {
	for _, a := range lc.Abilities {
		if a.Kind != akTriggered || a.Trigger == nil || a.Trigger.Event != "ETB" || len(a.Prog) != 1 || a.Prog[0].Op != "Search" {
			continue
		}
		if a.Prog[0].Obj.Kind == rTarget {
			return a, true
		}
	}
	return nil, false
}

// TestPartnerWithLowersTheClass walks every corpus K:Partner with carrier
// (CR 702.124j). The keyword lowers (the commander-pair marker has no game
// characteristic), and gorge kwPartnerWith's enters trigger lowers to one
// player target slot, either seat, and the targeted player's confirmed
// search of their own library (Optional$ True: SrConfirm; the kernel's
// fetcher answers, gorge's Chooser$ Targeted) into the hand for exactly the
// full printed partner name. A carrier still refused is refused for another
// node, never for the keyword or its trigger.
func TestPartnerWithLowersTheClass(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	checked := 0
	for _, card := range reg.AllCards() {
		f := card.Faces[0]
		full := pwFullName(f)
		if full == "" {
			continue
		}
		p := &pool{byName: map[string]*cards.Card{f.Name: card}, nDecks: map[string]int{}, reg: reg}
		lc := lowerCard(p, f.Name, card, false)
		for _, r := range lc.Refusals {
			if strings.HasPrefix(r.Node, "K:Partner with") || strings.Contains(r.Reason, "ChangeType$ with ValidTgts$") ||
				strings.Contains(r.Reason, "DefinedPlayer$ Targeted") {
				t.Errorf("%s: refused %s: %s", f.Name, r.Node, r.Reason)
			}
		}
		a, ok := pwSearch(lc)
		if !ok {
			t.Errorf("%s: no enters trigger lowered to the target player's search", f.Name)
			continue
		}
		in := a.Prog[0]
		if len(a.Targets) != 1 || a.Targets[0].F == nil || a.Targets[0].F.Players != 3 || len(a.Targets[0].F.Clauses) != 0 || in.Obj.N != 0 {
			t.Errorf("%s: target slots %+v, search of %v: want one slot over either player, searched", f.Name, a.Targets, in.Obj)
			continue
		}
		if in.C != zHand || in.CS != "Confirm" || !isConstN(in.A, 1) {
			t.Errorf("%s: search into %d for %v with %q, want one card into the hand behind the confirmation", f.Name, in.C, in.A, in.CS)
		}
		if in.F == nil || len(in.F.Clauses) != 1 || in.F.Clauses[0].Name != full || in.F.Clauses[0].Types != 0 {
			t.Errorf("%s: search filter %v, want the one name %q", f.Name, in.F, full)
			continue
		}
		checked++
	}
	if checked == 0 {
		t.Fatal("no K:Partner with carrier lowered its search; the corpus pin may have moved")
	}
	t.Logf("%d K:Partner with carriers lower the target player's search for their full partner name", checked)
}

// TestTargetPlayerOwnSearchOnlyThisShape: only the targeted player's search
// of their own library that they answer themself (DefinedPlayer$ and
// Chooser$ Targeted over one ValidTgts$ Player slot, into the hand) admits a
// ChangeType$ beside ValidTgts$ for a library origin (TestPartnerWithLowersTheClass
// lowers it); every neighbouring shape is declined with nothing read, so the
// gate refuses it as before.
func TestTargetPlayerOwnSearchOnlyThisShape(t *testing.T) {
	own := map[string]string{"ValidTgts": "Player", "DefinedPlayer": "Targeted", "Chooser": "Targeted"}
	with := func(k, v string) map[string]string {
		m := map[string]string{}
		for a, b := range own {
			m[a] = b
		}
		if v == "" {
			delete(m, k)
		} else {
			m[k] = v
		}
		return m
	}
	for _, tc := range []struct {
		params       map[string]string
		origin, dest int
	}{
		{with("DefinedPlayer", ""), zLibrary, zBattlefield}, // Fertilid: the searched library is the target's, onto the battlefield
		{with("Chooser", "You"), zLibrary, zExile},          // Hide // Seek: the controller picks from the target's library
		{own, zLibrary, zBattlefield},
		{own, zLibrary, zExile},
		{own, zHand, zHand},
		{with("ValidTgts", "Opponent"), zLibrary, zHand},
		{with("Chooser", ""), zLibrary, zHand},
		{with("DefinedPlayer", "You"), zLibrary, zHand},
		{with("TargetMax", "2"), zLibrary, zHand},
		{with("OriginAlternative", "Graveyard"), zLibrary, zHand},
		{with("RememberSearcher", "True"), zLibrary, zHand},
	} {
		lc := &lowerCtx{ab: &lAbility{}}
		ps := newParams(tc.params)
		got, err := lc.targetPlayerOwnSearch(ps, tc.origin, tc.dest)
		if got || err != nil || len(lc.ab.Targets) != 0 || ps.used["Chooser"] || ps.used["TgtPrompt"] {
			t.Errorf("%v %d->%d: admitted %v (%v), %d slots, Chooser$ read %v: want declined with nothing read",
				tc.params, tc.origin, tc.dest, got, err, len(lc.ab.Targets), ps.used["Chooser"])
		}
	}
}

// The corpus carriers of the other ChangeType$-with-ValidTgts$ library
// searches keep that refusal: the target's library searched by its owner
// onto the battlefield (Fertilid, Fertilid's Favor, Turtle Tracks), with a
// graveyard alternative (Auditore Ambush), two target players (Scheming
// Symmetry), onto the top (Varragoth), the controller's library picked by an
// opponent (Gifts Ungiven).
func TestTargetedLibrarySearchesStayRefused(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	for _, name := range []string{"Fertilid", "Fertilid's Favor", "Turtle Tracks", "Auditore Ambush", "Scheming Symmetry",
		"Varragoth, Bloodsky Sire", "Gifts Ungiven"} {
		card, ok := reg.Lookup(name)
		if !ok {
			t.Fatalf("%s not in the corpus", name)
		}
		p := &pool{byName: map[string]*cards.Card{card.Faces[0].Name: card}, nDecks: map[string]int{}, reg: reg}
		lc := lowerCard(p, card.Faces[0].Name, card, false)
		found := false
		for _, r := range lc.Refusals {
			found = found || strings.Contains(r.Reason, "ChangeType$ with ValidTgts$")
		}
		if !found {
			t.Errorf("%s: refusals %v, want the ChangeType$ with ValidTgts$ refusal", name, lc.Refusals)
		}
	}
}
