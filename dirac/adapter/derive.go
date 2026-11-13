package adapter

import (
	"slices"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/canon"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// noObject is the projection of an object that did not exist yet.
var noObject = dirac.ProjObj{Zone: kop.ZoneNone, Attached: dirac.NoObj, Blocks: dirac.NoObj}

// derive logs the difference between two positions. The kernel keeps no history, so
// a client's log is read off the positions: within one answer's burst the order is
// this function's, not the order things happened, a life gain and a loss net out, and
// a mana activation is not logged. It is a pure function of the game, so a replay
// compares it.
func (g *Game) derive(prev *dirac.Projection, ids *idTable, asked dirac.DecisionKind) {
	cur := &g.kp
	before := func(o int) *dirac.ProjObj {
		if o < int(prev.NObj) && ids.obj(uint8(o)) == g.ids.obj(uint8(o)) {
			return &prev.Objs[o]
		}
		return &noObject
	}
	g.deriveTurn(prev)
	common := 0
	for common < int(prev.NStack) && common < int(cur.NStack) && sameEntry(&prev.Stack[common], &cur.Stack[common]) {
		common++
	}
	for i := int(prev.NStack) - 1; i >= common; i-- {
		g.deriveLeft(&prev.Stack[i], ids, i)
	}
	for o := 0; o < int(cur.NObj); o++ {
		g.deriveMove(before(o), &cur.Objs[o], uint8(o))
	}
	for i := common; i < int(cur.NStack); i++ {
		g.derivePushed(&cur.Stack[i], i)
	}
	for o := 0; o < int(cur.NObj); o++ {
		was, now := before(o), &cur.Objs[o]
		if now.Zone != kop.ZoneBattlefield {
			continue
		}
		id := g.ids.obj(uint8(o))
		wasTapped := was.Zone == kop.ZoneBattlefield && was.Flags&dirac.ProjTapped != 0
		switch nowTapped := now.Flags&dirac.ProjTapped != 0; {
		case nowTapped && !wasTapped:
			g.log.Append(events.Event{Kind: events.Tap, Player: state.PlayerID(now.Ctrl), Obj: id})
		case !nowTapped && wasTapped:
			g.log.Append(events.Event{Kind: events.Untap, Player: state.PlayerID(now.Ctrl), Obj: id})
		}
	}
	for o := 0; o < int(cur.NObj); o++ {
		g.deriveMarks(before(o), &cur.Objs[o], g.ids.obj(uint8(o)))
	}
	g.derivePlayers(prev)
	for o := 0; o < int(cur.NObj); o++ {
		g.deriveAttach(before(o), &cur.Objs[o], g.ids.obj(uint8(o)), ids)
	}
	g.deriveCombat(before, asked)
}

// deriveTurn logs each step passed, and the turn as it moves.
func (g *Game) deriveTurn(prev *dirac.Projection) {
	cur := &g.kp
	if prev.Turn == cur.Turn && prev.Step == cur.Step {
		return
	}
	logged := stepOf[prev.Step]
	step := func(s kop.Step) {
		if stepOf[s] != logged {
			if logged == state.StepEndCombat {
				g.log.Append(events.Event{Kind: events.EndCombatReset})
			}
			logged = stepOf[s]
			g.log.Append(events.Event{Kind: events.StepChange, Player: state.PlayerID(cur.Active), Step: logged})
		}
	}
	if prev.Turn != cur.Turn {
		for s := prev.Step + 1; prev.Turn > 0 && s <= kop.StepCleanup; s++ {
			step(s)
		}
		g.log.Append(events.Event{Kind: events.TurnChange, Player: state.PlayerID(cur.Active), Amount: int32(cur.Turn)})
		g.log.Append(events.Event{Kind: events.StepChange, Player: state.PlayerID(cur.Active), Step: stepOf[kop.StepUntap]})
		logged = stepOf[kop.StepUntap]
		for s := kop.StepUpkeep; s <= cur.Step; s++ {
			step(s)
		}
		return
	}
	// a step earlier in the same turn is an added phase: only where it landed is known
	for s := prev.Step + 1; s < cur.Step; s++ {
		step(s)
	}
	step(cur.Step)
}

func sameEntry(a, b *dirac.ProjStack) bool {
	return a.Kind == b.Kind && a.Source == b.Source && a.Card == b.Card && a.Ability == b.Ability
}

// deriveLeft logs a stack entry that is gone. A cast or activation reversed before it
// finished never resolved: the spell's move back is the record of it.
func (g *Game) deriveLeft(e *dirac.ProjStack, ids *idTable, i int) {
	if g.building == e.Source {
		return
	}
	id := ids.stack[i]
	if e.Kind == 0 {
		id = ids.obj(uint8(e.Source))
	}
	g.log.Append(events.Event{Kind: events.Resolve, Player: state.PlayerID(e.Ctrl), Obj: id})
}

// deriveMove logs object o changing zone, ascending object id.
func (g *Game) deriveMove(was, now *dirac.ProjObj, o uint8) {
	from, to := zoneOf(was.Zone), zoneOf(now.Zone)
	if now.Token && from == state.ZLibrary {
		from = state.ZCeased // a token's object before it is made
	}
	if from == to {
		return
	}
	e := events.Event{Kind: events.MoveZone, Player: state.PlayerID(now.Owner), Obj: g.ids.obj(o), From: from, To: to}
	e.Secret = now.Flags&dirac.ProjFaceDown != 0 && from.Hidden()
	switch {
	case from == state.ZCeased && now.Token:
		e.Kind, e.Player, e.Text = events.TokenCreate, state.PlayerID(now.Ctrl), g.tbl.Cards[now.Card].Name
	case from == state.ZLibrary && to == state.ZHand:
		e.Kind = events.Draw
	case to == state.ZStack:
		e.Kind, e.Player = events.PutOnStack, state.PlayerID(now.Ctrl)
	}
	g.log.Append(e)
}

// derivePushed logs an ability put on the stack, and how a spell was cast once that
// is known. A spell's own arrival is its move.
func (g *Game) derivePushed(e *dirac.ProjStack, i int) {
	switch e.Kind {
	case 0:
		var flags uint64
		if g.s.FeatStackKicked(i) {
			flags = state.FlagKicked
		}
		if e.X != 0 || flags != 0 {
			g.log.Append(events.Event{Kind: events.CastInfo, Player: state.PlayerID(e.Ctrl), Obj: g.ids.obj(uint8(e.Source)),
				Amount: int32(e.X), Counter: events.FlagsString(flags)})
		}
	case 1:
		g.log.Append(events.Event{Kind: events.AbilityPush, Player: state.PlayerID(e.Ctrl), Obj: g.ids.obj(uint8(e.Source))})
	default:
		g.log.Append(events.Event{Kind: events.TriggerPush, Player: state.PlayerID(e.Ctrl), Obj: g.ids.obj(uint8(e.Source))})
	}
}

// deriveMarks logs a permanent's counters and damage changing.
func (g *Game) deriveMarks(was, now *dirac.ProjObj, id state.ObjID) {
	if now.Zone != kop.ZoneBattlefield {
		return
	}
	for k := range now.Counters {
		if d := int32(now.Counters[k]) - int32(was.Counters[k]); d != 0 && canon.CounterNames[k] != "" {
			g.log.Append(events.Event{Kind: events.CounterChange, Player: state.PlayerID(now.Ctrl), Obj: id, Counter: canon.CounterNames[k], Amount: d})
		}
	}
	for k := range now.CountersX {
		if d := int32(now.CountersX[k]) - int32(was.CountersX[k]); d != 0 {
			g.log.Append(events.Event{Kind: events.CounterChange, Player: state.PlayerID(now.Ctrl), Obj: id, Counter: kop.CtrXNames[k], Amount: d})
		}
	}
	if d := int32(now.Damage) - int32(was.Damage); d > 0 {
		g.log.Append(events.Event{Kind: events.Damage, Player: state.PlayerID(now.Ctrl), Obj: id, Amount: d})
	}
}

// derivePlayers logs life and poison changing.
func (g *Game) derivePlayers(prev *dirac.Projection) {
	for p := range g.kp.Life {
		if d := int32(g.kp.Life[p]) - int32(prev.Life[p]); d != 0 {
			g.log.Append(events.Event{Kind: events.LifeChange, Player: state.PlayerID(p), Amount: d})
		}
	}
	for p := range g.poison {
		n := int32(g.s.Poison(uint8(p)))
		if n != g.poison[p] {
			g.log.Append(events.Event{Kind: events.PlayerCounterChange, Player: state.PlayerID(p), Counter: "POISON", Amount: n - g.poison[p]})
			g.poison[p] = n
		}
	}
}

// deriveAttach logs an Aura or Equipment moving on or off its bearer.
func (g *Game) deriveAttach(was, now *dirac.ProjObj, id state.ObjID, ids *idTable) {
	if now.Zone != kop.ZoneBattlefield || was.Attached == now.Attached {
		return
	}
	if now.Attached != dirac.NoObj {
		g.log.Append(events.Event{Kind: events.Attach, Player: state.PlayerID(now.Ctrl), Obj: id, IDs: []state.ObjID{g.ids.obj(uint8(now.Attached))}})
		return
	}
	g.log.Append(events.Event{Kind: events.Unattached, Player: state.PlayerID(now.Ctrl), Obj: id, IDs: []state.ObjID{ids.obj(uint8(was.Attached))}})
}

// deriveCombat logs a declaration, one that declared nothing included.
func (g *Game) deriveCombat(before func(int) *dirac.ProjObj, asked dirac.DecisionKind) {
	cur := &g.kp
	var attacks [2][]state.ObjID
	var blocks [][2]state.ObjID
	for o := 0; o < int(cur.NObj); o++ {
		was, now := before(o), &cur.Objs[o]
		if now.Flags&dirac.ProjAttacking != 0 && was.Flags&dirac.ProjAttacking == 0 {
			seat := g.defender(now)
			attacks[seat] = append(attacks[seat], g.ids.obj(uint8(o)))
		}
		if now.Flags&dirac.ProjBlocking != 0 && was.Flags&dirac.ProjBlocking == 0 && now.Blocks != dirac.NoObj {
			blocks = append(blocks, [2]state.ObjID{g.ids.obj(uint8(now.Blocks)), g.ids.obj(uint8(o))})
		}
	}
	for p := range attacks {
		if len(attacks[p]) > 0 {
			g.log.Append(events.Event{Kind: events.DeclareAttackers, Player: state.PlayerID(p), IDs: attacks[p]})
		}
	}
	if asked == dirac.DkAttackers && len(attacks[0])+len(attacks[1]) == 0 {
		g.log.Append(events.Event{Kind: events.DeclareAttackers, Player: state.PlayerID(1 - cur.Active)})
	}
	if len(blocks) > 0 || asked == dirac.DkBlockers {
		slices.SortFunc(blocks, func(a, b [2]state.ObjID) int { return int(a[0]) - int(b[0]) })
		g.log.Append(events.Event{Kind: events.DeclareBlockers, Player: state.PlayerID(1 - cur.Active), Pairs: blocks})
	}
}

// defender is the seat an attacker attacks, a battle's or planeswalker's controller
// included.
func (g *Game) defender(o *dirac.ProjObj) uint8 {
	switch {
	case dirac.TCode(o.Attacks) >= dirac.PlayerTarget0 && dirac.TCode(o.Attacks) != dirac.NoTarget:
		return uint8((dirac.TCode(o.Attacks) - dirac.PlayerTarget0) & 1)
	case dirac.IsObjTarget(dirac.TCode(o.Attacks)):
		return g.kp.Objs[o.Attacks].Ctrl & 1
	}
	return (o.Ctrl + 1) & 1
}

// lossText says why a seat lost, from the position it lost in.
func (g *Game) lossText(p uint8) string {
	switch {
	case g.kp.Life[p] <= 0:
		return "life total is 0 or less"
	case g.s.Poison(p) >= 10:
		return "ten or more poison counters"
	}
	return "drew from an empty library"
}
