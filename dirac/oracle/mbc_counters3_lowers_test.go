package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestMBCCounters3Lowers plays the MBC counters-3 spellings kgen lowers in
// lockstep with gorge over the mbccounters3 fixture pool
// (dirac/cardset/pools/fixtures/mbccounters3), one subtest per spelling:
//
//   - requiem: Vile Requiem's "up to X target nonblack creatures, X the verse
//     counters on CARDNAME" (TargetMax$ X over Count$CardCounters.VERSE), whose
//     bound reads the source's own counters as the slot is announced;
//   - verse: the same bound on Recantation (permanents), Rumbling Crescendo
//     (lands) and Serra's Liturgy (artifacts and enchantments);
//   - core: Magmatic Core's divided damage over the same bound (its age
//     counters);
//   - grimdancer: the as-enters choice of two different counters from three.
//
// Each subtest's probe counts the effect it exists for and fails when it never
// happened, so a refused card (the pool would not build) or a game that never
// reaches the shape fails as surely as a divergence.
func TestMBCCounters3Lowers(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name  string
		decks [2][]cardset.Entry
		pol   func(seed uint64, p *mbcCounters3Tally) Policy
		// the least tally the games must reach
		min mbcCounters3Tally
	}{
		{"requiem", mbcRequiemDecks, mbcVersePolicy, mbcCounters3Tally{ctr: 100, acts: 20}},
		{"verse", mbcVerseDecks, mbcVersePolicy, mbcCounters3Tally{ctr: 100, acts: 20}},
		{"core", mbcCoreDecks, mbcVersePolicy, mbcCounters3Tally{ctr: 100, acts: 10}},
		{"grimdancer", mbcGrimdancerDecks, mbcPlainPolicy, mbcCounters3Tally{ctr: 120, asks: 60}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var tally mbcCounters3Tally
			pol := func(seed uint64) Policy { return mbcCounters3Probe(tc.pol(seed, &tally), &tally) }
			if div := lockstepRunPolicy(t, "mbccounters3", tc.decks, 120, true, nil, nil, pol); div != 0 {
				return // each divergence is already a test error
			}
			t.Logf("%s: %+v", tc.name, tally)
			if tally.ctr < tc.min.ctr || tally.acts < tc.min.acts || tally.asks < tc.min.asks {
				t.Fatalf("%s: tally %+v, want at least %+v over 120 seeds: the pool does not exercise it", tc.name, tally, tc.min)
			}
		})
	}
}

// mbcCounters3Tally is what a game's gorge log shows of the shape under test.
type mbcCounters3Tally struct {
	ctr  int // verse or age counters placed, or keyword counters (Grimdancer)
	acts int // activations of a verse card, and Savannah Lions deaths
	asks int // "counter_kinds" asks
}

func mbcPlainPolicy(seed uint64, _ *mbcCounters3Tally) Policy { return castFirstPolicy(seed) }

// mbcVerseCards are the pool cards whose activation reads their own counters.
var mbcVerseCards = map[string]bool{"Vile Requiem": true, "Recantation": true, "Rumbling Crescendo": true, "Serra's Liturgy": true}

// mbcVersePolicy is castFirstPolicy that sacrifices a verse card only once it
// holds the seed's number of counters (one to three; castFirstPolicy would, the
// moment it holds one, and the slot would only ever be announced at X = 1).
func mbcVersePolicy(seed uint64, _ *mbcCounters3Tally) Policy {
	base := castFirstPolicy(seed)
	want := int32(seed%3) + 1
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.G == nil || c.D.Kind != DPriority {
			return base.Pick(c)
		}
		d := *c.D
		d.Options = nil
		for _, a := range c.D.Options {
			if a.Kind() == ActActivate && mbcVerseCards[refCardName(c.G, a.Src())] {
				if o := c.G.E.G.Obj(c.G.id.obj[a.Src()]); o == nil || o.Counter("VERSE") < want {
					continue
				}
			}
			d.Options = append(d.Options, a)
		}
		return base.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
	})
}

// mbcCounters3Probe wraps base and tallies, when gorge leads, what its log
// gains: counters placed (verse, age, and the keyword counters of Grimdancer),
// activations of the pool's verse cards, Savannah
// Lions deaths, and the "counter_kinds" asks it poses.
func mbcCounters3Probe(base Policy, t *mbcCounters3Tally) Policy {
	seen := 0 // the log position read so far (one log per game, one probe per game)
	return PolicyFunc(func(c *PickCtx) Canon {
		a := base.Pick(c)
		if c.G == nil {
			return a
		}
		if fr := c.G.fr; fr != nil && fr.d != nil && fr.d.ResumeKind == "counter_kinds" {
			t.asks++
		}
		evs := c.G.E.L.Events
		for ; seen < len(evs); seen++ {
			e := evs[seen]
			name := ""
			if o := c.G.E.G.Obj(e.Obj); o != nil && o.Card != nil && len(o.Card.Faces) > 0 {
				name = o.Card.Faces[0].Name
			}
			switch {
			case e.Kind == events.CounterChange && e.Amount > 0 && !strings.EqualFold(e.Counter, "P1P1"):
				t.ctr++
			case e.Kind == events.AbilityPush && mbcVerseCards[name]:
				t.acts++
			case e.Kind == events.MoveZone && e.From == state.ZBattlefield && e.To == state.ZGraveyard && name == "Savannah Lions":
				t.acts++
			}
		}
		return a
	})
}

// mbcRequiemDecks: Vile Requiem and the mana for it on one side, nonblack
// bodies for it to destroy on the other.
var mbcRequiemDecks = [2][]cardset.Entry{
	{{Name: "Vile Requiem", Count: 12}, {Name: "Swamp", Count: 28}},
	{{Name: "Savannah Lions", Count: 16}, {Name: "Plains", Count: 24}},
}

// mbcVerseDecks: the three other verse enchantments, with lands, bodies and
// enchantments for them to hit.
var mbcVerseDecks = [2][]cardset.Entry{
	{{Name: "Recantation", Count: 7}, {Name: "Rumbling Crescendo", Count: 7}, {Name: "Mountain", Count: 13}, {Name: "Island", Count: 13}},
	{{Name: "Serra's Liturgy", Count: 7}, {Name: "Vile Requiem", Count: 3}, {Name: "Savannah Lions", Count: 8}, {Name: "Plains", Count: 12}, {Name: "Swamp", Count: 10}},
}

// mbcCoreDecks: Magmatic Core's end-step damage needs creatures to divide it
// among.
var mbcCoreDecks = [2][]cardset.Entry{
	{{Name: "Magmatic Core", Count: 14}, {Name: "Mountain", Count: 26}},
	{{Name: "Savannah Lions", Count: 16}, {Name: "Plains", Count: 24}},
}

// mbcGrimdancerDecks: Grimdancers to cast on both sides.
var mbcGrimdancerDecks = [2][]cardset.Entry{
	{{Name: "Grimdancer", Count: 16}, {Name: "Swamp", Count: 24}},
	{{Name: "Grimdancer", Count: 10}, {Name: "Savannah Lions", Count: 6}, {Name: "Swamp", Count: 12}, {Name: "Plains", Count: 12}},
}
