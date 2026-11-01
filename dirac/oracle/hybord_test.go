package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
)

// The printed-hybrid-order lockstep: the three BLB carriers whose hybrid pips
// are printed outside WUBRG order ({G/W}, {G/U}, {W/R}) are played in a real
// game under both surface configs. Every pay_* decision in these decks is one
// of those pips (the fourth card, Brightblade Stoat, has a plain pip), so the
// minimum counts prove the lifted announcement order reached, not just the
// lowering.
//
// The autopay surface offers no hybrid cast at all: gorge's cast planner
// excludes hybrid costs (dirac/featview.go, plannedCastCost's
// "cd.Cost.Hyb[0] != 0"), so EnsurePaymentActions never plans one, and the
// oracle drops the lowered plain cast there (gorge.go, "GorgeAutopaySurface
// && pa.Kind == "cast" && a.Mode() == 0"). The pay asks therefore come from
// the plain surface; the autopay seeds still run and must agree on every
// decision. This is pre-existing gorge behaviour, not a property of the lift.
var hybOrderDecks = [2][]cardset.Entry{
	{
		{Name: "Head of the Homestead", Count: 8}, // {3}{G/W}{G/W}
		{Name: "Pond Prophet", Count: 4},          // {G/U}{G/U}
		{Name: "Forest", Count: 16}, {Name: "Plains", Count: 12},
	},
	{
		{Name: "Veteran Guardmouse", Count: 8}, // {3}{W/R}
		{Name: "Brightblade Stoat", Count: 4},  // {1}{W}
		{Name: "Plains", Count: 16}, {Name: "Mountain", Count: 12},
	},
}

var hybOrderCarriers = []string{"Head of the Homestead", "Pond Prophet", "Veteran Guardmouse"}

// hybOrderTally counts the lifted mechanic: a pip announcement (a KChoose
// whose chosen option is pay_*, gorge's manaAsk; the policy records it) and
// the PutOnStack of each carrier (read from gorge's events).
type hybOrderTally struct {
	evAt    int
	pipAsks int
	casts   map[string]int
}

func (h *hybOrderTally) reset() {
	h.evAt, h.pipAsks = 0, 0
	h.casts = map[string]int{}
}

func (h *hybOrderTally) check(ref *GorgeSide, _ Side) *Divergence {
	evs := ref.E.L.Events
	if len(evs) < h.evAt {
		h.evAt = 0
	}
	for j := h.evAt; j < len(evs); j++ {
		if evs[j].Kind == events.PutOnStack {
			h.casts[evs[j].Text]++
		}
	}
	h.evAt = len(evs)
	return nil
}

// hybOrderPolicy casts whenever it can, else picks randomly. Casting is what
// reaches the pip ask; it never avoids a legal action to dodge a divergence.
func hybOrderPolicy(seed uint64, tally *hybOrderTally) Policy {
	rnd := RandomPolicy(seed)
	payCode, _ := chooseCode("pay")
	pay := ActChoose + CanonKind(payCode)
	return PolicyFunc(func(c *PickCtx) Canon {
		var buf []Canon
		for _, a := range c.D.Options {
			switch a.Kind() {
			case ActCast, ActActivate, ActPlayLand:
				buf = append(buf, a)
			}
		}
		var pick Canon
		if len(buf) == 0 {
			pick = rnd.Pick(c)
		} else {
			d := *c.D
			d.Options = buf
			pick = rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
		}
		if pick.Kind() == pay {
			tally.pipAsks++
		}
		return pick
	})
}

func TestHybOrderLockstep(t *testing.T) {
	t.Parallel()

	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("blb")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, d := range hybOrderDecks {
		for _, e := range d {
			c, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s not in the corpus", e.Name)
			}
			for i := 0; i < e.Count; i++ {
				decks[s] = append(decks[s], c)
			}
		}
		if len(decks[s]) != 40 {
			t.Fatalf("deck %d: %d cards, want 40", s, len(decks[s]))
		}
	}
	tbl, _, _, err := set.Matchup(hybOrderDecks[0], hybOrderDecks[1])
	if err != nil {
		t.Fatal(err)
	}

	const seeds = 48
	var tally hybOrderTally
	total := hybOrderTally{casts: map[string]int{}}
	var surfaceAsks [2]int
	for seed := uint64(1); seed <= seeds; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		surface := 0
		if cfg.GorgeAutopaySurface {
			surface = 1
		}
		tally.reset()
		m, err := NewMatchup(reg, [2]string{"hyb-a", "hyb-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		res := Lockstep(m, seed, k, hybOrderPolicy(seed, &tally), RunConfig{
			Config: cfg, MaxDecisions: 8000, MaxTurns: 40, Check: tally.check,
		})
		CloseSide(k)
		if res.Div != nil {
			t.Fatalf("seed %d (autopay %v): %s", seed, cfg.GorgeAutopaySurface, res.Div)
		}
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			t.Fatalf("seed %d (autopay %v): %s", seed, cfg.GorgeAutopaySurface, res.Stall)
		}
		total.pipAsks += tally.pipAsks
		surfaceAsks[surface] += tally.pipAsks
		for _, name := range hybOrderCarriers {
			total.casts[name] += tally.casts[name]
		}
	}
	t.Logf("%d seeds: %d pip announcements (plain %d, autopay %d), casts %v",
		seeds, total.pipAsks, surfaceAsks[0], surfaceAsks[1], total.casts)
	if total.pipAsks < 200 {
		t.Errorf("%d pip announcements over %d seeds, want at least 200", total.pipAsks, seeds)
	}
	if surfaceAsks[0] < 50 {
		t.Errorf("plain-surface pip announcements = %d, want at least 50", surfaceAsks[0])
	}
	want := map[string]int{"Head of the Homestead": 40, "Pond Prophet": 20, "Veteran Guardmouse": 40}
	for _, name := range hybOrderCarriers {
		if total.casts[name] < want[name] {
			t.Errorf("%s cast %d times, want at least %d", name, total.casts[name], want[name])
		}
	}
}
