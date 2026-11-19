package oracle

import "testing"

// Weatherlight Compleated (a creature at four or more phyresis counters) is
// destroyed and its own "Whenever a creature you control dies, put a phyresis
// counter on Weatherlight Compleated. Then draw a card if it has seven or more
// phyresis counters on it. If it doesn't, scry 1" fires on its death. gorge
// reads the condition off the source's last known information (CR 608.2h)
// and draws (Sunspire Gatekeepers, the drawn card, is the one evloop
// blamed); the kernel matched the counterless graveyard card and scried
// (tail-33 pair 46, decision 1109, Cards.Pos). A RefSelf condition whose
// source has left now reads the departed slot.
func TestMBCSunspireGatekeepersReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "self_condition_departed_source_lki")
}
