package effects

import (
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

func init() {
	Register("PutCounter", effPutCounter)
	Register("PutCounterAll", effPutCounterAll)
	Register("RemoveCounterAll", effRemoveCounterAll)
	Register("Regenerate", effRegenerate)
}

func effPutCounter(h Host, c *Ctx, sa *cards.SA) {
	// fx42 scoping: consume and clear the answered Optional$ election at the
	// top, so a nested PutCounter in the same chain poses its own ask.
	optAns := c.PutOpt
	c.PutOpt = ""
	n := Num(h, c, sa, "CounterNum", 1)
	if n < 0 {
		n = 0
	}
	kind := sa.Params["CounterType"]
	if kind == "" {
		kind = "P1P1"
	}
	if strings.EqualFold(strings.TrimSpace(sa.Params["Optional"]), "True") {
		switch {
		case optAns != "" && optAns != "yes":
			// Answered "no" (or any non-affirmative marker): the decline. No
			// counter is placed and no Note is emitted; the chained
			// SubAbility$ STILL RUNS -- the chain is owned by Resolve, not by
			// this body (the Attach.Optional precedent,
			// effects/attach.go:96-131; the chain-skip mechanism is the
			// DIFFERENT UnlessCost$ + UnlessResolveSubs$ pair, which none of
			// the corpus's Optional$ PutCounter lines carry). Black Widow's
			// "If you don't, ..." sub gates itself on its own Condition$ read
			// of the (empty) Remembered set, exactly as the oracle says.
			return
		case optAns == "":
			// Unanswered: pose the yes/no election -- but only when the put
			// would actually place something (at least one live recipient and
			// n > 0); with nothing legal to put on, decline and accept are the
			// same, so no ask (the Attach precedent's len(legal) == 0 gate).
			// The pickAnswered guard is the two-ask shape's own discipline:
			// an Optional$ bare-Choices$/DividedAsYouChoose$ SA asks TWICE
			// (election, then the recipient pick), and each resume builds a
			// FRESH Ctx -- the pick re-entry arrives with PutOpt already
			// consumed, so without this guard the election would re-pose over
			// the answered pick. The Done flag names the pick, never the
			// election: a pickDone pass is past the election by construction.
			pickAnswered := c.CounterPickDone || c.CounterDistDone
			if n > 0 && !pickAnswered && putCounterWouldPlace(h, c, sa) {
				d := &decision.Decision{Player: c.Controller, Kind: decision.KChoose, Min: 1, Max: 1,
					Source: c.Source, ResumeKind: "put_optional", ResumeSA: sa,
					ResumeRemembered: copyTargets(c.Remembered),
					Prompt:           "Put a counter on it?",
					Options: []decision.Option{
						{Index: 0, Kind: "yes", Label: "Yes — put the counter", Player: c.Controller},
						{Index: 1, Kind: "no", Label: "No", Player: c.Controller},
					}}
				// AskAsked suspends; the answer re-enters with Ctx.PutOpt set.
				// AskNoHost is the deterministic decline stand-in (R-9) — the
				// same class the Attach election falls back to (the clamp-
				// answered bot path answers option 0 = "yes", so a bot game
				// stays byte-identical to the pre-ask silent always-put).
				_ = Ask(h, d)
				return
			}
			// optAns == "yes" (or nothing to place): fall through to the
			// ordinary placement paths.
		}
	}
	// DividedAsYouChoose$ (Vastwood Hydra's "you may distribute a number of
	// +1/+1 counters equal to the number of +1/+1 counters on CARDNAME among
	// any number of creatures you control", 54 raw corpus PutCounter lines):
	// the CounterNum$ TOTAL is divided among the recipients, not placed on
	// each. Two carrier shapes, split on where the recipients come from:
	// Choices$ names a mid-resolution battlefield pick bounded by
	// MinChoiceAmount$/ChoiceAmount$; without Choices$ the recipients are the
	// ordinary chosen targets (ValidTgts$, already asked by the targeting
	// machinery).
	divided := strings.TrimSpace(sa.Params["DividedAsYouChoose"]) != ""
	// fx45 scoping: capture and clear the answered Choices$ pick BEFORE the
	// branch, so a nested PutCounter below cannot inherit the outer answer
	// (the fx42 discipline every answered field follows).
	distAns := c.CounterDist
	distDone := c.CounterDistDone
	c.CounterDist, c.CounterDistDone = nil, false
	// The bare-Choices$ pick's answer rides its own pair of fields (the
	// divided family and the bare pick can never both ask for one SA, but
	// each consumes and clears only its own).
	pickAns := c.CounterPick
	pickDone := c.CounterPickDone
	c.CounterPick, c.CounterPickDone = nil, false
	if divided {
		if strings.TrimSpace(sa.Params["Choices"]) != "" {
			putCounterPickDistribute(h, c, sa, n, kind, distAns, distDone)
			return
		}
		placed := putCounterSplit(h, n, kind, Defined(h, c, sa))
		rememberPlaced(c, sa, placed)
		return
	}
	if strings.TrimSpace(sa.Params["Choices"]) != "" {
		// The bare-Choices$ pick shape (task vow1; Promise of Loyalty): the
		// chooser picks which creatures take the counters, each chosen one
		// taking the full CounterNum$ (no division). The choice re-enters
		// through ResumeKind "counter_pick".
		putCounterChoose(h, c, sa, n, kind, pickAns, pickDone)
		return
	}
	// ETB$ True (the K:etbCounter expansion's body, Wishclaw Talisman and
	// every "enters with N counters" card): the counters are placed on the
	// ENTERING object as it enters, so the target does not have to be a
	// settled battlefield permanent yet. The replacement machinery runs the
	// body after the entry move in the ordinary flow, but a body reached
	// while the object is still mid-entry must place the counters anyway,
	// not skip on the battlefield precondition.
	etb := strings.EqualFold(strings.TrimSpace(sa.Params["ETB"]), "True")
	var placed []state.Target
	for _, t := range Defined(h, c, sa) {
		if t.IsPlayer {
			// A player target takes a PLAYER counter (energy's "you get {E}{E}{E}",
			// poison's "gets a poison counter"): the same instruction an object
			// target takes, but on the PlayerCounterChange event the engine's
			// player-counter state folds through. Skipping these (the pre-fix
			// behaviour) silently dropped the whole instruction -- the corpus
			// carries 156 player-targeted PutCounter lines.
			if p := PlayerOf(h, c, t); int(p) >= 0 && int(p) < len(h.Game().Players) {
				h.Emit(events.Event{Kind: events.PlayerCounterChange, Player: p,
					Counter: kind, Amount: n})
			}
			continue
		}
		o := h.Game().Obj(t.Obj)
		if o == nil || (o.Zone != state.ZBattlefield && !etb) {
			continue
		}
		h.Emit(events.Event{Kind: events.CounterChange, Obj: o.ID, Counter: kind, Amount: n})
		if !t.IsPlayer && t.Obj != 0 {
			placed = append(placed, t)
		}
	}
	rememberPlaced(c, sa, placed)
}

// putCounterWouldPlace reports whether the put this SA describes would
// place at least one counter on a live recipient, mirroring the live-
// recipient conditions each placement path applies:
//   - the two Choices$ shapes (bare pick and DividedAsYouChoose$
//     distribute): the Choices$ pool must hold an eligible battlefield
//     object AND the ask's Max (ChoiceAmount$, default 1 for the bare pick,
//     the CounterNum$ total for the divided distribute) must be at least
//     one -- the same bounds putCounterChoose/putCounterPickDistribute
//     read, so the election is never posed over a pool the placement
//     would refuse;
//   - the plain target loop: a player target whose PlayerOf resolves, or
//     an object target on the battlefield (or mid-entry when ETB$ True).
//
// It is a pure read: no event, no state change, replay-safe.
func putCounterWouldPlace(h Host, c *Ctx, sa *cards.SA) bool {
	g := h.Game()
	spec := strings.TrimSpace(sa.Params["Choices"])
	if spec != "" {
		found := false
		for _, p := range g.AliveFrom(0) {
			for _, id := range g.Zone(state.ZBattlefield, p) {
				if MatchesSpecCtx(g, spec, id, c.SpecContext(c.Controller)) {
					found = true
					break
				}
			}
			if found {
				break
			}
		}
		if !found {
			return false
		}
		defMax := int32(1)
		if strings.TrimSpace(sa.Params["DividedAsYouChoose"]) != "" {
			defMax = Num(h, c, sa, "CounterNum", 1)
		}
		return Num(h, c, sa, "ChoiceAmount", defMax) >= 1
	}
	etb := strings.EqualFold(strings.TrimSpace(sa.Params["ETB"]), "True")
	for _, t := range Defined(h, c, sa) {
		if t.IsPlayer {
			if p := PlayerOf(h, c, t); int(p) >= 0 && int(p) < len(h.Game().Players) {
				return true
			}
			continue
		}
		o := g.Obj(t.Obj)
		if o == nil {
			continue
		}
		if o.Zone == state.ZBattlefield || etb {
			return true
		}
	}
	return false
}

// rememberPlaced folds the objects a PutCounter pass just countered into the
// resolution's Remembered set, when the SA carries RememberCards$ True. The
// flag names the cards that WERE countered, never the attempt: a pass that
// placed no counter remembers nothing. A RepeatEach loop's rememberIteration
// propagates what the iteration remembered into the loop's own set, so
// Promise of Loyalty's chained SacAllOthers (the SAME iteration) and its
// loop-tail DBEffect (RememberObjects$ Remembered) both see the vowed
// creatures without any event-backed persistence.
func rememberPlaced(c *Ctx, sa *cards.SA, placed []state.Target) {
	if len(placed) == 0 || !strings.EqualFold(strings.TrimSpace(sa.Params["RememberCards"]), "True") {
		return
	}
	c.Remembered = append(c.Remembered, placed...)
}

// putCounterPickDistribute runs the Choices$ + DividedAsYouChoose$ shape: the
// recipients are a battlefield pick over the Choices$ filter, bounded by
// MinChoiceAmount$ (the ask's Min) and ChoiceAmount$ (the ask's Max, default
// the CounterNum$ total), and the CounterNum$ total is then divided among the
// chosen recipients.
//
// The recipient SET is a real KChoose ask whenever more than one eligible
// creature exists and a smaller-than-all set is legal (MinChoiceAmount$ below
// the eligible count): the strict-supersets gate every asking primitive here
// follows -- a set the rules force (Min == Max == the eligible count) or a
// single-eligible board leaves nothing to choose, so no decision is posed and
// the split runs over the only legal recipient list.
//
// The DIVISION among the chosen recipients is the deterministic stand-in the
// damage primitive's DividedAsYouChoose$ already ships (effects/damage.go):
// one counter at a time, round-robin in the player's answer order, so the
// earlier-chosen recipients take the extras. A per-counter division ask (the
// repeated one-pick ask Forge's UI models by clicking) is not posed.
func putCounterPickDistribute(h Host, c *Ctx, sa *cards.SA, total int32, kind string, ans []state.ObjID, ansDone bool) {
	if total <= 0 {
		return
	}
	g := h.Game()
	spec := strings.TrimSpace(sa.Params["Choices"])
	if ansDone {
		// Re-entry: the answered pick, in answer order. A recipient that left
		// the battlefield while the decision was outstanding takes nothing
		// (its share is lost, not redistributed -- the same totality stance
		// the target-based split takes).
		placed := putCounterSplit(h, total, kind, objTargets(ans))
		rememberPlaced(c, sa, placed)
		return
	}
	var eligible []state.ObjID
	for _, p := range g.AliveFrom(0) {
		for _, id := range g.Zone(state.ZBattlefield, p) {
			if MatchesSpecCtx(g, spec, id, c.SpecContext(c.Controller)) {
				eligible = append(eligible, id)
			}
		}
	}
	minCh := Num(h, c, sa, "MinChoiceAmount", 0)
	if minCh < 0 {
		minCh = 0
	}
	maxCh := Num(h, c, sa, "ChoiceAmount", total)
	if maxCh < 0 {
		maxCh = 0
	}
	if maxCh > int32(len(eligible)) {
		maxCh = int32(len(eligible))
	}
	if minCh > maxCh {
		minCh = maxCh
	}
	// The no-choice fallbacks share one deterministic recipient list: the
	// first maxCh eligible creatures in zone order -- for a forced set
	// (minCh >= the eligible count) that IS the only legal answer, and for
	// the fuzz/no-host run (R-9) it is the exact mirror of botpolicy's
	// "counter_dist" arm, so a bot-answered ask emits the same events the
	// silent build did.
	fallback := func() {
		picks := eligible
		if int32(len(picks)) > maxCh {
			picks = picks[:maxCh]
		}
		placed := putCounterSplit(h, total, kind, objTargets(picks))
		rememberPlaced(c, sa, placed)
	}
	// Ask gate: a real recipient choice needs two or more eligible creatures
	// room to differ (maxCh >= 1 leaves at least one recipient; minCh below
	// the eligible count leaves a smaller set legal).
	if len(eligible) < 2 || maxCh < 1 || minCh >= int32(len(eligible)) {
		fallback()
		return
	}
	d := &decision.Decision{Player: c.Controller, Kind: decision.KChoose,
		Min:        int(minCh),
		Max:        int(maxCh),
		Source:     c.Source,
		ResumeKind: "counter_dist",
		ResumeSA:   sa,
		Prompt:     "Distribute " + kind + " counter(s): choose where to place " + strconv.Itoa(int(total))}
	for _, id := range eligible {
		name := "a creature"
		if o := g.Obj(id); o != nil && o.Face() != nil {
			name = o.Face().Name
		}
		d.Options = append(d.Options, decision.Option{Index: len(d.Options),
			Kind: "counter_dist", Label: name, Obj: id, Player: c.Controller})
	}
	if Ask(h, d) == AskAsked {
		return // resolution suspended; the answer re-enters with Ctx.CounterDist set.
	}
	fallback()
}

// putCounterSplit divides the CounterNum$ total among the recipients
// round-robin in recipient order (the deterministic division stand-in) and
// emits one CounterChange per recipient with its share. Recipients that are
// no longer on the battlefield take nothing; the total is exact (every
// counter lands somewhere or is lost with a departed recipient, never
// invented).
func putCounterSplit(h Host, total int32, kind string, ts []state.Target) []state.Target {
	if total <= 0 {
		return nil
	}
	g := h.Game()
	var live []state.ObjID
	for _, t := range ts {
		if t.IsPlayer {
			continue
		}
		if o := g.Obj(t.Obj); o != nil && o.Zone == state.ZBattlefield {
			live = append(live, t.Obj)
		}
	}
	if len(live) == 0 {
		return nil
	}
	shares := make(map[state.ObjID]int32, len(live))
	for i := int32(0); i < total; i++ {
		shares[live[i%int32(len(live))]]++
	}
	var placed []state.Target
	for _, id := range live {
		if amt := shares[id]; amt > 0 {
			h.Emit(events.Event{Kind: events.CounterChange, Obj: id, Counter: kind, Amount: amt})
			placed = append(placed, state.Target{Obj: id})
		}
	}
	return placed
}

func objTargets(ids []state.ObjID) []state.Target {
	out := make([]state.Target, 0, len(ids))
	for _, id := range ids {
		if id != 0 {
			out = append(out, state.Target{Obj: id})
		}
	}
	return out
}

// putCounterChoose runs the bare-Choices$ PutCounter pick shape (no
// DividedAsYouChoose$ — task vow1; Promise of Loyalty's vow, mikey_mona's
// "target player chooses a creature they control and puts two +1/+1 counters
// on it", Haphazard Bombardment's four aim counters): the CHOOSER
// (Chooser$, default the resolving controller) picks
// MinChoiceAmount$..ChoiceAmount$ (default 1..1) battlefield objects out of
// the Choices$ pool, and EACH chosen object takes the full CounterNum$
// counters. The answer re-enters through ResumeKind "counter_pick" with
// Ctx.CounterPick; RememberCards$ True remembers the countered objects (the
// vow chain's SacAllOthers and DBEffect read them in the same walk).
//
// The ask gate is the strict-supersets rule every asking primitive here
// follows (a decision nobody could answer differently is never emitted): a
// pool with fewer than two eligible objects, a Max below one, or a Min at or
// above the eligible count leaves only the deterministic first-Max answer,
// so the placement runs without an ask. Promise of Loyalty's "each player
// chooses ONE creature" asks exactly when that player controls two or more
// eligible creatures.
func putCounterChoose(h Host, c *Ctx, sa *cards.SA, n int32, kind string, ans []state.ObjID, done bool) {
	if n <= 0 {
		return
	}
	g := h.Game()
	spec := strings.TrimSpace(sa.Params["Choices"])
	if done {
		// Re-entry: the answered pick, in answer order. A chosen creature
		// that left the battlefield while the decision was outstanding takes
		// nothing (the same totality stance the divided sibling takes).
		putCounterPickApply(h, c, sa, n, kind, ans)
		return
	}
	chooser, ok := putCounterChooserFor(h, c, strings.TrimSpace(sa.Params["Chooser"]))
	if !ok {
		h.Emit(events.Event{Kind: events.Note, Obj: c.Source,
			Text: "PutCounter Chooser$ unresolvable (" + sa.Params["Chooser"] + ")"})
		return
	}
	// Placer$ names whose placement the counters are attributed to; on the
	// choice shape every carrier spells it equal to Chooser$ and the
	// placement target is the chosen object regardless, so a value that
	// resolves to the chooser (or is absent) is a silent no-op and anything
	// else is one loud Note, placement unchanged.
	if pl := strings.TrimSpace(sa.Params["Placer"]); pl != "" {
		if pp, pok := putCounterChooserFor(h, c, pl); !pok || pp != chooser {
			h.Emit(events.Event{Kind: events.Note, Obj: c.Source,
				Text: "PutCounter Placer$ unmodelled (" + pl + ")"})
		}
	}
	var eligible []state.ObjID
	for _, p := range g.AliveFrom(0) {
		for _, id := range g.Zone(state.ZBattlefield, p) {
			if MatchesSpecCtx(g, spec, id, c.SpecContext(c.Controller)) {
				eligible = append(eligible, id)
			}
		}
	}
	minCh := Num(h, c, sa, "MinChoiceAmount", 1)
	if minCh < 0 {
		minCh = 0
	}
	maxCh := Num(h, c, sa, "ChoiceAmount", 1)
	if maxCh < 0 {
		maxCh = 0
	}
	if maxCh > int32(len(eligible)) {
		maxCh = int32(len(eligible))
	}
	if minCh > maxCh {
		minCh = maxCh
	}
	// The no-choice fallback shares one deterministic pick list with the
	// divided sibling: the first maxCh eligible objects in zone order -- for
	// a forced set (minCh >= the eligible count) that IS the only legal
	// answer, and for the fuzz/no-host run (R-9) it is the exact mirror of
	// botpolicy's "counter_pick" arm, so a bot-answered ask emits the same
	// events the silent build did.
	fallback := func() {
		picks := eligible
		if int32(len(picks)) > maxCh {
			picks = picks[:maxCh]
		}
		putCounterPickApply(h, c, sa, n, kind, picks)
	}
	if len(eligible) < 2 || maxCh < 1 || minCh >= int32(len(eligible)) {
		fallback()
		return
	}
	d := &decision.Decision{Player: chooser, Kind: decision.KChoose,
		Min:        int(minCh),
		Max:        int(maxCh),
		Source:     c.Source,
		ResumeKind: "counter_pick",
		ResumeSA:   sa,
		Prompt:     sa.Params["ChoiceTitle"]}
	for _, id := range eligible {
		name := "a creature"
		if o := g.Obj(id); o != nil && o.Face() != nil {
			name = o.Face().Name
		}
		d.Options = append(d.Options, decision.Option{Index: len(d.Options),
			Kind: "counter_pick", Label: name, Obj: id, Player: chooser})
	}
	if Ask(h, d) == AskAsked {
		return // resolution suspended; the answer re-enters with Ctx.CounterPick set.
	}
	fallback()
}

// putCounterPickApply places CounterNum$ counters on each live chosen object
// and, when the SA carries RememberCards$ True, remembers exactly the ones
// that took a counter.
func putCounterPickApply(h Host, c *Ctx, sa *cards.SA, n int32, kind string, picks []state.ObjID) {
	g := h.Game()
	var placed []state.Target
	for _, id := range picks {
		o := g.Obj(id)
		if o == nil || o.Zone != state.ZBattlefield {
			continue
		}
		h.Emit(events.Event{Kind: events.CounterChange, Obj: id, Counter: kind, Amount: n})
		placed = append(placed, state.Target{Obj: id})
	}
	rememberPlaced(c, sa, placed)
}

// putCounterChooserFor resolves one Chooser$/Placer$ value of the bare-
// Choices$ PutCounter pick to the player who answers. The allowlist covers
// every spelling the corpus's six Chooser$ carriers use plus the obvious
// defaults; anything else fails closed (nil, false) — a chooser is never
// guessed, because asking the WRONG player would record a choice nobody
// made. The Remembered-backed spellings read the RESOLUTION's Remembered
// player entries: inside a RepeatEach iteration the loop subject is exactly
// that entry (Promise of Loyalty's Player.IsRemembered, Eye of Doom's bare
// Remembered), falling back to the source's event-backed list the same way
// the Player.IsRemembered Defined selector does.
func putCounterChooserFor(h Host, c *Ctx, v string) (state.PlayerID, bool) {
	g := h.Game()
	firstRememberedPlayer := func() (state.PlayerID, bool) {
		for _, t := range c.Remembered {
			if t.IsPlayer {
				return t.Player, true
			}
		}
		if o := g.Obj(c.Source); o != nil {
			for _, t := range o.Remembered {
				if t.IsPlayer {
					return t.Player, true
				}
			}
		}
		return 0, false
	}
	firstChosenPlayer := func() (state.PlayerID, bool) {
		for _, t := range c.Chosen {
			if t.IsPlayer {
				return t.Player, true
			}
		}
		if o := g.Obj(c.Source); o != nil {
			for _, t := range o.Chosen {
				if t.IsPlayer {
					return t.Player, true
				}
			}
		}
		return 0, false
	}
	switch v {
	case "", "You", "True":
		return c.Controller, true
	case "Player.IsRemembered", "Remembered", "RememberedController":
		return firstRememberedPlayer()
	case "ChosenPlayer", "Player.Chosen":
		return firstChosenPlayer()
	case "TriggeredPlayer":
		if c.TriggerPlayer.IsPlayer {
			return c.TriggerPlayer.Player, true
		}
		return 0, false
	case "ThisTargetedPlayer", "TargetedPlayer", "Targeted":
		for _, t := range c.Targets {
			if t.IsPlayer {
				return t.Player, true
			}
			if o := g.Obj(t.Obj); o != nil {
				return o.Controller, true
			}
		}
		return 0, false
	}
	return 0, false
}

// effPutCounterAll sweeps ValidCards$ (default "Permanent") over the
// battlefield in deterministic order (g.AliveFrom(0), then each seat's zone
// order -- never a map range) and places CounterType$ (default "P1P1")
// counters on each match: CounterNum$ resolved through Num (default 1,
// negative clamped to 0 like both siblings), one events.CounterChange per
// recipient with a signed positive Amount. It is the mass-placement mirror
// of effRemoveCounterAll.
//
// Player-targeted sweep (ValidTgts$ Player, 7 raw corpus lines over 6
// carriers, e.g.
// Meadowboon's "put a +1/+1 counter on each creature target player
// controls"): the sweep is scoped to each CHOSEN player target's battlefield
// instead of the whole table; ValidCards$ stays the filter inside that
// scope, so an unqualified "Creature" means the target player's creatures.
// A ValidTgts$ Player line that reaches resolution with no chosen player
// target is loud, never silent.
//
// A second batch (ValidCards2$/CounterType2$/CounterNum2$, e.g. Brokers
// Ascendancy's "...and a loyalty counter on each planeswalker you control")
// runs as a second sweep after the first, with its own filter, kind (default
// P1P1) and count (default 1).
//
// Exotic parameters the core sweep cannot express stay LOUD (the
// effManifest out-of-scope pattern): Placer$ (who places, 7 corpus lines),
// TargetUnique$ (2) and AmountByChosenMap$ (1), and a ValidZone$ naming any
// zone other than the battlefield (2, both Exile -- suspended TIME
// counters). Registering the API removed the generic "unimplemented API"
// fallback, so without these notes the shapes would silently place nothing.
func effPutCounterAll(h Host, c *Ctx, sa *cards.SA) {
	var exotic []string
	if strings.TrimSpace(sa.Params["Placer"]) != "" {
		exotic = append(exotic, "Placer$")
	}
	if strings.TrimSpace(sa.Params["TargetUnique"]) != "" {
		exotic = append(exotic, "TargetUnique$")
	}
	if strings.TrimSpace(sa.Params["AmountByChosenMap"]) != "" {
		exotic = append(exotic, "AmountByChosenMap$")
	}
	if zone := strings.TrimSpace(sa.Params["ValidZone"]); zone != "" && !strings.EqualFold(zone, "Battlefield") {
		exotic = append(exotic, "ValidZone$ "+zone)
	}
	// A ValidTgts$ naming anything but the plain chosen-player sweep is an
	// exotic shape: anything else (Corrosion's "Opponent", a named
	// selector, a compound) would fall through to the whole-table branch
	// below and sweep the WRONG-WIDE set silently. Loud instead.
	if tgts := strings.TrimSpace(sa.Params["ValidTgts"]); tgts != "" && !strings.EqualFold(tgts, "Player") {
		exotic = append(exotic, "ValidTgts$ "+tgts)
	}
	if len(exotic) > 0 {
		h.Emit(events.Event{Kind: events.Note, Obj: c.Source,
			Text: "unimplemented PutCounterAll shape: " + strings.Join(exotic, ", ")})
		return
	}
	putCounterAllSweep(h, c, sa, sa.Params["ValidCards"], sa.Params["CounterType"], Num(h, c, sa, "CounterNum", 1))
	if strings.TrimSpace(sa.Params["ValidCards2"]) != "" {
		putCounterAllSweep(h, c, sa, sa.Params["ValidCards2"], sa.Params["CounterType2"], Num(h, c, sa, "CounterNum2", 1))
	}
}

// putCounterAllSweep is one PutCounterAll batch: filter spec, counter kind
// and count already resolved from their literal Params keys.
func putCounterAllSweep(h Host, c *Ctx, sa *cards.SA, spec, kind string, n int32) {
	if kind == "" {
		kind = "P1P1"
	}
	if spec == "" {
		spec = "Permanent"
	}
	if n < 0 {
		n = 0
	}
	if n <= 0 {
		// Same discipline as effRemoveCounterAll: a zero-amount batch is a
		// no-op, and emitting one CounterChange whose Amount overstates what
		// changed per object is log noise (an unresolvable CounterNum$
		// degrades to 0 through Num).
		return
	}
	players := h.Game().AliveFrom(0)
	if strings.TrimSpace(sa.Params["ValidTgts"]) == "Player" {
		players = nil
		for _, t := range c.Targets {
			if t.IsPlayer {
				players = append(players, t.Player)
			}
		}
		if len(players) == 0 {
			h.Emit(events.Event{Kind: events.Note, Obj: c.Source,
				Text: "unimplemented PutCounterAll shape: player-targeted sweep with no chosen player target"})
			return
		}
	}
	g := h.Game()
	for _, p := range players {
		for _, id := range g.Zone(state.ZBattlefield, p) {
			if !MatchesSpecCtx(g, spec, id, c.SpecContext(c.Controller)) {
				continue
			}
			h.Emit(events.Event{Kind: events.CounterChange, Obj: id, Counter: kind, Amount: n})
		}
	}
}

// effRemoveCounterAll sweeps ValidCards$ (default "Permanent") on the
// battlefield and removes CounterType$ counters from each match: CounterNum$
// (default 1) of them, or every counter of that kind the object actually has
// when AllCounters$ is "True". state.Object.AddCounter already clamps at
// zero, so removing more than an object has is harmless either way; the
// AllCounters$ case reads the object's own count first purely to avoid an
// event whose Amount overstates what changed.
func effRemoveCounterAll(h Host, c *Ctx, sa *cards.SA) {
	kind := sa.Params["CounterType"]
	if kind == "" {
		return
	}
	spec := sa.Params["ValidCards"]
	if spec == "" {
		spec = "Permanent"
	}
	all := sa.Params["AllCounters"] == "True"
	n := Num(h, c, sa, "CounterNum", 1)
	if n < 0 {
		n = 0
	}
	g := h.Game()
	for _, p := range g.AliveFrom(0) {
		for _, id := range g.Zone(state.ZBattlefield, p) {
			if !MatchesSpecCtx(g, spec, id, c.SpecContext(c.Controller)) {
				continue
			}
			amt := n
			if all {
				amt = g.Obj(id).Counter(kind)
			}
			if amt <= 0 {
				continue
			}
			h.Emit(events.Event{Kind: events.CounterChange, Obj: id, Counter: kind, Amount: -amt})
		}
	}
}

// effRegenerate grants a this-turn shield consumed by ReplaceDestruction.
func effRegenerate(h Host, c *Ctx, sa *cards.SA) {
	for _, t := range Defined(h, c, sa) {
		if t.IsPlayer {
			continue
		}
		o := h.Game().Obj(t.Obj)
		if o == nil || o.Zone != state.ZBattlefield {
			continue
		}
		h.Emit(events.Event{Kind: events.CounterChange, Obj: o.ID, Counter: "Shield", Amount: 1})
	}
}
