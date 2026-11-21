package oracle

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// CR 603.3c, 608.2h: Mage Slayer's carrier ("Whenever equipped creature
// attacks, it deals damage equal to its power to the player or planeswalker
// it's attacking") attacks a planeswalker and its trigger resolves with the
// carrier alive, and again after Devouring Light or Cloudshift removed it in
// response. gorge damages the DefenderObj the Defined$'s Valid
// Planeswalker.TriggeredDefender arm binds as the trigger is queued; the
// kernel's DealDamageFrom defending mode must neither stall
// (StallDefenderPW, the plain guard the lowering used to add) nor deal to
// the defending player there.
//
// mageslayer_defenderpw is a recorded game of the decks below; the test
// replays it and requires the mechanism: a Mage Slayer trigger on an
// attacked planeswalker resolved with its carrier alive, and one resolved
// after the carrier left the battlefield. It was recorded by playing seeds
// of these decks with a policy that attacks the planeswalker and answers the
// carrier's trigger with Devouring Light or Cloudshift on the attacker; the
// seed the record test found reached both arms.

const mspwRepro = "mageslayer_defenderpw"

var mspwDeck = deckOf(6, "Mage Slayer",
	deckOf(4, "Devouring Light",
		deckOf(4, "Cloudshift",
			deckOf(3, "Ajani Goldmane",
				deckOf(6, "Raging Goblin",
					deckOf(10, "Plains",
						deckOf(10, "Mountain",
							deckOf(10, "Forest")...)...)...)...)...)...)...)

// mspwTally follows gorge's event log: a Mage Slayer trigger pushed while its
// attacker attacks a planeswalker pushes a frame carrying the attacker; the
// attacker leaving the battlefield marks the frame gone; a Resolve pops it.
type mspwTally struct {
	last     int
	frames   []mspwFrame
	triggers int // Mage Slayer triggers on an attacked planeswalker/battle
	alive    int // of those, resolved with the attacker still on the battlefield
	gone     int // of those, resolved after the attacker left
}

type mspwFrame struct {
	atk  state.ObjID
	obj  bool // the carrier attacked a planeswalker or battle when the trigger pushed
	gone bool
}

func (a *mspwTally) check(g *GorgeSide, _ Side) *Divergence {
	if g == nil || g.E == nil || g.E.G == nil {
		return nil
	}
	evs := g.E.L.Events
	for ; a.last < len(evs); a.last++ {
		e := &evs[a.last]
		switch e.Kind {
		case events.MoveZone:
			if e.From != state.ZBattlefield {
				break
			}
			for i := range a.frames {
				if a.frames[i].atk == e.Obj {
					a.frames[i].gone = true
				}
			}
		case events.TriggerPush:
			if mspwCarrier(g.E.G, e.Obj) != "Mage Slayer" {
				a.frames = append(a.frames, mspwFrame{})
				break
			}
			f := mspwFrame{}
			if len(e.IDs) > 0 {
				f.atk = e.IDs[0]
				if o := g.E.G.Obj(e.IDs[0]); o != nil && o.IsAttacking && o.AttackingBattle != 0 {
					f.obj = true
					a.triggers++
				}
			}
			a.frames = append(a.frames, f)
		case events.PutOnStack:
			a.frames = append(a.frames, mspwFrame{})
		case events.Resolve:
			if len(a.frames) == 0 {
				break
			}
			f := a.frames[len(a.frames)-1]
			a.frames = a.frames[:len(a.frames)-1]
			if f.obj {
				if f.gone {
					a.gone++
				} else {
					a.alive++
				}
			}
		}
	}
	return nil
}

// mspwCarrier names obj when its face is Mage Slayer, else "".
func mspwCarrier(g *state.Game, obj state.ObjID) string {
	o := g.Obj(obj)
	if o == nil || o.Face() == nil {
		return ""
	}
	if o.Face().Name == "Mage Slayer" {
		return "Mage Slayer"
	}
	return ""
}

// mspwTriggerOnStack reports whether a Mage Slayer triggered ability is on
// the stack, so the policy saves its response spells for the window the test
// needs.
func mspwTriggerOnStack(g *state.Game) bool {
	if g == nil {
		return false
	}
	for _, id := range g.Stack {
		o := g.Obj(id)
		if o == nil || state.StackKindOf(g, o) != state.StackKindTriggered {
			continue
		}
		if src := g.Obj(o.Source); src != nil && src.Face() != nil {
			if src.Face().Name == "Mage Slayer" {
				return true
			}
		}
	}
	return false
}

