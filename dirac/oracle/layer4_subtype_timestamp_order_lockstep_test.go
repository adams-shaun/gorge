package oracle

import (
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
)

// Wishful Merfolk's "{1}{U}: loses defender and becomes a Human" (an Animate
// with RemoveCreatureTypes$ True) and Butch DeLoria, Tunnel Snake's
// "{1}{B}: another target creature becomes a Rogue in addition". Both are
// layer-4 subtype effects, ordered by timestamp (CR 613.1d, 613.7). The
// recording is the game where the additive Rogue record merged into its first
// application and kept an older timestamp, so a later Human replacement was
// applied after it and left Human only.

// TestLayer4SubtypeTimestampOrderReprosLockstep replays that game. After the
// fix the additive record takes a new timestamp on each application and
// resolves after the older replacement, leaving both Human and Rogue.
func TestLayer4SubtypeTimestampOrderReprosLockstep(t *testing.T) {
	stdReproNamed(t, "layer4_subtype_timestamp_order")
}

// layer4SubtypeTimestampOrderPolicy drives the four-seat pod into the
// mechanism: activate Wishful Merfolk and Butch DeLoria whenever possible,
// cast the pod's cards, aim Butch's "another target creature" at Wishful
// Merfolk, and play lands. Card names are read from gorge's side (the policy
// answers gorge; fourSeatSetPlay owns the matchup).
func layer4SubtypeTimestampOrderPolicy(seed uint64) Policy {
	rnd := RandomPolicy(seed)
	named := func(c *PickCtx, o Canon) string {
		if c.G == nil {
			return ""
		}
		g, ok := c.G.GorgeOption(o)
		if !ok {
			return ""
		}
		ob := c.G.E.G.Obj(g.Obj)
		if ob == nil || ob.Face() == nil {
			return ""
		}
		return ob.Face().Name
	}
	return PolicyFunc(func(c *PickCtx) Canon {
		for _, o := range c.D.Options {
			if o.Kind() == ActYes {
				return o
			}
		}
		if c.D.Kind == DTarget {
			for _, o := range c.D.Options {
				if o.Kind() == ActTarget && named(c, o) == "Wishful Merfolk" {
					return o
				}
			}
			for _, o := range c.D.Options {
				if o.Kind() == ActTarget && int(o.Tgt()) < int(RefPlayer) {
					return o
				}
			}
		}
		if c.D.Kind == DPriority {
			var act, butch, cast, land []Canon
			for _, o := range c.D.Options {
				switch o.Kind() {
				case ActActivate:
					switch named(c, o) {
					case "Wishful Merfolk":
						act = append(act, o)
					case "Butch DeLoria, Tunnel Snake":
						butch = append(butch, o)
					}
				case ActCast:
					switch named(c, o) {
					case "Wishful Merfolk", "Butch DeLoria, Tunnel Snake", "Reprobation", "Universal Automaton":
						cast = append(cast, o)
					}
				case ActPlayLand:
					land = append(land, o)
				}
			}
			pick := func(opts []Canon) Canon {
				d := *c.D
				d.Options = opts
				return rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
			}
			switch {
			case len(act) > 0:
				return act[0]
			case len(butch) > 0:
				return butch[0]
			case len(cast) > 0:
				return pick(cast)
			case len(land) > 0:
				return pick(land)
			}
		}
		return rnd.Pick(c)
	})
}

// TestLayer4SubtypeTimestampOrderFourSeat plays a four-seat pod holding the
// two cards plus Reprobation and a changeling, and requires every seed to run
// in lockstep and at least one permanent to reach two layer-4 subtype
// overrides with different timestamps. The check hook counts that mechanism in
// the kernel, so the test cannot pass on a pod that never assembles it.
func TestLayer4SubtypeTimestampOrderFourSeat(t *testing.T) {
	// synthReg first: it skips without a corpus, and printedDelveSet would
	// otherwise fatal on the missing .cards (a fresh clone must skip, not fail).
	synthReg(t)
	var deck []string
	deck = append(deck, deckOf(4, "Wishful Merfolk")...)
	deck = append(deck, deckOf(4, "Butch DeLoria, Tunnel Snake")...)
	deck = append(deck, deckOf(4, "Reprobation")...)
	deck = append(deck, deckOf(4, "Universal Automaton")...)
	deck = append(deck, deckOf(12, "Plains")...)
	deck = append(deck, deckOf(12, "Island")...)
	deck = append(deck, deckOf(12, "Swamp")...)
	pod := [4][]string{deck, deck, deck, deck}
	set := printedDelveSet(t, [2][]string{deck, deck})
	hits := 0
	play := fourSeatSetPlay(t, set, pod, nil,
		func(_ *GorgeSide, ker Side) *Divergence {
			if s, _, _ := KernelStateOf(ker); s != nil && s.DebugSubtypeOverridePairs() > 0 {
				hits++
			}
			return nil
		}, nil,
		func(seed uint64) Policy { return layer4SubtypeTimestampOrderPolicy(seed) })
	for seed := uint64(1); seed <= 20; seed++ {
		res := play(seed)
		if res.Div != nil {
			t.Fatalf("seed %d: divergence at %d: %s %s %v (last %s; ref options %v; ker options %v)",
				seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff,
				res.Div.LastSourced, res.Div.RefOptions, res.Div.KerOptions)
		}
		if res.StallUnmodelled {
			t.Fatalf("seed %d: kernel stopped unmodelled: %s", seed, res.Stall)
		}
	}
	if hits == 0 {
		t.Fatal("no permanent reached two layer-4 subtype overrides with different timestamps")
	}
	t.Logf("20 seeds, %d decisions with the ordering mechanism live", hits)
}

// TestLayer4SubtypeTimestampOrderMutation restores the old merge with
// DebugSubtypeReplaceLast: the additive record stays in its older position and
// the recording diverges again.
func TestLayer4SubtypeTimestampOrderMutation(t *testing.T) {
	r, err := ReadRepro(filepath.Join("testdata", "repro", "layer4_subtype_timestamp_order.repro.json"))
	if err != nil {
		t.Fatal(err)
	}
	old := dirac.DebugSubtypeReplaceLast
	dirac.DebugSubtypeReplaceLast = true
	defer func() { dirac.DebugSubtypeReplaceLast = old }()
	res := replayDecodedRes(t, synthReg(t), globSet(t, r.Pool, r), r)
	if res.Div == nil {
		t.Fatal("mutation merged the additive subtype record into its older copy with no divergence")
	}
	t.Logf("mutation diverges at %d: %s %s %v", res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
}
