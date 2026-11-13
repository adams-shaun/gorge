package dirac

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

const openingHand = 7

// New starts a game with the default Config (pass-elision on, no mulligan
// round). Seat 0 plays deckA, seat 1 deckB.
func New(tbl *Table, deckA, deckB []CardID, seed uint64) *State {
	return NewConfig(tbl, deckA, deckB, seed, Config{})
}

// NewConfig is New with a Config. Genesis follows gorge's genesisDeal: the
// toss is the first RNG draw (IntN(2)), then each seat in index order
// shuffles its whole library (Fisher-Yates, gorge rng.Shuffle) and draws 7.
func NewConfig(tbl *Table, deckA, deckB []CardID, seed uint64, cfg Config) *State {
	s := new(State)
	s.Init(tbl, deckA, deckB, seed, cfg)
	return s
}

// Init is NewConfig into an existing State.
func (s *State) Init(tbl *Table, deckA, deckB []CardID, seed uint64, cfg Config) {
	s.InitN(tbl, [MaxSeats][]CardID{deckA, deckB}, 2, seed, cfg)
}

// InitN starts a game of n seats (2 <= n <= MaxSeats) into an existing
// State: seat p plays decks[p]. Genesis follows gorge's genesisDeal: the
// toss is the first RNG draw, IntN over the seat count, then each seat in
// index order shuffles its library and draws 7. Objects are laid out deck by
// deck in seat order, then the sideboards. At two seats it is exactly Init.
func (s *State) InitN(tbl *Table, decks [MaxSeats][]CardID, n uint8, seed uint64, cfg Config) {
	if n < 2 || n > MaxSeats {
		panic("kernel: seat count out of range")
	}
	clear(s.perms[:s.permHi])
	s.stateCore = stateCore{}
	// the last-bearer pad's empty sentinel: a zero
	// record would name object 0, which is a real card.
	for i := range s.g.attLKI {
		s.g.attLKI[i].obj, s.g.attLKI[i].host = NoObj, NoObj
	}
	s.tbl = registerTable(tbl)
	lay := &s.k().cold
	if cfg.Commander {
		lay = &s.k().coldCmd // a partner pair's bookkeeping (zone_cmdpartner.go)
	}
	s.resetCold(lay)
	s.initPartners()
	s.g.on = s.k().gen != nil
	s.g.enteredOn = s.g.on && s.k().gen.enteredNeed
	if s.g.on && s.k().gen.graveDrawn {
		s.cfgx |= cfgxGraveDrawn
	}
	if s.g.on && s.k().gen.anyRepl&rbDrawMore != 0 {
		s.cfgx |= cfgxDrawMore
	}
	if s.g.on && s.k().gen.anyCantDraw {
		s.cfgx |= cfgxCantDraw
	}
	s.g.replaced = NoObj // idle: the as-enters replacement checks compare against NoObj
	if cfg.NoElide {
		s.cfg |= cfgNoElide
	}
	if cfg.AutoDivide {
		s.cfg |= cfgAutoDivide
	}
	if cfg.ArrangeRestOrder {
		s.cfgx |= cfgxArrangeRest
	}
	if cfg.GorgeAltAdd {
		s.cfg |= cfgGorgeAltAdd
	}
	if cfg.GorgeMustAttack {
		s.cfg |= cfgGorgeMustAttack
	}
	if cfg.GorgeAutopaySurface {
		s.cfg |= cfgGorgeAutopay
	}
	if cfg.GorgeUnpriced {
		s.cfg |= cfgGorgeUnpriced
	}
	if cfg.GorgeLowering {
		s.cfg |= cfgGorgeLowering
	}
	if cfg.Commander {
		s.cfg |= cfgCommander
	}
	s.divA = NoSlot
	s.mullMax = cfg.Mulligans
	s.win = -1
	s.rng = newRNG(seed)
	s.rng.rec = cfg.RecordChance
	s.g.nseat = n
	ns := int(s.nSeats())
	for p := 0; p < ns; p++ {
		if len(decks[p]) > MaxDeck {
			panic("kernel: deck larger than MaxDeck")
		}
		s.pl[p].life = int16(s.startingLife())
		s.pl[p].cmd = NoObj
	}
	for p := 0; p < ns; p++ {
		for _, c := range decks[p] {
			if int(s.nobj) >= MaxObj {
				panic("kernel: decks larger than MaxObj") // a pod of full Commander decks, past the four-seat build's objects
			}
			o := Obj(s.nobj)
			s.nobj++
			s.objs[o] = object{card: c, chosen: 255, zone: kop.ZoneLibrary, slot: NoSlot, acts: uint8(p) << objOwnerShift}
			s.touch(o)
			s.pl[p].libBottom(o)
		}
	}
	s.initSideboards(&cfg)
	if s.g.on {
		if why := s.k().gen.initStall(s.nSeats()); why != StallNone {
			s.stall(why) // loud; a built matchup refuses first (stallclass.go)
		}
	}
	toss := uint8(s.rng.IntN(ns))
	for p := uint8(0); p < uint8(ns); p++ {
		if s.cfg&cfgCommander != 0 {
			s.cmdGenesis(p, cfg.Commanders[p])
		}
		s.shuffleLib(p)
		for j := 0; j < openingHand; j++ {
			s.draw(p)
		}
		if s.over {
			return
		}
	}
	s.startPl = toss
	// the active player is gorge's zero value until the first turn begins
	// (the mulligan round and the opening-hand Leyline asks come first)
	s.active = 0
	if s.mullMax > 0 {
		s.mullOrder = mullOrderFrom(toss, uint8(ns))
		s.mullCur = 0
		s.ctl = ctlMullDeclare
		return
	}
	s.beginTurn(toss)
}

