package oracle

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// Repeatable charms (CanRepeatModes$): the lowered Cosmium Confluence, Moment
// of Reckoning and Season of Loss each choose one mode more than once, in a
// game both engines play decision by decision. TestStdSetReprosLockstep
// replays the three repros; TestCharmRepeatReprosChooseRepeats requires each
// recorded game to announce a repeated mode and resolve its instances, since
// a lockstep entry alone would pass a script that never cast the card.
//
// The games are recorded by charmRepeatRecord (CHARM_REPEAT_RECORD=1, the
// kernel table built from the decks): both seats play a scripted policy that
// casts the case's cards first, answers the modes decision from the case's
// picks, aims each target at an object not yet picked and passes otherwise.
type charmRepeatCase struct {
	name, card string
	prefer     []string // the cards the scripted seats cast first, after the case's own
	decks      [2][]string
	picks      []int // the mode indices the caster answers, in order
	// want holds for the recorded game: the card's repeated mode resolves this
	// many times (events of the kind that the repeated mode causes).
	kind events.Kind
	n    int
}

func deckOf(n int, card string, rest ...string) []string {
	d := []string{}
	for i := 0; i < n; i++ {
		d = append(d, card)
	}
	return append(d, rest...)
}

func charmRepeatCases() []charmRepeatCase {
	return []charmRepeatCase{
		{
			name: "cosmium_confluence_destroys_two_enchantments", card: "Cosmium Confluence",
			// three modes (search, put counters, destroy): destroy twice, search once
			decks: [2][]string{
				append(deckOf(6, "Cosmium Confluence"), deckOf(18, "Forest")...),
				append(deckOf(8, "Glorious Anthem"), deckOf(16, "Plains")...),
			},
			picks: []int{2, 2, 0}, kind: events.MoveZone, n: 2,
		},
		{
			name: "moment_of_reckoning_destroys_twice", card: "Moment of Reckoning",
			decks: [2][]string{
				append(deckOf(6, "Moment of Reckoning"), append(deckOf(9, "Plains"), deckOf(9, "Swamp")...)...),
				append(deckOf(8, "Glorious Anthem"), deckOf(16, "Plains")...),
			},
			picks: []int{0, 0}, kind: events.MoveZone, n: 2,
		},
		{
			name: "season_of_loss_sacrifices_three_times", card: "Season of Loss", prefer: []string{"Grizzly Bears"},
			// budget 5: three sacrifices (1 each) and a draw (2)
			decks: [2][]string{
				append(deckOf(6, "Season of Loss"), append(deckOf(4, "Grizzly Bears"), deckOf(14, "Swamp")...)...),
				append(deckOf(6, "Grizzly Bears"), deckOf(18, "Forest")...),
			},
			picks: []int{0, 0, 0, 1}, kind: events.MoveZone, n: 2,
		},
	}
}

// charmRepeatPolicy is the scripted policy of one case.
func charmRepeatPolicy(m *Matchup, c charmRepeatCase) Policy {
	prefer := append([]string{c.card, "Glorious Anthem"}, c.prefer...)
	next := 0
	picked := map[Ref]bool{}
	return PolicyFunc(func(p *PickCtx) Canon {
		opts := p.D.Options
		kindOf := func(k CanonKind) []Canon {
			var out []Canon
			for _, o := range opts {
				if o.Kind() == k {
					out = append(out, o)
				}
			}
			return out
		}
		switch p.D.Kind {
		case DModes:
			if next < len(c.picks) {
				for _, o := range opts {
					if o.Kind() == ActMode && int(o.Mode()) == c.picks[next] {
						next++
						return o
					}
				}
			}
			if d := kindOf(ActDone); len(d) > 0 {
				return d[0]
			}
		case DTarget:
			for _, o := range kindOf(ActTarget) {
				if !picked[o.Tgt()] {
					picked[o.Tgt()] = true
					return o
				}
			}
		case DAttackers, DBlockers:
			if d := kindOf(ActDone); len(d) > 0 {
				return d[0]
			}
		case DPriority:
			next, picked = 0, map[Ref]bool{}
			for _, want := range prefer {
				for _, o := range kindOf(ActCast) {
					if m.InstanceCard(int(o.Src())) == want {
						return o
					}
				}
			}
			if l := kindOf(ActPlayLand); len(l) > 0 {
				return l[0]
			}
		}
		return opts[0]
	})
}

// charmRepeatSet builds the kernel table the case's decks need.
func charmRepeatSet(t *testing.T, decks [2][]string) cardset.Set {
	t.Helper()
	return printedDelveSet(t, decks)
}

