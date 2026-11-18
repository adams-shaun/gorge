package oracle

import (
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// TestMBCFiltSearchRestrictedDedupReprosLockstep pins two tail-42 games where
// Gwenna, Eyes of Gaea (any colour, spent only on creature spells and creature
// abilities, CR 106.6) and Agent of Stromgald ({R}: Add {B}) sit beside an
// activation neither Gwenna nor the lands pay alone: Phyrexian Totem's animate
// and Galadhrim Bow's equip. Gwenna's restricted {R} pays the Agent's fee and
// the Agent's {B} is unrestricted, so gorge offers the activation. The
// kernel's filter search reached that line but dropped it as a duplicate:
// its pool-by-colour state key cannot tell Gwenna's restricted {B}{B} from
// the Agent's converted {B}, and the earlier unpriced sibling was keyed first.
// The search now prices a node before it consults the visited set, as gorge's
// scriptSearch.visit offers before it dedups.
//
// The _taken recording is the Totem game with its last action repeated: the
// replay activates the Totem a second time, so the kernel must also pay it.
func TestMBCFiltSearchRestrictedDedupReprosLockstep(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name, payee string
		at          int // the decision the kernel under-offered at
	}{
		{"filtsearch_priced_state_beside_unpriced_dup", "Phyrexian Totem", 363},
		{"filtsearch_priced_state_beside_unpriced_dup_taken", "Phyrexian Totem", 363},
		{"filtsearch_priced_state_beside_unpriced_dup_equip", "Galadhrim Bow", 425},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			filtDedupRepro(t, c.name, c.payee, c.at)
		})
	}
}

func filtDedupRepro(t *testing.T, name, payee string, at int) {
	t.Helper()
	r, err := ReadRepro(filepath.Join("testdata", "repro", name+".repro.json"))
	if err != nil {
		t.Fatal(err)
	}
	script := &ScriptPolicy{Actions: r.Actions}
	board, offered := false, false
	pol := PolicyFunc(func(c *PickCtx) Canon {
		if c.G != nil && c.Seq == at-1 {
			board = filtDedupBoard(c.G.E.G, payee)
		}
		if c.G != nil && c.Seq == at {
			for _, a := range c.D.Options {
				offered = offered || (a.Kind() == ActActivate && refCardName(c.G, a.Src()) == payee)
			}
		}
		return script.Pick(c)
	})
	res := replayDecodedScript(t, synthReg(t), globSet(t, r.Pool, r), r, pol)
	// The precondition: the decision before the under-offer has one seat
	// holding Gwenna, an Agent of Stromgald and the payee on the battlefield.
	if !board {
		t.Fatalf("precondition: decision %d has no seat with Gwenna, Eyes of Gaea, Agent of Stromgald and %s on the battlefield", at-1, payee)
	}
	if res.Div != nil {
		t.Fatalf("divergence at %d of %d: %s %s %v (gorge %v kernel %v)", res.Div.At, len(r.Actions), res.Div.What, res.Div.Field, res.Div.Diff, res.Div.RefOptions, res.Div.KerOptions)
	}
	if msg := reproSkipFailure(r, script); msg != "" {
		t.Fatal(msg)
	}
	if !offered {
		t.Fatalf("decision %d offered no %s activation: the replay no longer reaches the play", at, payee)
	}
	stallCheck(t, r, res, false)
}

// filtDedupBoard reports whether one seat controls Gwenna, an Agent of
// Stromgald and the payee on the battlefield.
func filtDedupBoard(g *state.Game, payee string) bool {
	var seen [2]map[string]bool
	for i := range g.Objs {
		o := &g.Objs[i]
		if o.Zone != state.ZBattlefield || o.Face() == nil || int(o.Controller) >= len(seen) {
			continue
		}
		if seen[o.Controller] == nil {
			seen[o.Controller] = map[string]bool{}
		}
		seen[o.Controller][o.Face().Name] = true
	}
	for _, s := range seen {
		if s["Gwenna, Eyes of Gaea"] && s["Agent of Stromgald"] && s[payee] {
			return true
		}
	}
	return false
}
