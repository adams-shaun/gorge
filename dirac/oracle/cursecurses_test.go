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

// The Commander curses' lockstep (CR 508.1, 303.4, 701.3b): each curse
// enchants a player and fires on an AttackersDeclared group whose defender
// carries it (AttackedTarget$ Player.EnchantedBy — the kernel's Tf4DefEnch
// gate: the LISTENING Aura is itself attached to the attacked seat, as
// Forge's player.isEnchantedBy(source) reads it); its controller pays
// (Opulence's Gold and Disturbance's and Shallow Graves's tapped Zombies are
// minted, Verbosity draws, Vitality gains life, Bounty untaps, Chaos's
// attacking player discards to draw, Inertia asks the attacking player for
// the target — TargetingPlayer$ TriggeredAttackingPlayer, TgfTrigChooser —
// and the curse's controller for the tap-or-untap election, Tapper$
// provenance riding the OpTap). The second leg every non-Inertia curse
// chains (TriggeredAttackingPlayer.Opponent+controlsCreature.attacking
// Player.EnchantedBy) names the declaring player when that player is an
// opponent of the curse's controller ("each opponent attacking that player
// does the same"); kgen lowers it to a trigger-player effect behind a
// controller-turn guard. Archnemesis enchants an opponent; its per-defender
// trigger makes the attacking player lose 2 and draw 2 while its controller
// gains 2, and its optional trigger (OptionalDecider$ TriggeredAttackingPlayer)
// attaches it to the attacking player (the Enchant:Opponent attach's W arm).

// The second leg is read when it RESOLVES (gorge's Defined$ at resolution), so
// both decks play Lightning Bolt and kccBoltPolicy casts one at an attacker
// while a curse trigger waits in the declare-attackers step: the attacker
// leaves combat before the leg resolves and neither engine pays the declaring
// player ("bolted attacker" in the branch counts).
//
// kccDeckA plays Opulence, Disturbance, Verbosity, Inertia and Archnemesis;
// kccDeckB plays Bounty, Vitality, Chaos and Shallow Graves. Each side
// enchants the other and both attack, so every carrier is cast, fired and
// resolved by both engines.
var kccDeckA = []cardset.Entry{
	{Name: "Curse of Opulence", Count: 2}, {Name: "Curse of Disturbance", Count: 2},
	{Name: "Curse of Verbosity", Count: 2}, {Name: "Curse of Inertia", Count: 2},
	{Name: "Archnemesis", Count: 2},
	{Name: "Lightning Bolt", Count: 4},
	{Name: "Grizzly Bears", Count: 8}, {Name: "Llanowar Elves", Count: 6},
	{Name: "Mountain", Count: 3}, {Name: "Island", Count: 3}, {Name: "Forest", Count: 2},
	{Name: "Swamp", Count: 2}, {Name: "Plains", Count: 2},
}

var kccDeckB = []cardset.Entry{
	{Name: "Curse of Bounty", Count: 2}, {Name: "Curse of Vitality", Count: 2},
	{Name: "Curse of Chaos", Count: 2}, {Name: "Curse of Shallow Graves", Count: 2},
	{Name: "Lightning Bolt", Count: 4},
	{Name: "Grizzly Bears", Count: 8}, {Name: "Llanowar Elves", Count: 6},
	{Name: "Forest", Count: 3}, {Name: "Plains", Count: 3}, {Name: "Swamp", Count: 3},
	{Name: "Island", Count: 2}, {Name: "Mountain", Count: 3},
}