// mspwOptCard is the card an ActCast / ActCastFace option casts.
func mspwOptCard(c *PickCtx, a Canon) *state.Object {
	o, ok := c.G.GorgeOption(a)
	if !ok {
		return nil
	}
	return c.G.E.G.Obj(o.Obj)
}

// mspwInstantName is a held response spell's name, else "".
func mspwInstantName(c *PickCtx, a Canon) string {
	ob := mspwOptCard(c, a)
	if ob == nil || ob.Face() == nil {
		return ""
	}
	for _, t := range ob.Face().Types {
		if t == "Instant" {
			switch ob.Face().Name {
			case "Devouring Light", "Cloudshift":
				return ob.Face().Name
			}
			return ""
		}
	}
	return ""
}

// mspwPolicy answers yes, attacks the opponent's planeswalker whenever the
// attack decision offers it, casts Devouring Light or Cloudshift on an
// attacking creature in response to a Mage Slayer trigger (the gone arm),
// and otherwise prefers casting Ajani Goldmane, casting Mage Slayer and
// equipping it, playing lands and other casts at random.
func mspwPolicy(seed uint64) Policy {
	rnd := RandomPolicy(seed)
	return PolicyFunc(func(c *PickCtx) Canon {
		for _, o := range c.D.Options {
			if o.Kind() == ActYes {
				return o
			}
		}
		inResponse := c.G != nil && mspwTriggerOnStack(c.G.E.G)
		if inResponse {
			for _, o := range c.D.Options {
				if (o.Kind() == ActCast || o.Kind() == ActCastFace) && mspwInstantName(c, o) != "" {
					return o
				}
			}
		}
		if c.D.Kind == DTarget {
			for _, o := range c.D.Options {
				if o.Kind() != ActTarget || int(o.Tgt()) >= int(RefPlayer) {
					continue
				}
				if ob := c.G.E.G.Obj(state.ObjID(o.Tgt())); ob != nil && ob.IsAttacking {
					return o
				}
			}
		}
		// attack the planeswalker whenever the declaration offers it
		for _, o := range c.D.Options {
			if o.Kind() != ActAttack {
				continue
			}
			if opt, ok := c.G.GorgeOption(o); ok && opt.Battle != 0 {
				return o
			}
		}
		// prefer Ajani, then Mage Slayer and its equip, then everything else
		score := func(a Canon) int {
			switch a.Kind() {
			case ActCast, ActCastFace:
				if ob := mspwOptCard(c, a); ob != nil && ob.Face() != nil {
					switch ob.Face().Name {
					case "Ajani Goldmane":
						return 5
					case "Mage Slayer":
						return 4
					}
					return 2
				}
			case ActActivate:
				if ob := mspwOptCard(c, a); ob != nil && ob.Face() != nil && ob.Face().Name == "Mage Slayer" {
					return 3
				}
				return 2
			case ActPlayLand:
				return 1
			}
			return 0
		}
		var pool []Canon
		best := 0
		for _, o := range c.D.Options {
			var s int
			switch o.Kind() {
			case ActCast, ActCastFace:
				if mspwInstantName(c, o) != "" {
					continue // saved for the carrier trigger's response window
				}
				s = score(o)
			case ActActivate, ActPlayLand:
				s = score(o)
			default:
				continue
			}
			if s == 0 {
				continue
			}
			if s > best {
				best, pool = s, pool[:0]
			}
			if s == best {
				pool = append(pool, o)
			}
		}
		if len(pool) > 0 {
			d := *c.D
			d.Options = pool
			return rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
		}
		// never spend a held instant outside its response window: pass instead
		// when that is an option.
		var safe []Canon
		for _, o := range c.D.Options {
			if (o.Kind() == ActCast || o.Kind() == ActCastFace) && mspwInstantName(c, o) != "" {
				continue
			}
			safe = append(safe, o)
		}
		if len(safe) == 0 {
			return rnd.Pick(c)
		}
		d := *c.D
		d.Options = safe
		return rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
	})
}

// mspwPlay plays r's decks on the custom pool's table and tallies the
// mechanism.
func mspwPlay(t *testing.T, r *Repro, pol func(*mspwTally) Policy) (Result, *mspwTally) {
	t.Helper()
	reg := synthReg(t)
	m, err := r.Matchup(reg)
	if err != nil {
		t.Fatal(err)
	}
	set := cardset.Raw(printedDelveSet(t, r.Cards))
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
	tally := &mspwTally{}
	rc := r.RunConfig()
	rc.MaxDecisions = len(r.Actions) + 400
	if len(r.Actions) == 0 {
		rc.MaxDecisions = 3000
	}
	rc.Check = tally.check
	rc.Finish = func(g *GorgeSide) { tally.check(g, nil) }
	k, err := SetSide(set, tbl, rc.Config)(m, r.Seed)
	if err != nil {
		t.Fatal(err)
	}
	defer CloseSide(k)
	return Lockstep(m, r.Seed, k, pol(tally), rc), tally
}

