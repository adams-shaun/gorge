package sanity

// allowList is the SORTED set of card names the guard never drops (see Drops):
// cards whose use on the seat's own permanent is sometimes deliberate. Add a
// name only with the census line that justifies it. Names are matched against
// the card name carried in the option label.
//
// Seed, from a census of the 15 constructed repo decks (every
// card whose first target step is hostile and can have only own targets:
// Doom Blade, Fatal Push, Ghost Quarter, Hero's Downfall, Karn Liberated, Path
// to Exile, Swords to Plowshares, Wasteland):
//
//   - Ghost Quarter: thinning your own land is a real line.
//
// Wasteland (172 of the default bot's 291 self-hits), Swords, Path, Fatal Push,
// Downfall, Doom Blade and Karn were judged mistakes and stay guarded.
var allowList = []string{
	"Ghost Quarter",
}
