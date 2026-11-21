package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Snow mana (CR 106.3, 107.4h): mana from a snow source is snow mana, and
// only snow mana pays a {S} pip. gorge tags a unit snow when its source's
// types hold Snow and no typed word (effects.ManaProducerTag; the typed word
// wins) and keeps a parallel per-colour tally of the snow units,
// state.Player.Snow, beside Player.Pool. The kernel keeps the same tally in
// the cold region coldSnow — a State field would fail
// TestNoNewFixedStateFields — clamped to its colour's remaining units the way
// genState.poolTre is (mana_artifact.go treSync): gorge spends a plain unit of
// a colour before a typed or snow one (pay/solve.go takeUnit), so after any
// spend the snow units left in a colour are min(Snow[c], Pool[c]) and the
// Treasure units left are min(Tre[c], Pool[c]-Snow[c]).
//
// The tag rides poolArtSnow, bit 7 of genState.poolArt: the bit is free (a
// slot's colour bits are 0..5, poolArtRestr is 6) and a set bit makes
// payFromPool's gate run artSync after any spend, which snowSyncs the tally,
// at no new State byte. The memo keys that carry the byte (key.art) may keep
// the bit: a state whose snow tally moved is a new state there, as a typed
// tag's move is.

// coldSnow is the pool's snow tally, per seat and colour, for a table whose
// genPrep.anySnow is set: MaxSeats*kop.NumManaKinds bytes, no view.
var coldSnow = newCold[uint8]("snow", func(g *genPrep) int {
	if g.anySnow {
		return MaxSeats * kop.NumManaKinds
	}
	return 0
})

// poolArtSnow marks, in poolArt, that p's pool holds a snow unit (snowTag).
const poolArtSnow = 1 << 7

// snowMana is the snow tag's precedence (prep.go's pcard.manaSnow): a card's
// mana is snow when its types hold Snow and no typed subtype word (Treasure,
// Cave, Desert: the word wins; CR 106.3, 205.4a; gorge's ManaProducerTag). The
// Artifact type is no such word: a Snow Artifact's unit is an Artifact unit and
// a snow unit both (gorge's "SArtifact<colour>" tag), so Replicating Ring's
// mana pays {S}.
func snowMana(c *kop.CardDef, typedWord bool) bool { return c.Snow && !typedWord }

// DebugSnowTypedIsSnow lets a typed snow source tag snow — a source whose
// types hold a typed word and Snow both (mutation testing only, ticket 03):
// the lockstep must catch it.
var DebugSnowTypedIsSnow bool

func init() { genNeed(tableAnySnow) }

// tableAnySnow reports whether any card's printed or ability cost asks {S}
// (kop.ManaCost.Snow, CR 107.4h) or reads the snow spent (CardDef.SpentSnow,
// Count$CastTotalManaSpent Snow): genPrep.anySnow, and with it the tally.
func tableAnySnow(t *kop.Table) bool {
	for i := range t.Cards {
		c := &t.Cards[i]
		if c.SpentSnow || c.Cost.Snow != 0 ||
			c.SpellCost.Mana.Snow != 0 || c.AltAddCost.Mana.Snow != 0 || c.OptCost.Mana.Snow != 0 ||
			c.KickerCost.Mana.Snow != 0 || c.FbCost.Mana.Snow != 0 || c.EscCost.Mana.Snow != 0 || c.EscBfCost.Mana.Snow != 0 {
			return true
		}
		for j := range c.Abilities {
			if c.Abilities[j].Cost.Mana.Snow != 0 {
				return true
			}
		}
	}
	return false
}

// snowAt is seat p's snow tally of colour c (0 when the table has no tally).
func (s *State) snowAt(p uint8, c int) uint8 {
	sx := coldSnow.of(s)
	if sx == nil {
		return 0
	}
	return sx[int(SeatIx(p))*kop.NumManaKinds+c]
}

// snowSetAt sets seat p's snow tally of colour c.
func (s *State) snowSetAt(p uint8, c int, v uint8) {
	if sx := coldSnow.of(s); sx != nil {
		sx[int(SeatIx(p))*kop.NumManaKinds+c] = v
	}
}

// snowClearAll empties every seat's tally, at a boundary that empties the pool
// (endStep, finishCleanup, drainMana's every seat, persistEmpty, persistTurn).
func (s *State) snowClearAll() {
	if sx := coldSnow.of(s); sx != nil {
		clear(sx)
	}
}

// snowTag records that the mana source at sl added a unit of colour col to
// p's pool and that the unit is snow (CR 106.3). The tag rides poolArtSnow, so
// the next spend's artSync clamps the tally; the clamp reads the pool of the
// unit just added, so a stale count above pool[col]-1 goes first. It is called
// after the unit is added, from artTag.
func (s *State) snowTag(k *prep, p uint8, sl Slot, col int) {
	s.snowTagN(k, p, s.cardAt(sl), col, 1)
}

