package searchseat

import (
	"math/rand/v2"
	"reflect"
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/botpolicy"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/internal/policynet"
	"github.com/adams-shaun/gorge/internal/searchprobe"
	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/rules"
	"github.com/adams-shaun/gorge/state"
	"github.com/adams-shaun/gorge/view"
)

// preferCardModel is a hand-built policynet Model whose head scores an option
// by one number: the embedding-table value of the option's card-identity row.
// Every row but the named cards' is zero, so the head "prefers" exactly the
// options naming those cards, by the given amounts.
func preferCardModel(prefs map[string]float32) *policynet.Model {
	const h, hidden = 1, 1
	m := &policynet.Model{
		Rows: policynet.TableRows, H: h, Hidden: hidden,
		InW:    2*h + policynet.OptionSlotWidth + policynet.OptionDenseWidth,
		Table:  make([]float32, policynet.TableRows*h),
		StateW: make([]float32, policynet.DenseWidth*h),
		StateB: make([]float32, h),
		HidB:   make([]float32, hidden),
		OutW:   []float32{1},
	}
	m.HidW = make([]float32, hidden*m.InW)
	m.HidW[h] = 1 // the option's hashed-embedding segment starts at offset H
	for name, v := range prefs {
		m.Table[policynet.HashID("card|"+name)] = v
	}
	return m
}

// attackFixture is a hand-built two-seat attackers decision: seat 0 may attack
// seat 1 with Alpha, Beta or Gamma (options 0, 1, 2), and the view that
// names them.
func attackFixture() (*decision.Decision, view.View) {
	d := &decision.Decision{Seq: 7, Player: 0, Kind: decision.KAttackers, Min: 0, Max: 3}
	v := view.View{Players: []view.PlayerView{{ID: 0}, {ID: 1}}}
	for i, name := range []string{"Alpha", "Beta", "Gamma"} {
		id := state.ObjID(101 + i)
		d.Options = append(d.Options, decision.Option{Index: i, Kind: "attack", Obj: id, Player: 1, Label: name})
		v.Players[0].Battlefield = append(v.Players[0].Battlefield, view.CardView{ID: id, Name: name, Types: "Creature", Power: 2, Toughness: 2})
	}
	return d, v
}

func attackChoices(t *testing.T, d *decision.Decision, bot decision.Intent, limit int) [][]int {
	t.Helper()
	var out [][]int
	for _, in := range searchprobe.AttackCandidates(d, bot, limit) {
		out = append(out, in.Choices)
	}
	return out
}

// The head prefers Gamma. The widened enumeration is, in order: 0 the bot's
// {Alpha}, 1 no attack {}, 2 all-in {Alpha,Beta,Gamma}, 3 {Alpha,Beta},
// 4 {Alpha,Gamma}. Under the BCE subset log-likelihood the two declarations
// that include Gamma (2 and 4) tie on top and the two that exclude it (1 and
// 3) tie below, so the ranking is 2, 4, 1, 3 -- each tie in enumeration
// order -- and the bot's answer stays first.
func TestPriorOrderKeepsTopKBotFirst(t *testing.T) {
	d, v := attackFixture()
	bot := decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{0}}
	choices := attackChoices(t, d, bot, 16)
	want := [][]int{{0}, nil, {0, 1, 2}, {0, 1}, {0, 2}}
	if !reflect.DeepEqual(choices, want) {
		t.Fatalf("fixture precondition: enumeration = %v, want %v", choices, want)
	}
	m := preferCardModel(map[string]float32{"Gamma": 5})

	// Precondition: the fake head really scores only Gamma.
	enc := make([]policynet.Option, len(d.Options))
	for i := range d.Options {
		enc[i] = policynet.EncodeOption(v, d.Player, d.Kind, d.Options[i], i, len(d.Options))
	}
	if s := m.Score(policynet.EncodeState(v, d.Player), enc); !reflect.DeepEqual(s, []float32{0, 0, 5}) {
		t.Fatalf("fixture precondition: head scores %v, want [0 0 5]", s)
	}

	for _, tc := range []struct {
		topK    int
		keep    []int
		changed bool
	}{
		{1, []int{0, 2}, true},
		{2, []int{0, 2, 4}, true},
		{4, []int{0, 2, 4, 1, 3}, false},
		{9, []int{0, 2, 4, 1, 3}, false},
		{0, []int{0}, false},
	} {
		keep, changed := priorOrder(m, v, d, bot, "attackers", choices, tc.topK)
		if !reflect.DeepEqual(keep, tc.keep) || changed != tc.changed {
			t.Errorf("topK %d: keep %v changed %v, want %v changed %v", tc.topK, keep, changed, tc.keep, tc.changed)
		}
	}
}

