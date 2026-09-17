package events

import (
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/state"
)

// Emit is the engine's only mutation path: append to the log, then fold into
// state. Replay calls Apply directly with logged events, so post-replay state
// equals post-play state by construction.
func Emit(g *state.Game, l *Log, e Event) Event {
	stored := l.Append(e)
	Apply(g, stored)
	return stored
}

// Apply folds one event into state. It must stay a pure function of (g, e):
// no randomness, no clock, no reads outside g.
// resolveSVarAcrossFaces resolves an Execute$ SVar name against the source
// object's card, trying the ACTIVE face's table first and then every face in
// index order. A one-face card behaves exactly as before (the active face
// IS the first hit). The multi-face case is why this helper exists: an
// Enchantment Room's alternate-face trigger (an unlocked room's "When you
// unlock this door", CR 309.5) names an SVar that lives on Face[1]'s table,
// which src.Face() -- the active face -- does not carry. First face whose
// table defines the name wins: deterministic, and a name defined on several
// faces resolves to the lowest index consistently on live play and replay.
func resolveSVarAcrossFaces(src *state.Object, name string) *cards.SA {
	if name == "" {
		return nil
	}
	if f := src.Face(); f != nil {
		if sa := cards.ResolveSVar(f.SVars, name); sa != nil {
			return sa
		}
	}
	if src.Card == nil {
		return nil
	}
	for _, cf := range src.Card.Faces {
		if sa := cards.ResolveSVar(cf.SVars, name); sa != nil {
			return sa
		}
	}
	return nil
}

