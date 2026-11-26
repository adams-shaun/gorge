package dirac

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// Planechase object space (CR 901): each seat's planar deck is a cold,
// pointer-free record of objects in ZonePlanarDeck, top-first. The top plane
// is face up; every other card is face down. The face-up plane's printed
// command-zone triggers and statics function from there, the same synthetic
// scan gorge's rules/planar.go uses (the ordinary battlefield walk never
// visits the planar deck).
//
// The record is a cold region because State's layout is pinned and a
// Planechase game is genesis configuration, not a table property: every
// table carries the (zeroed) record, and a game that deals no planar deck
// leaves it empty.

// MaxPlane is the most cards a seat's planar deck may hold (CR 901.1: a
// Planechase deck is ten cards; a game may also deal a partial deck).
const MaxPlane = 10

// planeSeat is one table's planar-deck records, indexed by seat. deck[p][0]
// is the face-up current plane; deck[p][1:] are face down in order. up[p]
// is the face-up mask, bit j set when deck[p][j] is face up: gorge keeps
// Object.FaceDown on the card, so a card the dig moved to the deck's bottom
// is face up THERE (its same-zone MoveZone fold clears FaceDown) until the
// next shuffle or planeswalk resets every card, and a position rule ("only
// the head is face up") cannot say that.
type planeSeat struct {
	deck [MaxSeats][MaxPlane]Obj
	up   [MaxSeats]uint16
	n    [MaxSeats]uint8
}

// coldPlane is the planar-deck record. Its length is set explicitly in
// prepare (one record per table, in the planechase layout), like the partner
// record.
var coldPlane = newCold[planeSeat]("plane", nil)

// anyPlanar reports whether cfg starts a Planechase game: the config bit, or a
// seat with a nonempty planar deck (the deck is the actual genesis input).
func anyPlanar(cfg *Config) bool {
	if cfg.Planechase {
		return true
	}
	for p := range cfg.PlanarN {
		if cfg.PlanarN[p] > 0 {
			return true
		}
	}
	return false
}

// planeCold is s's planar-deck record, nil for a State never Init'd.
func (s *State) planeCold() *planeSeat {
	if c := coldPlane.of(s); c != nil {
		return &c[0]
	}
	return nil
}

// initPlanarSeat creates seat p's planar-deck objects and records their
// order. They follow the seat's library objects, as gorge's genesis deal
// numbers them. The order is shuffled in Init, before the seat's commander is
// seated and its library shuffled.
func (s *State) initPlanarSeat(p uint8, cfg *Config) {
	ps := s.planeCold()
	if ps == nil {
		return
	}
	n := int(cfg.PlanarN[p])
	if n > MaxPlane {
		panic("kernel: planar deck larger than MaxPlane")
	}
	ps.n[p] = uint8(n)
	for j := 0; j < n; j++ {
		if int(s.nobj) >= MaxObj {
			panic("kernel: decks and planar decks larger than MaxObj")
		}
		o := s.nobj
		s.nobj++
		s.objs[o] = object{card: cfg.PlanarDecks[p][j], chosen: 255, zone: kop.ZonePlanarDeck, slot: NoSlot, acts: p << objOwnerShift}
		s.touch(o)
		ps.deck[p][j] = o
	}
}

// shufflePlanar is gorge's shufflePlanarDeck: one Fisher-Yates shuffle of
// seat p's planar deck, consuming len-1 rng values exactly as the library
// shuffle does.
func (s *State) shufflePlanar(p uint8) {
	ps := s.planeCold()
	if ps == nil {
		return
	}
	n := int(ps.n[p])
	for i := n - 1; i > 0; i-- {
		j := s.rng.IntN(i + 1)
		ps.deck[p][i], ps.deck[p][j] = ps.deck[p][j], ps.deck[p][i]
	}
	// foldPlanarDeckShuffle: every card of the shuffled deck is face down
	// (the fold sets FaceDown on each listed id); the deal's PlanarReveal
	// then turns the head back up.
	ps.up[p] = 0
}

// revealPlaneTop is gorge's PlanarReveal fold (genesis deal): the deck's
// head plane is face up.
func (s *State) revealPlaneTop(p uint8) {
	if ps := s.planeCold(); ps != nil {
		ps.up[p] = 1
	}
}

