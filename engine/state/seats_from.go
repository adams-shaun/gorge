package state

// SeatsFrom lists every seat, departed or not, in the order AliveFrom walks
// them from start (reversed turn order included). A scan that must still see
// the permanents of a player who lost in the state-based check now being
// applied (CR 704.3) walks these instead of the survivors.
func (g *Game) SeatsFrom(start PlayerID) []PlayerID {
	n := PlayerID(len(g.Players))
	out := make([]PlayerID, 0, n)
	for i := PlayerID(0); i < n; i++ {
		p := start + i
		if g.ReverseTurnOrder {
			p = start + n - i
		}
		if p >= n {
			p %= n
		}
		out = append(out, p)
	}
	return out
}