func Apply(g *state.Game, e Event) {
	switch e.Kind {
	case GameStart, DecisionAsk, DecisionMade, Note, Resolve, ModeChosen:
		// Markers. Resolve is deliberately inert: the resolving object leaves
		// the stack through its own MoveZone event, and popping here as well
		// would drop a second object. ModeChosen is a marker too: rules carries
		// its answer in a cast/trigger cache or suspended-resolution context, so
		// Apply writes nothing; the log lets replay re-derive the same branch.

	case Pair:
		// CR 702.103: a Soulbond pairing. Obj is the pairing permanent and
		// IDs[0] its chosen partner; both fields are set reciprocally when
		// both are battlefield permanents. Neither half is written when a
		// pairing ends (the paired field is reset by each object's own
		// Move when one leaves the battlefield).
		if len(e.IDs) > 0 {
			applyPair(g, e.Obj, e.IDs[0])
		}

	case MyriadCopy:
		// CR 702.109: a Myriad attacker token. Mint a copy of the source
		// attack-creature (same face/power/toughness, marked IsToken and
		// IsCopy) tapped and attacking the opponent named by Player. If the
		// source is gone the copy still enters (from its last-known
		// characteristics: we clone the object as it is now, which is the
		// best available LKI for a transient copy).
		//
		// This event only MINTS the object (in the untracked ZLibrary state
		// AddObject leaves it in); it deliberately does NOT move it onto the
		// battlefield. The caller (effects/myriad.go) follows this event with
		// a genuine MoveZone event for the mint's ID, so the token's entry is
		// an ordinary ChangesZone-matchable event and existing
		// enters-the-battlefield triggers (the token's own ETBs, and any
		// other permanent's "creature enters" trigger) see it exactly as they
		// would a cast or reanimated creature. A direct Move() here, as
		// TokenCreate/CardToken use, would leave the entry invisible to
		// zoneChangeMatches (Mode$ ChangesZone requires ev.Kind ==
		// events.MoveZone).
		src := g.Obj(e.Obj)
		if validPlayer(g, e.Player) && src != nil && src.Face() != nil {
			o := g.AddObject(src.Card, e.Player)
			o.FaceIdx = src.FaceIdx
			o.IsToken = true
			o.IsCopy = true
			o.IsMyriad = true
			o.Tapped = true
			o.IsAttacking = true
			if len(e.IDs) > 0 {
				o.Attacking = state.PlayerID(e.IDs[0])
			}
		}

	case MyriadCleanup:
		// CR 702.109a: every token created by Myriad is exiled at end of
		// combat. Move is called only from this event fold, so replay performs
		// the same deterministic arena-order cleanup without synthetic events.
		for i := range g.Objs {
			o := &g.Objs[i]
			if o.IsMyriad && o.Zone == state.ZBattlefield {
				Move(g, o.ID, state.ZBattlefield, state.ZExile)
			}
		}

	case Shuffle:
		if validPlayer(g, e.Player) {
			g.SetZone(state.ZLibrary, e.Player, append([]state.ObjID(nil), e.IDs...))
		}

	case MonarchChange:
		if validPlayer(g, e.Player) {
			g.Monarch, g.HasMonarch = e.Player, true
		}

	case StartingPlayerChange:
		if validPlayer(g, e.Player) {
			g.StartingPlayer, g.HasStartingPlayer = e.Player, true
		}

	case ControlChange:
		if validPlayer(g, e.Player) {
			if o := g.Obj(e.Obj); o != nil {
				changeControl(g, o, e.Player)
				// An AsLongAsControl goad ends the moment its controller
				// condition fails; pruning here keeps a later return of
				// control from reviving it.
				pruneGoads(g)
			}
		}

	case Imprint:
		if o := g.Obj(e.Obj); o != nil {
			if e.Text == "clear" {
				o.Imprinted = nil
			} else if e.Text == "forget" {
				// ForgetImprinted$ (Pump's Chrome Mox body): remove exactly the
				// named ids from the persistent Imprinted list, keeping the
				// rest -- a bad or partial payload degrades to a smaller
				// forget, never a wider one.
				drop := make(map[state.ObjID]bool, len(e.IDs))
				for _, id := range e.IDs {
					drop[id] = true
				}
				kept := make([]state.ObjID, 0, len(o.Imprinted))
				for _, id := range o.Imprinted {
					if !drop[id] {
						kept = append(kept, id)
					}
				}
				o.Imprinted = kept
			} else {
				// Text is an in-kind discriminator, not a new Event field:
				// ImprintCards$ records Forge's imprintedCards list while a
				// ChangeZone-to-exile records the separate exiledCards list.
				// Both associations survive replay, but only the latter is
				// pruned when its card leaves exile (in Move below).
				// Text "until-host-leaves" records ChangeZone's Duration$
				// UntilHostLeavesPlay association on the SOURCE object: the
				// exiled cards (IDs) come back to the zone carried in Amount
				// when the source leaves the battlefield (rules sweepExileReturn).
				if e.Text == "until-host-leaves" {
					from := state.Zone(e.Amount)
					if from.Valid() {
						for _, id := range e.IDs {
							if g.Obj(id) == nil {
								continue
							}
							dupe := false
							for _, en := range o.ExileReturn {
								if en.Obj == id {
									dupe = true
									break
								}
							}
							if !dupe {
								o.ExileReturn = append(o.ExileReturn, state.ExileReturnEntry{Obj: id, From: from})
							}
						}
					}
				} else {
					list := &o.Imprinted
					if e.Text == "exiled-with" {
						list = &o.ExiledCards
					}
					for _, id := range e.IDs {
						if g.Obj(id) != nil {
							*list = append(*list, id)
						}
					}
				}
			}
		}

	case LibraryOrder:
		// A library-arranging effect (Ponder, later Scry/Surveil) set a
		// complete new order on a player's library. Mechanically identical to
		// Shuffle's SetZone (see the same defensive copy below -- never alias
		// the event's slice into game state), but a separate Kind on purpose
		// (Ruling J1): Shuffle means "randomised, the old order is gone" to
		// a decoder, while LibraryOrder means "the player chose a new order"
		// -- a scry is not a shuffle, and a log reader must be able to tell
		// them apart. Same totality stance as every case here: an invalid
		// Player is a no-op, never a panic.
		if validPlayer(g, e.Player) {
			g.SetZone(state.ZLibrary, e.Player, append([]state.ObjID(nil), e.IDs...))
		}

	case ExtraTurn:
		// One grant or consumption of an extra turn (CR 500.7). The count and
		// the ordered pending queue are game state folded here so a log-only
		// reconstruction holds the same pending extras the live game did; the
		// turn structure's own consumption is the -1 form, emitted by rules'
		// advanceStep at the exact boundary it repeats the seat instead of
		// moving on. The queue is the ORDER the rule takes them in: grants
		// append in creation order, the -1 consumption removes the seat's LAST
		// entry (most recently created first, CR 500.7), so the total counts and
		// the queue agree by construction.
		if validPlayer(g, e.Player) && e.Amount != 0 {
			if g.ExtraTurns == nil {
				g.ExtraTurns = map[state.PlayerID]int{}
			}
			g.ExtraTurns[e.Player] += int(e.Amount)
			if g.ExtraTurns[e.Player] < 0 {
				g.ExtraTurns[e.Player] = 0
			}
			if e.Amount > 0 {
				// Amount is a number of distinct grants, not merely the
				// aggregate counter. Keep one queue entry per granted turn so
				// NumTurns$ 2 (Time Stretch) is consumed twice. Text is an
				// already encoded Event field; this canonical marker carries the
				// Forge SkipUntap$ rider without changing Event's hash-chain
				// schema.
				skipUntap := e.Text == ExtraTurnSkipUntapText
				for n := int32(0); n < e.Amount; n++ {
					g.ExtraTurnQueue = append(g.ExtraTurnQueue, state.ExtraTurn{Player: e.Player, SkipUntap: skipUntap})
				}
			} else {
				for i := len(g.ExtraTurnQueue) - 1; i >= 0; i-- {
					if g.ExtraTurnQueue[i].Player == e.Player {
						g.ExtraTurnQueue = append(g.ExtraTurnQueue[:i], g.ExtraTurnQueue[i+1:]...)
						break
					}
				}
			}
		}
		// Forge's ExtraTurnDelayedTrigger$ (Final Fortune: "At the beginning
		// of that turn's end step, you lose the game") registers the delayed
		// trigger HERE, at CONSUMPTION time, with the consumed turn's number
		// as its MinTurn -- so the ordinary Mode$ Phase delayed firing skips
		// the granting turn's own end step and fires exactly once, in the
		// granted turn. Registration must ride the consumption, not the grant:
		// with several extra turns pending (CR 500.7 takes them most recently
		// created first) the turn a grant PRODUCES is not known at grant time
		// -- it is exactly the turn about to begin when the -1 fires. Only the
		// -1 form registers; the +grant carries no Counter at consumption. A
		// consumption with no source object, no Execute$ name, or a source
		// whose face lacks the SVar degrades to no registration rather than
		// panicking (the same totality stance DelayedRegister applies).
		if e.Amount < 0 && e.Counter != "" && e.Obj != 0 && g.Obj(e.Obj) != nil {
			f := g.Obj(e.Obj).Face()
			if f != nil && cards.ResolveSVar(f.SVars, e.Counter) != nil {
				g.Delayed = append(g.Delayed, state.DelayedTrigger{
					ID:         g.DelayedNext,
					Phase:      state.StepEnd,
					Source:     e.Obj,
					Controller: e.Player,
					Execute:    e.Counter,
					MinTurn:    g.Turn + 1,
				})
				g.DelayedNext++
			}
		}

	case DoorUnlock:
		// CR 309.5: the unlock activation paid the locked half's mana cost as
		// a sorcery. The flag is what makes the alternate face's rules text
		// live (rules' trigger/static/ability scans) and what a Mode$
		// UnlockDoor trigger matches against. Totality: an unknown object, or
		// one already unlocked, is a no-op.
		if o := g.Obj(e.Obj); o != nil && !o.Unlocked {
			o.Unlocked = true
		}

	case SpeedChange:
		// One speed increment (CR 702.163). The cap and the once-per-turn
		// gate are the EMITTER's (rules' emit-side speed check) responsibility,
		// so Apply folds the delta plainly; a negative or oversized delta is
		// still clamped to [0, 4] defensively.
		if validPlayer(g, e.Player) {
			g.Players[e.Player].Speed += e.Amount
			if g.Players[e.Player].Speed < 0 {
				g.Players[e.Player].Speed = 0
			}
			if g.Players[e.Player].Speed > 4 {
				g.Players[e.Player].Speed = 4
			}
		}

	case MoveZone, Draw, PutOnStack:
		// CR 733.1 reverses a proposed cast with a real logged stack->origin
		// move. Preserve the entry history that preceded its stack proposal in
		// transient object state: a log-only replay sees the same PutOnStack,
		// captures the same fields and consumes them on the reverse move.
		wasStack := false
		if o := g.Obj(e.Obj); o != nil {
			wasStack = o.Zone == state.ZStack
			if e.To == state.ZStack {
				o.PreStackEntryThisTurn = o.EnteredThisTurn
				o.PreStackEntryFrom = o.EnteredFrom
				o.PreStackEnteredLen = len(g.Entered)
				o.HasPreStackEntry = true
			}
		}
		Move(g, e.Obj, e.From, e.To)
		if o := g.Obj(e.Obj); o != nil {
			if e.To == state.ZExile {
				switch e.Counter {
				case "exiled_with_face_down":
					// Hideaway's face-down exile (CR 702.75): the exiling source
					// rides in Amount, and FaceDown is state so a later projection
					// knows not to reveal the card.
					o.ExiledWith = state.ObjID(e.Amount)
					o.FaceDown = true
				case "exiled_with":
					o.ExiledWith = state.ObjID(e.Amount)
					o.FaceDown = false
				default:
					// MoveZone reserves its otherwise-unused IDs payload for the
					// source when an effect exiles a card (moveZoneEvent). This
					// provenance is event-derived, hence survives replay, and
					// clears as soon as the card leaves exile.
					if len(e.IDs) > 0 {
						o.ExiledWith = e.IDs[0]
					} else {
						o.ExiledWith = 0
					}
					o.FaceDown = false
				}
			} else {
				o.ExiledWith = 0
				o.FaceDown = false
			}
			if e.Text == "reversed" && o.HasPreStackEntry {
				o.EnteredThisTurn = o.PreStackEntryThisTurn
				o.EnteredFrom = o.PreStackEntryFrom
				if o.PreStackEnteredLen <= len(g.Entered) {
					g.Entered = g.Entered[:o.PreStackEnteredLen]
				}
				o.PreStackEntryThisTurn = false
				o.PreStackEntryFrom = state.ZLibrary
				o.PreStackEnteredLen = 0
				o.HasPreStackEntry = false
			} else if wasStack && e.To != state.ZStack {
				o.PreStackEntryThisTurn = false
				o.PreStackEntryFrom = state.ZLibrary
				o.PreStackEnteredLen = 0
				o.HasPreStackEntry = false
			}
		}
		// Source-dependent goads end as soon as their source leaves play.
		pruneGoads(g)

	case LifeChange:
		if validPlayer(g, e.Player) {
			g.Players[e.Player].Life += e.Amount
		}

	case Damage:
		if o := g.Obj(e.Obj); o != nil {
			// CR 306.8 / 120.3c: damage dealt to a planeswalker permanent
			// removes that many loyalty counters instead of being marked as
			// damage. The conversion lives here, on the one fold every
			// Damage event goes through, so spell/ability damage (effects/
			// damage.go), future combat damage and any emitter this build
			// gains later all convert the same way and a replay derives the
			// same loyalty from the same log. A planeswalker that is ALSO a
			// creature still takes marked damage (CR 120.3e -- the exchange
			// is not exclusive), and either way a positive amount records
			// that the object was dealt damage this turn. Prevention (CR
			// 702.16d) and protection replace or note the Damage event
			// before it reaches this fold, so a prevented hit converts
			// nothing -- which is why the walker exchange no longer needs
			// the bypass it used to travel by.
			walker := false
			if f := o.Face(); f != nil && f.IsPlaneswalker() {
				walker = true
				// Cleanup represents removal of marked damage with a negative
				// Damage event. It must never restore loyalty; only positive
				// damage has the CR 120.3c loyalty conversion.
				if e.Amount > 0 {
					o.AddCounter("LOYALTY", -e.Amount)
				}
			}
			// Counter is Damage's existing, encoded characteristic carrier:
			// effects/rules set it to creature from the current layer result.
			// The printed-face fallback retains direct-event callers and normal
			// printed creature behavior.
			creature := e.Counter == "creature" || (o.Face() != nil && o.Face().IsCreature())
			if !walker || creature {
				o.Damage += e.Amount
				if o.Damage < 0 {
					o.Damage = 0
				}
			}
			// A positive Damage event records that the object was dealt damage
			// this turn even if a later prevention/healing event clears its
			// marked damage. The history clears at the same TurnChange boundary
			// as the engine's other per-turn state below.
			if e.Amount > 0 {
				o.WasDealtDamageThisTurn = true
			}
		} else if validPlayer(g, e.Player) {
			g.Players[e.Player].Life -= e.Amount
		}

	case Tap:
		if o := g.Obj(e.Obj); o != nil {
			o.Tapped = true
		}
	case Untap:
		if o := g.Obj(e.Obj); o != nil {
			o.Tapped = false
		}

	case StepChange:
		g.Step = e.Step

	case TurnChange:
		if validPlayer(g, e.Player) {
			g.Turn = e.Amount
			g.Active = e.Player
			g.Players[e.Player].LandsPlayed = 0
			// g.Zone(ZBattlefield, e.Player) can only ever hold IDs that Move
			// already confirmed are real objects, so this nil check is
			// currently unreachable in practice -- but it is one line, it
			// matches every other zone-walk in this switch (DeclareAttackers,
			// DeclareBlockers) that guards g.Obj before dereferencing, and it
			// stops that invariant from becoming a silent, easy-to-reopen
			// panic if a future Kind ever populates a zone list some other
			// way. Found in the same audit as Ruling T20-e.
			for _, id := range g.Zone(state.ZBattlefield, e.Player) {
				if o := g.Obj(id); o != nil {
					o.SummonSick = false
				}
			}
			// TurnChange is the existing per-turn reset boundary. Zone-entry
			// provenance and damage history are object facts rather than facts
			// of the incoming active player, so reset every arena object here.
			for i := range g.Objs {
				g.Objs[i].EnteredThisTurn = false
				g.Objs[i].WasDealtDamageThisTurn = false
				g.Objs[i].ActivatedThisTurn = 0
				// Only default-duration goads expire at the goader's next turn.
				g.Objs[i].Goads = expireTurnGoads(g.Objs[i].Goads, e.Player)
			}
			// The per-add entry list is per-turn state too.
			g.Entered = nil
		}

	case Goad:
		if o := g.Obj(e.Obj); o != nil {
			if e.Amount == -1 {
				o.Goads = nil
				break
			}
			if !validPlayer(g, e.Player) {
				break
			}
			duration := e.Text
			if duration == "" {
				duration = "UntilYourNextTurn"
			}
			var source state.ObjID
			if len(e.IDs) > 0 {
				source = e.IDs[0]
			}
			controller := o.Controller
			if e.Amount > 0 && int(e.Amount-1) < len(g.Players) {
				controller = state.PlayerID(e.Amount - 1)
			}
			ge := state.GoadEffect{Player: e.Player, Source: source, Controller: controller, Duration: duration}
			for _, existing := range o.Goads {
				if existing == ge {
					return
				}
			}
			o.Goads = append(o.Goads, ge)
			pruneGoads(g)
		}

	case PlayerCounterChange:
		if validPlayer(g, e.Player) {
			g.Players[e.Player].AddCounter(e.Counter, e.Amount)
		}

	case Priority:
		if validPlayer(g, e.Player) {
			g.Priority = e.Player
			passes := e.Amount
			if passes < 0 {
				passes = 0
			}
			g.Passes = passes
		}

	case ManaAdd:
		if validPlayer(g, e.Player) {
			// "S<colour>" (e.g. "SW") is a SNOW mana unit (CR 107.4h): it lands
			// in the colour's pool slot and is tallied in Player.Snow so a {S}
			// pip can be paid only from it. One event moves both counters, so
			// the snow tally can never drift from the pool it parallels.
			if len(e.Counter) == 2 && e.Counter[0] == 'S' {
				idx := state.ManaIndex(e.Counter[1])
				g.Players[e.Player].Pool[idx] += e.Amount
				g.Players[e.Player].Snow[idx] += e.Amount
				break
			}
			idx := state.MC
			if e.Counter != "" {
				idx = state.ManaIndex(e.Counter[0])
			}
			player := &g.Players[e.Player]
			player.Pool[idx] += e.Amount
			if valid, srcID, restricted := ManaRestrictionFromText(e.Text); restricted {
				if e.Amount > 0 {
					player.RestrictedMana = append(player.RestrictedMana, state.ManaRestriction{
						Color: e.Counter, Amount: e.Amount, Valid: valid, Source: srcID,
					})
				} else if e.Amount < 0 {
					// A restricted spend event names exactly the restriction batch it
					// consumes. Walk insertion order so two matching additions replay
					// identically, and tolerate a malformed historical event that
					// over-spends its batch without making Pool negative here.
					need := -e.Amount
					for i := 0; i < len(player.RestrictedMana) && need > 0; {
						r := &player.RestrictedMana[i]
						if r.Color != e.Counter || r.Valid != valid {
							i++
							continue
						}
						used := r.Amount
						if used > need {
							used = need
						}
						r.Amount -= used
						need -= used
						if r.Amount == 0 {
							player.RestrictedMana = append(player.RestrictedMana[:i], player.RestrictedMana[i+1:]...)
							continue
						}
						i++
					}
				}
			}
		}

	case ManaClear:
		if validPlayer(g, e.Player) {
			g.Players[e.Player].Pool = state.Mana{}
			g.Players[e.Player].RestrictedMana = nil
			g.Players[e.Player].Snow = state.Mana{}
		}

	case CounterChange:
		if o := g.Obj(e.Obj); o != nil {
			o.AddCounter(e.Counter, e.Amount)
		}

	case DeclareAttackers:
		// e.Player names the attacking player for every ID in this event, so
		// it is validated once, like TurnChange/Priority above, rather than
		// per object. Nothing reads Object.Attacking yet, so an unvalidated
		// value cannot panic today -- but it is the same untrusted-seat-id
		// pattern as Ruling T20-e, found in the same audit, and closing it
		// now costs nothing for a well-formed event (Player is always valid
		// there).
		if !validPlayer(g, e.Player) {
			break
		}
		for _, id := range e.IDs {
			if o := g.Obj(id); o != nil {
				o.IsAttacking = true
				o.Attacking = e.Player
			}
		}

	case DeclareBlockers:
		for _, pr := range e.Pairs {
			a := g.Obj(pr[0])
			if a == nil || g.Obj(pr[1]) == nil {
				continue
			}
			a.BlockedBy = append(a.BlockedBy, pr[1])
		}

	case PlayerLost:
		if validPlayer(g, e.Player) {
			g.Players[e.Player].Lost = true
		}

	case GameOver:
		// Ruling T22-g (fix round 1): the first GameOver wins; a later one
		// on an already-finished game is a no-op. Without this guard, a log
		// carrying two GameOver events (a duplicate, a replay quirk, a
		// tampered log) produced a game simultaneously won (Winner set by
		// the first) and drawn (Draw set by the second) -- Over alone
		// cannot express "this event doesn't apply", so the guard has to
		// live here, ahead of everything else in this case.
		if g.Over {
			break
		}
		// Ruling T22-a (Amount is the draw/winner discriminator, the same
		// trick TargetsChosen already uses to tell its two target shapes
		// apart), pinned down fully by T22-g and, for Amount == 0's own
		// Player validity, T22-l: exactly two shapes are defined, and
		// everything else is not a GameOver this build recognizes at all.
		//   Amount == 0: a win, but ONLY when Player validates. Ruling
		//     T22-l (fix round 2): a fix-round-1 build of this let an
		//     invalid Player under Amount 0 still set Over, with Winner
		//     left at its untouched zero value -- an invalid seat silently
		//     reading as seat 0 winning, the exact defect class this whole
		//     discriminator exists to remove. For an untrusted log,
		//     refusing to end the game on a malformed win claim is the safe
		//     response: it is detectable (Over stays false) rather than
		//     manufacturing a draw the log never actually carried.
		//   Amount == 1: CR 104.4a's draw. Player is irrelevant; Winner is
		//     never touched and Draw says so explicitly, since PlayerID(0)
		//     is both Winner's zero value and a real seat and so cannot
		//     mean "nobody" on its own.
		//   anything else (including Amount == 0 with an invalid Player):
		//     not a shape this Kind defines. Previously any other Amount
		//     still set Over unconditionally and, since Winner's own guard
		//     only ever checked validPlayer, a tampered event naming an
		//     out-of-range Player under some third Amount read as "seat 0
		//     won" regardless. Changing nothing at all is the safe response
		//     to a shape this build cannot interpret.
		switch {
		case e.Amount == 0 && validPlayer(g, e.Player):
			g.Over = true
			g.Winner = e.Player
		case e.Amount == 1:
			g.Over = true
			g.Draw = true
		}

	case LandPlayed:
		if validPlayer(g, e.Player) {
			g.Players[e.Player].LandsPlayed++
		}

	case TargetsChosen:
		// Amount discriminates the target shape (Ruling T14-b's own
		// discriminator, extended by Task 4 with two more shapes that
		// APPEND rather than replace -- a spell can gain a second target
		// from a later effect without losing the first):
		//   0 (default): replace with object targets, read from IDs.
		//   1: replace with a single player target, read from Player.
		//   2: append one object target per entry in IDs.
		//   3: append a single player target, read from Player.
		if o := g.Obj(e.Obj); o != nil {
			switch e.Amount {
			case 1:
				if validPlayer(g, e.Player) {
					o.Targets = []state.Target{{Player: e.Player, IsPlayer: true}}
				}
			case 2:
				for _, id := range e.IDs {
					o.Targets = append(o.Targets, state.Target{Obj: id})
				}
			case 3:
				if validPlayer(g, e.Player) {
					o.Targets = append(o.Targets, state.Target{Player: e.Player, IsPlayer: true})
				}
			default:
				var targets []state.Target
				for _, id := range e.IDs {
					targets = append(targets, state.Target{Obj: id})
				}
				o.Targets = targets
			}
		}

	case FlipFace:
		if o := g.Obj(e.Obj); o != nil && o.Card != nil &&
			e.Amount >= 0 && int(e.Amount) < len(o.Card.Faces) {
			o.FaceIdx = uint8(e.Amount)
		}

	case ClockTick:
		g.Clock++

	case TriggerPush:
		// Ruling T20-a: the ability object is minted here, inside Apply, so
		// a log-only replay creates the exact same object a live game did --
		// not via a direct, unlogged AddObject call from rules.Engine. e.Obj
		// names the permanent whose trigger fired; a permanent that no
		// longer exists, or an out-of-range trigger index (stale data, a
		// tampered log), degrades to a no-op rather than panicking.
		//
		// Ruling T20-e: Player must be checked too, same as every sibling
		// case in this switch that indexes a Player-keyed slice (LifeChange,
		// TurnChange, Priority, ManaAdd, ManaClear). This case skipped it,
		// and an out-of-range Player flows straight into g.AddObject below,
		// then into Move's zoneOwner/SetZone/zoneIndex path, which indexes
		// g.zones (sized numZones*len(g.Players) at NewGame) with no bounds
		// check of its own -- panic: index out of range. Unreachable via
		// ordinary self-play (Player is always sourced from a real object's
		// controller) but directly reachable replaying an external,
		// corrupted, or tampered log -- exactly the case this event exists
		// to support.
		if !validPlayer(g, e.Player) {
			break
		}
		src := g.Obj(e.Obj)
		if src == nil {
			break
		}
		f := src.Face()
		if f == nil || e.Amount < -1 || int(e.Amount) >= len(f.Triggers) {
			break
		}
		o := g.AddObject(nil, e.Player)
		// Move first (it resets Remembered, among other stack-only fields,
		// for anything other than a battlefield destination -- see Move's
		// own default case below), then set what the ability actually
		// carries. Setting these before Move would have them wiped by that
		// same reset; ordering them after is what makes Remembered actually
		// survive onto the stack (Ruling T20-c).
		Move(g, o.ID, state.ZLibrary, state.ZStack)
		if e.Amount == -1 {
			// A layer-granted Dethrone has no printed Trigger index. The
			// matcher already established its condition; this logged sentinel
			// carries the fixed keyword body through replay.
			o.Ability = &cards.SA{Kind: "DB", API: "PutCounter", Params: map[string]string{
				"Defined": "Self", "CounterType": "P1P1", "CounterNum": "1",
			}}
		} else {
			o.Ability = f.Triggers[e.Amount].Effect
		}
		o.Source = e.Obj
		// FL-41: an id in IDs is either a real object (the ordinary case)
		// or a player reference (state.PlayerRef, rules.pushTrigger) --
		// triggerRemembered's DeclareAttackers case appends the defending
		// player to Remembered, and IDs has no field of its own for a bare
		// PlayerID, so that entry travels here encoded rather than as a bare
		// ObjID that would decode as "object 0": playersOf (effects/context.go)
		// filters that entry out, so Defined$ TriggeredDefendingPlayer would
		// resolve to nothing and the effect silently no-op.
		o.Remembered = rememberedFrom(e.IDs)

	case EndCombatReset:
		// Obj zero retains the original whole-combat reset. A nonzero Obj
		// removes only that permanent (regeneration). Keep a zero tombstone
		// in attackers' blocker lists: they remain blocked (CR 509.1h),
		// while liveBlockers ignores the removed blocker, even if it lives.
		for i := range g.Objs {
			o := &g.Objs[i]
			if e.Obj == 0 || o.ID == e.Obj {
				o.IsAttacking = false
				o.BlockedBy = nil
			} else {
				for j, id := range o.BlockedBy {
					if id == e.Obj {
						o.BlockedBy[j] = 0
					}
				}
			}
		}

	case CastInfo:
		if o := g.Obj(e.Obj); o != nil {
			o.X = e.Amount
			o.CastFlags = FlagsFrom(e.Counter)
		}

	case Choose:
		if o := g.Obj(e.Obj); o != nil {
			switch e.Counter {
			case "name":
				o.ChosenName = e.Text
			case "type":
				o.ChosenType = e.Text
			case "number":
				o.ChosenNumber = e.Amount
			case "riot":
				o.RiotChoice = e.Text
			case "chosen":
				o.Chosen = rememberedFrom(e.IDs)
			case "remembered":
				o.Remembered = append(o.Remembered, rememberedFrom(e.IDs)...)
			case "forget-remembered":
				// ForgetChanged$ True (Forge ChangeZoneEffect's
				// host.removeRemembered on the moved card): the named cards leave
				// the source object's persistent Remembered list. Player entries
				// and ids not named are kept, so a bad or partial payload degrades
				// to a smaller forget, never a wider one.
				drop := make(map[state.ObjID]bool, len(e.IDs))
				for _, id := range e.IDs {
					drop[id] = true
				}
				kept := make([]state.Target, 0, len(o.Remembered))
				for _, t := range o.Remembered {
					if !t.IsPlayer && drop[t.Obj] {
						continue
					}
					kept = append(kept, t)
				}
				o.Remembered = kept
			case "clear-remembered":
				o.Remembered = nil
			case "clear-chosen-card":
				// Forge's Cleanup ClearChosenCard$: the chosen-card half of the
				// object's chosen list goes; chosen players stay.
				kept := o.Chosen[:0]
				for _, t := range o.Chosen {
					if t.IsPlayer {
						kept = append(kept, t)
					}
				}
				o.Chosen = kept
			case "clear-chosen-player":
				// The chosen-PLAYER half goes; chosen cards stay.
				keptCards := o.Chosen[:0]
				for _, t := range o.Chosen {
					if !t.IsPlayer {
						keptCards = append(keptCards, t)
					}
				}
				o.Chosen = keptCards
			}
		}

	case TokenCreate:
		if !validPlayer(g, e.Player) {
			break
		}
		def, ok := g.Tokens[e.Text]
		if !ok || def == nil {
			break
		}
		o := g.AddObject(def, e.Player)
		o.IsToken = true
		Move(g, o.ID, state.ZLibrary, state.ZBattlefield)

	case CardToken:
		// A battlefield token that is a copy of the CARD object Obj names
		// (encore's "create a token copy" per opponent). Mirrors StackCopy's
		// snapshot discipline: every read from src is taken into a local
		// BEFORE AddObject, because AddObject may reallocate g.Objs and a
		// src pointer read after it would read the old backing array.
		// Totality like every case: a missing source (already ceased to
		// exist) or an invalid player mints nothing.
		if !validPlayer(g, e.Player) {
			break
		}
		src := g.Obj(e.Obj)
		if src == nil || src.Card == nil {
			break
		}
		card, faceIdx := src.Card, src.FaceIdx
		o := g.AddObject(card, e.Player)
		o.IsToken = true
		o.FaceIdx = faceIdx
		Move(g, o.ID, state.ZLibrary, state.ZBattlefield)
		// Encore encodes its required defender as seat+1; zero remains the
		// ordinary CardToken shape. The current turn is folded here so replay
		// reconstructs the same one-turn attack requirement.
		if e.Amount > 0 {
			defender := state.PlayerID(e.Amount - 1)
			if validPlayer(g, defender) {
				o.EncoreAttackTurn = g.Turn
				o.EncoreAttackDefender = defender
			}
		}

	case KeywordTriggerPush:
		if !validPlayer(g, e.Player) {
			break
		}
		src := g.Obj(e.Obj)
		if src == nil || src.Face() == nil {
			break
		}
		sa := cards.ResolveSVar(src.Face().SVars, e.Counter)
		if sa == nil {
			// A granted ward (rules.pushTrigger's __kwWard: payload) has no
			// SVar to resolve: the ability is rebuilt structurally from the
			// payload -- the same DB$ Ward | UnlessCost$ <cost> a printed
			// K:Ward's compiled trigger carries -- so the live game and the
			// replay mint identical objects from the event text alone.
			if rest, ok := strings.CutPrefix(e.Counter, "__kwWard:"); ok {
				sa = &cards.SA{Kind: "DB", API: "Ward",
					Params: map[string]string{"UnlessCost": rest, "TriggerDescription": "Ward"}}
			}
		}
		if sa == nil {
			break
		}
		incarnation := src.Incarnation
		o := g.AddObject(nil, e.Player)
		Move(g, o.ID, state.ZLibrary, state.ZStack)
		o.Ability = sa
		o.Source = e.Obj
		o.SourceIncarnation = incarnation

	case StackCopy:
		if !validPlayer(g, e.Player) {
			break
		}
		src := g.Obj(e.Obj)
		if src == nil || src.Zone != state.ZStack {
			break
		}
		// Snapshot everything read from src into locals before AddObject:
		// AddObject appends to g.Objs and may reallocate its backing array,
		// and src is a pointer into that array (g.Obj returns &g.Objs[id-1])
		// -- so a *src field read after AddObject would come from whatever
		// the old backing array still holds, not necessarily kept in sync
		// with the live object src names. Correct today only because
		// nothing between AddObject and the reads below mutates src; this
		// is the engine's one mutation path, so it does not get to rely on
		// that happening to remain true.
		card, faceIdx, ability, source := src.Card, src.FaceIdx, src.Ability, src.Source
		x, castFlags := src.X, src.CastFlags
		// Deep-copy, never alias: the copy's Targets/Remembered must be
		// able to change independently of the original's once both sit on
		// the stack.
		targets := append([]state.Target(nil), src.Targets...)
		remembered := append([]state.Target(nil), src.Remembered...)

		o := g.AddObject(card, e.Player)
		Move(g, o.ID, state.ZLibrary, state.ZStack)
		o.FaceIdx, o.Ability, o.Source = faceIdx, ability, source
		o.Targets = targets
		o.Remembered = remembered
		o.X, o.CastFlags, o.IsCopy = x, castFlags, true

	case Attach:
		if o := g.Obj(e.Obj); o != nil {
			if len(e.IDs) == 0 {
				o.AttachedTo = 0
			} else if g.Obj(e.IDs[0]) != nil {
				o.AttachedTo = e.IDs[0]
			}
		}

	case AbilityPush:
		// Mirrors TriggerPush above (Ruling T20-a): the ability object is
		// minted here, inside Apply, so a log-only replay creates the same
		// object a live game did.
		if !validPlayer(g, e.Player) {
			break
		}
		src := g.Obj(e.Obj)
		if src == nil {
			break
		}
		f := src.Face()
		if f == nil || e.Amount < 0 || int(e.Amount) >= len(f.Abilities) {
			break
		}
		o := g.AddObject(nil, e.Player)
		Move(g, o.ID, state.ZLibrary, state.ZStack)
		o.Ability = f.Abilities[e.Amount]
		o.Source = e.Obj
		// The per-source activation census (state/object.go's
		// ActivatedThisTurn): one AbilityPush per non-mana activation, folded
		// onto the source the same way the other per-turn object facts are.
		if src.Zone == state.ZBattlefield {
			src.ActivatedThisTurn++
		}
		// Same PlayerRef decode as TriggerPush above (FL-41): an activated
		// ability can remember a player the same way a trigger can, so the
		// two mint paths stay symmetric through rememberedFrom.
		o.Remembered = rememberedFrom(e.IDs)

	case DelayedRegister:
		// Ruling dt1-a: the registration is game state folded here, so a
		// log-only replay rebuilds the same set a live game held. Totality
		// like every case: an invalid controller, a nonexistent source, or a
		// non-canonical Step (at the very least a zero Step is untap, which
		// every DelayedRegister this build emits carries a real phase for)
		// degrades to a no-op rather than panicking.
		if !validPlayer(g, e.Player) {
			break
		}
		if g.Obj(e.Obj) == nil || !e.Step.Valid() {
			break
		}
		src := g.Obj(e.Obj)
		// Dash and Warp refer to the exact permanent that received their
		// keyword promise. Encore's grouped delayed trigger does not: CR
		// 603.7 leaves it independent of the card that created it, and it
		// must sacrifice its remembered token group even if that card later
		// changes zones and returns as a new incarnation.
		track := strings.HasPrefix(e.Counter, "__kwDash") ||
			strings.HasPrefix(e.Counter, "__kwWarp")
		// Event-matched (non-phase) registrations encode
		// "<Mode$ value>:<trigger SVar name>" in Text. The DelayedRegister
		// event gains no field of its own (Ruling T20-a's field-reuse
		// precedent); a Mode$ Phase registration's Text is the Forge Phase$
		// string, which never contains a colon, and the decode only splits on
		// the modes rules.registerOpeningEffectTriggers emits, so every
		// already-logged registration decodes as a phase one.
		mode, trigger := "", ""
		if i := strings.Index(e.Text, ":"); i > 0 && e.Text[:i] == "SpellCast" {
			mode, trigger = "SpellCast", e.Text[i+1:]
		}
		g.Delayed = append(g.Delayed, state.DelayedTrigger{
			ID:                g.DelayedNext,
			Phase:             e.Step,
			Source:            e.Obj,
			Controller:        e.Player,
			Execute:           e.Counter,
			Remembered:        rememberedFrom(e.IDs),
			MinTurn:           e.Amount,
			SourceIncarnation: src.Incarnation,
			TrackSource:       track,
			EventMode:         mode,
			Trigger:           trigger,
		})
		g.DelayedNext++

	case DelayedPush:
		// Ruling dt1-a: the ability object is minted here, inside Apply, so a
		// log-only replay creates the same object a live game did (the
		// Ruling T20-a precedent TriggerPush and AbilityPush already set).
		// Unlike those two, the Ability is not a face Triggers index -- a
		// delayed trigger's Effect is an SVar-named sub-ability on the
		// source's face -- so it is resolved here from e.Counter (the
		// Execute$ name) via cards.ResolveSVar. Firing is one-shot: the
		// matching registration is removed, which is what keeps a delayed
		// trigger from firing every turn.
		if !validPlayer(g, e.Player) {
			break
		}
		// Consume the registration first, even when its tracked permanent has
		// changed incarnation. A stale dash/warp promise expires once; it must
		// neither act on the returned object nor be retried forever. Ordinary
		// delayed triggers, including Encore's group cleanup, are independent
		// of their source and still resolve.
		var registration *state.DelayedTrigger
		for i := range g.Delayed {
			if g.Delayed[i].ID == uint32(e.Amount) {
				dt := g.Delayed[i]
				registration = &dt
				g.Delayed = append(g.Delayed[:i], g.Delayed[i+1:]...)
				break
			}
		}
		src := g.Obj(e.Obj)
		if src == nil {
			break
		}
		if registration != nil && registration.TrackSource &&
			src.Incarnation != registration.SourceIncarnation {
			break
		}
		if src.Face() == nil {
			break
		}
		sa := resolveSVarAcrossFaces(src, e.Counter)
		if sa == nil {
			break
		}
		o := g.AddObject(nil, e.Player)
		Move(g, o.ID, state.ZLibrary, state.ZStack)
		o.Ability = sa
		o.Source = e.Obj
		if registration != nil && registration.TrackSource {
			o.SourceIncarnation = src.Incarnation
		}
		o.Remembered = rememberedFrom(e.IDs)

	case CmdDamage:
		// Commander combat damage to a player (CR 903.10, Task m33): fold
		// Amount into Player's cumulative tally at the source commander's
		// match-wide dense index, exactly the slot m30's genesis sizes and
		// New's Commanders bookkeeping names, so commander B keeps one slot
		// - and one cumulative total - for the whole match, across zone
		// changes and recasts (state.ObjID is stable across moves). A
		// log-only reconstruction reproduces the tally because the event
		// itself carries it; deriving it from the existing Damage events is
		// impossible because those carry no source. Guarded to totality
		// like every case here: an out-of-range Player, a nonexistent
		// source, or a game whose CmdDamage was never sized (a non-
		// Commander game, or a hand-built state) is a no-op, never a panic.
		if validPlayer(g, e.Player) && e.Obj != 0 {
			if idx, ok := commanderDenseIndex(g, e.Obj); ok {
				if idx >= 0 && idx < len(g.Players[e.Player].CmdDamage) {
					g.Players[e.Player].CmdDamage[idx] += e.Amount
				}
			}
		}
	}
}

