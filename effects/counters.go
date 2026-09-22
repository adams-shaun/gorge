package effects

import (
	"fmt"
	"sort"
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
	Register("RemoveCounter", effRemoveCounter)
	Register("MoveCounter", effMoveCounter)
	Register("MultiplyCounter", effMultiplyCounter)
	Register("Proliferate", effProliferate)
	Register("Regenerate", effRegenerate)
}

// effMultiplyCounter is Forge's MultiplyCounterEffect: for each object or
// player the Defined$/ValidTgts$ spec names, ADD (Multiplier-1) x the current
// count of the affected counter kind(s) -- so the default Multiplier$ 2
// exactly DOUBLES them. CounterType$ names the ONE kind to multiply; absent
// ("double the number of EACH KIND of counter on target permanent",
// Aetheric Amplifier, Deepglow Skate, The Thing, Miles Morales), every kind
// the object already carries is multiplied. Multiplier$ resolves through Num,
// so a literal (the whole corpus: Multiplier$ 2), an SVar or an inline
// Count$ expression all price the same path; an absent Multiplier$ is 2.
//
// A target with no counters of the relevant kind(s) emits nothing -- adding
// zero is a no-op and one CounterChange of Amount 0 would be log noise (the
// effRemoveCounterAll discipline). One event per kind per target, so the
// event stream records the real CounterChange/PlayerCounterChange the engine
// folds, never a snapshot write.
func effMultiplyCounter(h Host, c *Ctx, sa *cards.SA) {
	mult := Num(h, c, sa, "Multiplier", 2)
	if mult < 1 {
		mult = 1
	}
	kind := strings.TrimSpace(sa.Params["CounterType"])
	g := h.Game()
	for _, t := range Defined(h, c, sa) {
		if t.IsPlayer {
			p := PlayerOf(h, c, t)
			if int(p) < 0 || int(p) >= len(g.Players) {
				continue
			}
			pl := &g.Players[p]
			// Deterministic: the player's own counter slice order, which is
			// insertion order and rebuilt identically on replay.
			kinds := counterKinds(kind, len(pl.Counters), func(i int) string { return pl.Counters[i].Kind }, func(i int) int32 { return pl.Counters[i].N })
			for _, k := range kinds {
				if add := (mult - 1) * pl.Counter(k); add > 0 {
					h.Emit(events.Event{Kind: events.PlayerCounterChange, Player: p,
						Counter: k, Amount: add})
				}
			}
			continue
		}
		o := g.Obj(t.Obj)
		if o == nil || o.Zone != state.ZBattlefield {
			continue
		}
		kinds := counterKinds(kind, len(o.Counters), func(i int) string { return o.Counters[i].Kind }, func(i int) int32 { return o.Counters[i].N })
		for _, k := range kinds {
			if add := (mult - 1) * o.Counter(k); add > 0 {
				h.Emit(events.Event{Kind: events.CounterChange, Obj: o.ID, Counter: k, Amount: add})
			}
		}
	}
}

// counterKinds is the kind list a counter primitive walks: the single
// CounterType$ when named, otherwise every kind the carrier already holds, in
// its own deterministic slice order (never a map walk).
//
// It reports only kinds the carrier actually has a POSITIVE count of. A slot
// can survive its counters being removed down to zero -- state's AddCounter
// clamps at zero and never prunes the slice entry (state/object.go,
// state/game.go) -- so a drained slot must not count as "has this kind":
// proliferating onto it would add a counter of a kind that is no longer there
// (CR 701.27a) and the eligibility gate below would offer a recipient with no
// counters at all. Callers pass the per-index count so the one helper is the
// single place that filters.
func counterKinds(kind string, n int, at func(int) string, count func(int) int32) []string {
	if kind != "" {
		return []string{kind}
	}
	out := make([]string, 0, n)
	for i := 0; i < n; i++ {
		if count(i) > 0 {
			out = append(out, at(i))
		}
	}
	return out
}

// hasCounters reports whether a counter carrier holds at least one counter of
// ANY kind, testing the COUNT and not the slice length (a drained slot stays in
// the slice at N == 0). This is the CR 701.27a eligibility gate, shared by the
// object and player walks.
func hasCounters(kinds []state.Counter) bool {
	for i := range kinds {
		if kinds[i].N > 0 {
			return true
		}
	}
	return false
}

