package kgen

import "github.com/adams-shaun/gorge/dirac/kop"

// walkCardDefRefs walks the filters and values a CardDef holds outside its
// abilities and programs. A matchup keeps only the subtypes refSubs reaches,
// so a field missing here has its subtype dropped from every mask and its
// filter reads NoSubtype: a negated one then passes everything, and a mutating
// cast's non-Human target offers Humans (CR 702.140a).
func walkCardDefRefs(d *kop.CardDef, filt func(kop.FilterID), val func(kop.ValueRef)) {
	for _, f := range [...]kop.FilterID{
		d.BeholdF,
		d.RevealF,
		d.OfferingF, // the offering's sacrifice filter (kw_offering.go)
		d.WardSac,
		d.CastBF,   // the frozen count's filter (CardDef.CastBF)
		d.ProtFrom, // K:Protection:Coyote can't name a subtype the matchup then drops
		d.Landwalk,
	} {
		filt(f)
	}
	for _, t := range [...]*kop.TargetSpec{&d.Enchant, &d.BestowTgt, &d.MutateTgt} {
		filt(t.Filter)
		filt(t.SpellF)
		val(t.MinV)
		val(t.MaxV)
	}
	for _, c := range [...]*kop.Cost{
		&d.SpellCost, &d.AltAddCost, &d.AltAddCost2, &d.KickerCost, &d.Repeated,
		&d.OptCost, &d.FbCost, &d.EscCost, &d.EscBfCost, &d.WarpPay,
		&d.RetraceCost, // the retrace cast's discard candidates
		&d.MorphPick,   // the turn-up pick's candidates
		&d.EvokePick,   // the evoked cast's hand-exile candidates
	} {
		walkCostRefs(c, filt)
	}
	val(d.ModesPickV) // a dynamic triggered charm's count filter
	val(d.MorphReduce)
}

// walkCostRefs walks the filters of one cost.
func walkCostRefs(c *kop.Cost, filt func(kop.FilterID)) {
	filt(c.Filter)
	filt(c.Discard2F)
	filt(c.DiscardSideF)
}