// commanderDenseIndex returns id's match-wide dense commander index - (valid
// commanders in every seat before its owner) plus (its position within its
// owner's Commanders list) - and whether id is a commander at all, mirroring
// the exact indexing rules.New assigns at genesis (see rules/engine.go):
// seat B's CmdDamage holds a slot for seat A's commander at A's commander's
// dense index, so the k-th commander of seat p is index
// sum(len(Commanders[s]) for s<p) + k. Walked deterministically by index over
// the seat slice and each seat's Commanders slice - never a map - so the
// order cannot and does not matter to the result.
func commanderDenseIndex(g *state.Game, id state.ObjID) (int, bool) {
	if id == 0 {
		return 0, false
	}
	for p := range g.Players {
		for k, c := range g.Players[p].Commanders {
			if c == id {
				idx := 0
				for s := 0; s < p; s++ {
					idx += len(g.Players[s].Commanders)
				}
				return idx + k, true
			}
		}
	}
	return 0, false
}

// rememberedFrom decodes an event's IDs into the Remembered list an ability
// object carries: a real object id becomes {Obj: id}, and a PlayerRef
// sentinel (state.PlayerRef, rules.pushTrigger) becomes {Player: p,
// IsPlayer: true}. TriggerPush and AbilityPush both use it so the two mint
// paths stay symmetric.
func rememberedFrom(ids []state.ObjID) []state.Target {
	var out []state.Target
	for _, id := range ids {
		if p, ok := id.PlayerRef(); ok {
			out = append(out, state.Target{Player: p, IsPlayer: true})
			continue
		}
		out = append(out, state.Target{Obj: id})
	}
	return out
}

