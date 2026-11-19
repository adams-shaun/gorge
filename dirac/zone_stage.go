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
	ReasonSeats       = "seats"         // not a two-seat game
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
)

// UnstageableReasons lists every reason NewFromWorld returns.
var UnstageableReasons = []string{
	ReasonTable, ReasonSeats, ReasonLayout, ReasonCard, ReasonZone, ReasonControl,
	ReasonPlace, ReasonStep, ReasonPriority, ReasonTurn, ReasonLife, ReasonBattlefield,
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
}

// World is one position, staged at a priority decision with an empty stack.
type World struct {
	Tbl  *kop.Table
	Cfg  Config // as the game is played; Mulligans is ignored (the round is over)
	Seed uint64 // seeds the staged game's own future chance
	// Objs are the game's cards by Obj index: seat 0's deck, then seat 1's,
	// as Init lays them out. A caller that does not know a hidden card's
	// object index deals them any consistent way.
	Objs        []WorldObj
	NSeat       uint8 // 2
	Turn        uint16
	Step        kop.Step
	Active      uint8
	Prio        uint8 // the priority holder
	Life        [2]int32
	LandsPlayed [2]uint8
}

// stageSteps are the steps a position can be staged in: a priority round
// with no combat in flight.
var stageSteps = [...]kop.Step{kop.StepUpkeep, kop.StepDraw, kop.StepMain1, kop.StepMain2, kop.StepEnd}

// combatKeywords are the keywords a staged creature may carry: they change
// combat and nothing else.
const combatKeywords = kop.KwFlying | kop.KwReach | kop.KwFirstStrike | kop.KwDoubleStrike |
	kop.KwDeathtouch | kop.KwLifelink | kop.KwTrample | kop.KwVigilance | kop.KwHaste |
	kop.KwMenace | kop.KwDefender

