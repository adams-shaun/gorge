package oracle

import (
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The lockstep: constructed matchups over the generated
// remparams pool, whose carriers read the remember/scope params
// (ChangeZoneAll RememberChanged$, ForgetOtherRemembered$, RememberLKI$ with
// the RememberedLKI / RememberedController readers, RememberControlled$,
// RememberDamaged$, PumpAll ValidTgts$ / Duration$, Discard UnlessType$).
// Each branch is counted from gorge's event log (a resolution's segment:
// its Resolve event up to the next Priority or Resolve) or, for the
// discard_unless election, from gorge's pending decision; never from the
// carrier's cast.
//
// Coveted Falcon (build-refused: a 0..Count target slot) and Grub's / Sygg's
// Command (build-refused: four target slots) are not in the pool, so M0 and
// M2 replace their copies with the matchup's other carriers and fillers.
// For the Common Good now lowers (PumpAll Duration$
// UntilYourNextTurn is a keyword grant until the controller's next turn), so
// its next-turn lockstep is dirac/oracle/pumpnt_test.go.
// Clive, Ifrit's Dominant has no matchup: its activation is Jill's
// MvForgetOther return (lowering-tested only).
var kremDecks = []struct {
	name  string
	seeds uint64
	decks [2][]cardset.Entry
}{{
	// M0 lki-donate: A gives B a permanent (Stiltzkin), B bounces it
	// (Boomerang Basics, Hotshot Investigators): the RememberLKI$ capture
	name: "lki-donate", seeds: 256,
	decks: [2][]cardset.Entry{{
		{Name: "Stiltzkin, Moogle Merchant", Count: 4}, {Name: "Nurturing Pixie", Count: 4}, {Name: "Unyielding Gatekeeper", Count: 3},
		{Name: "Savannah Lions", Count: 5}, {Name: "Ornithopter", Count: 4}, {Name: "Arid Archway", Count: 4},
		{Name: "Plains", Count: 12}, {Name: "Island", Count: 4},
	}, {
		{Name: "Boomerang Basics", Count: 4}, {Name: "Hotshot Investigators", Count: 2}, {Name: "Splash Portal", Count: 4},
		{Name: "Jill, Shiva's Dominant", Count: 2}, {Name: "Healer's Hawk", Count: 4}, {Name: "Wind Drake", Count: 4},
		{Name: "Island", Count: 12}, {Name: "Plains", Count: 8},
	}},
}, {
	name: "sweeps", seeds: 256,
	decks: [2][]cardset.Entry{{
		{Name: "Settle the Wreckage", Count: 4}, {Name: "Hex Magic", Count: 4}, {Name: "Elixir", Count: 3},
		{Name: "Savannah Lions", Count: 4}, {Name: "Raging Goblin", Count: 5}, {Name: "Plains", Count: 10}, {Name: "Mountain", Count: 10},
	}, {
		{Name: "Overwrite the Multiverse", Count: 4}, {Name: "Grizzly Bears", Count: 8}, {Name: "Llanowar Elves", Count: 4},
		{Name: "Elvish Mystic", Count: 4}, {Name: "Swamp", Count: 10}, {Name: "Forest", Count: 10},
	}},
}, {
	name: "pump-sweeps", seeds: 256,
	decks: [2][]cardset.Entry{{
		{Name: "Neutralize the Guards", Count: 4}, {Name: "Wail of War", Count: 4}, {Name: "How to Start a Riot", Count: 4},
		{Name: "Raging Goblin", Count: 6}, {Name: "Shivan Dragon", Count: 2}, {Name: "Swamp", Count: 10}, {Name: "Mountain", Count: 10},
	}, {
		{Name: "Merfolk Looter", Count: 4}, {Name: "The Black Arrow", Count: 4}, {Name: "Savannah Lions", Count: 5},
		{Name: "Healer's Hawk", Count: 7}, {Name: "Plains", Count: 10}, {Name: "Island", Count: 10},
	}},
}, {
	name: "discard-unless", seeds: 256,
	decks: [2][]cardset.Entry{kremM3, kremM3},
}, {
	// gorge prices a face-down cast and a turn-face-up from the floating
	// pool alone (facedown_test.go), which M0 never holds: Priest of Gix's
	// {B}{B}{B} pays Unyielding Gatekeeper's face-down {3}, Iridescent
	// Tiger's {W}{U}{B}{R}{G} its {1}{W} turn-up (or both). The Gatekeeper's
	// trigger then exiles its controller's own permanent (returned) or a
	// Grizzly Bears (its controller's Detective: TokenOwner$
	// RememberedController)
	name: "gatekeeper", seeds: 512,
	decks: [2][]cardset.Entry{{
		{Name: "Unyielding Gatekeeper", Count: 4}, {Name: "Priest of Gix", Count: 5}, {Name: "Iridescent Tiger", Count: 5},
		{Name: "Savannah Lions", Count: 2}, {Name: "Plains", Count: 8}, {Name: "Swamp", Count: 6}, {Name: "Mountain", Count: 10},
	}, {
		{Name: "Grizzly Bears", Count: 12}, {Name: "Forest", Count: 28},
	}},
}}

var kremM3 = []cardset.Entry{
	{Name: "Thirst for Knowledge", Count: 3}, {Name: "Thirst for Identity", Count: 3}, {Name: "Steamcore Scholar", Count: 3},
	{Name: "Alpharael, Dreaming Acolyte", Count: 3}, {Name: "Ornithopter", Count: 3}, {Name: "Memnite", Count: 3},
	{Name: "Wind Drake", Count: 2}, {Name: "Island", Count: 12}, {Name: "Swamp", Count: 8},
}

// kremCarriers must each resolve at least kremMinPlays times (a spell, an
// activation or a trigger of the card).
var kremCarriers = []string{
	"Stiltzkin, Moogle Merchant", "Nurturing Pixie", "Unyielding Gatekeeper", "Arid Archway",
	"Boomerang Basics", "Hotshot Investigators", "Splash Portal", "Jill, Shiva's Dominant",
	"Settle the Wreckage", "Hex Magic", "Elixir", "Overwrite the Multiverse",
	"Neutralize the Guards", "Wail of War", "How to Start a Riot", "The Black Arrow",
	"Thirst for Knowledge", "Thirst for Identity", "Steamcore Scholar", "Alpharael, Dreaming Acolyte",
}

const (
	kremMinPlays  = 20
	kremMinBranch = 10
)

// kremBranches names every counted branch (each must reach kremMinBranch).
var kremBranches = []string{
	"boomerang: controlled, draws", "boomerang: not controlled, no draw", "donated bounce: bouncer draws/investigates",
	"hotshot: no target", "hotshot: one target", "pixie: counter", "pixie: no counter",
	"archway: surveil", "archway: no surveil", "gatekeeper: return", "gatekeeper: detective",
	"splash portal: draw", "splash portal: no draw", "stiltzkin: draw", "jill: transformed",
	"hex magic: draws = exiled > 0", "overwrite: X >= 1", "settle: attacker exiled", "elixir: resolved",
	"black arrow: dragon destroyed", "black arrow: non-dragon",
	"pumpall: opponent scope", "pumpall: caster scope (riot)",
	"discard: none eligible", "discard: unless, 1 eligible", "discard: unless, >= 2 eligible", "discard: ordinary, eligible present",
}

// kremScan adds one game's branch counts (and its carrier resolutions) from
// gorge's event log.
func kremScan(g *GorgeSide, n map[string]int, plays map[string]int) {
	if g == nil {
		return
	}
	gm := g.E.G
	evs := g.E.L.Events
	ptgt := map[state.ObjID]state.PlayerID{} // a stack object's player target
	for i := 0; i < len(evs); i++ {
		ev := evs[i]
		switch ev.Kind {
		case events.TargetsChosen:
			if ev.Amount == 1 || ev.Amount == 3 && len(ev.IDs) == 0 { // a player target, or a sub-ability's player sub-target
				ptgt[ev.Obj] = ev.Player
			}
		}
		if ev.Kind != events.Resolve {
			continue
		}
		so := gm.Obj(ev.Obj)
		if so == nil {
			continue
		}
		name, ctrl := sourceCardName(gm, ev.Obj), so.Controller
		// the segment: up to the next Priority or Resolve
		var seg []events.Event
		for j := i + 1; j < len(evs) && evs[j].Kind != events.Priority && evs[j].Kind != events.Resolve; j++ {
			seg = append(seg, evs[j])
		}
		draws, invest, exiled, surveil, counter, ret, tok, toGY := 0, 0, 0, 0, 0, 0, 0, 0
		var bounced []state.ObjID
		dragonDied, nonDragonHit := false, false
		for _, e := range seg {
			switch e.Kind {
			case events.Draw:
				if e.Player == ctrl {
					draws++
				}
			case events.Investigate:
				if e.Player == ctrl {
					invest++
				}
			case events.Surveil:
				surveil++
			case events.CounterChange:
				if so.Source != 0 && e.Obj == so.Source && e.Amount > 0 {
					counter++
				}
			case events.TokenCreate:
				if e.Text == "wu_2_2_detective" {
					tok++
				}
			case events.Damage:
				if e.Obj != 0 && !strings.Contains(firstFaceName(gm, e.Obj), "Dragon") {
					nonDragonHit = true
				}
			case events.MoveZone:
				switch {
				case e.From == state.ZBattlefield && e.To == state.ZHand:
					bounced = append(bounced, e.Obj)
				case e.From == state.ZBattlefield && e.To == state.ZExile:
					exiled++
				case e.From == state.ZHand && e.To == state.ZExile:
					exiled++
				case e.From == state.ZExile && e.To == state.ZBattlefield:
					ret++
				case e.From == state.ZBattlefield && e.To == state.ZGraveyard:
					toGY++
					if firstFaceName(gm, e.Obj) == "Shivan Dragon" {
						dragonDied = true
					}
				}
			}
		}
		plays[name]++
		donated := func() bool {
			for _, b := range bounced {
				if o := gm.Obj(b); o != nil && o.Owner != ctrl {
					return true
				}
			}
			return false
		}
		switch name {
		case "Boomerang Basics":
			if len(bounced) == 0 {
				break
			}
			switch {
			case draws > 0 && donated():
				n["donated bounce: bouncer draws/investigates"]++
			case draws > 0:
				n["boomerang: controlled, draws"]++
			default:
				n["boomerang: not controlled, no draw"]++
			}
		case "Hotshot Investigators":
			if so.Ability == nil {
				break // the creature spell
			}
			if len(bounced) == 0 {
				n["hotshot: no target"]++
				break
			}
			n["hotshot: one target"]++
			if invest > 0 && donated() {
				n["donated bounce: bouncer draws/investigates"]++
			}
		case "Nurturing Pixie":
			if so.Ability == nil {
				break
			}
			if counter > 0 {
				n["pixie: counter"]++
			} else {
				n["pixie: no counter"]++
			}
		case "Arid Archway":
			if so.Ability == nil {
				break
			}
			if surveil > 0 {
				n["archway: surveil"]++
			} else {
				n["archway: no surveil"]++
			}
		case "Unyielding Gatekeeper":
			if so.Ability == nil {
				break
			}
			if ret > 0 {
				n["gatekeeper: return"]++
			}
			if tok > 0 {
				n["gatekeeper: detective"]++
			}
		case "Splash Portal":
			if exiled == 0 {
				break
			}
			if draws > 0 {
				n["splash portal: draw"]++
			} else {
				n["splash portal: no draw"]++
			}
		case "Stiltzkin, Moogle Merchant":
			if so.Ability != nil && draws > 0 {
				n["stiltzkin: draw"]++
			}
		case "Jill, Shiva's Dominant":
			if so.Ability != nil && exiled > 0 && ret > 0 {
				n["jill: transformed"]++
			}
		case "Hex Magic":
			if exiled > 0 && draws == exiled {
				n["hex magic: draws = exiled > 0"]++
			}
		case "Overwrite the Multiverse":
			if exiled > 0 {
				n["overwrite: X >= 1"]++
			}
		case "Settle the Wreckage":
			if exiled > 0 {
				n["settle: attacker exiled"]++
			}
		case "Elixir":
			if so.Ability != nil {
				n["elixir: resolved"]++
			}
		case "The Black Arrow":
			if so.Ability == nil {
				break
			}
			if dragonDied {
				n["black arrow: dragon destroyed"]++
			} else if nonDragonHit {
				n["black arrow: non-dragon"]++
			}
		case "Neutralize the Guards", "Wail of War", "How to Start a Riot":
			p, ok := ptgt[ev.Obj]
			if !ok {
				break
			}
			if p == ctrl {
				n["pumpall: caster scope (riot)"]++
			} else {
				n["pumpall: opponent scope"]++
			}
		}
	}
}

// kremPolicy is eagerPolicy that remembers the gorge side and counts the
// discard_unless election's branches from gorge's pending decision: the
// election's answer, and for "unless" whether a pick over two or more
// eligible cards follows.
func kremPolicy(seed uint64, played, n map[string]int, gs **GorgeSide) Policy {
	inner := eagerPolicy(seed, played)
	pendingUnless := false
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.G != nil {
			*gs = c.G
		}
		d := c.G.pendingSafe()
		if pendingUnless {
			pendingUnless = false
			if d != nil && d.ResumeKind == "discard" && strings.HasSuffix(d.Prompt, " card instead") {
				n["discard: unless, >= 2 eligible"]++
			} else {
				n["discard: unless, 1 eligible"]++
			}
		}
		a := inner.Pick(kremSteer(c))
		if d != nil && d.ResumeKind == "discard_unless" && c.G != nil {
			if o, ok := c.G.GorgeOption(a); ok {
				switch o.Kind {
				case "unless":
					pendingUnless = true
				case "ordinary":
					n["discard: ordinary, eligible present"]++
				}
			}
		}
		if d != nil && d.ResumeKind == "discard" && !strings.HasSuffix(d.Prompt, " card instead") && c.G != nil {
			// an ordinary pick with no election before it: the resolving
			// carrier's hand had no UnlessType$ card
			if top := c.G.E.G.Stack; len(top) > 0 {
				switch sourceCardName(c.G.E.G, top[len(top)-1]) {
				case "Thirst for Knowledge", "Thirst for Identity", "Steamcore Scholar", "Alpharael, Dreaming Acolyte":
					if !c.G.unlessAsked(top[len(top)-1]) {
						n["discard: none eligible"]++
					}
				}
			}
		}
		return a
	})
}

