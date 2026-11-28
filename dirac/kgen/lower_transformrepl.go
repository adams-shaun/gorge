package kgen

import "github.com/adams-shaun/gorge/engine/cards"

// transformRepl lowers an R:Event$ Transform line: "as this creature
// transforms into CARDNAME, <body>" (Sephiroth, One-Winged Angel's Super
// Nova). gorge matches the FlipFace of the card turning into the face that
// prints the line (replacement_match.go cards.ReplTransform, ValidCard$ read
// in the source's frame), runs the ReplaceWith$ body, and leaves the flip
// intact (replacement_phase.go applyTransformReplacement: an augmentation,
// not a substitute). The kernel runs the destination face's kop.ReplTransform
// program before the flip (act_transform.go transformRepls).
//
// Only Card.Self with a body that lowers to emblem creations alone (DB$
// Effect, Duration$ Permanent or not) lowers: the kernel runs the program
// inline in the flip, so a body that asks or targets has no place to suspend.
func (lc *lowerCtx) transformRepl(rp *params, with *cards.SA, rl *lRepl) error {
	const shape = "Transform replacement shape"
	if with == nil || rp.str("ValidCard") != "Card.Self" || with.API != "Effect" || with.Sub != nil {
		return refuse(shape)
	}
	for _, k := range []string{"Layer", "Optional", "ActiveZones"} {
		if _, ok := rp.m[k]; ok {
			return refuse(shape)
		}
	}
	ab := &lAbility{Kind: akLatent}
	if err := lc.withAbility(ab, func() error { return lc.chain(with, nil) }); err != nil {
		return err
	}
	if len(ab.Prog) == 0 {
		return refuse(shape)
	}
	for _, in := range ab.Prog {
		if in.Op != "CreateEmblem" {
			return refuse("Transform replacement body op %s", in.Op)
		}
	}
	rl.Prog = ab.Prog
	rl.Kind = "TransformProgram"
	return nil
}
