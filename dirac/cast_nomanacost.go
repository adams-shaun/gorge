package dirac

// CR 118.6 / 202.1b: an object with no mana cost can't be cast by paying its
// mana cost (a printed "no cost": Ancestral Vision, Lotus Bloom, Living End).
// The kernel prices it as a free cost, so gorge's filterNoManaCostCasts
// (rules/nomanacost.go, run over every cast source at the walk's one choke
// point) is mirrored here over the finished priority options: a cast that
// pays the printed cost (the plain and kicked hand cast, a play permission
// that does not waive it) is dropped; one that waives or replaces the cost
// (a free cast, flashback, an alternative cost, another face) stays.

// dropNoManaCostCasts removes from d the cast options of a no-mana-cost card
// that would pay its printed mana cost (order kept).
//
//go:noinline
func (s *State) dropNoManaCostCasts(k *prep, d *Decision) {
	const notPrinted = 2 | castFace | castFree | castFaceDown | castWarp | castWarpRecast
	n := 0
	for i := 0; i < int(d.N); i++ {
		a := d.Options[i]
		if a.Kind() == AkCast && a.Target() == 0 && a.Mode()&notPrinted == 0 && k.t.Cards[s.objs[Obj(a.Source())].card].NoManaCost {
			continue
		}
		d.Options[n] = a
		n++
	}
	d.N = uint16(n)
}
