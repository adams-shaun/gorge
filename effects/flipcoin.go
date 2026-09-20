package effects

import (
	"strings"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

func init() {
	Register("FlipCoin", effFlipCoin)
}

// FlipNotePrefix is the canonical coin-flip result Note's text prefix. ONE
// shared encoding serves every flip: api:FlipCoin (effFlipCoin below) and the
// cumulative-upkeep FlipCoin cost action (rules/cumulative.go, which calls
// FlipCoinNote) both emit it, and the trig:FlippedCoin matcher
// (rules/trigger_match.go's flippedCoinMatches) reads it — so a cost-side flip
// fires "whenever you win/lose a coin flip" exactly like an effect-side one
// (Karplusan Minotaur is the corpus card whose only missing primitive was the
// trigger). The Note is a replayable event: the random draw is the engine's
// seeded Rand, and a replay folds the Note without re-flipping, the same
// shape RollDice's per-roll Notes use.
const FlipNotePrefix = "flips a coin: "

// FlipCoinNote builds the canonical coin-flip result event: Player is the
// flipper, Obj the flipping source, Amount 1 = win/heads and 0 = lose/tails
// (Forge treats heads as win and tails as lose, including every NoCall$ line),
// and Text names the outcome for the transcript and the view.
func FlipCoinNote(source state.ObjID, flipper state.PlayerID, win bool) events.Event {
	if win {
		return events.Event{Kind: events.Note, Player: flipper, Obj: source,
			Amount: 1, Text: FlipNotePrefix + "heads"}
	}
	return events.Event{Kind: events.Note, Player: flipper, Obj: source,
		Amount: 0, Text: FlipNotePrefix + "tails"}
}

// FlipNoteResult reports whether ev is a canonical coin-flip result Note and
// which side it landed: the flipper (ev.Player), the win flag (ev.Amount == 1).
// Shared by the FlippedCoin trigger matcher; a non-flip Note (any other text)
// is not one.
func FlipNoteResult(ev events.Event) (flipper state.PlayerID, win bool, ok bool) {
	if ev.Kind != events.Note || !strings.HasPrefix(ev.Text, FlipNotePrefix) {
		return 0, false, false
	}
	return ev.Player, ev.Amount == 1, true
}

// flipperPlayers resolves the players who flip, in one shared resolver for
// both spellings the corpus uses: Defined$ (Mana Crypt's "Defined$ You") and
// Flipper$ ("that player flips" selectors — Remembered for the RepeatEach
// iteration subject, TriggeredActivator for a cast/tap trigger's actor,
// Player for "each player"). Everything resolves through
// knownDefinedTargets — the ONE defined-target resolver — so a Flipper$
// spelling can never disagree with the same spelling under Defined$. A
// present-but-unresolvable spec names nobody: ok=false, fail closed.
func flipperPlayers(h Host, c *Ctx, sa *cards.SA) (players []state.Target, named bool) {
	spec := strings.TrimSpace(sa.Params["Flipper"])
	if spec == "" {
		spec = strings.TrimSpace(sa.Params["Defined"])
	}
	if spec == "" {
		return nil, false
	}
	ts, ok := knownDefinedTargets(h, c, spec)
	if !ok {
		return nil, true
	}
	return playersOf(ts), true
}

// effFlipCoin implements DB$/SP$/AB$ FlipCoin (82 corpus files): flip a coin
// through the engine's seeded generator (heads = win, tails = lose), record
// each result on the canonical Note (FlipCoinNote — the encoding the
// FlippedCoin trigger matcher reads), and resolve the outcome branch per
// flip. The branch names are resolved through ResolveSVar exactly as
// RollDice's ResultSubAbilities$ are: the base SA carries no auto-link to
// WinSubAbility$/LoseSubAbility$ (cards/link.go links only SubAbility$), so
// this primitive resolves the name itself; the base's own SubAbility$ chain
// (after the branches) is the enclosing Resolve loop's ordinary walk.
//
// Parameters read:
//   - WinSubAbility$ (heads) / LoseSubAbility$ (tails), and the NoCall$
//     lines' spellings HeadsSubAbility$ / TailsSubAbility$ — same heads/tails
//     mapping, NoCall$ itself is cosmetic (Forge uses it to suppress the
//     "calls heads" announcement this build never makes).
//   - Amount$ (default 1): that many independent flips, each running its own
//     branch (TrigFlipCoins' "flip three coins" shape). An unresolvable value
//     degrades to one flip, the RollDice convention — a flip that happened
//     once is closer to "flip X coins" than a silent no-op.
//   - Defined$ / Flipper$: who flips, through one knownDefinedTargets
//     resolver (flipperPlayers). Absent, the resolving controller flips; a
//     present spec that resolves to no player (or an unmodelled selector)
//     flips nothing — the effLosesGame fail-closed convention.
//   - FlipUntilYouLose$ True (6 corpus lines: Okaun, Zndrsplt, Toothy and
//     Zndrsplt, Crazed Firecat, Mirror March): flip until the first tails,
//     running the win branch per winning flip and the lose branch once on
//     the losing flip. Cheap as a loop, so it is implemented, not noted.
//
// Unread, each loud when present (never silent): ForEachPlayer$ (one corpus
// line — one flip instead of one per opponent), RememberResult$ /
// RememberNumber$ (the per-flip result/wins memory a later sub reads back —
// the flips run, the memory does not persist; the chained reader degrades
// through definedSpec's FlippedHeads/FlippedTails fail-closed cases).
func effFlipCoin(h Host, c *Ctx, sa *cards.SA) {
	flippers, named := flipperPlayers(h, c, sa)
	if len(flippers) == 0 {
		if named {
			return
		}
		flippers = []state.Target{{Player: c.Controller, IsPlayer: true}}
	}
	if sa.Params["ForEachPlayer"] != "" {
		// One corpus line (an opponent-shaped "each opponent flips"); until a
		// per-player flip ask exists this resolves one flip, loudly.
		h.Emit(events.Event{Kind: events.Note, Obj: c.Source,
			Text: "FlipCoin ForEachPlayer$ unread: one flip instead of one per player"})
	}
	if sa.Params["RememberResult"] != "" || sa.Params["RememberNumber"] != "" {
		h.Emit(events.Event{Kind: events.Note, Obj: c.Source,
			Text: "FlipCoin RememberResult$/RememberNumber$ unread: results not remembered"})
	}
	untilLose := strings.EqualFold(sa.Params["FlipUntilYouLose"], "True")
	amount := Num(h, c, sa, "Amount", 1)
	if amount < 1 {
		amount = 1
	}
	winName := strings.TrimSpace(sa.Params["WinSubAbility"])
	if winName == "" {
		winName = strings.TrimSpace(sa.Params["HeadsSubAbility"])
	}
	loseName := strings.TrimSpace(sa.Params["LoseSubAbility"])
	if loseName == "" {
		loseName = strings.TrimSpace(sa.Params["TailsSubAbility"])
	}
	g := h.Game()
	for _, t := range flippers {
		if int(t.Player) < 0 || int(t.Player) >= len(g.Players) || g.Players[t.Player].Lost {
			continue
		}
		for i := int32(0); untilLose || i < amount; i++ {
			win := h.Rand(2) == 0
			h.Emit(FlipCoinNote(c.Source, t.Player, win))
			name := loseName
			if win {
				name = winName
			}
			if name != "" && c.SVars != nil {
				Resolve(h, c, cards.ResolveSVar(c.SVars, name))
				if h.Suspended() {
					return
				}
			}
			if untilLose && !win {
				break
			}
		}
	}
}