// An exactly flat head ranks nothing, so every non-bot candidate ties and the
// stable sort must keep enumeration order: the prior then IS the unguided
// list.
func TestPriorOrderTiesKeepEnumerationOrder(t *testing.T) {
	d, v := attackFixture()
	bot := decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{0}}
	choices := attackChoices(t, d, bot, 16)
	keep, changed := priorOrder(preferCardModel(nil), v, d, bot, "attackers", choices, 3)
	if !reflect.DeepEqual(keep, []int{0, 1, 2, 3}) || changed {
		t.Fatalf("flat head: keep %v changed %v, want [0 1 2 3] unchanged", keep, changed)
	}
}

// The cast arm ranks a candidate by its single option's score.
func TestPriorOrderCastRanksBySingleOption(t *testing.T) {
	d := &decision.Decision{Seq: 3, Player: 0, Kind: decision.KPriority, Min: 1, Max: 1}
	v := view.View{Players: []view.PlayerView{{ID: 0}, {ID: 1}}}
	for i, name := range []string{"Bolt", "Shock", "Opt"} {
		id := state.ObjID(201 + i)
		d.Options = append(d.Options, decision.Option{Index: i, Kind: "cast", Obj: id, Label: name})
		v.Players[0].Hand = append(v.Players[0].Hand, view.CardView{ID: id, Name: name, Types: "Instant"})
	}
	d.Options = append(d.Options, decision.Option{Index: 3, Kind: "pass"})
	bot := decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{0}}
	// Enumeration: bot {Bolt}, pass, Opt, Shock (any order the test chooses).
	choices := [][]int{{0}, {3}, {2}, {1}}
	m := preferCardModel(map[string]float32{"Shock": 3, "Opt": 1})
	keep, changed := priorOrder(m, v, d, bot, "cast", choices, 2)
	if !reflect.DeepEqual(keep, []int{0, 3, 2}) || !changed {
		t.Fatalf("cast: keep %v changed %v, want [0 3 2] changed", keep, changed)
	}
}

// legacyCandidates is the candidate list exactly as candidates() built it
// before the prior existed (Limit-capped enumerators, in their order).
func legacyCandidates(t *testing.T, collector *searchprobe.Collector, d *decision.Decision, bot decision.Intent, f searchprobe.Frame, opts Options) [][]searchprobe.Action {
	t.Helper()
	var out [][]searchprobe.Action
	switch d.Kind {
	case decision.KAttackers:
		for _, in := range searchprobe.AttackCandidates(d, bot, opts.Limit) {
			a, err := collector.Actions(d, in)
			if err != nil {
				return nil
			}
			out = append(out, a)
		}
	case decision.KPriority:
		a, err := collector.Actions(d, bot)
		if err != nil || len(a) != 1 {
			return nil
		}
		for _, c := range searchprobe.Candidates(f.Decision, a[0], opts.Limit) {
			out = append(out, []searchprobe.Action{c})
		}
	}
	return out
}

