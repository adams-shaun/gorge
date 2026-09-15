package effects

import (
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/events"
)

func init() {
	Register("AddTurn", effAddTurn)
	Register("LosesGame", effLosesGame)
	Register("RollDice", effRollDice)
}

// effAddTurn implements the extra-turn primitive (CR 500.7; 43 corpus files).
// The grant is one events.ExtraTurn per call: Player is the granted seat,
// Amount the NumTurns$ value (default 1, through the ordinary Num SVar
// indirection). Defined$ names the granted player when present (Rise of the
// Eldrazi's "Defined$ You", Time Walk's targeted form through its
// ValidTgts$ targets); the default is the resolving ability's controller.
//
// The turn structure (rules/turn.go's advanceStep) is what consumes the
// count: at the cleanup step's end, a seat whose ExtraTurns count is positive
// repeats as the active player and emits the -1 consumption, instead of the
// ordinary next-alive-seat rotation. Final Fortune's "At the beginning of
// that turn's end step, you lose the game" rides the same event: the granting
// ability's ExtraTurnDelayedTrigger$/ExtraTurnDelayedTriggerExecute$ pair is
// forwarded on the ExtraTurn event (Counter/Obj), and events.Apply registers
// the delayed Mode$ Phase trigger with the extra turn's number as MinTurn, so
// it skips the granting turn's own end step and fires exactly once, in the
// granted turn. Alchemist's Gambit's "during that turn, damage can't be
// prevented" static is NOT modelled (the CantPreventDamage static mode is an
// unimplemented Effect static -- the Note its Effect body emits says so).
func effAddTurn(h Host, c *Ctx, sa *cards.SA) {
	n := Num(h, c, sa, "NumTurns", 1)
	if n <= 0 {
		return
	}
	player := c.Controller
	if sa.Params["Defined"] != "" || sa.Params["ValidTgts"] != "" {
		for _, t := range Defined(h, c, sa) {
			if t.IsPlayer {
				player = t.Player
				break
			}
		}
	}
	if int(player) < 0 || int(player) >= len(h.Game().Players) {
		return
	}
	h.Emit(events.Event{Kind: events.ExtraTurn, Player: player, Amount: n,
		Obj: c.Source, Counter: sa.Params["ExtraTurnDelayedTriggerExecute"],
		Text: "extra turn"})
}

// effLosesGame implements DB$ LosesGame (53 corpus files): the Defined$
// player (the resolving ability's controller by default) loses the game right
// now, CR 104.2a -- the same PlayerLost event a 0-life elimination emits, so
// state-based actions sweep their permanents and the game-over check runs as
// for any other loss. Final Fortune and Last Chance reach this through the
// extra turn's delayed trigger; a seat that has already lost is skipped
// (losing twice is not a second game event).
func effLosesGame(h Host, c *Ctx, sa *cards.SA) {
	player := c.Controller
	if sa.Params["Defined"] != "" {
		for _, t := range Defined(h, c, sa) {
			if t.IsPlayer {
				player = t.Player
				break
			}
		}
	}
	g := h.Game()
	if int(player) < 0 || int(player) >= len(g.Players) || g.Players[player].Lost {
		return
	}
	h.Emit(events.Event{Kind: events.PlayerLost, Player: player, Text: "lost the game"})
}

// parseDieRanges splits Forge's ResultSubAbilities$ value into its range
// entries: "1-6:DBMana1,7-14:DBMana2,15-20:DBMana3" or the single-value
// "1:M2,2:Fog,..." or an "Else:DBDiscard" catch-all. The order is the string's
// own order (never a map), so the matching below is deterministic.
type dieRange struct {
	lo, hi int32
	name   string
	isElse bool
}

