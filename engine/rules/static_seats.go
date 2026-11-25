package rules

import "github.com/adams-shaun/gorge/engine/state"

// staticSeats lists the seats whose permanents' continuous statics apply, in
// APNAP order. A player who lost leaves the game as a state-based check
// applies (CR 800.4a), but that check's other actions happen simultaneously
// with the loss (CR 704.3): while sbaLossHeld the loser's permanents still
// carry their statics, so a creature that dies in the check is matched, by the
// types and abilities it had, as the creature it was (CR 603.10a) -- a Station
// Spacecraft's "it's an artifact creature at 4+" among them.
func (e *Engine) staticSeats() []state.PlayerID {
	if !e.sbaLossHeld {
		return e.G.AliveFrom(0)
	}
	// Every seat, departed or not, in the order AliveFrom(0) walks them
	// (reversed turn order included).
	n := state.PlayerID(len(e.G.Players))
	out := make([]state.PlayerID, 0, n)
	for i := state.PlayerID(0); i < n; i++ {
		p := i
		if e.G.ReverseTurnOrder {
			p = (n - i) % n
		}
		out = append(out, p)
	}
	return out
}
