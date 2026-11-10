package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// fbGrantCard lowers and builds a pool of a creature carrying the given static
// lines and a Lesson instant (the pool's vocabulary then names the subtype).
func fbGrantCard(t *testing.T, statics ...string) (*lCard, []built, error) {
	t.Helper()
	src := "Name:Grave Tutor Probe\nManaCost:3 U R\nTypes:Legendary Creature Human\nPT:4/4\n"
	for _, s := range statics {
		src += s + "\n"
	}
	byName := map[string]*cards.Card{}
	for _, s := range []string{src, "Name:Study Probe\nManaCost:2 R\nTypes:Instant Lesson\nA:SP$ Draw | NumCards$ 1 | SpellDescription$ Draw a card.\n"} {
		card, diags := cards.ParseBytes("probe.txt", []byte(s))
		if card == nil {
			t.Fatalf("parse: %v", diags)
		}
		if d := card.Link(); len(d) != 0 {
			t.Fatalf("link: %v", d)
		}
		byName[card.Faces[0].Name] = card
	}
	p := &pool{names: []string{"Grave Tutor Probe", "Study Probe"}, byName: byName, nDecks: map[string]int{}, reg: &cards.Registry{}}
	lw := lowerPool(p)
	_, bs, _, err := buildPool(lw)
	return lw.cards[0], bs, err
}

// Two graveyard flashback grants in the shape Iroh, Grand Lotus prints: one at
// each card's mana cost, one at a fixed {1}, both only on your turn.
const (
	fbGrantOwn   = "S:Mode$ Continuous | Affected$ Instant.nonLesson+YouOwn,Sorcery.nonLesson+YouOwn | AffectedZone$ Graveyard | AddKeyword$ Flashback | Condition$ PlayerTurn | Description$ Your turn: your plain instants and sorceries in the graveyard have flashback."
	fbGrantFixed = "S:Mode$ Continuous | Affected$ Lesson.YouOwn | AffectedZone$ Graveyard | AddKeyword$ Flashback:1 | Condition$ PlayerTurn | Description$ Your turn: your Lessons in the graveyard have flashback {1}."
)

// TestLowerAffzoneGraveFlashback: a Continuous static granting flashback to
// cards in its controller's graveyard lowers to kop.StatFbGrant (the kernel
// reads it live, kw_fbgrant.go), the parameterless grant at the card's mana
// cost and the "Flashback:1" grant with its own {1} (StFbOwnCost), both under
// the your-turn condition; Iroh, Grand Lotus is that pair. The shapes the
// kernel does not read still refuse by name.
func TestLowerAffzoneGraveFlashback(t *testing.T) {
	lc, bs, err := fbGrantCard(t, fbGrantOwn, fbGrantFixed)
	if err != nil {
		t.Fatal(err)
	}
	if len(lc.Refusals) != 0 {
		t.Fatalf("refusals %v, want none", lc.Refusals)
	}
	var sts []kop.Static
	for _, a := range bs[0].def.Abilities {
		if a.Kind == kop.AbStatic && a.Static.Kind == kop.StatFbGrant {
			sts = append(sts, a.Static)
		}
	}
	if len(sts) != 2 {
		t.Fatalf("FbGrant statics %+v, want two", sts)
	}
	own, fixed := sts[0], sts[1]
	if own.StFlags&kop.StFbOwnCost != 0 || own.GrantCost != (kop.ManaCost{}) {
		t.Errorf("mana-cost grant %+v: want no cost of its own", own)
	}
	if fixed.StFlags&kop.StFbOwnCost == 0 || fixed.GrantCost != (kop.ManaCost{Generic: 1}) {
		t.Errorf("fixed grant %+v: want StFbOwnCost and {1}", fixed)
	}
	for _, s := range sts {
		if s.Filter == 0 || s.Cond2 == 0 {
			t.Errorf("grant %+v: want a filter and the your-turn condition", s)
		}
	}
	if own.Filter == fixed.Filter {
		t.Errorf("both grants share filter %d, want the non-Lesson and the Lesson filters", own.Filter)
	}
	for _, c := range []struct{ name, line, want string }{
		{"opponent's graveyard", strings.Replace(fbGrantOwn, "+YouOwn", "+OppOwn", 2), "not your graveyard"},
		{"another keyword", strings.Replace(fbGrantOwn, "AddKeyword$ Flashback", "AddKeyword$ Escape", 1), "AffectedZone \"Graveyard\" without MayPlay"},
		{"effect zone", fbGrantOwn + " | EffectZone$ Graveyard", "EffectZone"},
		{"x cost", strings.Replace(fbGrantFixed, "Flashback:1", "Flashback:X", 1), "flashback grant cost"},
		{"non-mana cost", strings.Replace(fbGrantFixed, "Flashback:1", "Flashback:Sac<1/Creature>", 1), "flashback grant cost"},
		{"pump beside", fbGrantOwn + " | AddPower$ 1", "beside AddPower"},
	} {
		lc, _, _ := fbGrantCard(t, c.line)
		found := false
		for _, r := range lc.Refusals {
			found = found || strings.Contains(r.Reason, c.want)
		}
		if !found {
			t.Errorf("%s: refusals %v, want one naming %q", c.name, lc.Refusals, c.want)
		}
	}
}