func parseDieRanges(v string) []dieRange {
	var out []dieRange
	for _, part := range strings.Split(v, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		nameIdx := strings.IndexByte(part, ':')
		if nameIdx < 0 {
			continue
		}
		spec, name := strings.TrimSpace(part[:nameIdx]), strings.TrimSpace(part[nameIdx+1:])
		if strings.EqualFold(spec, "Else") {
			out = append(out, dieRange{name: name, isElse: true})
			continue
		}
		loS, hiS := spec, spec
		if dash := strings.IndexByte(spec, '-'); dash >= 0 {
			loS, hiS = spec[:dash], spec[dash+1:]
		}
		lo, err1 := strconv.ParseInt(strings.TrimSpace(loS), 10, 64)
		hi, err2 := strconv.ParseInt(strings.TrimSpace(hiS), 10, 64)
		if err1 != nil || err2 != nil {
			continue
		}
		out = append(out, dieRange{lo: int32(lo), hi: int32(hi), name: name})
	}
	return out
}

// effRollDice implements DB$ RollDice (110 corpus files): roll a die with
// Sides$ sides (default 6, the corpus's unmarked "roll a die"), through the
// engine's seeded generator -- the one randomness channel a replay can
// reproduce. The result is recorded as a Note (the transcript's die roll) and
// published two ways for the chained SubAbility$:
//
//   - ResultSubAbilities$: the "lo-hi:name,n:name,Else:name" range table; the
//     FIRST entry whose range contains the result has its SVar resolved and
//     run. Name Sticker Goblin's "1-6:...,7-14:...,15-20:..." is exactly this
//     shape.
//   - ResultSVar$: the result is stored on the resolution under the SVar name
//     the parameter names, so a chained SVar body can read it
//     ("SVar:X:SVar$Result/Minus.1", Velukan Dragon) and a ConditionCheckSVar$
//     can compare it (Kharis & The Beholder). The name "X" additionally
//     becomes the resolution's own {X} value, which is how a sub's Num$
//     X/NumCards$ X parameter reads the roll. Any other name is stored for the
//     SVar$ head alone.
//
// Amount$ (24 lines: "roll X dice" shapes) rolls that many dice and runs the
// range table once PER DIE (each die's own result matched independently --
// the reading "create a Treasure for each even result" needs), with a Note per
// roll; ResultSVar$ with multiple dice stores only the LAST die's result
// (every corpus multi-die line uses range tables, never ResultSVar$, so
// nothing real is lost). Modifier$ is added to every roll through the same
// Num/SVar evaluator that reads Sides$ and Amount$ (Wyll's Reversal and Danse
// Macabre's Y, Song of Inspiration's X). The unmodified die remains the only
// random draw; the modified result selects ResultSubAbilities$ ranges.
func effRollDice(h Host, c *Ctx, sa *cards.SA) {
	sides := Num(h, c, sa, "Sides", 6)
	if sides <= 0 {
		sides = 6
	}
	amount := Num(h, c, sa, "Amount", 1)
	if amount < 1 {
		amount = 1
	}
	if amount > 20 {
		// A computed absurdity (Amount$ X with a huge paid X) must not spin
		// the resolution; 20 is far past every corpus shape (max literal 5).
		amount = 20
	}
	modifier := Num(h, c, sa, "Modifier", 0)
	ranges := parseDieRanges(sa.Params["ResultSubAbilities"])
	name := sa.Params["ResultSVar"]
	var last int32
	for i := int32(0); i < amount; i++ {
		die := int32(h.Rand(int(sides))) + 1
		result := die + modifier
		last = result
		text := "rolls a d" + strconv.FormatInt(int64(sides), 10) + ": " + strconv.FormatInt(int64(die), 10)
		if modifier != 0 {
			text += " + " + strconv.FormatInt(int64(modifier), 10) + " = " + strconv.FormatInt(int64(result), 10)
		}
		h.Emit(events.Event{Kind: events.Note, Obj: c.Source, Text: text})
		if len(ranges) > 0 {
			sub := ""
			for _, r := range ranges {
				if r.isElse {
					continue
				}
				if result >= r.lo && result <= r.hi {
					sub = r.name
					break
				}
			}
			if sub == "" {
				for _, r := range ranges {
					if r.isElse {
						sub = r.name
						break
					}
				}
			}
			if sub != "" && c.SVars != nil {
				Resolve(h, c, cards.ResolveSVar(c.SVars, sub))
				if h.Suspended() {
					return
				}
			}
		}
	}
	if name != "" && amount == 1 {
		c.LastRoll = last
		c.LastRollName = name
		if name == "X" {
			c.X = last
		}
	}
}
