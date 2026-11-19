package oracle

import "testing"

// Channel costs spelled Discard<1/NICKNAME> at four seats: Forge writes the
// source's nickname where it means the card itself, and gorge's pay.costparts
// reads it exactly as CARDNAME (Arashi, the Sky Asunder; Jiwari, the Earth
// Aflame). Fliers (Wind Drake, Serra Angel) and ground creatures fill the
// decks so the two sweeps, X damage to each creature with and without
// flying, hit different boards. The expected answer at every decision is
// gorge's.
var channelNickDecks = [4][]string{
	podDeck(deckOf(4, "Arashi, the Sky Asunder"), deckOf(4, "Wind Drake"), deckOf(4, "Grizzly Bears"), deckOf(4, "Llanowar Elves"), deckOf(24, "Forest")),
	podDeck(deckOf(4, "Jiwari, the Earth Aflame"), deckOf(4, "Serra Angel"), deckOf(4, "Raging Goblin"), deckOf(4, "Shock"), deckOf(12, "Mountain"), deckOf(12, "Plains")),
	podDeck(deckOf(3, "Arashi, the Sky Asunder"), deckOf(3, "Jiwari, the Earth Aflame"), deckOf(4, "Wind Drake"), deckOf(4, "Spined Wurm"), deckOf(12, "Forest"), deckOf(12, "Mountain")),
	podDeck(deckOf(4, "Jiwari, the Earth Aflame"), deckOf(4, "Craw Wurm"), deckOf(4, "Air Elemental"), deckOf(4, "Savannah Lions"), deckOf(12, "Mountain"), deckOf(12, "Plains")),
}

func TestChannelNicknamePodLockstep(t *testing.T) {
	t.Parallel()
	podFixtureLockstep(t, "channelnick", channelNickDecks, 24, true, nil)
}
