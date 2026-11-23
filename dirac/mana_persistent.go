package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// PersistentMana$ True (gorge effects/mana_effect.go; events/apply_state_helpers.go
// applyManaAdd, foldManaClear, apply_turn.go): the units an AddMana with B = 2
// adds sit in the pool AND in State.pm[p][colour]. They survive every step
// boundary and the cleanup step; the turn change zeroes the tally, the units
// become ordinary mana and the cleanup-to-untap boundary empties them.
//
// Spends are ordinary-first (apply_state_helpers.go:370-400): only the share of
// a spend beyond the ordinary units reduces the tally, so the persistent units
// still floating are min(pm[p][c], pool[c]) and no spend path needs to know
// about them. persistSync re-derives that clamp after a pool payment (artSync).
//
// Firebending's combat units (State.fbR, red) are a subset of the same tally in
// gorge (CombatMana within PersistentMana); here they sit beside pm, and a
// spend takes the combat units first (apply_state_helpers.go:390-400).
//
// Restricted mana is persistent only as a State.rb batch of a restriction
// genPrep.restrPers marks (Karn, Legacy Reforged's artifact-only {C}): the
// units are not tallied in pm, the batch survives persistEmpty with its typed
// bit, and pmOn stays set for the seat while it floats. Every other
// restricted pool still empties — but a batch of a colour a live
// StatUnspentMana static protects survives every boundary with the slot's
// ordinary share (rbKeep; gorge foldManaClear's
// `r.Persistent || keep[slot]`), the turn change's included after gorge's
// TurnChange demotes every batch first (rbKeep's pers=false).

// DebugPersistEmpty is the mutation switch of TestPersistMutation: the step
// boundary keeps no persistent units, as if the mana were ordinary.
var DebugPersistEmpty bool

// StallPersistMana: more than 255 persistent units of one colour at once
// (State.pm).
const StallPersistMana = 100

func init() { stallNames[StallPersistMana] = "persistmana" }

// persistAdd tallies n persistent units of colour col for player p,
// saturating (a further unit would be dropped silently, so the game stalls
// instead).
func (s *State) persistAdd(p uint8, col, n int) {
	if n <= 0 {
		return
	}
	t := int(s.pm[SeatIx(p)][col]) + n
	if t > 255 {
		t = 255
		s.capFull(StallPersistMana)
	}
	s.pm[SeatIx(p)][col] = uint8(t)
	s.pmOn |= 1 << SeatIx(p)
}

// persistSync clamps p's persistent tallies (pm and firebending's fbR) to the
// units the pool still holds. Gorge spends the ordinary share of a slot first
// (apply_state_helpers.go:370-400); only the part of a spend beyond it reduces
// the tally, and the combat subset goes first within that (CombatMana -=
// min(CombatMana, d)), so for red the persistent total P = fbR + pm[R] drops
// to P' = min(P, pool[R]) and the drop comes out of fbR before pm[R].
func (s *State) persistSync(p uint8) {
	p = SeatIx(p)
	pool := &s.pl[p].pool
	red := int(s.fbR[p]) + int(s.pm[p][kop.R])
	left := min(red, int(pool[kop.R]))
	fb := int(s.fbR[p]) - min(int(s.fbR[p]), red-left)
	s.fbR[p], s.pm[p][kop.R] = uint8(fb), uint8(left-fb)
	var any uint8
	for c := range s.pm[p] {
		s.pm[p][c] = min(s.pm[p][c], pool[c])
		any |= s.pm[p][c]
	}
	if any == 0 && !s.rbPersist(p) {
		s.pmOn &^= 1 << p
	}
}

// persistKeep is the persistent share of p's pool that survives a step
// boundary: pm in every colour plus the firebending red unless the end-of-combat
// step is being left (ec; gorge foldStepChange subtracts CombatMana from the
// tally before ManaClear). The tallies shrink to what is kept.
func (s *State) persistKeep(p uint8, ec bool) [kop.NumManaKinds]uint8 {
	s.persistSync(p)
	var keep [kop.NumManaKinds]uint8
	if ec || DebugFirebendEmpty {
		s.fbR[p] = 0
	}
	if DebugPersistEmpty {
		s.pm[p] = [kop.NumManaKinds]uint8{}
		s.pmOn &^= 1 << p
	}
	keep = s.pm[p]
	keep[kop.R] += s.fbR[p]
	if s.pmOn&(pmUnspentR<<p) != 0 {
		keep[kop.R] = s.pl[p].pool[kop.R] // Effect UnspentMana Red keeps the whole red slot
	}
	return keep
}