// Move relocates an object between zones, preserving zone order and the
// one-object-one-zone invariant.
//
// The zone an object is removed from is always o.Zone — the object's own
// recorded location — never the caller-supplied from. from (and Event.From
// in the log) exist for the client and for replay to read, but a caller that
// gets it wrong must not be able to leave the object in its real zone while
// also adding it to to: that would put it in two zones at once, and a
// repeat of the same wrong move would duplicate it within one zone.
//
// Moving an object to the zone it is already in is not special-cased: it is
// removed from that zone and appended again, so it ends up at the end of the
// zone's order. That is deterministic and matches every other move.
func Move(g *state.Game, id state.ObjID, from, to state.Zone) {
	o := g.Obj(id)
	if o == nil || !o.Zone.Valid() || !to.Valid() {
		return
	}
	// The object's real (pre-move) zone, captured before o.Zone is
	// overwritten below -- Task 4's X/CastFlags/Chosen* reset needs to know
	// whether this object is actually LEAVING the battlefield, not merely
	// where the caller-supplied from claims it came from (the same
	// real-zone-over-claimed-zone rule this function already applies to the
	// removal itself, a few lines below).
	enteredFrom := o.Zone
	wasBattlefield := enteredFrom == state.ZBattlefield
	wasStack := enteredFrom == state.ZStack
	if enteredFrom == state.ZExile && to != state.ZExile {
		// Forge's exiledCards association is a zone relationship, not an
		// imprint. Once this object leaves exile it is a new object for that
		// association, even if a later effect exiles the same engine ObjID.
		// Do this inside Apply's Move fold so live play and log replay prune
		// every source's list identically. The ExileReturn list (ChangeZone's
		// Duration$ UntilHostLeavesPlay) prunes identically: a card that left
		// exile by any other path is no longer its exiler's business to return.
		for i := range g.Objs {
			g.Objs[i].ExiledCards = withoutObjID(g.Objs[i].ExiledCards, id)
			g.Objs[i].ExileReturn = withoutExileReturnObj(g.Objs[i].ExileReturn, id)
		}
	}
	if wasBattlefield && to != state.ZBattlefield {
		// Leaving combat removes this permanent as a blocker, but does not
		// make creatures it blocked unblocked (CR 506.4, 509.1h). Preserve
		// each attacker's blocker-list length with the same zero tombstone
		// EndCombatReset uses for regeneration. Dense arena order keeps this
		// deterministic, and the departing object's own state is cleared by
		// the zone reset below.
		for i := range g.Objs {
			other := &g.Objs[i]
			if other.ID == id {
				continue
			}
			for j, blocker := range other.BlockedBy {
				if blocker == id {
					other.BlockedBy[j] = 0
				}
			}
		}
	}
	remove(g, id, o.Zone, zoneOwner(o, o.Zone))
	// CR 400.7: leaving the battlefield makes the object a new object in
	// its next zone, so control-changing effects do not follow it. Reset
	// before choosing the destination's zone owner: a later graveyard/hand
	// re-entry must be placed under its owner, not its former controller.
	if wasBattlefield && to != state.ZBattlefield {
		o.Controller = o.Owner
	}
	if to != state.ZCeased {
		dst := zoneOwner(o, to)
		g.SetZone(to, dst, append(g.Zone(to, dst), id))
	}

	o.Zone = to
	// The incarnation stamp is used by promises tied to a particular
	// permanent (evoke/dash/warp), so only crossing the battlefield
	// boundary advances it. A provisional hand->stack->hand CR 733 reversal
	// must restore byte-identical state and is not a permanent incarnation.
	if enteredFrom != to && (enteredFrom == state.ZBattlefield || to == state.ZBattlefield) {
		o.Incarnation++
	}
	// A new object in a new zone has its owner's default control. The old
	// controller is needed above to remove it from the battlefield/stack, so
	// reset only after removal and placement have used that zone ownership.
	if to != state.ZBattlefield && to != state.ZStack {
		o.Controller = o.Owner
	}
	// A zone change is the single source of zone-entry provenance. Capture
	// the actual old zone (not Event.From, which Move deliberately treats as
	// advisory) so replay and a live game derive identical ThisTurnEntered*
	// state even from a malformed caller-supplied From.
	o.EnteredThisTurn = true
	o.EnteredFrom = enteredFrom
	// Record the per-add entry the Count$ThisTurnEntered_* heads and the
	// ThisTurnEntered* filter predicates read (Forge's per-zone
	// getCardsAddedThisTurn lists, one append per add). Every Move routes
	// through events.Apply, so this stays inside the Apply-only mutation
	// discipline; the TurnChange case clears the list with the rest of the
	// per-turn state. A reversed CR 733.1 cast proposal keeps its entries:
	// the proposal's PutOnStack entry and the reverse move's entry both
	// land, and no corpus head reads the zones that pair touches
	// (Hand_from_Stack does, and the double entry it sees is the honest
	// record of the two moves).
	g.Entered = append(g.Entered, state.ZoneEntry{Obj: id, To: to, From: enteredFrom})
	switch to {
	case state.ZBattlefield:
		o.SummonSick = true
		o.Damage = 0
		g.Clock++
		o.Timestamp = g.Clock
		// CR 306.5b: a planeswalker enters the battlefield with loyalty
		// counters equal to its starting loyalty, however it entered (a
		// resolving spell, a blink or re-entry, a search put it directly onto
		// the battlefield). Implementing the grant here, inside Move itself,
		// is what makes every battlefield-entry site covered by construction:
		// no rules/ or effects/ caller can mint an entry that skips it, and
		// replay (which re-runs Apply) derives the identical counters. Two
		// boundaries keep the grant exact:
		//
		//   - A battlefield->battlefield move (counters are NOT reset on a
		//     stay on the battlefield) must not re-stack loyalty, so the grant
		//     is skipped when the object was already on the battlefield.
		//   - A face whose starting loyalty this engine cannot read (absent,
		//     or Loyalty:X -- Nissa, Steward of Elements) grants nothing.
		//
		// Known gap (recorded in AGENTS.md): TokenCreate does NOT route
		// through Move, so a planeswalker TOKEN enters with zero loyalty.
		if !wasBattlefield {
			if f := o.Face(); f != nil && f.IsPlaneswalker() {
				if n, err := strconv.Atoi(strings.TrimSpace(f.Loyalty)); err == nil && n > 0 {
					o.AddCounter("LOYALTY", int32(n))
				}
			}
			// Riot's choice is made before this entry. Applying it in Move
			// makes all entry paths obey the same logged choice.
			switch o.RiotChoice {
			case "counter":
				o.AddCounter("P1P1", 1)
			case "haste":
				o.IntrinsicKeywords = append(o.IntrinsicKeywords, "Haste")
			}
			o.RiotChoice = ""
			// CR 702.151a (Sagas, kw:Chapter): "As this Saga enters ... add a
			// lore counter" -- the same every-entry-site grant the loyalty
			// half above is. The chapter-I trigger queues rules-side off this
			// Move event (rules' chapter check reads the live counter, which
			// by then includes this grant).
			if _, names := cards.SagaChapters(o.Face()); len(names) > 0 {
				o.AddCounter("LORE", 1)
			}
		}
	default:
		// CR 702.103: a Soulbond pair ends when either member leaves the
		// battlefield. Move itself is the complete logged state transition, so
		// clear the remaining member here too; replay derives the same break
		// without a second event.
		if wasBattlefield && o.Paired != 0 {
			if partner := g.Obj(o.Paired); partner != nil && partner.Paired == o.ID {
				partner.Paired = 0
			}
		}
		// Leaving the battlefield or the stack resets everything that only
		// exists while a permanent or spell is in play.
		o.Tapped = false
		o.Damage = 0
		o.IsAttacking = false
		o.BlockedBy = nil
		o.Counters = nil
		o.IntrinsicKeywords = nil
		o.ExiledWith = 0
		o.FaceDown = false
		o.RiotChoice = ""
		o.IsMyriad = false
		o.Paired = 0
		o.Targets = nil
		o.Remembered = nil
		if wasBattlefield {
			o.Imprinted = nil
		}
		// X/CastFlags/Chosen* carry cast-time and choose-time information
		// forward from the stack onto the permanent it resolves into (an
		// ETB "if it was kicked" trigger needs to read X/CastFlags off the
		// permanent, not just the spell) -- so hand/stack -> battlefield
		// must NOT reset them, and they only reset once the permanent
		// genuinely leaves the battlefield again. AttachedTo has no legal
		// life off the battlefield at all (an Aura/Equipment that isn't a
		// permanent cannot be "attached"), so it always resets here
		// regardless of where the object came from.
		if wasBattlefield {
			o.X, o.CastFlags = 0, 0
			o.ChosenName, o.ChosenType, o.ChosenNumber = "", "", 0
			o.Chosen = nil
		}
		// CR 107.3m: the paid X belongs to the spell on the stack and to the
		// permanent the spell becomes, and to nothing else. An object leaving
		// the stack for a zone OTHER than the battlefield -- a countered or
		// fizzled spell into the graveyard, a resolving instant/sorcery -- is
		// a card in a non-battlefield zone, where X in its text is 0. Without
		// this the stale paid X rides along: a countered Genesis Hydra
		// reanimated later would resolve its ETB trigger with the dead cast's
		// X instead of 0. (A stack->battlefield move keeps X/CastFlags -- the
		// battlefield case above deliberately does not reset them, which is
		// what lets an ETB trigger read them off the permanent.)
		if wasStack {
			o.X, o.CastFlags = 0, 0
		}
		// ChosenModes is needed only while a modal spell/ability resolves (or
		// when a permanent spell carries its announcement onto the battlefield).
		// Clearing it as an object leaves the stack keeps this derived cache out
		// of graveyards/exile; an aborted cast restores its captured prior value
		// after the reverse stack move.
		if wasStack {
			o.ChosenModes = nil
		}
		o.AttachedTo = 0
	}
}

