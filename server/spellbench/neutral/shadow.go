package neutral

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/rules"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/server/spellbench/protocol"
)

// shadow is a gorge engine rebuilt from one observation and parked at the
// seat's priority decision, so gorge's own payment planner can run on a
// board another engine reported. It exists only to be asked
// Engine.EnsurePaymentActions: every card is placed by folding events
// straight into the game (events.Emit), which queues no triggers, so the
// board is exactly the observed one and nothing resolves on it.
type shadow struct {
	e   *rules.Engine
	ids map[state.ObjID]state.ObjID // shadow object -> the builder's gorge id
}

// shadowDeckSize pads each seat's rebuilt deck; the filler cards are never
// moved out of the library.
const shadowDeckSize = 40

// placement is one observed object the shadow must hold.
type placement struct {
	rec   protocol.ObjectRecord
	zone  state.Zone
	owner state.PlayerID
	card  *cards.Card // nil for a token
	stem  string      // the token script, for a token
	stack *protocol.StackEntry
	obj   state.ObjID // the shadow object, once assigned
}

// filler is a card for the padding of a rebuilt deck.
func (b *Builder) filler() (*cards.Card, bool) {
	for _, n := range []string{"Wastes", "Plains", "Island", "Swamp", "Mountain", "Forest"} {
		if c, ok := b.Reg.Lookup(n); ok {
			return c, true
		}
	}
	return nil, false
}

// tokenStem is the token script an observed token stands for.
func (b *Builder) tokenStem(r protocol.ObjectRecord) string {
	f := b.face(r.CardName, true, r.Characteristics)
	if f == nil || r.CardName == nil {
		return ""
	}
	for _, stem := range b.tokens[*r.CardName] {
		if b.Reg.Tokens[stem].Faces[0] == f {
			return stem
		}
	}
	return ""
}

// shadow rebuilds the game o reports. It is nil, with no error, when the
// viewer does not hold priority (there is nothing to plan). A card in
// handLike is placed in the viewer's hand: the host offers a cast of it from
// another zone (an exiled card the seat may play, a graveyard cast), a
// permission the observation does not carry and the planner reads off the
// zone. A card another seat owns is rebuilt as the viewer's own, so it is in
// the hand the planner reads.
func (b *Builder) shadow(o *protocol.Observation, handLike map[string]bool) (*shadow, error) {
	sh, err := b.rebuild(o, handLike, o.PhaseStep)
	if err != nil && (o.PhaseStep == "declare_attackers" || o.PhaseStep == "declare_blockers" || o.PhaseStep == "combat_damage") {
		// The rebuilt game declared no attack, so gorge asks for attackers
		// at declare_attackers and skips the later steps; what the planner
		// offers at instant speed is the same at the end of combat.
		return b.rebuild(o, handLike, "end_of_combat")
	}
	return sh, err
}

