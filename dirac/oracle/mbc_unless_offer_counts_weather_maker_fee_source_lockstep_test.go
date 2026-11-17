package oracle

import "testing"

// CR 608.2g, 702.21a: before paying an unless cost a player may activate mana
// abilities, and Weather Maker's "{T}, Remove two charge counters: Add {C}{C}"
// is a fixed, source-anchored SubCounter fee on a fixed production (CR
// 605.3b). An unless {3} offer therefore takes the counter activation (two)
// plus the payer's one other untapped source (one), with the pool empty.
// gorge's census prices that shape (rules paid_mana_census.go unlessWindowUnits'
// SubCounter arm, pay.ManaSubCounterOnlyCost); the pre-fix kernel's probe
// priced only the plain {T} (one) and offered decline where gorge offered pay.
// The game: p0 casts Juggernaut on five Islands, p1 answers with Mana Leak's
// unless {3}, and p0 holds Weather Maker with two charge counters and exactly
// one untapped Island. The recording is on a custom table because std-tmt has
// no mana unless cost: its only ward (Punk Frogs' Ward {3}) pays through the
// ward mana window's affordsFor/windowAffords path, not the unless census.
func TestMBCWeatherMakerUnlessFeeReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "unless_offer_counts_weather_maker_fee_source")
}