// changeControl gives o to controller p. The battlefield is keyed by
// controller (zoneOwner), so a permanent moves from its old controller's list
// to the new one's, the way Forge's controllerChangeZoneCorrection does;
// every per-controller reader (untap step, attackers, blockers, mana and
// activation offers, statics, projections) then sees it under its controller.
// The stack is one shared list, so a spell only changes its Controller.
//
// On the battlefield a control change also:
//   - removes the permanent from combat (CR 506.4): it stops attacking, loses
//     its blockers, and attackers it blocked keep a zero tombstone so they stay
//     blocked (CR 509.1h), exactly as a departing blocker does in Move;
//   - makes it summoning sick (CR 302.6): its new controller has not controlled
//     it continuously since their most recent turn began. TurnChange clears it
//     from the active player's list, i.e. at its new controller's next turn.
//
// withoutObjID returns ids without id, retaining its order and avoiding an
// allocation when no entry matches. ExiledCards is a short insertion-ordered
// relation, so an ordered slice preserves deterministic selector results.
func withoutObjID(ids []state.ObjID, id state.ObjID) []state.ObjID {
	for i, got := range ids {
		if got != id {
			continue
		}
		out := append([]state.ObjID(nil), ids[:i]...)
		for _, got := range ids[i:] {
			if got != id {
				out = append(out, got)
			}
		}
		return out
	}
	return ids
}

