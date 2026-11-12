package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// plExtremeValue lowers the zone-scoped extreme heads gorge's
// playerCountExtreme answers (engine/effects/count_player.go):
// PlayerCountPlayers$HighestValid<zones> <spec> and
// PlayerCountOpponents$HighestValid<zones> <spec>. The measured corpus
// population over the heads, the cards the 2026-11 measure refused by name:
//
//   - zones Graveyard, spec Card.YouOwn -- Jace, the Perfected Mind's Y
//     (DB$ Draw NumCards$ Y behind Count$Compare CheckGrave GE20.3.1),
//     Visions of Beyond's CheckGrave, Nightmares and Daydreams'
//     CheckGrave, Jotun Grunt's X;
//   - zones Graveyard,Library,Exile, spec Card.IsRemembered+YouOwn --
//     Windfall's, Jace's Archivist's and Whispering Madness' X (the
//     greatest number of cards a player discarded this way);
//   - zones Graveyard, spec Creature.YouOwn, group Opponents -- Nautiloid
//     Ship's X.
//
// The kernel reads ExPlExtreme: the highest, over the living group in seat
// order, of the member's count of matching objects in the named zones, each
// member read from their OWN side (gorge playerCountExtremeZones'
// SpecContext(p); the walker rebinds ctrl to the member, so a "You"/"Opp"
// word in the spec is the member's). Only the Highest form is lowered:
// every corpus carrier reads the highest, and gorge's Lowest extreme
// (LowestValidHand, ...) has no carrier over these heads. The named zones
// are only the three the corpus spells (Graveyard, Library, Exile).
func (lc *lowerCtx) plExtremeValue(head, rest string) (lValue, error) {
	prop, spec, _ := strings.Cut(rest, " ")
	group, zones, ok := plExtremeHead(head, prop)
	if !ok {
		return lValue{}, refuse("%s$%s", head, rest)
	}
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return lValue{}, refuse("%s$%s with no spec", head, rest)
	}
	f, err := lc.filter(spec)
	if err != nil {
		return lValue{}, err
	}
	if err = plExtremeView(f); err != nil {
		return lValue{}, err
	}
	return lValue{Kind: vPlExtreme, N: group | zones, F: f}, nil
}

// plExtremeHead parses "<Highest>Valid<zone words>" (gorge
// playerCountExtreme's prop grammar: the Highest prefix, then the zone
// words, one or a comma list, gorge playerCountZones' countZone and
// multiCountZones). A repeated zone, a Hand or Battlefield word or an
// unknown word drops the head, exactly as gorge's playerCountZones does
// (an unresolvable verdict, never a fake zero).
func plExtremeHead(head, prop string) (group, zones int, ok bool) {
	switch head {
	case "PlayerCountPlayers":
		group = kop.PlExtPlayers
	case "PlayerCountOpponents":
		group = kop.PlExtOpponents
	default:
		return 0, 0, false
	}
	if !strings.HasPrefix(prop, "HighestValid") {
		return 0, 0, false
	}
	for _, w := range strings.Split(strings.TrimPrefix(prop, "HighestValid"), ",") {
		bit := 0
		switch w {
		case "Graveyard":
			bit = kop.PlExtGrave
		case "Library":
			bit = kop.PlExtLibrary
		case "Exile":
			bit = kop.PlExtExile
		default:
			return 0, 0, false
		}
		if zones&bit != 0 {
			return 0, 0, false // a repeated zone: gorge drops the head too
		}
		zones |= bit
	}
	return group, zones, true
}

// plExtremeView rejects a lowered filter whose words name a player the
// walker cannot rebind. The walker reads every member from their own side
// (at.ctrl = p), which rebinds the spec's You and Opp words exactly as
// gorge's SpecContext(p) does; a qualifier bound to some other referent --
// a target player (TargetedPlayerOwn), the repeat player of a RepeatEach
// body (RememberedPlayerCtrl), the enchantment's controller, the defender,
// the source's combat damager -- names someone else, so the extreme would
// read the wrong member's side. Source-relative words keep the resolution's
// source (gorge's SpecContext swaps only the player words), so they stay.
func plExtremeView(f *lFilter) error {
	if f.Players != 0 {
		return refuse("the extreme's spec names players")
	}
	for _, c := range f.Clauses {
		for _, w := range [2]uint8{c.Ctrl, c.Owner} {
			switch w {
			case whoAny, whoYou, whoOpp:
			default:
				return refuse("the extreme's spec names a player outside the counted member")
			}
		}
	}
	return nil
}
