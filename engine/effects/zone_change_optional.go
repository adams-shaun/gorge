package effects

import (
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// changeZoneOptionalConfirm is the known-origin ChangeZone's Optional$
// confirmation (CR 603.5, 608.2d; Forge's changeKnownOriginResolve
// `if (optional && !chooser.confirmAction(...)) continue`). Each object the
// move loop would move is offered to the chooser -- Chooser$'s first player,
// else the resolving controller -- as a yes/no; a decline drops that object
// only and the chained SubAbility$ still runs (Resolve owns the chain). With no
// answer (no host) "may" plays as "do" (R-9), the hand walker's fallback, so a
// no-host replay is unchanged. The hand, hidden-pick and search walkers pose
// their own confirmations before reaching the object path.
func changeZoneOptionalConfirm(h Host, c *Ctx, cz *ChangeZoneParams, targets []state.Target, to state.Zone, originAll bool) []state.Target {
	chooser := c.Controller
	if cz.Chooser != "" {
		if p, ok := chooserPlayer(h, c, cz.Chooser); ok {
			chooser = p
		}
	}
	kept := make([]state.Target, 0, len(targets))
	for _, t := range targets {
		o := h.Game().Obj(t.Obj)
		if t.IsPlayer || o == nil || changeZoneMoveSkip(h, cz, o, to, originAll) || sourceReturnedSincePush(h, c, o) {
			kept = append(kept, t)
			continue
		}
		prompt := cz.OptionalPrompt
		if prompt == "" {
			prompt = "Move " + objName(h.Game(), o.ID) + " to " + to.String() + "?"
		}
		d := &decision.Decision{Player: chooser, Kind: decision.KChoose, Min: 1, Max: 1,
			Source: c.Source, ResumeKind: "changezone_optional", ResumeTarget: int(o.ID),
			Prompt: prompt,
			Options: []decision.Option{
				{Index: 0, Kind: "yes", Label: "Yes", Player: chooser},
				{Index: 1, Kind: "no", Label: "No", Player: chooser},
			}}
		if ans, ok := AskTape(h, d); ok && !tapeAnswerYes(ans) {
			continue
		}
		kept = append(kept, t)
	}
	return kept
}
