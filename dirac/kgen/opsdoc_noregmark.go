package kgen

// opsdoc_noregmark.go: the OPS.md row for OpMarkNoRegen (gap-effect-containers).
func init() {
	opsEntries = append(opsEntries, opsDoc{
		"op:MarkNoRegen",
		"OpMarkNoRegen (Obj target or remembered)",
		"The battlefield creature of Obj takes the mechNoRegen slot mark (the arena's can't-be-regenerated mask; CR 701.19c): its regeneration shields stay but are never spent, so a destruction or lethal damage destroys it. The mark ends at the turn's cleanup and when the slot takes a new permanent (a creature that leaves the battlefield and returns is a new object, CR 400.7). With Obj RefRemembered every remembered object still a battlefield creature is marked (up to four); a ref that resolves to nothing marks nothing. No decision. Gorge: an Effect whose one static is Mode$ CantRegenerate over ValidCard$ Card.IsRemembered, with RememberObjects$ Targeted or Remembered and ForgetOnMoved$ Battlefield, no ValidTgts$ of its own and no Duration$ (the restriction is registered over the remembered id and forgotten as it moves); the shape without ForgetOnMoved$ Battlefield, or with a Duration$, is refused by kgen because the kernel record has no such lifetime. Furnace Brood, Hurr Jackal, Orcish Healer, Gravebind.",
	})
}