// kremSteer narrows a priority decision toward the carriers' branches,
// never past legality: a face-down Unyielding Gatekeeper is turned face up
// whenever that special action is offered (its trigger is the RememberLKI$
// carrier), and Settle the Wreckage is held while no creature of an
// opponent of the decider is attacking (it exiles attackers only).
func kremSteer(c *PickCtx) *PickCtx {
	if c.G == nil || c.D.Kind != DPriority {
		return c
	}
	g := c.G.E.G
	up, _ := specialCode("turn_face_up")
	attackers := false
	for _, id := range g.Zone(state.ZBattlefield, state.PlayerID(1-c.D.Player)) {
		if o := g.Obj(id); o != nil && o.IsAttacking {
			attackers = true
		}
	}
	var keep []Canon
	for _, a := range c.D.Options {
		if a.Kind() == ActSpecial && a.Mode()&0xf == up {
			d := *c.D
			d.Options = []Canon{a}
			return &PickCtx{D: &d, G: c.G, Seq: c.Seq}
		}
		if a.Kind() == ActCast && !attackers {
			if o, ok := c.G.GorgeOption(a); ok && firstFaceName(g, o.Obj) == "Settle the Wreckage" {
				continue
			}
		}
		keep = append(keep, a)
	}
	if len(keep) == len(c.D.Options) {
		return c
	}
	d := *c.D
	d.Options = keep
	return &PickCtx{D: &d, G: c.G, Seq: c.Seq}
}

