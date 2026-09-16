package effects

import (
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
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
	text := "extra turn"
	if strings.EqualFold(sa.Params["SkipUntap"], "True") {
		// Text is an encoded Event field. The canonical marker is folded by
		// events.Apply into the individual queued grant, so replay retains
		// this turn-specific rider without extending Event's fixed schema.
		text = events.ExtraTurnSkipUntapText
	}
	h.Emit(events.Event{Kind: events.ExtraTurn, Player: player, Amount: n,
		Obj: c.Source, Counter: sa.Params["ExtraTurnDelayedTriggerExecute"], Text: text})
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

// effRollDice implements DB$ RollDice (131 corpus files, measured with
// /usr/bin/grep -rlE over .cards/cardsfolder): roll a die with
// Sides$ sides (default 6, the corpus's unmarked "roll a die"), through the
// engine's seeded generator -- the one randomness channel a replay can
// reproduce. The result is recorded as a Note (the transcript's die roll) and
// published for the chained SubAbility$ under the names the parameters
// designate. Every publication lands in Ctx.RollPubs (and the primary one
// mirrors into LastRoll/LastRollName), read through rollPublished by Num's
// bare-name fallback, evalCountExpr's SVar$ head and a bare-name body, and
// Ctx.SpecContext's numeric-RHS resolver:
//
//   - ResultSVar$: the "SVar$<name>" indirection. ONE die publishes that
//     die's own result (Velukan Dragon's "SVar:X:SVar$Result/Minus.1" between
//     rolls and any ConditionCheckSVar$); several dice publish the TOTAL of
//     their results, which is exactly how the corpus's multi-roll readers
//     read it -- Neverwinter Hydra "a number of +1/+1 counters on it equal to
//     the total of those results", Pair o' Dice Lost "total mana value X or
//     less ... where X is the total of those results" -- and Spark Fiend's
//     StoreSVar Expression$ Result reads the same total through the
//     bare-name body path. The r2 review's truncation is gone: multi-roll
//     resolutions publish.
//   - UseDifferenceBetweenRolls$ True (5 corpus lines, all two dice -- the
//     Ungencoded CrankContraption family, Boomflinger et al.): publishes
//     |die1 - die2| instead of the total, so "damage ... equal to the
//     difference between those results" (NumDmg$ Result) and
//     TokenAmount$/Amount$ Result read the difference.
//   - ChosenSVar$ / OtherSVar$ (5 corpus lines, the Endeavor cycle, all two
//     dice): the controller CHOOSES one of the rolled results -- a real
//     KChoose (Min == Max == 1, one "roll" option per die in roll order),
//     answered through ResumeKind "roll" with the per-die results carried on
//     the decision (decision.Decision.Rolls) and the rules resume point, so
//     the re-entry publishes ChosenSVar$ = the picked die's result and
//     OtherSVar$ = the unpicked one's (sums, so the shape generalises past
//     two dice) without re-rolling. A host that cannot ask (the fuzz
//     stand-in, R-9) and botpolicy's clamp fallback both keep the FIRST die
//     -- deterministic and non-wedging. A single-die ChosenSVar$ needs no
//     ask: that die is the chosen result.
//   - MaxRollsResults$ True / EvenOddResults$ True (Luck Bobblehead):
//     publish "MaxRolls" (the count of results equal to the die's maximum,
//     sides+modifier -- "If you rolled 6 exactly seven times") and
//     "EvenResults"/"OddResults" (the counts of even/odd results -- the
//     "create a Treasure token for each even result" sub reads
//     SVar$EvenResults through the ordinary SVar$ indirection).
//
// Amount$ (24 lines: "roll X dice" shapes) rolls that many dice and runs the
// range table once PER DIE (each die's own result matched independently --
// the reading "create a Treasure for each even result" needs), with a Note per
// roll. Modifier$ is added to every roll through the same Num/SVar evaluator
// that reads Sides$ and Amount$ (Wyll's Reversal and Danse Macabre's Y, Song
// of Inspiration's X); the modified result is what ranges match and what
// every publication totals. The unmodified die remains the only random draw.
func effRollDice(h Host, c *Ctx, sa *cards.SA) {
	// fx42 scoping: capture and clear the answered choose-one-result BEFORE
	// anything else, so a nested RollDice below this walk poses its own ask
	// instead of inheriting the outer answer.
	rolls := c.RollResults
	pick := c.RollPick
	done := c.RollDone
	c.RollResults, c.RollPick, c.RollDone = nil, nil, false

	sides := Num(h, c, sa, "Sides", 6)
	if sides <= 0 {
		sides = 6
	}
	amount := Num(h, c, sa, "Amount", 1)
	if amount < 1 {
		amount = 1
	}
	modifier := Num(h, c, sa, "Modifier", 0)
	chosenName := strings.TrimSpace(sa.Params["ChosenSVar"])
	otherName := strings.TrimSpace(sa.Params["OtherSVar"])

	publish := func(name string, v int32) {
		name = strings.TrimSpace(name)
		if name == "" {
			return
		}
		c.RollPubs = append(c.RollPubs, RollPub{Name: name, Value: v})
		if name == "X" {
			c.X = v
		}
	}

	if done {
		// Re-entry: the choose-one-result answer arrived. The chosen options'
		// Index values name the dice (into `rolls`, the per-die results the
		// asking first pass carried on the decision) the player picked; the
		// chosen value is the sum of the picked dice's results, the other
		// value the sum of the rest -- for the corpus's two-die Endeavor
		// cycle that is exactly "one result" and "the other result".
		chosenSum, otherSum, picked := int32(0), int32(0), 0
		isPicked := func(i int) bool {
			for _, idx := range pick {
				if idx == i {
					return true
				}
			}
			return false
		}
		for i, r := range rolls {
			if isPicked(i) {
				chosenSum += r
				picked++
			} else {
				otherSum += r
			}
		}
		if picked == 0 && len(rolls) > 0 {
			// A malformed or empty answer picks nothing: the first die stands
			// in (deterministically, the same die the R-9 and clamp fallbacks
			// keep) rather than publishing a chosen value no real choice
			// could produce.
			chosenSum, otherSum = rolls[0], 0
			for _, r := range rolls[1:] {
				otherSum += r
			}
		}
		publish(chosenName, chosenSum)
		publish(otherName, otherSum)
		if chosenName != "" {
			c.LastRoll, c.LastRollName = chosenSum, chosenName
		}
		return
	}

	// First pass: roll the dice.
	dice := make([]int32, 0, amount)
	ranges := parseDieRanges(sa.Params["ResultSubAbilities"])
	for i := int32(0); i < amount; i++ {
		die := int32(h.Rand(int(sides))) + 1
		result := die + modifier
		dice = append(dice, result)
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

	// Primary result publication (ResultSVar$): one die's own result, the
	// total of several, or the two-die difference.
	total := int32(0)
	for _, r := range dice {
		total += r
	}
	pub := total
	if strings.EqualFold(sa.Params["UseDifferenceBetweenRolls"], "True") && len(dice) == 2 {
		pub = dice[0] - dice[1]
		if pub < 0 {
			pub = -pub
		}
	}
	resultName := strings.TrimSpace(sa.Params["ResultSVar"])
	if resultName != "" {
		c.LastRoll, c.LastRollName = pub, resultName
		publish(resultName, pub)
	}
	// MaxRollsResults$ / EvenOddResults$ (Luck Bobblehead): the counts a
	// chained sub reads back through the published names.
	if sa.Params["MaxRollsResults"] == "True" {
		maxResult := sides + modifier
		n := int32(0)
		for _, r := range dice {
			if r == maxResult {
				n++
			}
		}
		publish("MaxRolls", n)
	}
	if sa.Params["EvenOddResults"] == "True" {
		even, odd := int32(0), int32(0)
		for _, r := range dice {
			if r%2 == 0 {
				even++
			} else {
				odd++
			}
		}
		publish("EvenResults", even)
		publish("OddResults", odd)
	}

	// The choose-one-result ask (ChosenSVar$/OtherSVar$, the Endeavor
	// cycle): one die of the rolled set becomes the chosen result, the rest
	// the other. Exactly one die is chosen (Min == Max == 1), one "roll"
	// option per die in roll order, answered through ResumeKind "roll" with
	// the per-die results carried on the decision for the resume.
	if chosenName != "" && len(dice) > 1 {
		d := &decision.Decision{Player: c.Controller, Kind: decision.KChoose,
			Min:        1,
			Max:        1,
			Source:     c.Source,
			ResumeKind: "roll",
			ResumeSA:   sa,
			Rolls:      append([]int32(nil), dice...),
			Prompt:     "Choose one rolled result"}
		for i, r := range dice {
			d.Options = append(d.Options, decision.Option{Index: i,
				Kind: "roll", Label: "die " + strconv.Itoa(i+1) + ": result " + strconv.FormatInt(int64(r), 10),
				Player: c.Controller})
		}
		if h.Ask(d) {
			return // resolution suspended; the answer re-enters with Ctx.RollResults/RollPick set.
		}
		// Fuzz/no-engine host (R-9): the deterministic stand-in keeps the
		// first die -- the exact option botpolicy's clamp fallback takes.
		h.Emit(events.Event{Kind: events.Note, Obj: c.Source,
			Text: "keeps the first rolled result (no engine host to ask)"})
		other := int32(0)
		for _, r := range dice[1:] {
			other += r
		}
		publish(chosenName, dice[0])
		publish(otherName, other)
		c.LastRoll, c.LastRollName = dice[0], chosenName
		return
	}
	if chosenName != "" && len(dice) == 1 {
		// A single-die choose is vacuous: that die is the chosen result; the
		// other value has no die to name and publishes nothing.
		publish(chosenName, dice[0])
		c.LastRoll, c.LastRollName = dice[0], chosenName
	}
}

// rollPublished reports the value a DB$ RollDice of this resolution
// published under name: the primary ResultSVar$ slot (LastRollName/LastRoll)
// or one of the Ctx.RollPubs entries (ChosenSVar$/OtherSVar$ and the
// MaxRollsResults$/EvenOddResults$ counts). ok=false on any resolution that
// rolled nothing or published no such name -- the conservative same-as-before
// degrade every unmodelled head applies.
func rollPublished(c *Ctx, name string) (int32, bool) {
	if c == nil {
		return 0, false
	}
	if c.LastRollName != "" && c.LastRollName == name {
		return c.LastRoll, true
	}
	for _, p := range c.RollPubs {
		if p.Name == name {
			return p.Value, true
		}
	}
	return 0, false
}
