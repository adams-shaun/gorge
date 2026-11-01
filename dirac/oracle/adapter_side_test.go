package oracle

import (
	"fmt"
	"maps"
	"os"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/adapter"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/game"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/engine/view"
)

func adapterRun(t *testing.T, name string, first, n int, mulligans int, cfg Config, check func(ref *GorgeSide, a *adapter.Game)) (clean, total int) {
	t.Helper()
	c, pool := testPoolCorpus(t, name)
	set := rawSet(name)
	surface := dirac.SurfaceFull
	if cfg.GorgeAutopaySurface {
		surface = dirac.SurfaceAutopay
	}
	cfg.ArrangeRestOrder = true
	e, err := adapter.New([]string{name}, c.Reg, surface)
	if err != nil {
		t.Skip(err)
	}
	for _, p := range PairingsFrom(pool, first, n, 1) {
		m, _, err := SetMatchup(set, c, p.A, p.B, mulligans)
		if err != nil {
			t.Fatal(err)
		}
		k, err := AdapterSide(e, cfg)(m, p.Seed)
		if err != nil {
			continue
		}
		rc := RunConfig{Config: cfg}
		if check != nil {
			rc.Check = func(ref *GorgeSide, s Side) *Divergence {
				if a, ok := s.(*adapterSide); ok && len(a.chosen) == 0 && !a.closed {
					check(ref, a.g)
				}
				return nil
			}
		}
		res := Lockstep(m, p.Seed, k, RandomPolicy(p.Seed), rc)
		total++
		if res.Clean() {
			clean++
		} else if res.Div != nil {
			t.Errorf("%s pair %d %s vs %s seed %d: %s", name, p.Index, p.A, p.B, p.Seed, res.Div)
		} else {
			t.Errorf("%s pair %d: not clean: %+v", name, p.Index, res)
		}
		CloseSide(k)
	}
	return clean, total
}

// TestAdapterIsALockstepSide plays a served game against gorge: every answer goes
// through Submit as a whole Intent, and the position is compared at each decision.
func TestAdapterIsALockstepSide(t *testing.T) {
	pairs := envPairs(100)
	for _, name := range []string{"fdn", "blb"} {
		for _, autopay := range []bool{true, false} {
			t.Run(fmt.Sprintf("%s/autopay=%v", name, autopay), func(t *testing.T) {
				if testing.Short() && (name != "fdn" || !autopay) {
					t.Skip("short")
				}
				clean, total := adapterRun(t, name, 0, pairs, 1, Config{GorgeAutopaySurface: autopay}, nil)
				t.Logf("%d of %d clean", clean, total)
				if total == 0 {
					t.Fatal("no game played")
				}
			})
		}
	}
}

// TestNativeDecisionMatchesGorge compares each non-priority decision a client sees
// with gorge's: its kind, bounds, source and options in order.
func TestNativeDecisionMatchesGorge(t *testing.T) {
	pairs := envPairs(40)
	for _, name := range []string{"fdn", "blb"} {
		for _, autopay := range []bool{true, false} {
			t.Run(fmt.Sprintf("%s/autopay=%v", name, autopay), func(t *testing.T) {
				if testing.Short() && (name != "fdn" || !autopay) {
					t.Skip("short")
				}
				decisionCheck(t, name, pairs, autopay)
			})
		}
	}
}

func decisionCheck(t *testing.T, name string, pairs int, autopay bool) {
	tally := map[string]int{}
	diffs := map[string]int{}
	samples := map[string]string{}
	check := func(ref *GorgeSide, a *adapter.Game) {
		gd, ad := ref.Pending(), a.Pending()
		if gd == nil || ad == nil {
			return
		}
		if gd.Kind == decision.KPriority {
			for _, what := range abilityDiffs(ref, gd, a, ad) {
				k := "priority " + what
				diffs[k]++
				samples[k] = what
			}
			return
		}
		gd = withoutManualRoutes(gd)
		key := string(gd.Kind)
		if gd.Kind == decision.KChoose && len(gd.Options) > 0 {
			key += "/" + gd.Options[0].Kind
		}
		tally[key]++
		for _, what := range decisionDiffs(ref, gd, a, ad) {
			k := key + " " + what
			diffs[k]++
			if samples[k] == "" {
				samples[k] = fmt.Sprintf("gorge %s served %s", optionShape(gd, ref.InstRef), optionShape(ad, a.RefOf))
			}
		}
	}
	clean, total := adapterRun(t, name, 0, pairs, 1, Config{GorgeAutopaySurface: autopay}, check)
	var b strings.Builder
	fmt.Fprintf(&b, "games %d clean %d\n", total, clean)
	for _, k := range slices.Sorted(maps.Keys(tally)) {
		fmt.Fprintf(&b, "%8d %s\n", tally[k], k)
	}
	if p := os.Getenv("ADAPTER_OUT"); p != "" {
		os.WriteFile(p, []byte(b.String()), 0o644)
	}
	t.Log(b.String())
	for _, k := range slices.Sorted(maps.Keys(diffs)) {
		t.Errorf("%d %s: %s", diffs[k], k, samples[k])
	}
}

