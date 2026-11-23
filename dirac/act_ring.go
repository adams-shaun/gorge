package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// The Ring (CR 701.54; gorge engine/effects/ring.go effRingTemptsYou,
// events/apply_misc.go foldRingTemptsYou, rules/trigger_match.go
// checkRingEmblemTriggers and rules/combat/restrictions.go
// RingBearerOutpowers). Cold and out of line: a table with no OpExt
// ExtRingTemptsYou keeps no array (genPrep.uses, useRing) and nobody is ever
// tempted.
//
// State: coldRing, one record per seat (seat-generic; no seat-count branch):
// lvl the temptation count (gorge state.Player.RingTempted, the emblem's
// level gates read it uncapped) and bearer the seat's Ring-bearer (gorge
// state.Player.RingBearer; NoObj when none, gorge Obj 0). The Ring emblem
// itself is not a record: gorge registers its layer-4 Legendary static once,
// at the first temptation, and never removes it (engine/effects/ring.go
// ringEmblemLegendary), so "the player has the emblem" is lvl > 0.
//
//   - 701.54a: ExtRingTemptsYou tempts its player (gorge: always the
//     resolving controller; no corpus line carries Defined$). The count rises
//     by one and a creature the player controls becomes (or stays) their
//     Ring-bearer. With no creature controlled the event still counts and the
//     designation clears (CR 701.54d, gorge Obj 0); with exactly one the
//     strict-supersets rule designates it silently; with more the player is
//     asked (raRingBearer), the deterministic default offered FIRST exactly as
//     gorge's KChoose does (engine/effects/ring.go: the existing Ring-bearer
//     when still controlled, else the first eligible creature in zone order --
//     ordering the options any other way would silently switch a bot's bearer
//     off the deterministic default). A stale or stray answer falls back to
//     the default (gorge ringAnsweredBearer). Every path then queues the
//     "Whenever the Ring tempts you" listeners (TrigRingTemptsYou, gorge
//     ringTemptsMatches: the event's player the tempted seat, its object the
//     new bearer -- with no bearer a ValidCard$ trigger never fires).
//   - 701.54c: the emblem's first ability, "Your Ring-bearer is legendary and
//     can't be blocked by creatures with greater power", is read live:
//     typesGen adds Legendary to the bearer of a seat with lvl > 0
//     (chars_gen.go), and canBlock forbids a blocker whose power is greater
//     (ringOutpowers). The emblem's triggered levels are queued after the
//     event's card triggers (gorge checkRingEmblemTriggers): level 2 loots
//     when the bearer attacks, level 3 registers an end-of-combat sacrifice
//     promise when the bearer becomes blocked, and level 4 drains each
//     opponent when the bearer deals combat damage to a player. They are the
//     section below.
//
// The Legendary add sits at the TOP of layer 4 (after every table override),
// not at the emblem's own registration timestamp: the emblem is engine-side
// and carries no table timestamp, and a later SetTypes override on the bearer
// would remove the Legendary in gorge's timestamp order while it stays here.
// No corpus line sets the bearer's types after the first temptation today;
// if one appears, this is the corner to model.

// ChGenRingBearer is the ring-bearer pick (gorge effRingTemptsYou's
// "ring_bearer" KChoose): source = the chosen creature.
const ChGenRingBearer ChoiceKind = 204

// RingStats counts the mechanic's events; oracle tests only.
type RingStats struct {
	Tempts       int // temptations folded (the count rose)
	Designated   int // of those, ones whose bearer is a creature
	Cleared      int // of those, ones with no creature controlled (bearer NoObj)
	Asks         int // ring-bearer asks posed
	Fires        int // TrigRingTemptsYou triggers queued
	Loot         int // level-2 loot triggers queued
	Block        int // level-3 block triggers queued
	Drain        int // level-4 drain triggers queued
	Promises     int // level-3 end-of-combat promises registered
	PromiseFires int // level-3 promises put on the stack at end of combat
}

// DebugRingStats is the test hook; nil in normal play.
var DebugRingStats *RingStats

var coldRing = newCold[ringSeat]("ring", func(g *genPrep) int {
	if g.ringOn() {
		return MaxSeats
	}
	return 0
})