// Over a real game's eligible decisions: a nil Prior builds today's list byte
// for byte; a Prior keeps the bot's answer first, at most 1+PriorTopK
// candidates, every one drawn from the widened enumeration, and answers
// identically on a second run.
func TestPriorOnRealGameDecisions(t *testing.T) {
	m := policynet.NewModel(policynet.TableRows, 8, 8, rand.New(rand.NewPCG(11, 12)))
	m.ResidualW = 2
	counts := map[decision.Kind]int{}
	ranked, changedN := 0, 0
	for _, seed := range []uint64{7, 8, 9} {
		names, decks := testutil.SampleDecks(t, 2)
		cfg := rules.Config{Seed: seed, Names: names, Decks: decks}
		actor := state.PlayerID(0)
		e := rules.New(cfg)
		e.Advance()
		rngs := searchprobe.BotRandoms(cfg.Seed, len(cfg.Names))
		board := botpolicy.NewBoard(len(cfg.Names))
		feed := NewFeed(actor)
		for steps := 0; !e.G.Over && steps < 3000; steps++ {
			d := e.Pending()
			if d == nil {
				break
			}
			b := botpolicy.BoardFromGameInto(e.G, e, d.Player, &board)
			in := botpolicy.Decide(b, d, rngs[d.Player])
			f, ok := feed.Observe(e)
			if !ok {
				break
			}
			if d.Player == actor {
				off := Defaults()
				if Eligible(d, off) {
					legacy := legacyCandidates(t, feed.Collector(), d, in, f, off)
					got, kind, ok := candidates(feed.Collector(), e, d, in, f, off)
					var tr Trace
					if ok {
						got = applyPrior(feed.Collector(), e, d, in, kind, got, off, &tr)
					}
					if !reflect.DeepEqual(tr, Trace{}) {
						t.Fatalf("seed %d step %d: nil Prior recorded prior diagnostics %+v", seed, steps, tr)
					}
					if !reflect.DeepEqual(got, legacy) {
						t.Fatalf("seed %d step %d: nil Prior list %v, legacy %v", seed, steps, got, legacy)
					}

					on := Defaults()
					on.Prior, on.PriorTopK = m, 2
					run := func() ([][]searchprobe.Action, Trace) {
						var tr Trace
						c, kind, ok := candidates(feed.Collector(), e, d, in, f, on)
						if !ok {
							return nil, tr
						}
						return applyPrior(feed.Collector(), e, d, in, kind, c, on, &tr), tr
					}
					c1, tr1 := run()
					c2, tr2 := run()
					if !reflect.DeepEqual(c1, c2) || !reflect.DeepEqual(tr1, tr2) {
						t.Fatalf("seed %d step %d: prior not deterministic", seed, steps)
					}
					if len(legacy) >= 2 {
						counts[d.Kind]++
						if len(c1) == 0 || !reflect.DeepEqual(c1[0], legacy[0]) {
							t.Fatalf("seed %d step %d: prior list does not start with the bot's answer", seed, steps)
						}
						if tr1.PriorRanked {
							ranked++
							if len(c1) > 1+on.PriorTopK {
								t.Fatalf("seed %d step %d: kept %d > 1+topK", seed, steps, len(c1))
							}
						} else if !reflect.DeepEqual(c1, legacy) {
							t.Fatalf("seed %d step %d: unranked prior list %v, want today's %v", seed, steps, c1, legacy)
						}
						if tr1.PriorChanged {
							changedN++
						}
						wide, _, _ := candidates(feed.Collector(), e, d, in, f, on)
						for _, c := range c1 {
							if !slices.ContainsFunc(wide, func(w []searchprobe.Action) bool { return reflect.DeepEqual(w, c) }) {
								t.Fatalf("seed %d step %d: kept candidate %v not in the widened enumeration", seed, steps, c)
							}
						}
					}
				}
				if err := feed.RecordAnswer(d, in); err != nil {
					t.Fatalf("record: %v", err)
				}
			}
			if err := e.Submit(in); err != nil {
				t.Fatalf("submit: %v", err)
			}
		}
	}
	t.Logf("decisions: attackers %d cast %d; ranked %d, changed %d", counts[decision.KAttackers], counts[decision.KPriority], ranked, changedN)
	if counts[decision.KAttackers] == 0 || ranked == 0 {
		t.Fatalf("fixture exercised no ranked attackers decision (counts %v, ranked %d)", counts, ranked)
	}
}
