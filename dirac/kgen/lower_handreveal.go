package kgen

// handRevealOnly reports whether a Mode$ Continuous body is exactly the
// view-only "play with ... hands revealed" look (Telepathy: the opponents'
// hands; the same static over Card.YouOwn reveals the controller's own):
//
//	Mode$ Continuous | AffectedZone$ Hand | Affected$ Card.OppOwn |
//	MayLookAt$ Player
//
// gorge records it as a ContinuousEffect with MayLookAt, MayLookAtAll and
// AffectedZone "Hand" (engine/rules/layers.go's MayLookAt branch). Every
// reader of that permission skips it: the library-top read
// (Engine.libraryTopLook, layers_static.go) passes over any AffectedZone
// other than Library, and the exile read (MayLookAtFaceDown) answers only
// for a face-down exiled card. A revealed hand changes no event, decision
// or legality (CR 400.2 names the hand hidden; revealing it is information
// only), so the exact kernel lowering is no static at all.
//
// The whitelist is exact: any other rules key (EffectZone$ Command, a
// condition), another Affected$ spelling (a remembered player's hand) or
// another MayLookAt$ value keeps the named refusal.
func handRevealOnly(p map[string]string) bool {
	for k := range p {
		switch k {
		case "Mode", "Affected", "AffectedZone", "MayLookAt", "Description", "Secondary":
		default:
			return false
		}
	}
	if p["AffectedZone"] != "Hand" || p["MayLookAt"] != "Player" {
		return false
	}
	switch p["Affected"] {
	case "Card.OppOwn", "Card.YouOwn":
		return true
	}
	return false
}