// ringSeat is one seat's ring record (act_ring.go's state). ver and ctrl
// stamp the designated incarnation and its controller at designation time:
// a zone re-entry (CR 400.7) or a control change ends the designation, and
// ctrlMove clears the record on the latter so a later control round trip
// cannot revive it.
type ringSeat struct {
	lvl    uint8
	bearer Obj
	ver    uint8
	ctrl   uint8
}

// ringCold is the per-seat ring records, or nil when the table never tempts.
func (s *State) ringCold() []ringSeat { return coldRing.of(s) }

// ringPromiseCold is the level-3 promise slots, or nil when the table never
// tempts.
func (s *State) ringPromiseCold() []ringPromise { return coldRingPromise.of(s) }

// Ring is seat p's temptation count, 0 when the table never tempts (oracle).
func (s *State) Ring(p uint8) int {
	if c := s.ringCold(); c != nil && p < MaxSeats {
		return int(c[SeatIx(p)].lvl)
	}
	return 0
}

// RingBearer is seat p's live Ring-bearer, NoObj when none or the table never
// tempts (oracle). It reads through ringBearerAt, so a departed incarnation or
// a control change clears the designation here too.
func (s *State) RingBearer(p uint8) Obj {
	if p >= MaxSeats {
		return NoObj
	}
	return s.ringBearerAt(p)
}

// useRing is genPrep.uses' bit for a table where some program tempts
// (ExtRingTemptsYou): the cold buffer holds each seat's count and bearer.
var useRing = newGenUse(func(t *kop.Table) bool {
	for i := range t.Cards {
		for j := range t.Cards[i].Abilities {
			for _, in := range t.Cards[i].Abilities[j].Prog {
				if in.Op == kop.OpExt && in.Ext() == kop.ExtRingTemptsYou {
					return true
				}
			}
		}
	}
	return false
})

// ringOn reports whether the table tempts (act_ring.go's state is laid out).
func (g *genPrep) ringOn() bool { return g != nil && g.uses.has(useRing) }

// ringEligible writes the battlefield creatures player p controls, in entry
// order (gorge effRingTemptsYou's eligible walk over Zone(ZBattlefield, p)).
func (s *State) ringEligible(k *prep, p uint8, buf *[MaxPerm]Slot) int {
	// Entry order, not slot order: slots are lowest-free and reused, so a
	// re-entered permanent can sit below an older one (gorge reads
	// Zone(ZBattlefield, p) in entry order).
	n := s.battlefieldOrder(p, s.ctrlM[p].and(s.occ), buf[:], 0)
	m := 0
	for i := 0; i < n; i++ {
		if s.isCreature(k, buf[i]) {
			buf[m] = buf[i]
			m++
		}
	}
	return m
}

// ringBearerAt is seat p's live designation: the bearer ringFold recorded
// while it is still the same incarnation on the battlefield under the seat
// that designated it. Gorge clears the designation on every battlefield
// leave (events/apply_zone.go clearRingBearers) and every control change
// (events/apply_player.go foldControlChange); the kernel stamps the
// incarnation and the designation-time controller here and ctrlMove drops a
// moved bearer's record, so neither a zone re-entry nor a control round trip
// revives it.
func (s *State) ringBearerAt(p uint8) Obj {
	c := s.ringCold()
	if c == nil {
		return NoObj
	}
	r := &c[SeatIx(p)]
	if r.lvl == 0 || r.bearer == NoObj {
		return NoObj
	}
	o := &s.objs[r.bearer]
	if o.zone != kop.ZoneBattlefield || o.ver != r.ver || s.perms[o.slot].ctrl != r.ctrl {
		return NoObj
	}
	return r.bearer
}

// ringControlChanged drops every seat's designation naming o when another
// player gains control of it (CR 701.54b; gorge foldControlChange). The new
// controller keeps its own designation, if any (gorge's PlayerID(i) !=
// e.Player).
func (s *State) ringControlChanged(o Obj, to uint8) {
	c := s.ringCold()
	if c == nil {
		return
	}
	for i := range c {
		if c[i].bearer == o && uint8(i) != to {
			c[i].bearer, c[i].ver, c[i].ctrl = NoObj, 0, 0
		}
	}
}

