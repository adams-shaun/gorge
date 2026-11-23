package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Activations priced by the filter script and gorge's search slots. canActivate
// prices an activation the plain census is short of against the filter script
// (actScriptPays) with no bound. gorge's potential walk gives the prefix search
// to the first potentialScriptPlays plays whose census is incomplete, hand casts
// first and then the battlefield's activations in entry order, and leaves the
// rest unpriced (potentialCensusFallback: "census_incomplete"). So a board with
// more such plays than slots offers only the first ones (CR 605.3a, 602.2b: Myr
// Propagator's {3},{T} copy of itself, seventeen of them beside a Ferrous Lake).

// actScripted is the activations in a decision that only the filter script
// prices, in gorge's walk order.
type actScripted struct {
	n   int
	act [MaxOptions]Action
	key [MaxOptions]uint64
}

// actScriptedIn collects into out the activations of d that the plain census
// does not pay: the ones actScriptPays added.
func (s *State) actScriptedIn(k *prep, d *Decision, p uint8, sorc bool, out *actScripted) {
	var av avail
	built := false
	x := s.g.availX
	for i := 0; i < int(d.N); i++ {
		a := d.Options[i]
		if a.Kind() != AkActivate || a.Mode() >= 255 {
			continue
		}
		ob := &s.objs[Obj(a.Source())]
		if ob.zone != kop.ZoneBattlefield {
			continue
		}
		sl := ob.slot
		ai := int(a.Mode())
		abs := k.t.Cards[s.cardAt(sl)].Abilities
		if ai >= len(abs) || abs[ai].Kind != kop.AbActivated {
			continue
		}
		if !built {
			s.availFor(k, p, &av)
			built = true
		}
		s.g.availX = x | availNoActScript
		plain := s.canActivate(k, sl, ai, &abs[ai], sorc, &av)
		s.g.availX = x
		if plain {
			continue
		}
		kk := s.walkKey(a, p, i)
		j := out.n
		for ; j > 0 && out.key[j-1] > kk; j-- {
			out.act[j], out.key[j] = out.act[j-1], out.key[j-1]
		}
		out.act[j], out.key[j] = a, kk
		out.n++
	}
}

// actScriptTrim drops from d the script-priced activations past the search
// slots the casts scriptOffers searched (used) left: gorge's walk searches
// those casts first.
func actScriptTrim(d *Decision, in *actScripted, used int) {
	for j := max(potentialScriptPlays-used, 0); j < in.n; j++ {
		d.remove(in.act[j])
	}
}

// scriptWalk runs the script walk: scriptOffers' casts and, on a board with a
// filter, the budget the activations only the filter script prices share with
// them.
func (s *State) scriptWalk(k *prep, d, capSpent *Decision, p uint8, sorc bool) {
	if s.g.availX != 0 {
		return
	}
	if k.gen.anyFilt && s.scriptFilt(k, p).any() {
		var acts actScripted
		s.actScriptedIn(k, d, p, sorc, &acts)
		used := 0
		if s.scriptAny(k) {
			used = s.scriptOffers(k, d, capSpent, p)
		}
		actScriptTrim(d, &acts, used)
		return
	}
	if s.scriptAny(k) {
		s.scriptOffers(k, d, capSpent, p)
	}
}
