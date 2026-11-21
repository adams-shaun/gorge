package kgen

import "github.com/adams-shaun/gorge/dirac/kop"

func init() {
	opsEntries = append(opsEntries,
		opsDoc{"card:Spectacle", "CardDef.HasSpectacle + SpectacleCost", "K:Spectacle:<cost> (CR 702.137a; no free KwMask bit, so its own field): the hand offers the spectacle cast at SpectacleCost (gorge's \"spectacle\" cast option, altcast_modes.go altSpectacle, a potential play) while some living opponent of the caster lost life this turn (any of the three at four seats). The cast rides stackEntry.mode's impending bit (kgen refuses it beside every other cast route, on an {X} or modal spell and on a multi-face card), and the permanent it produces carries the kicked mark, which the `spectacle` filter word (Card.Self+spectacle, Card.spectacle: Rafter Demon, Rix Maadi Reveler) reads as F2Kicked. A GRANTED Spectacle stays refused. dirac/kw_spectacle.go."},
		opsDoc{"card:Riot", "CardDef.Riot", "Printed Riot (CR 702.136a; kop.KwRiot; gorge cast_etbchoice.go faceETBChoice): the controller answers an as-enters ask on every face-up battlefield entry (a face-down manifest or cloak poses nothing, CR 708.5) -- mode 0 the +1/+1 counter (placed as an entry counter), mode 1 haste, which holds as a base keyword for as long as the permanent stays (the arena's mechRiotHaste mark). A cast creature's ask rides the pre-entry ask with the spell on the stack. A GRANTED Riot (Rhythm of the Wild, Uncivil Unrest, Spider-Punk's Spiders) stays refused: gorge reads the printed keyword only. Riot beside Unleash or an as-enters choice is refused. dirac/kw_riot.go."})
	opsCardFlagUsage = append(opsCardFlagUsage,
		struct {
			label string
			on    func(d *kop.CardDef) bool
		}{"card:Spectacle", func(d *kop.CardDef) bool { return d.HasSpectacle }},
		struct {
			label string
			on    func(d *kop.CardDef) bool
		}{"card:Riot", func(d *kop.CardDef) bool { return d.Riot }})
}