// decisionDiffs names what differs between two decisions.
func decisionDiffs(ref *GorgeSide, gd *decision.Decision, a *adapter.Game, ad *decision.Decision) []string {
	var out []string
	if ad.Kind != gd.Kind {
		out = append(out, "kind")
	}
	// gorge's maximum can exceed the options there are, unless they repeat
	gmax, amax := min(gd.Max, len(gd.Options)), min(ad.Max, len(ad.Options))
	if gd.Repeatable {
		gmax, amax = gd.Max, ad.Max
	}
	if ad.Min != gd.Min || amax != gmax || ad.Repeatable != gd.Repeatable {
		out = append(out, fmt.Sprintf("bounds gorge %d..%d served %d..%d", gd.Min, gd.Max, ad.Min, ad.Max))
	}
	if ref.InstRef(gd.Source) != a.RefOf(ad.Source) {
		out = append(out, "source")
	}
	if len(ad.Options) != len(gd.Options) {
		return append(out, fmt.Sprintf("count gorge %d served %d", len(gd.Options), len(ad.Options)))
	}
	for i := range gd.Options {
		g, s := &gd.Options[i], &ad.Options[i]
		if g.Kind != s.Kind || ref.InstRef(g.Obj) != a.RefOf(s.Obj) {
			return append(out, "options or their order")
		}
		if g.Required != s.Required || g.MinBlockers != s.MinBlockers {
			out = append(out, "required or minimum blockers")
		}
		// a blocker's group names its object, whose id differs
		if strings.HasPrefix(g.Group, "target-") && g.Group != s.Group || (g.Group == "") != (s.Group == "") {
			out = append(out, fmt.Sprintf("group gorge %q served %q", g.Group, s.Group))
		}
		if gd.Kind == decision.KTarget && g.Controller != s.Controller {
			out = append(out, "target controller")
		}
		if g.ManaSymbol != s.ManaSymbol || g.Kind != "division" && g.Amount != s.Amount {
			out = append(out, fmt.Sprintf("%s symbol or amount", g.Kind))
		}
	}
	if gd.TargetsWithSameController != ad.TargetsWithSameController {
		out = append(out, "same controller")
	}
	if (gd.TargetEffect == nil) != (ad.TargetEffect == nil) || gd.TargetEffect != nil && gd.TargetEffect.API != ad.TargetEffect.API {
		out = append(out, "target effect")
	}
	return out
}

// abilityDiffs compares what a seat reads of each activated ability it may play: its
// cost, as a set of parts because gorge orders them, and the loyalty facts served.
func abilityDiffs(ref *GorgeSide, gd *decision.Decision, a *adapter.Game, ad *decision.Decision) []string {
	var out []string
	parts := func(c string) []string {
		f := strings.Fields(c)
		slices.Sort(f)
		return f
	}
	for i := range gd.Options {
		g := &gd.Options[i]
		if g.Kind != "ability" {
			continue
		}
		for j := range ad.Options {
			s := &ad.Options[j]
			if s.Kind != "ability" || s.Ability != g.Ability || ref.InstRef(g.Obj) != a.RefOf(s.Obj) {
				continue
			}
			if !slices.Equal(parts(g.Cost), parts(s.Cost)) {
				out = append(out, fmt.Sprintf("cost of %s: gorge %q served %q", g.Label, g.Cost, s.Cost))
			}
			if (g.Loyalty == nil) != (s.Loyalty == nil) {
				out = append(out, "loyalty of "+g.Label)
			} else if g.Loyalty != nil && (g.Loyalty.Delta != s.Loyalty.Delta || g.Loyalty.X != s.Loyalty.X || g.Loyalty.Ultimate != s.Loyalty.Ultimate) {
				out = append(out, "loyalty of "+g.Label)
			}
		}
	}
	return out
}