// planeTop is seat p's face-up current plane, NoObj when it has none.
func (s *State) planeTop(p uint8) Obj {
	ps := s.planeCold()
	if ps == nil || int(p) >= MaxSeats || ps.n[p] == 0 {
		return NoObj
	}
	return ps.deck[p][0]
}

// planePos is o's position in seat p's planar deck, -1 when it is not there.
func (s *State) planePos(p uint8, o Obj) int {
	ps := s.planeCold()
	if ps == nil || int(p) >= MaxSeats {
		return -1
	}
	for i := 0; i < int(ps.n[p]); i++ {
		if ps.deck[p][i] == o {
			return i
		}
	}
	return -1
}

// planeFaceDown reports whether o, a card in a planar deck, is face down,
// by the record's face-up mask (see planeSeat.up).
func (s *State) planeFaceDown(o Obj) bool {
	if o == NoObj || int(o) >= int(s.nobj) || s.objs[o].zone != kop.ZonePlanarDeck {
		return false
	}
	p := s.objs[o].owner()
	j := s.planePos(p, o)
	if j < 0 {
		return false
	}
	ps := s.planeCold()
	return ps == nil || ps.up[p]&(1<<uint(j)) == 0
}

// planeswalk rotates seat p's planar deck (CR 901.8): the current plane goes
// to the bottom and the next card is revealed. The away and arrive triggers
// are queued on the planes involved, as gorge's PlanarWalk fold plus
// checkPlaneswalkTriggers do.
func (s *State) planeswalk(k *prep, p uint8) {
	ps := s.planeCold()
	if ps == nil || ps.n[p] < 2 {
		return
	}
	away := ps.deck[p][0]
	n := int(ps.n[p])
	copy(ps.deck[p][:n-1], ps.deck[p][1:n])
	ps.deck[p][n-1] = away
	// foldPlanarWalk: every card of the rotated deck is face down, the new
	// head face up.
	ps.up[p] = 1
	s.queuePlaneTriggers(k, p, away, kop.TrigPlaneswalkedFrom)
	s.queuePlaneTriggers(k, p, ps.deck[p][0], kop.TrigPlaneswalkedTo)
}

// chaosEnsues queues the current plane's Mode$ ChaosEnsues ability (CR
// 901.9). It is the marker's own plane -- the DB$ ChaosEnsues verb and the
// planar die both name the plane they erupt.
func (s *State) chaosEnsues(k *prep, p uint8) {
	s.queuePlaneTriggers(k, p, s.planeTop(p), kop.TrigChaosEnsues)
}

// queuePlaneTriggers queues the printed triggers of event ev on plane o.
// Only the command-zone spelling every corpus plane uses is read (gorge's
// synthetic plane scans bypass the ordinary zone gate for exactly this
// reason: the plane lives in ZonePlanarDeck, not ZoneCommand).
func (s *State) queuePlaneTriggers(k *prep, p uint8, o Obj, ev kop.TrigEvent) {
	if o == NoObj || int(o) >= int(s.nobj) {
		return
	}
	ob := &s.objs[o]
	if ob.zone != kop.ZonePlanarDeck || s.planePos(p, o) < 0 {
		return
	}
	cd := &k.t.Cards[ob.card]
	for i := range cd.Abilities {
		a := &cd.Abilities[i]
		if a.Kind != kop.AbTriggered || a.AFlags&kop.AfLatent != 0 || a.Trig.Event != ev {
			continue
		}
		if a.Trig.Zone != 0 && a.Trig.Zone != kop.ZoneCommand {
			continue
		}
		s.queueTrig(k, trigListener{obj: o, sl: NoSlot, ctrl: p, card: ob.card, ab: uint8(i)}, a, NoObj, p, 0)
	}
}

