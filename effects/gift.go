package effects

// Gift (CR 702.168, Bloomburrow). "You may promise an opponent a gift as you
// cast this spell. If you do, [the gift happens]." The keyword is a casting
// OPTION, read rules-side rather than expanded in cards/keywords.go -- the
// kw:MayFlashSac precedent (its own doc names the family: a cast-time choice
// rules reads directly). So this file registers the two primitives the
// mechanic contributes and nothing else:
//
//   - kw:Gift is the keyword marker cards/primitive.go already names for every
//     `K:Gift` line. The real behaviour lives in rules: the cast-flow
//     election (rules/cast.go's giftAsk), the promise state folded by
//     events.GiftPromise, the gift body's execution at resolution
//     (rules/stack.go), the PromisedGift filter predicate
//     (effects/filter.go), the Count$PromisedGift head (effects/count.go) and
//     the Defined$ Promised / TokenOwner$ Promised referent
//     (effects/context.go).
//   - trig:GiveGift is the "whenever you give a gift" trigger family
//     (Jolly Gerbils). It matches the events.GiveGift marker Apply folds
//     when a promised gift is actually given, exactly the Investigate
//     shape -- a dedicated Kind, not a Note.
//
// RegisterNonAPI records both as implemented so effects.Supported() -- which
// feeds cards.Registry.Coverage and the `make report` playable count -- sees
// them; leaving them out would keep every carrier measured unsupported even
// though the behaviour is real.
func init() {
	RegisterNonAPI("kw:Gift", "trig:GiveGift")
}