// ringDefault is the deterministic bearer choice (gorge ringDefaultBearer):
// the existing Ring-bearer when player p still controls it on the battlefield
// (the designation persists until another creature becomes the bearer or
// control changes, even when it has stopped being a creature), else the first
// eligible creature in zone order, else NoObj.
func (s *State) ringDefault(k *prep, p uint8, elig []Slot) Obj {
	if cur := s.ringBearerAt(p); cur != NoObj {
		return cur
	}
	if len(elig) > 0 {
		return s.perms[elig[0]].obj
	}
	return NoObj
}

// ringFold is gorge foldRingTemptsYou: the count rises by one and bearer
// becomes (or clears) the designation, then the temptation's listeners queue.
//
//go:noinline
func (s *State) ringFold(k *prep, p uint8, bearer Obj) {
	r := &s.ringCold()[SeatIx(p)]
	r.lvl++
	r.bearer = bearer
	if bearer == NoObj {
		r.ver, r.ctrl = 0, 0
	} else {
		r.ver, r.ctrl = s.objs[bearer].ver, p
	}
	if st := DebugRingStats; st != nil {
		st.Tempts++
		if bearer != NoObj {
			st.Designated++
		} else {
			st.Cleared++
		}
	}
	s.fireRingTempt(k, p, bearer)
}

// fireRingTempt queues the "Whenever the Ring tempts you" listeners for the
// temptation of seat p whose event names bearer (gorge ringTemptsMatches on
// events.RingTemptsYou): battlefield listeners in gorge's walk order, then
// graveyard cards (TriggerZones$ Graveyard, Ringwraiths). No slot
// subscription.
//
//go:noinline
func (s *State) fireRingTempt(k *prep, p uint8, bearer Obj) {
	st := DebugRingStats
	var ord [MaxPerm]Slot
	n := s.listenerOrder(s.occ, &ord)
	for _, sl := range ord[:n] {
		s.slotTriggers(k, sl, func(a *kop.Ability) bool { return a.Trig.Event == kop.TrigRingTemptsYou }, func(l trigListener, a *kop.Ability) {
			b := s.npend
			s.playerTrigGenObj(k, kop.TrigRingTemptsYou, l, a, p, 0, bearer)
			if st != nil && s.npend > b {
				st.Fires++
			}
		})
	}
	for _, c := range k.gen.graveTrig {
		cd := &k.t.Cards[c]
		for pp := uint8(0); pp < s.nSeats(); pp++ {
			if s.out(pp) {
				continue
			}
			pl := &s.pl[pp]
			for i := 0; i < int(pl.graveN); i++ {
				o := pl.grave[i]
				if s.objs[o].card != c {
					continue
				}
				for j := range cd.Abilities {
					a := &cd.Abilities[j]
					if a.Kind != kop.AbTriggered || a.Trig.Zone != kop.ZoneGraveyard || a.Trig.Event != kop.TrigRingTemptsYou {
						continue
					}
					l := trigListener{obj: o, sl: NoSlot, ctrl: pp, card: c, ab: uint8(j)}
					b := s.npend
					s.playerTrigGenObj(k, kop.TrigRingTemptsYou, l, a, p, 0, bearer)
					if st != nil && s.npend > b {
						st.Fires++
					}
				}
			}
		}
	}
}

// ringTemptsExt is ExtRingTemptsYou: the player Obj is tempted (CR 701.54a;
// gorge effRingTemptsYou, always the resolving controller).
//
//kmanifest:ext ExtRingTemptsYou
func (s *State) ringTemptsExt(k *prep, in *kop.Instr) (ok, done bool) {
	p := s.playerRef(k, in.Obj)
	if p == 255 {
		return true, true
	}
	var buf [MaxPerm]Slot
	switch n := s.ringEligible(k, p, &buf); n {
	case 0:
		// CR 701.54d: no creature controlled -- the temptation still counts
		// and the trigger still fires; the designation clears.
		s.ringFold(k, p, NoObj)
	case 1:
		// Strict-supersets: one eligible creature is no choice (gorge
		// effRingTemptsYou). Designate it silently -- no ask.
		s.ringFold(k, p, s.perms[buf[0]].obj)
	default:
		s.res.ask, s.res.askP = raRingBearer, p
		if st := DebugRingStats; st != nil {
			st.Asks++
		}
		return false, true // the ask is posed; its answer folds
	}
	return true, true
}

