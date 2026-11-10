package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// the initiative (CR 726) and the Undercity dungeon (CR 309,
// 701.49), copied from gorge (effects/initiative.go effTakeInitiative,
// effects/venture.go effVenture, rules/dungeon_room.go, rules/turn.go's
// upkeep venture and rules/combat.go's combat-damage steal). Cold and out of
// line: a table with no OpTakeInitiative never takes these paths
// (genPrep.anyInit).
//
// State: genState.initH (0 nobody, else holder+1), genState.room[p] (0: p
// has no dungeon, else the marker's room index+1) and genState.dObj[p] (p's
// dungeon object, read only while room[p] != 0): five bytes in the padding
// before genState.mo, so State does not grow.
//
//   - 726.2/726.3: OpTakeInitiative makes its player the single holder, then
//     ventures into Undercity (726.5: a holder taking it again ventures too).
//   - 701.49a: with no dungeon, a dungeon object enters p's command zone
//     (gorge DungeonCreate: an object of the dungeon card, no token, in the
//     oracle's card instances like any object) with the marker on the top
//     room; 701.49b: else the marker follows the room's arrow, or the
//     venturer picks between two (gorge's "room" KChoose in printed order,
//     ChRoom); 701.49c: from the bottommost room the dungeon is completed
//     (gorge DungeonComplete + DungeonRemove: the object ceases) and a new
//     one begins.
//   - 309.4c: the room entered queues its ability, the dungeon card's latent
//     ability of the same index, a trigger controlled by the dungeon's owner
//     whose source is the dungeon object (gorge checkDungeonRoomTriggers).
//   - 704.5t (dungeonSBA): a dungeon whose marker is on the bottommost room
//     and that is the source of no pending or stacked room ability is
//     completed (gorge dungeonCompletion).
//   - 726.2: at the beginning of the holder's upkeep, and when creatures a
//     player controls deal combat damage to the holder (that player takes
//     the initiative), a source-less trigger ventures into Undercity
//     (gorge's "__initiative_venture": abDelayed with trigAmt initTrigAmt).

// DebugNoInitSteal (TestInitLockstepMutation): combat damage to the holder
// takes nothing.
var DebugNoInitSteal bool

const (
	initTrigAmt = -2 // the abDelayed entry's trigAmt marking the venture trigger (speed's is -1)

	// raRoom: the venture's room ask (ChRoom).
	raRoom uint8 = 250
)

// ChRoom is gorge's venture "room" KChoose: mode 0 the room's first
// NextRoom$ arrow, 1 its second.
const ChRoom ChoiceKind = 250

// prepInit sets anyInit and the dungeon card: some program takes the
// initiative.
func prepInit(t *kop.Table, g *genPrep) {
	for i := range t.Cards {
		for j := range t.Cards[i].Abilities {
			for _, in := range t.Cards[i].Abilities[j].Prog {
				if in.Op == kop.OpTakeInitiative {
					g.anyInit = true
					g.dungeon = CardID(in.C)
				}
			}
		}
	}
}

// initOn: the table has a dungeon card (prep.anyInit; a generated table
// with an OpTakeInitiative, so k.gen is set).
func initOn(k *prep) bool { return k.anyInit }

// Initiative is the initiative's holder, -1 when nobody has it (oracle).
func (s *State) Initiative() int8 { return int8(s.g.initH) - 1 }

// DungeonRoom is player p's venture marker: 0 no dungeon, else the room
// index + 1 in the dungeon's printed order (oracle).
func (s *State) DungeonRoom(p uint8) uint8 { return s.g.room[SeatIx(p)] }

// takeInitiative is OpTakeInitiative; false: the venture asks.
func (s *State) takeInitiative(k *prep, in *kop.Instr) bool {
	p := s.playerRef(k, in.Obj)
	if p > 1 {
		return true
	}
	s.g.initH = p + 1
	return s.venture(k, p)
}

// venture moves p's marker (gorge effVenture); false: the room ask is
// posed (s.res.ask = raRoom).
func (s *State) venture(k *prep, p uint8) bool {
	if s.g.room[p] == 0 {
		s.dungeonEnter(k, p)
		return true
	}
	nx := k.t.Cards[k.gen.dungeon].DungeonNext[s.g.room[p]-1]
	switch {
	case nx == 0:
		s.dungeonRemove(p) // CR 701.49c: completed; a new one begins
		s.dungeonEnter(k, p)
	case nx>>4 == 0:
		s.markerEnter(k, p, nx&15-1)
	default:
		s.res.ask, s.res.askP = raRoom, p
		return false
	}
	return true
}

// dungeonEnter puts a new dungeon object into p's command zone with the
// marker on the top room (gorge ventureEnter).
func (s *State) dungeonEnter(k *prep, p uint8) {
	if s.objTableFull() {
		s.capFull(StallBoard)
		return
	}
	// the object keeps newToken's ofToken: ceaseTokensGen walks the object
	// table's token suffix, which a non-token mint would cut short; the
	// dungeon is never in a zone it ceases from, and the projection reads
	// it as no token (gorge AddObject)
	o := s.newToken(k.gen.dungeon, p)
	s.objs[o].zone = kop.ZoneCommand
	s.g.dObj[p] = o
	s.markerEnter(k, p, 0)
}

