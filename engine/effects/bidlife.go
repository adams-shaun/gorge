package effects

import (
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

func init() { Register("BidLife", effBidLife) }

// effBidLife implements SP$ BidLife (Forge's BidLifeEffect: Illicit Auction,
// Mages' Contest, Pain's Reward -- "You start the bidding with a bid of N.
// In turn order, each player may top the high bid. The bidding ends if the
// high bid stands. The high bidder loses life equal to the high bid and ...").
//
// One resolution:
//  1. The bidders are the resolving controller plus every LIVING player
//     OtherBidder$ names (Mages' Contest's TargetedController); with no
//     OtherBidder$ every living player bids. Turn order is AliveFrom's, the
//     same one the printed "in turn order" reads.
//  2. The opening bid is StartBidding$ as written: a literal number
//     (Mages' Contest's 1), Any (Pain's Reward: the controller picks, asked
//     in place, the deterministic no-host pick 0), or absent (Illicit
//     Auction's 0). The controller opens as the high bidder.
//  3. The bidding runs laps in turn order starting AFTER the current high
//     bidder. A lap asks each other bidder in turn to TOP the high bid
//     (bid a higher number) or pass; topping makes the bidder the high
//     bidder and starts a new lap after them. A lap in which nobody topped
//     ends the bidding (the printed "the high bid stands"). A bidder never
//     tops their own high bid, and a bid is bounded by the bidders' highest
//     life total (the honest ceiling for both the opening and the top ask:
//     a bid past it is only ever a seat's own death), so the laps always
//     terminate. The deterministic no-host answer is PASS, the first
//     option, so an askless engine ends the bidding at the opening bid.
//  4. The winner (the high bidder) is remembered as a PLAYER target and
//     their bid is recorded as the source's chosen number, then
//     BidSubAbility$ resolves through the ordinary sub-ability chain: the
//     `Defined$ Remembered` / `NewController$ Remembered` riders read the
//     winner, `LifeAmount$ X` with `SVar:X:Count$ChosenNumber` reads the
//     bid, and Mages' Contest's counter reads
//     `ConditionPlayerDefined$ Remembered | ConditionPlayerContains$ You`
//     through the shared condition gate.
func effBidLife(h Host, c *Ctx, sa *cards.SA) {
	g := h.Game()
	bidders := bidLifeBidders(h, c, sa)
	if len(bidders) == 0 {
		return
	}
	bidderSet := make(map[state.PlayerID]bool, len(bidders))
	for _, p := range bidders {
		bidderSet[p] = true
	}
	// The ask ceiling: the bidders' highest life total (CR 107.3i's bound is
	// the game's honest one; bidding past your own life is still legal, the
	// list just never offers more than the table could pay for).
	cap := int32(0)
	for _, p := range bidders {
		if life := g.Players[p].Life; life > cap {
			cap = life
		}
	}
	holder, high := c.Controller, int32(0)
	// StartBidding$ is a number or the one word `Any` (the opener names the
	// opening bid): a non-numeric, non-empty spelling is the word.
	open := strings.TrimSpace(sa.ParamStr(cards.PKStartBidding))
	openN, openErr := strconv.Atoi(open)
	switch {
	case open != "" && openErr != nil:
		d := bidLifeNumberDecision(c, sa, c.Controller, 0, cap,
			"Choose your opening bid", "bid_open")
		if ans, ok := AskTape(h, d); ok && len(ans) > 0 {
			high = int32(ans[0].Amount)
		}
	default:
		if openErr == nil && openN > 0 {
			high = int32(openN)
		}
	}
	// The laps. ringAfter lists the bidders in turn order starting after
	// holder; a lap where nobody topped ends the bidding.
	for {
		topped := false
		for _, p := range ringAfter(g, holder, bidderSet) {
			if p == holder {
				continue
			}
			if high >= cap {
				break // no bid above the high bid is offerable to anyone
			}
			d := bidLifeNumberDecision(c, sa, p, high+1, cap,
				"Top the high bid ("+strconv.Itoa(int(high))+") or pass", "bid_top")
			// The pass option is index 0 and every top renumbers after it,
			// so an option's Index is always its position (Validate's
			// invariant).
			d.Options = append([]decision.Option{{Index: 0, Kind: "pass",
				Label: "Pass", Player: p}}, d.Options...)
			for i := range d.Options {
				d.Options[i].Index = i
			}
			if ans, ok := AskTape(h, d); ok && len(ans) > 0 && ans[0].Index != 0 {
				high, holder = int32(ans[0].Amount), p
				topped = true
				break
			}
		}
		if !topped {
			break
		}
	}
	// The winner: remembered as a player target for the sub-ability riders,
	// the bid recorded as the source's chosen number for
	// `SVar:X:Count$ChosenNumber`, one public Note naming both.
	c.Remembered = append(c.Remembered, state.Target{Player: holder, IsPlayer: true})
	rememberPlayerBothHalves(h, c, holder)
	h.Emit(events.Event{Kind: events.Choose, Obj: c.Source, Counter: "number", Amount: high})
	h.Emit(events.Event{Kind: events.Note, Obj: c.Source, Player: holder,
		Text: "wins the bidding at " + strconv.Itoa(int(high)) + " life"})
	if name := strings.TrimSpace(sa.ParamStr(cards.PKBidSubAbility)); name != "" {
		Resolve(h, c, cards.ResolveSVar(c.SVars, name))
	}
}

// bidLifeBidders is the deterministic bidder list: the resolving controller,
// then (in order, deduped, living only) the players OtherBidder$ names; with
// no OtherBidder$ every living player joins.
func bidLifeBidders(h Host, c *Ctx, sa *cards.SA) []state.PlayerID {
	g := h.Game()
	out := []state.PlayerID{c.Controller}
	seen := map[state.PlayerID]bool{c.Controller: true}
	add := func(p state.PlayerID) {
		if int(p) >= len(g.Players) || seen[p] || g.Players[p].Lost {
			return
		}
		seen[p] = true
		out = append(out, p)
	}
	if spec := strings.TrimSpace(sa.ParamStr(cards.PKOtherBidder)); spec != "" {
		for _, p := range definedPlayerIDs(h, c, spec) {
			add(p)
		}
		return out
	}
	for _, p := range g.AliveFrom(0) {
		add(p)
	}
	return out
}

// ringAfter lists the bidders in turn order starting after holder (the one
// seat no lap asks: a bidder never tops their own high bid).
func ringAfter(g *state.Game, holder state.PlayerID, bidders map[state.PlayerID]bool) []state.PlayerID {
	var out []state.PlayerID
	for _, p := range g.AliveFrom(g.NextAlive(holder)) {
		if bidders[p] {
			out = append(out, p)
		}
	}
	return out
}

// bidLifeNumberDecision is the one number ask both bid phases pose: every
// value from lo through cap as its own option, in ascending order, so the
// first legal option is the smallest honest bid.
func bidLifeNumberDecision(c *Ctx, sa *cards.SA, p state.PlayerID, lo, cap int32, prompt, resume string) *decision.Decision {
	d := &decision.Decision{Player: p, Kind: decision.KChoose, Min: 1, Max: 1,
		Source: c.Source, ResumeKind: resume, ResumeSA: sa,
		Prompt: prompt}
	for v := lo; v <= cap; v++ {
		d.Options = append(d.Options, decision.Option{Index: len(d.Options),
			Kind: resume, Label: "Bid " + strconv.Itoa(int(v)), Amount: int(v), Player: p})
	}
	return d
}
