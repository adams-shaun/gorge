package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/state"
)

// The hosted-Commander zone-change, search and choice shapes the kernel now
// lowers (ticket cli-20261127T093652Z-ee7d5c3c; the dirac gap census
// census-1127b): each subtest plays one card's shape in real two-seat games
// against gorge, castFirstPolicy deciding, and fails on the first divergence.
// The tally is the precondition the compared decisions depend on: a seed set
// that never plays the card fails the subtest rather than passing vacuously.

type zoneChoiceCase struct {
	name   string
	deck   []cardset.Entry
	lower  []string
	seeds  uint64
	min    int
	tally  func(g *state.Game) int
	policy func(seed uint64) Policy // nil: castFirstPolicy
}

func TestHostedCmdrZoneChoiceLockstep(t *testing.T) {
	t.Parallel()
	cases := []zoneChoiceCase{
		{
			// SP$ ChangeZone Origin$ Hand -> Library, LibraryPosition$ -1, ChangeNum$ X
			// (Count$ValidHand Card.YouOwn), RememberChanged$, then Draw
			// Count$RememberedSize/Plus.1 (pro-shaper, vivi-ornitier-cedh)
			name: "valakut-awakening",
			deck: []cardset.Entry{
				{Name: "Valakut Awakening", Count: 8}, {Name: "Grizzly Bears", Count: 12},
				{Name: "Mountain", Count: 20},
			},
			lower: []string{"Valakut Awakening"},
			seeds: 24, min: 6,
			tally: func(g *state.Game) int { return cedhZoneCount(g, state.ZGraveyard, "Valakut Awakening") },
		},
		{
			// DB$ Sacrifice | SacValid$ Land | ChangeNum$ 1 on the enters
			// trigger (pro-shaper's Glacial Chasm)
			name: "glacial-chasm",
			deck: []cardset.Entry{
				{Name: "Glacial Chasm", Count: 8}, {Name: "Grizzly Bears", Count: 12},
				{Name: "Mountain", Count: 20},
			},
			lower: []string{"Glacial Chasm"},
			seeds: 24, min: 4,
			tally: func(g *state.Game) int {
				return cedhZoneCount(g, state.ZBattlefield, "Glacial Chasm") + cedhZoneCount(g, state.ZGraveyard, "Glacial Chasm")
			},
		},
		{
			// AB$ ChangeZone up to four basic lands from one graveyard to the
			// top of their library, then DB$ ChoosePlayer (Choices$ Player, no
			// Defined$) only when no card was targeted, then a DelayedTrigger
			// RememberObjects$ "TargetedOwner & ChosenPlayer" drawing at the next
			// upkeep (vivi-ornitier-cedh's Lodestone Bauble). Stitcher's Supplier
			// mills lands into the graveyards.
			name: "lodestone-bauble",
			deck: []cardset.Entry{
				{Name: "Lodestone Bauble", Count: 8}, {Name: "Stitcher's Supplier", Count: 8},
				{Name: "Swamp", Count: 12}, {Name: "Forest", Count: 12},
			},
			lower: []string{"Lodestone Bauble"},
			seeds: 24, min: 6,
			tally: func(g *state.Game) int { return cedhZoneCount(g, state.ZGraveyard, "Lodestone Bauble") },
		},
		{
			// the same Bauble with a graveyard eater (Scavenging Ooze exiles one
			// of the Bauble's targets in response): target 0 is illegal at
			// resolution while another target of the slot stands, so gorge's
			// TargetedOwner is the owner of the legal ones and the draw still
			// comes
			name: "lodestone-bauble-first-target-gone",
			deck: []cardset.Entry{
				{Name: "Lodestone Bauble", Count: 6}, {Name: "Stitcher's Supplier", Count: 12},
				{Name: "Scavenging Ooze", Count: 6}, {Name: "Swamp", Count: 8}, {Name: "Forest", Count: 8},
			},
			lower: []string{"Lodestone Bauble", "Scavenging Ooze"},
			seeds: 48, min: 4,
			tally:  baubleFirstTargetGoneTally(),
			policy: baubleOozePolicy,
		},
		{
			// the hand-move shape beside Valakut Awakening: Vendilion Clique's
			// enters trigger (DB$ ChangeZone Origin$ Hand -> Library,
			// LibraryPosition$ -1) newly lowers with the broadened hand move
			name: "vendilion-clique",
			deck: []cardset.Entry{
				{Name: "Vendilion Clique", Count: 8}, {Name: "Grizzly Bears", Count: 8},
				{Name: "Island", Count: 24},
			},
			lower: []string{"Vendilion Clique"},
			seeds: 24, min: 4,
			tally: func(g *state.Game) int {
				return cedhZoneCount(g, state.ZBattlefield, "Vendilion Clique") + cedhZoneCount(g, state.ZGraveyard, "Vendilion Clique")
			},
		},
		{
			// SP$ ChangeZone search a land, then DefinedPlayer$ Opponent's optional
			// search (RememberSearcher$), then a second search of
			// Count$RememberedSize lands (ulalek-eldrazi's Tempt with Discovery)
			name: "tempt-with-discovery",
			deck: []cardset.Entry{
				{Name: "Tempt with Discovery", Count: 8}, {Name: "Grizzly Bears", Count: 12},
				{Name: "Forest", Count: 20},
			},
			lower: []string{"Tempt with Discovery"},
			seeds: 24, min: 6,
			tally: func(g *state.Game) int { return cedhZoneCount(g, state.ZGraveyard, "Tempt with Discovery") },
		},
		{
			// the enters-cast trigger: Dig 4 to exile from a target opponent, then
			// DB$ ChangeZone Origin$ Exile ... ChangeType$ Land.TargetedPlayerOwn,
			// GainControl$ True, ChangeNum$ Count$ValidExile of the same filter
			// (ulalek-eldrazi's Oblivion Sower)
			name: "oblivion-sower",
			deck: []cardset.Entry{
				{Name: "Oblivion Sower", Count: 10}, {Name: "Forest", Count: 30},
			},
			lower: []string{"Oblivion Sower"},
			seeds: 24, min: 4,
			tally: func(g *state.Game) int { // lands the gain-control pick took from the opponent's exile
				n := 0
				for p := range g.Players {
					for _, id := range g.Zone(state.ZBattlefield, state.PlayerID(p)) {
						if o := g.Obj(id); o != nil && o.Controller != o.Owner {
							n++
						}
					}
				}
				return n
			},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			d := c.deck
			mk := c.policy
			if mk == nil {
				mk = castFirstPolicy
			}
			got := cedhPodRunWith(t, "zonechoice-"+c.name, [][]cardset.Entry{d, d}, c.lower, c.seeds, c.tally, mk)
			t.Logf("%s: evidence %d (want %d)", c.name, got, c.min)
			if got < c.min {
				t.Fatalf("too little evidence: %d (want %d)", got, c.min)
			}
		})
	}
}

