package kgen

// The controller of a damage trigger's source: Defined$
// TriggeredSourceController and OptionalDecider$ TriggeredSourceController
// on a plain DamageDone line ("whenever a creature deals combat damage to
// one of your opponents, its controller may draw a card": Edric, Spymaster
// of Trest).
//
// gorge reads the source the causing event recorded and answers its
// controller (effects.TriggeredSourceController: the live object's
// Controller, the damaged object's being the Remembered fallback only when
// no source was recorded). The kernel's DamageDone queue carries the damage
// source as the entry's trigger object whenever the recipient is not a
// per-referent object (ddoLine / ddoRecipient return srcObj for a player
// recipient and for a line with no referent read), and RefTriggerTgtCtrl
// is the controller of that trigger object read live at resolution, its
// owner once it left the battlefield, as gorge's zone moves reset Controller
// to Owner. A per-referent line (Mode$ DamageDoneOnce, an object recipient
// filter) carries the damaged object instead and stays refused.

// dmgSourceIsTriggerObj reports whether a DamageDone line's kernel trigger
// object is the damage source: not a per-referent line, no object recipient
// filter.
func dmgSourceIsTriggerObj(tr *lTrigger) bool {
	return tr != nil && tr.Event == "DamageDone" && !tr.PerRef && !tr.Batch &&
		(tr.F2 == nil || len(tr.F2.Clauses) == 0)
}