// withoutExileReturnObj drops every ExileReturn entry naming id, preserving
// the order of the survivors (the same withoutObjID contract for the
// entry-valued list).
func withoutExileReturnObj(entries []state.ExileReturnEntry, id state.ObjID) []state.ExileReturnEntry {
	for i, got := range entries {
		if got.Obj != id {
			continue
		}
		out := append([]state.ExileReturnEntry(nil), entries[:i]...)
		for _, got := range entries[i:] {
			if got.Obj != id {
				out = append(out, got)
			}
		}
		return out
	}
	return entries
}

func changeControl(g *state.Game, o *state.Object, p state.PlayerID) {
	if o.Controller == p {
		return
	}
	if o.Zone == state.ZBattlefield {
		remove(g, o.ID, state.ZBattlefield, o.Controller)
		g.SetZone(state.ZBattlefield, p, append(g.Zone(state.ZBattlefield, p), o.ID))
		for i := range g.Objs {
			other := &g.Objs[i]
			if other.ID == o.ID {
				continue
			}
			for j, blocker := range other.BlockedBy {
				if blocker == o.ID {
					other.BlockedBy[j] = 0
				}
			}
		}
		o.IsAttacking = false
		o.BlockedBy = nil
		o.SummonSick = true
	}
	o.Controller = p
}

