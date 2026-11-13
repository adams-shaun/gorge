package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Artifact mana. gorge tags the mana an artifact's mana ability adds (Sol
// Ring emits "Artifact", a Treasure "ArtifactTreasure": effects.ManaProducerTag).
// A tagged unit sits in its colour's bucket for the planner (pay.PlanPool), and
// gorge's prefix search keys on the tagged units apart (Player.ManaUnits).
//
// genState.poolArt holds, per player, the pool slots (bit per W U B R G C)
// that hold an artifact unit. gorge spends a plain unit before a tagged one
// (pay.takeUnit), so a slot keeps its tagged unit for as long as it is not
// empty: the bit is cleared when the slot empties (artSync, after each pool
// spend) and with the pool at a step boundary.

// gorge's typed tallies (state.Player.TypedMana: Treasure, Cave, Desert and
// Artifact-only) read the same way, so mana a Desert or a Cave adds floats as a
// tagged unit too. The tag changes no offer: gorge's planners read a tagged
// unit as plain mana of its colour (pay.PlanPool; TestPlannerPaysPastATreasureUnit),
// and a restricted one as the payment admits it (CR 106.6). Open: the
// filter-land and Giada-window pool additions do not tag (no typed source
// reaches them).

// typedManaSubtypes is the subtype set of the Desert and Cave producers.
func typedManaSubtypes(t *kop.Table) kop.SubMask {
	var m kop.SubMask
	for i, n := range t.Subtypes {
		if n == "Desert" || n == "Cave" {
			m = m.With(i)
		}
	}
	return m
}

// poolArtRestr marks, in poolArt, that p's restricted mana (State.rb) came
// from a typed source (Interdimensional Web Watch is an artifact): gorge spends
// the plain units of a colour before such a unit, as it does a tagged one.
const poolArtRestr = 1 << 6

// DebugRestrTypedFirst spends a typed restricted unit before the plain units
// of its colour (the restricted-first read the carve alone would suggest),
// mutation testing only: the lockstep must catch it.
var DebugRestrTypedFirst bool

// artTag records that the mana source at sl added a unit of colour col to
// p's pool. It is called after the unit is added.
func (s *State) artTag(k *prep, p uint8, sl Slot, col int) {
	if k.artMana && k.cards[s.cardAt(sl)].manaTyped {
		s.g.poolArt[SeatIx(p)] |= 1 << col
		if k.cards[s.cardAt(sl)].castSrc&csTreasure != 0 {
			if n := &s.g.poolTre[SeatIx(p)][col]; *n < 255 {
				*n++
			}
		}
	}
}

// treSync is the Treasure half of artSync. gorge spends a plain unit of a
// colour before a Treasure's (pay.takeUnit), so after any spend the Treasure
// units still floating in a slot are the fewest the slot's remaining units
// can hold: min(tallied, left). Each one that went was spent, and when a spell
// is being built that is the cast's Treasure-spend bit (bflTre, CR 601.2h).
// The bit follows the spend, not the tap: a Treasure tapped (or sacrificed at
// priority) whose mana still floats when the cast is paid did not pay for it.
func (s *State) treSync(p uint8) {
	t := &s.g.poolTre[SeatIx(p)]
	for c := range t {
		if t[c] == 0 {
			continue
		}
		if left := min(t[c], s.pl[SeatIx(p)].pool[c]); left < t[c] {
			t[c] = left
			if s.buildLive() && s.bld().e.kind == seSpell {
				s.bld().bfl |= bflTre
			}
		}
	}
}

// artSync drops the slots of p's pool that have emptied.
func (s *State) artSync(p uint8) {
	if s.fbR[SeatIx(p)]|(s.pmOn>>SeatIx(p))&1 != 0 {
		s.persistSync(p) // ordinary mana is spent first (kw_firebend.go, mana_persistent.go)
	}
	a := s.g.poolArt[SeatIx(p)]
	for c := 0; c < kop.NumManaKinds; c++ {
		if a&(1<<c) != 0 && s.pl[SeatIx(p)].pool[c] == 0 {
			a &^= 1 << c
		}
	}
	s.g.poolArt[SeatIx(p)] = a
	s.treSync(p)
	if s.g.defSrc[SeatIx(p)] != 0 && s.pl[SeatIx(p)].pool[s.g.defCol[SeatIx(p)]] == 0 {
		s.g.defSrc[SeatIx(p)], s.g.defCol[SeatIx(p)] = 0, 0 // the deferred source's mana is spent (mana_deferred.go)
	}
}
