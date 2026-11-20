package oracle

// The creator-sourced delayed-exile lockstep (GORGE A10). Rionya, Fire
// Dancer's token-copy rider registers __kwWarpExile with the token as the
// affected object and Rionya as the creator (CR 603.7d's |CR= trailer).
// Thassa, Deep-Dwelling blinked Rionya while the rider waited, gorge's
// SourceReturned stamp followed the creator's re-entry and Defined$ Self
// named nothing: the token survived the end-step exile. dirac exiled it.

import (
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/canon"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestDelayedExileCreatorBlinkedReprosLockstep replays the recorded
// divergent game: after the guard it replays clean to the end of the
// recording.
func TestDelayedExileCreatorBlinkedReprosLockstep(t *testing.T) {
	stdReproNamed(t, "delayed_exile_creator_blinked")
}

// TestDelayedExileCreatorBlinkedMutation restores the unguarded
// SourceIsNewObject read through effects.DebugDelayedExileCreatorIdentity
// and requires the recorded game to diverge again (dirac is right; dirac
// has nothing to mutate, so the hook is gorge-side).
func TestDelayedExileCreatorBlinkedMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	r, err := ReadRepro(filepath.Join("testdata", "repro", "delayed_exile_creator_blinked.repro.json"))
	if err != nil {
		t.Fatal(err)
	}
	effects.DebugDelayedExileCreatorIdentity = true
	defer func() { effects.DebugDelayedExileCreatorIdentity = false }()
	res := replayDecodedRes(t, synthReg(t), globSet(t, r.Pool, r), r)
	if res.Div == nil {
		t.Fatalf("mutation produced no divergence over %d recorded actions", len(r.Actions))
	}
	t.Logf("mutation diverged at %d: %s %s %v", res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
}

// blinkExileTally counts, from gorge's own log, a creator-sourced exile
// rider that exiled its affected object after the creator was exiled since
// the registration — the exact mechanism the guard protects.
type blinkExileTally struct {
	last       int
	creatorOf  map[state.ObjID]state.ObjID
	blinked    map[state.ObjID]bool
	fired      map[state.ObjID]bool // the promise's rider has been pushed
	regs       int                  // creator-sourced registrations seen
	blinks     int                  // tracked creators exiled while tracked
	reached    int
	firstWhere string
}

func newBlinkExileTally() *blinkExileTally {
	return &blinkExileTally{
		creatorOf: map[state.ObjID]state.ObjID{},
		blinked:   map[state.ObjID]bool{},
		fired:     map[state.ObjID]bool{},
	}
}

// walk folds the log's new events into the tally; check and finish share it.
func (p *blinkExileTally) walk(ref *GorgeSide) {
	g := ref.E.G
	evs := ref.E.L.Events
	for ; p.last < len(evs); p.last++ {
		e := &evs[p.last]
		switch e.Kind {
		case events.DelayedRegister:
			if e.Counter != "__kwWarpExile" {
				continue
			}
			i := strings.LastIndex(e.Text, "|CR=")
			if i < 0 {
				continue // self-sourced riders are not this mechanism
			}
			n, err := strconv.ParseUint(strings.TrimSpace(e.Text[i+4:]), 10, 32)
			if err != nil {
				continue
			}
			cr := state.ObjID(n)
			if g.Obj(cr) == nil {
				continue
			}
			p.creatorOf[e.Obj] = cr
			p.blinked[cr] = false // the promise is fresh; only a later blink counts
			p.regs++
		case events.DelayedPush:
			if e.Counter != "__kwWarpExile" {
				continue
			}
			// The rider fired; its affected object's next exile move is the
			// rider's own, not another effect's blink of the token.
			for aff, cr := range p.creatorOf {
				if cr == e.Obj {
					p.fired[aff] = true
				}
			}
		case events.MoveZone:
			cr, tracked := p.creatorOf[e.Obj]
			if !tracked {
				// a tracked CREATOR exiled: it was blinked while its promise
				// (or the rider on the stack) waited.
				if e.To == state.ZExile {
					if _, isCreator := p.blinked[e.Obj]; isCreator {
						p.blinked[e.Obj] = true
						p.blinks++
					}
				}
				continue
			}
			wasFired := p.fired[e.Obj]
			wasBlinked := p.blinked[cr]
			delete(p.creatorOf, e.Obj)
			delete(p.fired, e.Obj)
			delete(p.blinked, cr)
			if e.To == state.ZExile && wasFired && wasBlinked {
				// the rider's own exile of its affected object, after the
				// creator was blinked since the registration.
				p.reached++
				p.firstWhere = "affected exiled after its creator's blink"
			}
		}
	}
}

// rionyaBlinkChooser steers the play the mechanism needs: Rionya and Thassa
// come out first, the token copies Mogg Maniac (a copy of Rionya would die
// to the legend rule at once), the mint spell is held for the end step, and
// the blink lands either while the promise is pending or while the rider
// itself is on the stack.
func rionyaBlinkChooser() func(*PickCtx) (Canon, bool) {
	promiseBlinks := 0
	pick := func(c *PickCtx, name string) (Canon, bool) {
		for _, o := range c.D.Options {
			if o.Kind() != ActTarget {
				continue
			}
			opt, ok := c.G.GorgeOption(o)
			if !ok || opt.Obj == 0 {
				continue
			}
			if ob := c.G.E.G.Obj(opt.Obj); ob != nil && ob.Face() != nil && ob.Face().Name == name {
				return o, true
			}
		}
		return 0, false
	}
	return func(c *PickCtx) (Canon, bool) {
		if c.G == nil {
			return 0, false
		}
		g := c.G.E.G
		optionObj := func(o Canon) *state.Object {
			opt, ok := c.G.GorgeOption(o)
			if !ok || opt.Obj == 0 {
				return nil
			}
			return g.Obj(opt.Obj)
		}
		d := c.G.Pending()
		if d == nil {
			return 0, false
		}
		if d.Kind == decision.KPriority {
			// The exile rider itself is on the stack: blink Rionya in
			// response (the live games' scenario), then pass so it resolves.
			for _, sid := range g.Stack {
				ro := g.Obj(sid)
				if ro == nil || ro.Ability == nil || ro.Ability.API != "ChangeZone" ||
					ro.Ability.Params["Origin"] != "Battlefield" || ro.Ability.Params["Destination"] != "Exile" {
					continue
				}
				if promiseBlinks < 1 {
					for _, o := range c.D.Options {
						if o.Kind() == ActActivate {
							if ob := optionObj(o); ob != nil && ob.Face() != nil && ob.Face().Name == "Thassa, Deep-Dwelling" {
								promiseBlinks++
								return o, true
							}
						}
					}
				}
				for _, o := range c.D.Options {
					if o.Kind() == ActPass {
						return o, true
					}
				}
				return 0, false
			}
			// Build the pod's engine before anything else: land first, then
			// the enablers. The legendary casts are skipped once the seat
			// controls one, so the spare turns cast the next enabler.
			controls := func(p state.PlayerID, name string) bool {
				for _, id := range g.Zone(state.ZBattlefield, p) {
					if o := g.Obj(id); o != nil && o.Face() != nil && o.Face().Name == name {
						return true
					}
				}
				return false
			}
			wants := []struct {
				kind canon.CanonKind
				name string
			}{{ActPlayLand, ""}}
			if !controls(d.Player, "Rionya, Fire Dancer") {
				wants = append(wants,
					struct {
						kind canon.CanonKind
						name string
					}{ActCast, "Rionya, Fire Dancer"},
					struct {
						kind canon.CanonKind
						name string
					}{ActCastFace, "Rionya, Fire Dancer"})
			}
			if !controls(d.Player, "Thassa, Deep-Dwelling") {
				wants = append(wants,
					struct {
						kind canon.CanonKind
						name string
					}{ActCast, "Thassa, Deep-Dwelling"},
					struct {
						kind canon.CanonKind
						name string
					}{ActCastFace, "Thassa, Deep-Dwelling"})
			}
			// A pending creator-sourced promise of this seat: blink Rionya
			// ONCE, then pass so the turn reaches the end step while the
			// promise is still pending.
			pendingPromise := false
			for i := range g.Delayed {
				if dt := &g.Delayed[i]; dt.Execute == "__kwWarpExile" && dt.Creator != 0 && dt.Controller == d.Player {
					pendingPromise = true
					break
				}
			}
			if pendingPromise {
				if promiseBlinks < 1 && controls(d.Player, "Thassa, Deep-Dwelling") {
					for _, o := range c.D.Options {
						if o.Kind() == ActActivate {
							if ob := optionObj(o); ob != nil && ob.Face() != nil && ob.Face().Name == "Thassa, Deep-Dwelling" {
								promiseBlinks++
								return o, true
							}
						}
					}
				}
				for _, o := range c.D.Options {
					if o.Kind() == ActPass {
						return o, true
					}
				}
				return 0, false
			}
			// Cast the spell that fires Rionya's trigger only at the END
			// step: a promise minted there lives across the whole following
			// turn, whose rounds offer the blink with full mana (the
			// main-phase rounds are otherwise spent empty, and an empty
			// seat's single-option priority is elided, never posed).
			if g.Step == state.StepEnd {
				freeIslands := 0
				for _, id := range g.Zone(state.ZBattlefield, d.Player) {
					if o := g.Obj(id); o != nil && o.Face() != nil && o.Face().Name == "Island" && !o.Tapped {
						freeIslands++
					}
				}
				if controls(d.Player, "Rionya, Fire Dancer") && controls(d.Player, "Thassa, Deep-Dwelling") && freeIslands >= 2 {
					wants = append(wants,
						struct {
							kind canon.CanonKind
							name string
						}{ActCast, "Steady Progress"})
				}
			}
			wants = append(wants,
				struct {
					kind canon.CanonKind
					name string
				}{ActActivate, "Thassa, Deep-Dwelling"})
			for _, want := range wants {
				for _, o := range c.D.Options {
					if o.Kind() != want.kind {
						continue
					}
					ob := optionObj(o)
					if ob == nil || ob.Face() == nil {
						continue
					}
					if want.name == "" || ob.Face().Name == want.name {
						return o, true
					}
				}
			}
			return 0, false
		}
		if d.Kind != decision.KTarget {
			return 0, false
		}
		// The pending wire decision does not carry its ability's source; the
		// prompt names it ("Choose a target for <source>").
		if strings.Contains(d.Prompt, "for Thassa, Deep-Dwelling") {
			if a, ok := pick(c, "Rionya, Fire Dancer"); ok {
				return a, true
			}
			// Rionya is not on the battlefield: avoid the token copies that
			// carry a pending exile promise -- blinking one breaks it (the
			// affected object changed zones, CR 400.7) and wastes the window.
			for _, o := range c.D.Options {
				if o.Kind() != ActTarget {
					continue
				}
				if opt, ok := c.G.GorgeOption(o); ok && opt.Obj != 0 {
					if ob := g.Obj(opt.Obj); ob != nil && !ob.IsToken {
						return o, true
					}
				}
			}
			return 0, false
		}
		if strings.Contains(d.Prompt, "for Rionya, Fire Dancer") {
			// A Rionya-copy token dies to the legend rule at once, so the
			// copy target is the vanilla creature.
			return pick(c, "Mogg Maniac")
		}
		return 0, false
	}
}

// developmentPolicy plays a land first, then casts, then activates, then
// attacks -- combatPolicy's uniform pick starves a 30-turn pod of mana drops.
// Optional triggers still answer yes; the chooser steers the target asks and
// holds the mint spell for the end step.
func developmentPolicy(seed uint64, choose func(*PickCtx) (Canon, bool)) Policy {
	rnd := RandomPolicy(seed)
	return PolicyFunc(func(c *PickCtx) Canon {
		if a, ok := choose(c); ok {
			return a
		}
		// Steady Progress is the token-mint spell: casting it outside the
		// end step spends the pool and leaves every later round of the turn
		// single-option (elided), so the promise is minted into a window no
		// blink can be asked in.
		avail := make([]Canon, 0, len(c.D.Options))
		for _, o := range c.D.Options {
			if o.Kind() == ActCast && c.G != nil {
				if opt, ok := c.G.GorgeOption(o); ok && opt.Obj != 0 {
					if ob := c.G.E.G.Obj(opt.Obj); ob != nil && ob.Face() != nil &&
						ob.Face().Name == "Steady Progress" && c.G.E.G.Step != state.StepEnd {
						continue
					}
				}
			}
			avail = append(avail, o)
		}
		for _, o := range avail {
			if o.Kind() == ActYes {
				return o
			}
		}
		for _, kind := range []canon.CanonKind{ActPlayLand, ActCast, ActCastFace, ActActivate, ActAttack} {
			var buf []Canon
			for _, o := range avail {
				if o.Kind() == kind {
					buf = append(buf, o)
				}
			}
			if len(buf) > 0 {
				d := *c.D
				d.Options = buf
				return rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
			}
		}
		d := *c.D
		d.Options = avail
		return rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
	})
}

// TestDelayedExileCreatorBlinkedFourSeat plays the mechanism on a four-seat
// pod: Rionya makes a Mogg Maniac token copy with the |CR= exile rider,
// Thassa blinks Rionya, and the end-step exile must still remove the token.
// No seed may diverge, and the mechanism must be reached at least once.
func TestDelayedExileCreatorBlinkedFourSeat(t *testing.T) {
	corpusDirOrSkip(t)
	set := cardsettest.Synth(t, "dr-mbc-abaa")
	deck := deckOf(4, "Rionya, Fire Dancer",
		deckOf(4, "Thassa, Deep-Dwelling",
			deckOf(4, "Mogg Maniac",
				deckOf(12, "Steady Progress",
					deckOf(8, "Mountain", deckOf(16, "Island")...)...)...)...)...)
	var tally *blinkExileTally
	base := fourSeatSetPlay(t, set, [4][]string{deck, deck, deck, deck}, nil,
		func(g *GorgeSide, _ Side) *Divergence {
			if tally != nil {
				tally.walk(g)
			}
			return nil
		},
		func(g *GorgeSide) {
			if tally != nil {
				tally.walk(g)
			}
		},
		func(seed uint64) Policy {
			return developmentPolicy(seed, rionyaBlinkChooser())
		})
	// the counters accumulate across seeds; the per-object maps stay per game
	var regs, blinks, reached int
	play := func(seed uint64) Result {
		tally = newBlinkExileTally()
		res := base(seed)
		regs += tally.regs
		blinks += tally.blinks
		reached += tally.reached
		return res
	}
	const seeds = 20
	fourSeatAtkClean(t, play, seeds)
	t.Logf("creator-blink: %d reached, %d creator-sourced registrations, %d creator blinks across %d seeds",
		reached, regs, blinks, seeds)
	if reached == 0 {
		t.Fatalf("the mechanism was never reached in %d seeds: no registered rider exiled its object after a creator blink", seeds)
	}
}