// withoutManualRoutes drops a resolution payment's choose-your-sources route: a served
// game declares no manual mana, and the kernel funds every payment by the plan.
func withoutManualRoutes(d *decision.Decision) *decision.Decision {
	c := *d
	c.Options = slices.DeleteFunc(slices.Clone(d.Options), func(o decision.Option) bool { return strings.HasSuffix(o.Kind, "_manual") })
	return &c
}

// optionShape prints a decision's options as the comparison reads them.
func optionShape(d *decision.Decision, ref func(state.ObjID) Ref) string {
	var b strings.Builder
	fmt.Fprintf(&b, "[%s src#%d] ", d.Prompt, ref(d.Source))
	for i := range d.Options {
		o := &d.Options[i]
		fmt.Fprintf(&b, "%s", o.Kind)
		if o.Obj != 0 {
			fmt.Fprintf(&b, "#%d", ref(o.Obj))
		}
		if o.Mode != "" {
			fmt.Fprintf(&b, ":%s", o.Mode)
		}
		b.WriteByte(' ')
	}
	return b.String()
}

// envPairs is n, or ADAPTER_PAIRS for a longer run by hand.
func envPairs(n int) int {
	if v, err := strconv.Atoi(os.Getenv("ADAPTER_PAIRS")); err == nil && v > 0 {
		return v
	}
	return n
}

// startServed is a served game on the first FDN pair.
func startServed(t *testing.T) (*adapter.Engine, *Matchup, *adapter.Game) {
	t.Helper()
	c, pool := testPoolCorpus(t, "fdn")
	e, err := adapter.New([]string{"fdn"}, c.Reg, dirac.SurfaceFull)
	if err != nil {
		t.Skip(err)
	}
	p := PairingsFrom(pool, 0, 1, 1)[0]
	m, err := c.Matchup(p.A, p.B, 1)
	if err != nil {
		t.Fatal(err)
	}
	gm, err := e.New(game.Config{Seed: p.Seed, Names: []string{"p0", "p1"}, Decks: [][]*cards.Card{m.Decks[0], m.Decks[1]}, Mulligans: 1})
	if err != nil {
		t.Fatal(err)
	}
	return e, m, gm.(*adapter.Game)
}

// advanceToPriority answers with each decision's first legal shape until a priority
// decision is posed.
func advanceToPriority(t *testing.T, g *adapter.Game) *decision.Decision {
	t.Helper()
	for i := 0; i < 100; i++ {
		d := g.Pending()
		if d == nil {
			t.Fatal("game ended early")
		}
		if d.Kind == decision.KPriority {
			return d
		}
		in := decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{}}
		for j := 0; j < d.Min; j++ {
			in.Choices = append(in.Choices, j)
		}
		if err := g.Submit(in); err != nil {
			t.Fatal(err)
		}
	}
	t.Fatal("no priority decision")
	return nil
}

func TestSubmitRefusalLeavesStateIntact(t *testing.T) {
	_, _, g := startServed(t)
	d := g.Pending()
	seq, kind := d.Seq, d.Kind
	if err := g.Submit(decision.Intent{Seq: d.Seq + 1, Player: d.Player, Choices: []int{0}}); err == nil {
		t.Fatal("a stale seq was accepted")
	}
	if err := g.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{len(d.Options) + 3}}); err == nil {
		t.Fatal("an option out of range was accepted")
	}
	if p := g.Pending(); p == nil || p.Seq != seq || p.Kind != kind {
		t.Fatalf("the refused answers moved the game: %+v", p)
	}
}

