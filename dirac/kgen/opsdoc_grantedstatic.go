package kgen

import "github.com/adams-shaun/gorge/dirac/kop"

// grantUsage records the grant fields st carries.
func grantUsage(st *kop.Static, name string, add func(l, card string)) {
	if st.Grant != 0 {
		add("st:Grant", name)
	}
	if st.GrantSt != 0 {
		add("st:GrantSt", name)
	}
}

func init() {
	opsStaticFields = append(opsStaticFields, opsDoc{"st:GrantSt", "Static.GrantSt + CardDef.Granted", "A Continuous static's AddStaticAbility$ grant: the affected objects each have the static CardDef.Granted[GrantSt-1], read with the recipient as its source and the recipient's controller as you (gorge layers.go AddStaticAbility registers one instance per recipient). Not one of the card's own statics: Abilities never lists it, and only a reader that walks eachStaticG sees it (the maximum hand size of Nerd Rage's enchanted creature's controller)."})
}