// ringAskOptions poses the ring-bearer ask (gorge effRingTemptsYou's
// decision): the deterministic default offered FIRST when it is one of the
// eligible creatures, then the rest in zone order.
//
//kmanifest:askopts raRingBearer
func (s *State) ringAskOptions(k *prep, d *Decision) {
	p := s.res.askP
	d.Sub = ChGenRingBearer
	var buf [MaxPerm]Slot
	n := s.ringEligible(k, p, &buf)
	def := s.ringDefault(k, p, buf[:n])
	// The deterministic default is offered FIRST when it is one of the
	// eligible creatures (gorge effRingTemptsYou); a default outside the
	// eligible list (a still-controlled bearer that stopped being a
	// creature) is offered nowhere.
	for i := 0; i < n; i++ {
		if s.perms[buf[i]].obj == def {
			d.add(MakeAction(AkChoose, TCode(def), 0, 0))
			break
		}
	}
	for i := 0; i < n; i++ {
		if o := s.perms[buf[i]].obj; o != def {
			d.add(MakeAction(AkChoose, TCode(o), 0, 0))
		}
	}
}

// ringAskAnswer applies the ring-bearer ask's answer (gorge
// ringAnsweredBearer): the pick when it is still a creature p controls on the
// battlefield, else the deterministic default; then the fold.
//
//kmanifest:askapply raRingBearer
func (s *State) ringAskAnswer(k *prep, a Action) {
	p := s.res.askP
	s.res.ask = raNone
	var buf [MaxPerm]Slot
	n := s.ringEligible(k, p, &buf)
	pick := NoObj
	if a.Kind() == AkChoose {
		if o := Obj(a.Source()); o != NoObj && s.objs[o].zone == kop.ZoneBattlefield {
			sl := s.objs[o].slot
			if s.perms[sl].ctrl == p && s.isCreature(k, sl) {
				pick = o
			}
		}
	}
	if pick == NoObj {
		pick = s.ringDefault(k, p, buf[:n])
	}
	s.ringFold(k, p, pick)
	s.continueRes(k, 0)
}

// ringLegend reports whether the bearer of the Ring emblem at sl's
// controller's seat is sl (gorge ringEmblemLegendary's Affects
// Card.IsRingbearer+YouCtrl read live): true only while that seat has the
// emblem (lvl > 0, CR 701.54e) and still controls its designated creature.
func (s *State) ringLegend(k *prep, sl Slot) bool {
	p := s.perms[sl].ctrl
	return s.ringBearerAt(p) == s.perms[sl].obj
}

// ringTySlots is the set of battlefield slots whose cached types the Ring
// emblem's Legendary static touches (refreshTy folds it into perm.gty for
// the projection): each seat's designated bearer. A slot that stops being a
// bearer leaves the set and refreshTy's tyM sweep clears its ovGenTy, the
// printed types reading again.
//
// The legend-rule scan (turn.go legendCheck) candidates from the printed
// legM mask, so a Ring-bearer that is not printed legendary is not a
// candidate there: a same-named printed legend beside it is not compared.
// No corpus game reaches that corner; the emblem's continuous add itself
// rides gty, so every typesAt reader (value_turntallies.go's legendary
// combat hit included) sees it.
//
//go:noinline
func (s *State) ringTySlots(k *prep) slotMask {
	var m slotMask
	for p := uint8(0); p < s.nSeats(); p++ {
		if b := s.ringBearerAt(p); b != NoObj {
			m = m.with(s.objs[b].slot)
		}
	}
	return m
}

// ringOutpowers is gorge RingBearerOutpowers (CR 701.54c): blocker b cannot
// block attacker a when a is its controller's Ring-bearer and b's power is
// greater than a's.
func (s *State) ringOutpowers(k *prep, b, a Slot) bool {
	if !s.ringLegend(k, a) {
		return false
	}
	return s.power(k, b) > s.power(k, a)
}