// TestMageSlayerDefenderPWReprosLockstep replays the recorded game: no
// divergence and no kernel stall, every action served, and both arms of the
// mechanism reached -- a Mage Slayer trigger on an attacked planeswalker
// resolved with its carrier alive, and one after the carrier left.
func TestMageSlayerDefenderPWReprosLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	r, err := ReadRepro(filepath.Join("testdata", "repro", mspwRepro+".repro.json"))
	if err != nil {
		t.Fatal(err)
	}
	pol := &ScriptPolicy{Actions: r.Actions}
	res, tally := mspwPlay(t, r, func(*mspwTally) Policy { return pol })
	if res.Div != nil {
		t.Fatalf("divergence at %d of %d: %s %s %v\nref %q\nker %q\n%s", res.Div.At, len(r.Actions), res.Div.What, res.Div.Field, res.Div.Diff, res.Div.RefOptions, res.Div.KerOptions, res.Div.Detail)
	}
	if res.StallUnmodelled || res.Stall != "" && len(res.Stall) >= len(KernelStallPrefix) && res.Stall[:len(KernelStallPrefix)] == KernelStallPrefix {
		t.Fatalf("kernel stopped: %s", res.Stall)
	}
	if msg := reproSkipFailure(r, pol); msg != "" {
		t.Fatal(msg)
	}
	if tally.alive == 0 || tally.gone == 0 {
		t.Fatalf("%d Mage Slayer triggers on an attacked planeswalker (%d alive, %d gone): the game does not reach the mechanism", tally.triggers, tally.alive, tally.gone)
	}
}

// TestMageSlayerDefenderPWRecord searches seeds of the decks for a game that
// reaches both arms and writes it (MAGESLAYER_RECORD=1); the games it writes
// are the committed repros.
func TestMageSlayerDefenderPWRecord(t *testing.T) {
	if os.Getenv("MAGESLAYER_RECORD") == "" {
		return
	}
	reg := synthReg(t)
	set := cardset.Raw(printedDelveSet(t, [2][]string{mspwDeck, mspwDeck}))
	entries := func(names []string) []cardset.Entry {
		es := make([]cardset.Entry, len(names))
		for i, n := range names {
			es[i] = cardset.Entry{Name: n, Count: 1}
		}
		return es
	}
	for seed := uint64(1); seed <= 200; seed++ {
		r := &Repro{Format: reproFormat, Decks: [2]string{"mspw_0", "mspw_1"},
			Cards: [2][]string{mspwDeck, mspwDeck}, Seed: seed, PassElision: false,
			Surface: "full", Kernel: "custom", Pool: "custom",
			Why: "Mage Slayer's DealDamageFrom defending mode damages the planeswalker its carrier attacked, alive and after the carrier left"}
		m, err := r.Matchup(reg)
		if err != nil {
			t.Fatal(err)
		}
		tbl, _, _, err := set.Matchup(entries(r.Cards[0]), entries(r.Cards[1]))
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, Config{GorgeAutopaySurface: true})(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		tally := &mspwTally{}
		rc := r.RunConfig()
		rc.MaxDecisions = 4000
		rc.Check = tally.check
		rc.Finish = func(g *GorgeSide) { tally.check(g, nil) }
		res := Lockstep(m, seed, k, mspwPolicy(seed), rc)
		CloseSide(k)
		if res.Div != nil {
			t.Fatalf("seed %d: divergence at %d: %s %s %v", seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
		}
		if res.StallUnmodelled || res.Stall != "" {
			t.Fatalf("seed %d: kernel stopped: %s", seed, res.Stall)
		}
		if tally.alive == 0 || tally.gone == 0 {
			t.Logf("seed %d: %d triggers (%d alive, %d gone): no mechanism", seed, tally.triggers, tally.alive, tally.gone)
			continue
		}
		r.setActions(res.Actions)
		if err := r.Write(filepath.Join("testdata", "repro", mspwRepro+".repro.json")); err != nil {
			t.Fatal(err)
		}
		t.Logf("seed %d: %d triggers (%d alive, %d gone), %d decisions", seed, tally.triggers, tally.alive, tally.gone, len(res.Actions))
		return
	}
	t.Fatal("no seed reached both arms of the mechanism")
}