// dungeonRemove completes p's dungeon: its object ceases (gorge
// DungeonRemove moves it to ZCeased).
func (s *State) dungeonRemove(p uint8) {
	o := s.g.dObj[p]
	s.touch(o)
	s.objs[o].zone = kop.ZoneNone
	s.objs[o].ver++
	s.g.room[p] = 0
}

// markerEnter moves p's marker to room r and queues its ability (CR 309.4c).
func (s *State) markerEnter(k *prep, p, r uint8) {
	s.g.room[p] = r + 1
	o := s.g.dObj[p]
	s.touch(o)
	if s.pl[p].lost {
		return
	}
	s.enqueue(o, s.objs[o].ver, k.gen.dungeon, r, p, NoObj, p, 0)
}

// initVentureTrig queues p's source-less "ventures into Undercity" trigger.
func (s *State) initVentureTrig(p uint8) {
	s.enqueue(NoObj, 0, 0, abDelayed, p, NoObj, p, initTrigAmt)
}

// initUpkeep is the holder's upkeep venture trigger (CR 726.2; gorge
// turn.go, queued after the step's printed and delayed triggers).
//
//go:noinline
func (s *State) initUpkeep(k *prep) {
	if s.g.initH == s.active+1 && !s.pl[s.active].lost {
		s.initVentureTrig(s.active)
	}
}

// initCombat is CR 726.2's take after a combat damage pass (gorge
// runCombatAssignments' tail): an attacking creature's damage to the
// defending player who holds the initiative makes the active player take
// it, and the venture trigger is queued after the pass's other triggers.
//
//go:noinline
func (s *State) initCombat(k *prep, r *dmgRec) {
	var def uint8
	if MaxSeats == 2 {
		def = 1 - s.active
		if s.g.initH != def+1 || DebugNoInitSteal {
			return
		}
	} else {
		// the initiative's holder, when an opponent holds it
		def = s.g.initH - 1
		if s.g.initH == 0 || def == s.active || DebugNoInitSteal {
			return
		}
	}
	for i := 0; i < r.nev; i++ {
		ev := &r.ev[i]
		if ev.dst == NoSlot && ev.n > 0 && s.perms[ev.src].ctrl == s.active && s.perms[ev.src].atk == PlayerTarget0+def {
			s.g.initH = s.active + 1
			s.initVentureTrig(s.active)
			return
		}
	}
}

// resolveVentureTrig resolves the venture trigger; false: the room ask is
// posed and the answer finishes the resolution.
//
//go:noinline
func (s *State) resolveVentureTrig(k *prep) bool {
	p := s.res.e.ctrl
	if s.pl[p].lost || s.venture(k, p) {
		return true
	}
	s.ctl = ctlResolve
	return false
}

// dungeonSBA is CR 704.5t (gorge dungeonCompletion).
//
//go:noinline
func (s *State) dungeonSBA(k *prep) {
	for p := uint8(0); p < s.nSeats(); p++ {
		if s.out(p) {
			continue
		}
		r := s.g.room[p]
		if r == 0 || k.t.Cards[k.gen.dungeon].DungeonNext[r-1] != 0 || s.pl[p].lost {
			continue
		}
		o := s.g.dObj[p]
		busy := false
		for i := 0; i < int(s.npend) && !busy; i++ {
			busy = s.pend[i].obj == o && s.pend[i].ab != abDelayed
		}
		for i := 0; i < int(s.sp) && !busy; i++ {
			busy = s.stack[i].obj == o && s.stack[i].kind == seTrigger
		}
		if s.res.active && s.res.e.obj == o && s.res.e.kind == seTrigger {
			busy = true // the room ability resolving now has not left the stack
		}
		if !busy {
			s.dungeonRemove(p)
		}
	}
}

// execInitiative runs this family's ops (execGoad's default).
//
//go:noinline
func (s *State) execInitiative(k *prep, in *kop.Instr) (ok, done bool) {
	switch in.Op {
	case kop.OpTakeInitiative:
		return s.takeInitiative(k, in), true
	case kop.OpDigCounters:
		if n := s.value(k, in.A); n > 0 {
			s.g.enterCtr.n, s.g.enterCtr.kind = uint8(n), kop.CounterKind(in.C)
		} else {
			s.g.enterCtr.n = 0
		}
		return true, true
	}
	return s.execStdmech(k, in) // cast_altfree.go
}

func (s *State) askOptionsInit(k *prep, d *Decision) bool {
	if s.res.ask != raRoom {
		return s.askOptionsStdmech(k, d) // cast_altfree.go
	}
	d.Sub = ChRoom
	d.add(MakeAction(AkChoose, 0, 0, 0))
	d.add(MakeAction(AkChoose, 0, 0, 1))
	return true
}

// applyAskInit takes the room answer, then the resolution goes on: the
// venture trigger finishes, OpTakeInitiative's program continues.
func (s *State) applyAskInit(k *prep, a Action) bool {
	if s.res.ask != raRoom {
		return s.applyAskStdmech(k, a) // cast_altfree.go
	}
	p := s.res.askP
	nx := k.t.Cards[k.gen.dungeon].DungeonNext[s.g.room[p]-1]
	r := nx & 15
	if a.Mode() == 1 {
		r = nx >> 4
	}
	s.markerEnter(k, p, r-1)
	if s.res.e.ab == abDelayed {
		s.res.ask = raNone
		s.finishResolve(k, false)
		return true
	}
	s.continueRes(k, 0)
	return true
}
