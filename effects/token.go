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
//
// RememberOriginalTokens$ True (Forum Filibuster, Diregraf Horde, Dain
// Ironfoot and 5 more corpus carriers, all `DB$ Token` shapes) takes the SAME
// branch. Forge's own distinction is original-token-vs-post-replacement
// mint, and in this build that distinction collapses in favour of the flag:
// every mint this call proposes IS an original token, because the per-emitted
// event `want` capture runs before any token replacement could rewrite it and
// replacement EXTRA mints get no riders at all (the tokrepl1 contract --
// Academy Manufactor remembers only its first mint). One documented
// divergence stays: under a Type$ ReplaceToken rewrite (Divine Visitation)
// `g.Obj(want)` is the REPLACED mint, so the remembered object is the
// replaced token, not the token the script named. All 8 carriers are plain
// `DB$ Token` lines with no `R:` replacement in reach (measured at the
// corpus pin), so the divergence is corpus-unreachable today.
func effToken(h Host, c *Ctx, sa *cards.SA) {
	g := h.Game()
	n := Num(h, c, sa, "TokenAmount", 1)
	// owners is the per-mint owner list: one entry for every shape the
	// switch resolves, so the mint loop below can give EACH owner its own
	// tokens when a spelling names several (trig:Vote's
	// TokenOwner$ TriggeredOpponentVotedSame -- "each opponent who voted
	// ... creates a Treasure"). Every pre-existing shape resolves exactly
	// one owner, so the loop is byte-identical for them.
	owners := []state.PlayerID{c.Controller}
	switch v := sa.Params["TokenOwner"]; v {
	case "", "You":
		// The default: the controller, already set above.
	case "Opponent":
		for _, p := range g.AliveFrom(c.Controller) {
			if p != c.Controller {
				owners = []state.PlayerID{p}
				break
			}
		}
	case "TriggeredOpponentVotedSame", "TriggeredOpponentVotedDiff":
		// The vote-carrier referent (trig:Vote): each player in the List$
		// set the firing trigger captured creates its own token. An EMPTY
		// set creates nothing -- "each opponent who voted ..." is vacuous
		// when nobody did, never a token for the controller (the old
		// unrecognised-owner fallback would have minted a wrong one and
		// noted).
		ps := c.TriggeredOpponentsVotedSame
		if v == "TriggeredOpponentVotedDiff" {
			ps = c.TriggeredOpponentsVotedDiff
		}
		owners = make([]state.PlayerID, 0, len(ps))
		owners = append(owners, ps...)
	case "Imprinted", "ImprintedController":
		// Forge's TokenOwner$ ImprintedController: the controller of the
		// RepeatEach iteration's current imprinted subject, and only that
		// (UseImprinted$ binds the subject). The ordinary Defined resolver owns
		// the selector, including the last-known controller a ChangeZone's
		// RememberLKI$ captured -- Curse of the Swine's Boar per exiled
		// creature. A subject whose controller cannot be resolved leaves the
		// controller default, the same silent degrade the other miss cases take.
		for _, t := range Defined(h, c, &cards.SA{Params: map[string]string{"Defined": v}}) {
			if t.IsPlayer {
				owners = []state.PlayerID{t.Player}
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
					owners = []state.PlayerID{o.Owner}
					break
				}
			}
		}
	case "ThisTargetedPlayer":
		// The player one of the charm's modes targeted (Shadrix Silverquill,
		// the duo cycle, verdant/ashlings/prismari command -- 7 corpus files
		// carry the spelling on a Token): the first player-kind entry of this
		// resolution's own target list. With the cross-mode TargetUnique split
		// (effCharm's charmCrossModeRun) that list is exactly the running
		// mode's own target, so the token is created BY the player the mode
		// targeted, not by the ability's controller. A resolution with no
		// player target keeps the controller, the same silent degrade the
		// other miss cases here take.
		for _, t := range c.Targets {
			if t.IsPlayer {
				owners = []state.PlayerID{t.Player}
				break
			}
		}
	default:
		h.Emit(events.Event{Kind: events.Note, Obj: c.Source,
			Text: "unrecognized TokenOwner " + v + ", defaulting to the controller"})
	}
	// RememberOriginalTokens$ True mirrors RememberTokens$ exactly (see the
	// doc above for the original-vs-replaced-mint note). The 8 carriers all
	// chain a `DB$ ImmediateTrigger` "when you do" sub that reads this set.
	remember := sa.Params["RememberTokens"] == "True" ||
		sa.Params["RememberOriginalTokens"] == "True"
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
	// WithCountersType$/WithCountersAmount$ (Printlifter Ooze's "create a
	// 0/0 ... token ... The token enters with X +1/+1 counters on it"): every
	// token this call creates enters with that many of the named counter
	// kind, emitted as ONE CounterChange per mint right after the mint -- the
	// ChangeZone entry counters' exact shape (zone.go's WithCounters read),
	// so AddCounter replacements (Doubling Season) and every CounterAdded
	// trigger see an entry counter the way they see a ChangeZone one. The
	// amount resolves through the ordinary Num grammar: a signed literal
	// (incubob's WithCountersAmount$ 1), an SVar name on the resolving face
	// (Printlifter Ooze's WithCountersAmount$ X over SVar:X:Count$Valid
	// Creature.YouCtrl), or an inline Count$... -- the same resolution the
	// TokenPower$/TokenToughness$ read above uses. A WithCountersType$ with
	// no WithCountersAmount$ defaults to 1; an amount the grammar cannot
	// resolve is loud (one Note for the whole call, never per mint) and the
	// set is skipped -- the token enters WITHOUT the counters, the honest
	// degrade that for a 0/0 script means the zero-toughness SBA sweeps it
	// visibly rather than a silent wrong count. The minted-object guard is
	// the same g.Obj(want) identity check the other riders read: under a
	// token replacement the counters land on the first mint only, the
	// tokrepl1 extra-mints-get-no-riders contract this file's
	// RememberTokens$ doc already records.
	withKind := strings.TrimSpace(sa.Params["WithCountersType"])
	var withAmt int32
	var withOK bool
	if withKind != "" {
		if _, present := sa.Params["WithCountersAmount"]; present {
			if v, ok := NumResolved(h, c, sa, "WithCountersAmount", 1); ok {
				withAmt, withOK = v, true
			} else {
				h.Emit(events.Event{Kind: events.Note, Obj: c.Source, Player: c.Controller,
					Text: "WithCountersAmount$ " + strings.TrimSpace(sa.Params["WithCountersAmount"]) +
						" is not implemented; the token enters with no " + withKind + " counters"})
			}
		} else {
			withAmt, withOK = 1, true
		}
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
	// The AtEOT$ rider's affected set is every mint the loop actually mints,
	// collected here and scheduled by ONE scheduleAtEOT call after the loop:
	// the call (and, for an out-of-scope value, its one loud Note) is per
	// resolution, never per mint -- a multi-token body with an out-of-scope
	// value must not emit one Note per token.
	var minted []state.ObjID
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
		for _, owner := range owners {
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
				if withOK && g.Obj(want) != nil {
					h.Emit(events.Event{Kind: events.CounterChange, Obj: want, Counter: withKind, Amount: withAmt})
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
				// AtEOT$ (Valduk, Zektar Shrine Expedition: "exile those tokens at
				// the beginning of the next end step"): remember the predicted mint
				// id (the CopyPermanent pattern); the shared reader schedules the
				// whole minted set in one call after the loop.
				minted = append(minted, want)
			}
		}
	}
	scheduleAtEOT(h, c, sa, minted)
}
