package kgen

import "github.com/adams-shaun/gorge/dirac/kop"

func init() {
	opsCardFields = append(opsCardFields, opsDoc{"card:UmbraArmor", "CardDef.UmbraArmor", "Printed Umbra armor (CR 702.89a; gorge effects.ReplaceUmbraArmor): were the enchanted permanent destroyed (a destroy effect, a sweeper, lethal damage, a deathtouch mark), it instead loses all its damage and the deathtouch mark and the first Aura attached to it that carries the keyword (battlefield order) is destroyed. Checked after R:Event$ Destroy lines, the regeneration shield (not NoRegen$-gated: umbra armor is not regeneration) and a shield counter; one Aura per destruction. Not for a toughness-0 death, a sacrifice or an exile; an Aura that lost its abilities does not apply. A granted umbra armor (Umbra Mystic) stays refused. Kernel kw_umbraarmor.go. Snake Umbra, Bear Umbra."})
}

// keywordFlagUsage records the printed-keyword card fields d carries.
func keywordFlagUsage(d *kop.CardDef, name string, add func(l, card string)) {
	if d.Wither {
		add("card:Wither", name)
	}
	if d.UmbraArmor {
		add("card:UmbraArmor", name)
	}
	if d.Dredge != 0 {
		add("card:Dredge", name)
	}
}
