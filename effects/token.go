package effects

import (
	"strings"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

func init() { Register("Token", effToken) }

// effToken creates TokenAmount$ tokens of each TokenScript$ (a comma-
// separated list of Game.Tokens stems) for TokenOwner$ (the controller by
// default; only "Opponent" is resolved specially, matching Defined's own
// "You"/"Opponent" pair in context.go). Every other TokenOwner$ form the
// corpus uses (a fidelity gap this task does not close) still falls back to
// the controller rather than doing nothing, but now says so: a Note names
// the unrecognised value, so the gap is visible rather than silently
// papered over the way an unqualified fallback would be.
//
// Every token is its own TokenCreate event, in the order this loop visits
// them (outer: TokenScript$ stems left to right; inner: TokenAmount$ copies
// in index order) -- never a map-range order, so replay reproduces the same
// objects with the same IDs. events.Apply's TokenCreate case is what
// actually mints the object from Game.Tokens; this function only ever
// proposes the event through h.Emit, so it stays a pure proposer like every
// other primitive in this package (Ruling: effects never writes state.Game
// directly, only events.Apply does).
//
// An unknown TokenScript$ key is a Note ("unknown token script <key>") and
// creates nothing -- a Link-time diagnostic ought to have already caught a
// script that names a stem outside Game.Tokens, but Resolve's own totality
// stance (never panic on untrusted/unexpected input) applies here too.
//
// TokenTapped$ True makes every token this call creates enter tapped: the
// ordinary Tap event with the "entered tapped" text the ChangeZone paths
// emit for their own Tapped$ (an entry state, not the CR 701.21a event of
// becoming tapped) lands right after each mint, so the token is on the
// battlefield untapped for exactly one folded event and then tapped.
// TokenAttacking$ True (Mobilize, Kari Zev) makes every token this call
// creates enter tapped and attacking the combat's defending player through
// the appended events.TokenAttacks kind; see the implementation comment at
// the read below for the no-defender degrade. The other TokenAttacking$
// selector forms stay census-free gaps.
//
// TokenPower$/TokenToughness$ set the token's P/T from a dynamic value
// (Skyclave Apparition's X/X Illusion, SVar:X:Remembered$CardManaCost): the
// value resolves through the ordinary Num grammar (a signed literal, an SVar
// body, an inline Count$...), and the set is a PERMANENT layer-7b SubSet
// continuous effect on the token itself -- the same registration shape
// Amass uses for its type grant, so it is replay-rebuilt by re-execution
// and dies with the token's battlefield presence. A side the script does
// not name keeps the token script's printed value; a named side the Num
// grammar cannot resolve is a loud Note and the whole set is skipped (the
// token keeps its printed P/T, which for the corpus's */* scripts means a
// 0/0 the zero-toughness SBA sweeps -- the honest degrade, never a silent
// wrong value).
//
// RememberTokens$ True hands every object this call actually created to the
// rest of the chain via Ctx.Remembered, which is how Living Weapon's own
// keyword expansion (cards/keywords.go) attaches the Germ it just made: its
// SubAbility is `DB$ Attach | Defined$ Remembered`, and Resolve walks Sub
// with the SAME *Ctx, so appending here is what that Attach later reads.
func effToken(h Host, c *Ctx, sa *cards.SA) {
	g := h.Game()
	n := Num(h, c, sa, "TokenAmount", 1)
	owner := c.Controller
	switch v := sa.Params["TokenOwner"]; v {
	case "", "You":
		// The default: the controller, already set above.
	case "Opponent":
		for _, p := range g.AliveFrom(c.Controller) {
			if p != c.Controller {
				owner = p
				break
			}
		}
	case "RememberedOwner":
		// The owner of the first remembered OBJECT (Skyclave Apparition's
		// "the exiled card's owner creates the token"). The same group the
		// Remembered$ SVar head reads -- the source card's shared list first,
		// the firing trigger's own referent capture excluded -- so the
		// exiled card, not the leaving host, is the first candidate. No
		// remembered object (the ability's own ConditionPresent$ gate should
		// have kept this call from running at all) keeps the controller.
		for _, t := range rememberedWithSource(h, c) {
			if !t.IsPlayer {
				if o := g.Obj(t.Obj); o != nil {
					owner = o.Owner
					break
				}
			}
		}
	default:
		h.Emit(events.Event{Kind: events.Note, Obj: c.Source,
			Text: "unrecognized TokenOwner " + v + ", defaulting to the controller"})
	}
	remember := sa.Params["RememberTokens"] == "True"
	// AttachedTo$ names the permanent the token enters attached to (the Wicked
	// Role of Charming Scoundrel's ETB, 50+ corpus lines): the value is a
	// Defined$-grammar selector, resolved with the ordinary resolver against
	// a shallow SA that carries it in Defined$, so every corpus spelling
	// (Targeted, Self, Remembered, ChosenCard, ...) works without a second
	// resolver. Each minted token is attached to the FIRST resolved object
	// target; an object that has left play by resolution time attaches
	// nothing (the token simply enters unattached, the Aura's unattached
	// state).
	attachedTo := strings.TrimSpace(sa.Params["AttachedTo"])
	var attachTo state.ObjID
	if attachedTo != "" {
		sub := *sa
		sub.Params = map[string]string{"Defined": attachedTo}
		for _, t := range Defined(h, c, &sub) {
			if !t.IsPlayer {
				attachTo = t.Obj
				break
			}
		}
	}
	// TokenPower$/TokenToughness$: resolve both dynamic sides once, before
	// the mint loop. Absent side = the token script's printed value, read off
	// each minted object's face below. A named-but-unresolvable side is loud
	// (one Note for the whole call) and skips the set entirely.
	var setPow, setTgh int32
	var hasPow, hasTgh bool
	dynBad := ""
	if raw, ok := sa.Params["TokenPower"]; ok {
		if v, resolved := NumResolved(h, c, sa, "TokenPower", 0); resolved {
			setPow, hasPow = v, true
		} else {
			dynBad = "TokenPower$ " + raw
		}
	}
	if raw, ok := sa.Params["TokenToughness"]; ok {
		if v, resolved := NumResolved(h, c, sa, "TokenToughness", 0); resolved {
			setTgh, hasTgh = v, true
		} else {
			if dynBad != "" {
				dynBad += ", "
			}
			dynBad += "TokenToughness$ " + raw
		}
	}
	if dynBad != "" {
		h.Emit(events.Event{Kind: events.Note, Obj: c.Source, Player: c.Controller,
			Text: dynBad + " is not implemented; the token keeps its script's printed P/T"})
	}
	tapped := strings.EqualFold(strings.TrimSpace(sa.Params["TokenTapped"]), "True")

	// TokenAttacking$ True (Mobilize, Kari Zev's "tapped and attacking"
	// rider): every token this call creates enters attacking the combat's
	// DEFENDING player, read from the firing Attacks trigger's own referent
	// capture (rules/trigger_referents.go binds c.DefendingPlayer from the
	// DeclareAttackers event; the defending player is ev.Player there --
	// the engine batches attackers per defender). Only the literal True
	// form is implemented: the corpus's other selector values (Remembered
	// x5, RememberedPlayer x3, TriggeredAttackedTarget x4, TriggeredDefender
	// x1) keep the census-free degrade they had, now named by ONE loud Note
	// per call instead of silence. A True with NO defender in context (an
	// ACTIVATED AB$ Token rider like kavaron_harrier or militias_pride -- no
	// trigger context exists) still enters (tapped, when TokenTapped$ says
	// so) but NOT attacking, under one deterministic Note: never a guessed
	// defender. The mark itself rides the appended events.TokenAttacks kind
	// (events/apply.go), so replay rebuilds it.
	attackCtx := false
	var attackDefender state.PlayerID
	if attack := strings.TrimSpace(sa.Params["TokenAttacking"]); attack != "" {
		switch {
		case strings.EqualFold(attack, "True") && c.DefendingPlayer.IsPlayer:
			attackCtx = true
			attackDefender = c.DefendingPlayer.Player
		case strings.EqualFold(attack, "True"):
			h.Emit(events.Event{Kind: events.Note, Obj: c.Source, Player: c.Controller,
				Text: "TokenAttacking$ with no defending player in context; the token enters but does not attack"})
		default:
			h.Emit(events.Event{Kind: events.Note, Obj: c.Source, Player: c.Controller,
				Text: "TokenAttacking$ " + attack + " is not implemented; the token enters but does not attack"})
		}
	}

	for _, key := range strings.Split(sa.Params["TokenScript"], ",") {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		if _, ok := g.Tokens[key]; !ok {
			h.Emit(events.Event{Kind: events.Note, Obj: c.Source, Text: "unknown token script " + key})
			continue
		}
		for i := int32(0); i < n; i++ {
			// want is the ID the new object will get if TokenCreate's own
			// Apply case actually mints one (state.Game.AddObject assigns
			// NextID, then increments it) -- a direct, positive identity
			// check, rather than inferring a mint happened from g.Objs
			// having grown by watching its length before and after.
			want := g.NextID
			h.Emit(events.Event{Kind: events.TokenCreate, Player: owner, Text: key})
			if remember && g.Obj(want) != nil {
				c.Remembered = append(c.Remembered, state.Target{Obj: want})
				eventRemember(h, c, want)
			}
			if tapped && g.Obj(want) != nil {
				h.Emit(events.Event{Kind: events.Tap, Obj: want, Player: owner, Text: "entered tapped"})
			}
			if attackCtx && g.Obj(want) != nil {
				h.Emit(events.Event{Kind: events.TokenAttacks, Obj: want, Player: owner,
					IDs: []state.ObjID{state.ObjID(attackDefender)}, Text: "entered attacking"})
			}
			if (hasPow || hasTgh) && g.Obj(want) != nil && g.Obj(want).Face() != nil {
				// The absent side keeps the token script's printed value. Every
				// corpus script a dynamic side rides (u_x_x_illusion, ...) is a
				// characteristic-defining */* whose printed read is 0, so both
				// sides are effectively always named together.
				pow, tgh := int32(g.Obj(want).Face().Power()), int32(g.Obj(want).Face().Toughness())
				if hasPow {
					pow = setPow
				}
				if hasTgh {
					tgh = setTgh
				}
				h.AddContinuous(state.ContinuousEffect{
					Source:       want,
					Controller:   owner,
					Affects:      "Card.Self",
					Layer:        state.LPT,
					Sub:          state.SubSet,
					SetPower:     pow,
					SetToughness: tgh,
					HasSet:       true,
					Permanent:    true,
				})
			}
			if attachTo != 0 && g.Obj(want) != nil && g.Obj(attachTo) != nil {
				h.Emit(events.Event{Kind: events.Attach, Obj: want, IDs: []state.ObjID{attachTo}})
			}
			if strings.EqualFold(strings.TrimSpace(sa.Params["ImprintTokens"]), "True") && g.Obj(want) != nil {
				// ImprintTokens$ True (Ugin, the Ineffable's [+1] spirit token):
				// the created token is IMPRINTED with the cards the resolution
				// remembered -- the face-down-exiled card the preceding Dig
				// captured -- so Card.IsImprinted matches the token exactly as
				// Forge's imprintedCards association would. An empty remembered
				// set records nothing: an imprint of nothing is not an imprint.
				ids := make([]state.ObjID, 0, len(c.Remembered))
				for _, t := range c.Remembered {
					if !t.IsPlayer && t.Obj != 0 {
						ids = append(ids, t.Obj)
					}
				}
				if len(ids) > 0 {
					h.Emit(events.Event{Kind: events.Imprint, Obj: want, IDs: ids})
				}
			}
		}
	}
}
