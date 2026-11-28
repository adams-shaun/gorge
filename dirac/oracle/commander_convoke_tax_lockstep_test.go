package oracle

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kgen"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestCommanderConvokeTaxLockstep: a commander's Convoke taps pay the TOTAL
// cost (CR 702.51a) and the CR 903.8 tax is part of that total, so the
// kernel's command-zone offer credits the greedy tap credit over the taxed
// total, not the printed cost alone (dirac offerCommanderSlot; gorge
// offerCastableCommander/convokeTaxOfferCredit).
//
// The position: Winter, Team Player ({4}{R}, Convoke) cast once from the
// command zone (her twelve untapped Goblin Digging Teams pay the printed
// {4}{R} with the announcement, the mana pool staying empty -- the offer
// gates price the pool, so no land could pre-float it and a land's mana
// would pay the tax itself), killed in combat, back in the command zone
// with the {2} tax and seven Teams still untapped -- credit for the whole
// {6}{R} and no mana at all. gorge must offer the taxed cast (the engine
// test TestCommanderConvokePaysTax pins its side) and the kernel must
// offer the same cast: Lockstep compares the option lists, so a kernel
// that still prices the credit before the tax diverges exactly at that
// decision. The mutation leg flips dirac.DebugCommanderNoCredit and
// requires the divergence, so the test cannot pass without reaching the
// offer.
const (
	convokeTaxCmd     = "Winter, Team Player" // {4}{R} 3/3, one noncreature-spell trigger that never fires here
	convokeTaxCredit  = "Goblin Digging Team" // 1/1 red: Convoke pays {1} or {R} (CR 702.51a)
	convokeTaxBlocker = "Kavu Runner"         // a vanilla 3/3: trades with the commander
)

// convokeTaxCreditCount is the credit bench: twelve Teams pay the first
// cast's {4}{R} (five of them) and leave seven untapped for the taxed
// {6}{R} -- seven taps, the {R} pip and the {6} generic.
const convokeTaxCreditCount = 12

// convokeTaxScenario is the four-seat pod (Commander is four players, and
// the scenario validator enforces it): seat 0 the protagonist, seat 1 the
// blocker, seats 2 and 3 empty and passive.
var convokeTaxScenario = func() string {
	bench := make([]string, convokeTaxCreditCount)
	for i := range bench {
		bench[i] = `"` + convokeTaxCredit + `"`
	}
	return fmt.Sprintf(`{"format":"gorge-start-1","focus":"%s","players":4,"game":"commander",
 "opponent":"passive","active":0,"turn":1,"phase":"main1","max_turns":10,
 "setup":{"p0":{"commanders":["%s"],
                 "battlefield":[%s]},
          "p1":{"commanders":["Ghalta, Primal Hunger"],"battlefield":["%s"]},
          "p2":{"commanders":["Ghalta, Primal Hunger"]},
          "p3":{"commanders":["Ghalta, Primal Hunger"]}}}`,
		convokeTaxCmd, convokeTaxCmd, strings.Join(bench, ","), convokeTaxBlocker)
}()

// convokeTaxStats counts the runs that reached the taxed position with the
// cast offered (witnessed) and reached it withheld on both sides (short).
type convokeTaxStats struct {
	witnessed     int
	short         int
	field         int // decisions where the commander was on the battlefield
	cmdZone       int // decisions where she was in the command zone (taxed or not)
	castSeen      int // priority decisions that carried an ActCast for the commander
	last          *state.Game
	lastTurn      int32
	lastStall     string
	lastDecis     int
	optDump       string
	fieldNameDump string
	zoneDump      string
	poolMax       int32
}