// Terminal reports whether the game is over and the winner (-1 = draw).
func (s *State) Terminal() (bool, int8) { return s.over, s.win }

// Turn is the current turn number (1-based).
func (s *State) Turn() int { return int(s.turn) }

// Life returns player p's life total.
func (s *State) Life(p int) int { return int(s.pl[p].life) }

// Step runs the game forward to the next decision and returns it (Kind ==
// DkNone when the game is over).
func (s *State) Step() Decision {
	var d Decision
	s.StepInto(&d)
	return d
}

// StepInto is Step writing into d (no Decision copy). With elision on
// (the default), a decision with a single option is answered in place and
// counted in Skipped instead of being returned.
func (s *State) StepInto(d *Decision) {
	for {
		s.stepRaw(d)
		if s.over || s.g.stalled != 0 {
			// a capacity stall while the options were built: nothing to answer.
			// A generated table latches the stall (capFull) rather than ending
			// the game inside the build, so options built past it are not posed.
			s.stallLatched()
			d.Kind, d.N = DkNone, 0
			return
		}
		if d.N > 1 {
			s.sortCanonical(d)
		}
		if d.N != 1 || s.cfg&cfgNoElide != 0 {
			if d.N == 0 && d.Kind != DkNone {
				// a caller would answer it with a stale option
				panic("kernel: a decision with no options")
			}
			return
		}
		s.Skipped[d.Player]++
		s.Apply(d.Options[0])
	}
}

// StepOne is one round of StepInto's loop: it runs the game forward to the
// next decision and returns it even when elision would answer it in place.
// A caller that answers every decision Elides reports with Elide and steps
// on plays exactly the game StepInto plays (TestStepOneIsStepInto), so an
// observer of every decision -- which must see the stack and
// the forced targets between two posed decisions -- reads the elided ones
// without changing anything. StepInto itself (the hot loop's) is unchanged.
func (s *State) StepOne(d *Decision) {
	s.stepRaw(d)
	if s.over || s.g.stalled != 0 {
		s.stallLatched()
		d.Kind, d.N = DkNone, 0
		return
	}
	if d.N > 1 {
		s.sortCanonical(d)
	}
	if d.N == 0 && d.Kind != DkNone {
		panic("kernel: a decision with no options")
	}
}

// Elides reports whether StepInto would answer d, StepOne's decision, in
// place (a single option, elision on).
func (s *State) Elides(d *Decision) bool { return d.N == 1 && s.cfg&cfgNoElide == 0 }

// Elide answers d as StepInto's elision does: its lone option, counted in
// Skipped.
func (s *State) Elide(d *Decision) {
	s.Skipped[d.Player]++
	s.Apply(d.Options[0])
}

