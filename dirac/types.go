package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// CardID indexes the matchup's card table (kop.Table.Cards); 255 = none.
type CardID = kop.CardID

// Table is the card table the kernel plays (hand-written or generated).
type Table = kop.Table

// Slot is a battlefield slot index; 255 = none.
type Slot uint8

// Obj is a card instance (object) index into State.objs; 255 = none. The
// first len(deckA) objects are seat 0's deck in deck order, the next
// len(deckB) seat 1's; tokens take the remaining indices.
type Obj uint8

const (
	NoSlot Slot = 255
	NoObj  Obj  = 255
	// PlayerTarget0.. encode a player in an Action's target byte and in
	// stack target records; PlayerTarget0 and StackTarget0 are set per size
	// class (seats_default.go, seats_mp.go). PlayerTarget1 is seat 1's code;
	// PlayerCode(p) is the spelling that holds at every seat count.
	PlayerTarget1 uint8 = PlayerTarget0 + 1
	NoTarget      uint8 = 0xFF
	// StackTarget0 encodes the activated or triggered ability at stack
	// index i (from the bottom) as StackTarget0+i in a target byte, the
	// oracle's canonical RefStack+i (tgt_stackability.go): codes
	// below MaxObj are objects, StackTarget0..PlayerTarget0-1 stack
	// abilities, PlayerTarget0.. players (0xFE is a departed stack
	// target, 0xFF none).
)

// isObjTarget reports whether target byte c names an object (a card or
// token instance), not a stack ability, a player or no target. Every
// object-target read goes through it (TestObjTargetPredicate).
func isObjTarget(c uint8) bool { return c < MaxObj }

// isPlayerTarget reports whether target byte c names a player, not an
// object, a stack ability or no target. A raw "c >= PlayerTarget0" also
// admits NoTarget, so the player-range test goes through here.
func isPlayerTarget(c uint8) bool { return c >= PlayerTarget0 && c <= PlayerTarget0+(MaxSeats-1) }

// IsObjTarget is isObjTarget for the kernel's clients (the search bots, the
// feature encoders, the oracle).
func IsObjTarget(c uint8) bool { return c < MaxObj }

// Action is a packed answer to a Decision:
//
//	kind<<24 | source<<16 | target<<8 | mode
//
// source and target are Obj indices (or PlayerTarget*), mode an
// action-specific small value. See ActionKind for each kind's fields.
type Action uint32

// ActionKind is an Action's kind.
type ActionKind uint8

const (
	AkPass         ActionKind = iota // pass priority
	AkPlayLand                       // source = land object (hand, or exile for a land-front Adventure, CR 715.3d)
	AkCast                           // source = card object (hand, or graveyard for flashback); mode bit0 = kicked, bit1 = flashback; bit 2 a face cast, target = the face (face_castface.go)
	AkActivate                       // source = permanent object; mode = ability index in its CardDef
	AkAttack                         // source = creature object declared as an attacker
	AkBlock                          // source = blocker object, target = attacker object
	AkDone                           // finish the current multi-step declaration (attackers / blockers)
	AkTarget                         // target = chosen object or PlayerTarget*; NoTarget = "no target" for an up-to slot
	AkMode                           // mode = chosen mode index
	AkChoose                         // mode = chosen value (yes=1/no=0, a number, a colour); source = chosen object when choosing a card
	AkKeep                           // keep the opening hand
	AkMulligan                       // mulligan
	AkOrder                          // trigger order: source = the trigger's source Obj, target = its pending-trigger index, mode = its occurrence index among pending triggers from that source
	AkTurnFaceUp                     // source = face-down permanent object; special action (CR 708.6)
	AkPlot                           // source = a hand card; the plot special action (CR 702.170a, 116.2k)
	AkUnlock                         // source = a Room permanent, mode = the door (CR 709.5e, 116.2k; face_rooms.go)
	AkCastCasualty                   // source = a hand card; the casualty cast (CR 702.153a, kw_casualty.go)
	AkSneak                          // source = a hand card; cast it for its sneak cost (CR 702.190a; kw_sneak.go)
	AkBestow                         // source = a hand card; cast it bestowed, for its bestow cost (CR 702.103a; kw_bestow.go)
	AkWebSling                       // source = a hand card; cast it for its web-slinging cost (CR 702.188a; kw_websling.go)
	AkMayFlash                       // source = a hand card; cast it as though it had flash for its mana cost plus the extra (CR 702.8; kw_mayflash.go)
	AkCastConspire                   // source = a hand card; the conspired cast of a spell with granted conspire (CR 702.78a; kw_conspire.go)
	AkCount
)