func (g *GorgeSide) pendingSafe() *pendingDecision {
	if g == nil {
		return nil
	}
	d := g.Pending()
	if d == nil {
		return nil
	}
	return &pendingDecision{ResumeKind: d.ResumeKind, Prompt: d.Prompt}
}

type pendingDecision struct{ ResumeKind, Prompt string }

// unlessAsked reports whether stack object id's resolution has posed the
// discard_unless election (a DecisionAsk "choose" since its Resolve).
func (g *GorgeSide) unlessAsked(id state.ObjID) bool {
	evs := g.E.L.Events
	for i := len(evs) - 1; i >= 0; i-- {
		switch evs[i].Kind {
		case events.Resolve:
			return false
		case events.DecisionAsk:
			if evs[i].Text == "choose" {
				return true
			}
		}
	}
	return false
}

func TestRemparamsLockstep(t *testing.T) {
	t.Parallel()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("remparams")
	if err != nil {
		t.Fatal(err)
	}
	games := 0
	played, n, plays := map[string]int{}, map[string]int{}, map[string]int{}
	stalls := map[string]int{}
	for mi, mu := range kremDecks {
		decks := kcastDeck(t, reg, mu.decks)
		tbl, _, _, err := set.Matchup(mu.decks[0], mu.decks[1])
		if err != nil {
			t.Fatal(err)
		}
		for seed := uint64(1); seed <= mu.seeds; seed++ {
			cfg := Config{GorgeAutopaySurface: seed%2 == 0}
			m, err := NewMatchup(reg, [2]string{"krem-a", "krem-b"}, decks, 1)
			if err != nil {
				t.Fatal(err)
			}
			k, err := SetSide(set, tbl, cfg)(m, seed)
			if err != nil {
				t.Fatal(err)
			}
			var gs *GorgeSide
			res := Lockstep(m, seed, k, kremPolicy(seed, played, n, &gs), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20})
			CloseSide(k)
			games++
			if strings.HasPrefix(res.Stall, KernelStallPrefix) {
				stalls[res.Stall]++
			}
			kremScan(gs, n, plays)
			if d := res.Div; d != nil {
				lo := max(d.At-12, 0)
				t.Logf("matchup %d (%s) seed %d last actions %v", mi, mu.name, seed, actionStrings(res.Actions[lo:d.At]))
				kcastDump(t, gs)
				t.Errorf("matchup %d (%s) seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", mi, mu.name, seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
				return
			}
		}
	}
	var keys []string
	for k := range plays {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var ps []string
	for _, k := range keys {
		ps = append(ps, fmt.Sprintf("%s:%d", k, plays[k]))
	}
	t.Logf("%d games, kernel stalls %v", games, stalls)
	t.Logf("resolutions: %s", strings.Join(ps, " "))
	for _, b := range kremBranches {
		t.Logf("branch %-45s %d", b, n[b])
		if n[b] < kremMinBranch {
			t.Errorf("branch %q: %d games, want at least %d", b, n[b], kremMinBranch)
		}
	}
	for _, c := range kremCarriers {
		if plays[c] < kremMinPlays {
			t.Errorf("carrier %s resolved %d times, want at least %d", c, plays[c], kremMinPlays)
		}
	}
}
