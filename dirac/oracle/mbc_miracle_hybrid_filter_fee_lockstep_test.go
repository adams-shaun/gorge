package oracle

import "testing"

// CR 106.1, 601.2g: Sister Repentia's miracle cast ({W}{B}) is priced against
// gorge's potential pool, which admits a filter land's paid ability only once the
// other sources cover its fee. Plains beside Graven Cairns ({B/R},{T}: two mana)
// cannot pay the hybrid fee (the Plains makes {W}, Cairns' own {C} is its tap), so
// the Cairns adds only {C} and gorge offers only "no". The kernel counted the
// {B/R} fee as one mana of any colour and offered the yes too.
func TestMBCSisterRepentiaReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "miracle_hybrid_filter_fee_not_covered")
}
