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
// before genState.mo, so State does not grow. p's completed dungeons (read as
// DungeonsCompleted$Amount through plCtr) are the cold region coldDDone.
//
//   - 726.2/726.3: OpTakeInitiative makes its player the single holder and
//     queues the source-less "ventures into Undercity" trigger (726.5: a
//     holder taking it again ventures too); the trigger resolves later, like
//     the upkeep and combat-damage takes.
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
//   - 726.2: when a TakeInitiative effect resolves, at the beginning of the
//     holder's upkeep, and when creatures a player controls deal combat
//     damage to the holder (that player takes the initiative), a source-less
//     trigger ventures into Undercity (gorge's "__initiative_venture":
//     abDelayed with trigAmt initTrigAmt).
//   - 726.4 (more than two seats): when the holder leaves the game, the
//     active player takes the initiative, or the next player in turn order
//     still in the game when the active player is the one leaving (gorge
//     initiativeHandoffOnDeparture, designationHeir); the new holder queues
//     the venture trigger. It runs at each loss site as the seat is marked
//     lost (initLeft), immediately before the monarch handoff, in gorge's
//     loss order.

// DebugNoInitSteal (TestInitLockstepMutation): combat damage to the holder
// takes nothing.
var DebugNoInitSteal bool

// DebugNoInitHandoff (TestFourSeatInitiativeHandoffLockstep): a departed
// holder keeps the initiative (CR 726.4 dropped).
var DebugNoInitHandoff bool

const (
	initTrigAmt = -2 // the abDelayed entry's trigAmt marking the venture trigger (speed's is -1)

	// raRoom: the venture's room ask (ChRoom).
	raRoom uint8 = 250
)

// ChRoom is gorge's venture "room" KChoose: mode 0 the room's first
// NextRoom$ arrow, 1 its second.
const ChRoom ChoiceKind = 250