// poolAddFrom adds n units of colour col that a resolving spell or ability of
// card produced to p's pool, snow-tagged when card's mana is snow.
func (s *State) poolAddFrom(k *prep, p uint8, card CardID, col, n int) {
	s.poolAdd(p, col, n)
	s.snowTagN(k, p, card, col, n)
	s.artTagN(k, p, card, col, n) // an artifact's (or a Desert's, a Cave's) mana is typed (mana_artifact.go)
}

// snowTagN is snowTag for n units a resolving spell or ability of card added
// at once (OpAddMana; the source is the stack entry's card, a snow spell's mana
// is snow mana as a snow permanent's is, gorge's ManaProducerTag): the stale
// count above the units the pool held before the add goes first.
func (s *State) snowTagN(k *prep, p uint8, card CardID, col, n int) {
	if k.gen == nil || !k.gen.anySnow || n <= 0 {
		return
	}
	pc := &k.cards[card]
	if !pc.manaSnow && !(DebugSnowTypedIsSnow && pc.manaTyped && k.t.Cards[card].Snow) {
		return
	}
	sx := coldSnow.of(s)
	i := SeatIx(p)
	t := &sx[int(i)*kop.NumManaKinds+col]
	if left := int(s.pl[i].pool[col]) - n; int(*t) > left {
		*t = uint8(max(left, 0))
	}
	*t = uint8(min(int(*t)+n, 255))
	s.g.poolArt[i] |= poolArtSnow
}

// snowSync clamps p's tally to the units its pool still holds and clears
// poolArtSnow when every colour's tally is zero. gorge spends a plain unit of
// a colour before a snow one (pay.takeUnit), so the tally of a colour is the
// fewest the slot's remaining units can hold: min(tallied, left). artSync
// calls it before treSync, so the Treasure half reads the snow units of a
// slot out first.
func (s *State) snowSync(p uint8) {
	sx := coldSnow.of(s)
	if sx == nil {
		return
	}
	i := SeatIx(p)
	any := false
	for c := 0; c < kop.NumManaKinds; c++ {
		n := &sx[int(i)*kop.NumManaKinds+c]
		if pool := s.pl[i].pool[c]; *n > pool {
			*n = pool
		}
		any = any || *n != 0
	}
	if !any {
		s.g.poolArt[i] &^= poolArtSnow
	}
}

// snowTotal is p's snow units in the pool, the tally synced first.
func (s *State) snowTotal(p uint8) int {
	s.snowSync(p)
	n := 0
	for c := 0; c < kop.NumManaKinds; c++ {
		n += int(s.snowAt(p, c))
	}
	return n
}

// snowPreSpend, as a cast's payment begins, keeps the caster's snow units on
// the build when the spell's card reads Count$CastTotalManaSpent Snow
// (CardDef.SpentSnow), for snowSpentRec.
func (s *State) snowPreSpend(k *prep, e *stackEntry) {
	if k.gen != nil && k.gen.anySnow && k.t.Cards[e.card].SpentSnow {
		s.bld().snowPre = uint8(min(s.snowTotal(e.ctrl), 255))
	}
}

// snowSpentRec is the amount a cast records as its spend (histCast): for a
// SpentSnow card the snow units its payment took out of the pool (gorge
// Object.ManaSnowSpent, which captureCastSpend counts), the units before the
// payment less the units left; any other card's is the mana spent.
func (s *State) snowSpentRec(k *prep, e *stackEntry, spent int) int {
	if DebugSnowSpentTotal || k.gen == nil || !k.gen.anySnow || !k.t.Cards[e.card].SpentSnow {
		return spent
	}
	return max(int(s.bld().snowPre)-s.snowTotal(e.ctrl), 0)
}

// DebugSnowSpentTotal records a SpentSnow cast's mana value, not its snow
// units (mutation testing only, TestSnowSpentMutation).
var DebugSnowSpentTotal bool

// snowPays reports whether p's pool pays cost, which asks {S} pips (CR 107.4h):
// the coloured pips come off a copy of the pool first (gorge's ResolveManaWith
// pays them before the {S} pips, and spends a plain unit of a colour before a
// snow one), the tally is clamped to what is left, cost.Snow snow units are
// needed across the colours, and the rest of the pool covers Generic. A cost
// the lowering never emits with {S} (a hybrid or {X} pip) is never paid.
func (s *State) snowPays(p uint8, cost *kop.ManaCost) bool {
	if cost.Hyb[0] != 0 || cost.X != 0 {
		return false
	}
	pool := s.pl[SeatIx(p)].pool
	tot, snow := 0, 0
	for c := 0; c < kop.NumManaKinds; c++ {
		if pool[c] < cost.Pips[c] {
			return false
		}
		pool[c] -= cost.Pips[c]
		tot += int(pool[c])
		snow += int(min(s.snowAt(p, c), pool[c]))
	}
	return snow >= int(cost.Snow) && tot-int(cost.Snow) >= int(cost.Generic)
}

