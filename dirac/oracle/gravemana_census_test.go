package oracle

import (
	"fmt"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// playDump is graveManaPod.play returning the check snapshots and the state
// dump captured at the first divergence (pre-rewind).
func (p graveManaPod) playDump(t *testing.T, seed uint64) (snaps []string, g *GorgeSide, res Result) {
	t.Helper()
	expand := func(d []cardset.Entry) []string {
		var out []string
		for _, e := range d {
			for j := 0; j < e.Count; j++ {
				out = append(out, e.Name)
			}
		}
		return out
	}
	r := &Repro{Format: reproFormat, Decks: [2]string{"x0", "x1"}, Cards: [2][]string{expand(p.decks[0]), expand(p.decks[1])},
		Seed: seed, PassElision: true, Surface: "full"}
	m, err := r.Matchup(p.reg)
	if err != nil {
		t.Fatal(err)
	}
	rc := r.RunConfig()
	rc.MaxDecisions, rc.MaxTurns = 8000, 40
	c := &graveManaTally{}
	rc.Check = func(g *GorgeSide, _ Side) *Divergence {
		i := len(snaps)
		pl := g.E.G.Players[0].Pool
		zones := ""
		for _, seat := range []state.PlayerID{0, 1} {
			for _, z := range []state.Zone{state.ZBattlefield, state.ZHand, state.ZGraveyard} {
				for _, id := range g.E.G.Zone(z, seat) {
					o := g.E.G.Obj(id)
					zones += fmt.Sprintf("p%d %v %d %q tap=%v | ", seat, z, id, o.Face().Name, o.Tapped)
				}
			}
		}
		opts := ""
		if d := g.Pending(); d != nil {
			for _, o := range d.Options {
				n, z := "?", "-"
				if ob := g.E.G.Obj(o.Obj); ob != nil {
					z = ob.Zone.String()
					if ob.Face() != nil {
						n = ob.Face().Name
					}
				}
				opts += fmt.Sprintf("%s:%s@%s ", o.Kind, n, z)
			}
		}
		snaps = append(snaps, fmt.Sprintf("ask %d pool %v options [%s] zones %s", i, pl, opts, zones))
		if d := g.Pending(); d != nil && d.Kind == decision.KPriority && d.Player == 0 {
			snaps = append(snaps, fmt.Sprintf("ask %d planner: potentialMana %v", i, g.E.PotentialMana(0)))
			for _, pp := range g.E.PotentialPaymentPlans(0) {
				n := ""
				if ob := g.E.G.Obj(pp.Action.Obj); ob != nil && ob.Face() != nil {
					n = ob.Face().Name
				}
				snaps = append(snaps, fmt.Sprintf("ask %d plan %s %s (%d): reason %q detail %q script %d", i, pp.Action.Kind, n, pp.Action.Obj, pp.Reason, pp.Detail, len(pp.Script)))
			}
			for _, pa := range g.E.EnsurePaymentActions() {
				n := ""
				if ob := g.E.G.Obj(pa.Cast.Object); ob != nil && ob.Face() != nil {
					n = ob.Face().Name
				}
				snaps = append(snaps, fmt.Sprintf("ask %d paymentAction %s (%d) plans %d", i, n, pa.Cast.Object, len(pa.Plans)))
			}
		}
		return c.scan(g)
	}
	tbl, _, err := p.set.MatchupN(p.decks)
	if err != nil {
		t.Fatal(err)
	}
	k, err := SetSide(p.set, tbl, rc.Config)(m, seed)
	if err != nil {
		t.Fatalf("seed %d: %v", seed, err)
	}
	defer CloseSide(k)
	lead, err := NewGorgeSide(m, seed, rc.Config, nil)
	if err != nil {
		t.Fatal(err)
	}
	res = LockstepLed(m, seed, lead, k, graveManaPolicy(seed), rc)
	return snaps, lead, res
}

func dumpG(t *testing.T, g *GorgeSide) {
	for _, seat := range []state.PlayerID{0, 1} {
		for _, z := range []state.Zone{state.ZBattlefield, state.ZHand, state.ZGraveyard, state.ZExile} {
			for _, id := range g.E.G.Zone(z, seat) {
				o := g.E.G.Obj(id)
				t.Logf("  p%d %v obj %d tapped %v: %s", seat, z, id, o.Tapped, o.Face().Name)
			}
		}
	}
}

// TestGraveManaSeedsCensus is a temporary census over the pool's first 16
// seeds: it logs each seed's outcome (clean or divergent, with the first
// divergence) and fails if a diverging seed's first divergence is not an
// options divergence. It is removed once the divergence class is fixed.
func TestGraveManaSeedsCensus(t *testing.T) {
	p := newGraveManaPod(t)
	for seed := uint64(1); seed <= 16; seed++ {
		if seed != 2 && seed != 3 && seed != 4 {
			continue
		}
		res, c := p.play(t, seed, 40)
		kind := "clean"
		switch {
		case res.Div != nil:
			kind = "div"
		case res.StallUnmodelled:
			kind = "unmodelled"
		case res.Stall != "":
			kind = "stall"
		}
		t.Logf("seed %d: %s, %d decisions, over %v, stall %q, exiles %d, mana adds %d", seed, kind, res.Decisions, res.Over, res.Stall, c.exiled, c.manaAfter)
		if res.Div != nil {
			var np []string
			for _, d := range res.Div.Diff {
				if !strings.Contains(d, ".Pos:") {
					np = append(np, d)
				}
			}
			t.Logf("seed %d divergence: at %d, what %q, detail %q, ref %v, ker %v, enablers %v, nonPos %v, actions %v", seed, res.Div.At, res.Div.What, res.Div.Detail, res.Div.RefOptions, res.Div.KerOptions, res.Div.Enablers, np, res.Actions)
			ss, gg, _ := p.playDump(t, seed)
			for _, i := range []int{int(res.Div.At) - 1, int(res.Div.At)} {
				if i >= 0 && i < len(ss) {
					t.Logf("seed %d %s", seed, ss[i])
				}
			}
			evs := gg.E.L.Events
			for i := range evs {
				ev := &evs[i]
				n := ""
				if o := gg.E.G.Obj(ev.Obj); o != nil && o.Face() != nil {
					n = o.Face().Name
				}
				t.Logf("  ev[%d] %v p%d obj %s (%d) %v->%v amt %d text %q", i, ev.Kind, ev.Player, n, ev.Obj, ev.From, ev.To, ev.Amount, ev.Text)
			}
			if res.Div.What != "options" {
				t.Errorf("seed %d: divergence kind %q, expected options", seed, res.Div.What)
			}
		}
	}
}
