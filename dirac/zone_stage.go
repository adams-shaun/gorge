package dirac

import (
	"fmt"
	"sort"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// stage.go: NewFromWorld builds a State at a priority decision from a
// World, a plain description of one position (every card with its zone and
// place, life, turn and step). It reads nothing but the World. Where the
// World comes from is the caller's affair: dirac/shadow builds it from a
// seat's observation plus the hidden zones a determinization dealt, and
// TestStageMatchesReplay builds it from gorge's true position, the verifier.
//
// A position outside the minimal scope returns ErrUnstageable and no State:
// never a partial or guessed one.

// Reasons an ErrUnstageable carries from NewFromWorld. dirac/shadow adds
// its own for what only an observation shows (shadow.Reasons).
const (
	ReasonTable       = "table"         // no table
	ReasonSeats       = "seats"         // not a game of 2..MaxSeats seats
	ReasonLayout      = "object-layout" // objects not grouped by owner, or a deck outside 7..MaxDeck
	ReasonCard        = "card"          // a card outside lands, creatures and combat keywords
	ReasonZone        = "zone"          // an object outside library, hand, battlefield, graveyard
	ReasonControl     = "control"       // a permanent under another seat's control
	ReasonPlace       = "place"         // zone positions not a dense 0..n-1 order
	ReasonStep        = "step"          // not a quiet priority step: upkeep, draw, main, end
	ReasonPriority    = "priority"      // a priority holder the step cannot have
	ReasonTurn        = "turn"          // turn 0, or an active seat out of range
	ReasonLife        = "life"          // a life total outside the int16 range
	ReasonBattlefield = "battlefield"   // more permanents than battlefield slots
	ReasonToken       = "token"         // a token the table cannot rebuild: not on the battlefield, not a plain creature token, or out of the layout
	ReasonAttach      = "attachment"    // an attachment the staging cannot rebuild: not an Aura or Equipment, off the battlefield, or on a non-creature
	ReasonCombat      = "combat"        // a combat mark or pose the staging cannot rebuild (zone_stagecombat.go)
	ReasonCommander   = "commander"     // a commander or a command zone the staging cannot rebuild (zone_stagecmd.go)
)

// UnstageableReasons lists every reason NewFromWorld returns.
var UnstageableReasons = []string{
	ReasonTable, ReasonSeats, ReasonLayout, ReasonCard, ReasonZone, ReasonControl,
	ReasonPlace, ReasonStep, ReasonPriority, ReasonTurn, ReasonLife, ReasonBattlefield,
	ReasonToken, ReasonAttach, ReasonStackEntry, ReasonCombat, ReasonCommander,
}

// ErrUnstageable is the error of a position that cannot be staged.
type ErrUnstageable struct {
	Reason string
	Detail string // what exactly, for a log; never compared
}

func (e ErrUnstageable) Error() string {
	if e.Detail == "" {
		return "unstageable: " + e.Reason
	}
	return "unstageable: " + e.Reason + ": " + e.Detail
}

func unstageable(reason, format string, a ...any) error {
	return ErrUnstageable{Reason: reason, Detail: fmt.Sprintf(format, a...)}
}

// WorldObj is one card of a World.
type WorldObj struct {
	Card  CardID
	Owner uint8
	Zone  kop.Zone // ZoneLibrary, ZoneHand, ZoneBattlefield or ZoneGraveyard
	// Pos is the card's place in its zone: the library from the top (0 is the
	// next draw), the hand and graveyard in order, the battlefield in the
	// controller's list order (oldest first). Each zone's places, per owner
	// (per controller on the battlefield), are exactly 0..n-1.
	Pos    uint8
	Ctrl   uint8 // the controller of a permanent; its owner's seat otherwise
	Tapped bool
	// Sick is the summoning-sick mark: the permanent came under its
	// controller's control since that seat's most recent turn began.
	Sick bool
	// Damage is the damage marked on a battlefield creature (CR 120.3e) and
	// P1P1 its +1/+1 counters. Both are battlefield state: dirac keeps neither
	// off the battlefield, and a card there carries none. A creature with
	// Damage above zero was dealt damage this turn (WasDealtDamageThisTurn).
	Damage uint8
	P1P1   uint16
	// Token marks a token: a plain creature token (a CardDef with TToken) on
	// the battlefield. Tokens come after every card of the decks in Objs, in
	// the order they were made, so their object indices are the ones the
	// game gave them; their owner is their controller.
	Token bool
	// Attached marks an Aura or Equipment on the battlefield attached to the
	// creature at object index Host (any seat's). An Aura on the battlefield
	// is always attached; an Equipment may not be.
	Attached bool
	Host     Obj
	// Combat marks of a battlefield creature (zone_stagecombat.go). Attacking
	// is set on an attacker, with AtkSeat the player it attacks. Blocked marks
	// an attacker that has blockers; a blocker has Blocking set, Blocks the
	// object index of its attacker and BlockOrd its place (from 1) in that
	// attacker's block order.
	Attacking bool
	AtkSeat   uint8
	Blocked   bool
	Blocking  bool
	Blocks    Obj
	BlockOrd  uint8
}

// World is one position, staged at a priority decision.
type World struct {
	Tbl  *kop.Table
	Cfg  Config // as the game is played; Mulligans is ignored (the round is over)
	Seed uint64 // seeds the staged game's own future chance
	// Objs are the game's cards by Obj index: seat 0's deck, then seat 1's,
	// as Init lays them out. A caller that does not know a hidden card's
	// object index deals them any consistent way.
	Objs []WorldObj
	// Stack is the stack, bottom to top (see zone_stagestack.go).
	Stack       []WorldStack
	NSeat       uint8 // 2..MaxSeats
	Turn        uint16
	Step        kop.Step
	Active      uint8
	Prio        uint8 // the priority holder
	Life        [MaxSeats]int32
	LandsPlayed [MaxSeats]uint8
	// Pose is the decision pending: a priority round (zero), or a declare
	// attackers or declare blockers pose with no pick made yet.
	Pose Pose
	// BlkDef is the defender asked at PoseBlockers when NSeat > 2 (read nowhere
	// at two seats: the defender is the active seat's one opponent).
	BlkDef uint8
	// Lost marks a seat that has left the game (CR 800.4a): it owns only
	// ceased objects (Zone kop.ZoneNone), every card of its deck, and it is out
	// of every rotation. At least two seats are not lost; the active seat, the
	// priority holder, an attacker's target and BlkDef are none of them lost.
	Lost [MaxSeats]bool
	// Commander is a Commander game (CR 903, 40 life): Cmd[p] is seat p's
	// commander, an object p owns, in the command zone or any zone staging
	// admits (NoObj none), CmdCasts[p] its casts from the command zone (CR
	// 903.8) and CmdDamage[q][p] the combat damage player q has taken from it
	// (CR 903.10a; Projection.CmdDmg's layout). With Commander off every Cmd
	// field is zero or NoObj. See zone_stagecmd.go.
	Commander bool
	Cmd       [MaxSeats]Obj
	CmdCasts  [MaxSeats]uint8
	CmdDamage [MaxSeats][MaxSeats]uint8
}

// stageSteps are the steps a position can be staged in: a priority round or
// a combat pose before combat damage (zone_stagecombat.go).
var stageSteps = [...]kop.Step{kop.StepUpkeep, kop.StepDraw, kop.StepMain1, kop.StepBeginCombat,
	kop.StepAttackers, kop.StepBlockers, kop.StepMain2, kop.StepEnd}

// combatKeywords are the keywords a staged creature may carry: they change
// combat and nothing else.
const combatKeywords = kop.KwFlying | kop.KwReach | kop.KwFirstStrike | kop.KwDoubleStrike |
	kop.KwDeathtouch | kop.KwLifelink | kop.KwTrample | kop.KwVigilance | kop.KwHaste |
	kop.KwMenace | kop.KwDefender

// stageableCard reports why def is outside the staging scope, "" when it is
// a basic-style land (mana abilities only), a creature with combat keywords
// and at most a self +1/+1 counter ability, a plain creature token (token),
// an Aura or Equipment of the shape zone_stageattach.go admits, or a spell
// that only makes tokens.
func stageableCard(def *kop.CardDef, token bool) string {
	if def.Face != 0 || def.FaceMode != 0 || len(def.Faces) != 0 {
		return "multi-face"
	}
	if token != (def.Types&kop.TToken != 0) {
		return "token"
	}
	switch {
	case token:
		return stageableToken(def)
	case def.Types&(kop.TAura|kop.TEquipment) != 0:
		return stageableAttachment(def)
	case def.Types&(kop.TSorcery|kop.TInstant) != 0:
		return stageableTokenSpell(def)
	}
	return stageableBody(def)
}

// stageableBody is stageableCard for a land or a creature.
func stageableBody(def *kop.CardDef) string {
	const allowed = kop.TLand | kop.TBasic | kop.TCreature | kop.TLegendary
	// legendary: a creature only (the legend rule is a state-based action
	// staging never performs)
	if def.Types&^allowed != 0 || def.Types&(kop.TLand|kop.TCreature) == 0 || def.Types&kop.TLegendary != 0 && def.Types&kop.TCreature == 0 {
		return "type"
	}
	if def.Types&kop.TLand != 0 && def.Types&kop.TCreature != 0 {
		return "land creature"
	}
	if def.Keywords&^combatKeywords != 0 {
		return "keyword"
	}
	if def.Types&kop.TLand != 0 {
		for i := range def.Abilities {
			if def.Abilities[i].Kind != kop.AbMana {
				return "land ability"
			}
		}
		return ""
	}
	for i := range def.Abilities {
		if !selfCounterAbility(&def.Abilities[i]) {
			return "ability"
		}
	}
	if def.Enchant != (kop.TargetSpec{}) || def.EquipCost != (kop.ManaCost{}) || def.Leyline || def.Dredge != 0 || def.UmbraArmor {
		return "ability"
	}
	return ""
}

// selfCounterAbility reports whether ab is an activated ability whose only
// effect is a +1/+1 counter on its own source ("{1}, sacrifice another
// creature: put a +1/+1 counter on this"). It keeps no per-permanent state
// beyond the counter itself: not once per turn, no target, no choice.
func selfCounterAbility(ab *kop.Ability) bool {
	if ab.Kind != kop.AbActivated || ab.NTargets != 0 || ab.NModes != 0 || ab.AFlags != 0 || len(ab.Prog) != 1 {
		return false
	}
	in := &ab.Prog[0]
	return in.Op == kop.OpPutCounter && in.Obj == kop.RefSelf && in.C == int16(kop.CtrP1P1) && in.A == kop.Const(1)
}

// NewFromWorld returns the game at w's position, at the priority decision
// Step poses next. It is an ErrUnstageable when w is outside the minimal
// scope: two seats; lands and creatures with combat keywords and no other
// ability; the library, hand, graveyard and battlefield; tapped and
// summoning-sick marks; a quiet priority step. The result is a fresh State;
// its hidden zones are w's assignment, not anything it was dealt.
func NewFromWorld(w *World) (*State, error) {
	s := new(State)
	if err := s.initFromWorld(w); err != nil {
		return nil, err
	}
	return s, nil
}

// stageZone is the index validate counts a zone of a World object under.
func stageZone(z kop.Zone) (int, bool) {
	switch z {
	case kop.ZoneLibrary:
		return 0, true
	case kop.ZoneHand:
		return 1, true
	case kop.ZoneBattlefield:
		return 2, true
	case kop.ZoneGraveyard:
		return 3, true
	case kop.ZoneStack:
		return 4, true
	case kop.ZoneCommand:
		return 5, true // a commander only (validateCommandObj)
	}
	return 0, false
}

func (w *World) validate() error {
	if w.Tbl == nil {
		return unstageable(ReasonTable, "no table")
	}
	if err := w.validateSeats(); err != nil {
		return err
	}
	if err := w.validateLost(); err != nil {
		return err
	}
	bf, err := w.validateObjs()
	if err != nil {
		return err
	}
	if err := w.validateStack(); err != nil {
		return err
	}
	if err := w.validateCombat(); err != nil {
		return err
	}
	if bf > MaxPerm {
		return unstageable(ReasonBattlefield, "%d permanents", bf)
	}
	if err := w.validateCommanders(); err != nil {
		return err
	}
	return w.validatePlaces()
}

// validateSeats checks the seat count, the turn, the step, the priority holder
// and the life totals.
func (w *World) validateSeats() error {
	if w.NSeat < 2 || w.NSeat > MaxSeats {
		return unstageable(ReasonSeats, "%d seats", w.NSeat)
	}
	if w.Turn == 0 || w.Active >= w.NSeat {
		return unstageable(ReasonTurn, "turn %d active %d", w.Turn, w.Active)
	}
	ok := false
	for _, st := range stageSteps {
		ok = ok || st == w.Step
	}
	if !ok {
		return unstageable(ReasonStep, "step %d", w.Step)
	}
	// Prio: a round starts at the active seat and each pass moves to the next
	// seat, so any holder below NSeat is a round some passes in (roundPasses).
	if w.Prio >= w.NSeat {
		return unstageable(ReasonPriority, "holder %d", w.Prio)
	}
	for p := 0; p < int(w.NSeat); p++ {
		if w.Life[p] != int32(int16(w.Life[p])) {
			return unstageable(ReasonLife, "seat %d life %d", p, w.Life[p])
		}
	}
	return nil
}

// validateLost checks the seats that have left: two or more remain, and the
// active seat, the priority holder, each attacker's target and the blockers
// pose's defender are among them. A game whose active seat left mid-turn is
// refused, not staged.
func (w *World) validateLost() error {
	alive := 0
	for p := uint8(0); p < MaxSeats; p++ {
		switch {
		case !w.Lost[p]:
			if p < w.NSeat {
				alive++
			}
		case p >= w.NSeat:
			return unstageable(ReasonSeats, "seat %d is lost and out of range", p)
		case w.Commander && w.Cmd[p] != NoObj:
			return unstageable(ReasonCommander, "seat %d has left and keeps a commander", p)
		}
	}
	switch {
	case alive < 2:
		return unstageable(ReasonSeats, "%d seats not lost", alive)
	case w.Lost[w.Active]:
		return unstageable(ReasonSeats, "the active seat %d has left", w.Active)
	case w.Lost[w.Prio]:
		return unstageable(ReasonSeats, "the priority holder %d has left", w.Prio)
	case w.Pose == PoseBlockers && w.NSeat > 2 && w.BlkDef < w.NSeat && w.Lost[w.BlkDef]:
		return unstageable(ReasonSeats, "the defender %d has left", w.BlkDef)
	}
	for i := range w.Objs {
		if o := &w.Objs[i]; o.Attacking && o.AtkSeat < w.NSeat && w.Lost[o.AtkSeat] {
			return unstageable(ReasonSeats, "object %d attacks seat %d, which has left", i, o.AtkSeat)
		}
	}
	return nil
}

// validateObjs checks every object of w and returns the battlefield total.
func (w *World) validateObjs() (int, error) {
	var n [MaxSeats]int
	bf := 0
	prev, firstToken := uint8(0), -1
	if len(w.Objs) > MaxObj {
		return 0, unstageable(ReasonLayout, "%d objects", len(w.Objs))
	}
	for i := range w.Objs {
		o := &w.Objs[i]
		if o.Owner >= w.NSeat || (o.Owner < prev && !o.Token) {
			return 0, unstageable(ReasonLayout, "object %d owner %d", i, o.Owner)
		}
		if !o.Token {
			prev = o.Owner
			n[o.Owner]++
		}
		if int(o.Card) >= len(w.Tbl.Cards) {
			return 0, unstageable(ReasonCard, "object %d card %d outside the table", i, o.Card)
		}
		if why := stageableCard(&w.Tbl.Cards[o.Card], o.Token); why != "" {
			if o.Token || why == "token" {
				return 0, unstageable(ReasonToken, "%s: %s", w.Tbl.Cards[o.Card].Name, why)
			}
			return 0, unstageable(ReasonCard, "%s: %s", w.Tbl.Cards[o.Card].Name, why)
		}
		if err := w.checkTokenAttach(i, &firstToken); err != nil {
			return 0, err
		}
		if err := w.validateObjZone(i, o); err != nil {
			return 0, err
		}
		if o.Ctrl >= w.NSeat || o.Ctrl != o.Owner {
			return 0, unstageable(ReasonControl, "object %d owner %d controller %d", i, o.Owner, o.Ctrl)
		}
		if o.Zone == kop.ZoneCommand {
			if err := w.validateCommandObj(i); err != nil {
				return 0, err
			}
		}
		if o.Zone != kop.ZoneBattlefield && (o.Tapped || o.Sick || o.Damage != 0 || o.P1P1 != 0 || o.combatMarked()) {
			return 0, unstageable(ReasonZone, "object %d off the battlefield carries a mark", i)
		}
		if o.Zone == kop.ZoneBattlefield {
			bf++
		}
	}
	for p := 0; p < int(w.NSeat); p++ {
		if n[p] < openingHand || n[p] > MaxDeck {
			return 0, unstageable(ReasonLayout, "seat %d has %d cards", p, n[p])
		}
	}
	return bf, nil
}

// validateObjZone checks the zone of object i. A seat that has left owns only
// ceased objects (ZoneNone, no token: a ceased token's gap in the layout is not
// staged), and nobody else owns one.
func (w *World) validateObjZone(i int, o *WorldObj) error {
	lost := w.Lost[o.Owner]
	switch {
	case o.Zone == kop.ZoneNone && lost && o.Token:
		return unstageable(ReasonToken, "object %d: a ceased token", i)
	case o.Zone == kop.ZoneNone && lost:
		return nil
	case lost:
		return unstageable(ReasonZone, "object %d in zone %d of seat %d, which has left", i, o.Zone, o.Owner)
	}
	if o.Token && o.Zone == kop.ZoneNone {
		return nil // a ceased token (CR 111.7): checkTokenAttach vetted it
	}
	if _, ok := stageZone(o.Zone); !ok {
		return unstageable(ReasonZone, "object %d in zone %d", i, o.Zone)
	}
	return nil
}

// validatePlaces requires every zone's places to be exactly 0..n-1.
func (w *World) validatePlaces() error {
	for p := uint8(0); p < w.NSeat; p++ {
		for _, z := range []kop.Zone{kop.ZoneLibrary, kop.ZoneHand, kop.ZoneBattlefield, kop.ZoneGraveyard} {
			var seen [MaxDeck]bool
			m := 0
			for i := range w.Objs {
				if o := &w.Objs[i]; o.Owner == p && o.Zone == z {
					if int(o.Pos) >= MaxDeck || seen[o.Pos] {
						return unstageable(ReasonPlace, "seat %d zone %d place %d", p, z, o.Pos)
					}
					seen[o.Pos] = true
					m++
				}
			}
			for q := 0; q < m; q++ {
				if !seen[q] {
					return unstageable(ReasonPlace, "seat %d zone %d has no place %d of %d", p, z, q, m)
				}
			}
		}
	}
	return nil
}

func (s *State) initFromWorld(w *World) error {
	if err := w.validate(); err != nil {
		return err
	}
	var decks [MaxSeats][]CardID
	for i := range w.Objs {
		if !w.Objs[i].Token {
			decks[w.Objs[i].Owner] = append(decks[w.Objs[i].Owner], w.Objs[i].Card)
		}
	}
	cfg := w.Cfg
	cfg.Mulligans = 0
	cfg.Commander = w.Commander
	cfg.Commanders = w.cmdConfig()
	cfg.SideN = [MaxSeats]uint8{}
	s.InitN(w.Tbl, decks, w.NSeat, w.Seed, cfg)
	if s.over || s.g.stalled != 0 {
		return unstageable(ReasonLayout, "the table stalls at its start")
	}
	k := s.k()
	if k.gen != nil && k.gen.crimeOn {
		for i := range w.Stack {
			if w.Stack[i].NT != 0 {
				return unstageable(ReasonStackEntry, "the crime ledger is not in a view")
			}
		}
	}
	// Init dealt every seat an opening hand; take them all back and place
	// each object as w says.
	for p := range s.pl {
		pl := &s.pl[p]
		pl.libLo, pl.libHi, pl.handN, pl.graveN, pl.exileN = 0, 0, 0, 0, 0
		pl.mark, pl.drawnTurn = 0, 0
		pl.landsPlayed = 0
	}
	for i := range w.Objs {
		o := &w.Objs[i]
		ob := &s.objs[i]
		ob.of &^= ofInHand
		ob.zone = kop.ZoneNone
		s.touch(Obj(i))
		_ = o
	}
	if !s.stageTokens(w) {
		return unstageable(ReasonToken, "the tokens do not take the objects after the decks")
	}
	// The order each zone is filled in is its Pos order.
	idx := make([]int, len(w.Objs))
	for i := range idx {
		idx[i] = i
	}
	sort.SliceStable(idx, func(a, b int) bool { return w.Objs[idx[a]].Pos < w.Objs[idx[b]].Pos })
	for _, i := range idx {
		o := &w.Objs[i]
		ob := &s.objs[i]
		pl := &s.pl[o.Owner]
		ob.ver++
		switch o.Zone {
		case kop.ZoneLibrary:
			ob.zone = kop.ZoneLibrary
			pl.libBottom(Obj(i))
		case kop.ZoneHand:
			ob.zone = kop.ZoneHand
			ob.of |= ofInHand
			pl.hand[pl.handN] = Obj(i)
			pl.handN++
		case kop.ZoneGraveyard:
			ob.zone = kop.ZoneGraveyard
			ob.of |= ofInHand
			pl.grave[pl.graveN] = Obj(i)
			pl.graveN++
		case kop.ZoneCommand:
			ob.zone = kop.ZoneCommand // pl.cmd is seated: InitN ran cmdGenesis
			s.noteCommandZone(Obj(i))
		}
	}
	// Permanents enter owner by owner in list order: perm.ts is the order of
	// the controller's battlefield list.
	for p := uint8(0); p < w.NSeat; p++ {
		for _, i := range idx {
			o := &w.Objs[i]
			if o.Zone != kop.ZoneBattlefield || o.Owner != p {
				continue
			}
			if !o.Token {
				s.objs[i].of |= ofInHand
			}
			sl := s.enterNF(k, Obj(i), o.Ctrl, o.Tapped, kop.ZoneNone, 0)
			if !o.Sick {
				s.perms[sl].flags &^= pfSick
			}
			s.stageMarks(sl, o)
			s.touchSl(sl)
		}
	}
	if w.Commander {
		s.stageCommanders(w)
	}
	s.stageAttached(k, w)
	s.stageStack(k, w)
	s.stageCombat(k, w)
	for p := 0; p < int(w.NSeat); p++ {
		s.pl[p].life = int16(w.Life[p])
		// a seat that has left: its objects are ceased already (the reset above)
		// and it enters no zone. Not lose: that is the play-time path, and the
		// position is after it.
		s.pl[p].lost = w.Lost[p]
	}
	s.pl[w.Active].landsPlayed = w.LandsPlayed[w.Active]
	s.turn = w.Turn
	s.active = w.Active
	s.g.exBase = w.Active // a staged position is on a normal turn: the rotation resumes after its holder
	// the seat that took turn 1, when nobody took an extra turn
	s.startPl = uint8((int(w.Active) + int(w.NSeat) - int(w.Turn-1)%int(w.NSeat)) % int(w.NSeat))
	s.step = w.Step
	s.carry = carryNone
	s.ctl = ctlPriority
	s.prio = w.Prio
	s.passes = w.roundPasses()
	s.stagePose(w)
	s.mullMax = 0
	return nil
}

// stageMarks sets the damage and +1/+1 counters of the permanent at sl.
func (s *State) stageMarks(sl Slot, o *WorldObj) {
	p := &s.perms[sl]
	p.damage = o.Damage
	p.p1 = o.P1P1
	if o.Damage > 0 {
		s.g.dmgTurn.set(p.obj) // dealt damage this turn, as dmgPerm records it
	}
}