func (s *State) stepRaw(d *Decision) {
	k := s.k()
	// CR 310.11: gorge's startDefeatedCast drains the defeat queue at the top
	// of every step(), before the step's own action. A battle defeated during
	// a resolution in THIS step therefore gets its offer at the top of the
	// next step, after the priority round that follows the defeat, not in
	// place of that priority.
	if k.gen != nil && k.gen.anyBattle && s.g.nmay > 0 && s.ctl != ctlBuild {
		s.wakeDefeat()
		d.N = 0
		if s.defeatAsk(k, d) {
			return
		}
	}
	for {
		d.N = 0
		d.Sub = ChNone
		if s.over {
			d.Kind = DkNone
			return
		}
		if k.gen != nil && s.stepGen(k, d) {
			return
		}
		switch s.ctl {
		case ctlMullDeclare:
			p := s.mullOrder[s.mullCur]
			d.Kind, d.Player = DkMulligan, p
			d.add(MakeAction(AkKeep, 0, 0, 0))
			if s.pl[p].mulls < s.mullMax {
				d.add(MakeAction(AkMulligan, 0, 0, 0))
			}
			return
		case ctlMullBottom:
			p := s.mullOrder[s.mullCur]
			d.Kind, d.Sub, d.Player = DkChoose, ChBottom, p
			pl := &s.pl[p]
			for i := 0; i < int(pl.handN); i++ {
				d.add(MakeAction(AkChoose, TCode(pl.hand[i]), 0, 0))
			}
			return
		case ctlStepBegin:
			s.beginStep(k)
			// gorge's step head runs startDefeatedCast before this step's
			// priority; wake the records queued by beginStep's SBA pass. A
			// combat damage step is the exception: its damage SBA queues the
			// defeat after the step's actions, and gorge asks at the next
			// step's head, after this step's priority round.
			if k.gen != nil && k.gen.anyBattle && s.g.nmay > 0 && s.step != kop.StepDamage && s.step != kop.StepFirstStrikeDamage {
				s.wakeDefeat()
			}
		case ctlPriority:
			// A resolution's own SBA tail can queue a defeat with ctl still
			// ctlPriority; gorge's next step() asks at its head, before this
			// step's priority. Deferred records (combat damage) stay for the
			// next stepRaw's head.
			if k.gen != nil && k.gen.anyBattle && s.g.nmay > 0 && s.defeatAsk(k, d) {
				return
			}
			if k.gen != nil && k.gen.anySuspend && s.g.nmay > 0 && s.suspendAsk(k, d) {
				return // CR 702.62a: the may-cast ask of a card whose last counter went (kw_suspend.go)
			}
			// state-based actions and triggers repeat until neither has anything
			// left, and only then does a player receive priority (CR 117.5): a
			// final chapter removed for want of a legal target (CR 603.3d) leaves
			// its Saga to be sacrificed (CR 714.4)
			if s.sba(k) {
				continue
			}
			// the pass queued a returning Aura's host ask (queueAskRet):
			// gorge asks it inside the pass, before the triggers and this
			// priority (stepGen poses it)
			if k.gen != nil && s.g.nask > 0 && s.g.ask[0].sl == NoSlot {
				continue
			}
			// The SBA pass just exiled a battle and queued it: gorge's next
			// step() head asks before this step's priority (a spell's SBA
			// tail, not a combat/step-action SBA, which stays deferred).
			if k.gen != nil && k.gen.anyBattle && s.g.nmay > 0 && s.defeatAsk(k, d) {
				return
			}
			if s.npend > 0 {
				s.ctl = ctlPlaceTriggers
				continue
			}
			if s.out(s.prio) {
				s.prio = s.nextAlive(s.prio) // gorge grantPriority: a departed holder's turn passes on
			}
			d.Kind, d.Player = DkPriority, s.prio
			s.priorityOptions(k, d)
			return
		case ctlPlaceTriggers:
			if s.placeTriggers(k, d) {
				return
			}
		case ctlBuild:
			if s.buildStep(k, d) {
				return
			}
		case ctlAttackers:
			if s.attackerOptions(k, d) {
				return
			}
		case ctlBlockers:
			if s.blockerOptions(k, d) {
				if d.Kind == DkBlockers && blkTaxOn(k) {
					s.blkTaxFilter(k, d) // combat_blocktax.go
				}
				return
			}
		case ctlResolve:
			if k.gen != nil && k.gen.ctrlSBA {
				// gorge settles control after every event: a Destroy that
				// ended an Aura's steal is seen by the ask the resolution
				// poses next, not at the state-based check after it
				s.ctrlSBA(k)
			}
			if s.resolveOptions(k, d) {
				return
			}
		case ctlLegend:
			s.legendOptions(k, d)
			return
		case ctlExileOrder:
			s.exileOrderOptions(k, d)
			return
		case ctlTurnUpX:
			s.turnUpXOptions(d) // face_turnfaceup.go
			return
		case ctlCmdZone0, ctlCmdZone1, ctlCmdZone2, ctlCmdZone3, ctlCmdZone4, ctlCmdZone5, ctlCmdZone6, ctlCmdZone7:
			s.askCmdZone(k, d, s.ctl-ctlCmdZone0) // CR 903.9a (zone_cmdzone.go)
			return
		case ctlDivide:
			if s.divA != NoSlot {
				s.divideOptions(k, d)
				return
			}
			s.damageStep(k, s.divFirst, true) // after an answer: a division or a replacement order
		case ctlCleanupDiscard:
			p := s.active
			d.Kind, d.Sub, d.Player = DkChoose, ChCleanupDiscard, p
			pl := &s.pl[p]
			for i := 0; i < int(pl.handN); i++ {
				if len(s.cold.parked) == 0 || !s.parked(pl.hand[i]) {
					d.add(MakeAction(AkChoose, TCode(pl.hand[i]), 0, 0))
				}
			}
			return
		default:
			panic("kernel: bad control state")
		}
	}
}

