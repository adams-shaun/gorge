package dirac

import (
	"math"
	"math/bits"
	"sync"
	"sync/atomic"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// Capacity constants. State is a fixed-size value; these bound it.
const (
	MaxDeck    = 100 // cards per seat's deck (a Commander deck's 100)
	MaxSide    = 15  // cards per seat's sideboard (a Config's; each one is an object, so MaxObj bounds the decks and sideboards together)
	MaxPerm    = 192 // battlefield slots, shared by the pod; the slot mask's three words hold up to 192. 112 before 2026-11-19, when random-policy four-seat Commander pods filled it in 8 of 256 cmd5 pods (40-card cuts) and reached 188 at 192
	MaxStack   = 80  // stack entries (24 before 2026-11-03, raised with MaxPending: the triggers a pending queue holds all go on the stack together, CR 603.3b)
	MaxPending = 80  // triggered abilities waiting to be put on the stack (24 before 2026-11-03: Return to the Light Realms bringing back a graveyard of creatures beside three enters listeners queues more)
	MaxHand    = MaxDeck
	MaxDelayed = 32 // 16 before 2026-11-03: each earthbent land keeps its leave-keyed return record while it lives
)

// ObjWords is the number of 64-bit words an object bitset takes (the change
// marks, objBits). MaxObj (seats.go) may be any value
// up to StackTarget0: an Obj is a uint8 whose StackTarget0.. codes are stack
// abilities, PlayerTarget0.. players and 0xFE/0xFF a departed stack target
// and NoTarget/NoObj in target bytes.
const ObjWords = (MaxObj + 63) / 64

// Compile-time bounds: a constant conversion that overflows fails the build.
const (
	_ = uint8(StackTarget0 - MaxObj) // MaxObj <= StackTarget0 (Obj StackTarget0.. are stack-ability / player targets / none)
	_ = uint8(255 - MaxPerm)         // MaxPerm <= 255 (Slot 255 is NoSlot)
)

// Config bits.
type Config struct {
	// NoElide turns pass-elision off: every priority decision is posed even
	// when passing is the only legal action (the oracle lines up with gorge
	// this way). With elision on, such a decision is skipped and counted in
	// State.Skipped.
	NoElide bool
	// Mulligans is the London mulligan round's limit (gorge Config.Mulligans):
	// 0 skips the round entirely.
	Mulligans uint8
	// AutoDivide assigns a non-trampler's combat damage among two or more
	// blockers automatically (lethal in block order, the rest to the last
	// blocker) instead of asking its controller as gorge does. Off by
	// default; a search may turn it on to cut branching.
	AutoDivide bool
	// RecordChance records every random draw for ChanceTranscript (the
	// oracle's forced-outcome replay).
	RecordChance bool
	// ArrangeRestOrder adds a second phase to a scry or surveil (gorge
	// KArrange) after the top picks close: the cards not kept on top are
	// ordered by the answerer (CR 404.3: cards put into a graveyard
	// together may be arranged in any order; CR 701.22a: the scry bottom
	// likewise), one ChArrange pick per card but the last, which is
	// implied. Off (the zero value) the rest goes to the graveyard / bottom
	// in library order, gorge's answer with no Rest. A host whose wire
	// carries the pile-B order (the SpellBench v2 mapping's Intent.Rest)
	// turns it on.
	ArrangeRestOrder bool
	// PriorityManaAsk offers the activation of a colour-asking mana source
	// that asks every player whether to pay its UnlessCost$ (Rhystic Cave) as a
	// priority option, and poses its colour ask and the players' unless asks as
	// decisions (mana_priask.go). The oracle sets it with oracle.Config's knob:
	// gorge's payment windows never list such a source (CR 605.4), so its asks
	// are reachable only through this priority option. Off (the zero value) the
	// source is offered nowhere, as before.
	PriorityManaAsk bool
	// GorgeAltAdd poses an either-or additional cost the way gorge does:
	// the ask lists the unpayable parts too (choosing one undoes the cast,
	// the card going to the end of the hand, and returns to priority).
	// Either way the cast is offered only when one part can be paid in
	// full (its picks and the spell's mana with that part's), gorge's
	// existential gate; off, only the payable parts are listed: an
	// environment for a search or a policy has no no-op answer to loop on.
	// The oracle sets it.
	GorgeAltAdd bool
	// GorgeAutopaySurface offers only the casts gorge's priority list
	// offers: a spell its payment planner never prices -- an {X} cost, an
	// either-or additional cost (Eaten Alive), a target-conditional cost
	// reduction (Luminous Rebuke), FeatPlannedCast -- is offered only when
	// the floating pool alone pays it (gorge's legacy cast option). The
	// lockstep oracle sets it; off, the kernel offers every castable spell.
	GorgeAutopaySurface bool
	// GorgeUnpriced offers the plays gorge's payment planner never prices
	// only when the floating pool alone pays them (gorge then lists the
	// legacy option): an activation an Equipment grants (Fishing Pole's),
	// a cast through a permission that lets mana of any type be spent
	// (Tinybones), and a cast two permissions offer (Muldrotha's per-type
	// statics for an artifact creature: gorge's "ambiguous"). The lockstep
	// oracle sets it; off, the kernel offers them whenever payable.
	GorgeUnpriced bool
	// GorgeLowering offers and pays a plain cast gorge's cast planner never
	// prices the way the oracle reaches it in gorge -- by lowering a
	// potential play (rules potentialModeCastPlan): it is offered when the
	// pool pays it, or when the potential planner prices it (a
	// target-conditional reduction at its full cost; an either-or
	// additional cost when a mana-free part's picks can be paid), and the
	// planner's sources for the spell's mana cost (no target chosen) are
	// tapped into the pool as the cast begins, before any choice, unless
	// the pool already pays it; the cast then pays from the pool. The
	// oracle sets it when GorgeAutopaySurface is off; a search environment
	// leaves it off (the mana would float).
	GorgeLowering bool
	// GorgeMustAttack poses a creature that attacks each combat if able
	// (Juggernaut, Flamewake Phoenix) the way gorge does: an attacker pick
	// like any other, the declaration's Done withheld until it is picked
	// (gorge marks the option Required). Off, such creatures are declared
	// attacking the defending player before the declaration, which a
	// search's declaration candidates rely on. The oracle sets it.
	GorgeMustAttack bool
	// Commander starts a Commander game (CR 903): each seat's commander,
	// Commanders[p] (up to two indices into that seat's deck, a partner pair
	// (CR 702.124); -1 none), is put into the command zone before the seat's
	// shuffle and opening hand, as gorge's genesis does, and each player
	// starts at 40 life (CR 903.7). A caller that sets it gives gorge
	// StartingLife 40.
	Commander  bool
	Commanders [MaxSeats][2]int16
	// SeatCommanders seats Commanders[p] in the command zone at genesis
	// without playing a Commander game: no 40 life, no command-zone cast offer
	// and no commander damage. It is how a Vanguard (Sisay) starts in the
	// command zone, as gorge's genesis seats Config.Commanders regardless of
	// format (a Vanguard's static functions from there, CR 605.3b).
	SeatCommanders bool
	// Side is each seat's sideboard, the cards "outside the game" (CR 400.11):
	// Side[p][:SideN[p]], at most MaxSide per seat. They are objects of their
	// own, numbered after both decks, in the zone ZoneSideboard; only a play
	// permission that names the sideboard (The Holodeck) ever reaches one
	// (zone_sideboard.go). The zero value is no sideboard.
	Side  [MaxSeats][MaxSide]CardID
	SideN [MaxSeats]uint8
	// Planechase starts a Planechase game (CR 901): each seat's planar deck
	// PlanarDecks[p][:PlanarN[p]] is dealt face down with its top plane face
	// up in ZonePlanarDeck (zone_planechase.go). The face-up plane's command-zone
	// triggers and statics function from the planar deck; the planar die and
	// the chaos/planeswalk actions are the ExtOp handlers there.
	Planechase  bool
	PlanarDecks [MaxSeats][MaxPlane]CardID
	PlanarN     [MaxSeats]uint8
}

// object is one card instance.
// object flag bits (object.of).
const (
	ofToken uint8 = 1 << iota
	ofSickLeft
	ofInHand
	ofCopy    // a token created as a copy of a permanent (CopyAsToken)
	ofPending // a token Aura copy waiting outside every zone for its enchant ask (CR 303.4f)
	// ofAdventure: an Adventure card in exile that got there from the
	// stack as its Adventure (CR 715.4), castable as its front
	// (face_castface.go); cleared when it leaves exile
	ofAdventure
	// ofAbort1 / ofHeld: gorge's per-card no-progress abort count
	// (rules cast_commit.go abortCast castAborts / suppressedCast): the
	// card's cast was reversed once / at least twice with no progress since
	// the hold-out window opened (noProgress); stale once progress is made,
	// cleared at the next reversal after it (reverseCast,
	// cost_abortcount.go)
	ofAbort1
	ofHeld
)

// object stays 8 bytes: the owner shares the acts byte with the activation
// count (objActsMask, objOwnerBits: seats.go), and
// chosen takes the byte the owner used to hold.

type object struct {
	card CardID
	// chosen is the creature subtype a mid-resolution ChooseType stored on
	// this card (gorge Object.ChosenType). It survives the stack and the
	// graveyard, so a second cast of the same card object reuses it, and
	// it is cleared when the card leaves the battlefield.
	chosen uint8
	zone   kop.Zone
	slot   Slot  // battlefield slot when zone == ZoneBattlefield
	ver    uint8 // bumped on every zone change (CR 400.7: a new object)
	// acts holds this turn's non-mana activation count in its low bits
	// (objActsMask) and the owner in its top bits (objOwnerBits). It counts
	// activations of the object whose source is still on the battlefield
	// once the costs are paid (gorge's Object.ActivatedThisTurn, counted at
	// AbilityPush: a sacrificed source does not count; a new object when it
	// leaves (CR 400.7), reset at every turn's start; the network reads it).
	acts uint8
	// of holds the object's flags: ofToken; ofSickLeft, the summoning-sick
	// bit the object had when it last left the battlefield (gorge keeps
	// Object.SummonSick on a card off the battlefield, and the network
	// reads it); ofInHand, the card was in its owner's hand during the game
	// (its kept opening hand and every card put into the hand; reported
	// as in_hand).
	of uint8
}

// owner is the object's owner seat, packed into acts.
func (o *object) owner() uint8 { return (o.acts & objOwnerBits) >> objOwnerShift }

// setOwner packs the object's owner into acts, keeping its activation count.
func (o *object) setOwner(p uint8) { o.acts = o.acts&objActsMask | (p&(MaxSeats-1))<<objOwnerShift }

// activations is this turn's non-mana activation count (FeatActivations).
func (o *object) activations() uint8 { return o.acts & objActsMask }

// perm flags.
const (
	pfTapped uint16 = 1 << iota
	pfSick          // came under its controller's control since their most recent turn began
	pfAttacking
	pfBlocking
	pfBlocked // attacker: became blocked
	pfDeathtouched
	pfRegen       // regeneration shield
	pfDealtFirst  // dealt first-strike damage this combat (no regular damage unless double strike)
	pfCtrlEOT     // controlled until EOT (return to the previous controller at cleanup)
	pfAnimEOT     // animated until EOT
	pfLoyaltyUsed // a loyalty ability of this permanent was activated this turn
	pfOnceGame    // a once-per-game ("activate only once") ability of this permanent was activated
	pfDtMore      // a second deathtouch mark (gorge adds a "Deathtouched" counter per deathtouch damage event)
	pfBatchGone   // the slot's leave has fired in a simultaneous departure still on the battlefield (zones.go fireLeave); removePerm clears it
)

// faceDown reports whether the permanent at sl is face down (CR 708.2;
// face_facedown.go keeps the mark in perm.gty's top bit, which only
// a generated table with a Morph-family card ever sets).
func (s *State) faceDown(_ *prep, sl Slot) bool {
	return s.perms[sl].gty&gtyFaceDown != 0
}

// markDT adds n of gorge's deathtouch marks to p (two kept: pfDeathtouched
// and pfDtMore).
func (p *perm) markDT(n int) {
	for ; n > 0; n-- {
		if p.flags&pfDeathtouched == 0 {
			p.flags |= pfDeathtouched
		} else {
			p.flags |= pfDtMore
		}
	}
}

// override record flags (the fixed-order per-permanent override that stands
// in for layers: copy -> type set -> lose abilities -> set P/T).
const (
	ovSetPT uint8 = 1 << iota
	ovSetPTEOT
	ovTypes
	ovTypesEOT
	ovLoseAll
	ovLoseAllEOT
	ovCopy
	ovGenTy // perm.gty holds the generated table's override-derived types (refreshTy)
)

// override is a permanent's override record.
type override struct {
	flags  uint8
	setP   int8
	setT   int8
	walk   uint8 // walkOn: two or more order-sensitive effects, the CR 613 walk derives it (refreshTy; static_layerwalk.go); walkCond (static_reentryguard.go)
	copyOf CardID
	types  kop.TypeMask
	kw     kop.KwMask // keywords the override grants (Animate)
}

// perm is a battlefield slot.
type perm struct {
	// (fields ordered widest first, 56 bytes)
	eotKw kop.KwMask
	// eotKwRep are the bits granted until end of turn at least twice and
	// three times (gorge lists a keyword once per granting effect)
	eotKwRep [2]kop.KwMask
	ovr      override
	flags    uint16
	ts       uint16
	// Counters, sparse: p1 is the +1/+1 count; ck/cn are the
	// ctrSlots lowest other kinds it holds (ck 0 = a free slot: CtrP1P1 never
	// takes one), kept packed at the front in ascending kind order so equal
	// counter states are equal bytes (ObjHash). Higher kinds are entries of
	// the shared overflow table (genState.ctrOver, ctr_sparse.go), reached only when
	// every slot is taken. Counts are 16 bits: gorge's are int32 and a Mossborn
	// Hydra-style doubling must not saturate at 255.
	// ctrN/ctrLow/setCtrN/addCtrN/subCtrN are the raw accessors. Production
	// counter placements go through State.placeCounters, the one funnel
	// that also applies the CantPutCounter prohibition and the AddCounter
	// replacements (ctr_cantput.go).
	p1       uint16
	cn       [ctrSlots]uint16
	obj      Obj
	ctrl     uint8
	damage   uint8
	eotP     int8 // until-end-of-turn pump, saturated to the int8 range; the true amount of a larger one is in State.ptx
	eotT     int8
	attached Slot // what this Aura/Equipment is attached to
	blocking Slot // blocker: the attacker it blocks
	nblock   uint8
	actUsed  uint8            // OncePerTurn abilities activated this turn (bit per ability index); beside nblock so the objWord and TCode fields below pack without a pad byte
	chosen   objWord          // "as this enters" choice; a last bearer's host (zones.go)
	bord     objWord          // a last bearer's host (static_lastbearer.go); blocker: declaration order (damage assignment order); attacker: declaration order (Attacks trigger queueing)
	atk      TCode            // attacker: the defender it attacks (PlayerTarget* or a planeswalker Obj)
	ck       [ctrSlots]uint16 // general counter slots' kinds (see p1)
	// cs is each general slot's CR 613.7 timestamp (the layer clock, State.ts)
	// when a keyword counter (CR 122.1b) was last put on, gorge's
	// state.Counter.Stamp; 0 for every other kind, so equal counter states stay
	// equal bytes. It moves with ck/cn (ctrInsert, ctrFree); the overflow
	// table's entries carry theirs (ctrOverEntry.ts).
	cs [ctrSlots]uint16
	// cseen records the counter kinds held since it entered, as one is
	// removed (seeCtr; gorge lists a removed kind at 0): bit i < ctrSlots
	// is general slot i (a seen slot keeps its kind at count 0), then
	// cseenP1 and cseenDT (the deathtouch mark, FeatCtrDeathtouched).
	cseen uint8
	// atkVer is the object version the attacker's target (atk) had when the
	// attack was declared. A planeswalker or battle that left the
	// battlefield and came back is a new object (CR 400.7), not the
	// declared target, so it is no longer being attacked (CR 506.4c). Zero
	// for a player or no target. The byte is written, not padding:
	// ObjHash hashes every byte of perm.
	atkVer uint8
	// spare is the byte the 16-bit counter kinds (ck) left ahead of card,
	// named so perm has no padding (ObjHash hashes every byte of it). Its
	// bits in use are spareSwitchPT (kw_kwmark.go), spareCopyFaceDown
	// (copy_facedown.go), spareRegenX (repl_regenerate.go) and spareSolved
	// (kw_crewsaddle.go).
	spare uint8
	card  CardID       // the card's id (the copy-of id lives in ovr)
	gty   kop.TypeMask // types derived by a generated table's overrides (ovGenTy)
}

// ctrSlots is the number of general (non-+1/+1) counter kinds a permanent
// stores itself; further kinds go to the overflow table (ctr_sparse.go).
const ctrSlots = 3

// spareSwitchPT is perm.spare's until-end-of-turn P/T switch parity
// (ExtKwMark KwMarkSwitchPT, CR 613.4d).
const spareSwitchPT uint8 = 1

// Slots hold the lowest kinds, so a kind with fewer than ctrSlots general
// kinds below it is never in the overflow table: ctrLow reads it.
var _ = [1]struct{}{}[kop.CtrStun-ctrSlots]

const (
	cseenP1 uint8 = 1 << (ctrSlots + iota) // +1/+1 seen
	cseenDT                                // deathtouch mark seen
)

// ctrN is p's count of counter kind c.
func (p *perm) ctrN(s *State, c int) int {
	if n := p.ctrLow(c); n != 0 || p.ck[ctrSlots-1] == 0 {
		return n
	}
	return s.ctrOverN(p.obj, c)
}

// ctrLow is p's count of +1/+1, -1/-1, loyalty or stun counters, the kinds
// the slots always hold, without a State to reach the overflow table.
func (p *perm) ctrLow(c int) int {
	if c == int(kop.CtrP1P1) {
		return int(p.p1)
	}
	// the occupied slots are packed (ctrInsert, ctrFree): the first empty
	// one ends the search, and most permanents have none
	for i := range p.ck {
		if p.ck[i] == 0 {
			return 0
		}
		if int(p.ck[i]) == c {
			return int(p.cn[i])
		}
	}
	return 0
}

// setCtrN sets p's count of counter kind c, saturating at the 16-bit
// storage bound. gorge's count is unbounded (Bristly Bill, Spine Sower's
// doubling reached 81920), so a placement past 0xffff is declared, not
// clamped away silently: the kernel reports its counter capacity
// (StallCounters) and stores the value it would hold, and the game stops at
// the next step boundary (capFull's latch).
// It reports false, setting nothing, when c needs an entry and the overflow
// table is full: the caller stalls (StallCounters).
func (p *perm) setCtrN(s *State, c, n int) bool {
	if n < 0 {
		n = 0
	}
	if n > 0xffff {
		s.ctrStall()
		n = 0xffff
	}
	if DebugCtrXDropDivinity && c == int(kop.CtrDivinity) && n > p.ctrN(s, c) {
		return true // test-only mutation (ctr_named.go): placement stores nothing
	}
	if c == int(kop.CtrP1P1) {
		p.p1 = uint16(n)
		return true
	}
	for i := range p.ck {
		if int(p.ck[i]) == c {
			grew := n > int(p.cn[i])
			p.cn[i] = uint16(n)
			if grew && ctrKindKw(uint16(c)) != 0 {
				p.cs[i] = s.tick() // a put refreshes the stamp (gorge stampKeywordCounter)
			}
			if n == 0 && p.cseen&(1<<i) == 0 {
				p.ctrFree(s, i)
			}
			return true
		}
	}
	if p.ck[ctrSlots-1] != 0 && s.ctrOverSet(p.obj, c, uint16(n)) {
		return true
	}
	if n == 0 {
		return true
	}
	return p.ctrInsert(s, c, uint16(n))
}

// addCtrN adds n counters of kind c to p (false: no room, see setCtrN).
// Production placements use State.placeCounters, which gates this on the
// CantPutCounter statics and applies the AddCounter replacements first.
func (p *perm) addCtrN(s *State, c, n int) bool { return p.setCtrN(s, c, p.ctrN(s, c)+n) }

// subCtrN removes n counters of kind c from p (removing no more than it has).
func (p *perm) subCtrN(s *State, c, n int) {
	have := p.ctrN(s, c)
	if n > have {
		n = have
	}
	p.setCtrN(s, c, have-n)
}

// player.mark bits.
const (
	pmDrewEmpty uint8 = 1 << iota // CR 704.5b: the next state-based check loses the seat
	pmLostPass                    // lost in the pass now running, not yet swept: its graveyard still counts for that pass's scan (beginSBAPass, markPassLosses)
)

// player is one seat.
type player struct {
	life        int16
	lib         [MaxDeck + 8]Obj
	libLo       uint8 // library = lib[libLo:libHi], lib[libLo] is the top
	libHi       uint8
	hand        [MaxHand]Obj
	handN       uint8
	grave       [MaxDeck]Obj
	graveN      uint8
	exile       [MaxDeck]Obj
	exileN      uint8
	pool        [kop.NumManaKinds]uint8
	landsPlayed uint8
	drawnTurn   uint8
	mulls       uint8
	lost        bool
	mark        uint8           // pm* bits: drew from an empty library since the last check (pmDrewEmpty); lost in the state-based pass now running (pmLostPass)
	lifeGained  int16           // this turn
	lifeLost    int16           // this turn
	lastLost    int16           // the life lost in the previous turn (ExLifeLostLastTurn)
	scry        uint8           // this turn: scry instructions in bits 0-3, surveil instructions in bits 4-7 (one byte: it fills padding)
	sac         [3]uint8        // this turn: [sacAll] every sacrifice, [sacArtifact] artifact cards, [sacFood] Food cards
	disc        uint8           // cards this player discarded this turn
	cmd         Obj             // this seat's commander (CR 903.3), NoObj none; set at Init (zone_cmdinit.go); a partner is the cold cmdPartner record (zone_cmdpartner.go)
	cmdCast     uint8           // times the commander was cast from the command zone (CR 903.8)
	cmdAsk      uint8           // commander bookkeeping byte for the command-zone replacement
	cmdDmg      [MaxSeats]uint8 // combat damage dealt to this player by each seat's commander (CR 903.10a)
}

// stack entry kinds.
const (
	seSpell uint8 = iota
	seAbility
	seTrigger
)

// stack entry flags.
const (
	sfKicked uint8 = 1 << iota
	sfFlashback
	sfCountered
)

// A spell entry's remVer, never a delayed program's, holds the keywords the
// spell gained on the stack (OpGrantKeyword at a spell, Judith, Carnage
// Connoisseur; grantSpellKw, srcKw). A copy does not keep them (CR 707.2).
const (
	spellGainedDeathtouch objWord = 1 << iota
	spellGainedLifelink
)

// stackEntry is one spell or ability on the stack.
// Face-down spells use mode bit 4, outside the modal-spell mode bitmask
// (bits 0..3) and the warp provenance bits (6..7); ab == 255 (no spell
// ability) distinguishes them from a modal spell. Family occupies bits 0..1.
const (
	seFaceDown = 1 << 4
	seFamMask  = 3
)

// seAtkPlayer is a trigger entry's bound attack (combat.go fireAttacksGen
// stamps it on the pending trigger's skw; takePending moves it to the
// entry's mode): the trigger's attack was on a player, so a trigger whose
// attacker left the battlefield before it resolves still damages the
// defending player (CR 603.3c, 608.2h; filter_combatwords.go OpAtkPWGuard).
// A trigger entry keeps no other mode bit beside seAtkCtrl: the modal-spell,
// face-down, room, warp, bestowed and impending bits are spell and ability
// bits.
const seAtkPlayer uint8 = 1 << 5

// seAtkCtrl is a trigger entry's second bound-attack bit (beside
// seAtkPlayer): the attack was on the trigger's own controller (combat.go
// fireAttacksGen stamps it on the pending trigger's skw; takePending moves it
// to the entry's mode). Such a trigger stalls at its OpAtkPWGuard whether its
// attacker is alive (filter_combatwords.go) or has left — gorge's
// TriggeredDefender.Opponent names nobody for an attacker its controller does
// not control (Mage Slayer on a stolen creature), so the gone arm must not
// fall through to a guessed defender.
const seAtkCtrl uint8 = 1 << 3

type stackEntry struct {
	kind   uint8
	obj    Obj    // the spell's object, or the ability's source object
	srcVer uint8  // the source's ver when the ability was created
	sid    uint8  // an ability's stack identity, for a stack-ability target (tgt_stackability.go; 0 = none; the padding byte); a spell's FINALITY rider count (trig_mayplaystatic.go)
	card   CardID // the source card (last known information)
	ab     uint8  // ability index in the card's CardDef
	ctrl   uint8
	mode   uint8 // chosen modes bitmask
	x      objWord
	flags  uint8
	nt     uint8
	tgt    [kop.MaxTargets]TCode
	tver   [kop.MaxTargets]uint8
	// crim is CR 700.13's pick-time verdicts for the root target slots
	// (act_commitcrime.go): bit i says slot i's pick was a crime when it was
	// made, bit 7 (crimRec) that the asks recorded verdicts, so checkWard's
	// walk judges the target as it was targeted and not what a cost or a
	// trigger made it since (a sacrificed self-target sits in its owner's
	// graveyard at placement). A spell keeps the build's bflCrime instead.
	crim    uint8
	trigObj Obj
	trigPl  uint8
	// btPl is a BecomesTarget trigger's targeted player + 1 (gorge
	// TriggerTarget{Player}); 0 = none. trigSA is its targeting spell or
	// ability + 1 (gorge TriggerStack, Defined$ TriggeredSourceSA); 0 = none,
	// with trigSASid the ability's stack identity (0 for a spell), so a
	// counter removes the ability that targeted rather than a trigger on the
	// same source permanent. Both ride the pending trigger's bytes
	// (pendingTrig.defSeat/trigSA; its identity there is wardSid): the object
	// half keeps trigObj, so neither can share that slot.
	btPl      uint8
	trigSA    uint8 // low byte of the targeting object + 1; the ninth bit is skw's skwSAHi (saWord)
	trigSASid uint8
	// defSeat is an attack trigger's defending player + 1, bound as it was
	// queued (pendingTrig.defSeat; 0 = none; defSeatObj tags an attack on a
	// planeswalker or battle, whose object defSeatIxMask indexes in the cold
	// atkObj table: trig_atkobj.go). At two seats only a TriggeredDefender damage
	// trigger is bound, when the attacker may leave before it resolves. A
	// Squad self-ETB trigger stores its cast's payment count here instead
	// (1..maxMultikick; triggers_gen.go enqueueLKI copies it from the
	// pending trigger), at every seat count, so a reader gates on the card's
	// RepeatedKind and the trigger shape (exprs.go timesKicked). It takes the
	// padding byte before trigAmt.
	defSeat uint8
	trigAmt int16
	// etbSrc is the source of a spell's pending enters-with rider count
	// (ctr_effectentry.go): the resolving SpellCast trigger's source, whose
	// Effect registers the live Event$ Moved replacement gorge composes over
	// the entry. Stamped only when the entrant's own Updated body is exactly
	// the shape the CR 616.1 entry-order ask orders (repl_etborder.go); any
	// other spell's count is applied silently, as before. With no count to
	// name it also holds the one other enter-replacement source of the ask
	// (etbOtherOrderPark).
	// Placed before the odd-length byte run that follows so spc's
	// two-byte alignment does not grow the struct.
	etbSrc Obj
	li     uint8 // the triggering object's last known information: LKI pool index (zone_lki.go)
	// remVer is a delayed program's remembered object's ver at registration
	// and remSet whether it was captured: RefDelayedRemembered reads nothing
	// once the object has changed zones since (CR 603.7c, 400.7; refObjCtx).
	// A spell has no remembered object: see spellGainedDeathtouch. A granted
	// trigger has none either, and keeps the permanent that granted it here
	// instead: see trigHost.
	remVer objWord
	remSet bool
	xi     uint8 // multi-object target slots: genState.xtp index + 1 (gen.go)
	grp    uint8 // an AttackersDeclared trigger's attacker group: State.cold.atkGrp index + 1 (trig_attackers.go)
	skw    uint8 // an ability's source's deathtouch and lifelink as it left the battlefield (skwValid, skwDT, skwLL; kw_srcleft.go)
	// trigMV is the triggering spell's pre-cast mana value, snapshotted
	// when a SpellCast trigger is queued (value_spellmv.go): the
	// triggering spell can leave the stack before the value is read.
	trigMV objWord
	// prov is a spell's cast provenance, recorded only for a
	// table that reads it (genPrep.anyCastProv): bits 0-2 the zone it was
	// cast from (kop.Zone), bits 3-7 the creatures that convoked it
	// (saturating at 31). It takes the struct's padding byte.
	prov uint8
	// cvok is the creatures that convoked the spell's cast (CR 702.66),
	// stamped at payment only for a table that reads them (the genUse
	// useConvokedReads, kw_convokelist.go): State.cold.convoked index + 1
	// (0 = none). It takes the struct's padding byte.
	cvok uint8
	// spc is the colours of mana spent to cast a spell,
	// tallied at payment only for a table that reads them
	// (genPrep.anyColSpent, mana_converge.go): bits 0-9 five 2-bit
	// saturating counts (W U B R G, 0-3); bit 10 is reserved for
	// bits 11-15 the extra +1/+1 counters the spell enters
	// with (ctr_effectentry.go). A stack copy is never cast: 0.
	spc uint16
	// etbX holds the third and fourth Effect-made enters-with counts of a spell
	// (ctr_effectentry.go): the pair code in spc holds the first two (or a
	// first alone), and an effect the code cannot hold lands here, each its
	// own count of 1..255. Zero is none.
	etbX [2]uint8
}

// pendingTrig is a triggered ability waiting to be put on the stack.
// faceDown reports whether e is a face-down permanent spell (Morph,
// Megamorph or Disguise cast face down).
func (e *stackEntry) faceDown() bool { return e.ab == 255 && e.mode&seFaceDown != 0 }

// morphFamily is e's face-down family (kop.MorphMorph, MorphMegamorph or
// MorphDisguise); 0 when e is not face down.
func (e *stackEntry) morphFamily() uint8 {
	if !e.faceDown() {
		return 0
	}
	return e.mode & seFamMask
}

type pendingTrig struct {
	obj     Obj
	srcVer  uint8
	card    CardID
	ab      uint8
	ctrl    uint8
	trigObj Obj
	trigPl  uint8
	// A BecomesTarget player half's targeted player + 1 (gorge
	// TriggerTarget{Player, IsPlayer}) rides defSeat: that byte is the attack
	// trigger's defending player, and a player-target trigger never has one,
	// so the kinds do not overlap (becameTargetPlayer, trig_becomestarget.go;
	// takePending moves it to stackEntry.btPl only when trigSA is set).
	// trigSA is the targeting spell or ability + 1 whose targeting queued a
	// BecomesTarget trigger (gorge TriggerStack / Defined$ TriggeredSourceSA);
	// 0 = none. trigObj keeps the targeted object (TriggerTarget), so the
	// source half cannot share its slot (kgen lower_counterspell.go). An
	// ability's stack identity rides wardSid (the same byte a ward trigger's
	// answered ability uses): the source permanent can itself be a trigger's
	// source, so the object alone does not name the entry a counter removes.
	trigSA  uint8 // low byte of the targeting object + 1; the ninth bit is skw's skwSAHi (saWord)
	trigAmt int16
	li      uint8 // the triggering object's last known information: LKI pool index (zone_lki.go)
	// remVer is a delayed program's remembered object's ver at registration
	// and remSet whether it was captured: RefDelayedRemembered reads nothing
	// once the object has changed zones since (CR 603.7c, 400.7; refObjCtx).
	// A granted trigger has no remembered object and keeps the permanent that
	// granted it here instead (stackEntry.trigHost)
	// (the captured flag rides pendingTrig.skw's skwRem bit: no byte of its own)
	remVer objWord
	// x is the trigger object's spell's paid X, bound as it triggered
	// (Tf2XBound, CR 107.3m; trig_xpaid.go): the stack entry's x.
	// It takes the struct's padding byte.
	x objWord
	// trigMV is the triggering spell's pre-cast mana value, snapshotted
	// when the trigger is queued (value_spellmv.go).
	trigMV objWord
	// wardSid is the stack identity of the ability a ward trigger answers,
	// captured when checkWard queues the trigger; it also carries a
	// BecomesTarget trigger's targeting ability (trigSA's identity, Kira's
	// Defined$ TriggeredSourceSA). The stack at placement can hold a newer
	// ability of the same source (two triggered abilities of one source can
	// be pending at once), which is not the warded/targeting one (CR 603.3b,
	// 701.6a). 0 = none: a spell is named by its trigObj, and a table with
	// no ward flag has no identity to stamp.
	wardSid uint8
	// grp is the AttackersDeclared event's attacker group: State.cold.atkGrp
	// index + 1 (trig_attackers.go; 0 = none)
	grp uint8
	// skw is the source's departure keywords (stackEntry.skw), with skwMany and skwRem above them
	skw uint8
	// defSeat is the defending player + 1 of the attack event that queued
	// the trigger (gorge TriggerContext.DefendingPlayer, bound from the
	// DeclareAttackers event; fireAttacksGen), 0 = none, with defSeatObj set
	// when the attack was on a planeswalker or battle. Past two seats every
	// attack trigger is stamped; at two seats only a TriggeredDefender
	// damage trigger is. It takes the struct's padding byte. A Squad self-ETB
	// trigger also stores its cast's payment count here (1..maxMultikick;
	// triggers_gen.go enqueueLKI), at every seat count; a reader gates on the
	// card's RepeatedKind and the trigger shape (exprs.go timesKicked).
	defSeat uint8
}

// delayed is a delayed triggered ability (OpDelayedTrigger).
type delayed struct {
	act  uint8 // kop.Op to perform
	ctrl uint8
	obj  Obj
	ver  objWord // the object's version; a dlyLeaveGrave record's registering source
	when uint8   // 0 = next end step
	// creator is the spell/permanent that created an AtEOT$ rider, the
	// trigger's stack source (CR 603.7d-e, trig_ateotcreator.go);
	// NoObj on every other record (the source is obj).
	creator Obj
}

// The table registry: a State names its table by a 16-bit index (State
// holds no pointers), resolved through a two-level directory of
// 256-entry chunks, so up to 65536 tables can be live at once and a
// lookup is two loads with no lock. ReleaseTable frees an index for reuse.
const tableChunk = 256

var (
	tablesMu   sync.Mutex
	tableIdx   = map[*kop.Table]uint16{} // registration only, never ranged
	tableFree  []uint16
	tableNext  int
	prepChunks [65536 / tableChunk]atomic.Pointer[[tableChunk]*prep]
)

// registerTable returns t's registry index, preparing it on first use.
func registerTable(t *kop.Table) uint16 {
	tablesMu.Lock()
	defer tablesMu.Unlock()
	if i, ok := tableIdx[t]; ok {
		return i
	}
	var i uint16
	switch {
	case len(tableFree) > 0:
		i = tableFree[len(tableFree)-1]
		tableFree = tableFree[:len(tableFree)-1]
	case tableNext < 65536:
		i = uint16(tableNext)
		tableNext++
	default:
		panic("kernel: 65536 tables registered at once (ReleaseTable the finished ones)")
	}
	c := prepChunks[i/tableChunk].Load()
	if c == nil {
		c = new([tableChunk]*prep)
		prepChunks[i/tableChunk].Store(c)
	}
	c[i%tableChunk] = prepare(t)
	tableIdx[t] = i
	return i
}

// ReleaseTable frees t's registry entry: every State playing t becomes
// invalid, and t's index is reused by a later registration. A table that
// was never registered is ignored.
func ReleaseTable(t *Table) {
	tablesMu.Lock()
	defer tablesMu.Unlock()
	i, ok := tableIdx[t]
	if !ok {
		return
	}
	delete(tableIdx, t)
	prepChunks[i/tableChunk].Load()[i%tableChunk] = nil
	tableFree = append(tableFree, i)
}

// control states: where Step resumes.
const (
	ctlNone           uint8 = iota
	ctlMullDeclare          // pose keep/mulligan to mullOrder[mullCur]
	ctlMullBottom           // pose a bottom pick to mullOrder[mullCur]
	ctlStepBegin            // perform the turn-based actions of s.step
	ctlPriority             // SBA, triggers, then a priority decision for s.prio
	ctlPlaceTriggers        // put pending triggers on the stack (APNAP, order decisions)
	ctlBuild                // an entry being put on the stack: modes / X / targets
	ctlAttackers            // declare-attackers picks
	ctlBlockers             // declare-blockers picks
	ctlResolve              // a resolution suspended on a decision
	ctlCleanupDiscard       // discard-to-hand-size picks
	ctlLegend               // legend rule: the controller picks the permanent to keep
	ctlDivide               // combat damage division picks (or the damage step continuing)
	ctlOver
	ctlExileOrder // a parked move to a graveyard with several exile-instead effects: which applies (CR 616.1)
	ctlTurnUpX    // a keyword turn-up awaiting its announced X (face_turnfaceup.go; parked in genState.pick*)
	// the CR 903.9a commander-zone ask: ctlCmdZone0 + seat + MaxSeats*slot
	// (zone_cmdzone.go cmdZoneCode); 4..7 are reached only past two seats
	ctlCmdZone0
	ctlCmdZone1
	ctlCmdZone2
	ctlCmdZone3
	ctlCmdZone4
	ctlCmdZone5
	ctlCmdZone6
	ctlCmdZone7
	// ctlTurnUpTap: transient inside face_facedown.go's turn-up tap step, so the
	// planner prices its restricted sources as the special action's payment
	// (payDescNow); parked in genState.pickLeft/pickF, never a decision.
	ctlTurnUpTap
	// ctlBand: the CR 702.22c band declaration, posed after the attackers are
	// declared for each attacker with banding that has a candidate (combat_band.go;
	// s.lastPick is the leader asked)
	ctlBand
	// ctlReflectPick: the colour of the reflected mana a planned tap of an
	// all-at-once source makes under a reflect carrier (mana_reflectpickask.go;
	// the mask and the payer ride genState.reflMask / reflP)
	ctlReflectPick
	// ctlEnlist: the CR 702.154a enlist election, posed once the attackers
	// are declared for each attacker with Enlist that has a creature to tap
	// (kw_enlist.go; s.lastPick is the attacker asked)
	ctlEnlist
	// ctlTurnUpReveal: a keyword turn-up whose cost reveals a hand card,
	// awaiting the pick among several candidates (act_revealpick.go; parked in
	// genState.pick*)
	ctlTurnUpReveal
	// ctlManaAsk: a priority-time activation of a colour-asking unless source
	// (Rhystic Cave) under Config.PriorityManaAsk: the colour ask, then each
	// player's unless ask (mana_priask.go; parked in genState.winSrc,
	// winUnless*)
	ctlManaAsk
	// ctlExert: the CR 702.100a exert election, posed once the attackers are
	// declared (after the band asks) for each attacker carrying an
	// OptionalAttackCost offer (combat_exert.go; s.lastPick is the attacker asked)
	ctlExert
)

// every seat's two command-zone ask states fit ctlCmdZone0..7
const _ = uint8(ctlCmdZone7 - ctlCmdZone0 - (2*MaxSeats - 1))

// build stages.
const (
	bsAltAdd uint8 = iota
	bsTapCost
	bsX
	bsCosts
	bsModes
	bsTargets
	bsPay
	bsPips      // the hybrid pip announcements (gorge manaAsk: after modes, before targets)
	bsDivide    // the CR 601.2d division of a divided slot's total among its targets (gorge divisionAsk: after the targets, before payment)
	bsPush      // the entry goes on the stack (CR 601.2a / 602.2a: gorge pushCast / pushActivation)
	bsGift      // the gift election (CR 702.174a: gorge giftAsk, after the either-or additional cost; kw_gift.go)
	bsMana      // the CR 601.2g mana abilities: the planned sources tapped into the pool, before the cost objects (gorge manaWindowAsk, then costObjectAsks)
	bsConvoke   // the CR 702.51a convoke announcement (gorge convokeAsk, after the tap costs and before X)
	bsCasualty  // the casualty sacrifice election (CR 702.153a: gorge optionalSacrificeAsk, before convoke / X; kw_casualty.go)
	bsEvidence  // the CollectEvidence<N> graveyard pick of an activation (CR 701.59a: gorge evidenceAsk, after the division and before payment; cost_collectevidence.go)
	bsDelve     // the granted-delve graveyard announcement (CR 702.66a: gorge delveAsk, after convoke and X and before the pip asks; kw_delve.go)
	bsConspire  // the granted-conspire tap election (CR 702.78a: gorge conspireAsk, after the tap costs, before convoke; kw_conspire.go)
	bsMiracle   // the miracle reveal ask of a trigger not yet on the stack (CR 702.94a: gorge askTriggerOptional at placement; kw_miracle.go)
	bsReplicate // the replicate count ask (CR 702.56a: gorge replicateAsk, after the tap costs and before the multikicker ask; kw_squadreplicate.go)
	bsMulti     // the multikicker count ask (CR 702.43: gorge multikickAsk, after the tap costs and before Convoke/X; kw_multikicker.go)
	bsSquad     // the squad count ask (CR 702.157a: gorge squadAsk in preCostAsks, after the multikicker ask and before Convoke/X; kw_squadreplicate.go)
	nBuildStages
)

// build is the stack entry being put on the stack (a cast spell, an
// activated ability, or a triggered ability).
type build struct {
	e       stackEntry
	stage   uint8
	ti      uint8 // next target slot
	modesN  uint8 // modes picked so far
	pendIdx uint8
	// subCtr: counters still to choose for Quilled Greatwurm's graveyard
	// cast ("by removing six counters from among creatures you control");
	// each pick is a pickSubCtr entry in picks/pickKs, paid at bsPay
	subCtr uint8
	// cost picks (discard / sacrifice / tap-creatures costs): pickK is the
	// cost flag being chosen, pickLeft the picks still owed, picks the
	// objects chosen so far (paid at bsPay).
	pickK     pickKind
	pickLeft  uint8
	pickN     uint8
	picks     [24]Obj // Lathril taps ten Elves; 24 since kcaps2x (was 12: a tap-any-number cost past 12 picks, c6)
	pickKs    [24]pickKind
	donePicks pickKind
	dilLo     uint8 // bflDilemma: the pending triggers that dilemma queued, [dilLo, dilHi)
	dilHi     uint8
	bfl       uint16   // bflPushed: the entry is on the stack (top); bflLowered: the oracle lowered the cast
	altAdd    uint8    // either-or additional cost: 0 the spell's own, 1 CardDef.AltAddCost
	pipN      uint8    // hybrid pips announced so far
	pipC      [2]uint8 // their colours (W U B R G = 0..4), in pip order
	pushNP    uint8    // pending triggers when the spell was pushed: a reversed cast drops the ones its own move queued (CR 733.1)
	manaNP    uint8    // 1 + pending triggers when the spell's mana step began (0 = not begun): the reversal keeps what its mana abilities queued after it (unpushSpell)
	fbGrants  uint8    // the card's flashback grants before this cast ended them (the undo restores them)
	divSlot   uint8    // bsDivide: the divided target slot
	divLeft   uint8    // bsDivide: points still to place beyond each target's 1
	cas       uint8    // casElected / casPaid: the casualty cast (kw_casualty.go)
	origin    kop.Zone // a pushed spell's zone before the push (the CR 733.1 reversal returns it there)
	libPos    uint8    // origin library: the card's index from the top before the push (the reversal puts it back there, CR 733.2)
	castN     uint8    // the ring entry noteCast recorded for this cast (index+1; 0 = none)
	obsSl     Slot     // 1 + the slot of the observed mana source (a Tf2ManaFromSelf reader) this cast tapped; 0 none (mana_spendtrigger.go)
	obsMore   [7]Slot  // 1 + each further plain observed source the same cast tapped (several Path of Ancestry), each rider reading its own; 0 free (noteTapSrc)
	raiseSl   Slot     // the may-play static whose non-mana RaiseCost$ this cast owes: its slot (NoSlot = the cast card's own static); raiseAb 0 = none (cost_mayplayraise.go)
	raiseAb   uint8    // Static.B of that static: 1 + the latent ability holding the raise cost
	fixedGen  uint8    // the generic part as last priced (bflCostFixed: the total cost, CR 601.2f)
	obsWin    uint8    // obsWinTapped: obsSl was set by a manual-window tap before the payment, so it counts only if the payment spends that source's colour; obsWinPaying: the payment began (mana_spendtrigger.go obsWinSettle)
	pipShort  bool     // the lowered pip window ran dry (loweredPipWindow): the cost objects are still asked, and the payment reverses the cast
	altAbort  bool     // the chosen either-or part cannot be paid: the fallback CR 601.2g window runs at bsMana (manaStep) and reverses the cast (CR 733.1)
	impPlan   bool     // gorge's planner priced this Improvise cast (kw_improvisepay.go)
	snowPre   uint8    // a SpentSnow card's cast: the seat's snow units before the mana payment (snowSpentRec, mana_snow.go)
	payLifeMV bool     // the cast went through a may-play permission whose alternative cost is life equal to its mana value (cast_mayplaypaylife.go)
	payEnergy uint8    // the cast went through a may-play permission whose alternative cost is this much energy (kop.StMayPayEnergy, Nissa, Worldsoul Speaker; paid beside payLifeMV)
	// repeatCost is the Squad/Replicate payment's pick cost scaled to the
	// answered count, returned by buildPickCost so the ordinary pick stages
	// pay it (kw_squadreplicate.go).
	repeatCost kop.Cost
}

// build.bfl bits. bflLowered: a cast gorge's planner does not price,
// which the oracle lowers (beginCast taps the witness at priority and
// casts from the floating pool), so gorge's cast has no payment plan and
// the part of the cost the pool does not hold is paid through its mana
// window (finishBuild: payWindow).
const (
	bflPushed uint16 = 1 << iota
	bflLowered
	bflNoProg   // noProgress held when the spell was pushed: the CR 733.1 reversal's hold-out test reads it, not the push's own touches
	bflManaStep // the planner taps in bsMana (manaStep): a token sacrificed for mana stays in the graveyard until the SBA, as gorge's planned activations leave it
	bflAnyColor // the may-play cast's MayPlayIgnoreColor rider: any colour pays a coloured pip ({C} stays strict)
	bflCostComp // transient, inside buildCost's reductions only: the cast ring hides this cast's own push (gorge costCompositionEvent)
	bflCrime    // a spell pick was a crime (CR 700.13; act_commitcrime.go): finishBuild fires the crime listeners
	bflTre      // this cast tapped a Treasure source for mana (CastSpecTreasure)
)

// bflTargetPaid: a convoke cast gorge's planner takes because only its own
// target pays it (targetPaidPlanned): with no creature announced, its plan runs
// at the printed cost as any planned cast's does, not the manual window.
const bflTargetPaid uint16 = 1 << 8

// bflCostFixed: the spell's mana step priced it, and paying (a Treasure's
// sacrifice under affinity, a cost's discard) does not move its total cost.
const bflCostFixed uint16 = 1 << 9

// bflTokenGone: a token sacrificed for mana after this cast was pushed. gorge's token ceases at
// the state-based check after the cast's reversal, an event that ends the no-progress
// interval, and dirac's has already gone (reverseCast).
const bflTokenGone uint16 = 1 << 10

// bflUsedMark: this cast marked a once-per-turn permission static used (legal_gen.go
// castMayPlay), which a reversed cast gives back (CR 733.1; reverseCast).
const bflUsedMark uint16 = 1 << 14

// bflDilemma: the build's choice of modes faced a dilemma (trig_dilemma.go).
const bflDilemma uint16 = 1 << 15

// bflWinTap: the cast window stopped at a tap-another-permanent source with several
// permanents to tap, and the build waits on that pick (ChWindowTap, mana_window.go).
// bflMsDone: manaStep's own steps before the window have run, so the window's re-entry
// after the pick does not repeat them. bflPayDone: the same for finishBuild's cost
// payments before the mana.
const (
	bflWinTap  uint16 = 1 << 11
	bflMsDone  uint16 = 1 << 12
	bflPayDone uint16 = 1 << 13
	// bflPlanCut: a Treasure sacrificed by the planner's tap parked behind a
	// replacement ask. Gorge's payment plan stops at a step that poses a decision
	// (paymentFallbackChoiceRequired), so the rest of the cost is paid through the
	// manual window, not a fresh plan: manaStep's re-entry taps through the window.
	bflPlanCut uint16 = 1 << 14
)

// resolution ask kinds (resCtx.ask).
const (
	raNone uint8 = iota
	raDiscard
	raScryTop   // scry: keep card on top? (one card at a time)
	raSurveil   // surveil: put into graveyard?
	raSacrifice // choose a permanent to sacrifice
	raChooseColor
	raChooseType
	raChooseCard
	raOptional     // "you may" trigger: yes/no
	raGeneric      // GenericChoice branch
	raDigPick      // dig: choose a card to keep
	raOptSac       // optional sacrifice: 0..Amount picks (act_optionalask.go)
	raOptDiscMay   // optional discard election: discard?
	raOptDisc      // optional discard pick (post-election, or AnyNumber)
	raChooseNumber // mid-resolution ChooseNumber (ExtChooseNumberMid; repl_choosenumber.go)
)

// resFlags is resCtx.flags: single-bit facts of the resolution in progress.
type resFlags uint8

const (
	rfTrigGone resFlags = 1 << iota // the trigger's triggering permanent left and came back (CR 400.7)
	rfNoPrev                        // the damage op executing now can't be prevented (static_cantprevent.go)
	rfRestPh                        // scry/surveil: the top phase closed; the rest cards are being ordered
	rfCopyGrp                       // a copy op has entered a copy and may park for the next one (interp_gen.go copyTokens): its later copies join the first one's AtEOT$ sacrifice trigger
	// rfBodyOp / rfBodyTurn mark a pending dredge ask of a draw-replacement
	// body card (kw_dredge.go dredgeBodyPose): the ask is raDredgeRes, and
	// the flag says which enclosing site the answer resumes. rfBodyOp: an
	// OpDraw card (the chain slot rides g.pickP, the loop's res.left /
	// g.pickZ / g.pickLeft); rfBodyTurn: the turn draw (the chain slot rides
	// res.left, the answer finishes the draw step).
	rfBodyOp
	rfBodyTurn
	rfOppCopy // a per-opponent copy walk is entering its copies (kw_encore.go ExtEncore, kw_myriad.go ExtMyriad): copyTokenEnterFrom gives each the duty or attack that names the opponent res.left
	// rfPayPark: an unless-cost PayReturn move raised a CR 903.9b
	// commander_zone park while the payment was being settled. The payment
	// suspends with the resolving entry still on the stack; cmdZoneAnswer
	// finishes the move and the payment (cost_return.go).
	rfPayPark
)

// resCtx is a resolution in progress.
type resCtx struct {
	e       stackEntry
	pc      uint8
	ask     uint8
	askP    uint8   // who answers
	left    uint8   // remaining picks for a multi-pick op
	chosen  objWord // a choice: a seat, an index, or (RefChosen) an object
	lastDmg int16
	lastTok Obj
	loopN   [2]uint8 // Repeat counters (nesting depth 2)
	loopPC  [2]uint8
	loopS   [2]uint8
	loopD   uint8
	eachObj Obj
	peek    [8]Obj       // scry/surveil/dig look-at cards
	peekSel uint8        // answers so far (bit per peek index)
	arr     [MaxHand]Obj // arrange: cards kept on top, in pick order (then, in the rest phase, the rest cards in pick order); a discard's picks, one slot per hand card
	restSel uint8        // scry/surveil rest phase (Config.ArrangeRestOrder): rest picks so far (bit per peek index)
	topN    uint8        // scry/surveil rest phase: the top picks (arr[:topN]); the rest picks follow
	peekN   uint8
	peekI   uint8
	active  bool
	// trigGone: the entry is a trigger whose triggering permanent had left and come back (two moves
	// past its last known information) before the resolution began: a new object (CR 400.7). The one
	// move a death trigger records, or a move this resolution makes itself, is no such change.
	// The flag is resFlags bit rfTrigGone; rfNoPrev is the damage op executing now
	// that can't be prevented (kop.DmgNoPrevention, DealDamage NoPrevention$;
	// static_cantprevent.go), set only around its damage. One byte for both: a
	// further bool would grow State past its pinned layout (statelayout_test.go).
	flags resFlags
	bad   uint8  // targets illegal at resolution (bit per slot)
	xbad  xtMask // multi-object extras illegal at resolution (bit per xTargets index; only with genPrep.anyEachTgt)
	// col is the resolution's mid-resolution ChooseColor answer (colour
	// index + 1, 0 none; ExtChooseColorRes), which a ChosenColor filter
	// reads (static_colourprot.go resColour)
	col uint8
	// nc is a mid-resolution NameCard answer: the named card's CardID plus
	// one, 0 none (repl_namecard.go). A stack source's chosen name (gorge
	// Object.ChosenName) while its chain runs; a battlefield source keeps
	// its own in the chosen byte.
	nc uint8
	// chPl is the resolution's mid-resolution ChoosePlayer answer (seat + 1,
	// 0 none; ExtChoosePlayerMid), which RefChosenPlayer reads while the
	// resolution holds it (repl_chooseplayer.go resChosenPlayer). OpChoosePlayer's
	// random pick (Maddening Hex's Attach to the chosen player) is kept here
	// too, seat + 1 or 0 for an empty pool, and RefResChosenPlayer reads it
	// (act_chooseplayer.go); one byte, as the layout has no padding for a second.
	chPl  uint8
	cpl   uint8 // the chosen player and the remembered players (ExtChoosePlayerRes; act_chooseplayerres.go)
	remLK int16 // sum of the last-known power or toughness of the permanents this resolution sacrificed under RememberSacrificed$ (zone_rememberlki.go)
}

// State is a whole game. Copy one with CopyFrom, or probe on a fork; go vet
// rejects an assignment.
type State struct {
	_ noCopy
	stateCore
	// Outside stateCore so that CopyFrom can copy only the slots in use. A hot
	// array stays in State itself, so reaching it costs no extra load.
	perms [MaxPerm]perm
	cold  coldArrays
	pool  *ForkPool
}

// stateCore is a game's rules state: a fixed-size value with no pointers,
// maps, slices, strings or interfaces.
type stateCore struct {
	tbl  uint16
	cfg  uint8
	ctl  uint8
	over bool
	win  int8

	turn    uint16
	step    kop.Step
	active  uint8
	prio    uint8
	passes  uint8
	startPl uint8
	cfgx    uint8 // more Config bits (cfgx*) and transient latches, in the header's padding byte
	ts      uint16

	rng rng

	pl     [MaxSeats]player
	objs   [MaxObj]object
	nobj   Obj
	permHi Slot              // one past the highest battlefield slot this game has used: perms[permHi:] is all zero
	lstamp [MaxPerm]uint16   // CR 613.7e layer restamp on attach (static_attachtimestamp.go layerTs)
	kwx    [maxKwRepX]kwRepX // until-end-of-turn keyword repeats past the third (grantEOT)
	nkwx   uint8
	ptx    [maxPtX]ptRepX // until-end-of-turn pumps past the int8 range (eotPT)
	nptx   uint8
	occ    slotMask           // occupied battlefield slots
	ctrlM  [MaxSeats]slotMask // slots by controller
	subs   [nSubEvents]slotMask
	statM  slotMask // slots whose card has a static ability
	fxM    slotMask // slots with any record in g.fx (CR 613 plan: beside statM)
	grantM slotMask // statM slots whose static can grant an ability (grantedAbilitiesN)
	legM   slotMask // slots whose card is legendary (legend-rule candidates)

	stack      [MaxStack]stackEntry
	sp         uint8
	pend       [MaxPending]pendingTrig
	npend      uint8
	orderLeft  uint8 // trigger-order picks still owed in the current ordering round
	orderN     uint8 // picks made so far (ordList[:orderN], pending indices)
	orderPlace uint8 // ordered triggers still to push (ordList[:orderPlace])
	ordList    [MaxPending]uint8
	dly        [MaxDelayed]delayed
	ndly       uint8

	res resCtx

	// mulligan round
	mullOrder [MaxSeats]uint8
	mullCur   uint8
	mullKept  [MaxSeats]bool
	mullMull  [MaxSeats]bool // mulliganed this pass
	mullLeft  uint8          // bottom picks left for the current seat
	mullMax   uint8

	// combat
	nAttackers uint8
	fsStep     bool // a first-strike damage step happens this combat
	blockSeq   uint8
	declared   bool  // attackers were declared this combat (the step jump keys on the declaration, not on who is still attacking)
	carry      uint8 // how the next priority round opens: carryNone (active player, no passes), carryAP (active, one pass counted)
	lastPick   Slot

	discardLeft uint8
	legP        uint8    // legend rule: the choosing player
	legCard     CardID   // legend rule: the duplicated legendary card
	legDie      slotMask // legend rule: the creature casualties parked at the ask, applied with the answer
	legSaga     slotMask // legend rule: the Sagas due their sacrifice at the ask, one batch with the answer (CR 704.3)
	divA        Slot     // damage division: the attacker or two-attacker blocker being divided (NoSlot = none in progress)
	divI        uint8    // index (in block order) of the blocker being assigned
	divRem      uint8    // damage left to assign
	divFirst    bool     // the division is for the first-strike damage step
	// fbR[p] is player p's firebending red (CR 702.189a): the red units of
	// a PersistentUntilEndOfCombat AddMana this combat that survive step
	// boundaries until the end-of-combat step ends. Spends are
	// ordinary-first, so the units still floating at a boundary are
	// min(fbR[p], pool[R]) (kw_firebend.go).
	fbR [MaxSeats]uint8
	// rb[p] is player p's restricted mana floating beside the pool, batch by
	// batch in gorge's order (Giada's Angel-only {W}, a Village's {U};
	// gorge's Players[p].RestrictedMana): spent only by a payment its
	// restriction admits, projected as pool mana (mana_restrbatch.go).
	rb [MaxSeats][maxRB]rbatch
	// pm[p][c] is player p's persistent units of colour c (PersistentMana$
	// True, B = 2 on the AddMana): they survive step boundaries and the
	// cleanup step, and become ordinary mana at the turn change. Spends are
	// ordinary-first, so the units floating at a boundary are
	// min(pm[p][c], pool[c]) (mana_persistent.go). pmOn bit p is set
	// while pm[p] is nonzero, so the hot guards test one byte.
	pm   [MaxSeats][kop.NumManaKinds]uint8
	pmOn uint8

	// the last clean board scan of sba (static_sbaskip.go): the touch count it
	// ran at, valid when sbaOK; the rest of its key is g.sbaFP. They fill the
	// padding before chg (State's size is unchanged)
	sbaOK uint8 // 0 none; sbaKeyTouch: the touch count is the key; sbaKeyPrint: the fingerprint too
	sbaT  uint32

	// change marks since SetEpoch (marks.go): one bit per object
	chg    [ObjWords]uint64
	epoch  uint32
	touchN uint32 // object writes so far (marks.go)
	line   uint64 // the line of play (SetLine)
	undos  uint8  // answers taken back (Undos)
	// gorge's held-out cast (CR 733.2, Config.GorgeAltAdd): the card whose
	// cast was undone, how many times in a row with no progress between,
	// and the progress key (touch count, turn, step) after the last undo
	heldObj   Obj
	heldN     uint8
	heldTurn  uint16
	heldStep  kop.Step
	heldTouch uint32
	stamp     [MaxObj]uint8 // per object: the low 8 bits of touchN at its last write

	// Skipped counts decisions elided because they had a single option
	// (with Config.NoElide off), per player: chiefly priority decisions
	// where passing is the only legal action.
	Skipped [MaxSeats]uint32
	nlC     [4]Obj   // Braided Net locks 5..8 (genState.nlA; fills the padding before g; act_lockpermanent.go)
	g       genState // the generated-table extension (gen.go)
	// outCode maps a per-game outside-type code (its index) to the TypeVocab
	// index of the type that owns it, +1 (0 free). act_choosetype.go
	// typeVocabCode allocates one on the first choice of that type, so two
	// sources that chose the same outside type compare equal and two
	// different types differ even when the vocabulary has more outside
	// types than free codes. [256]uint16 is a multiple of 64 bytes, so
	// perms keeps its cache-line phase.
	outCode [256]uint16

	// Decisions counts the decisions answered (SeqNo).
	Decisions uint32

	// payHyb: a trigger's own hybrid PayMana cost's announced faces, one bit
	// per pip from bit 0 (set: the second printed face) under a marker bit
	// above the last one announced (cost_trighybrid.go). Cleared when
	// the ask settles. It sits here, after Decisions, because genState's
	// reserve has no byte left: the byte fills the padding before perms.
	payHyb uint8
}

const (
	cfgNoElide uint8 = 1 << iota
	cfgAutoDivide
	cfgGorgeAltAdd
	cfgGorgeAutopay
	cfgGorgeLowering
	cfgGorgeMustAttack
	cfgGorgeUnpriced
	// cfgCommander: a Commander game (Config.Commander; zone_cmdinit.go).
	cfgCommander

	// cfgGorgeSurface: this game keeps gorge's option surface, not the rules census.
	cfgGorgeSurface = cfgGorgeAutopay | cfgGorgeLowering | cfgGorgeUnpriced
)

// State.cfgx bits.
const (
	// cfgxNoSorcery is transient, never a Config bit: set only while
	// InstantSpeed (value_turnfacts.go) enumerates the priority options with
	// the sorcery-speed permission withheld, and cleared before it returns.
	cfgxNoSorcery uint8 = 1 << iota
	// cfgxGraveDrawn: the table has a Drawn trigger that works from the
	// graveyard (Sneaky Snacker; trig_gravedrawn.go), so a draw
	// fires Drawn with no battlefield subscriber. Set once at Init.
	cfgxGraveDrawn
	// cfgxArrangeRest: Config.ArrangeRestOrder.
	cfgxArrangeRest
	// cfgxShieldPass is transient, never a Config bit: a state-based pass spent a
	// shield counter, so the check runs again (State.sba).
	cfgxShieldPass uint8 = 1 << 7
	// cfgxDestroyHealPass is transient, never a Config bit: a state-based pass
	// applied a ReplDestroyHeal replacement, so the check runs again (a
	// toughness-0 creature still dies after the counters leave).
	cfgxDestroyHealPass uint8 = 1 << 3
)

func (s *State) k() *prep { return prepChunks[s.tbl/tableChunk].Load()[s.tbl%tableChunk] }

// grantEOT grants keywords m until end of turn, counting repeats.
func (p *perm) grantEOT(m kop.KwMask) {
	p.eotKwRep[1] |= p.eotKwRep[0] & m
	p.eotKwRep[0] |= p.eotKw & m
	p.eotKw |= m
}

// kwRepX counts a keyword's until-end-of-turn grants on one permanent
// beyond the three perm.eotKw and perm.eotKwRep hold (Balmor's trample
// from a fourth spell: gorge lists the keyword once per granting effect).
type kwRepX struct {
	sl  Slot
	bit uint8 // the keyword's bit index; | kwSup: grants a loss of all abilities set aside (suppressEOT)
	n   uint8 // grants beyond the third; with kwSup all of the keyword's grants
}

// kwSup marks a kwRepX record holding until-end-of-turn grants suppressed
// by a later loss of all abilities.
const kwSup = 0x40

// maxKwRepX bounds the kwRepX records live at once; a grant past it
// stalls (StallKwRepX). 32 since 2026-11-03 (was 8): Indelicate Healer gives
// every creature its controller has lifelink once per Klingon or Warrior
// entering, so a fourth entry in a turn needs a record per creature.
const maxKwRepX = 32

// ptRepX holds the true until-end-of-turn pump of a permanent whose pump
// left the int8 range perm.eotP and perm.eotT hold (Jumbo Cactuar's
// +9999/+0 on attack). The perm keeps the amount saturated to int8, so its
// sign is right wherever it is read without the table.
type ptRepX struct {
	sl   Slot
	p, t int16
}

// maxPtX bounds the ptRepX records live at once; a pump that needs a fifth
// stalls (StallPT).
const maxPtX = 4

// eotPT is sl's until-end-of-turn power and toughness pump.
func (s *State) eotPT(sl Slot) (int, int) {
	if s.nptx != 0 {
		for i := 0; i < int(s.nptx); i++ {
			if r := &s.ptx[i]; r.sl == sl {
				return int(r.p), int(r.t)
			}
		}
	}
	p := &s.perms[sl]
	return int(p.eotP), int(p.eotT)
}

// setEotPT stores sl's pump (each within int16): inline when both fit int8,
// else saturated inline with the true amounts in ptx. A full table stalls
// (StallPT) and changes nothing.
func (s *State) setEotPT(sl Slot, np, nt int) bool {
	p := &s.perms[sl]
	i := 0
	for i < int(s.nptx) && s.ptx[i].sl != sl {
		i++
	}
	if np == int(int8(np)) && nt == int(int8(nt)) {
		if i < int(s.nptx) {
			s.dropPtX(sl)
		}
		p.eotP, p.eotT = int8(np), int8(nt)
		return true
	}
	if i == int(s.nptx) {
		if i >= maxPtX {
			s.capFull(StallPT)
			return false
		}
		s.nptx++
	}
	s.ptx[i] = ptRepX{sl: sl, p: int16(np), t: int16(nt)}
	p.eotP, p.eotT = int8(max(min(np, math.MaxInt8), math.MinInt8)), int8(max(min(nt, math.MaxInt8), math.MinInt8))
	return true
}

// dropPtX forgets sl's out-of-range pump (it left, or the turn ended).
func (s *State) dropPtX(sl Slot) {
	n := 0
	for i := 0; i < int(s.nptx); i++ {
		if s.ptx[i].sl != sl {
			s.ptx[n] = s.ptx[i]
			n++
		}
	}
	for i := n; i < int(s.nptx); i++ {
		s.ptx[i] = ptRepX{}
	}
	s.nptx = uint8(n)
}

// grantEOT grants the permanent at sl keywords m until end of turn,
// counting the repeats past the third in kwx.
func (s *State) grantEOT(sl Slot, m kop.KwMask) {
	p := &s.perms[sl]
	for x := p.eotKwRep[1] & m; x != 0; x &= x - 1 {
		bit := uint8(bits.TrailingZeros32(uint32(x)))
		found := false
		for i := 0; i < int(s.nkwx); i++ {
			if r := &s.kwx[i]; r.sl == sl && r.bit == bit {
				if r.n < 255 {
					r.n++
				}
				found = true
				break
			}
		}
		if !found && int(s.nkwx) < maxKwRepX {
			s.kwx[s.nkwx] = kwRepX{sl: sl, bit: bit, n: 1}
			s.nkwx++
		} else if !found {
			s.capFull(StallKwRepX)
		}
	}
	p.grantEOT(m)
}

// dropKwRepX forgets the counted repeats on sl (it left, or lost its
// grants).
func (s *State) dropKwRepX(sl Slot) {
	n := 0
	for i := 0; i < int(s.nkwx); i++ {
		if s.kwx[i].sl != sl {
			s.kwx[n] = s.kwx[i]
			n++
		}
	}
	for i := n; i < int(s.nkwx); i++ {
		s.kwx[i] = kwRepX{}
	}
	s.nkwx = uint8(n)
}

// DebugCtrs is a test hook: the object's counter kinds and counts.
func (s *State) DebugCtrs(o Obj) (int, []int, []int) {
	if o >= MaxObj || s.objs[o].slot == NoSlot {
		return 0, nil, nil
	}
	p := &s.perms[s.objs[o].slot]
	var ks, ns []int
	for i := range p.ck {
		if p.cn[i] != 0 {
			ks = append(ks, int(p.ck[i]))
			ns = append(ns, int(p.cn[i]))
		}
	}
	for _, e := range s.ctrOverOf(o, 0) {
		if e.n != 0 {
			ks = append(ks, int(e.kind))
			ns = append(ns, int(e.n))
		}
	}
	return int(p.p1), ks, ns
}
