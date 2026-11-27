package adapter

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestCombinedNameCardsResolveInPool: a split, adventure or modal card is found
// under the "A // B" name its pool keys, not only under its front face.
func TestCombinedNameCardsResolveInPool(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	set, err := cardset.ByName("hosted-cmdr")
	if err != nil {
		t.Skip(err)
	}
	e, err := New([]string{"hosted-cmdr"}, reg, dirac.SurfaceFull)
	if err != nil {
		t.Skip(err)
	}
	for _, name := range []string{
		"Murderous Rider // Swift End", // adventure
		"Wear // Tear",                 // split
		"Dusk // Dawn",                 // split
		"Heaven // Earth",
		"Sephiroth, Fabled SOLDIER // Sephiroth, One-Winged Angel", // modal
	} {
		if _, ok := set.Lookup(name); !ok {
			t.Fatalf("precondition: pool hosted-cmdr does not key %q", name)
		}
		c, ok := reg.Lookup(name)
		if !ok || len(c.Faces) < 2 {
			t.Fatalf("precondition: registry card %q (found %v) needs several faces", name, ok)
		}
		if _, ok := set.Lookup(frontName(c)); ok {
			t.Fatalf("precondition: the pool also keys the front name of %q", name)
		}
		if got := poolName(set, c); got != name {
			t.Errorf("poolName = %q, want %q", got, name)
		}
		es := entries(set, []*cards.Card{c, c})
		if len(es) != 1 || es[0].Name != name || es[0].Count != 2 {
			t.Errorf("entries(%q) = %+v, want one entry of 2", name, es)
		}
		if err := e.CheckDeck([]*cards.Card{c}); err != nil && strings.Contains(err.Error(), "is not in the pool") {
			t.Errorf("%q: %v", name, err)
		}
	}
}
