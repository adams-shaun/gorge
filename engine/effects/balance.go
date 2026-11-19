package effects

import (
	"slices"
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

func init() { Register("Balance", effBalance) }

// effBalance implements DB$/AB$ Balance (Forge's BalanceEffect, Magus of the
// Balance's "{4}{W}, {T}, Sacrifice ...: Each player chooses a number of
// lands they control equal to the number of lands controlled by the player
// who controls the fewest, then sacrifices the rest. Players discard cards
// and sacrifice creatures the same way."; the Magus runs the effect three
// times through its SubAbility$ chain -- lands on the battlefield, then
// Zone$ Hand, then creatures).
//
// One run:
//  1. The zone is Zone$ (Hand discards; every other value, including absent,
//     is the battlefield and sacrifices).
//  2. Each LIVING player's matching item count is read UP FRONT, over the
//     board before anything moves (CR 608.2h), through Valid$ (default:
//     the whole hand when Zone$ is Hand, every permanent otherwise) judged
//     from each player's own zone.
//  3. The fewest of those counts is the number every player keeps. Each
//     player holding MORE than the fewest chooses WHICH of their own items
//     to keep (the Oracle's wording: "chooses a number of lands they control
//     equal to the number ... controlled by the player who controls the
//     fewest"), asked in AliveFrom's APNAP seat order and answered in place
//     via AskTape; with no host to ask, the first min items in zone order
//     are kept under a Note, the same deterministic stand-in the discard
//     window records. Players at or below the fewest keep everything.
//  4. Everything NOT chosen is discarded (Zone$ Hand, one events.Discard per
//     card) or sacrificed (battlefield, the SacrificeBlocked gate then one
//     events.Sacrifice per permanent) inside the one BatchDepartures window
//     a mass sacrifice shares (the effSacrificeAll bracket), so replacements
//     and look-backs read the pre-balance board.
//
// The Valid$ read, the Zone$ read and the keep ask are the whole parameter
// surface; Cost$, SubAbility$, SpellDescription$ and AILogic$ are the
// generic machinery's (the activation cost, the SubAbility$ chain that runs
// the Magus's next zone, the printed text).
func effBalance(h Host, c *Ctx, sa *cards.SA) {
	g := h.Game()
	zone := strings.TrimSpace(sa.ParamStr(cards.PKZone))
	zoneKind, zoneOK := ParseZoneWord(zone)
	hand := zoneOK && zoneKind == state.ZHand
	z := state.ZBattlefield
	if hand {
		z = state.ZHand
	}
	spec := strings.TrimSpace(sa.ParamStr(cards.PKValid))
	if spec == "" {
		if hand {
			spec = "Card"
		} else {
			spec = "Permanent"
		}
	}
	sc := c.SpecContext(c.Controller)
	bindSpecLayerTables(h, &sc.Layers, spec, false)
	// The up-front counts: players is AliveFrom's APNAP seat order, items is
	// indexed by seat so the walk below is seat-keyed, never map-ordered.
	players := g.AliveFrom(c.Controller)
	items := make([][]state.ObjID, len(g.Players))
	fewest := -1
	for _, p := range players {
		zoneIDs := append([]state.ObjID(nil), g.Zone(z, p)...)
		for _, id := range zoneIDs {
			if MatchesSpecCtx(g, spec, id, sc) {
				items[p] = append(items[p], id)
			}
		}
		if fewest < 0 || len(items[p]) < fewest {
			fewest = len(items[p])
		}
	}
	// The moving window opens before the first player's chosen cards leave
	// their zone, so every member's replacements read the pre-balance board.
	h.BatchDepartures(nil)
	defer h.EndBatchDepartures()
	for _, p := range players {
		if len(items[p]) <= fewest {
			continue
		}
		var keep []state.ObjID
		d := &decision.Decision{Player: p, Kind: decision.KChoose, Min: fewest, Max: fewest,
			Source: c.Source, ResumeKind: "balance_keep", ResumeSA: sa, ResumeTarget: int(p),
			Prompt: "Choose which to keep (everyone keeps as many as the fewest)"}
		for _, id := range items[p] {
			d.Options = append(d.Options, decision.Option{Index: len(d.Options),
				Kind: "balance_keep", Label: "Keep " + objectLabel(g, id), Obj: id, Player: p})
		}
		if ans, ok := AskTape(h, d); ok {
			for _, o := range ans {
				keep = append(keep, o.Obj)
			}
		} else {
			h.Emit(events.Event{Kind: events.Note, Obj: c.Source, Player: p,
				Text: "keeps its first " + strconv.Itoa(fewest) + " (no engine host to ask)"})
			keep = append(keep, items[p][:fewest]...)
		}
		for _, id := range items[p] {
			if slices.Contains(keep, id) {
				continue
			}
			if hand {
				h.Emit(events.Discard(id, p))
				continue
			}
			if h.SacrificeBlocked(id, false) {
				continue
			}
			h.Emit(events.Sacrifice(id))
		}
	}
}

// objectLabel is an ask option's label for one object: the face's name, or
// "a card" when the object carries none (the discardAsk convention).
func objectLabel(g *state.Game, id state.ObjID) string {
	if o := g.Obj(id); o != nil && o.Face() != nil {
		return o.Face().Name
	}
	return "a card"
}