// --- the emblem's triggered levels (CR 701.54c; gorge
// rules/trigger_match.go checkRingEmblemTriggers, queueRingEmblem,
// events/apply_helpers.go ringEmblemAbility, engine/effects/ring.go
// effRingBlockerSacrifice) ---
//
// Level N is active iff the seat's count >= N; each level names one event:
//
//	2  the bearer among the declared attackers (DeclareAttackers): the
//	   controller draws a card, then discards a card (the discard their
//	   own choice, TgtChoose; gorge's Draw You 1 with the Discard Sub).
//	3  one trigger per (Ring-bearer, blocker) pair in declaration order
//	   (DeclareBlockers, gorge's event Pairs): at resolution an
//	   end-of-combat promise to sacrifice the blocker is registered (CR
//	   603.7: created on resolution, so a countered trigger sacrifices
//	   nothing). The blocker rides the pending entry's trigObj with its
//	   queue-time version in srcVer, gorge's queue-time incarnation stamp:
//	   a blocker that left the battlefield and returned before the trigger
//	   resolves is a new object the promise does not name (CR 400.7), and
//	   resolveRingTrig re-checks both at registration.
//	4  a combat-damage event whose source is the bearer and whose recipient
//	   is a player (a hit onto a permanent is not damage to a player): each
//	   opponent of the bearer's controller loses 3 life.
//
// The enqueues run after the event's card triggers (gorge queues the
// emblem's entries in the synthetic scan after the per-face walk), per seat
// in seat order. The end-of-combat promises fire at the end-of-combat step
// after the card delayed programs and the token promises (turn.go); gorge
// drains one registration-ordered list, so a promise interleaved between
// card registrations would fire at its registration place there -- a corner
// no corpus game reaches with both a card end-of-combat promise and a ring
// one live.

// The abDelayed entry's trigAmt markers (the venture's -2, the monarch's
// -3/-4, the rad drain's -5).
const (
	ringLootAmt  int16 = -6 // level 2: draw a card, then discard a card
	ringBlockAmt int16 = -7 // level 3: register the blocker promise (trigObj the blocker, srcVer its stamp)
	ringDrainAmt int16 = -8 // level 4: each opponent loses 3 life
)

// StallRingPromise: more than MaxDelayed live level-3 promises.
const StallRingPromise = 177

func init() { stallNames[StallRingPromise] = "ringpromise" }

// ringPromise is one registered level-3 promise: sacrifice the blocker when
// it is still the object that was blocked (ver its stamp, CR 400.7). ctrl is
// the delayed trigger's controller, the emblem owner (CR 603.7d; gorge
// effRingBlockerSacrifice schedules with c.Controller); the blocker's own
// controller performs the sacrifice when it resolves.
type ringPromise struct {
	obj  Obj
	ver  uint8
	ctrl uint8
}

var coldRingPromise = newCold[ringPromise]("ringpromise", func(g *genPrep) int {
	if g.ringOn() {
		return MaxDelayed
	}
	return 0
})

// coldRingPromiseN is the live promise count, one byte in a cold region with
// no coldArrays view: no stateCore or genState field carries it.
var coldRingPromiseN = newCold[uint8]("ringpromisen", func(g *genPrep) int {
	if g.ringOn() {
		return 1
	}
	return 0
})

// ringPromiseN is the count of live level-3 promises, cleared when the
// end-of-combat step fires them; nil when the table never tempts.
func (s *State) ringPromiseN() *uint8 {
	c := coldRingPromiseN.of(s)
	if c == nil {
		return nil
	}
	return &c[0]
}

// DebugRingNoLoot (the mutation test): level 2 queues no trigger.
var DebugRingNoLoot bool

// DebugRingNoPromise (the mutation test): level 3 resolves to nothing.
var DebugRingNoPromise bool

// DebugRingNoDrain (the mutation test): level 4 queues no trigger.
var DebugRingNoDrain bool

// ringTrig reports whether e is one of the emblem's source-less triggers.
func ringTrig(e *stackEntry) bool {
	return e.obj == NoObj && e.trigAmt >= ringDrainAmt && e.trigAmt <= ringLootAmt
}

