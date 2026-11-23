package oracle

import "testing"

// CR 601.2b, 601.2c: Dominate ({X}{1}{U}{U}, target creature with mana value X
// or less) is castable only while some X both pays and leaves it a legal
// target. The recorded game holds two Islands, Fire-Lit Thicket ({T}: Add {C},
// and "{R/G}, {T}: Add {R}{R}/{R}{G}/{G}{G}") and Staff of Compleation ("{T},
// Pay 2 life: Add one mana of any color") untapped with only a mana value 2
// and higher creatures in play: four mana at most, so X is at most 1 and no
// creature qualifies. gorge's payment-plan census used to mark the Thicket's
// hybrid-fee ability a gaining gap (gapGains), which abandoned the count
// proof; with the Staff's any-colour production pricing the potential bound
// unbounded, the X-bound target sweep then offered the cast at X = 2 ({3}{U}{U}
// onto Tymora's Invoker) although the seat cannot pay it. The census now bounds
// a once-only fee-bearing gap's budget net at production minus the fee,
// counting only what exceeds the source's own covered production, so the
// recorded game replays clean.
func TestMBCOptionsDominateCastOfferReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "options_dominate_priority_cast_offer")
}