// persistEmpty empties both pools but the persistent units (ManaClear), at a
// step boundary (ec: the end-of-combat step ends) and in the cleanup step.
// A live StatUnspentMana static keeps its protected slots' whole old pool
// (gorge's ManaClear keep letters; static_unspentmana.go), captured before
// the drop like the tallies are.
func (s *State) persistEmpty(ec bool) {
	var prot [MaxSeats]uint8
	var oldPools [MaxSeats][kop.NumManaKinds]uint8
	if g := s.k().gen; g != nil && g.uses.has(useUnspentMana) {
		for p := range s.pl {
			prot[p] = s.unspentMask(s.k(), uint8(p))
			oldPools[p] = s.pl[p].pool
		}
	}
	var restr [MaxSeats]uint8
	for p := range s.pl {
		keep := s.persistKeep(uint8(p), ec)
		if prot[p] != 0 {
			for c := 0; c < int(kop.NumManaKinds); c++ {
				if prot[p]&(1<<uint(c)) != 0 {
					keep[c] = oldPools[p][c] // the protected slot keeps its whole old pool
				}
			}
		}
		s.pl[p].pool = keep
		if prot[p] != 0 {
			s.rbKeep(uint8(p), prot[p], true) // the keep letters protect the slot's restricted batches too
		} else {
			s.rbClearStep(uint8(p))
		}
		if s.rbAny(uint8(p)) {
			restr[p] = s.g.poolArt[p] & poolArtRestr // a kept batch stays typed (Karn, Legacy Reforged)
		}
	}
	s.g.defSrc = [MaxSeats]uint8{}
	s.g.defCol = [(MaxSeats + 1) / 2]uint8{}
	s.talliesKeep(prot, restr) // a kept slot's tag tallies survive with it (mana_snow.go, mana_artifact.go)
}

// persistTurn is the turn change. Gorge's TurnChange zeroes the tally and the
// very next boundary, cleanup to untap (rules/turn.go setStep, which emits
// ManaClear for a seat whose pool is nonempty, before the first decision of
// the new turn), empties the now-ordinary units, so both pools empty here.
func (s *State) persistTurn() {
	// an UnspentMana keep rides the turn change's ManaClear as any other
	// boundary's (gorge rules/turn.go setStep emits it here too): the
	// protected slots keep their old pool through the zeroing.
	var prot [MaxSeats]uint8
	var oldPools [MaxSeats][kop.NumManaKinds]uint8
	if g := s.k().gen; g != nil && g.uses.has(useUnspentMana) {
		for p := range s.pl {
			prot[p] = s.unspentMask(s.k(), uint8(p))
			oldPools[p] = s.pl[p].pool
		}
	}
	s.pm = [MaxSeats][kop.NumManaKinds]uint8{}
	s.pmOn = 0
	var restr [MaxSeats]uint8
	for p := range s.pl {
		s.pl[p].pool = [kop.NumManaKinds]uint8{}
		if prot[p] != 0 {
			// gorge's TurnChange demotes every batch before the
			// cleanup-to-untap ManaClear, so only the keep-letter colours
			// survive here (rbKeep's pers=false)
			s.rbKeep(uint8(p), prot[p], false)
			if s.rbAny(uint8(p)) {
				restr[p] = s.g.poolArt[p] & poolArtRestr // a kept batch stays typed (Karn, Legacy Reforged)
			}
		} else {
			s.rbClear(uint8(p))
		}
	}
	if any := s.unspentProtAny(prot); any {
		for p := range s.pl {
			for c := 0; c < int(kop.NumManaKinds); c++ {
				if prot[p]&(1<<uint(c)) != 0 {
					s.pl[p].pool[c] = oldPools[p][c]
				}
			}
		}
	}
	s.g.defSrc = [MaxSeats]uint8{}
	s.g.defCol = [(MaxSeats + 1) / 2]uint8{}
	s.talliesKeep(prot, restr) // a kept slot's tag tallies survive with it (mana_snow.go, mana_artifact.go)
}

// PersistMana is each player's persistent units per colour (State.pm) for the
// oracle's probes.
func (s *State) PersistMana() [MaxSeats][kop.NumManaKinds]uint8 { return s.pm }
