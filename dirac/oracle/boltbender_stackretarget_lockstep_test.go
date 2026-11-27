package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// boltbenderDeck: Disguise Boltbender, the {3} face-down cast priced from a
// Priest of Urabrask's floating {R}{R}{R}, a Shock to leave on the stack when
// Boltbender turns up, and the Moxen/lands that pay Shock and the {1}{R}
// turn-up from untapped sources.
var boltbenderDeck = []cardset.Entry{
	{Name: "Boltbender", Count: 4},
	{Name: "Shock", Count: 4},
	{Name: "Mox Ruby", Count: 4},
	{Name: "Mox Emerald", Count: 4},
	{Name: "Priest of Urabrask", Count: 4},
	{Name: "Mountain", Count: 20},
}

var boltbenderTurnUp = func() uint8 { c, _ := specialCode("turn_face_up"); return c }()

type boltbenderCounts struct {
	asks, picks, turnUps, downs, shocks int
}

// boltbenderStage is the policy's per-seat setup state.
type boltbenderStage struct {
	down  [2]bool // Boltbender is on the battlefield face down
	shock [2]bool // a Shock is waiting on the stack for the turn-up
}

// boltbenderRedSources counts the untapped red sources the seat can spend on
// Shock and the {1}{R} turn-up: the Mountains and Mox Rubies the policy plays.
func boltbenderRedSources(c *PickCtx, p int) int {
	if c.G == nil {
		return 0
	}
	g := c.G.E.G
	n := 0
	for i := range g.Objs {
		o := &g.Objs[i]
		if o.Zone != state.ZBattlefield || o.Tapped || o.Controller != state.PlayerID(p) || o.Face() == nil {
			continue
		}
		switch o.Face().Name {
		case "Mountain", "Mox Ruby":
			n++
		}
	}
	return n
}

// boltbenderStackShock reports whether the priority's stack is exactly the
// Shock spell: the one object Boltbender's trigger must see to redirect.
func boltbenderStackShock(c *PickCtx) bool {
	if c.G == nil || len(c.G.E.G.Stack) != 1 {
		return false
	}
	o := c.G.E.G.Obj(c.G.E.G.Stack[0])
	return o != nil && o.Face() != nil && o.Face().Name == "Shock"
}

// boltbenderPolicy steers one game into the redirect: build mana, cast
// Boltbender face down from the Priest's float, cast exactly one Shock, and
// turn Boltbender up while that Shock alone is on the stack. The per-entry
// redirect ask (ChKsrTarget) picks a new target rather than Done, so the mass
// walk's write is exercised; all other asks fall back to the seed's
// RandomPolicy.
func boltbenderPolicy(seed uint64, st *boltbenderCounts, seq *boltbenderStage) Policy {
	rnd := RandomPolicy(seed)
	return PolicyFunc(func(c *PickCtx) Canon {
		// The per-entry redirect ask: a new target (not Done) is what makes
		// the mass walk write, and the count proves the path ran.
		if c.G != nil && c.G.fr != nil && c.G.fr.d != nil && c.G.fr.d.Prompt == "Choose new target" {
			st.asks++
			for _, a := range c.D.Options {
				if a.Kind() >= ActChoose {
					st.picks++
					return a
				}
			}
		}
		// Shock's cast target and any other target ask.
		if c.D.Kind == DTarget {
			for _, a := range c.D.Options {
				if a.Kind() >= ActChoose {
					return a
				}
			}
		}
		// The turn-up trigger's optional "use it?" ask: take it, or the
		// redirect never runs.
		if c.D.Kind == DTriggerOptional {
			for _, a := range c.D.Options {
				if a.Kind() == ActYes {
					return a
				}
			}
		}
		// Any other choose: take an option rather than closing with Done.
		if c.D.Kind == DChoose {
			for _, a := range c.D.Options {
				if a.Kind() >= ActChoose {
					return a
				}
			}
		}
		p := int(c.D.Player)
		var (
			mox, priest, land, face, shock, up                         Canon
			haveMox, havePriest, haveLand, haveFace, haveShock, haveUp bool
		)
		for _, a := range c.D.Options {
			n := optionFaceName(c, a)
			switch {
			case a.Kind() == ActCast && isMoxName(n):
				mox, haveMox = a, true
			case a.Kind() == ActCast && isFloaterName(n):
				priest, havePriest = a, true
			case a.Kind() == ActCastFace && n == "Boltbender":
				face, haveFace = a, true
			case a.Kind() == ActCast && n == "Shock":
				shock, haveShock = a, true
			case a.Kind() == ActSpecial && a.Mode()&0x0f == boltbenderTurnUp:
				up, haveUp = a, true
			case a.Kind() == ActPlayLand:
				land, haveLand = a, true
			}
		}
		stackEmpty := c.G == nil || len(c.G.E.G.Stack) == 0
		// A Shock that left the stack without a turn-up (or was countered)
		// no longer waits for its redirect: the next face-down try casts a
		// fresh one.
		if stackEmpty {
			seq.shock[p] = false
		}
		switch {
		case haveUp && seq.shock[p] && boltbenderStackShock(c) && seq.down[p]:
			seq.down[p], seq.shock[p] = false, false
			st.turnUps++
			return up
		case !stackEmpty:
			// Wait for the stack to empty before the next setup cast; a
			// random in-response cast would leave two objects under the
			// turn-up trigger and hide the single-object redirect.
			for _, a := range c.D.Options {
				if a.Kind() == ActPass {
					return a
				}
			}
		case haveFace && !seq.down[p]:
			seq.down[p] = true
			st.downs++
			return face
		case haveShock && seq.down[p] && !seq.shock[p] && boltbenderRedSources(c, p) >= 3:
			seq.shock[p] = true
			st.shocks++
			return shock
		case haveMox:
			return mox
		case havePriest:
			return priest
		case haveLand:
			return land
		}
		var buf []Canon
		for _, a := range c.D.Options {
			if a.Kind() == ActSpecial && a.Mode()&0x0f == boltbenderTurnUp {
				continue // the turn-up is only taken with Shock on the stack
			}
			buf = append(buf, a)
		}
		if len(buf) == 0 {
			for _, a := range c.D.Options {
				if a.Kind() == ActPass {
					return a
				}
			}
			return c.D.Options[0]
		}
		d := *c.D
		d.Options = buf
		return rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
	})
}