func TestConcedeEndsTheGame(t *testing.T) {
	_, _, g := startServed(t)
	d := advanceToPriority(t, g)
	at := -1
	for i := range d.Options {
		if d.Options[i].Kind == "concede" {
			at = i
		}
	}
	if at < 0 {
		t.Fatal("no concede option")
	}
	if err := g.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{at}}); err != nil {
		t.Fatal(err)
	}
	b := g.Board()
	if g.Pending() != nil || !b.Over || b.Winner == d.Player {
		t.Fatalf("after concede: pending %v over %v winner %v", g.Pending(), b.Over, b.Winner)
	}
	if g.StallReason() != "" {
		t.Fatalf("a conceded game reads as stalled: %q", g.StallReason())
	}
}

func TestEveryOptionHasALabel(t *testing.T) {
	_, _, g := startServed(t)
	for i := 0; i < 300 && g.Pending() != nil; i++ {
		d := g.Pending()
		for j := range d.Options {
			if d.Options[j].Label == "" {
				t.Fatalf("%s option %d (%s) has no label", d.Kind, j, d.Options[j].Kind)
			}
		}
		in := decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{}}
		if d.Kind == decision.KPriority {
			// the last option but concede passes
			in.Choices = []int{len(d.Options) - 2}
		} else {
			for j := 0; j < d.Min; j++ {
				in.Choices = append(in.Choices, j)
			}
		}
		if err := g.Submit(in); err != nil {
			t.Fatal(err)
		}
	}
}

func TestForkIsIndependent(t *testing.T) {
	_, _, g := startServed(t)
	d := advanceToPriority(t, g)
	f := g.Fork().(*adapter.Game)
	in := decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{len(d.Options) - 2}}
	before := g.Pending().Seq
	if err := f.Submit(in); err != nil {
		t.Fatal(err)
	}
	if g.Pending().Seq != before {
		t.Fatal("answering the fork moved the game")
	}
	if err := g.Submit(in); err != nil {
		t.Fatal(err)
	}
	if f.Pending().Seq != g.Pending().Seq || f.Pending().Kind != g.Pending().Kind {
		t.Fatalf("the same answer diverged: fork %d %s, game %d %s", f.Pending().Seq, f.Pending().Kind, g.Pending().Seq, g.Pending().Kind)
	}
}

func TestAcceptsNamesTheCard(t *testing.T) {
	e, m, _ := startServed(t)
	base := game.Config{Names: []string{"p0", "p1"}, Decks: [][]*cards.Card{m.Decks[0], m.Decks[1]}}
	if err := e.Accepts(base); err != nil {
		t.Fatalf("a playable config: %v", err)
	}
	three := base
	three.Decks = [][]*cards.Card{m.Decks[0], m.Decks[1], m.Decks[0]}
	if err := e.Accepts(three); err == nil || !strings.Contains(err.Error(), "two seats") {
		t.Fatalf("three seats: %v", err)
	}
	cmd := base
	cmd.Format = game.FormatCommander
	if err := e.Accepts(cmd); err == nil || !strings.Contains(err.Error(), "constructed") {
		t.Fatalf("commander: %v", err)
	}
	stranger := &cards.Card{Path: "Not A Card In This Set"}
	bad := base
	bad.Decks = [][]*cards.Card{append(append([]*cards.Card(nil), m.Decks[0]...), stranger), m.Decks[1]}
	if err := e.Accepts(bad); err == nil || !strings.Contains(err.Error(), stranger.Path) {
		t.Fatalf("a card outside the set is not named: %v", err)
	}
}

func TestHandHiddenFromOpponent(t *testing.T) {
	_, _, g := startServed(t)
	advanceToPriority(t, g)
	v := view.ProjectFor(g.Board(), g.Chars(), 0, view.Seat, nil)
	if len(v.Players[1].Hand) != 0 || v.Players[1].HandSize == 0 {
		t.Fatalf("seat 0 reads seat 1's hand: %d cards, size %d", len(v.Players[1].Hand), v.Players[1].HandSize)
	}
	if len(v.Players[0].Hand) == 0 {
		t.Fatal("seat 0 cannot read its own hand")
	}
}

func TestSpectatorSeesNoHand(t *testing.T) {
	_, _, g := startServed(t)
	advanceToPriority(t, g)
	v := view.ProjectFor(g.Board(), g.Chars(), 0, view.Public, nil)
	for p := range v.Players {
		if len(v.Players[p].Hand) != 0 {
			t.Fatalf("a spectator reads seat %d's hand", p)
		}
	}
}
