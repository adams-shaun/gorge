package effects

import (
	"strconv"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

func init() {
	Register("MultiplePiles", effMultiplePiles)
}

// effMultiplePiles implements Forge's MultiplePilesEffect for the shape
// cards.ParseMultiplePiles names (Whims of the Fates): starting with the
// controller, each player separates the permanents the ValidCards$ spec
// matches among those they control into Piles$ piles (Piles$-1 subset asks,
// the last pile is the offered-order rest, any pile may be empty); then a pile
// is picked for each of those players at random from the engine's seeded rng
// (Host.Rand, so a replay re-derives it) and the ChosenPile$ body runs ONCE
// over the union of the picked piles (Whims's SacrificeAll ValidCards$
// Card.IsRemembered), as twoPilesRunPile does for a TwoPiles pile.
//
// A player with no matching permanents asks nothing and draws nothing. A
// one-permanent set skips the split asks (nobody could answer differently)
// but still draws the random pile. The no-host stand-in puts the first
// permanent in the first pile and the rest in the last. Any other shape is
// ONE loud Note and nothing moves (cards.MultiplePilesShapeSupported keeps
// coverage in step).
func effMultiplePiles(h Host, c *Ctx, sa *cards.SA) {
	shape, ok := cards.ParseMultiplePiles(sa)
	if !ok {
		h.Emit(events.Event{Kind: events.Note, Obj: c.Source,
			Text: "unimplemented MultiplePiles shape"})
		return
	}
	body := twoPilesBodyFor(h, c, sa.ParamStr(cards.PKChosenPile), "ChosenPile")
	if body == "" {
		return // twoPilesBodyFor noted an unresolvable body
	}
	g := h.Game()
	players := g.AliveFrom(c.Controller)
	if shape.You {
		players = []state.PlayerID{c.Controller}
	}
	// Every separation first (CR 101.4 order, controller first), then the
	// random picks, as the Oracle text sequences them.
	sets := make([][][]state.ObjID, 0, len(players))
	for _, p := range players {
		pool := twoPilesBattlefieldSet(h, c, sa, p)
		if len(pool) == 0 {
			sets = append(sets, nil)
			continue
		}
		sets = append(sets, multiplePilesSeparate(h, c, sa, shape.Piles, p, pool))
	}
	var chosen []state.ObjID
	for _, piles := range sets {
		if piles != nil {
			chosen = append(chosen, piles[h.Rand(len(piles))]...)
		}
	}
	twoPilesRunPile(h, c, body, chosen)
}

// multiplePilesSeparate asks sep to divide pool into n piles: for each of the
// first n-1 piles a KChoose subset pick (Min 0) over the cards not yet placed;
// the remainder is the last pile.
func multiplePilesSeparate(h Host, c *Ctx, sa *cards.SA, n int, sep state.PlayerID, pool []state.ObjID) [][]state.ObjID {
	g := h.Game()
	piles := make([][]state.ObjID, 0, n)
	rest := pool
	for i := 0; i < n-1; i++ {
		if len(pool) == 1 || len(rest) == 0 {
			piles = append(piles, nil)
			continue
		}
		d := &decision.Decision{Player: sep, Kind: decision.KChoose,
			Min: 0, Max: len(rest), Source: c.Source,
			ResumeKind: "multiplepiles_split", ResumeSA: sa,
			Prompt: "Separate your permanents into " + strconv.Itoa(n) + " piles: pick the cards of pile " + strconv.Itoa(i+1)}
		for _, id := range rest {
			name := "a card"
			if o := g.Obj(id); o != nil && o.Face() != nil {
				name = o.Face().Name
			}
			d.Options = append(d.Options, decision.Option{Index: len(d.Options),
				Kind: "multiplepiles", Label: name, Obj: id, Player: sep})
		}
		pick := []state.ObjID{}
		if i == 0 {
			pick = append(pick, rest[0]) // no-host stand-in
		}
		if ans, ok := AskTape(h, d); ok {
			pick = counterAnswerObjs(ans)
		}
		in := idSet(rest)
		var pile []state.ObjID
		for _, id := range pick {
			if in[id] {
				pile = append(pile, id)
				delete(in, id)
			}
		}
		piles = append(piles, pile)
		rest = restOf(rest, idSet(pile))
	}
	return append(piles, rest)
}
