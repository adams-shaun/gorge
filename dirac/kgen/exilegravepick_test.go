package kgen

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
)

// cloneSAChain deep-copies an ability and its Sub chain. Link resolved the
// chain from the SVar text at load, so the corpus card's nodes are shared;
// the test edits its own copy.
func cloneSAChain(sa *cards.SA) *cards.SA {
	if sa == nil {
		return nil
	}
	n := *sa
	n.Params = make(map[string]string, len(sa.Params))
	for k, v := range sa.Params {
		n.Params[k] = v
	}
	n.Sub = cloneSAChain(sa.Sub)
	return &n
}

// withDBReturnParam clones Binding Negotiation and adds one parameter to the
// linked DBReturn ChangeZone node (the node the SVar names), so the test
// proves the guard sees it rather than an unlinked SVar edit.
func withDBReturnParam(t *testing.T, p *pool, key, value string) {
	t.Helper()
	c := kfhClone(p.byName["Binding Negotiation"])
	for i, a := range c.Faces[0].Abilities {
		c.Faces[0].Abilities[i] = cloneSAChain(a)
	}
	edited := false
	for _, a := range c.Faces[0].Abilities {
		for n := a; n != nil; n = n.Sub {
			if n.API != "ChangeZone" || n.Params["Origin"] != "Exile" || n.Params["Destination"] != "Graveyard" {
				continue
			}
			n.Params[key] = value
			edited = true
		}
	}
	if !edited {
		t.Fatalf("Binding Negotiation has no linked Exile->Graveyard ChangeZone node")
	}
	p.byName["Binding Negotiation"] = c
}

// An Origin$ Exile -> Destination$ Graveyard hidden pick is lowered only as
// the game-wide walk: the kernel's exwPickCands always lists every seat's
// exile, so a scoping parameter that makes gorge effHiddenPick narrow the
// pool to one player's zone would pose a decision the two engines answer
// differently. The pick refuses each such spelling by name.
func TestExileGravePickRefusesScopedPool(t *testing.T) {
	for _, tc := range []struct{ name, value string }{
		{"ValidTgts", "Opponent"},
		{"DefinedPlayer", "Targeted"},
		{"Chooser", "You"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			kp := kfhBuild(t, []string{"Binding Negotiation"}, func(p *pool) {
				withDBReturnParam(t, p, tc.name, tc.value)
			})
			kp.refusedWith(t, "Binding Negotiation", "exile pick "+tc.name+"$: the kernel's exile walk is game-wide")
		})
	}
}
