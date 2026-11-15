package kgen

import (
	"sort"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
)

// markSplitShapes are the script keys on which the kernel's persistent
// remembered mark (kop.CondRemMark, a bare non-empty bit per source) and
// gorge's o.Remembered member list can part ways:
//
//  1. ForgetChanged$ / ForgetPlayed$ (gorge forgetRememberedOne) empties the
//     list while the mark stays standing;
//  2. RememberLKI$ without RememberChanged$ (gorge zone_change.go: ctx only)
//     sets the mark where gorge persists nothing.
//
// Either only shows on an UNRESTRICTED ConditionDefined$ Remembered gate, the
// one lowering ORs the mark into (lower_effect.go nodeConditionCore).
var markSplitShapes = []string{"ForgetChanged", "ForgetPlayed", "RememberLKI"}

// markSplitBenign is the ratchet over the corpus: every lowered card that
// carries the mark gate beside a markSplitShapes write, and why the split is
// not observable on it. It only shrinks; a card that joins it needs the
// membership model (a per-object count in the cold arena decremented by
// forgetChanged, or write sites that carry the persistent flavour), not a new
// entry.
//
// Each entry is the Navigation Orb / Troop of Ponies shape: the ability
// sacrifices its source as a cost (no later resolution of the same source can
// read a stale mark) and its chain closes with Cleanup ClearRemembered$ (the
// mark dies with the list, rememberMarkClr). The only reading of the split is
// inside one resolution, where the gated node's Defined$ Remembered is the
// empty set (the one searched land was already forgotten by the node before
// it), so the extra pass moves nothing.
var markSplitBenign = map[string]string{
	"A-Navigation Orb": "sacrifices itself; ClearRemembered$ closes the chain; the gated move's set is empty",
	"Navigation Orb":   "sacrifices itself; ClearRemembered$ closes the chain; the gated move's set is empty",
	"Troop of Ponies":  "sacrifices itself; ClearRemembered$ closes the chain; the gated move's set is empty",
}

func TestRememberedGateShapeCensus(t *testing.T) {
	p, err := loadCardList(testSetlistPath(t, "ALL"), ratchetCards(t))
	if err != nil {
		t.Fatal(err)
	}
	var hasMark func(c lCond) bool
	hasMark = func(c lCond) bool {
		if c.Kind == cRemMark {
			return true
		}
		for _, s := range c.Sub {
			if hasMark(s) {
				return true
			}
		}
		return false
	}
	gates := 0
	found := map[string][]string{}
	for _, c := range lowerPool(p).cards {
		if len(c.Refusals) != 0 {
			continue
		}
		gate := false
		for _, a := range c.Abilities {
			for _, in := range a.Prog {
				gate = gate || hasMark(in.Cond)
			}
		}
		if !gate {
			continue
		}
		gates++
		var cs census
		for _, f := range p.byName[c.Name].Faces {
			cs.face(c.Name, f)
		}
		for l, set := range cs.use {
			for _, k := range markSplitShapes {
				if set[c.Name] && strings.HasSuffix(l, " "+k) {
					found[c.Name] = append(found[c.Name], l)
				}
			}
		}
	}
	if gates == 0 {
		t.Fatal("no lowered card carries the persistent-remembered mark gate: the census reads nothing")
	}
	var names []string
	for n := range found {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		why, ok := markSplitBenign[n]
		if !ok {
			sort.Strings(found[n])
			t.Errorf("%s: the mark gate beside %v is not in markSplitBenign: the mark holds non-emptiness only, "+
				"so a later resolution can take a gate gorge's emptied list refuses; implement the membership model "+
				"(dirac/kgen/opsdoc.go cond:RemMark), do not add an entry", n, found[n])
			continue
		}
		_ = why
		if !sacrificesSelfAndClears(p.byName[n]) {
			t.Errorf("%s: markSplitBenign says it sacrifices itself and ends in ClearRemembered$, its script no longer does", n)
		}
	}
	for n := range markSplitBenign {
		if _, ok := found[n]; !ok {
			t.Errorf("markSplitBenign lists %s, which no longer carries the gate beside a split shape: delete the entry", n)
		}
	}
	t.Logf("%d lowered cards carry the mark gate; %d beside a split write shape, all pinned", gates, len(names))
}

// sacrificesSelfAndClears: a face ability pays Sac<1/CARDNAME> and some SVar
// of the same face runs ClearRemembered$ True.
func sacrificesSelfAndClears(c *cards.Card) bool {
	for _, f := range c.Faces {
		sac := false
		for _, a := range f.Abilities {
			sac = sac || strings.Contains(a.Params["Cost"], "Sac<1/CARDNAME>")
		}
		clr := false
		for name := range f.SVars {
			if sa := cards.ResolveSVar(f.SVars, name); sa != nil && sa.Params["ClearRemembered"] == "True" {
				clr = true
			}
		}
		if sac && clr {
			return true
		}
	}
	return false
}