// stageableCard reports why def is outside the minimal scope, "" when it is a
// basic-style land (mana abilities only) or a creature with combat keywords
// and no abilities.
func stageableCard(def *kop.CardDef) string {
	if def.Face != 0 || def.FaceMode != 0 || len(def.Faces) != 0 {
		return "multi-face"
	}
	const allowed = kop.TLand | kop.TBasic | kop.TCreature
	if def.Types&^allowed != 0 || def.Types&(kop.TLand|kop.TCreature) == 0 {
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
	if len(def.Abilities) != 0 {
		return "ability"
	}
	if def.Enchant != (kop.TargetSpec{}) || def.EquipCost != (kop.ManaCost{}) || def.Leyline || def.Dredge != 0 || def.UmbraArmor {
		return "ability"
	}
	return ""
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

func (w *World) validate() error {
	if w.Tbl == nil {
		return unstageable(ReasonTable, "no table")
	}
	if w.NSeat != 2 {
		return unstageable(ReasonSeats, "%d seats", w.NSeat)
	}
	if w.Turn == 0 || w.Active >= 2 {
		return unstageable(ReasonTurn, "turn %d active %d", w.Turn, w.Active)
	}
	ok := false
	for _, st := range stageSteps {
		ok = ok || st == w.Step
	}
	if !ok {
		return unstageable(ReasonStep, "step %d", w.Step)
	}
	// Prio: the active player holds a round that has just opened; the other
	// holds it after one pass. Nothing else is a quiet round.
	if w.Prio >= 2 {
		return unstageable(ReasonPriority, "holder %d", w.Prio)
	}
	for p := range w.Life {
		if w.Life[p] != int32(int16(w.Life[p])) {
			return unstageable(ReasonLife, "seat %d life %d", p, w.Life[p])
		}
	}
	var n [2]int
	var count [2][5][2]int // owner, zone (lib, hand, bf, grave), ctrl
	prev := uint8(0)
	for i := range w.Objs {
		o := &w.Objs[i]
		if o.Owner >= 2 || o.Owner < prev {
			return unstageable(ReasonLayout, "object %d owner %d", i, o.Owner)
		}
		prev = o.Owner
		n[o.Owner]++
		if int(o.Card) >= len(w.Tbl.Cards) {
			return unstageable(ReasonCard, "object %d card %d outside the table", i, o.Card)
		}
		if why := stageableCard(&w.Tbl.Cards[o.Card]); why != "" {
			return unstageable(ReasonCard, "%s: %s", w.Tbl.Cards[o.Card].Name, why)
		}
		z := 0
		switch o.Zone {
		case kop.ZoneLibrary:
			z = 0
		case kop.ZoneHand:
			z = 1
		case kop.ZoneBattlefield:
			z = 2
		case kop.ZoneGraveyard:
			z = 3
		default:
			return unstageable(ReasonZone, "object %d in zone %d", i, o.Zone)
		}
		if o.Ctrl >= 2 || (o.Zone == kop.ZoneBattlefield && o.Ctrl != o.Owner) || (o.Zone != kop.ZoneBattlefield && o.Ctrl != o.Owner) {
			return unstageable(ReasonControl, "object %d owner %d controller %d", i, o.Owner, o.Ctrl)
		}
		if o.Zone != kop.ZoneBattlefield && (o.Tapped || o.Sick) {
			return unstageable(ReasonZone, "object %d off the battlefield carries a mark", i)
		}
		count[o.Owner][z][0]++
	}
	for p := range w.Life {
		if n[p] < openingHand || n[p] > MaxDeck {
			return unstageable(ReasonLayout, "seat %d has %d cards", p, n[p])
		}
	}
	if count[0][2][0]+count[1][2][0] > MaxPerm {
		return unstageable(ReasonBattlefield, "%d permanents", count[0][2][0]+count[1][2][0])
	}
	// every zone's places are exactly 0..n-1
	for p := uint8(0); p < 2; p++ {
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
		decks[w.Objs[i].Owner] = append(decks[w.Objs[i].Owner], w.Objs[i].Card)
	}
	cfg := w.Cfg
	cfg.Mulligans = 0
	cfg.Commander = false
	cfg.SideN = [MaxSeats]uint8{}
	s.InitN(w.Tbl, decks, 2, w.Seed, cfg)
	if s.over || s.g.stalled != 0 {
		return unstageable(ReasonLayout, "the table stalls at its start")
	}
	k := s.k()
	// Init dealt every seat an opening hand; take them all back and place
	// each object as w says.
	for p := range s.pl {
		pl := &s.pl[p]
		pl.libLo, pl.libHi, pl.handN, pl.graveN, pl.exileN = 0, 0, 0, 0, 0
		pl.drewEmpty, pl.drawnTurn = false, 0
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
		}
	}
	// Permanents enter owner by owner in list order: perm.ts is the order of
	// the controller's battlefield list.
	for p := uint8(0); p < 2; p++ {
		for _, i := range idx {
			o := &w.Objs[i]
			if o.Zone != kop.ZoneBattlefield || o.Owner != p {
				continue
			}
			s.objs[i].of |= ofInHand
			sl := s.enterNF(k, Obj(i), o.Ctrl, o.Tapped, kop.ZoneNone, 0)
			if !o.Sick {
				s.perms[sl].flags &^= pfSick
			}
			s.touchSl(sl)
		}
	}
	for p := range w.Life {
		s.pl[p].life = int16(w.Life[p])
	}
	s.pl[w.Active].landsPlayed = w.LandsPlayed[w.Active]
	s.turn = w.Turn
	s.active = w.Active
	// the starting player took the odd turns
	s.startPl = w.Active
	if w.Turn%2 == 0 {
		s.startPl = s.nextSeat(w.Active)
	}
	s.step = w.Step
	s.carry = carryNone
	s.ctl = ctlPriority
	s.prio = w.Prio
	s.passes = 0
	if w.Prio != w.Active {
		s.passes = 1
	}
	s.mullMax = 0
	return nil
}
