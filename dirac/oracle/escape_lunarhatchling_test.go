package oracle

import (
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// Lunar Hatchling's escape cost (Escape-{4}{G}{U}, Exile a land you control,
// Exile five other cards from your graveyard) has a battlefield-exile part
// beside the graveyard one (CardDef.EscBfCost). The directed repro game is
// escape_carriers_test.go's lunar_hatchling_escape_cast; this file pins the
// boundary the cost raises: the land exiled is one of the six that pay the
// cast's mana, so with exactly six untapped lands in play the offer depends on
// whether a land a mana ability taps for the cast is a candidate. gorge
// decides: its planner prices no escape mode (potentialModeBaseCost has no
// arm), so it offers the cast only while the floating pool pays it, never from
// untapped lands, with six lands or seven. The kernel's offer must match at
// every decision (the lockstep compares the option lists); the test pins what
// gorge decided. The recorded game (Lotus Cobra's floating mana) is the
// escape cast taken; TestEscapeLunarHatchlingExiles reads its payment order.

// hatchlingBoundaryDecks are a mill-and-ramp-free build: two Hatchlings, the
// graveyard fodder spells (Mulch puts the non-lands of the top four into the
// graveyard, Thrill of Possibility discards) and basics.
var hatchlingBoundaryDecks = [2][]cardset.Entry{
	{
		{Name: "Lunar Hatchling", Count: 2}, {Name: "Mulch", Count: 10}, {Name: "Thrill of Possibility", Count: 4},
		{Name: "Island", Count: 8}, {Name: "Forest", Count: 16},
	},
	{{Name: "Plains", Count: 40}},
}

// hatchlingTally counts, for one land cap and surface, the decisions at which
// the decider had the Hatchling and five other cards in the graveyard and
// exactly landCap untapped lands in its main phase with an empty stack (the
// boundary), how many of them offered the escape cast, and the escape casts
// taken.
type hatchlingTally struct {
	boundary, offered, cast int
	maxLands                int // most lands the decider ever had in play
}

// hatchlingPolicy plays the lands up to landCap (and no more), discards the
// Hatchling, casts the fodder spells, and takes the escape cast whenever it is
// offered. Everything else is the seed's random play.
func hatchlingPolicy(seed uint64, landCap int, tally *hatchlingTally) Policy {
	base := RandomPolicy(seed)
	return PolicyFunc(func(p *PickCtx) Canon {
		if p.G == nil || p.D.Kind != DPriority {
			return base.Pick(p)
		}
		g := p.G.E.G
		pl := state.PlayerID(p.D.Player)
		hatchling, others := 0, 0
		for _, id := range g.Zone(state.ZGraveyard, pl) {
			if escapeObjName(p.G, id) == "Lunar Hatchling" {
				hatchling++
			} else {
				others++
			}
		}
		lands, untapped := 0, 0
		for _, id := range g.Zone(state.ZBattlefield, pl) {
			switch escapeObjName(p.G, id) {
			case "Island", "Forest":
				lands++
				if ob := g.Obj(id); ob != nil && !ob.Tapped {
					untapped++
				}
			}
		}
		tally.maxLands = max(tally.maxLands, lands)
		var esc, disc, spell, keep []Canon
		for _, x := range p.D.Options {
			o, ok := p.G.GorgeOption(x)
			if !ok {
				keep = append(keep, x)
				continue
			}
			name := escapeObjName(p.G, o.Obj)
			switch {
			case o.Kind == "cast" && o.Mode == "escape" && name == "Lunar Hatchling":
				esc = append(esc, x)
			case o.Kind == "cast" && name == "Lunar Hatchling":
				// the plain cast would keep the Hatchling out of the graveyard
			case o.Kind == "play_land" && lands >= landCap:
				// the cap holds the land count at the boundary
			case o.Kind == "discard" && name == "Lunar Hatchling":
				disc = append(disc, x)
			case o.Kind == "cast" && (name == "Mulch" || name == "Thrill of Possibility"):
				spell = append(spell, x)
				keep = append(keep, x)
			default:
				keep = append(keep, x)
			}
		}
		if hatchling > 0 && others >= 5 && lands == landCap && untapped == lands && len(g.Stack) == 0 && g.Step.IsMain() && g.Active == pl {
			tally.boundary++
			if len(esc) > 0 {
				tally.offered++
			}
		}
		if len(esc) > 0 {
			tally.cast++
			return esc[0]
		}
		if len(disc) > 0 {
			return disc[0]
		}
		// the filtered menu, preferring the fodder spells
		d := *p.D
		d.Options = keep
		if len(spell) > 0 && seed%3 != 0 {
			d.Options = spell
		}
		return base.Pick(&PickCtx{D: &d, G: p.G, Seq: p.Seq})
	})
}

// hatchlingBoundaryRun plays seeds 1..seeds with the land cap on one surface
// and returns the tally; a divergence or a stalled game fails the test.
func hatchlingBoundaryRun(t *testing.T, landCap int, autopay bool, seeds uint64) hatchlingTally {
	t.Helper()
	reg, err := cards.SharedCorpus(corpusDirOrSkip(t))
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("escape")
	if err != nil {
		t.Fatal(err)
	}
	tbl, _, _, err := set.Matchup(hatchlingBoundaryDecks[0], hatchlingBoundaryDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, d := range hatchlingBoundaryDecks {
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
	var tally hatchlingTally
	for seed := uint64(1); seed <= seeds; seed++ {
		cfg := Config{GorgeAutopaySurface: autopay}
		m, err := NewMatchup(reg, [2]string{"hatchling-a", "hatchling-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		res := Lockstep(m, seed, k, hatchlingPolicy(seed, landCap, &tally), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20})
		CloseSide(k)
		if d := res.Div; d != nil {
			t.Fatalf("seed %d (landCap %d, autopay %v): diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, landCap, autopay, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
		}
	}
	return tally
}

// TestEscapeLunarHatchlingBoundary: with the lands capped at six and at seven,
// the kernel offers Lunar Hatchling's escape exactly when gorge does, on both
// surfaces: never from untapped lands. Each run reaches its boundary (the
// precondition: the Hatchling and five other cards in the graveyard, landCap
// untapped lands, an empty stack in the decider's main phase), where the
// lockstep compares gorge's option list with the kernel's.
func TestEscapeLunarHatchlingBoundary(t *testing.T) {
	for _, c := range []struct {
		landCap int
		autopay bool
	}{{6, false}, {7, false}, {6, true}, {7, true}} {
		tally := hatchlingBoundaryRun(t, c.landCap, c.autopay, 120)
		if tally.offered != 0 || tally.cast != 0 {
			t.Errorf("landCap %d autopay %v: gorge offered the escape from untapped lands: %+v", c.landCap, c.autopay, tally)
		}
		if tally.maxLands != c.landCap {
			t.Errorf("landCap %d autopay %v: the decider reached %d lands", c.landCap, c.autopay, tally.maxLands)
		}
		if tally.boundary == 0 {
			t.Errorf("landCap %d autopay %v: no decision with the Hatchling, five other graveyard cards and %d untapped lands", c.landCap, c.autopay, c.landCap)
		}
	}
}

// hatchlingPayment is what the recorded game's gorge log shows around the
// Hatchling's escape cast.
type hatchlingPayment struct {
	cast        bool
	graveBefore int      // owner 0's graveyard cards when the cast began, the Hatchling among them
	exiled      []string // the first six cards exiled after the cast began, "from:name"
	escapedID   state.ObjID
}

func readHatchlingPayment(g *GorgeSide) hatchlingPayment {
	var out hatchlingPayment
	grave := 0
	for _, e := range g.E.L.Events {
		if !out.cast {
			if e.Kind == events.PutOnStack && e.From == state.ZGraveyard && escapeObjName(g, e.Obj) == "Lunar Hatchling" {
				out.cast, out.graveBefore, out.escapedID = true, grave, e.Obj
				continue
			}
			if e.Kind == events.MoveZone {
				if ob := g.E.G.Obj(e.Obj); ob != nil && ob.Owner == 0 {
					if e.To == state.ZGraveyard {
						grave++
					}
					if e.From == state.ZGraveyard {
						grave--
					}
				}
			}
			continue
		}
		if e.Kind == events.MoveZone && e.To == state.ZExile && len(out.exiled) < 6 {
			from := "battlefield"
			if e.From == state.ZGraveyard {
				from = "graveyard"
			}
			out.exiled = append(out.exiled, from+":"+escapeObjName(g, e.Obj))
			if e.Obj == out.escapedID {
				out.exiled[len(out.exiled)-1] += "(self)"
			}
		}
	}
	return out
}

// TestEscapeLunarHatchlingExiles replays the recorded escape game and reads
// the payment from gorge's log: the cast comes from a graveyard holding the
// Hatchling and at least five other cards, and the cost exiles one land from
// the battlefield first and then five OTHER graveyard cards (gorge's exAsk
// walks the Exile<1/Land> token before the ExileFromGrave<5/Card.Other>
// one). The kernel agrees at every decision and every event field
// (TestEscapeReprosReach); this pins the order the log shows, which the
// exile zone's order carries.
func TestEscapeLunarHatchlingExiles(t *testing.T) {
	reg := synthReg(t)
	var c escapeCase
	for _, x := range escapeReproCases() {
		if x.carrier == "Lunar Hatchling" {
			c = x
		}
	}
	if c.carrier == "" {
		t.Fatal("no Lunar Hatchling repro case")
	}
	var got hatchlingPayment
	c.reached = func(g *GorgeSide) bool {
		got = readHatchlingPayment(g)
		return got.cast
	}
	r, err := ReadRepro(filepath.Join("testdata", "repro", c.name+".repro.json"))
	if err != nil {
		t.Fatal(err)
	}
	script := &ScriptPolicy{Actions: r.Actions}
	res, reached := effectLongPlay(t, reg, c.effectLongCase, r, printedDelveSet(t, r.Cards), script)
	if d := res.Div; d != nil {
		t.Fatalf("divergence at %d: %s %s %v %s", d.At, d.What, d.Field, d.Diff, d.Detail)
	}
	if !reached || !got.cast {
		t.Fatal("the recorded game never casts the Hatchling from the graveyard")
	}
	if got.graveBefore < 6 {
		t.Fatalf("graveyard held %d cards at the cast, want the Hatchling and at least five others", got.graveBefore)
	}
	if len(got.exiled) != 6 {
		t.Fatalf("exiled after the cast began: %v, want a land and five graveyard cards", got.exiled)
	}
	if first := got.exiled[0]; first != "battlefield:Forest" && first != "battlefield:Mountain" && first != "battlefield:Swamp" {
		t.Errorf("first exile %q, want a land from the battlefield", first)
	}
	for _, x := range got.exiled[1:] {
		if len(x) < 10 || x[:10] != "graveyard:" || x == "graveyard:Lunar Hatchling(self)" || x[len(x)-6:] == "(self)" {
			t.Errorf("exile %q, want an OTHER graveyard card (never the cast Hatchling)", x)
		}
	}
}