// kccScan counts the carriers' casts in gorge's event log, the curse-minted
// tokens and the player attaches (an Attach whose Player names the seat —
// the Enchant:Player/Opponent attach event; an Archnemesis that moved to the
// attacking player counts once per move).
func kccScan(g *GorgeSide, n map[string]int) {
	if g == nil {
		return
	}
	for _, ev := range g.E.L.Events {
		switch ev.Kind {
		case events.PutOnStack:
			if ob := g.E.G.Obj(ev.Obj); ob != nil && ob.Face() != nil && ev.From == state.ZHand {
				switch nm := ob.Face().Name; nm {
				case "Curse of Opulence", "Curse of Disturbance", "Curse of Verbosity",
					"Curse of Bounty", "Curse of Vitality", "Curse of Inertia",
					"Curse of Chaos", "Curse of Shallow Graves", "Archnemesis":
					n[nm]++
				}
			}
		case events.TokenCreate:
			n["tokens"]++
		case events.Attach:
			if ev.Text == "attach to player" {
				n["player attaches"]++
			}
		}
	}
}

// kccBoltPolicy answers a priority decision in the declare-attackers step with
// a non-empty stack (a curse trigger waiting) by casting a Lightning Bolt when
// one is offered; the target decision that follows stays with the inner
// policy, which reaches an attacker in some games.
func kccBoltPolicy(c *PickCtx) (Canon, bool) {
	var none Canon
	if c.G == nil || c.D.Kind != DPriority || c.G.E.G.Step != state.StepDeclareAttackers || len(c.G.E.G.Stack) == 0 {
		return none, false
	}
	for _, o := range c.D.Options {
		if o.Kind() != ActCast {
			continue
		}
		if g, ok := c.G.GorgeOption(o); ok {
			if ob := c.G.E.G.Obj(g.Obj); ob != nil && ob.Face() != nil && ob.Face().Name == "Lightning Bolt" {
				return o, true
			}
		}
	}
	return none, false
}

func runCursecurses(t *testing.T, seeds uint64) (games, divs int, branch map[string]int) {
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("cursecurses")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	ents := [2][]cardset.Entry{kccDeckA, kccDeckB}
	for s := range decks {
		for _, e := range ents[s] {
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
	tbl, _, _, err := set.Matchup(ents[0], ents[1])
	if err != nil {
		t.Fatal(err)
	}
	played, branch := map[string]int{}, map[string]int{}
	for seed := uint64(1); seed <= seeds; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"kcc-a", "kcc-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		var gs *GorgeSide
		inner := eagerPolicy(seed, played)
		pol := PolicyFunc(func(c *PickCtx) Canon {
			if c.G != nil {
				gs = c.G
			}
			if a, ok := kccBoltPolicy(c); ok {
				branch["bolt in response"]++
				return a
			}
			return inner.Pick(c)
		})
		res := Lockstep(m, seed, k, pol, RunConfig{Config: cfg, MaxDecisions: 2500, MaxTurns: 25})
		CloseSide(k)
		games++
		kccScan(gs, branch)
		if d := res.Div; d != nil {
			divs++
			lo := d.At - 12
			if lo < 0 {
				lo = 0
			}
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nlast sourced %q implicate %v\nlast actions %v\nref %v\nker %v",
				seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.LastSourced, d.Implicates(m),
				actionStrings(res.Actions[lo:d.At]), d.RefOptions, d.KerOptions)
			return
		}
	}
	return
}

func TestCursecursesLockstep(t *testing.T) {
	t.Parallel()

	games, divs, branch := runCursecurses(t, 200)
	var keys []string
	for k := range branch {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var bs []string
	for _, k := range keys {
		bs = append(bs, k+"="+fmt.Sprint(branch[k]))
	}
	t.Logf("%d games, %d divergences; %s", games, divs, strings.Join(bs, " "))
	for _, nm := range []string{"Curse of Opulence", "Curse of Disturbance", "Curse of Verbosity",
		"Curse of Bounty", "Curse of Vitality", "Curse of Inertia", "Curse of Chaos",
		"Curse of Shallow Graves", "Archnemesis"} {
		if branch[nm] < 2 {
			t.Errorf("%s %d, want at least 2 casts", nm, branch[nm])
		}
	}
	if branch["tokens"] < 2 {
		t.Errorf("tokens %d, want at least 2", branch["tokens"])
	}
	if branch["player attaches"] < 1 {
		t.Errorf("player attaches %d, want at least 1", branch["player attaches"])
	}
}