// eachPlaneTrigger calls f for every command-zone trigger of ev on a seat's
// face-up current plane. It is the plane counterpart of eachCommandTrigger
// (triggers_gen.go).
func (s *State) eachPlaneTrigger(k *prep, ev kop.TrigEvent, f func(l trigListener, a *kop.Ability)) {
	ps := s.planeCold()
	if ps == nil {
		return
	}
	any := false
	for p := range ps.n {
		any = any || ps.n[p] > 0
	}
	if !any {
		return
	}
	for p := uint8(0); p < s.nSeats(); p++ {
		if s.out(p) {
			continue
		}
		o := s.planeTop(p)
		if o == NoObj || s.objs[o].zone != kop.ZonePlanarDeck {
			continue
		}
		cd := &k.t.Cards[s.objs[o].card]
		for i := range cd.Abilities {
			a := &cd.Abilities[i]
			if a.Kind == kop.AbTriggered && a.AFlags&kop.AfLatent == 0 &&
				a.Trig.Event == ev && a.Trig.Zone == kop.ZoneCommand {
				f(trigListener{obj: o, sl: NoSlot, ctrl: p, card: s.objs[o].card, ab: uint8(i)}, a)
			}
		}
	}
}

// chaosEnsuesExt is ExtChaosEnsues (DB$ ChaosEnsues): chaos ensues on the
// resolving controller's current plane. Defined$ You (or absent) is the
// player arm; Defined$ Remembered (The Fertile Lands of Saulvinia) is the
// remembered arm: each remembered plane of the resolving controller's OWN
// planar deck erupts while it is still face up -- the dig's RememberFound$
// put the revealed plane at the deck's bottom and the same-zone move cleared
// its face-down state, so the remembered plane is face up there (gorge
// effChaosEnsues' chaosDestinations arm plus planar.go
// checkChaosEnsuesTriggers' boundary: a face-up plane of ev.Player's own
// deck, whatever seat of it now holds).
//
//kmanifest:ext ExtChaosEnsues
func (s *State) chaosEnsuesExt(k *prep, in *kop.Instr) (bool, bool) {
	if in.Obj == kop.RefRemembered {
		p := s.res.e.ctrl
		seen := 0
		var seenArr [MaxPlane]Obj
		for i := 0; i < int(s.g.nrem); i++ {
			o := s.g.rem[i]
			if o == NoObj || s.objs[o].zone != kop.ZonePlanarDeck {
				continue
			}
			if s.planePos(p, o) < 0 {
				continue // a plane that left the deck (gorge planarRememberedIDs)
			}
			if s.planeFaceDown(o) {
				continue // chaos erupts the plane only while it is still face up
			}
			dup := false
			for j := 0; j < seen; j++ {
				if seenArr[j] == o {
					dup = true
					break
				}
			}
			if dup {
				continue
			}
			seenArr[seen] = o
			seen++
			s.queuePlaneTriggers(k, p, o, kop.TrigChaosEnsues)
		}
		if seen == 0 {
			// Nothing remembered qualifies: gorge's effChaosEnsues falls back
			// to the controller's current plane.
			s.chaosEnsues(k, p)
		}
		return true, true
	}
	if p := s.playerRef(k, in.Obj); p != 255 {
		s.chaosEnsues(k, p)
	}
	return true, true
}

// planeswalkExt is ExtPlaneswalk (DB$ Planeswalk): the player Obj planeswalks.
//
//kmanifest:ext ExtPlaneswalk
func (s *State) planeswalkExt(k *prep, in *kop.Instr) (bool, bool) {
	if p := s.playerRef(k, in.Obj); p != 255 {
		s.planeswalk(k, p)
	}
	return true, true
}

// rollPlanarDiceExt is ExtRollPlanarDice (DB$ RollPlanarDice): the player
// rolls A six-sided planar dice (CR 901.3a: 1-4 blank, 5 planeswalk, 6
// chaos), each draw from the engine rng exactly as gorge's Rand(6)+1. Every
// kept result carries its CR 901.4 consequence.
//
//kmanifest:ext ExtRollPlanarDice
func (s *State) rollPlanarDiceExt(k *prep, in *kop.Instr) (bool, bool) {
	p := s.playerRef(k, in.Obj)
	if p == 255 {
		return true, true
	}
	n := s.value(k, in.A)
	if n < 1 {
		return true, true
	}
	for i := 0; i < n; i++ {
		switch s.rng.IntN(6) + 1 {
		case 5:
			s.planeswalk(k, p)
		case 6:
			s.chaosEnsues(k, p)
		}
	}
	return true, true
}