// TestBoltbenderStackRetargetLockstep: 64 seeds of the disguised Boltbender
// turning up with exactly one Shock on the stack. The trigger's ChangeTargets
// over Defined$ ValidStack SpellAbility must ask the same per-entry
// ChKsrTarget decision in gorge and the kernel and write the same new target;
// the counts fail the test if the games never reach the redirect at all.
func TestBoltbenderStackRetargetLockstep(t *testing.T) {
	t.Parallel()

	reg := synthReg(t)
	var decks [2][]*cards.Card
	for s := range decks {
		for _, e := range boltbenderDeck {
			c, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s not in the corpus", e.Name)
			}
			for i := 0; i < e.Count; i++ {
				decks[s] = append(decks[s], c)
			}
		}
	}
	var deckNames [2][]string
	for s := range deckNames {
		for _, e := range boltbenderDeck {
			for i := 0; i < e.Count; i++ {
				deckNames[s] = append(deckNames[s], e.Name)
			}
		}
	}
	set := printedDelveSet(t, deckNames)
	tbl, _, _, err := set.Matchup(boltbenderDeck, boltbenderDeck)
	if err != nil {
		t.Fatal(err)
	}
	var total boltbenderCounts
	var tuEvents int
	for seed := uint64(1); seed <= 64; seed++ {
		var st boltbenderCounts
		var stage boltbenderStage
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"boltbender-a", "boltbender-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		seen := 0
		check := func(ref *GorgeSide, _ Side) *Divergence {
			evs := ref.E.L.Events
			for ; seen < len(evs); seen++ {
				if evs[seen].Kind == events.TurnFaceUp {
					tuEvents++
				}
			}
			return nil
		}
		res := Lockstep(m, seed, k, boltbenderPolicy(seed, &st, &stage), RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40, Check: check})
		CloseSide(k)
		if d := res.Div; d != nil {
			lo := d.At - 12
			if lo < 0 {
				lo = 0
			}
			t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:d.At]))
			t.Fatalf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
		}
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			t.Fatalf("seed %d: stalled: %s", seed, res.Stall)
		}
		total.asks += st.asks
		total.picks += st.picks
		total.turnUps += st.turnUps
		total.downs += st.downs
		total.shocks += st.shocks
	}
	t.Logf("%d games, %d downs, %d shocks, %d redirect asks, %d new targets, %d turn-ups; %d turn-face-up events", 64, total.downs, total.shocks, total.asks, total.picks, total.turnUps, tuEvents)
	if total.asks == 0 {
		t.Fatalf("no game asked Boltbender's stack redirect: the lockstep does not see the mass walk")
	}
	if total.picks == 0 {
		t.Fatalf("no redirect ask picked a new target: the lockstep never wrote the redirect")
	}
}