// charmRepeatEvents reports the recorded game's repeated announcement of
// card: the stack object that announced a mode twice, and the number of
// kind-events between its Resolve and the next Resolve.
func charmRepeatEvents(evs []events.Event, g *state.Game, card string, kind events.Kind) (repeated bool, hits int) {
	for i := range evs {
		e := &evs[i]
		if e.Kind != events.ModeChosen {
			continue
		}
		o := g.Obj(e.Obj)
		if o == nil || o.Face() == nil || o.Face().Name != card {
			continue
		}
		seen := map[string]bool{}
		for _, l := range strings.Split(e.Text, ",") {
			repeated = repeated || seen[l]
			seen[l] = true
		}
		if !repeated {
			continue
		}
		res := false
		for j := i + 1; j < len(evs); j++ {
			switch {
			case evs[j].Kind == events.Resolve && evs[j].Obj == e.Obj:
				res = true
			case evs[j].Kind == events.Resolve && res:
				return repeated, hits
			case res && evs[j].Kind == kind:
				hits++
			}
		}
		return repeated, hits
	}
	return false, 0
}

func charmRepeatPlay(t *testing.T, reg *cards.Registry, c charmRepeatCase, r *Repro, set cardset.Set, pol Policy) (Result, bool, int) {
	t.Helper()
	m, err := r.Matchup(reg)
	if err != nil {
		t.Fatal(err)
	}
	entries := func(names []string) []cardset.Entry {
		es := make([]cardset.Entry, len(names))
		for i, n := range names {
			es[i] = cardset.Entry{Name: n, Count: 1}
		}
		return es
	}
	tbl, _, _, err := set.Matchup(entries(r.Cards[0]), entries(r.Cards[1]))
	if err != nil {
		t.Fatal(err)
	}
	rc := r.RunConfig()
	rc.MaxDecisions = len(r.Actions) + 200
	if pol == nil {
		pol = &ScriptPolicy{Actions: r.Actions}
	}
	var repeated bool
	var hits int
	rc.Finish = func(g *GorgeSide) {
		repeated, hits = charmRepeatEvents(g.E.L.Events, g.E.G, c.card, c.kind)
	}
	k, err := SetSide(set, tbl, rc.Config)(m, r.Seed)
	if err != nil {
		t.Fatal(err)
	}
	defer CloseSide(k)
	return Lockstep(m, r.Seed, k, pol, rc), repeated, hits
}

// TestCharmRepeatReprosChooseRepeats replays the recorded games: no
// divergence, every scripted action offered, a repeated mode announced and
// its instances resolved.
func TestCharmRepeatReprosChooseRepeats(t *testing.T) {
	reg := synthReg(t)
	for _, c := range charmRepeatCases() {
		t.Run(c.card, func(t *testing.T) {
			r, err := ReadRepro(filepath.Join("testdata", "repro", c.name+".repro.json"))
			if err != nil {
				t.Fatal(err)
			}
			script := &ScriptPolicy{Actions: r.Actions}
			res, repeated, hits := charmRepeatPlay(t, reg, c, r, charmRepeatSet(t, r.Cards), script)
			if res.Div != nil {
				t.Fatalf("divergence at %d: %s %s %v", res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
			}
			if script.Skipped != 0 {
				t.Errorf("%d scripted actions were not offered", script.Skipped)
			}
			if !repeated {
				t.Fatalf("the recorded game never announced a repeated mode of %s", c.card)
			}
			if hits < c.n {
				t.Fatalf("%d %v events from the %s resolution, want at least %d", hits, c.kind, c.card, c.n)
			}
		})
	}
}

// charmRepeatRecord plays the cases' scripted policies over seeds until one
// announces a repeated mode, and writes the repro. It runs only with
// CHARM_REPEAT_RECORD=1; the games it writes are the committed repros.
func TestCharmRepeatRecord(t *testing.T) {
	if os.Getenv("CHARM_REPEAT_RECORD") == "" {
		return // a recording tool, not coverage: it returns rather than skips, since a skip here must name a missing pool (testdata/poolskips.txt)
	}
	reg := synthReg(t)
	for _, c := range charmRepeatCases() {
		t.Run(c.card, func(t *testing.T) {
			set := charmRepeatSet(t, c.decks)
			for seed := uint64(1); seed <= 400; seed++ {
				r := &Repro{Format: reproFormat, Decks: [2]string{c.name + "_0", c.name + "_1"}, Cards: c.decks, Seed: seed, PassElision: true, Surface: "autopay", Kernel: "custom"}
				m, err := r.Matchup(reg)
				if err != nil {
					t.Fatal(err)
				}
				res, repeated, hits := charmRepeatPlay(t, reg, c, r, set, charmRepeatPolicy(m, c))
				if res.Div != nil {
					t.Fatalf("seed %d: divergence at %d: %s %s %v", seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
				}
				if !repeated || hits < c.n {
					continue
				}
				r.setActions(res.Actions)
				if err := r.Write(filepath.Join("testdata", "repro", c.name+".repro.json")); err != nil {
					t.Fatal(err)
				}
				t.Logf("seed %d: %d decisions, %d events", seed, len(res.Actions), hits)
				return
			}
			t.Fatalf("no seed announced a repeated mode of %s", c.card)
		})
	}
}
