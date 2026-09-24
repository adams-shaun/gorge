package rules

import (
	"sort"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/effects"
	"github.com/adams-shaun/gorge/internal/testutil"
)

// TestValueHeadRegistryMatchesEvaluator keeps effects' modelledValueHeads
// list -- the "count:<head>" primitives the supported-card gate
// (cards.Registry.Unsupported) checks -- honest in BOTH directions against
// the evaluator itself: every referenced value SVar body in the corpus is
// evaluated with effects.EvalCountOK against a real engine, and a head is
// modelled exactly when at least one of its corpus bodies resolves. A head
// that resolves but is unlisted keeps playable cards out of the pool; a
// listed head that never resolves puts cards with an unreadable gate into
// it (the fuzz-cov3 audit's registration gap). Both are named.
func TestValueHeadRegistryMatchesEvaluator(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	e := layerEngine(t)
	resolves := map[string]bool{}
	seen := map[string]bool{}
	for _, c := range reg.Cards {
		for _, f := range c.Faces {
			if f.Name == "" || len(f.SVars) == 0 {
				continue
			}
			referenced := map[string]bool{}
			for _, h := range f.ValueHeads() {
				referenced[strings.TrimPrefix(h, cards.ValueHeadPrefix)] = true
			}
			if len(referenced) == 0 {
				continue
			}
			// An off-zone object: the source a count reads, without a
			// battlefield presence that would grow every later census.
			id := e.G.AddObject(c, 0).ID
			names := make([]string, 0, len(f.SVars))
			for name := range f.SVars {
				names = append(names, name)
			}
			sort.Strings(names)
			for _, name := range names {
				body := f.SVars[name]
				head, ok := cards.ValueHead(body)
				if !ok || !referenced[head] {
					continue
				}
				seen[head] = true
				if resolves[head] {
					continue
				}
				ctx := &effects.Ctx{Source: id, Controller: 0, SVars: f.SVars}
				if _, ok := effects.EvalCountOK(e, ctx, strings.TrimSpace(body)); ok {
					resolves[head] = true
				}
			}
		}
	}
	listed := map[string]bool{}
	for _, h := range effects.ModelledValueHeads() {
		listed[h] = true
	}
	sup := effects.Supported()
	var unlisted, stale []string
	for h := range seen {
		if resolves[h] && !listed[h] {
			unlisted = append(unlisted, h)
		}
		if !resolves[h] && listed[h] {
			stale = append(stale, h)
		}
	}
	for h := range listed {
		if !seen[h] {
			stale = append(stale, h+" (no corpus carrier)")
		}
		if !sup[cards.ValueHeadPrefix+h] {
			t.Errorf("modelled head %q is not registered in effects.Supported", h)
		}
	}
	sort.Strings(unlisted)
	sort.Strings(stale)
	if len(unlisted) > 0 {
		t.Errorf("count heads the evaluator resolves but effects.modelledValueHeads omits (add them):\n  %s", strings.Join(unlisted, "\n  "))
	}
	if len(stale) > 0 {
		t.Errorf("count heads effects.modelledValueHeads lists but no corpus body resolves (remove them):\n  %s", strings.Join(stale, "\n  "))
	}
}
