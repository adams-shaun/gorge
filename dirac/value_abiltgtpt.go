package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// The last known power and toughness of an activated ability's object target
// (CR 608.2h). "Destroy target creature. Put loyalty counters on CARDNAME equal
// to that creature's toughness" (Garruk the Slayer's -10) reads the toughness
// after the Destroy has moved the creature, so the value is the one it had on
// the battlefield, +1/+1 counters and effects included, not its printed card's.
// A spell's target is snapshot as it starts resolving (resolve.go); an
// ability's only when some ability of the table reads its target's P/T
// (prep.anyAbilTgtPT), so every other table keeps the pool as it was.

// prepAbilTgtPT sets anyAbilTgtPT: some activated ability's program reads its
// target's power or toughness.
func prepAbilTgtPT(t *kop.Table, p *prep) {
	for i := range t.Cards {
		for j := range t.Cards[i].Abilities {
			a := &t.Cards[i].Abilities[j]
			if a.Kind != kop.AbActivated {
				continue
			}
			for _, in := range a.Prog {
				for _, v := range [2]kop.ValueRef{in.A, in.B} {
					if v.K == kop.VTargetPower || v.K == kop.VTargetToughness {
						p.anyAbilTgtPT = true
					}
				}
			}
		}
	}
}

// snapResolveTgtLKI snapshots the resolving entry e's object target as the
// resolution starts: a spell's always (Condemn), an activated ability's when
// the table reads an ability target's P/T. An entry that already holds a
// record keeps it: a sacrificed cost's last known information (RefSacrificed:
// Kazuul's Fury deals the sacrificed creature's power, not its target's).
func (s *State) snapResolveTgtLKI(k *prep, e *stackEntry) {
	if k.gen == nil || e.li != 0 || !isObjTarget(e.tgt[0]) {
		return
	}
	if e.kind == seSpell || e.kind == seAbility && k.anyAbilTgtPT {
		s.res.e.li = s.snapLKI(Obj(e.tgt[0]))
	}
}

// abilTgtLKIPT is the snapshot's power and toughness for ability e's first
// target o once it has left the battlefield; ok is false for any other entry.
func (s *State) abilTgtLKIPT(e *stackEntry, o Obj) (pw, t int, ok bool) {
	if e.kind != seAbility || e.li == 0 || s.lk(e.li).obj != o {
		return 0, 0, false
	}
	r := s.lk(e.li)
	return int(r.power), int(r.tough), true
}
