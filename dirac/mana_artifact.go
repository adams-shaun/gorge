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
	s.snowTag(k, p, sl, col) // the snow half of the tag (mana_snow.go)
	if k.artMana && k.cards[s.cardAt(sl)].manaTyped {
		s.g.poolArt[SeatIx(p)] |= 1 << col
		if sx := coldArtN.of(s); sx != nil {
			if n := &sx[int(SeatIx(p))*kop.NumManaKinds+col]; *n < 255 {
				*n++
			}
		}
		if k.cards[s.cardAt(sl)].castSrc&csTreasure != 0 {
			if n := &s.g.poolTre[SeatIx(p)][col]; *n < 255 {
				*n++
			}
		}
	}
}

// DebugTypedManaNoTag skips the resolving path's artifact tag (mutation
// testing only): a resolving DB$ Mana of a typed source adds untagged, while
// gorge's ManaProducerTag still tags it — the lockstep's tag-count projection
// must catch it (TestTypedArtifactManaMutation).
var DebugTypedManaNoTag bool

// TypedManaStats counts the resolving path's typed mana, for oracle tests only.
type TypedManaStats struct {
	Added   int // units a resolving DB$ Mana of a typed source tagged (artTagN)
	Planned int // typed units a planned payment (a cast build) spent (artSync's clamp)
	Manual  int // typed units a spend outside a planned payment spent
}

// DebugTypedManaStats is the test hook; nil in normal play.
var DebugTypedManaStats *TypedManaStats

// artTagN is artTag for n units a resolving spell's or trigger's DB$ Mana of
// card added at once (poolAddFrom; the source is the stack entry's card, whose
// printed face gorge's ManaProducerTag reads): the pool's colour bit, the
// tagged-unit tally (coldArtN, when the table holds one) and a Treasure's
// tally, the clauses artTag runs after the mana-ability path's add. A
// restricted add bypasses it (addManaRestr marks poolArtRestr instead).
func (s *State) artTagN(k *prep, p uint8, card CardID, col, n int) {
	if n <= 0 || DebugTypedManaNoTag || !k.artMana || !k.cards[card].manaTyped {
		return
	}
	i := SeatIx(p)
	s.g.poolArt[i] |= 1 << col
	if sx := coldArtN.of(s); sx != nil {
		t := &sx[int(i)*kop.NumManaKinds+col]
		*t = uint8(min(int(*t)+n, 255))
	}
	if k.cards[card].castSrc&csTreasure != 0 {
		if t := &s.g.poolTre[i][col]; *t < 255 {
			*t = uint8(min(int(*t)+n, 255))
		}
	}
	if st := DebugTypedManaStats; st != nil {
		st.Added += n
	}
}

