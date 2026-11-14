package oracle

import (
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// A depletion rider beside a counter-cost gap: gorge's potential planner
// prices a cast only a deferred source's activation funds (rules
// potential_plan.go: the census misses a deplete land, the relaxed proof
// admits the search, potentialPrefixScript finds the prefix), and the kernel
// must offer it beside it. Pair 39 (dr-mbc-bb, seed 17133499884003748923)
// diverged at decision 700 on gorge's Cast Relic Crush {4}{G}, funded by a
// prefix activating Peat Bog (a deplete rider) beside Wall of Roots (a
// counter-cost gap): the kernel's script search routed a board whose gaps are
// a Taps-deferred one beside counter-cost ones to the tap-another search,
// whose gaps (rs.gaps, mana_tapanother.go) hold no Taps-deferred source, and
// listed only the pass.
const riderCtrGapRepro = "rider_beside_ctr_gap_peat_bog"

// TestRiderBesideCtrGapReplay replays the recorded game (both engines offer
// the same options at every decision) and, at the decision where gorge prices
// the scripted cast, requires the kernel's own priority decision to list the
// same cast.
func TestRiderBesideCtrGapReplay(t *testing.T) {
	t.Parallel()
	r, err := ReadRepro(filepath.Join("testdata", "repro", riderCtrGapRepro+".repro.json"))
	if err != nil {
		t.Fatal(err)
	}
	reg := synthReg(t)
	set := globSet(t, r.Pool, r)
	m, err := r.Matchup(reg)
	if err != nil {
		t.Fatal(err)
	}
	tbl, _, _, err := set.Matchup(entryList(r.Cards[0]), entryList(r.Cards[1]))
	if err != nil {
		t.Fatal(err)
	}
	rc := r.RunConfig()
	rc.MaxDecisions = len(r.Actions) + 200
	pinned := false
	rc.Check = func(ref *GorgeSide, k Side) *Divergence {
		d := ref.Pending()
		if d == nil || d.Kind != decision.KPriority {
			return nil
		}
		for _, pp := range ref.E.PotentialPaymentPlans(d.Player) {
			if pp.Action.Kind != "cast" || pp.Action.Label != "Cast Relic Crush" {
				continue
			}
			// gorge's prefix activates the deplete land (Peat Bog), whose
			// rider sacrifices it once its last counter leaves (CR 605.3b):
			// the play the posed priority never lists.
			scripts := false
			for _, st := range pp.Script {
				for _, pk := range st.Picks {
					if o := ref.E.G.Obj(pk.Obj); o != nil && o.Face() != nil && o.Face().Name == "Peat Bog" {
						scripts = true
					}
				}
			}
			if !scripts {
				return &Divergence{What: "script", Field: "script",
					Detail: "gorge's Cast Relic Crush plan does not activate Peat Bog"}
			}
			// The same cast on the kernel side, as a canon: a cast of the hand
			// card, whatever its mode bits.
			var hand state.ObjID
			for _, id := range ref.E.G.Zone(state.ZHand, d.Player) {
				if o := ref.E.G.Obj(id); o != nil && o.Face() != nil && o.Face().Name == "Relic Crush" {
					hand = id
				}
			}
			if hand == 0 {
				return &Divergence{What: "state", Field: "hand", Detail: "Relic Crush not in hand at the priced cast"}
			}
			want := Pack(ActCast, ref.InstRef(hand), RefNone, 0)
			_, _, kopts := KernelStateOf(k)
			if kopts == nil {
				return &Divergence{What: "options", Field: "kernel", Detail: "not the real kernel side"}
			}
			for _, c := range kopts {
				if c.Kind() == want.Kind() && c.Src() == want.Src() {
					pinned = true
					return nil // the kernel offers the same cast
				}
			}
			return &Divergence{What: "options", Field: "options",
				Detail: "gorge prices Cast Relic Crush through the deplete land's activation, the kernel lists only the pass"}
		}
		return nil
	}
	k, err := SetSide(set, tbl, rc.Config)(m, r.Seed)
	if err != nil {
		t.Fatal(err)
	}
	defer CloseSide(k)
	res := Lockstep(m, r.Seed, k, &ScriptPolicy{Actions: r.Actions}, rc)
	if res.Div != nil {
		t.Fatalf("divergence at %d: %s %s %v", res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
	}
	stallCheck(t, r, res, false)
	if !pinned {
		t.Fatal("the replay never reaches gorge's Cast Relic Crush plan funded by the deplete land")
	}
}

// entryList is a repro deck's card list as Matchup's entries.
func entryList(names []string) []cardset.Entry {
	es := make([]cardset.Entry, len(names))
	for i, n := range names {
		es[i] = cardset.Entry{Name: n, Count: 1}
	}
	return es
}
