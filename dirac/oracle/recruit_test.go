package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
)

// The Recruit lockstep: the eight Standard carriers of
// `DB$ Recruit` (gorge effRecruit) beside the shared draw/discard support and
// basics. All eight carriers lower to OpExt ExtRecruit; Sound the Trumpets
// also carries its Recruit node's ConditionCheckSVar$ X =
// SpellTargeted$CardManaCostLKI guard (ExMVOf over the countered spell).
//
// Nothing else in the main pool draws and then discards in one resolution, so
// a gorge Draw event followed by a discard is a Recruit: the discarded card's
// type decides the soldier token, and the event count at the pick tells the
// no-ask (one-card) case from a choice. The main decks never empty a hand
// (the land-heavy boards keep 2+ cards), so a second scenario adds One with
// Nothing (discard your hand) to reach the empty-hand Recruit: an empty hand
// draws one card and discards it unasked.

var krecruitDecks = [2][]cardset.Entry{
	// A (WR): the four white/red carriers, land-heavy so land discards
	// happen, with dead Opts (no Island) as nonland discards.
	{
		{Name: "Esgaroth Garrison", Count: 3}, {Name: "Great Gilded Boat", Count: 3},
		{Name: "Lake-town Lookout", Count: 3}, {Name: "Long Lake Nuisance", Count: 3},
		{Name: "Lightning Bolt", Count: 4}, {Name: "Grizzly Bears", Count: 4},
		{Name: "Plains", Count: 8}, {Name: "Mountain", Count: 8}, {Name: "Opt", Count: 4},
	},
	// B (UG): the other carriers, including Sound the Trumpets, whose Recruit
	// is guarded by the countered spell's mana value (SpellTargeted$
	// CardManaCostLKI -> ExMVOf(Target0)).
	{
		{Name: "Patient Instructor", Count: 2}, {Name: "The Mountain-king's Return", Count: 2},
		{Name: "The Queen of Dale", Count: 2}, {Name: "Sound the Trumpets", Count: 2},
		{Name: "Hill Giant", Count: 4}, {Name: "Llanowar Elves", Count: 4}, {Name: "Opt", Count: 4},
		{Name: "Island", Count: 10}, {Name: "Forest", Count: 10},
	},
}

// krecruitNoAskDecks empty a hand with One with Nothing, then cast an
// enter-trigger Recruit carrier from it: the ETB's draw leaves exactly one
// card, which Recruit discards without a pick.
var krecruitNoAskDecks = [2][]cardset.Entry{
	{
		{Name: "Esgaroth Garrison", Count: 4}, {Name: "Long Lake Nuisance", Count: 4},
		{Name: "Patient Instructor", Count: 4}, {Name: "One with Nothing", Count: 4},
		{Name: "Opt", Count: 4}, {Name: "Plains", Count: 8},
		{Name: "Island", Count: 8}, {Name: "Swamp", Count: 4},
	},
	{
		{Name: "Grizzly Bears", Count: 4}, {Name: "Hill Giant", Count: 4},
		{Name: "Lightning Bolt", Count: 4}, {Name: "Llanowar Elves", Count: 4},
		{Name: "Plains", Count: 8}, {Name: "Mountain", Count: 8}, {Name: "Forest", Count: 8},
	},
}

// krecruitCounts is the coverage the test reports.
type krecruitCounts struct {
	games, divs, stalls, capStalls         int
	recruits, landNoTok, nonlandTok, noAsk int
}

func (c krecruitCounts) add(o krecruitCounts) krecruitCounts {
	return krecruitCounts{
		games: c.games + o.games, divs: c.divs + o.divs,
		stalls: c.stalls + o.stalls, capStalls: c.capStalls + o.capStalls,
		recruits: c.recruits + o.recruits, landNoTok: c.landNoTok + o.landNoTok,
		nonlandTok: c.nonlandTok + o.nonlandTok, noAsk: c.noAsk + o.noAsk,
	}
}

// krecruitPolicy casts whenever it can, then plays a land or activates, else
// picks at random, and records the event count when gorge poses a Recruit
// discard pick (so a one-card no-ask discard has no entry there).
func krecruitPolicy(seed uint64, asks map[int]bool) Policy {
	rnd := RandomPolicy(seed)
	var buf []Canon
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.G != nil {
			if d := c.G.Pending(); d != nil && d.ResumeKind == "discard" &&
				strings.HasPrefix(d.Prompt, "Recruit:") {
				asks[len(c.G.E.L.Events)] = true
			}
		}
		buf = buf[:0]
		for _, a := range c.D.Options {
			if a.Kind() == ActCast {
				buf = append(buf, a)
			}
		}
		if len(buf) == 0 {
			for _, a := range c.D.Options {
				if a.Kind() == ActPlayLand {
					buf = append(buf, a)
				}
			}
		}
		if len(buf) == 0 {
			for _, a := range c.D.Options {
				if a.Kind() == ActActivate || a.Kind() == ActSpecial {
					buf = append(buf, a)
				}
			}
		}
		if len(buf) == 0 {
			return rnd.Pick(c)
		}
		d := *c.D
		d.Options = buf
		return rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
	})
}