// isCastKind reports whether k is a cast action (plain or casualty).
func isCastKind(k ActionKind) bool { return k == AkCast || k == AkCastCasualty || k == AkCastConspire }

// scriptPlayKinds is the set of kinds a scripted activation can fund: the
// casts, an activation, a plot and a sneak (script_gen.go); one mask test.
const scriptPlayKinds = 1<<AkCast | 1<<AkCastCasualty | 1<<AkCastConspire | 1<<AkActivate | 1<<AkPlot | 1<<AkSneak

func isScriptPlayKind(k ActionKind) bool { return scriptPlayKinds>>k&1 != 0 }

// MakeAction packs an Action.
func MakeAction(k ActionKind, source, target, mode uint8) Action {
	return Action(uint32(k)<<24 | uint32(source)<<16 | uint32(target)<<8 | uint32(mode))
}

func (a Action) Kind() ActionKind { return ActionKind(a >> 24) }
func (a Action) Source() uint8    { return uint8(a >> 16) }
func (a Action) Target() uint8    { return uint8(a >> 8) }
func (a Action) Mode() uint8      { return uint8(a) }

// DecisionKind is what a Decision asks.
type DecisionKind uint8

const (
	DkNone         DecisionKind = iota // the game is over
	DkPriority                         // pass / play a land / cast / activate
	DkAttackers                        // add an attacker or finish
	DkBlockers                         // add a block or finish
	DkTarget                           // choose a target for the spell/ability being put on the stack
	DkModes                            // choose a mode of a modal spell
	DkChoose                           // a yes/no, number, colour or card choice
	DkTriggerOrder                     // choose which simultaneous trigger goes on the stack next
	DkMulligan                         // keep or mulligan
)

// MaxOptions bounds a Decision's options: an any-target choice over a full
// battlefield, and a creature-type choice over gorge's 388 creature types
// (the Star Trek words took the vocabulary past the former 384).
const MaxOptions = 512

// ChoiceKind says what a DkChoose decision (and its AkChoose picks) chooses.
type ChoiceKind uint8

const (
	ChNone           ChoiceKind = iota
	ChX                         // mode = the value of X
	ChCostDiscard               // source = a hand card discarded to pay a cost
	ChCostSacrifice             // source = a permanent sacrificed to pay a cost
	ChCostTap                   // source = an untapped creature tapped to pay a cost (tapXType)
	ChBottom                    // source = a hand card put on the bottom (London mulligan)
	ChDiscard                   // source = a hand card discarded by an effect
	ChSacrifice                 // source = a permanent sacrificed by an effect
	ChScry                      // source = the looked-at card; mode 1 = keep on top, 0 = bottom
	ChSurveil                   // source = the looked-at card; mode 1 = graveyard, 0 = keep on top
	ChArrange                   // source = the kept card to put next from the top (scry/surveil order)
	ChOptional                  // mode 1 = yes / 0 = no: an optional ("you may") triggered ability at resolution
	ChWardPay                   // mode 1 = pay / 0 = don't: a ward cost
	ChLegend                    // source = the legendary permanent to keep (legend rule)
	ChDivide                    // source = a blocker, mode = combat damage assigned to it
	ChCleanupDiscard            // source = a hand card discarded to hand size in cleanup
	ChAltAdd                    // target = the either-or additional cost part (0 the spell's own, 1 the alternative), mode = option position (payable parts first)
	ChCostSubCtr                // source = a creature to remove one counter from to pay a cost, mode = its counter kind (Quilled Greatwurm)
	ChCostBlight                // source = a creature to blight to pay a cost (CR 701.68)
	ChOptDiscMay                // mode 1 = yes / 0 = no: a discard election ("you may discard") (act_optionalask.go)
	ChCommanderZone             // source = the commander, mode 0 = command zone / 1 = leave it (CR 903.9a/b; zone_cmdzone.go)
	ChVoteCard                  // source = the permanent voted for (gorge effCardVote's KChoose option kind "vote_card", CR 701.38a; act_vote.go)
)

