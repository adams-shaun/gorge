package kgen

// opsControlSpell is the OPS.md family for the stack control transfer (an
// api:ControlSpell Mode$ Gain effect). opsEntries (opsdoc.go) places it in
// the document; a new item goes in its own opsdoc_<mechanic>.go.
var opsControlSpell = []opsDoc{
	{"ext:ControlSpell", "OpExt ExtControlSpell (Obj the cast's spell target slot)", "The resolving controller gains control of the spell on the stack that Obj names (api ControlSpell Mode$ Gain, Commandeer's \"Gain control of target noncreature spell\"; CR 613.1b layer 2 on the stack, gorge effControlSpell's Gain arm emitting one stack-scoped ControlChange, choose_control.go): the entry's controller becomes the resolving controller, so the artifact, enchantment or planeswalker the spell becomes enters under them (resolve.go enter's e.ctrl). An object that is not on the stack, and a stack ability (gorge's Object-on-the-stack gate), transfer nothing. No duration and no control records: a stack entry has none, and the permanent keeps the controller it entered under. NewController$ (gorge controlPlayer naming a player) and any other Mode$ refuse. dirac/act_controlspell.go. Commandeer."},
}

func init() { opsEntries = append(opsEntries, opsControlSpell...) }