// treSync is the Treasure half of artSync. gorge spends a plain unit of a
// colour before a Treasure's (pay.takeUnit), so after any spend the Treasure
// units still floating in a slot are the fewest the slot's remaining units
// can hold: min(tallied, left). The slot's snow units go first too — after the
// plain ones, before a Treasure's (pay.takeUnit spends plain, then typed, then
// snow) — so when p's pool holds snow units (poolArtSnow) the clamp reads them
// out first: min(tallied, left-Snow[left]). snowSync has clamped the tally to
// the slot by the time this runs (artSync calls it first), so left-Snow does
// not go under zero. Each one that went was spent, and when a spell is being
// built that is the cast's Treasure-spend bit (bflTre, CR 601.2h). The bit
// follows the spend, not the tap: a Treasure tapped (or sacrificed at
// priority) whose mana still floats when the cast is paid did not pay for it.
func (s *State) treSync(p uint8) {
	i := SeatIx(p)
	t := &s.g.poolTre[i]
	var sx []uint8
	if s.g.poolArt[i]&poolArtSnow != 0 {
		sx = coldSnow.of(s)
	}
	for c := range t {
		if t[c] == 0 {
			continue
		}
		left := s.pl[i].pool[c]
		if sx != nil {
			if n := sx[int(i)*kop.NumManaKinds+c]; n < left {
				left -= n
			} else {
				left = 0
			}
		}
		if left > t[c] {
			left = t[c]
		}
		if left < t[c] {
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
		// gorge spends a plain unit before a tagged one, so the tagged units
		// left in a slot are the fewest it can hold (treSync's clamp)
		if sx := coldArtN.of(s); sx != nil {
			if n := &sx[int(SeatIx(p))*kop.NumManaKinds+c]; *n > s.pl[SeatIx(p)].pool[c] {
				if st := DebugTypedManaStats; st != nil && int(*n)-int(s.pl[SeatIx(p)].pool[c]) > 0 {
					// a cast build's payment is the planned one; an ability
					// activation's build (seAbility) and every other drain are
					// the manual window's
					if s.ctl == ctlBuild && s.bld().e.kind == seSpell {
						st.Planned += int(*n) - int(s.pl[SeatIx(p)].pool[c])
					} else {
						st.Manual += int(*n) - int(s.pl[SeatIx(p)].pool[c])
					}
				}
				*n = s.pl[SeatIx(p)].pool[c]
			}
		}
	}
	s.g.poolArt[SeatIx(p)] = a
	s.snowSync(p) // the snow half of the tag, before the Treasure clamp reads it (mana_snow.go)
	s.treSync(p)
	if s.g.defSrc[SeatIx(p)] != 0 && s.pl[SeatIx(p)].pool[s.defColOf(p)] == 0 {
		s.g.defSrc[SeatIx(p)] = 0
		s.setDefCol(p, 0) // the deferred source's mana is spent (mana_deferred.go)
	}
}

// coldArtN is the pool's tagged-unit tally, per seat and colour, for a table
// with a typed producer (genPrep.anyArtN): gorge keys its prefix search on the
// tagged units by count (Player.ManaUnits), where poolArt holds one bit per
// colour. A cold region, as coldSnow is: a State field would fail
// TestNoNewFixedStateFields. Clamped to the colour's units left (artSync).
var coldArtN = newCold[uint8]("artn", func(g *genPrep) int {
	if g.uses.has(useArtN) {
		return MaxSeats * kop.NumManaKinds
	}
	return 0
})

// useArtN: the table holds a typed producer (the tally exists).
var useArtN = newGenUse(tableAnyTypedMana)

// tableAnyTypedMana reports whether any card's mana is typed (prep.go manaTyped)
// and the tally can be read: a pool unit of a colour is spent plain-first, so
// two search states that differ in which unit a payment spent (a typed {U}
// beside a land's) key apart in gorge. That needs mana spent from the pool
// beside the typed producer: a hand-mana card's plain mana, or a mana ability
// that costs mana (Energy Refractor's "{2}: Add one mana of any color", a
// filter land's fee).
func tableAnyTypedMana(t *kop.Table) bool {
	// Any typed producer alone would make a vanilla fixture's table need gen,
	// and the fresh-clone tests pin a nil gen there. A table with typed
	// producers but neither keeps one bit per colour, as before.
	spends := false
	for i := range t.Cards {
		if t.Cards[i].HandMana != 0 {
			spends = true
			break
		}
		for j := range t.Cards[i].Abilities {
			if a := &t.Cards[i].Abilities[j]; a.Kind == kop.AbMana && a.Cost.Mana.MV() > 0 {
				spends = true
			}
		}
	}
	if !spends {
		return false
	}
	typed := typedManaSubtypes(t)
	for i := range t.Cards {
		c := &t.Cards[i]
		if c.Types&kop.TArtifact == 0 && (c.Subtypes[0]&typed[0]|c.Subtypes[1]&typed[1]) == 0 {
			continue
		}
		for j := range c.Abilities {
			if c.Abilities[j].Kind == kop.AbMana {
				return true
			}
		}
	}
	return false
}

// artNClearSeat empties seat p's tagged-unit tally (its pool emptied).
func (s *State) artNClearSeat(p uint8) {
	if sx := coldArtN.of(s); sx != nil {
		clear(sx[int(SeatIx(p))*kop.NumManaKinds : int(SeatIx(p))*kop.NumManaKinds+kop.NumManaKinds])
	}
}

// artNClearAll empties every seat's tagged-unit tally (a boundary that empties the pool).
func (s *State) artNClearAll() {
	if sx := coldArtN.of(s); sx != nil {
		clear(sx)
	}
}

// DebugArtKeyBit keys the prefix search's tagged units by their per-colour bit,
// not their count (mutation testing only: the lockstep must catch it).
var DebugArtKeyBit bool

// DebugWinFeeStallsAlways applies the fee-only stall to a window with no
// pending cast too (mutation testing only: the lockstep must catch it).
var DebugWinFeeStallsAlways bool