func effPutCounter(h Host, c *Ctx, sa *cards.SA) {
	// fx42 scoping: consume and clear the answered Optional$ election at the
	// top, so a nested PutCounter in the same chain poses its own ask.
	optAns := c.PutOpt
	c.PutOpt = ""
	// Adapt$ (CR 702.35a; task param-adapt): an AB/DB$ PutCounter carrying
	// Adapt$ N reads N as the count -- Pteramander's `Adapt$ 4`, Jetfire's
	// chained `SVar:DBAdapt:DB$ PutCounter | Adapt$ 3`. The corpus writes only
	// literal values (measured: Adapt$ 1-4 over the 25 raw lines) and no
	// carrier pairs it with CounterNum$, so the read is a fallback, never a
	// competition. An unresolvable body degrades to 0, Num's convention.
	n := int32(1)
	if strings.TrimSpace(sa.Params["CounterNum"]) != "" {
		n = Num(h, c, sa, "CounterNum", 1)
	} else if strings.TrimSpace(sa.Params["Adapt"]) != "" {
		n = Num(h, c, sa, "Adapt", 1)
	} else if strings.TrimSpace(sa.Params["Monstrosity"]) != "" {
		// Monstrosity$ (CR 701.31; Giggling Skitterspike's `{5}: Monstrosity
		// 5`, task agent-20260919T190014Z): a Monstrosity line names its own
		// counter amount and no carrier pairs it with CounterNum$/Adapt$
		// (measured over the 36 raw corpus lines), so the read is a fallback
		// in the same chain. The literal AND X shapes resolve through the
		// ordinary Num grammar -- the announced X (Domesticated Hydra's
		// `Cost$ X G G G`, Vitality Hunter's `Cost$ X W W`) and an SVar X
		// (Grim Giganotosaurus's `SVar:X:Count$Valid
		// Creature.OppCtrl+powerGE4`). An unresolvable body degrades to 0,
		// Num's convention.
		n = Num(h, c, sa, "Monstrosity", 1)
	}
	if n < 0 {
		n = 0
	}
	adapt := strings.TrimSpace(sa.Params["Adapt"]) != ""
	mono := strings.TrimSpace(sa.Params["Monstrosity"]) != ""
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
	// Bolster$ (CR 701.36's bolster keyword action; 24 raw corpus lines, every
	// one of them a PutCounter SA -- SP/AB/DB heads alike -- so no separate
	// DB$ Bolster API is needed): "choose a creature with the least toughness
	// among creatures you control and put N +1/+1 counters on it." Bolster$
	// names N (a literal, or an SVar -- Sandsteppe War Riders' Bolster$ X);
	// the counter kind is the ordinary CounterType$ (default P1P1) computed
	// above. fx42 scoping: consume and clear the answered tie pick first, so
	// a nested PutCounter in the same chain cannot inherit it.
	if _, ok := sa.Params["Bolster"]; ok {
		pickAns := c.CounterPick
		pickDone := c.CounterPickDone
		c.CounterPick, c.CounterPickDone = nil, false
		putCounterBolster(h, c, sa, kind, pickAns, pickDone)
		return
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
		// Adapt$'s put is itself conditional (CR 702.35a: "If this creature has
		// no +1/+1 counters on it, put N +1/+1 counters on it"). For an AB$
		// activation the offer-time gate (rules/legal.go's adaptGateOK) already
		// withheld the ability while counters were present, but counters can
		// arrive in response between activation and resolution -- the effect's
		// own if-condition, not the activation restriction, is what governs the
		// put. It is also the only gate a chained DB$ body (Jetfire's
		// "then adapt 3") ever gets, since an activation restriction does not
		// govern a resolution-time body.
		if adapt && o.Counter("P1P1") > 0 {
			continue
		}
		// CR 701.31b defense-in-depth: a monstrosity ability's activation is
		// gated once-only at offer time (rules/legal.go's monstrosityGateOK),
		// but the resolve-time read keeps an already-monstrous permanent from
		// taking a second batch through a path no offer gate covers (a
		// chained body, a future granted route). Corpus-unreachable today.
		if mono && o.Monstrous {
			continue
		}
		h.Emit(events.Event{Kind: events.CounterChange, Obj: o.ID, Counter: kind, Amount: n})
		// The mark (CR 701.31b: "...and it becomes monstrous"): one
		// AlterAttribute per placed object, emitted AFTER its counters so the
		// BecomeMonstrous triggers see the counters already landed. Amount is
		// the monstrosity COUNT -- Hydra Broodmaster's
		// `SVar:MonstrosityX:TriggerCount$Amount` reads the triggering event's
		// Amount -- and Player names the controller at mark time so the
		// trigger's referents bind it. Gated on the param's presence, so every
		// other PutCounter shape emits byte-identically.
		if mono {
			h.Emit(events.Event{Kind: events.AlterAttribute, Obj: o.ID,
				Player: o.Controller, Text: "Monstrous", Amount: n})
		}
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

// putCounterBolster implements PutCounter's Bolster$ parameter (CR 701.36's
// bolster keyword action): the bolstering player -- the resolving controller;
// every corpus carrier bolsters "creatures you control" -- chooses a creature
// with the LEAST toughness among their creatures and puts N +1/+1 counters
// on it. The toughness read is the engine's effects-side P/T convention
// (face toughness plus P1P1 counters, the Count$CardToughness read --
// layer-derived toughness is not visible below rules). A tie at the minimum
// is a real election (CR 701.36's "choose"): a KChoose over the tied
// creatures answered through the shared "counter_pick" resume arm
// (Ctx.CounterPick/CounterPickDone, consumed and cleared by the caller --
// fx42); botpolicy's "counter_pick" arm takes the first option, so a
// bot-answered game emits the same events the R-9 no-host fallback (the
// first tied creature in zone order) does. RememberCards$ True rides
// putCounterPickApply, exactly like the bare Choices$ pick.
func putCounterBolster(h Host, c *Ctx, sa *cards.SA, kind string, ans []state.ObjID, done bool) {
	n := Num(h, c, sa, "Bolster", 1)
	if n < 0 {
		n = 0
	}
	g := h.Game()
	var cands []state.ObjID
	best := int32(0)
	for _, id := range g.Zone(state.ZBattlefield, c.Controller) {
		o := g.Obj(id)
		if o == nil || o.Face() == nil || !hasType(o, "Creature") {
			continue
		}
		t := int32(o.Face().Toughness()) + o.Counter("P1P1")
		if len(cands) == 0 || t < best {
			best, cands = t, []state.ObjID{id}
			continue
		}
		if t == best {
			cands = append(cands, id)
		}
	}
	if n == 0 || len(cands) == 0 {
		// Nothing to place or nobody to place it on: bolstering zero is a
		// no-op (the election and the decline would place the same thing).
		return
	}
	if len(cands) == 1 {
		putCounterPickApply(h, c, sa, n, kind, cands)
		return
	}
	if done {
		// Re-entry: the answered tie pick, applied as-is (a chosen creature
		// that left the battlefield while the decision was outstanding takes
		// nothing -- putCounterPickApply's zone guard).
		putCounterPickApply(h, c, sa, n, kind, ans)
		return
	}
	d := &decision.Decision{Player: c.Controller, Kind: decision.KChoose,
		Min: 1, Max: 1, Source: c.Source,
		ResumeKind: "counter_pick", ResumeSA: sa,
		Prompt: "Bolster " + strconv.Itoa(int(n)) + " — choose a creature with the least toughness"}
	for _, id := range cands {
		name := "a creature"
		if o := g.Obj(id); o != nil && o.Face() != nil {
			name = o.Face().Name
		}
		d.Options = append(d.Options, decision.Option{Index: len(d.Options),
			Kind: "counter_pick", Label: name, Obj: id, Player: c.Controller})
	}
	if Ask(h, d) == AskAsked {
		return // resolution suspended; the answer re-enters with Ctx.CounterPick set.
	}
	// The R-9 no-host fallback: the first tied creature in zone order -- the
	// exact mirror of botpolicy's "counter_pick" first-option answer.
	putCounterPickApply(h, c, sa, n, kind, cands[:1])
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

// effRemoveCounter is Forge's RemoveCounterEffect: for each object or player
// the Defined$ spec names (an ability with no Defined$ acts on its chosen
// targets through Ctx.Targets; one that names neither acts on its source --
// the ordinary Defined contract), remove CounterNum$ counters of CounterType$
// from it. The corpus shape is overwhelmingly the chained DB$ sub-ability
// (178 of 202 raw RemoveCounter lines), Self-dominated: Prize Pig's
// "remove those counters and untap it" payoff is the pin.
//
// CounterNum$ resolves through the shared Num evaluator (a literal, an SVar
// name such as the corpus's X/Y/SekkiX/Result/NumDmg, or an inline Count$);
// "All" means every counter of that kind the object actually has -- the
// AllCounters$ discipline effRemoveCounterAll uses, reading the count first
// so the event's Amount never overstates. A literal n over an object holding
// fewer also clamps to what is there (state.Object.AddCounter clamps at zero
// either way; emitting -n would overstate). One events.CounterChange with a
// signed negative Amount per (object, kind); an object with none of the kind
// emits nothing (the zero-batch no-op discipline both siblings follow).
//
// CounterType$ All means EVERY kind the object holds, in its own
// deterministic slice order (the counterKinds helper this file already
// shares with MultiplyCounter), one CounterChange per kind present.
//
// RememberRemoved$ True records one remembered entry per removed counter on
// the source's persistent (event-backed) remembered list -- one Choose
// "remembered" event per (object, kind) batch, the object's id repeated once
// per removed counter, so Count$RememberedSize (the HOST CARD's list,
// effects/count.go) reads the truthful size. Prize Pig's untap gate is
// ConditionCheckSVar$ X with SVar:X:Count$RememberedSize, so this rider is
// load-bearing for it. Duplicates are deliberate: the only consumer measured
// for this rider is a size count (and Cleanup's ClearRemembered$ clears the
// list again), so one entry per counter is the honest encoding.
//
// Exotic shapes stay LOUD (the effPutCounterAll exotic pattern -- one Note
// naming the shape, nothing moves): CounterType$ Any (a choose-which-kind
// ask), Choices$/ChoiceOptional$ (a mid-resolution pick), UpTo$ (a bounded
// election), CounterNum$ Any, CounterNumShared$, a TgtZone$ naming anything
// but the battlefield (the suspended-TIME-counter family), RememberAmount$
// (a removed NUMBER the remembered list has no honest channel for) and
// Optional$ (a may-remove election). Registering the API removed the generic
// "unimplemented API" fallback, so without these notes the shapes would
// silently remove nothing.
// One Note per Defined$-named OBJECT that is not on the battlefield (or no
// longer exists) instead of the silent skip r1 shipped: the corpus's
// RememberedLKI/Imprinted/ChosenCard defined sets can name a graveyard or
// exile card, the TgtZone$ gate above is the only sanctioned off-battlefield
// family, and swallowing a named target silently is an unledgered narrowing.
// The note records the skip loudly; nothing moves (removal off-battlefield
// stays a TgtZone$ task).
func effRemoveCounter(h Host, c *Ctx, sa *cards.SA) {
	var exotic []string
	if strings.EqualFold(strings.TrimSpace(sa.Params["CounterType"]), "Any") {
		exotic = append(exotic, "CounterType$ Any")
	}
	if strings.TrimSpace(sa.Params["Choices"]) != "" || strings.TrimSpace(sa.Params["ChoiceOptional"]) != "" {
		exotic = append(exotic, "Choices$")
	}
	if strings.TrimSpace(sa.Params["UpTo"]) != "" {
		exotic = append(exotic, "UpTo$")
	}
	if strings.EqualFold(strings.TrimSpace(sa.Params["CounterNum"]), "Any") {
		exotic = append(exotic, "CounterNum$ Any")
	}
	if strings.TrimSpace(sa.Params["CounterNumShared"]) != "" {
		exotic = append(exotic, "CounterNumShared$")
	}
	if zone := strings.TrimSpace(sa.Params["TgtZone"]); zone != "" && !strings.EqualFold(zone, "Battlefield") {
		exotic = append(exotic, "TgtZone$ "+zone)
	}
	if strings.EqualFold(strings.TrimSpace(sa.Params["RememberAmount"]), "True") {
		exotic = append(exotic, "RememberAmount$")
	}
	if strings.EqualFold(strings.TrimSpace(sa.Params["Optional"]), "True") {
		exotic = append(exotic, "Optional$")
	}
	if len(exotic) > 0 {
		h.Emit(events.Event{Kind: events.Note, Obj: c.Source,
			Text: "unimplemented RemoveCounter shape: " + strings.Join(exotic, ", ")})
		return
	}
	kind := strings.TrimSpace(sa.Params["CounterType"])
	if kind == "" {
		kind = "P1P1"
	}
	allKinds := strings.EqualFold(kind, "All")
	numAll := false
	n := int32(1)
	if raw := strings.TrimSpace(sa.Params["CounterNum"]); raw != "" {
		if strings.EqualFold(raw, "All") {
			numAll = true
		} else {
			n = Num(h, c, sa, "CounterNum", 1)
			if n < 0 {
				n = 0
			}
		}
	}
	kindArg := kind
	if allKinds {
		kindArg = "" // counterKinds: every kind the carrier holds
	}
	g := h.Game()
	for _, t := range Defined(h, c, sa) {
		if t.IsPlayer {
			p := PlayerOf(h, c, t)
			if int(p) < 0 || int(p) >= len(g.Players) {
				continue
			}
			pl := &g.Players[p]
			for _, k := range dedupeKinds(counterKinds(kindArg, len(pl.Counters), func(i int) string { return pl.Counters[i].Kind }, func(i int) int32 { return pl.Counters[i].N })) {
				count := pl.Counter(k)
				amt := n
				if numAll {
					amt = count
				}
				removed := amt
				if removed > count {
					removed = count
				}
				if removed <= 0 {
					continue
				}
				h.Emit(events.Event{Kind: events.PlayerCounterChange, Player: p, Counter: k, Amount: -removed})
				rememberRemoved(h, c, sa, state.PlayerRef(p), removed)
			}
			continue
		}
		o := g.Obj(t.Obj)
		if o == nil {
			h.Emit(events.Event{Kind: events.Note, Obj: c.Source,
				Text: fmt.Sprintf("RemoveCounter target %d no longer exists; skipped", t.Obj)})
			continue
		}
		if o.Zone != state.ZBattlefield {
			h.Emit(events.Event{Kind: events.Note, Obj: c.Source,
				Text: fmt.Sprintf("RemoveCounter target %d is not on the battlefield (zone %s); skipped", o.ID, o.Zone)})
			continue
		}
		for _, k := range dedupeKinds(counterKinds(kindArg, len(o.Counters), func(i int) string { return o.Counters[i].Kind }, func(i int) int32 { return o.Counters[i].N })) {
			count := o.Counter(k)
			amt := n
			if numAll {
				amt = count
			}
			removed := amt
			if removed > count {
				removed = count
			}
			if removed <= 0 {
				continue
			}
			h.Emit(events.Event{Kind: events.CounterChange, Obj: o.ID, Counter: k, Amount: -removed})
			rememberRemoved(h, c, sa, o.ID, removed)
		}
	}
}

// dedupeKinds preserves first-occurrence order and drops repeats, so a
// carrier whose Counters slice ever holds the same kind twice emits one
// CounterChange per kind, never two.
func dedupeKinds(kinds []string) []string {
	if len(kinds) < 2 {
		return kinds
	}
	seen := make(map[string]bool, len(kinds))
	out := kinds[:0]
	for _, k := range kinds {
		if seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, k)
	}
	return out
}

// effMoveCounter implements Forge's MoveCounterEffect: counters of
// CounterType$ move from an ORIGIN set to a DESTINATION set -- a counter
// leaves its origin once and lands ONCE (CR 122.5: a counter moves, it is
// not copied and not removed-and-put; a multi-destination sweep DISTRIBUTES
// the moved total across the destinations, round-robin in destination
// order). `effects.Register("MoveCounter")` is what makes the ability offered
// at all; before it every carrier fell to the generic
// "unimplemented API MoveCounter" Note and moved nothing (Diamond City's
// second ability, Weapon Rack, Aetherborn Marauder, Spike Cannibal, Bioshift,
// ... -- 33 raw SA lines over 32 corpus files).
//
// Forge names the two sets with four params, and the corpus's own oracle text
// disambiguates every combination:
//
//   - Origin = Source$ when present (a Defined$-style selector: Self is the
//     ability's source, ParentTarget/Targeted the parent ability's chosen
//     targets), else ValidSource$ (a battlefield filter sweep), else the
//     chosen targets.
//   - Destination = Defined$ when present, else ValidDefined$ (a sweep),
//     else the chosen targets -- but when the origin ALSO came from the
//     chosen targets (the 2-target "... onto another target ..." spells),
//     target 0 is the origin and the REMAINING targets are the destinations.
//
// CounterType$: a literal kind (P1P1/LOYALTY/SHIELD/...), All (every kind the
// carrier holds), EachNotOn (each kind the origin holds that the destination
// does not -- Goldberry), or Any (a real "choose a kind" ask among the
// distinct kinds the origin holds; a single-kind origin takes that kind with
// no ask, the strict-supersets convention). CounterNum$: a literal, X, All
// (everything of the kind on the origin), or Any (a real any-number ask up to
// what the origin holds -- Min 0 is a legitimate decline).
//
// Riders: RememberPut$ True appends each DESTINATION that received a counter
// to Ctx.Remembered (Goldberry's SVar:X:Remembered$Amount gate);
// RememberAmount$ True appends the origin id once per counter moved, so the
// engine's list-length channel (Count$RememberedNumber == len(Ctx.Remembered),
// the same encoding rememberRemoved uses) reads the AMOUNT (Black Panther's
// SVar:X:Count$RememberedNumber). An unresolvable CounterNum$ body, a
// CounterNum$ Any riding a multi-kind CounterType$ (All/EachNotOn), a
// TgtZone$ naming anything but the battlefield, and player origins or
// destinations are LOUD-degraded -- one Note naming the shape, nothing moves
// (the effPutCounterAll/effRemoveCounter exotic pattern). TargetUnique$ True
// (one carrier, vacuous on a single-target ask) is tolerated silently.
func effMoveCounter(h Host, c *Ctx, sa *cards.SA) {
	// fx42 scoping: capture and clear the answered asks BEFORE the walk, so a
	// nested MoveCounter below this one poses its own ask instead of
	// inheriting the outer answer (the Proliferate discipline).
	kindAns, kindDone := c.MoveCounterKind, c.MoveCounterKindDone
	nAns, nDone := c.MoveCounterN, c.MoveCounterNDone
	c.MoveCounterKind, c.MoveCounterKindDone = "", false
	c.MoveCounterN, c.MoveCounterNDone = 0, false

	kindParam := strings.TrimSpace(sa.Params["CounterType"])
	numParam := strings.TrimSpace(sa.Params["CounterNum"])

	// Loud-degrade the shapes the core cannot express, before anything moves.
	var exotic []string
	if zone := strings.TrimSpace(sa.Params["TgtZone"]); zone != "" && !strings.EqualFold(zone, "Battlefield") {
		exotic = append(exotic, "TgtZone$ "+zone)
	}
	if raw, present := sa.Params["CounterNum"]; present && numParam != "" && !strings.EqualFold(numParam, "All") && !strings.EqualFold(numParam, "Any") {
		if _, ok := NumResolved(h, c, sa, "CounterNum", 1); !ok {
			// A body Num cannot price (a bare SVar name the face lacks, an
			// exotic Count$ head) would silently move zero; name it instead.
			exotic = append(exotic, "CounterNum$ "+raw)
		}
	}
	if len(exotic) > 0 {
		h.Emit(events.Event{Kind: events.Note, Obj: c.Source,
			Text: "unimplemented MoveCounter shape: " + strings.Join(exotic, ", ")})
		return
	}

	origin, originFromTargets, ok := moveCounterOrigin(h, c, sa)
	if !ok {
		return // unresolvable Source$: fail closed (the definedSpec discipline)
	}
	if len(origin) == 0 {
		return
	}
	dests := moveCounterDest(h, c, sa, originFromTargets)
	if len(dests) == 0 {
		return
	}

	// Player origins/destinations are out of scope (no corpus carrier needs
	// PlayerCounterChange here); name the shape rather than invent a move.
	for _, t := range origin {
		if t.IsPlayer {
			h.Emit(events.Event{Kind: events.Note, Obj: c.Source,
				Text: "unimplemented MoveCounter shape: player origin"})
			return
		}
	}
	for _, t := range dests {
		if t.IsPlayer {
			h.Emit(events.Event{Kind: events.Note, Obj: c.Source,
				Text: "unimplemented MoveCounter shape: player destination"})
			return
		}
	}

	g := h.Game()
	// Resolve the origin object(s) once (a battlefield object that left while
	// a nested ask was outstanding is skipped by the move walk below).
	origins := make([]*state.Object, 0, len(origin))
	for _, t := range origin {
		if o := g.Obj(t.Obj); o != nil && o.Zone == state.ZBattlefield {
			origins = append(origins, o)
		}
	}
	if len(origins) == 0 {
		return
	}

	// CounterType$ Any: a real kind pick over the distinct kinds the origins
	// hold, when more than one kind is present (a single-kind origin takes
	// that kind with no ask). A multi-origin Any shape has no single pick to
	// pose; loud-degrade it.
	chosenKind := kindParam
	if strings.EqualFold(kindParam, "Any") {
		if !kindDone {
			if len(origins) != 1 {
				h.Emit(events.Event{Kind: events.Note, Obj: c.Source,
					Text: "unimplemented MoveCounter shape: CounterType$ Any over multiple origins"})
				return
			}
			kinds := moveCounterKindsOf(origins[0])
			if len(kinds) >= 2 {
				chooser := c.Controller
				d := &decision.Decision{Player: chooser, Kind: decision.KChoose,
					Min: 1, Max: 1, Source: c.Source, ResumeKind: "move_counter_kind", ResumeSA: sa,
					Prompt: "MoveCounter: choose a counter kind to move"}
				for _, k := range kinds {
					d.Options = append(d.Options, decision.Option{Index: len(d.Options),
						Kind: "move_counter_kind", Label: k})
				}
				if Ask(h, d) == AskAsked {
					return
				}
				// No host (R-9): the deterministic first-kind stand-in, the same
				// pick botpolicy's arm takes.
				chosenKind = kinds[0]
			} else if len(kinds) == 1 {
				chosenKind = kinds[0]
			} else {
				return // origin holds no counters: nothing moves
			}
		} else {
			chosenKind = kindAns
			if chosenKind == "" {
				return
			}
		}
	}

	// CounterNum$ All means "everything of the kind on the origin" -- resolved
	// per origin below. A literal/X resolves once. CounterNum$ Any asks the
	// amount: only a single-kind, single-origin shape can pose one ask.
	numAll := strings.EqualFold(numParam, "All")
	numAny := strings.EqualFold(numParam, "Any")
	var num int32
	if !numAll && !numAny {
		num = Num(h, c, sa, "CounterNum", 1)
		if num < 0 {
			num = 0
		}
	}
	if numAny {
		if strings.EqualFold(kindParam, "All") || strings.EqualFold(kindParam, "EachNotOn") {
			h.Emit(events.Event{Kind: events.Note, Obj: c.Source,
				Text: "unimplemented MoveCounter shape: CounterNum$ Any over multiple kinds"})
			return
		}
		if len(origins) != 1 {
			h.Emit(events.Event{Kind: events.Note, Obj: c.Source,
				Text: "unimplemented MoveCounter shape: CounterNum$ Any over multiple origins"})
			return
		}
		if !nDone {
			maxN := origins[0].Counter(chosenKind)
			if maxN > 0 {
				chooser := c.Controller
				d := &decision.Decision{Player: chooser, Kind: decision.KChoose,
					Min: 0, Max: int(maxN), Source: c.Source, ResumeKind: "move_counter", ResumeSA: sa,
					Prompt: "MoveCounter: choose how many counters to move"}
				for i := 0; i <= int(maxN); i++ {
					d.Options = append(d.Options, decision.Option{Index: len(d.Options),
						Kind: "move_counter", Label: fmt.Sprintf("%d", i), Amount: i})
				}
				if Ask(h, d) == AskAsked {
					return
				}
			}
			// No host (R-9): the deterministic take-all stand-in.
			num = maxN
		} else {
			num = nAns
		}
	}

	// The move walk. Each origin loses `moved` of each kind; the moved total
	// is DISTRIBUTED across the destinations (CR 122.5 -- see the walk).
	// RememberPut$ appends the destinations that actually received a counter;
	// RememberAmount$ appends the origin once per counter moved.
	rememberPut := strings.EqualFold(strings.TrimSpace(sa.Params["RememberPut"]), "True")
	rememberAmount := strings.EqualFold(strings.TrimSpace(sa.Params["RememberAmount"]), "True")
	var rememberedDests []state.Target
	var amountIDs []state.ObjID
	for _, o := range origins {
		for _, k := range moveCounterKindsFor(chosenKind, o, dests, g) {
			count := o.Counter(k)
			if count <= 0 {
				continue
			}
			moved := num
			if numAll {
				moved = count
			}
			if moved > count {
				moved = count
			}
			if moved <= 0 {
				continue
			}
			// The live destinations for this origin: on the battlefield and
			// not the origin itself (moving a counter from a permanent to
			// itself is a no-op, never a -/+ pair on the same id). Built BEFORE
			// the origin's loss is emitted: an origin whose every destination
			// left the battlefield (or was never live) keeps its counters.
			live := make([]*state.Object, 0, len(dests))
			for _, t := range dests {
				if d := g.Obj(t.Obj); d != nil && d.Zone == state.ZBattlefield && d.ID != o.ID {
					live = append(live, d)
				}
			}
			if len(live) == 0 {
				continue
			}
			h.Emit(events.Event{Kind: events.CounterChange, Obj: o.ID, Counter: k, Amount: -moved})
			// CR 122.5: a moved counter leaves the origin once and lands ONCE
			// -- a multi-destination sweep (Forgotten Ancient's "move any number
			// of +1/+1 counters ... onto other creatures") DISTRIBUTES moved
			// across the destinations, it never gives each destination the
			// whole moved set (that would mint (M-1)*moved counters out of
			// nothing). Distribution is the deterministic round-robin in
			// destination order (a stand-in for the per-destination election
			// the card text implies -- M4): destination j receives
			// floor((moved+M-1-j)/M), so 2 over 2 creatures is 1 and 1, and a
			// single-destination shape -- every other measured carrier -- is
			// byte-identical to the whole-set move it had before.
			m := int32(len(live))
			for j, d := range live {
				share := (moved + m - 1 - int32(j)) / m
				if share <= 0 {
					continue
				}
				h.Emit(events.Event{Kind: events.CounterChange, Obj: d.ID, Counter: k, Amount: share})
				rememberedDests = append(rememberedDests, state.Target{Obj: d.ID})
			}
			for i := int32(0); i < moved; i++ {
				amountIDs = append(amountIDs, o.ID)
			}
		}
	}
	if rememberPut && len(rememberedDests) > 0 {
		c.Remembered = append(c.Remembered, rememberedDests...)
	}
	if rememberAmount && len(amountIDs) > 0 {
		c.Remembered = append(c.Remembered, objTargets(amountIDs)...)
	}
}

// moveCounterChosen is the chosen-target set a MoveCounter's origin/
// destination defaults read: the generic ValidTgts$ pre-ask's ANSWER
// (c.PickedTargets, mvts1) when one is outstanding, else the resolution's own
// target list. Preferring PickedTargets is the established convention
// (effects/context.go Defined, effects/damage.go, effects/zone.go): a sub the
// pre-ask asked (Nesting Grounds' `Source$ ParentTarget | ValidTgts$
// Permanent`, Rikku's, Black Panther's) must receive the sub's OWN chosen
// targets, never the parent's (c.Targets), while a depth-0 shape the
// placement/announcement ask covered (Bioshift's TargetMin$ 2, a Weapon Rack
// activation) has PickedTargets nil and keeps c.Targets.
func moveCounterChosen(c *Ctx) []state.Target {
	if c.PickedTargets != nil {
		return c.PickedTargets
	}
	return c.Targets
}

// moveCounterOrigin resolves the FROM set of a MoveCounter: Source$ (a
// fail-closed Defined$-style selector), else ValidSource$ (a battlefield
// filter sweep), else the chosen targets -- ALL of them when the SA names its
// destination explicitly (Defined$/ValidDefined$, so the chosen targets are
// only the origin half of a "target X, onto CARDNAME" shape), else only
// target 0 (the 2-target "... onto another target ..." spells, where the
// remaining targets are the destinations). originFromTargets reports the
// 2-target branch so the destination default can drop target 0. ok is false
// only for an explicit Source$ this build cannot resolve -- the fail-closed
// direction, never a fallback to the source or the targets.
func moveCounterOrigin(h Host, c *Ctx, sa *cards.SA) (ts []state.Target, fromTargets, ok bool) {
	if src := strings.TrimSpace(sa.Params["Source"]); src != "" {
		t, resolved := definedSpec(h, c, src)
		return t, false, resolved
	}
	if filt := strings.TrimSpace(sa.Params["ValidSource"]); filt != "" {
		return battlefieldValidTargets(h, c, filt), false, true
	}
	if moveCounterNamesDestination(sa) {
		return copyTargets(moveCounterChosen(c)), false, true
	}
	chosen := moveCounterChosen(c)
	if len(chosen) == 0 {
		return nil, true, true
	}
	return copyTargets(chosen[:1]), true, true
}

// moveCounterNamesDestination reports whether the SA names its TO set
// explicitly, which decides whether the chosen targets are the whole origin
// set or just target 0.
func moveCounterNamesDestination(sa *cards.SA) bool {
	return strings.TrimSpace(sa.Params["Defined"]) != "" ||
		strings.TrimSpace(sa.Params["ValidDefined"]) != ""
}

// moveCounterDest resolves the TO set: Defined$, else ValidDefined$, else the
// chosen targets (moveCounterChosen: the pre-ask's answer when one is
// outstanding) -- the remaining targets after target 0 when the origin also
// came from the chosen targets (the 2-target shapes), else all of them.
func moveCounterDest(h Host, c *Ctx, sa *cards.SA, originFromTargets bool) []state.Target {
	if strings.TrimSpace(sa.Params["Defined"]) != "" {
		return Defined(h, c, sa)
	}
	if filt := strings.TrimSpace(sa.Params["ValidDefined"]); filt != "" {
		return battlefieldValidTargets(h, c, filt)
	}
	ts := moveCounterChosen(c)
	if originFromTargets {
		if len(ts) <= 1 {
			return nil
		}
		return copyTargets(ts[1:])
	}
	return copyTargets(ts)
}

// moveCounterKindsOf lists the distinct counter kinds an object holds with a
// POSITIVE count, in deterministic slice order (the counterKinds discipline).
func moveCounterKindsOf(o *state.Object) []string {
	return dedupeKinds(counterKinds("", len(o.Counters),
		func(i int) string { return o.Counters[i].Kind },
		func(i int) int32 { return o.Counters[i].N }))
}

// moveCounterKindsFor expands the CounterType$ parameter for one origin: a
// literal or an Any pick is that one kind; All is every kind the origin
// holds; EachNotOn is each kind the origin holds that NO destination already
// has (Goldberry's "each kind not on CARDNAME").
func moveCounterKindsFor(kind string, o *state.Object, dests []state.Target, g *state.Game) []string {
	switch {
	case strings.EqualFold(kind, "All"):
		return moveCounterKindsOf(o)
	case strings.EqualFold(kind, "EachNotOn"):
		var out []string
		for _, k := range moveCounterKindsOf(o) {
			present := false
			for _, t := range dests {
				if d := g.Obj(t.Obj); d != nil && d.Counter(k) > 0 {
					present = true
					break
				}
			}
			if !present {
				out = append(out, k)
			}
		}
		return out
	default:
		return []string{kind}
	}
}

// rememberRemoved records RememberRemoved$'s persistent half: one Choose
// "remembered" event on the source object with the removed-from object's id
// repeated once per removed counter (events.Apply appends each id, so the
// source's event-backed Remembered -- Count$RememberedSize's read -- grows by
// exactly the removed count). A player entry rides the PlayerRef encoding
// rememberedFrom decodes. The ctx-level list is deliberately NOT touched:
// the only consumer measured for this rider reads the persistent list, and a
// ctx append would widen a chained Defined$ Remembered reader for free.
func rememberRemoved(h Host, c *Ctx, sa *cards.SA, id state.ObjID, removed int32) {
	if !strings.EqualFold(strings.TrimSpace(sa.Params["RememberRemoved"]), "True") || c.Source == 0 {
		return
	}
	ids := make([]state.ObjID, 0, removed)
	for i := int32(0); i < removed; i++ {
		ids = append(ids, id)
	}
	h.Emit(events.Event{Kind: events.Choose, Obj: c.Source, Counter: "remembered", IDs: ids})
}

// effProliferate implements Forge's Proliferate (CR 701.27): the resolving
// player chooses any number of permanents and/or players that already have at
// least one counter (of any kind), and gives each chosen recipient another
// counter of EACH kind already there. A recipient carrying no counter of any
// kind is not choosable (CR 701.27a's "that have a counter on them"), and
// chooses nothing when no such recipient exists -- silently, with no ask and
// no Note (the OnlyEmptyAnswer discipline: a Max-0 KChoose is never posted).
//
// The eligible population is the battlefield in seat order (g.AliveFrom, then
// g.Zone per seat -- both deterministic slices, never a map walk) followed by
// the alive players with a counter. The chooser is the resolving controller
// (every corpus carrier proliferates for its own controller; Forge's
// ProliferateEffect takes its chooser from the ability's controller).
//
// The ask is the any-number convention effDiscard's Optional$ shape uses, NOT
// the strict-supersets gate putCounterChoose follows: Min 0 and Max the
// eligible count is a REAL choice the moment one eligible recipient exists
// ("{} vs {that one}" is a genuine election), so `len(eligible) < 2` does not
// suppress it. Only zero eligible is a no-choice shape.
//
// Amount$ is the number of times to proliferate, resolved through Num (a
// literal, an SVar or an inline Count$ expression). A value Num cannot resolve
// (the corpus's X-on-a-non-cast and `Number$2/Minus.Y` bodies) loud-degrades
// with one Note and proliferates nothing -- the documented fail-closed
// direction, never a silent 1. Because proliferation adds +1 of each EXISTING
// kind, N proliferations over one fixed recipient set equal one batch of +N;
// this build poses ONE ask and applies +Amount per recipient (deterministic
// and documented) rather than N sequential asks.
//
// The answer re-enters through ResumeKind "proliferate" with Ctx.Proliferate,
// which carries BOTH shapes -- an object recipient (Obj) and a player
// recipient (Player + IsPlayer) -- so the shared "counter_pick" arm, which
// reads Obj only, is deliberately not reused. RememberPut$ True (Ripples of
// Potential) remembers exactly the recipients that took a counter.
func effProliferate(h Host, c *Ctx, sa *cards.SA) {
	g := h.Game()
	// fx42 scoping: capture and clear the answered pick BEFORE the walk, so a
	// nested Proliferate below this one poses its own ask instead of
	// inheriting the outer answer (the BlightPicks discipline).
	picks := c.Proliferate
	done := c.ProliferateDone
	c.Proliferate, c.ProliferateDone = nil, false

	// Loud-fail-closed on any parameter outside the whitelist (the effBlight
	// case-whitelist shape): Amount$/RememberPut$ read here, Defined$/
	// ValidTgts$ are inert on every corpus carrier, Cost$/SorcerySpeed$/
	// Planeswalker$ are activation metadata the offer machinery already reads,
	// the Condition* family is consumed by the shared SVar-condition gate in
	// Resolve, SubAbility$ is chained by the ordinary walk, and the
	// description keys are display-only. One Note names the first unknown key
	// in sorted order (a map range must never reach an event unsorted) and the
	// whole body no-ops -- so an unmodelled shape degrades loudly rather than
	// silently guessing.
	var unknown []string
	for k := range sa.Params {
		switch k {
		case "Amount", "RememberPut", "Defined", "ValidTgts",
			"Cost", "SorcerySpeed", "Planeswalker",
			"ConditionCheckSVar", "ConditionSVarCompare",
			"ConditionDefined", "ConditionPresent", "ConditionCompare",
			"SubAbility", "SpellDescription", "StackDescription",
			"TriggerDescription", "Description", "PrecostDesc", "CostDesc",
			"ActivationZone", "AILogic", "AIPreference", "DeckHas", "DeckHints":
		default:
			unknown = append(unknown, k)
		}
	}
	if len(unknown) > 0 {
		sort.Strings(unknown)
		h.Emit(events.Event{Kind: events.Note, Obj: c.Source,
			Text: "Proliferate unmodelled parameter " + unknown[0]})
		return
	}

	n, ok := NumResolved(h, c, sa, "Amount", 1)
	if !ok {
		// Num resolved an absent Amount$ to its default 1; a value that is
		// PRESENT but unresolvable is the loud-degrade shape. NumResolved
		// reports ok=false for both, so distinguish by presence.
		if _, present := sa.Params["Amount"]; present {
			h.Emit(events.Event{Kind: events.Note, Obj: c.Source,
				Text: "Proliferate Amount$ unresolvable (" + sa.Params["Amount"] + ")"})
			return
		}
		n = 1
	}
	// A bare `Amount$ X` resolves through the cast's own X, but an activated
	// ability (Karn's Bastion-style `Cost$ 1 T | Amount$ X`) has no X to
	// resolve -- Num returns the zero c.X, which is not a legitimate "zero
	// times" but an unannounced count. Loud-degrade it (the same fail-closed
	// direction as the unresolvable bodies) rather than silently proliferating
	// nothing: the caller can tell an announced X (nonzero) from none.
	if strings.TrimSpace(sa.Params["Amount"]) == "X" && n <= 0 {
		h.Emit(events.Event{Kind: events.Note, Obj: c.Source,
			Text: "Proliferate Amount$ X unresolvable (no X announced)"})
		return
	}
	if n <= 0 {
		return
	}

	if done {
		applyProliferate(h, c, sa, picks, n)
		return
	}

	// The eligible set: battlefield recipients in seat/zone order, then alive
	// players with a counter. Both walks are deterministic slices.
	var eligible []state.Target
	for _, p := range g.AliveFrom(0) {
		for _, id := range g.Zone(state.ZBattlefield, p) {
			if o := g.Obj(id); o != nil && hasCounters(o.Counters) {
				eligible = append(eligible, state.Target{Obj: id})
			}
		}
	}
	for _, p := range g.AliveFrom(0) {
		if hasCounters(g.Players[p].Counters) {
			eligible = append(eligible, state.Target{Player: p, IsPlayer: true})
		}
	}
	if len(eligible) == 0 {
		// Nothing carries a counter: no choice exists, so no ask (and no
		// Note -- this is the correct resolution, not a degradation).
		return
	}

	chooser := c.Controller
	d := &decision.Decision{Player: chooser, Kind: decision.KChoose,
		Min:        0,
		Max:        len(eligible),
		Source:     c.Source,
		ResumeKind: "proliferate",
		ResumeSA:   sa,
		Prompt:     "Proliferate: choose any number of permanents and/or players"}
	for _, t := range eligible {
		label := "a player"
		if t.IsPlayer {
			if t.Player >= 0 && int(t.Player) < len(g.Players) {
				label = g.Players[t.Player].Name
			}
			d.Options = append(d.Options, decision.Option{Index: len(d.Options),
				Kind: "proliferate", Label: label, Obj: 0, Player: t.Player})
			continue
		}
		o := g.Obj(t.Obj)
		if o != nil && o.Face() != nil {
			label = o.Face().Name
		}
		owner := chooser
		if o != nil {
			owner = o.Controller
		}
		d.Options = append(d.Options, decision.Option{Index: len(d.Options),
			Kind: "proliferate", Label: label, Obj: t.Obj, Player: owner})
	}
	if Ask(h, d) == AskAsked {
		return // resolution suspended; the answer re-enters with Ctx.Proliferate set.
	}
	// The no-host (R-9) and empty-answer stand-in takes ALL eligible, the
	// exact mirror of botpolicy's "proliferate" arm, so a bot-answered ask
	// emits the same events the silent build would.
	h.Emit(events.Event{Kind: events.Note, Obj: c.Source, Player: chooser,
		Text: "proliferate resolved without a choice (no engine host to ask)"})
	applyProliferate(h, c, sa, eligible, n)
}

// applyProliferate gives each live chosen recipient +n of each kind of
// counter it already carries, one event per kind (objects -> CounterChange,
// players -> PlayerCounterChange), then remembers the recipients when the SA
// carries RememberPut$ True. A recipient that left the battlefield (or died)
// while the decision was outstanding takes nothing -- the putCounterPickApply
// zone-guard stance.
func applyProliferate(h Host, c *Ctx, sa *cards.SA, picks []state.Target, n int32) {
	g := h.Game()
	remember := strings.EqualFold(strings.TrimSpace(sa.Params["RememberPut"]), "True")
	var placed []state.Target
	for _, t := range picks {
		if t.IsPlayer {
			p := t.Player
			if int(p) < 0 || int(p) >= len(g.Players) || g.Players[p].Lost {
				continue
			}
			pl := &g.Players[p]
			kinds := counterKinds("", len(pl.Counters), func(i int) string { return pl.Counters[i].Kind }, func(i int) int32 { return pl.Counters[i].N })
			for _, k := range kinds {
				h.Emit(events.Event{Kind: events.PlayerCounterChange, Player: p,
					Counter: k, Amount: n})
			}
			placed = append(placed, state.Target{Player: p, IsPlayer: true})
			continue
		}
		o := g.Obj(t.Obj)
		if o == nil || o.Zone != state.ZBattlefield {
			continue
		}
		kinds := counterKinds("", len(o.Counters), func(i int) string { return o.Counters[i].Kind }, func(i int) int32 { return o.Counters[i].N })
		for _, k := range kinds {
			h.Emit(events.Event{Kind: events.CounterChange, Obj: o.ID, Counter: k, Amount: n})
		}
		placed = append(placed, state.Target{Obj: o.ID})
	}
	if remember && len(placed) > 0 {
		c.Remembered = append(c.Remembered, placed...)
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