// convokeTaxPolicy plays the scripted game deterministically: seat 0 casts
// its commander from the command zone once with the whole announcement paid
// by Convoke taps (five Teams: one for the {R} pip, four for the generic --
// the mana pool never holds anything, so the OFFER below is priced purely
// on the credit), attacks with the commander alone, and passes forever
// after; seat 1 does nothing but block the commander with the Kavu Runner
// (which kills it, sending it back to the command zone under CR 903.9 --
// the policy answers the owner's ask with "command zone"). At every seat 0
// priority with the taxed position up it counts whether the cast is offered
// and passes: the offer is the test's witness, the second cast is not.
func convokeTaxPolicy(seed uint64, st *convokeTaxStats) Policy {
	rnd := RandomPolicy(seed)
	pass := Pack(ActPass, RefNone, RefNone, 0)
	done := Pack(ActDone, RefNone, RefNone, 0)
	convCode, _ := chooseCode("convoke")
	chooseConvoke := ActChoose + CanonKind(convCode)
	redMode := strings.Index(manaSyms, "R")
	convR, convG := 0, 0 // Teams tapped for the first cast's {R} pip and {4}
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.G == nil {
			return rnd.Pick(c)
		}
		d := c.G.Pending()
		if d == nil {
			return rnd.Pick(c)
		}
		switch d.Kind {
		case decision.KPriority:
			st.last, st.lastTurn, st.lastDecis = c.G.E.G, c.G.E.G.Turn, c.Seq
			for _, o := range c.D.Options {
				if o.Kind() == ActCast {
					st.castSeen++
					break
				}
			}
			for _, id := range c.G.E.G.Zone(state.ZBattlefield, 0) {
				if o := c.G.E.G.Obj(id); o != nil && o.Face() != nil && o.Face().Name == convokeTaxCmd {
					st.field++
				}
			}
			for _, id := range c.G.E.G.Zone(state.ZCommand, 0) {
				if o := c.G.E.G.Obj(id); o != nil && o.Face() != nil && o.Face().Name == convokeTaxCmd {
					st.cmdZone++
				}
			}
			if d.Player != 0 {
				return pass // seat 1 never acts (no Ghalta cast, no attacks)
			}
			g := c.G.E.G
			if st.optDump == "" {
				for _, o := range d.Options {
					st.optDump += o.Kind + ":" + o.Label + " "
				}
				for _, id := range g.Zone(state.ZBattlefield, 0) {
					if o := g.Obj(id); o != nil && o.Face() != nil {
						st.fieldNameDump += o.Face().Name + ","
					}
				}
				for _, z := range []state.Zone{state.ZHand, state.ZCommand, state.ZLibrary} {
					for _, id := range g.Zone(z, 0) {
						o := g.Obj(id)
						if o == nil || o.Face() == nil {
							continue
						}
						if st.zoneDump == "" || z != state.ZLibrary {
							st.zoneDump += fmt.Sprintf("%s#%d=%s ", z, c.G.InstRef(id), o.Face().Name)
						}
					}
				}
			}
			if p := g.Players[0].Pool[state.MR]; p > st.poolMax {
				st.poolMax = p
			}
			onField := false
			for _, id := range g.Zone(state.ZBattlefield, 0) {
				if o := g.Obj(id); o != nil && o.Face() != nil && o.Face().Name == convokeTaxCmd {
					onField = true
				}
			}
			var cmd state.ObjID
			for _, id := range g.Zone(state.ZCommand, 0) {
				if o := g.Obj(id); o != nil && o.Face() != nil && o.Face().Name == convokeTaxCmd {
					cmd = id
				}
			}
			if onField || cmd == 0 {
				return pass // the attack phase does the rest; nothing to cast
			}
			taxed := g.Players[0].CmdCasts[0] > 0
			if taxed && !(g.Active == 0 && g.Step.IsMain()) {
				return pass // sorcery speed: the offer only exists in p0's own main phases
			}
			want := c.G.InstRef(cmd)
			for _, o := range c.D.Options {
				if o.Kind() == ActCast && o.Src() == want {
					if taxed {
						st.witnessed++
						return pass // the offer is the witness; the cast stays unmade
					}
					convR, convG = 0, 0 // the announcement pays the printed cost
					return o
				}
			}
			if taxed {
				st.short++
			}
			return pass
		case decision.KCommanderZone:
			for _, o := range c.D.Options {
				if o.Kind() == ActCommanderZone && o.Mode() == 0 {
					return o // option 0: put it into the command zone
				}
			}
			for _, o := range c.D.Options {
				if o.Kind() == ActCommanderZone {
					return o
				}
			}
			return rnd.Pick(c)
		case decision.KChoose:
			// The first cast's Convoke announcement: one untapped Team per
			// pick, the red pip first so the {R} is covered, then four
			// generic pips. Done appears only once the announcement balances
			// (gorge's convokeFrame withholds it from an incomplete answer),
			// so returning it here asserts rather than assumes the balance.
			if d.Player == 0 && len(d.Options) > 0 && strings.HasPrefix(d.Options[0].Kind, "convoke_") {
				if convR+convG >= 5 {
					return done
				}
				want := redMode
				if convR > 0 {
					want = 5 // convoke_generic
				}
				for _, o := range d.Options {
					m := 5
					if o.Kind != "convoke_generic" {
						m = strings.Index(manaSyms, strings.TrimPrefix(o.Kind, "convoke_"))
					}
					if m != want {
						continue
					}
					for _, a := range c.D.Options {
						if a.Kind() == chooseConvoke && a.Src() == c.G.InstRef(o.Obj) && int(a.Mode()) == m {
							if m == 5 {
								convG++
							} else {
								convR++
							}
							return a
						}
					}
				}
				return done
			}
		}
		switch c.D.Kind {
		case DAttackers:
			if d.Player == 0 {
				for _, o := range c.D.Options {
					if o.Kind() == ActAttack && gorgeOptionName(c, o) == convokeTaxCmd {
						return o // the commander alone; the Teams stay home untapped
					}
				}
			}
			return done
		case DBlockers:
			for _, o := range c.D.Options {
				if o.Kind() == ActBlock && gorgeOptionName(c, o) == convokeTaxBlocker {
					return o // the 3/3 takes it down with it
				}
			}
			return done
		}
		return rnd.Pick(c)
	})
}

