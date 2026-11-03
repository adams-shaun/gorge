package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// A granted-mana prefix step on a source's own sacrificing ability (a Treasure beside a
// grant) moves the source to the graveyard at priority, and does not park the move for
// a replacement order ask as the build's and a resolution's mana steps do. The park
// only matters when two exile-instead replacements compete for the move (CR 616.1;
// graveMoveCompetes counts more than one), so grantApply is sound while no pool
// pairs a granted mana ability with two such replacements over a token. std-big's
// Rest in Peace can be doubled, but its only granter, Lotus Ring, is refused;
// std-fin's A Realm Reborn grants the Treasure its mana, but its one replacement
// over tokens is a legendary creature's (Hades, Sorcerer of Eld), which a game holds
// once, and Darkness Crystal's covers creatures. A pool that gains a pair must park
// the move in grantApply before it joins the list.
// singletonRepl: the replacement cards a game holds one of (a legendary permanent).
var singletonRepl = map[string]bool{"face:Emet-Selch, Unsundered#1": true}

func TestGrantedPrefixSacrificeHasNoReplacementPair(t *testing.T) {
	t.Parallel()
	for _, name := range cardsettest.SynthNames(t) {
		set := cardsettest.Synth(t, name)
		tb, err := set.Pool()
		if err != nil {
			t.Fatal(err)
		}
		sup := set.Supported()
		var grant, repl []string
		for i := range tb.Cards {
			c := &tb.Cards[i]
			if i < len(sup) && !sup[i] {
				continue
			}
			for j := range c.Abilities {
				a := &c.Abilities[j]
				if a.Kind == kop.AbStatic && a.Static.Kind == kop.StatContinuous && a.Static.Grant != 0 &&
					int(a.Static.Grant-1) < len(c.Abilities) && c.Abilities[a.Static.Grant-1].Kind == kop.AbMana {
					grant = append(grant, c.Name)
					break
				}
			}
			for j := range c.Abilities {
				a := &c.Abilities[j]
				if a.Kind == kop.AbReplacement && a.Repl.Kind == kop.ReplExileOnLeave && admitsArtifactToken(tb, a.Repl.Filter) {
					repl = append(repl, c.Name)
					break
				}
			}
		}
		if len(grant) > 0 && (len(repl) > 1 || len(repl) == 1 && !singletonRepl[repl[0]]) {
			t.Errorf("%s pairs granted mana %v with an exile-instead replacement over tokens %v: park grantApply's sacrifice", name, grant, repl)
		}
	}
}

// admitsArtifactToken: some alternative of f takes an artifact token.
func admitsArtifactToken(t *kop.Table, f kop.FilterID) bool {
	if f == 0 {
		return true
	}
	for ; f != 0; f = t.Filters[f].Or {
		fl := &t.Filters[f]
		if fl.Types != 0 && fl.Types&kop.TArtifact == 0 || fl.NotTypes&kop.TArtifact != 0 || fl.Flags&kop.FNontoken != 0 {
			continue
		}
		return true
	}
	return false
}
