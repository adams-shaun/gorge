package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// untapLockOps returns the C of each SkipUntap a lowered card carries.
func untapLockOps(lc *lCard) []int {
	var cs []int
	for _, ab := range lc.Abilities {
		for _, in := range ab.Prog {
			if in.Op == "SkipUntap" {
				cs = append(cs, in.C)
			}
		}
	}
	return cs
}

// TestLowerEffectReplDuration: the Effect that locks a remembered permanent's
// untap for as long as its controller controls the source (Duration$
// AsLongAsControl) lowers to OpSkipUntap C kop.SkipUntapLockStep (the untap
// step's: Ty Lee, Chi Blocker) or kop.SkipUntapLockAny (every untap event:
// Spider-Woman, Secret Agent). Each out-of-shape variant still refuses by name.
func TestLowerEffectReplDuration(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	for _, c := range []struct {
		name string
		want int
	}{
		{"Ty Lee, Chi Blocker", kop.SkipUntapLockStep},
		{"Spider-Woman, Secret Agent", kop.SkipUntapLockAny},
	} {
		card, ok := reg.Lookup(c.name)
		if !ok || card == nil {
			t.Fatalf("%s not in the corpus", c.name)
		}
		p := &pool{byName: map[string]*cards.Card{card.Faces[0].Name: card}, nDecks: map[string]int{}, reg: reg}
		lc := lowerCard(p, card.Faces[0].Name, card, false)
		if got := keclReason(lc.Refusals); got != "" {
			t.Errorf("%s: refusals %q, want none", c.name, got)
		}
		if got := untapLockOps(lc); len(got) != 1 || got[0] != c.want {
			t.Errorf("%s: SkipUntap C %v, want [%d]", c.name, got, c.want)
		}
	}

	const base = "Name:Exile Bf Probe\nManaCost:2 U\nTypes:Creature Human\nPT:2/2\n" +
		"T:Mode$ ChangesZone | Origin$ Any | Destination$ Battlefield | ValidCard$ Card.Self | Execute$ TrigT | TriggerDescription$ probe\n" +
		"SVar:TrigT:DB$ Tap | ValidTgts$ Creature | SubAbility$ DBLock\n" +
		"SVar:DBLock:DB$ Effect | RememberObjects$ Targeted | ForgetOnMoved$ Battlefield | ReplacementEffects$ NoUntap | Duration$ AsLongAsControl\n" +
		"SVar:NoUntap:Event$ Untap | ValidCard$ Card.IsRemembered | ValidStepTurnToController$ You | Layer$ CantHappen | Description$ probe\n"
	if r := exileBfSynth(t, base); len(r) != 0 {
		t.Fatalf("the probe script itself refuses: %q", keclReason(r))
	}
	if got := untapLockOps(untapLockProbe(t, base)); len(got) != 1 || got[0] != kop.SkipUntapLockStep {
		t.Fatalf("the probe lowers to SkipUntap C %v, want [%d]", got, kop.SkipUntapLockStep)
	}
	for _, c := range []struct{ name, from, to, want string }{
		{"other duration", "Duration$ AsLongAsControl", "Duration$ UntilEndOfTurn", "Effect replacement Duration$"},
		{"no forget", "ForgetOnMoved$ Battlefield | ", "", "Effect AsLongAsControl untap ForgetOnMoved$"},
		{"forget exile", "ForgetOnMoved$ Battlefield", "ForgetOnMoved$ Exile", "Effect AsLongAsControl untap ForgetOnMoved$"},
		{"exile on moved", "ForgetOnMoved$ Battlefield", "ForgetOnMoved$ Battlefield | ExileOnMoved$ Battlefield", "Effect AsLongAsControl untap ExileOnMoved$"},
		{"remember self", "RememberObjects$ Targeted", "RememberObjects$ Self", "Effect AsLongAsControl untap RememberObjects"},
		{"other step scope", "ValidStepTurnToController$ You", "ValidStepTurnToController$ Player.Activator", "Effect AsLongAsControl untap replacement ValidStepTurnToController$"},
		{"not the remembered", "ValidCard$ Card.IsRemembered", "ValidCard$ Creature.YouCtrl", "Effect AsLongAsControl untap replacement shape"},
		{"extra key", "Layer$ CantHappen", "Layer$ CantHappen | ActiveZones$ Battlefield", "Effect AsLongAsControl untap replacement unread"},
		{"not cant-happen", "Layer$ CantHappen", "Layer$ Other", "Effect AsLongAsControl untap replacement shape"},
		{"two lines", "ReplacementEffects$ NoUntap", "ReplacementEffects$ NoUntap,NoUntap", "Effect replacement Duration$"},
	} {
		src := strings.Replace(base, c.from, c.to, 1)
		if src == base {
			t.Fatalf("%s: variant did not change the script", c.name)
		}
		if got := keclReason(exileBfSynth(t, src)); !strings.Contains(got, c.want) {
			t.Errorf("%s: refusals %q, want %q", c.name, got, c.want)
		}
	}

	// A spell's source is gone when it resolves: the lock refuses there.
	spell := "Name:Exile Bf Probe\nManaCost:2 U\nTypes:Sorcery\nA:SP$ Tap | ValidTgts$ Creature | SubAbility$ DBLock | SpellDescription$ probe\n" +
		"SVar:DBLock:DB$ Effect | RememberObjects$ Targeted | ForgetOnMoved$ Battlefield | ReplacementEffects$ NoUntap | Duration$ AsLongAsControl\n" +
		"SVar:NoUntap:Event$ Untap | ValidCard$ Card.IsRemembered | Layer$ CantHappen | Description$ probe\n"
	if got := keclReason(exileBfSynth(t, spell)); !strings.Contains(got, "untap lock outside a triggered or activated ability") {
		t.Errorf("spell: refusals %q", got)
	}
	// With no step scope the lock is the any-event form.
	anyForm := strings.Replace(base, "ValidStepTurnToController$ You | ", "", 1)
	if got := untapLockOps(untapLockProbe(t, anyForm)); len(got) != 1 || got[0] != kop.SkipUntapLockAny {
		t.Errorf("no step scope: SkipUntap C %v, want [%d]", got, kop.SkipUntapLockAny)
	}
}

// untapLockProbe lowers the synthetic probe card src.
func untapLockProbe(t *testing.T, src string) *lCard {
	t.Helper()
	card, diags := cards.ParseBytes("exilebf_probe.txt", []byte(src))
	if card == nil {
		t.Fatalf("parse: %v", diags)
	}
	if d := card.Link(); len(d) != 0 {
		t.Fatalf("link: %v", d)
	}
	const name = "Exile Bf Probe"
	p := &pool{names: []string{name}, byName: map[string]*cards.Card{name: card},
		nDecks: map[string]int{}, reg: &cards.Registry{}}
	return lowerCard(p, name, card, false)
}
