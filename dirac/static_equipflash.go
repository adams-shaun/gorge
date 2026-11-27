package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// S:Mode$ CastWithFlash | ValidSA$ Activated.Equip | Caster$ You (kop.StatEquipFlash,
// Leonin Shikari; Forge Anew's copy under Condition$ PlayerTurn): its controller
// may activate Equip abilities any time they could cast an instant (CR 702.6;
// gorge rules equipAtInstantSpeed, which gates the offer's SorcerySpeed$ test
// on it). Timing only: costs, targets and the other activation gates stand.

// useEquipFlash: a card of the table has a StatEquipFlash static (a genPrep use
// bit, not a field: TestNoNewFixedStateFields).
var useEquipFlash = newGenUse(func(t *kop.Table) bool { return tableHasStatic(t, kop.StatEquipFlash) })

// equipFlash reports whether a live StatEquipFlash static of player p (its
// condition holding for p, statCond) lets p activate Equip ability a at instant
// speed.
func (s *State) equipFlash(k *prep, a *kop.Ability, p uint8) bool {
	if k.gen == nil || a.AFlags&kop.AfEquip == 0 || DebugEquipFlashOff || !k.gen.uses.has(useEquipFlash) {
		return false
	}
	hit := false
	s.eachStatic(k, func(st statSrc) bool {
		if st.ctrl == p && st.a.Kind == kop.AbStatic && st.a.Static.Kind == kop.StatEquipFlash && s.statCond(k, st) {
			hit = true
			return false
		}
		return true
	})
	return hit
}

// DebugEquipFlashOff is the lockstep mutation switch (test only): the static is
// ignored and Equip stays sorcery-speed, as before the lowering.
var DebugEquipFlashOff bool
