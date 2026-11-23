package shadowseat

import (
	"fmt"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/canon"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/rules"
	"github.com/adams-shaun/gorge/engine/view"
	"github.com/adams-shaun/gorge/testkit/bench"
	"github.com/adams-shaun/gorge/testkit/testutil"
	"math/rand/v2"
)

// mapChecker checks, at each staged decision of the seat it wraps, that the
// dirac options and the gorge options meet at the canonical action.
type mapChecker struct {
	t       *testing.T
	s       *Seat
	e       *rules.Engine
	checked int
	skipped int
	submits int
	menace  int
	fails   int
}

func (c *mapChecker) failf(format string, a ...any) {
	c.t.Helper()
	if c.fails++; c.fails <= 8 {
		c.t.Errorf(format, a...)
	}
}

func (c *mapChecker) check(v *view.View, d *decision.Decision) {
	c.t.Helper()
	switch d.Kind {
	case decision.KPriority, decision.KAttackers, decision.KBlockers:
	default:
		return
	}
	x, reason := c.s.stage(v, d)
	if reason != "" {
		c.skipped++
		return
	}
	c.checked++
	ans, dups := gorgeAnswers(v, d, x.num)
	for _, cn := range dups {
		c.failf("decision %d: two gorge options map to %v", d.Seq, cn)
	}
	// dirac -> gorge: every option but AkDone maps to a gorge answer, once
	image := map[canon.Canon]int{}
	for i := 0; i < int(x.dd.N); i++ {
		a := x.dd.Options[i]
		if a.Kind() == dirac.AkDone {
			continue
		}
		cn := x.tr.Action(a)
		image[cn]++
		if _, ok := ans[cn]; !ok {
			c.failf("decision %d (%s): dirac option %v canon %v has no gorge option; gorge options: %v", d.Seq, d.Kind, a, cn, optionList(d))
		}
	}
	// gorge -> dirac: every pass, land, admitted ability, attacker, blocker
	// and planned cast is the image of exactly one dirac option
	keys := make([]canon.Canon, 0, len(ans))
	for cn := range ans {
		keys = append(keys, cn)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
	for _, cn := range keys {
		a := ans[cn]
		if cn.Kind() == canon.ActCast && a.pay == nil {
			continue // a plain cast option the planner did not price
		}
		if n := image[cn]; n != 1 {
			c.failf("decision %d (%s): gorge answer %v has %d dirac options; dirac options: %v", d.Seq, d.Kind, cn, n, diracList(&x.tr, &x.dd))
		}
	}
	// every one-pick answer is accepted by gorge
	for _, cn := range keys {
		a := ans[cn]
		in := intentOf(d, []answer{a})
		c.submits++
		if err := c.e.Clone().Submit(in); err != nil {
			if d.Kind == decision.KBlockers && strings.Contains(err.Error(), "must be blocked by at least 2 creatures") {
				// a menace attacker: one block alone is no legal declaration
				// (CR 702.110b); the pair is the answer's, not a one-pick one
				c.menace++
				continue
			}
			c.failf("decision %d (%s): gorge refuses the answer for %v: %v", d.Seq, d.Kind, cn, err)
		}
	}
}

func optionList(d *decision.Decision) []string {
	var out []string
	for i := range d.Options {
		o := &d.Options[i]
		out = append(out, fmt.Sprintf("%d:%s/obj%d/mode%q", o.Index, o.Kind, o.Obj, o.Mode))
	}
	return out
}

func diracList(tr *canon.Translator, dd *dirac.Decision) []string {
	var out []string
	for i := 0; i < int(dd.N); i++ {
		out = append(out, tr.Action(dd.Options[i]).String())
	}
	return out
}

func TestShadowSeatMapsEveryOption(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	p := vanillaPool()
	c := &mapChecker{t: t}
	for seed := uint64(1); seed <= 30; seed++ {
		g := p.setup(t, reg, seed)
		c.s = p.newSeat(t, reg, g, 8, 4, seed)
		rec := &recorder{Seat: c.s, before: c.check}
		play(t, g, 0, rec, seed, bench.Hooks{Setup: func(e *rules.Engine) { c.e = e }})
	}
	t.Logf("checked %d decisions (%d unstageable), %d gorge submissions (%d single blocks of a menace attacker exempt)", c.checked, c.skipped, c.submits, c.menace)
	if c.checked < 300 {
		t.Errorf("only %d decisions checked, want at least 300", c.checked)
	}
}

// playMany plays n games of pool p, the shadow seat alternating seats, and
// returns the summed stats, the wins and the refusals.
func playMany(t *testing.T, p *pool, n int, sims, worlds int) (st Stats, wins, games, refusals int) {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	st.Reasons, st.CardRefusals = map[string]int{}, map[string]int{}
	for i := 0; i < n; i++ {
		seed := uint64(i + 1)
		g := p.setup(t, reg, seed)
		sd := p.newSeat(t, reg, g, sims, worlds, seed)
		idx := i % 2
		o := play(t, g, idx, sd, seed, bench.Hooks{
			Submit: func(e *rules.Engine, seatIdx int, d *decision.Decision, in decision.Intent) (bool, error) {
				if err := e.Submit(in); err != nil {
					if seatIdx == idx {
						refusals++
						t.Errorf("seed %d: gorge refused the shadow seat's intent at %s decision %d: %v", seed, d.Kind, d.Seq, err)
					}
					return true, err
				}
				return true, nil
			},
		})
		games++
		if !o.Draw && !o.IsStalled() && o.WinnerSeat == idx {
			wins++
		}
		s := sd.Stats()
		st.Decisions += s.Decisions
		st.Searched += s.Searched
		st.Continued += s.Continued
		st.Fallbacks += s.Fallbacks
		for k, v := range s.Reasons {
			st.Reasons[k] += v
		}
		for k, v := range s.CardRefusals {
			st.CardRefusals[k] += v
		}
	}
	return
}

func sortedKeys(m map[string]int) []string {
	var ks []string
	for k := range m {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	return ks
}

func logStats(t *testing.T, st Stats, wins, games int) {
	t.Helper()
	t.Logf("%d decisions: %d searched (%d continued), %d fallbacks; %d of %d games won by the shadow seat", st.Decisions, st.Searched, st.Continued, st.Fallbacks, wins, games)
	for _, k := range sortedKeys(st.Reasons) {
		t.Logf("  reason %s: %d", k, st.Reasons[k])
	}
	for _, k := range sortedKeys(st.CardRefusals) {
		t.Logf("  card refusal %s: %d", k, st.CardRefusals[k])
	}
}

func TestShadowSeatPlaysHeuristic(t *testing.T) {
	st, wins, games, _ := playMany(t, vanillaPool(), 12, 32, 4)
	logStats(t, st, wins, games)
	if st.Searched*10 < st.Decisions*3 {
		t.Errorf("searched %d of %d decisions, want at least 30%%", st.Searched, st.Decisions)
	}
}

// mbcPool is the corpus pool of dirac/shadow's stage_cards_test.go: Pacifism,
// Swiftfoot Boots, Dragon Fodder, Raise the Alarm, two basic lands and
// creatures with combat keywords.
func mbcPool(t *testing.T) *pool {
	t.Helper()
	cardsettest.Require(t, "mbc")
	set, err := cardset.ByName("mbc")
	if err != nil {
		t.Fatal(err)
	}
	lands := []string{"Plains", "Mountain"}
	creatures := []string{
		"Fencing Ace", "Exultant Skymarcher", "Tormented Angel", "Charging Monstrosaur",
		"Wayward Giant", "Nimble Birdsticker", "Starlit Angel", "Shrike Force", "Faithbearer Paladin",
		"Knight of the Tusk",
	}
	var list []cardset.Entry
	for _, n := range append(append([]string{"Pacifism", "Swiftfoot Boots", "Dragon Fodder", "Raise the Alarm"}, lands...), creatures...) {
		list = append(list, cardset.Entry{Name: n, Count: 1})
	}
	tbl, _, _, err := set.Matchup(list, list)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]kop.CardID{}
	for i := range tbl.Cards {
		if tbl.Cards[i].Types&kop.TToken == 0 {
			id[tbl.Cards[i].Name] = kop.CardID(i)
		}
	}
	return &pool{tbl: tbl, set: set, deck: func(r *rand.Rand) []kop.CardID {
		var d []kop.CardID
		add := func(n int, names ...string) {
			for i := 0; i < n; i++ {
				d = append(d, id[names[r.IntN(len(names))]])
			}
		}
		add(16, lands...)
		add(3, "Pacifism")
		add(3, "Swiftfoot Boots")
		add(4, "Dragon Fodder")
		add(3, "Raise the Alarm")
		add(14, creatures...)
		return d
	}}
}

func TestShadowSeatContinuesTargets(t *testing.T) {
	st, wins, games, _ := playMany(t, mbcPool(t), 12, 16, 4)
	logStats(t, st, wins, games)
	if st.Continued < 5 {
		t.Errorf("%d target decisions answered from a plan, want at least 5", st.Continued)
	}
}

func TestShadowSeatDeterministic(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	p := vanillaPool()
	for seed := uint64(1); seed <= 2; seed++ {
		g := p.setup(t, reg, seed)
		var runs [2][]decision.Intent
		for i := range runs {
			rec := &recorder{Seat: p.newSeat(t, reg, g, 16, 4, seed)}
			play(t, g, int(seed%2), rec, seed, bench.Hooks{})
			runs[i] = rec.intents
		}
		if len(runs[0]) < 20 {
			t.Fatalf("seed %d: only %d intents", seed, len(runs[0]))
		}
		if !reflect.DeepEqual(runs[0], runs[1]) {
			t.Errorf("seed %d: two plays of the same game gave different intents", seed)
		}
	}
}