// Apply answers the last Decision with a, which must be one of its Options.
func (s *State) Apply(a Action) {
	k := s.k()
	s.Decisions++
	switch s.ctl {
	case ctlMullDeclare:
		s.applyMulligan(a)
	case ctlMullBottom:
		p := s.mullOrder[s.mullCur]
		o := Obj(a.Source())
		s.detach(o)
		s.objs[o].ver++
		s.objs[o].zone = kop.ZoneLibrary
		s.pl[p].libBottom(o)
		s.mullLeft--
		if s.mullLeft == 0 {
			s.nextBottom()
		}
	case ctlPriority:
		s.applyPriority(k, a)
	case ctlPlaceTriggers:
		s.applyTriggerOrder(k, a)
	case ctlBuild:
		s.applyBuild(k, a)
	case ctlAttackers:
		s.applyAttack(k, a)
	case ctlBlockers:
		s.applyBlock(k, a)
	case ctlResolve:
		s.applyResolve(k, a)
	case ctlLegend:
		s.applyLegend(k, a)
	case ctlExileOrder:
		s.applyExileOrder(k, a)
	case ctlTurnUpX:
		s.applyTurnUpX(k, a) // face_turnfaceup.go
	case ctlCmdZone0, ctlCmdZone1, ctlCmdZone2, ctlCmdZone3, ctlCmdZone4, ctlCmdZone5, ctlCmdZone6, ctlCmdZone7:
		s.applyCmdZone(k, s.ctl-ctlCmdZone0, a.Mode()) // CR 903.9a (zone_cmdzone.go)
	case ctlDivide:
		s.applyDivide(k, a)
	case ctlCleanupDiscard:
		o := Obj(a.Source())
		s.discardLeft--
		s.discardPicks(k, []Obj{o}, parkCleanup)
		if s.discardLeft == 0 {
			// gorge takes all the picks at once and orders the replacements after
			if s.parkedWaiting() {
				s.ctl = ctlExileOrder
			} else {
				s.finishCleanup(k)
			}
		}
	case ctlGenAsk:
		s.applyGen(k, a)
	default:
		panic("kernel: Apply with no pending decision")
	}
}

// --- mulligan (gorge engine/rules/mulligan.go) ---

func (s *State) applyMulligan(a Action) {
	p := s.mullOrder[s.mullCur]
	if a.Kind() == AkKeep {
		s.mullKept[p] = true
	} else {
		s.mullMull[p] = true
		s.pl[p].mulls++
	}
	s.nextMullDeclarer()
}

// nextMullDeclarer moves to the next un-kept seat of this pass; at the end
// of a pass every seat that mulliganed redraws (hand to the library end in
// hand order, shuffle, draw 7), then the next pass or the bottoming starts.
func (s *State) nextMullDeclarer() {
	for {
		s.mullCur++
		if s.mullCur >= s.nSeats() {
			break
		}
		if !s.mullKept[s.mullOrder[s.mullCur]] {
			return
		}
	}
	for i := 0; i < int(s.nSeats()); i++ {
		p := s.mullOrder[i]
		if !s.mullMull[p] {
			continue
		}
		s.mullMull[p] = false
		pl := &s.pl[p]
		for pl.handN > 0 {
			o := pl.hand[0]
			s.detach(o)
			s.objs[o].of &^= ofInHand
			s.objs[o].ver++
			s.objs[o].zone = kop.ZoneLibrary
			pl.libBottom(o)
		}
		s.shuffleLib(p)
		for j := 0; j < openingHand; j++ {
			s.draw(p)
		}
		if s.over {
			return
		}
	}
	for i := 0; i < int(s.nSeats()); i++ {
		if !s.mullKept[s.mullOrder[i]] {
			s.mullCur = uint8(i)
			s.ctl = ctlMullDeclare
			return
		}
	}
	s.mullCur = 255
	s.nextBottom()
}

// nextBottom poses each seat's bottoming (one pick per card owed).
func (s *State) nextBottom() {
	for {
		s.mullCur++
		if s.mullCur >= s.nSeats() {
			s.beginTurn(s.startPl)
			return
		}
		p := s.mullOrder[s.mullCur]
		n := s.pl[p].mulls
		if MaxSeats > 2 && s.nSeats() > 2 && n > 0 {
			n-- // CR 103.5c: a multiplayer game's first mulligan is free (gorge freeMulligans)
		}
		if n > s.pl[p].handN {
			n = s.pl[p].handN
		}
		if n > 0 {
			s.mullLeft = n
			s.ctl = ctlMullBottom
			return
		}
	}
}
