package effects

import (
	"strings"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

func init() {
	Register("Tap", effTap)
	Register("Pump", effPump)
	Register("PumpAll", effPumpAll)
	Register("Animate", effAnimate)
	Register("Protection", effProtection)
}

func effTap(h Host, c *Ctx, sa *cards.SA) {
	for _, t := range Defined(h, c, sa) {
		if t.IsPlayer {
			continue
		}
		o := h.Game().Obj(t.Obj)
		if o == nil || o.Zone != state.ZBattlefield || o.Tapped {
			continue
		}
		h.Emit(events.Event{Kind: events.Tap, Obj: o.ID})
	}
}

// effPump, effPumpAll, effAnimate and effProtection are all continuous
// effects: they build a state.ContinuousEffect and hand it to the Host,
// which is rules.Engine.AddContinuous, so the CR 613 layer system Task 19
// built actually has a caller (Task 19c). Every one of them is a temporary
// grant from a resolving spell or ability, never a permanent's own printed
// static, so they always set UntilEOT: true -- the layer system's existing
// EndOfTurnCleanup already drops those on schedule, and Engine.active()
// never checks the source's battlefield presence for an UntilEOT effect
// (Giant Growth outlives the instant that cast it), so nothing else needs
// to change for these to expire correctly.
//
// Each one scopes its effect to exactly the object it targets with
// Affects: "Card.Self" and Source: <that object's ID> -- not the resolving
// ability's own source -- reusing the same Self-predicate pattern Task 19's
// own lord-effect tests already established (layers_test.go), rather than
// inventing a new filter form.
func effPump(h Host, c *Ctx, sa *cards.SA) {
	att := Num(h, c, sa, "NumAtt", 0)
	def := Num(h, c, sa, "NumDef", 0)
	kws := splitKeywords(sa.Params["KW"])
	for _, t := range Defined(h, c, sa) {
		if t.IsPlayer {
			continue
		}
		o := h.Game().Obj(t.Obj)
		if o == nil || o.Zone != state.ZBattlefield {
			continue
		}
		registerPumpEffects(h, c, o.ID, att, def, kws)
	}
}

// effPumpAll bakes in the affected set at resolution time (CR 611.2c: such an
// effect applies only to the permanents matching the filter when the ability
// resolves, not to ones that start matching later), so it registers one
// continuous effect per matching object -- found by walking AliveFrom(0)'s
// fixed APNAP seat order and each seat's battlefield zone slice in its
// existing registration order, never a map -- rather than one shared
// filter-based effect that Derived would re-evaluate against the battlefield
// forever.
func effPumpAll(h Host, c *Ctx, sa *cards.SA) {
	att := Num(h, c, sa, "NumAtt", 0)
	def := Num(h, c, sa, "NumDef", 0)
	kws := splitKeywords(sa.Params["KW"])
	spec := sa.Params["ValidCards"]
	if spec == "" {
		spec = "Creature"
	}
	g := h.Game()
	for _, p := range g.AliveFrom(0) {
		for _, id := range g.Zone(state.ZBattlefield, p) {
			if MatchesSpecFrom(g, spec, id, c.Controller, c.Source) {
				registerPumpEffects(h, c, id, att, def, kws)
			}
		}
	}
}

// registerPumpEffects is Pump and PumpAll's shared per-object registration:
// a layer-7c modification for a nonzero stat change, and a separate layer-6
// grant for any keywords, since Derived applies each layer independently.
// Skipping a zero/empty half avoids polluting Engine.continuous with an
// effect that would never do anything (a keyword-only Pump has no stat
// change to register, and vice versa).
func registerPumpEffects(h Host, c *Ctx, id state.ObjID, att, def int32, kws []string) {
	if att != 0 || def != 0 {
		h.AddContinuous(state.ContinuousEffect{
			Source: id, Affects: "Card.Self", Controller: c.Controller,
			Layer: state.LPT, Sub: state.SubModify,
			AddPower: att, AddToughness: def, UntilEOT: true,
		})
	}
	if len(kws) > 0 {
		h.AddContinuous(state.ContinuousEffect{
			Source: id, Affects: "Card.Self", Controller: c.Controller,
			Layer: state.LAbilities, AddKeywords: kws, UntilEOT: true,
		})
	}
}

// splitKeywords parses a KW$ parameter's "&"-joined keyword list -- the same
// separator Forge's own AddKeyword$ (an S:Mode$ Continuous static's
// parameter, e.g. Sword of Fire and Ice's "Protection from red & Protection
// from blue") uses for the same purpose. A single keyword has no "&" and
// comes back as a one-element slice.
func splitKeywords(kw string) []string {
	kw = strings.TrimSpace(kw)
	if kw == "" {
		return nil
	}
	parts := strings.Split(kw, "&")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// effAnimate does not require the target to already be on the battlefield --
// Forge's own Animate targets a creature card in a graveyard as often as a
// permanent -- so unlike Pump it has no zone guard beyond "the object still
// exists". Setting base P/T is layer 7b (SubSet), which is why it uses
// SetPower/SetToughness/HasSet rather than Pump's AddPower/AddToughness: a
// later set must overwrite an earlier one regardless of timestamp order
// (TestSetBeforeModifyRegardlessOfTimestamp already locks that in), where an
// Add would incorrectly stack.
//
// The P/T effect is only registered when Power$ or Toughness$ is actually
// present: roughly two thirds of the corpus's own DB$ Animate calls (e.g.
// Kitesail Larcenist, Kami of Industry) use it purely to grant a type or
// keyword change and never mention Power$/Toughness$ at all. Num's zero
// default would otherwise turn every one of those into "becomes a 0/0",
// silently killing the very creature the card was granting an ability to.
func effAnimate(h Host, c *Ctx, sa *cards.SA) {
	_, hasPower := sa.Params["Power"]
	_, hasToughness := sa.Params["Toughness"]
	pw := Num(h, c, sa, "Power", 0)
	tf := Num(h, c, sa, "Toughness", 0)
	types := strings.Fields(sa.Params["Types"])
	for _, t := range Defined(h, c, sa) {
		if t.IsPlayer {
			continue
		}
		o := h.Game().Obj(t.Obj)
		if o == nil {
			continue
		}
		if hasPower || hasToughness {
			h.AddContinuous(state.ContinuousEffect{
				Source: o.ID, Affects: "Card.Self", Controller: c.Controller,
				Layer: state.LPT, Sub: state.SubSet,
				SetPower: pw, SetToughness: tf, HasSet: true, UntilEOT: true,
			})
		}
		if len(types) > 0 {
			h.AddContinuous(state.ContinuousEffect{
				Source: o.ID, Affects: "Card.Self", Controller: c.Controller,
				Layer: state.LType, AddTypes: types, UntilEOT: true,
			})
		}
	}
}

func effProtection(h Host, c *Ctx, sa *cards.SA) {
	gains := resolveGains(sa.Params["Gains"], sa.Params["Choices"])
	if gains == "" {
		return
	}
	for _, t := range Defined(h, c, sa) {
		if t.IsPlayer {
			continue
		}
		o := h.Game().Obj(t.Obj)
		if o == nil || o.Zone != state.ZBattlefield {
			continue
		}
		h.AddContinuous(state.ContinuousEffect{
			Source: o.ID, Affects: "Card.Self", Controller: c.Controller,
			Layer: state.LAbilities, AddKeywords: []string{"Protection from " + gains},
			UntilEOT: true,
		})
	}
}

// resolveGains turns Protection's Gains$ parameter into a concrete quality
// string ("red", "artifacts", ...). Gains$ Choice (Mother of Runes: "Gains$
// Choice | Choices$ AnyColor") names no chooser this build has -- a real
// player choice is Task 20's job, the same simplification effCharm and
// effVote already apply to Choices$ elsewhere in this package -- so it
// resolves deterministically instead of asking: AnyColor (the only Choices$
// value this corpus uses here) becomes white, the fixed first-of-WUBRG
// default; anything else takes the first comma-separated entry, matching
// effCharm/effVote's own "first choice" convention.
func resolveGains(gains, choices string) string {
	if !strings.EqualFold(gains, "Choice") {
		return gains
	}
	if strings.EqualFold(choices, "AnyColor") {
		return "white"
	}
	return strings.TrimSpace(strings.SplitN(choices, ",", 2)[0])
}