// ChMultikick is CR 702.43's multikicker count ask (gorge KChoose "multikick",
// mode = the count; kw_multikicker.go). Explicit ordinal, the iota tail stays
// untouched.
const ChMultikick ChoiceKind = 254

// Decision is the next question the game asks a player.
type Decision struct {
	Kind    DecisionKind
	Sub     ChoiceKind // for DkChoose: what is chosen
	Player  uint8
	N       uint16
	Options [MaxOptions]Action
}

func (d *Decision) add(a Action) {
	if int(d.N) >= MaxOptions {
		panic("kernel: decision option overflow")
	}
	d.Options[d.N] = a
	d.N++
}

// remove drops option a (order kept).
func (d *Decision) remove(a Action) {
	for i := 0; i < int(d.N); i++ {
		if d.Options[i] == a {
			copy(d.Options[i:d.N], d.Options[i+1:d.N])
			d.N--
			return
		}
	}
}

// canonKey is a's sort key in the oracle's canonical option order (kernel
// /oracle canon.go: kind<<24|src<<16|tgt<<8|mode over the canonical kinds,
// which order like the kernel's): the raw packing except where the
// canonical form packs differently.
func (s *State) canonKey(d *Decision, a Action) uint32 {
	switch a.Kind() {
	case AkChoose:
		switch d.Sub {
		case ChDiscard: // canonical (Mode, -, -, hand position)
			pl := &s.pl[s.objs[a.Source()].owner()]
			for i := 0; i < int(pl.handN); i++ {
				if pl.hand[i] == Obj(a.Source()) {
					return uint32(AkChoose)<<24 | uint32(i)
				}
			}
		case ChAltAdd: // canonical value = option position
			return uint32(AkChoose)<<24 | uint32(a.Mode())
		case ChOptional: // canonical Yes before No
			return uint32(AkChoose)<<24 | uint32(1-a.Mode()) // seat-ok: a yes/no option, not a seat
		case ChOptDiscMay: // canonical Yes before No
			return uint32(AkChoose)<<24 | uint32(1-a.Mode()) // seat-ok: a yes/no option, not a seat
		case ChGenDigOpt: // canonical Yes before No
			return uint32(AkChoose)<<24 | uint32(1-a.Mode()) // seat-ok: a yes/no option, not a seat
		}
	case AkOrder: // (Order, source, -, occurrence)
		return uint32(a) & 0xffff00ff
	}
	return uint32(a)
}

// sortCanonical orders d's options canonically (stable insertion sort; the
// option lists are short).
func (s *State) sortCanonical(d *Decision) {
	var keys [MaxOptions]uint32
	n := int(d.N)
	for i := 0; i < n; i++ {
		keys[i] = s.canonKey(d, d.Options[i])
	}
	for i := 1; i < n; i++ {
		k, a := keys[i], d.Options[i]
		j := i
		for j > 0 && keys[j-1] > k {
			keys[j], d.Options[j] = keys[j-1], d.Options[j-1]
			j--
		}
		keys[j], d.Options[j] = k, a
	}
}