// ringAtkTrigs queues level 2 for each seat whose Ring-bearer is among the
// declared attackers atk (in seat order, gorge's player loop over
// DeclareAttackers).
//
//go:noinline
func (s *State) ringAtkTrigs(k *prep, atk []Slot) {
	if DebugRingNoLoot {
		return
	}
	st := DebugRingStats
	for p := uint8(0); p < s.nSeats(); p++ {
		if s.out(p) {
			continue
		}
		r := &s.ringCold()[SeatIx(p)]
		if r.lvl < 2 {
			continue
		}
		b := s.ringBearerAt(p)
		if b == NoObj {
			continue
		}
		for _, sl := range atk {
			if s.perms[sl].obj == b {
				s.enqueue(NoObj, 0, 0, abDelayed, p, NoObj, p, ringLootAmt)
				if st != nil {
					st.Loot++
				}
				break
			}
		}
	}
}

// ringBlockTrigs queues level 3 once per (Ring-bearer, blocker) pair in
// declaration order (gorge's DeclareBlockers Pairs).
//
//go:noinline
func (s *State) ringBlockTrigs(k *prep) {
	st := DebugRingStats
	var ps [2 * MaxPerm]blkPair
	nb := s.blockPairs(&ps)
	if nb == 0 {
		return
	}
	for p := uint8(0); p < s.nSeats(); p++ {
		if s.out(p) {
			continue
		}
		r := &s.ringCold()[SeatIx(p)]
		if r.lvl < 3 {
			continue
		}
		bearer := s.ringBearerAt(p)
		if bearer == NoObj {
			continue
		}
		for _, pr := range ps[:nb] {
			if s.perms[pr.a].obj != bearer || pr.b == NoSlot {
				continue
			}
			b := s.perms[pr.b].obj
			// The queue-time incarnation stamp (gorge queueRingEmblem's
			// TriggerRememberedStamps): rides srcVer.
			s.enqueue(NoObj, s.objs[b].ver, 0, abDelayed, p, b, s.perms[pr.b].ctrl, ringBlockAmt)
			if st != nil {
				st.Block++
			}
		}
	}
}

// ringCombatHit queues level 4 once per damage event r deals from the
// bearer to a player, in assignment order (gorge's Damage events, a hit
// onto a permanent excluded).
//
//go:noinline
func (s *State) ringCombatHit(k *prep, r *dmgRec) {
	if DebugRingNoDrain {
		return
	}
	st := DebugRingStats
	for i := 0; i < r.nev; i++ {
		ev := &r.ev[i]
		if ev.dst != NoSlot || ev.n <= 0 {
			continue
		}
		sl := ev.src
		if sl == NoSlot {
			continue
		}
		if !s.ringLegend(k, sl) {
			continue
		}
		r := &s.ringCold()[SeatIx(s.perms[sl].ctrl)]
		if r.lvl < 4 {
			continue
		}
		p := s.perms[sl].ctrl
		s.enqueue(NoObj, 0, 0, abDelayed, p, NoObj, p, ringDrainAmt)
		if st != nil {
			st.Drain++
		}
	}
}

// ringLootDiscard is the loot's discard half (gorge's Discard You TgtChoose
// 1): with no card in hand nothing; with exactly one the strict-supersets
// rule discards it silently; with more the player is asked (raRingLoot).
// It never pops: the caller owns the resolution's one finish (resolveTop's
// abDelayed arm for the emblem trigger, dredgeResTail for the dredge resume,
// ringAskLootAnswer for the ask).
//
//go:noinline
func (s *State) ringLootDiscard(k *prep, p uint8) {
	pl := &s.pl[p]
	switch {
	case pl.handN == 0:
		return
	case int(pl.handN) == 1 && s.objs[pl.hand[0]].of&ofCopy == 0:
		s.discardObj(k, pl.hand[0])
		return
	case int(pl.handN) == 1:
		return // the only card is a copy, not a discardable card
	default:
		s.res.ask, s.res.askP, s.res.left = raRingLoot, p, 1
		s.res.chosen = objWord(p)
		s.ctl = ctlResolve // the mid-resolution ask is posed by the engine
	}
}

// ringAskLootOptions poses the loot discard ask: the hand in hand order
// (gorge's TgtChoose over the hand).
//
//kmanifest:askopts raRingLoot
func (s *State) ringAskLootOptions(k *prep, d *Decision) {
	p := uint8(s.res.chosen)
	d.Sub = ChDiscard // gorge poses an effect discard as KModes "discard"
	pl := &s.pl[p]
	for i := 0; i < int(pl.handN); i++ {
		if o := pl.hand[i]; s.objs[o].of&ofCopy == 0 {
			d.add(MakeAction(AkChoose, TCode(o), 0, 0))
		}
	}
}