// snowCensusAffords is avail.affords for a cost with {S} pips: a census read
// (ungrAvail's, the planner's pool plus untapped printed mana sources) prices
// them. avail.affords refuses every {S} cost -- gorge never plans one
// (PlanCostDetail cost:snow) -- so the census's {S} pips ride the pool's snow
// units alone, the read the offer gate prices them with (poolPays reads
// snowPays): each pip takes one unit the pool's snow tally still counts, in
// gorge's ResolveManaWith colour order (W U B R G C, pay/solve.go alt.Snow)
// with backtracking, and the residual cost is avail.affords over the units
// left. A {S} pip never takes a tapped source's production: no snow read past
// the pool exists (the bound carries no snow tally, mana.go potentialManaFor).
// A cost with a hybrid, twobrid or {X} pip alongside is never planned
// (snowPays), and the coloured pips ride the ordinary affords read.
func (s *State) snowCensusAffords(p uint8, cost *kop.ManaCost, av *avail) bool {
	if cost.Hyb[0] != 0 || cost.Tb != 0 || cost.X != 0 {
		return false
	}
	var pool, snow [kop.NumManaKinds]uint8
	pl := &s.pl[SeatIx(p)]
	for c := 0; c < kop.NumManaKinds; c++ {
		pool[c] = pl.pool[c]
		snow[c] = s.snowAt(p, c) // the tally is clamped to the pool (snowSync)
	}
	return snowCensusRec(cost, av, &pool, &snow)
}

// snowCensusRec is snowCensusAffords's pip walk: one {S} pip per level, each
// colour's units shared between the avail and the pool/snow copies so a unit
// the pip takes cannot also pay the residual cost.
func snowCensusRec(cost *kop.ManaCost, av *avail, pool, snow *[kop.NumManaKinds]uint8) bool {
	if cost.Snow == 0 {
		return av.affords(cost, pipMaskOf(cost), 0)
	}
	for c := 0; c < kop.NumManaKinds; c++ { // pay/solve.go alt.Snow's colour order
		if snow[c] == 0 || pool[c] == 0 {
			continue
		}
		av2 := *av
		av2.remove(1<<c, 1)
		p2, s2 := *pool, *snow
		p2[c]--
		s2[c]--
		r := *cost
		r.Snow--
		if snowCensusRec(&r, &av2, &p2, &s2) {
			return true
		}
	}
	return false
}

// DebugSnowAnyUnit lets a {S} pip take any unit of the pool in generic order,
// not a snow one (mutation testing only, TestSnowCostMutation).
var DebugSnowAnyUnit bool

// SnowStats counts {S} costs, for oracle tests only.
type SnowStats struct {
	PipsPaid int // {S} pips payFromPool paid from a snow unit
	Offered  int // {S} activations offered
}

// DebugSnowStats is the test hook; nil in normal play.
var DebugSnowStats *SnowStats

// snowPay pays n {S} pips from p's pool, after its coloured pips: each takes
// the first colour of W U B R G C whose snow tally is non-zero (gorge
// ResolveManaWith alt.Snow), decrementing the pool and the tally. The tally is
// synced first (a plain unit of a colour goes before a snow one). It panics, as
// the other payment arms do, when no snow unit is left.
func (s *State) snowPay(p uint8, n uint8) {
	s.snowSync(p)
	pl := &s.pl[SeatIx(p)]
	order := [kop.NumManaKinds]int{kop.W, kop.U, kop.B, kop.R, kop.G, kop.C}
	if DebugSnowAnyUnit {
		order = [kop.NumManaKinds]int{kop.C, kop.W, kop.U, kop.B, kop.R, kop.G}
	}
	for ; n > 0; n-- {
		paid := false
		for _, c := range order {
			if pl.pool[c] == 0 || !DebugSnowAnyUnit && s.snowAt(p, c) == 0 {
				continue
			}
			pl.pool[c]--
			if t := s.snowAt(p, c); t > 0 {
				s.snowSetAt(p, c, t-1)
			}
			paid = true
			break
		}
		if !paid {
			panic("kernel: payment failed (snow)")
		}
		if st := DebugSnowStats; st != nil {
			st.PipsPaid++
		}
	}
}
