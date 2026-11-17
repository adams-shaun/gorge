package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
)

// TestImpForgetThirdSourceLockstep: an effect other than the carrier moves an
// imprinted card out of exile, and the carrier's leave-exile trigger forgets
// it. Binding Negotiation's else branch moves a face-up exiled card the target
// owns to its owner's graveyard; the fixture's only face-up exiled cards are
// the carriers' imprints. The trigger's ValidCard$ is judged at the listener
// (the carrier), not at the resolving entry (Binding Negotiation's spell), so a
// third-source move must still fire the forget.
func TestImpForgetThirdSourceLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	var st dirac.SrcMemStats
	dirac.DebugSrcMemStats = &st
	defer func() { dirac.DebugSrcMemStats = nil }()
	run := runFixtureLockstepPolicy(t, "impforget", impForgetThirdDecks, 120, true, nil, impForgetThirdPolicy)
	t.Logf("%d games, %d divergences, %d stalls, casts %v, recorded %d seeded %d reads %d matched %d cleared %d forgot %d", run.games, run.divs, run.stalls, run.casts, st.Recorded, st.Seeded, st.Reads, st.Matched, st.Cleared, st.Forgot)
	if run.divs != 0 || run.stalls != 0 {
		t.Errorf("%d divergences, %d stalls; want none", run.divs, run.stalls)
	}
	if run.casts["Binding Negotiation"] < 5 {
		t.Errorf("only %d Binding Negotiation casts: the third-source move was never offered", run.casts["Binding Negotiation"])
	}
	if st.Forgot < 1 {
		t.Errorf("no store record was forgotten: the third-source move did not fire the leave-exile trigger")
	}
}

var impForgetThirdDecks = []fixtureDecks{{
	{
		{Name: "Mirror Golem", Count: 4}, {Name: "Binding Negotiation", Count: 4},
		{Name: "Grizzly Bears", Count: 8}, {Name: "Hill Giant", Count: 6},
		{Name: "Forest", Count: 6}, {Name: "Swamp", Count: 8}, {Name: "Mountain", Count: 4},
	},
	{
		{Name: "Mirror Golem", Count: 4}, {Name: "Binding Negotiation", Count: 4},
		{Name: "Grizzly Bears", Count: 8}, {Name: "Hill Giant", Count: 6},
		{Name: "Forest", Count: 6}, {Name: "Swamp", Count: 8}, {Name: "Mountain", Count: 4},
	},
}}

var impForgetThirdWatch = map[string]bool{
	"Mirror Golem": true, "Binding Negotiation": true,
}

// impForgetThirdPolicy plays the watched casts, takes every optional imprint
// exile, declines Binding Negotiation's discard so its else branch moves the
// face-up exiled card, confirms that pick, and never attacks: the carriers
// stay on the battlefield to hear the third source's move.
func impForgetThirdPolicy(seed uint64, casts map[string]int) Policy {
	watch := fixtureWatchPolicy(seed, impForgetThirdWatch, casts)
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.G != nil {
			if d := c.G.Pending(); d != nil {
				src := ""
				if o := c.G.E.G.Obj(d.Source); o != nil && o.Face() != nil {
					src = o.Face().Name
				}
				for _, a := range c.D.Options {
					o, ok := c.G.GorgeOption(a)
					if !ok {
						continue
					}
					switch d.ResumeKind {
					case "changezone_optional":
						if src == "Mirror Golem" && o.Kind == "yes" {
							return a
						}
					case "discard":
						if src == "Binding Negotiation" && a.Kind() == ActDone {
							return a
						}
					case "hidden_pick_confirm":
						if src == "Binding Negotiation" && o.Kind == "yes" {
							return a
						}
					}
				}
			}
			if c.D.Kind == DAttackers || c.D.Kind == DBlockers {
				for _, a := range c.D.Options {
					if a.Kind() == ActDone {
						return a
					}
				}
			}
		}
		return watch.Pick(c)
	})
}
