package dirac

// The layer timestamp of a graveyard-functioning static (static_gycont.go).
// gorge builds the graveyard static with Object.LayerTimestamp (layers.go),
// and an object's stamps are written only on the battlefield: at entry
// (apply_zone.go) and by a restamp on attach or transform (restampLayer). A
// card in the graveyard therefore keeps the layer stamp it had when it last
// left the battlefield, and a card that was never on one has none (0). So a
// loses-all-abilities Aura attached after the card entered the battlefield
// removes the grant, and one attached before it does not (CR 613.1f, 613.7).
//
// The kernel keeps that stamp, State.lstamp's value at the departure, in a
// cold per-object region only a table with such a static has
// (a card with sclsGyCont); the stamp is read by grantTs.

// DebugGraveyardGrantTsZero (TestGraveyardStaticGrantTimestampMutation)
// restores the old behaviour: a graveyard source's grant timestamp is 0,
// older than every loss of abilities.
var DebugGraveyardGrantTsZero bool

// coldGyStamp is the per-object layer stamp a card kept on leaving the
// battlefield, for a table with a graveyard Continuous static.
var coldGyStamp = newCold[uint16]("gyStamp", func(g *genPrep) int {
	for _, m := range g.sCls { // per card, set by prepStaticClasses before prepCold
		if m&sclsGyCont != 0 {
			return MaxObj
		}
	}
	return 0
})

// gyStampSet records ts as the layer stamp object o carries off the
// battlefield.
func (s *State) gyStampSet(o Obj, ts uint16) {
	if r := coldGyStamp.of(s); int(o) < len(r) {
		r[o] = ts
	}
}

// gyStampClr ends the stamp of an object id a new object reuses (CR 400.7).
func (s *State) gyStampClr(o Obj) { s.gyStampSet(o, 0) }

// gyStampOf is the layer timestamp of the graveyard object o's statics.
func (s *State) gyStampOf(o Obj) uint16 {
	if DebugGraveyardGrantTsZero {
		return 0
	}
	if r := coldGyStamp.of(s); int(o) < len(r) {
		return r[o]
	}
	return 0
}