// krecruitRun plays one deck pair and counts from gorge's events.
func krecruitRun(t *testing.T, reg *cards.Registry, set cardset.Set, tbl [2][]cardset.Entry, decks [2][]*cards.Card, seeds int) krecruitCounts {
	t.Helper()
	var out krecruitCounts
	tblv, _, _, err := set.Matchup(tbl[0], tbl[1])
	if err != nil {
		t.Fatal(err)
	}
	for seed := uint64(1); seed <= uint64(seeds); seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"krecruit-a", "krecruit-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tblv, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		asks := map[int]bool{}
		g, err := NewGorgeSide(m, seed, cfg, nil)
		if err != nil {
			t.Fatal(err)
		}
		res := LockstepLed(m, seed, g, k, krecruitPolicy(seed, asks),
			RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20})
		CloseSide(k)
		out.games++
		if d := res.Div; d != nil {
			out.divs++
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v",
				seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
		}
		if res.Stall != "" {
			if strings.HasPrefix(res.Stall, KernelStallPrefix) {
				out.stalls++
				t.Errorf("seed %d: kernel stall: %s", seed, res.Stall)
			} else {
				out.capStalls++
			}
		}
		// The recruit's draw is followed by its one discard, with only the
		// discard pick's decision events between when the hand had a choice.
		evs := g.E.L.Events
		for i := 0; i < len(evs); i++ {
			if evs[i].Kind != events.Draw {
				continue
			}
			j := i + 1
			for j < len(evs) && (evs[j].Kind == events.DecisionAsk || evs[j].Kind == events.DecisionMade) {
				j++
			}
			if j >= len(evs) {
				break
			}
			dis := evs[j]
			if !events.IsDiscard(dis) || dis.Player != evs[i].Player {
				continue
			}
			out.recruits++
			ob := g.E.G.Obj(dis.Obj)
			land := ob != nil && ob.Face() != nil && ob.Face().IsLand()
			tok := j+1 < len(evs) && evs[j+1].Kind == events.TokenCreate &&
				evs[j+1].Text == "w_1_1_human_soldier"
			switch {
			case land && !tok:
				out.landNoTok++
			case !land && tok:
				out.nonlandTok++
			}
			asked := false
			for x := i + 1; x <= j; x++ {
				asked = asked || asks[x]
			}
			if !asked {
				out.noAsk++
			}
			i = j // step past the discard
		}
	}
	return out
}

func TestRecruitLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("recruit")
	if err != nil {
		t.Fatal(err)
	}
	decksOf := func(md [2][]cardset.Entry) [2][]*cards.Card {
		var decks [2][]*cards.Card
		for s, d := range md {
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
				t.Fatalf("deck %d: %d cards", s, len(decks[s]))
			}
		}
		return decks
	}
	main := krecruitRun(t, reg, set, krecruitDecks, decksOf(krecruitDecks), 256)
	noAsk := krecruitRun(t, reg, set, krecruitNoAskDecks, decksOf(krecruitNoAskDecks), 64)
	all := main.add(noAsk)
	t.Logf("main: %d games, %d divergences, %d stalls (%d cap); recruits %d, land-no-token %d, nonland-token %d, one-card-no-ask %d",
		main.games, main.divs, main.stalls, main.capStalls, main.recruits, main.landNoTok, main.nonlandTok, main.noAsk)
	t.Logf("no-ask scenario: %d games, %d divergences, %d stalls (%d cap); recruits %d, one-card-no-ask %d",
		noAsk.games, noAsk.divs, noAsk.stalls, noAsk.capStalls, noAsk.recruits, noAsk.noAsk)
	t.Logf("total: %d games, %d divergences, %d stalls (%d cap); recruits %d, land-no-token %d, nonland-token %d, one-card-no-ask %d",
		all.games, all.divs, all.stalls, all.capStalls, all.recruits, all.landNoTok, all.nonlandTok, all.noAsk)
	if all.divs != 0 {
		t.Fatalf("%d divergences", all.divs)
	}
	if all.stalls != 0 {
		t.Fatalf("%d kernel stalls", all.stalls)
	}
	if all.recruits < 40 {
		t.Errorf("only %d Recruit resolutions", all.recruits)
	}
	if all.landNoTok < 15 {
		t.Errorf("only %d land discards with no token", all.landNoTok)
	}
	if all.nonlandTok < 15 {
		t.Errorf("only %d nonland discards with a token", all.nonlandTok)
	}
	if all.noAsk < 5 {
		t.Errorf("only %d one-card no-ask Recruits", all.noAsk)
	}
}
