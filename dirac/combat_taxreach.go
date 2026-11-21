package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Table.Emblems and Table.Overrides are the pool's records, not the table's:
// a table holds every emblem and override its pool interns (13 and 96 in a
// real Commander pod), whichever cards are in play. The attack and block tax
// walks refuse a table over an emblem or an override of some kind only when a
// card of this table can make one (tableReach), so a pod without the creating
// card is not refused over a record another deck interned.

// taxReach is the pool records a table's cards can make.
type taxReach struct {
	emblem   []bool // Table.Emblems[i] some card creates (OpCreateEmblem)
	override []bool // Table.Overrides[i] some card or a created emblem applies
}

// tableReach scans every card of t, then the abilities of each emblem found,
// for OpCreateEmblem and for the override indexes the programs and statics
// carry: StatOverride (Override-1), OpOverride and OpOverrideAll (C; a
// chosen-type or chosen-colour record names the first of five), OpCopyAsTokenEx
// (C-1), OpBecomeCopy (C>>1 - 1, A.N - 1), OpAmass and OpEmpower (B.N&0xfff - 1).
func tableReach(t *kop.Table) taxReach {
	r := taxReach{emblem: make([]bool, len(t.Emblems)), override: make([]bool, len(t.Overrides))}
	var work [][]kop.Ability
	for i := range t.Cards {
		work = append(work, t.Cards[i].Abilities)
	}
	for len(work) > 0 {
		abs := work[len(work)-1]
		work = work[:len(work)-1]
		for j := range abs {
			a := &abs[j]
			if a.Kind == kop.AbStatic && a.Static.Kind == kop.StatOverride {
				r.markOv(int(a.Static.Override) - 1)
			}
			for k := range a.Prog {
				in := &a.Prog[k]
				switch in.Op {
				case kop.OpCreateEmblem:
					if c := int(in.C); c >= 0 && c < len(r.emblem) && !r.emblem[c] {
						r.emblem[c] = true
						work = append(work, t.Emblems[c].Abilities)
					}
				case kop.OpOverride, kop.OpOverrideAll:
					r.markOv(int(in.C))
					if in.Op == kop.OpOverride && in.B.K == kop.VConst && (in.B.N == kop.OvChosenType || in.B.N == kop.OvChosenColor) {
						for d := 1; d < 5; d++ {
							r.markOv(int(in.C) + d)
						}
					}
				case kop.OpCopyAsTokenEx:
					r.markOv(int(in.C) - 1)
				case kop.OpBecomeCopy:
					r.markOv(int(in.C>>1) - 1)
					if in.A.K == kop.VConst {
						r.markOv(int(in.A.N) - 1)
					}
				case kop.OpAmass, kop.OpEmpower:
					r.markOv(int(in.B.N&0xfff) - 1)
				}
			}
		}
	}
	return r
}

func (r *taxReach) markOv(i int) {
	if i >= 0 && i < len(r.override) {
		r.override[i] = true
	}
}

// mustAttackEmblem reports whether a card of t creates an emblem holding a
// MustAttack static (the walk does not model a requirement beside a tax).
func (r *taxReach) mustAttackEmblem(t *kop.Table, floor bool) bool {
	for i, made := range r.emblem {
		if !made {
			continue
		}
		for j := range t.Emblems[i].Abilities {
			a := &t.Emblems[i].Abilities[j]
			if a.Kind == kop.AbStatic && (a.Static.Kind == kop.StatMustAttack || floor && blkTaxFloorStatic(a)) {
				return true
			}
		}
	}
	return false
}

// landGrantLive reports whether the table's statics or reachable overrides
// add a basic land type (g.blgOn, narrowed to what a card of t can apply).
func (r *taxReach) landGrantLive(t *kop.Table, g *genPrep) bool {
	if !g.blgOn {
		return false
	}
	var basic kop.SubMask
	for _, s := range g.blgSub {
		if s != subOutside {
			basic = basic.With(int(s))
		}
	}
	meets := func(m kop.SubMask) bool { return m[0]&basic[0]|m[1]&basic[1] != 0 }
	for i := range t.Cards {
		for j := range t.Cards[i].Abilities {
			a := &t.Cards[i].Abilities[j]
			if a.Kind == kop.AbStatic && a.Static.Kind == kop.StatContinuous && meets(a.Static.AddSub) {
				return true
			}
		}
	}
	for i, ok := range r.override {
		if ok && meets(t.Overrides[i].AddSub) {
			return true
		}
	}
	return false
}