func TestCommanderConvokeTaxLockstep(t *testing.T) {
	t.Parallel()
	sc, err := ParseScenario([]byte(convokeTaxScenario))
	if err != nil {
		t.Fatal(err)
	}
	c, err := OpenCorpus(corpusDirOrSkip(t), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	m, err := c.ScenarioMatchup(sc, 0)
	if err != nil {
		t.Fatal(err)
	}
	var allNames []string
	var decks [4][]*cards.Card
	for p := range decks {
		decks[p] = m.SeatDeck(p)
		for _, card := range decks[p] {
			allNames = append(allNames, cardName(card))
		}
	}
	set := cardset.Raw(convokeTaxSet(t, allNames))
	entries := func(d []*cards.Card) []cardset.Entry {
		var out []cardset.Entry
		for _, card := range d {
			n := cardName(card)
			i := slices.IndexFunc(out, func(e cardset.Entry) bool { return e.Name == n })
			if i < 0 {
				out = append(out, cardset.Entry{Name: n})
				i = len(out) - 1
			}
			out[i].Count++
		}
		return out
	}
	es := make([][]cardset.Entry, 0, 4)
	for _, d := range decks {
		es = append(es, entries(d))
	}
	tbl, _, err := set.MatchupN(es)
	if err != nil {
		t.Fatal(err)
	}
	rc := RunConfig{Config: Config{}, MaxDecisions: 2500, MaxTurns: 10}
	st := &convokeTaxStats{}
	for seed := uint64(1); seed <= 4 && st.witnessed == 0; seed++ {
		k, err := SetSide(set, tbl, rc.Config)(m, seed)
		if err != nil {
			t.Fatalf("seed %d: %v", seed, err)
		}
		res := Lockstep(m, seed, k, convokeTaxPolicy(seed, st), rc)
		CloseSide(k)
		if res.Div != nil {
			t.Fatalf("seed %d: divergence at decision %d: %s %s %v (%s)\nref %v\nker %v\nzones %s | field %s | opts %s",
				seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff, res.Div.Detail,
				res.Div.RefOptions, res.Div.KerOptions, st.zoneDump, st.fieldNameDump, st.optDump)
		}
	}
	if st.witnessed == 0 {
		zone := "nowhere"
		casts := int32(-1)
		if st.last != nil {
			casts = st.last.Players[0].CmdCasts[0]
			for _, z := range []state.Zone{state.ZCommand, state.ZBattlefield, state.ZGraveyard, state.ZExile, state.ZLibrary, state.ZHand, state.ZStack} {
				for _, id := range st.last.Zone(z, 0) {
					if o := st.last.Obj(id); o != nil && o.Face() != nil && o.Face().Name == convokeTaxCmd {
						zone = z.String()
					}
				}
			}
		}
		t.Fatalf("no seed reached the taxed position with the cast offered in 4 games (short %d, cast-option-seen %d, on-field decisions %d, command-zone decisions %d, commander zone %s, CmdCasts %d, last turn %d at decision %d, poolMax %d, first p0 options %q, p0 field %q, zones %s): the position must be reached and offered",
			st.short, st.castSeen, st.field, st.cmdZone, zone, casts, st.lastTurn, st.lastDecis, st.poolMax, st.optDump, st.fieldNameDump, st.zoneDump)
	}
	if st.short != 0 {
		t.Fatalf("the taxed position was reached %d times WITHOUT the offer, though seven untapped Teams alone credit the whole {6}{R} tax total", st.short)
	}
	t.Logf("%d taxed-position decisions all offered the cast on both sides", st.witnessed)

	// The mutation leg: with the kernel's command-zone credit off, it withholds
	// the taxed cast gorge offers -- the replay must diverge at it.
	dirac.DebugCommanderNoCredit = true
	defer func() { dirac.DebugCommanderNoCredit = false }()
	k, err := SetSide(set, tbl, rc.Config)(m, 1)
	if err != nil {
		t.Fatal(err)
	}
	res := Lockstep(m, 1, k, convokeTaxPolicy(1, &convokeTaxStats{}), rc)
	CloseSide(k)
	if res.Div == nil {
		t.Fatal("the uncredited kernel offer changed no game: the replay does not reach the taxed convoke cast")
	}
}

// convokeTaxSet is printedDelveSet's four-seat shape: the kernel table for
// the decks' own cards (a [2][]string fixture cannot hold a four-seat pod).
func convokeTaxSet(t *testing.T, cardNames []string) cardset.Set {
	t.Helper()
	names := slices.Clone(cardNames)
	slices.Sort(names)
	names = slices.Compact(names)
	cardsDir, err := cardset.CorpusDir()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	list := filepath.Join(dir, "cards.txt")
	if err := os.WriteFile(list, []byte(strings.Join(names, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "out")
	if err := os.Mkdir(out, 0o755); err != nil {
		t.Fatal(err)
	}
	if code := kgen.Main([]string{"-cards", cardsDir, "-card-list", list, "-pool", "convoke-tax", "-out", out}); code != 0 {
		t.Fatalf("kgen: exit %d", code)
	}
	pd, err := cardset.LoadFile(filepath.Join(out, "convoke-tax.ktab"))
	if err != nil {
		t.Fatal(err)
	}
	return cardset.AllowUnverified(cardset.NewDataSet(pd))
}