// validPlayer reports whether p indexes an existing seat.
func validPlayer(g *state.Game, p state.PlayerID) bool {
	return int(p) < len(g.Players)
}

// expireTurnGoads drops only default-duration relationships made by p.
func expireTurnGoads(in []state.GoadEffect, p state.PlayerID) []state.GoadEffect {
	out := in[:0]
	for _, ge := range in {
		if ge.Player == p && (ge.Duration == "" || ge.Duration == "UntilYourNextTurn") {
			continue
		}
		out = append(out, ge)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// pruneGoads enforces source/control conditions from replayable state.
func pruneGoads(g *state.Game) {
	for i := range g.Objs {
		o := &g.Objs[i]
		out := o.Goads[:0]
		for _, ge := range o.Goads {
			active := o.Zone == state.ZBattlefield
			switch ge.Duration {
			case "AsLongAsInPlay":
				src := g.Obj(ge.Source)
				active = active && src != nil && src.Zone == state.ZBattlefield
			case "AsLongAsControl":
				active = o.Zone == state.ZBattlefield && o.Controller == ge.Controller
			}
			if active {
				out = append(out, ge)
			}
		}
		if len(out) == 0 {
			o.Goads = nil
		} else {
			o.Goads = out
		}
	}
}

// zoneOwner picks whose zone list an object belongs to: the battlefield and the
// stack are keyed by controller, every private zone by owner.
func zoneOwner(o *state.Object, z state.Zone) state.PlayerID {
	if z == state.ZBattlefield || z == state.ZStack {
		return o.Controller
	}
	return o.Owner
}

func remove(g *state.Game, id state.ObjID, z state.Zone, p state.PlayerID) {
	src := g.Zone(z, p)
	for i, x := range src {
		if x == id {
			out := make([]state.ObjID, 0, len(src)-1)
			out = append(out, src[:i]...)
			out = append(out, src[i+1:]...)
			g.SetZone(z, p, out)
			return
		}
	}
}

// applyPair writes a Soulbond pairing (CR 702.103): both permanents' Paired
// fields are set to each other's id when both are on the battlefield. Neither
// half is written for an object that has left the battlefield (its own Move
// already reset it).
func applyPair(g *state.Game, srcID, partnerID state.ObjID) {
	src := g.Obj(srcID)
	partner := g.Obj(partnerID)
	if src != nil && src.Zone == state.ZBattlefield && partner != nil && partner.Zone == state.ZBattlefield {
		src.Paired = partnerID
		partner.Paired = srcID
	}
}
