package effects

import (
	"strings"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/state"
)

// chosenTargetsFor serves ANY ValidTgts$-declared SA that resolves with no
// ask having covered its targeting: the generic pre-ask inside
// effects.Resolve's dispatch loop (task mvts1). The placement ask
// (rules/trigger_queue.go pushTrigger) and the announcement ask
// (rules/stack.go resolveTop's ability/spell branches) cover only the
// depth-1 body of a trigger or cast -- a deeper sub of the Execute chain
// (the "when you do" family: Mogg Bombers' DealDamage, Aria of Flame's
// verse ping, Kor Outfitter's Attach) used to arrive here with no chosen
// targets and either moved nothing silently or inherited the OUTER SA's
// targets. This poses the sub's own ask through the same Host.LegalTargets
// census the placement ask uses, as a KChoose over the "tgts" resume arm;
// the answer lands in Ctx.TargetsPick and the re-entered pass consumes it
// (the ask's ResumeSA is exactly this SA, so the pending frame re-enters
// it). A host that cannot ask takes the deterministic first-max stand-in
// (R-9), which is what botpolicy's first-option KChoose default answers
// with for card/player options.
//
// The ok return is NOT "targets were found" -- it is "use the returned set
// INSTEAD of Defined's own fallthrough": ok=true with a nil set means the
// ask was posed and SUSPENDED the resolution (effects.Resolve stops before
// dispatching the body), and the answered re-entry consumes
// Ctx.TargetsPick here. Every other shape returns false and the caller
// keeps Defined's own behaviour.
//
// The ask never fires when the SA also carries Defined$ (an already-named
// fetch list is Forge's no-ask shape), when it is an API$ ChangeZone body
// (effChangeZone's own mid-resolution ask, changeZoneChosenTargets, owns
// that shape -- the closed ChangeZone slice), when this is the depth-0
// entry SA of a resolution whose TargetsOffered marker is set (the
// placement ask covered exactly that SA; its targets are already in
// Ctx.Targets and a Min-0 elected-zero must not be re-posed), or when an
// answered pre-ask for this very SA is waiting in Ctx.TargetsPick. Bounds
// come from TargetMin$/TargetMax$ through the ordinary Num grammar,
// clamped to the eligible count; Min == Max == 0 or an empty eligible set
// is no ask and no move -- a decision nobody could answer differently is
// never emitted, and a ValidTgts$ spec that matches nobody (an unknown
// predicate fails closed) keeps today's empty-set no-op.
//
// Deliberately NOT honoured here (measured carrier population: one sub
// each, both noted in the mvts1 report): TargetsForEachPlayer$ and
// DividedAsYouChoose$ -- the ask offers the plain TargetMin$/TargetMax$
// bounds, exactly like the placement ask does for the same parameters.
func chosenTargetsFor(h Host, c *Ctx, sa *cards.SA, atRoot bool) ([]state.Target, bool) {
	if strings.TrimSpace(sa.Params["ValidTgts"]) == "" ||
		strings.TrimSpace(sa.Params["Defined"]) != "" {
		return nil, false
	}
	if sa.CompiledAPI() == cards.APIChangeZone || sa.API == "ChangeZone" {
		return nil, false
	}
	if c.TargetsPickDone {
		ans := c.TargetsPick
		c.TargetsPickDone, c.TargetsPick = false, nil
		return ans, true
	}
	if c.OfferedSA != nil && sa.Line == c.OfferedSA.Line {
		// The placement/announcement ask covered exactly THIS SA (matched by
		// Line: ResolveSVar parses fresh on every call, so pointer identity
		// never holds between two derivations of the same body -- the
		// matching convention rules' charmModeTarget established). Its
		// targets are already in Ctx.Targets and a Min-0 elected-zero must
		// not be re-posed.
		return nil, false
	}
	if atRoot && c.TargetsOffered {
		// Belt and braces for a depth-0 entry under an offered marker whose
		// OfferedSA derivation did not fire (e.g. a modal root whose own
		// ValidTgts$ was not the ask's subject).
		return nil, false
	}
	chooser := c.Controller
	candidates := h.LegalTargets(chooser, c.Source, sa)
	min := Num(h, c, sa, "TargetMin", 1)
	max := Num(h, c, sa, "TargetMax", 1)
	if max > int32(len(candidates)) {
		max = int32(len(candidates))
	}
	if min > max {
		min = max
	}
	if min < 0 {
		min = 0
	}
	if max <= 0 {
		// Nothing eligible (or an explicitly zero bound): no ask, no move.
		return nil, false
	}
	return poseTargetsAsk(h, c, sa, chooser, candidates, min, max, "tgts")
}

// poseTargetsAsk is the shared tail of both ValidTgts$ mid-resolution asks
// (this file's chosenTargetsFor and zone.go's changeZoneChosenTargets): a
// KChoose over the eligible candidates -- one option per target, players
// labelled from the player table, cards from the face name -- posted through
// the shared Ask boundary under the "choice"-shaped resume transport the
// caller names, with the R-9 no-host stand-in (the first max candidates in
// offered order) when the host cannot ask. ok=true with a nil set is the
// SUSPENDED outcome; ok=true with a non-nil set is the stand-in.
func poseTargetsAsk(h Host, c *Ctx, sa *cards.SA, chooser state.PlayerID,
	candidates []state.Target, min, max int32, resumeKind string,
) ([]state.Target, bool) {
	prompt := strings.TrimSpace(sa.Params["TgtPrompt"])
	if prompt == "" {
		prompt = "Choose target"
	}
	d := &decision.Decision{Player: chooser, Kind: decision.KChoose,
		Min: int(min), Max: int(max), Source: c.Source,
		ResumeKind: resumeKind, ResumeSA: sa,
		ResumeRemembered: copyTargets(c.Remembered), Prompt: prompt}
	for _, t := range candidates {
		o := decision.Option{Index: len(d.Options)}
		label := ""
		if t.IsPlayer {
			o.Kind, o.Player = "player", t.Player
			if p := h.Game(); int(t.Player) < len(p.Players) {
				label = p.Players[t.Player].Name
			}
		} else {
			o.Kind, o.Obj = "card", t.Obj
			if g := h.Game().Obj(t.Obj); g != nil && g.Face() != nil {
				label = g.Face().Name
			}
		}
		o.Label = label
		d.Options = append(d.Options, o)
	}
	if Ask(h, d) == AskAsked {
		return nil, true
	}
	return candidates[:max], true
}
