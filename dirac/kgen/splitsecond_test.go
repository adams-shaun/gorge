package kgen

import "testing"

// A split second grant to a controller's spells lowers to a stack keyword
// static the kernel reads off the spells on the stack; a grant keyed to the
// cast (provenance words) has no such read and is refused.
func TestSplitSecondStackGrant(t *testing.T) {
	const samut = "Samut, Tyrant of Naktamun"
	kp := kfhBuild(t, []string{samut}, nil)
	kp.accepted(t, samut)
	kp = kfhBuild(t, []string{samut}, func(p *pool) {
		c := kfhClone(p.byName[samut])
		c.Faces[0].Statics[0].Params["Affected"] = "Instant.wasCastByYou,Sorcery.wasCastByYou"
		p.byName[samut] = c
	})
	kp.refusedWith(t, samut, "is not a plain YouCtrl spell filter")
}