// baubleFirstTargetGoneTally counts Lodestone Bauble activations seen on the
// stack whose first target has left the graveyard while a later target of the
// slot is still in one: the resolution that reads the owner of the first LIVE
// target. It is cumulative within one game and restarts on the next.
func baubleFirstTargetGoneTally() func(g *state.Game) int {
	var cur *state.Game
	seen := map[state.ObjID]bool{}
	n := 0
	return func(g *state.Game) int {
		if g != cur {
			cur, n = g, 0
			clear(seen)
		}
		for _, id := range g.Stack {
			o := g.Obj(id)
			if o == nil || seen[id] || len(o.Targets) < 2 {
				continue
			}
			if src := g.Obj(o.Source); src == nil || cardName(src.Card) != "Lodestone Bauble" {
				continue
			}
			inGy := func(t state.Target) bool {
				x := g.Obj(t.Obj)
				return !t.IsPlayer && x != nil && x.Zone == state.ZGraveyard
			}
			if inGy(o.Targets[0]) {
				continue
			}
			for _, t := range o.Targets[1:] {
				if inGy(t) {
					seen[id] = true
					n++
					break
				}
			}
		}
		return n
	}
}

// baubleOnStack is the topmost Lodestone Bauble activation on the stack that
// targets two or more cards, or nil. It is not only the stack's top: a
// proposed ability is on the stack while its own targets are chosen.
func baubleOnStack(g *state.Game) *state.Object {
	for i := len(g.Stack) - 1; i >= 0; i-- {
		o := g.Obj(g.Stack[i])
		if o == nil || len(o.Targets) < 2 {
			continue
		}
		if src := g.Obj(o.Source); src != nil && cardName(src.Card) == "Lodestone Bauble" {
			return o
		}
	}
	return nil
}

// baubleOozePolicy answers a Bauble activation on the stack with Scavenging
// Ooze's exile ability aimed at the Bauble's FIRST target, so the Bauble
// resolves with target 0 gone and the rest of its slot standing. Every other
// decision is castFirstPolicy's, except that a target ask is never closed
// early.
func baubleOozePolicy(seed uint64) Policy {
	base := castFirstPolicy(seed)
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.G == nil {
			return base.Pick(c)
		}
		g := c.G.E.G
		if b := baubleOnStack(g); b != nil {
			for _, a := range c.D.Options {
				switch a.Kind() {
				case ActActivate:
					if src := g.Obj(c.G.id.obj[a.Src()]); src != nil && cardName(src.Card) == "Scavenging Ooze" {
						return a
					}
				case ActTarget:
					if c.G.id.obj[a.Tgt()] == b.Targets[0].Obj {
						return a
					}
				}
			}
		}
		// never close a target ask early: the Bauble takes its four cards
		for _, a := range c.D.Options {
			if a.Kind() == ActTarget {
				return a
			}
		}
		// and never fire the Bauble at fewer than two basic lands in a graveyard
		if baubleHasPair(g) {
			return base.Pick(c)
		}
		d := *c.D
		d.Options = nil
		for _, a := range c.D.Options {
			if a.Kind() == ActActivate {
				if src := g.Obj(c.G.id.obj[a.Src()]); src != nil && cardName(src.Card) == "Lodestone Bauble" {
					continue
				}
			}
			d.Options = append(d.Options, a)
		}
		if len(d.Options) == 0 {
			return base.Pick(c)
		}
		return base.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
	})
}

// baubleHasPair reports whether some graveyard holds two basic land cards.
func baubleHasPair(g *state.Game) bool {
	for p := range g.Players {
		n := 0
		for _, id := range g.Zone(state.ZGraveyard, state.PlayerID(p)) {
			if o := g.Obj(id); o != nil && (cardName(o.Card) == "Swamp" || cardName(o.Card) == "Forest") {
				n++
			}
		}
		if n >= 2 {
			return true
		}
	}
	return false
}