// prepInit sets anyInit and the dungeon card: some program takes the
// initiative. The plain venture's candidate dungeons (every dungeon card of
// the table but the Undercity, in table order) go on the prep's dungeons.
func prepInit(t *kop.Table, p *prep, g *genPrep) {
	uc := false
	for i := range t.Cards {
		for j := range t.Cards[i].Abilities {
			for _, in := range t.Cards[i].Abilities[j].Prog {
				if in.Op == kop.OpTakeInitiative {
					g.anyInit, uc = true, true
					g.dungeon = CardID(in.C)
				}
				if in.Op == kop.OpExt && in.Ext() == kop.ExtVenture {
					g.anyInit = true
				}
			}
		}
	}
	if !g.anyInit {
		return
	}
	// a plain venture offers every dungeon but Undercity, in table (key) order
	// (gorge dungeonCandidates)
	for i := range t.Cards {
		if len(t.Cards[i].DungeonNext) != 0 && !(uc && CardID(i) == g.dungeon) {
			p.dungeons = append(p.dungeons, CardID(i))
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

// takeInitiative is OpTakeInitiative: p takes the designation and the
// source-less CR 726.2 venture trigger is queued, exactly as gorge's
// QueueInitiativeVenture does from effTakeInitiative. It never asks here; the
// trigger resolves through resolveVentureTrig.
func (s *State) takeInitiative(k *prep, in *kop.Instr) bool {
	p := s.playerRef(k, in.Obj)
	if p >= MaxSeats {
		return true
	}
	s.g.initH = p + 1
	s.initVentureTrig(p)
	return true
}

// dungeonDef is the dungeon card p's marker is in (its object's card).
func (s *State) dungeonDef(k *prep, p uint8) *kop.CardDef {
	return &k.t.Cards[s.objs[s.g.dObj[p]].card]
}

// venture moves p's marker (gorge effVenture); false: an ask is posed
// (s.res.ask = raRoom or raDungeon). undercity: venture into Undercity (the
// initiative's, CR 726.2), else a plain venture (CR 701.49a).
func (s *State) venture(k *prep, p uint8, undercity bool) bool {
	if s.g.room[p] == 0 {
		return s.dungeonChoose(k, p, undercity)
	}
	nx := s.dungeonDef(k, p).DungeonNext[s.g.room[p]-1]
	switch {
	case nx == 0:
		s.dungeonRemove(p) // CR 701.49c: completed; a new one begins
		return s.dungeonChoose(k, p, undercity)
	case nx>>4 == 0:
		s.markerEnter(k, p, nx&15-1)
	default:
		if st := DebugVentureStats; st != nil {
			st.RoomAsks++
		}
		s.res.ask, s.res.askP = raRoom, p
		return false
	}
	return true
}

// dungeonEnter puts a new dungeon object of card c into p's command zone
// with the marker on the top room (gorge ventureEnter).
func (s *State) dungeonEnter(k *prep, p uint8, c CardID) {
	if s.objTableFull() {
		s.capFull(StallBoard)
		return
	}
	// the object keeps newToken's ofToken: ceaseTokensGen walks the object
	// table's token suffix, which a non-token mint would cut short; the
	// dungeon is never in a zone it ceases from, and the projection reads
	// it as no token (gorge AddObject)
	o := s.newToken(c, p)
	s.objs[o].zone = kop.ZoneCommand
	s.g.dObj[p] = o
	s.markerEnter(k, p, 0)
}

// dDoneCold is the per-seat completed-dungeon counts (gorge Player.
// CompletedDungeons), or nil when the table never ventures (genPrep.anyInit).
// A cold region: genState has no padding to spare without moving the pinned
// fixed layout.
func (s *State) dDoneCold() []uint8 { return coldDDone.of(s) }

var coldDDone = newCold[uint8]("dungeonsDone", func(g *genPrep) int {
	if g.anyInit {
		return MaxSeats
	}
	return 0
})

// dungeonRemove completes p's dungeon: its object ceases (gorge
// DungeonRemove moves it to ZCeased) and the completed count grows (gorge
// DungeonComplete; both callers are completions).
func (s *State) dungeonRemove(p uint8) {
	o := s.g.dObj[p]
	s.touch(o)
	s.objs[o].zone = kop.ZoneNone
	s.objs[o].ver++
	s.g.room[p] = 0
	if c := s.dDoneCold(); c != nil {
		c[p] = uint8(min(int(c[p])+1, 255)) // gorge foldDungeonComplete
	}
}

// markerEnter moves p's marker to room r and queues its ability (CR 309.4c).
func (s *State) markerEnter(k *prep, p, r uint8) {
	s.g.room[p] = r + 1
	o := s.g.dObj[p]
	s.touch(o)
	if s.pl[p].lost {
		return
	}
	s.enqueue(o, s.objs[o].ver, s.objs[o].card, r, p, NoObj, p, 0)
}

// initVentureTrig queues p's source-less "ventures into Undercity" trigger.
func (s *State) initVentureTrig(p uint8) {
	s.enqueue(NoObj, 0, 0, abDelayed, p, NoObj, p, initTrigAmt)
}

// initLeft runs as seat p is marked lost, at every loss site (sba's life and
// empty-library loops, poisonSBA, cmdDamageSBA, lose). It sits immediately
// before monarchLeft, matching gorge's losePlayer, which hands the initiative
// off before the monarch (cantlose.go). Those are gorge's loss order
// (checkLoseConditions: life, then an empty library, then poison, then
// commander damage, each in seat order), and gorge's losePlayer hands the
// designation off before the batch's next loss is made, so a seat that loses
// later in the same check is still in the game when the heir is chosen. At two
// seats a loss ends the game and the call folds away.
func (s *State) initLeft(k *prep, p uint8) {
	if !s.duel() && k.gen != nil && k.anyInit && s.g.initH == p+1 {
		s.initHandoff(k, p)
	}
}

// initHandoff is CR 726.4 for the departed holder p: the active player takes
// the initiative, else the next player in turn order still in the game (gorge
// designationHeir), and the new holder ventures -- gorge's handoff queues the
// CR 726.2 venture trigger exactly as the combat-damage take does. With nobody
// else left the designation stays.
//
//go:noinline
func (s *State) initHandoff(k *prep, p uint8) {
	if DebugNoInitHandoff {
		return
	}
	h, ok := s.designationHeir(p)
	if !ok {
		return
	}
	s.g.initH = h + 1
	s.initVentureTrig(h)
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
	if s.duel() {
		def = s.nextSeat(s.active)
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
		if ev.dst == NoSlot && ev.n > 0 && s.perms[ev.src].ctrl == s.active && s.perms[ev.src].atk == PlayerCode(def) {
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
	if s.pl[p].lost || s.venture(k, p, true) {
		return true
	}
	s.ctl = ctlResolve
	return false
}

// initSBA is the state-based pass's dungeon step: true when a dungeon was
// completed and the pass runs again.
func (s *State) initSBA(k *prep) bool { return initOn(k) && s.dungeonSBA(k) }

// dungeonSBA is CR 704.5t (gorge dungeonCompletion); true: a dungeon was
// completed, so the state-based pass runs again.
//
//go:noinline
func (s *State) dungeonSBA(k *prep) (done bool) {
	for p := uint8(0); p < s.nSeats(); p++ {
		if s.out(p) {
			continue
		}
		r := s.g.room[p]
		if r == 0 || s.dungeonDef(k, p).DungeonNext[r-1] != 0 || s.pl[p].lost {
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
			done = true
		}
	}
	return done
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
	nx := s.dungeonDef(k, p).DungeonNext[s.g.room[p]-1]
	r := nx & 15
	if a.Mode() == 1 {
		r = nx >> 4
	}
	s.markerEnter(k, p, r-1)
	s.ventureResume(k)
	return true
}

// ventureResume goes on after a venture ask's answer: the venture trigger
// finishes, a program (OpTakeInitiative's, ExtVenture's) continues.
func (s *State) ventureResume(k *prep) {
	if s.res.e.ab == abDelayed {
		s.res.ask = raNone
		s.finishResolve(k, false)
		return
	}
	s.continueRes(k, 0)
}
