// Keyword expansion split out so tickets touching different keywords stop
// colliding on one file. Registered by head; a duplicate head panics
// (registerKeyword).

package cards

// kwFortify expands K:Fortify (CR 702.67) to the same AB$ Attach shape
// kw:Equip mints, with the land-facing defaults: a Fortification attaches
// to a LAND you control, only as a sorcery, and it enters unattached and
// stays on the battlefield if the land leaves (rules/attach.go's SBA
// detaches a non-Aura attachment whose bearer left, which is exactly that
// rule; the FortifiedBy filter predicate in effects/filter.go is what every
// Card.FortifiedBy / Land.FortifiedBy reader -- C.A.M.P.'s TapsForMana
// trigger, Darksteel Garrison's Affected$ static and Taps trigger --
// matches through). The corpus carries only the bare-cost shape ("3"), but
// the shared parser accepts the full colon-field grammar Equip does, so a
// future restriction line ("3:Land.YouCtrl+Snow") expands the same way.
func kwFortify(f *Face, i int, k, head, param string, has func(kind, line string) bool) {
	kwAttachCost(f, i, k, param, has, attachKWCfg{
		kw: "Fortify", defaultTgts: "Land.YouCtrl", prompt: "Select target land you control",
	})
}

func init() { registerKeyword(kwFortify, "Fortify") }