func (b *Builder) rebuild(o *protocol.Observation, handLike map[string]bool, phaseStep string) (*shadow, error) {
	viewer, err := Seat(o.Viewer)
	if err != nil {
		return nil, err
	}
	if o.ActiveSeat == nil || o.PrioritySeat == nil {
		return nil, fmt.Errorf("no priority seat")
	}
	active, err := Seat(*o.ActiveSeat)
	if err != nil {
		return nil, err
	}
	holder, err := Seat(*o.PrioritySeat)
	if err != nil {
		return nil, err
	}
	if holder != viewer {
		return nil, nil
	}
	si := slices.Index(steps, phaseStep)
	if step := state.Step(si); si < 0 || step == state.StepUntap || step == state.StepCleanup {
		return nil, fmt.Errorf("no priority in phase_step %q", phaseStep)
	}
	filler, ok := b.filler()
	if !ok {
		return nil, fmt.Errorf("no filler card")
	}

	var all []*placement
	var decks [2][]*cards.Card
	add := func(r protocol.ObjectRecord, zone state.Zone, st *protocol.StackEntry) error {
		if r.CardName == nil || r.FaceDown {
			return nil
		}
		owner, err := Seat(r.OwnerSeat)
		if err != nil || int(owner) >= len(decks) {
			return fmt.Errorf("owner %q", r.OwnerSeat)
		}
		if handLike[r.ObjectID] {
			zone, owner = state.ZHand, viewer
		}
		p := &placement{rec: r, zone: zone, owner: owner, stack: st}
		if r.Token || r.Copy {
			if p.stem = b.tokenStem(r); p.stem == "" || zone != state.ZBattlefield {
				return nil
			}
		} else {
			if p.card = b.ref(r.CardName).Card; p.card == nil {
				return nil
			}
			decks[owner] = append(decks[owner], p.card)
		}
		all = append(all, p)
		return nil
	}
	for _, pl := range o.Players {
		seat, err := Seat(pl.Seat)
		if err != nil {
			return nil, err
		}
		for _, z := range []struct {
			zone state.Zone
			recs []protocol.ObjectRecord
		}{{state.ZHand, pl.Hand}, {state.ZBattlefield, pl.Battlefield}, {state.ZGraveyard, pl.Graveyard}, {state.ZExile, pl.Exile}} {
			if z.zone == state.ZHand && seat != viewer {
				continue
			}
			for _, r := range z.recs {
				if err := add(r, z.zone, nil); err != nil {
					return nil, err
				}
			}
		}
	}
	for i := range o.Stack {
		e := &o.Stack[i]
		if e.StackKind != "spell" || e.CardName == nil || e.Copy {
			continue
		}
		rec := protocol.ObjectRecord{ObjectRef: e.ObjectRef, FaceDown: e.FaceDown}
		if err := add(rec, state.ZStack, e); err != nil {
			return nil, err
		}
	}
	for i := range decks {
		for len(decks[i]) < shadowDeckSize {
			decks[i] = append(decks[i], filler)
		}
	}
	e := b.shadowEngine(decks)

	// Assign each observed card a genesis object of its card.
	free := map[state.PlayerID]map[*cards.Card][]state.ObjID{0: {}, 1: {}}
	for i := range e.G.Objs {
		if ob := &e.G.Objs[i]; ob.Card != nil {
			free[ob.Owner][ob.Card] = append(free[ob.Owner][ob.Card], ob.ID)
		}
	}
	used := map[state.ObjID]bool{}
	for _, p := range all {
		if p.card == nil {
			continue
		}
		list := free[p.owner][p.card]
		if len(list) == 0 {
			return nil, fmt.Errorf("no object for %s", p.card.Faces[0].Name)
		}
		p.obj, free[p.owner][p.card] = list[0], list[1:]
		used[p.obj] = true
	}
	emit := func(ev events.Event) { events.Emit(e.G, e.L, ev) }
	// The opening hand is not the seat's: shelve what is not observed.
	for pi := range e.G.Players {
		for _, id := range slices.Clone(e.G.Zone(state.ZHand, state.PlayerID(pi))) {
			if !used[id] {
				emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZHand, To: state.ZLibrary})
			}
		}
	}
	passes := int32(len(o.PassedSeats))
	if passes == 0 && holder != active {
		passes = 1
	}
	emit(events.Event{Kind: events.TurnChange, Player: active, Amount: int32(o.Turn)})
	emit(events.Event{Kind: events.StepChange, Step: state.Step(si)})
	emit(events.Event{Kind: events.Priority, Player: holder, Amount: passes})

	sh := &shadow{e: e, ids: map[state.ObjID]state.ObjID{}}
	for _, p := range all {
		switch {
		case p.stem != "":
			controller, err := Seat(p.rec.ControllerSeat)
			if err != nil {
				return nil, err
			}
			before := len(e.G.Objs)
			emit(events.Event{Kind: events.TokenCreate, Player: controller, Text: p.stem})
			if len(e.G.Objs) == before {
				continue
			}
			p.obj = e.G.Objs[before].ID
		case p.zone == state.ZStack:
			emit(events.Event{Kind: events.PutOnStack, Obj: p.obj, Player: p.owner, From: e.G.Obj(p.obj).Zone, To: state.ZStack})
		default:
			if from := e.G.Obj(p.obj).Zone; from != p.zone {
				emit(events.Event{Kind: events.MoveZone, Obj: p.obj, From: from, To: p.zone})
			}
		}
		if p.stack != nil {
			sh.ids[p.obj] = b.IDs.Of(p.stack.ObjectID)
		} else {
			sh.ids[p.obj] = b.IDs.Of(p.rec.ObjectID)
		}
	}
	byV2 := map[string]state.ObjID{}
	for obj, id := range sh.ids {
		byV2[b.IDs.V2(id)] = obj
	}
	for _, p := range all {
		pm := p.rec.Permanent
		if p.zone != state.ZBattlefield || pm == nil || p.obj == 0 {
			continue
		}
		if !pm.SummoningSick {
			emit(events.Event{Kind: events.SetupSettled, Obj: p.obj})
		}
		if pm.Tapped {
			emit(events.Event{Kind: events.Tap, Obj: p.obj})
		}
		for _, k := range slices.Sorted(maps.Keys(pm.Counters)) {
			emit(events.Event{Kind: events.CounterChange, Obj: p.obj, Counter: strings.ToUpper(k), Amount: int32(pm.Counters[k])})
		}
		if pm.AttachedTo != nil && pm.AttachedTo.Object != nil {
			if bearer, ok := byV2[pm.AttachedTo.Object.ObjectID]; ok {
				emit(events.Event{Kind: events.Attach, Obj: p.obj, IDs: []state.ObjID{bearer}})
			}
		}
	}
	for _, pl := range o.Players {
		seat, _ := Seat(pl.Seat)
		if d := pl.Life - e.G.Players[seat].Life; d != 0 {
			emit(events.Event{Kind: events.LifeChange, Player: seat, Amount: d})
		}
		for i, n := range [...]uint32{pl.ManaPool.W, pl.ManaPool.U, pl.ManaPool.B, pl.ManaPool.R, pl.ManaPool.G, pl.ManaPool.C} {
			if n > 0 {
				emit(events.Event{Kind: events.ManaAdd, Player: seat, Amount: int32(n), Counter: string("WUBRGC"[i])})
			}
		}
		if seat == active {
			for range pl.LandsPlayedThisTurn {
				emit(events.Event{Kind: events.LandPlayed, Player: seat})
			}
		}
	}
	e.Advance()
	if d := e.Pending(); d == nil || d.Kind != decision.KPriority || d.Player != viewer {
		return nil, fmt.Errorf("the rebuilt game does not pose the seat's priority")
	}
	return sh, nil
}

// shadowOpeningSeeds bounds the shuffles shadowEngine tries.
const shadowOpeningSeeds = 64

// shadowEngine is a new game over the decks whose opening deal poses no
// begin-the-game-with ask (a Gemstone Caverns dealt into a seat's opening
// hand). The observed game is long past its opening, and answering the ask
// would play the pre-game round on into a first turn the rebuild then
// overwrites, so a deal that poses one is reshuffled under the next seed.
func (b *Builder) shadowEngine(decks [2][]*cards.Card) *rules.Engine {
	var e *rules.Engine
	for seed := uint64(1); seed <= shadowOpeningSeeds; seed++ {
		e = rules.New(rules.Config{Seed: seed, Names: []string{"s0", "s1"}, Decks: [][]*cards.Card{decks[0], decks[1]}, Tokens: b.Reg.Tokens})
		if d := e.Pending(); d == nil || len(d.Options) == 0 || d.Options[0].Kind != "opening_yes" {
			break
		}
	}
	return e
}
