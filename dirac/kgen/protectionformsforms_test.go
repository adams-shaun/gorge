package kgen

import "testing"

// protectionformsForms: the protection keyword forms beyond the printed K:
// lines, checked as staticConditionForms are. The granted spellings lower as
// data -- a colour mask, plus an optional quality filter -- never as a kop
// bit per colour (KwMask has no free bits for white, blue or green):
//
//   - "Protection from <colour>" and "Protection from each color" in a
//     Continuous static's AddKeyword$ and in a Pump's or Animate's KW$ lower
//     to colour clauses of the grant's quality filter (kop.Static.ProtFrom /
//     kop.Override.ProtFrom; the kernel reads the clauses beside the printed
//     CardDef.ProtFrom at the damage, block, target and attach sites).
//   - "Protection:<spec>:<desc>" lowers through lc.filter into the grant's
//     quality filter (kop.Override.ProtFrom for an override grant, the
//     existing kop.Static.ProtFrom for a static); an exception field still
//     refuses, and so does any spec the filter cannot lower.
//   - On a host that is neither artifact nor enchantment, the colour
//     spellings keep pumpKeywords' printed route: the black and red ones
//     lower to the KwProtBlack / KwProtRed bits (Mystic Familiar), the rest
//     refuse there as they always have.
//
// "Protection from everything" is the KwProtectionAll bit as before, not a
// colour.
var protectionformsForms = []formCase{
	{card: "Sword of Fire and Ice", form: "AddKeyword$ Protection from red & Protection from blue (Static.ProtFrom colour clauses)", lir: "static Continuous F={types=0x2 EquippedBy} F2=nil cond=true P=2 T=2 kw=[] addsub=[] amount=- zone=0 protfrom={types=0x0 color=R | types=0x0 color=U}"},
	{card: "Eldritch Immunity", form: "Pump KW$ Protection from each color (Override.ProtFrom)", lir: "override{protfrom={types=0x0 color=WUBRG}}"},
	{card: "Figure of Fable", form: "Animate Keywords$ Protection:Player.Opponent:desc (Override.ProtFrom)", lir: "override{pt=7/8 addsub=Kithkin,Avatar rmcreaturesub protfrom={types=0x0 ctrl=2}}"},
	{card: "Mystic Familiar", form: "Continuous AddKeyword$ Protection from black on a creature host (pumpKeywords' KwProtBlack route)", lir: "kw=[ProtectionFromBlack]"},
}

func TestProtectionFormsForms(t *testing.T) { checkForms(t, protectionformsForms) }
