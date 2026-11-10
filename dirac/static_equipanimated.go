package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// prepEquipAnim sets genPrep.equipAnim: CR 704.5p's Equipment arm of sba is
// reachable only when a permanent can gain or lose the Equipment type while
// it is attached. That takes an Equipment card (kop.TEquipment: an Equipment
// can be attached without ever having an Equip ability, by its own ETB trigger
// or an effect — Cryptic Coat) or an Equip ability (kop.AfEquip), and an
// animator that can reach an Equipment: an override that adds or sets the
// Creature type and is carried by an Equipment itself (Ensouled Scimitar), by
// an effect on another object (Karn's Touch, Relic's Roar), by a static whose
// affected filter admits an artifact (Ensoul Artifact) or by a copy's
// exception; or the reverse, an override that adds or sets Equipment
// (Domesticated Watercourse's Animate), whose end leaves an attachment with no
// Equipment type. A creature-land or Vehicle animating itself (Soulstone
// Sanctuary, Tangle Tumbler) and a static over creatures only (Witness
// Protection) cannot, so FDN and BLB leave the flag false and keep the arm off
// sba's per-permanent path. (An Equip never targets its own Equipment: kgen
// lowers its target filter with FOther, so targetOK needs no flag.)
func prepEquipAnim(t *kop.Table, g *genPrep) {
	equip := false
	for i := 0; i < len(t.Cards) && !equip; i++ {
		if t.Cards[i].Types&kop.TEquipment != 0 { // an Equipment need not have Equip (Cryptic Coat)
			equip = true
			break
		}
		for j := range t.Cards[i].Abilities {
			if t.Cards[i].Abilities[j].AFlags&kop.AfEquip != 0 {
				equip = true
				break
			}
		}
	}
	if !equip {
		return
	}
	creatureOvr := func(o uint16) bool {
		return int(o) < len(t.Overrides) && (t.Overrides[o].AddTypes|t.Overrides[o].SetTypes)&kop.TCreature != 0
	}
	// equipmentOvr: the override makes a permanent an Equipment (Domesticated
	// Watercourse's Animate). When its duration ends the derived types lose
	// Equipment and an attachment left behind is illegal (CR 704.5p, the
	// "lost Equipment" half sbaAttachedCreature models).
	equipmentOvr := func(o uint16) bool {
		return int(o) < len(t.Overrides) && (t.Overrides[o].AddTypes|t.Overrides[o].SetTypes)&kop.TEquipment != 0
	}
	// filterMayEquip: the filter can match an Equipment (an alternative, or no
	// type restriction, or an artifact/Equipment type)
	filterMayEquip := func(f kop.FilterID) bool {
		for {
			fl := &t.Filters[f]
			if fl.Types == 0 || fl.Types&(kop.TArtifact|kop.TEquipment) != 0 {
				return true
			}
			if fl.Or == 0 {
				return false
			}
			f = fl.Or
		}
	}
	// earthbend makes a land a creature, and a static that makes artifacts
	// lands (Toph, the First Metalbender) lets that land be an Equipment
	landOvr := func(o uint16) bool {
		return int(o) < len(t.Overrides) && (t.Overrides[o].AddTypes|t.Overrides[o].SetTypes)&kop.TLand != 0
	}
	bend, landEquip := false, false
	for i := range t.Cards {
		c := &t.Cards[i]
		for j := range c.Abilities {
			a := &c.Abilities[j]
			if a.Kind == kop.AbStatic && a.Static.Kind == kop.StatOverride && a.Static.Override > 0 &&
				landOvr(a.Static.Override-1) && filterMayEquip(a.Static.Filter) {
				landEquip = true
			}
			for _, in := range a.Prog {
				bend = bend || in.Op == kop.OpEarthbend
			}
			if bend && landEquip {
				g.equipAnim = true
				return
			}
			if a.Kind == kop.AbStatic && a.Static.Kind == kop.StatOverride && a.Static.Override > 0 &&
				(creatureOvr(a.Static.Override-1) && filterMayEquip(a.Static.Filter) || equipmentOvr(a.Static.Override-1)) {
				g.equipAnim = true
				return
			}
			for _, in := range a.Prog {
				if in.Op == kop.OpExt && in.Ext() == kop.ExtCloneCopy {
					g.equipAnim = true // a permanent become a copy can stop being an Equipment (Shuri)
					return
				}
				if x := copyExcOf(&in); x > 0 && creatureOvr(uint16(x-1)) {
					g.equipAnim = true // a copy of an Equipment made a creature (Nexus of Becoming)
					return
				}
				switch {
				case in.Op == kop.OpOverride && (creatureOvr(uint16(in.C)) || equipmentOvr(uint16(in.C))):
					if equipmentOvr(uint16(in.C)) || in.Obj != kop.RefSelf || c.Types&kop.TEquipment != 0 {
						g.equipAnim = true
						return
					}
				case in.Op == kop.OpOverrideAll && (creatureOvr(uint16(in.C)) && filterMayEquip(in.F) || equipmentOvr(uint16(in.C))):
					g.equipAnim = true
					return
				}
			}
		}
	}
}