// ringAskLootAnswer applies the loot discard ask: the pick is discarded and
// the trigger's resolution finishes.
//
//kmanifest:askapply raRingLoot
func (s *State) ringAskLootAnswer(k *prep, a Action) {
	s.res.ask = raNone
	if a.Kind() == AkChoose {
		if o := Obj(a.Source()); o != NoObj && s.objs[o].zone == kop.ZoneHand {
			s.discardObj(k, o)
		}
	}
	s.finishResolve(k, false)
}

// ringPromiseReg registers the level-3 promise of the blocker the level-3
// trigger resolves for (gorge effRingBlockerSacrifice): the blocker must
// still be the object that was blocked (its version the queue-time stamp) on
// the battlefield; the promise is controlled by the trigger's controller (the
// emblem owner, CR 603.7d), not the blocker's controller.
//
//go:noinline
func (s *State) ringPromiseReg(k *prep, e *stackEntry) {
	if DebugRingNoPromise {
		return
	}
	b := e.trigObj
	if b == NoObj {
		return
	}
	o := &s.objs[b]
	if o.ver != e.srcVer || o.zone != kop.ZoneBattlefield {
		return // CR 400.7: the blocker left and returned, or is simply gone
	}
	a, n := s.ringPromiseCold(), s.ringPromiseN()
	if n == nil || *n >= uint8(len(a)) {
		s.capFull(StallRingPromise)
		return
	}
	a[*n] = ringPromise{obj: b, ver: e.srcVer, ctrl: e.ctrl}
	*n++
	if st := DebugRingStats; st != nil {
		st.Promises++
	}
}

// fireRingPromises puts the live level-3 promises on the stack at the
// end-of-combat step, in registration order, and clears them (the promise is
// this combat's: gorge's thisTurn rider). Every registered promise fires:
// CR 603.7 keeps a delayed trigger that names an object that has since left
// the battlefield — a blocker that combat damage killed before end of combat —
// on the stack, resolving to nothing (gorge's fire-time scan only requires the
// source to still resolve; checkDelayedTriggers' src == nil remove is the sole
// gate), so the CR 400.7 half is resolveDelayed's own version and zone
// re-check, with the blocker's controller's sacrifice (sacBlocked, the CR
// 616.1 park) and the ordinary sacrifice machinery.
//
//go:noinline
func (s *State) fireRingPromises(k *prep) {
	if k.gen == nil || !k.gen.ringOn() {
		return
	}
	a, cnt := s.ringPromiseCold(), s.ringPromiseN()
	if cnt == nil {
		return
	}
	n := int(*cnt)
	for i := 0; i < n; i++ {
		pr := a[i]
		s.enqueue(pr.obj, pr.ver, s.objs[pr.obj].card, abDelayed, pr.ctrl, pr.obj, pr.ctrl, int16(kop.OpSacrifice))
		if st := DebugRingStats; st != nil {
			st.PromiseFires++
		}
	}
	*cnt = 0
}

// resolveRingTrig resolves one of the emblem's source-less triggers.
//
//go:noinline
func (s *State) resolveRingTrig(k *prep, e *stackEntry) {
	switch e.trigAmt {
	case ringLootAmt:
		p := e.ctrl
		if s.pl[SeatIx(p)].lost {
			return
		}
		if !s.dredgeResPose(k, p, dredgeResRing, 1) {
			s.ctl = ctlResolve
			return // the dredge ask is posed; its answer's tail discards
		}
		s.draw(p)
		s.ringLootDiscard(k, p)
	case ringBlockAmt:
		s.ringPromiseReg(k, e)
	case ringDrainAmt:
		p := e.ctrl
		if s.pl[SeatIx(p)].lost {
			return
		}
		q := s.players(k, kop.RefEachOpponent)
		for i := uint8(0); i < q.n; i++ {
			s.loseLife(k, q.s[i], 3)
		}
	}
}

// ringCountOf is ExRingTempted (gorge count_body_player.go RingTemptedYou):
// seat p's temptation count, 0 when the table never tempts.
func (s *State) ringCountOf(p uint8) int {
	return s.Ring(p)
}
