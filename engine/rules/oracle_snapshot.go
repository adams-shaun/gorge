package rules

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/rules/chars"
	"github.com/adams-shaun/gorge/engine/state"
)

// OracleSnapshot is the engine-neutral observable state of an oracle
// scenario at one checkpoint (spec 2026-10-02-xmage-compliance-oracle-design
// §7). The XMage driver emits the same shape, so the comparator diffs the
// two field by field. Building one never writes game state.
type OracleSnapshot struct {
	Checkpoint string              `json:"checkpoint"`
	Turn       int32               `json:"turn"`
	Step       string              `json:"step"`
	Active     int                 `json:"active"`
	Priority   int                 `json:"priority"`
	Over       bool                `json:"over"`
	Winner     *int                `json:"winner,omitempty"`
	Players    []OracleSnapPlayer  `json:"players"`
	Permanents []OracleSnapPerm    `json:"permanents"`
	Stack      []OracleSnapStack   `json:"stack"`
	Offered    []OracleSnapOffered `json:"offered,omitempty"`
}

type OracleSnapOffered struct {
	Source string `json:"source"`
	Kind   string `json:"kind"`
	Label  string `json:"label,omitempty"`
}

// OracleSnapPlayer is one seat's public and (scenarios are omniscient)
// hidden state.
type OracleSnapPlayer struct {
	Seat         int              `json:"seat"`
	Life         int32            `json:"life"`
	Counters     map[string]int32 `json:"counters,omitempty"`
	Hand         []string         `json:"hand"`      // sorted
	Graveyard    []string         `json:"graveyard"` // zone order
	Exile        []string         `json:"exile"`     // sorted
	Command      []string         `json:"command"`   // sorted
	LibraryCount int              `json:"library_count"`
	LibraryTop   []string         `json:"library_top"` // top first, at most oracleLibraryTopN
	Pool         string           `json:"pool"`        // WUBRGC letters
	// Lost marks a seat that has lost the game (CR 104.3). Only a game of
	// three or more seats goes on past it (CR 800.4a), so it is false in
	// every two-seat checkpoint but a game-over one.
	Lost bool `json:"lost,omitempty"`
}

// OracleSnapPerm is one battlefield permanent. Ref is the object's scenario
// ref (objRef): the one a scenario step would use to name it.
type OracleSnapPerm struct {
	Ref              string           `json:"ref"`
	Name             string           `json:"name"`
	Controller       int              `json:"controller"`
	Owner            int              `json:"owner"`
	Token            bool             `json:"token,omitempty"`
	Tapped           bool             `json:"tapped,omitempty"`
	FaceDown         bool             `json:"face_down,omitempty"`
	PT               string           `json:"pt,omitempty"` // creatures only
	Damage           int32            `json:"damage,omitempty"`
	Counters         map[string]int32 `json:"counters,omitempty"`
	Types            []string         `json:"types"` // sorted
	AllCreatureTypes bool             `json:"all_creature_types,omitempty"`
	Colors           string           `json:"colors"`
	Keywords         []string         `json:"keywords,omitempty"` // sorted
	AttachedTo       string           `json:"attached_to,omitempty"`
	Attacking        bool             `json:"attacking,omitempty"`
	Blocking         bool             `json:"blocking,omitempty"`
}

// OracleSnapStack is one stack object, listed top first.
type OracleSnapStack struct {
	Kind       string `json:"kind"`   // "spell" or "ability"
	Source     string `json:"source"` // ref of the spell, or of the ability's source
	Controller int    `json:"controller"`
	// Trigger is the index into the source face's Triggers of the printed
	// trigger that put this ability on the stack ("" for a spell, an
	// activated ability, or a trigger that is not one of the face's own).
	// It lets the generator tell which of a card's triggers fired. The
	// comparator does not read it.
	Trigger string `json:"trigger,omitempty"`
}

// OracleDecision is one non-priority decision the scenario answered.
// Step is the scenario step that posed it (-1 during setup); PickIdx and
// PickRefs name the picks by option index and by object ref ("pN" for a
// player), so the generator can script the same answer for XMage.
type OracleDecision struct {
	Step     int      `json:"step"`
	Seat     int      `json:"seat"`
	Kind     string   `json:"kind"` // target, yesno, mode, choose_n, order, attackers, blockers
	Options  int      `json:"options"`
	Picks    []string `json:"picks"`
	PickIdx  []int    `json:"pick_idx"`
	PickRefs []string `json:"pick_refs"`
	// PickRefsInexact is parallel to PickRefs: true when the ref was derived
	// by counting live objects rather than by identity -- an anonymous
	// library/hand card that was never named at setup, whose ordinal
	// XMage's zone-order alias binding cannot reproduce. Omitted or false
	// means the ref is a setup identity or a token, which the driver binds
	// exactly.
	PickRefsInexact []bool `json:"pick_refs_inexact,omitempty"`
	// OptionRefs records game-object identities among the offered options. It
	// lets downstream answer adapters disambiguate same-named object picks.
	OptionRefs []string `json:"option_refs,omitempty"`
	// OptionGroups records the non-empty decision.Option.Group of each offered
	// option, in option order. Only the engine's target asks set a Group
	// (rules' askTarget/cast.go and effects' poseTargetsAsk, all
	// "target-controller-<seat>"), so a recorded group tells a downstream
	// adapter that a KChoose is really a target ask -- a mid-resolution
	// TargetsForEachPlayer$ ask (Kaya, Spirits' Justice's exile-each) poses one
	// XMage target per opponent even though the engine records it as a choose.
	OptionGroups []string `json:"option_groups,omitempty"`
	// ArrangeLabels is the label of every offered option of a KArrange, in
	// option order, and ArrangeKind the destination Kind they all share ("" if
	// they disagree). An answered arrange records only the kept picks, so an
	// EMPTY answer (every looked-at card to the graveyard) would otherwise
	// leave the generator no card names to script for XMage's selection.
	ArrangeLabels []string `json:"arrange_labels,omitempty"`
	ArrangeKind   string   `json:"arrange_kind,omitempty"`
	// ObjectPicks records only selected game-object identities, in submission
	// order. Unlike a snapshot delta, these are the objects the player chose.
	ObjectPicks []string `json:"object_picks,omitempty"`
	// PickKinds is the engine option kind of each pick (parallel to Picks),
	// so the generator can tell a card pick XMage poses as a target from a
	// labelled pick XMage poses as a makeChoose choice without guessing from
	// the label text.
	PickKinds []string `json:"pick_kinds,omitempty"`
	// Resume is the decision's ResumeKind: it tells the controller-facing
	// which-opponent ask ("opp_pick", XMage's ChoicePlayer) from a player
	// target, which share the option kind "player".
	Resume string `json:"resume,omitempty"`
	Via    string `json:"via"` // how the runner answered: target, answer, or a fallback
	// GorgeKind and First let a generator script the same decision for
	// gorge's runner: the raw decision kind and option 0's label.
	GorgeKind string `json:"gorge_kind"`
	First     string `json:"first,omitempty"`
	Min       int    `json:"min"`
	Max       int    `json:"max"`
	// PerPlayer marks a target ask shaped by Forge's TargetsForEachPlayer$
	// (CR 601.2c): XMage poses one target for EACH player in seat order, so
	// the generator must answer one seat at a time. SeatCount is the number
	// of seats in the match, the number of asks XMage makes. Both are false/
	// zero for every ordinary target ask, so the generator's output is
	// unchanged for them.
	PerPlayer bool `json:"per_player,omitempty"`
	SeatCount int  `json:"seat_count,omitempty"`
	// Retarget marks a ChangeTargets redirect pick (CR 115.7): a choose ask
	// whose options are the legal NEW targets. XMage poses it as a
	// chooseTarget on the target queue, not a makeChoose dialog, so the
	// generator answers it as a target. False for every other choose ask.
	Retarget bool `json:"retarget,omitempty"`
	// XMulti marks an X announcement of a cost naming X more than once
	// (Gogo's "XMin1 X X", {X}{X}): XMage's unscripted announce does not pay
	// the repeated X even when the pool leaves one legal value, so it must be
	// scripted.
	XMulti bool `json:"x_multi,omitempty"`
	// PerOpponent marks a per-player target ask whose filter admits only
	// opponents' objects: XMage asks no target for the controller's seat.
	PerOpponent bool `json:"per_opponent,omitempty"`
	// Divided is the literal DividedAsYouChoose$ total of a target ask whose
	// ability divides its amount among the targets (distribute counters,
	// divided damage). XMage poses that slot as a TargetAmount and needs the
	// amount with each target, even a lone one gorge never split.
	Divided int `json:"divided,omitempty"`
	// UnposedSlots counts the "up to N" target slots of a triggered ability's
	// chain that the engine settled with an empty answer right after this
	// decision, without posing them (no legal candidate). XMage still asks
	// each such slot, so the generator closes it with a target skip.
	UnposedSlots int `json:"unposed_slots,omitempty"`
	// LeadingUnposed counts the "up to N" slots of a triggered ability's chain
	// the engine settled empty BEFORE this, the chain's first posed ask (the
	// root slot with no candidate, or a leading link). XMage asks them first,
	// so the generator closes each with a target skip ahead of the picks.
	LeadingUnposed int `json:"leading_unposed,omitempty"`
	// CopyInherited counts the inherited targets of the declaration a
	// "copy_targets" ask (CR 707.10c) is retargeting. XMage poses one
	// "Change this N of M target?" chooseUse per inherited target before the
	// new-target pick, so the generator needs the count to emit the booleans;
	// gorge offers those slots as the ask's leading keep-current options, so a
	// pick index below the count keeps that slot. Omitted (zero) for every
	// other decision, leaving its JSON byte-identical.
	CopyInherited int `json:"copy_inherited,omitempty"`
	// AltPayable counts the options of an AlternateAdditionalCost either-or
	// ask (option kind "altaddcost") the cast could pay. XMage's OrCost poses
	// its chooseUse only when two or more of its costs can be paid, so the
	// generator scripts the boolean only for AltPayable >= 2.
	AltPayable int `json:"alt_payable,omitempty"`
	// TriggerCostMana marks a triggered "you may pay <cost>" ask whose cost
	// is mana only. XMage's DoIfCostPaid poses its chooseUse for a mana cost
	// even when it cannot be paid (gorge then offers only the decline), but
	// not for an unpayable non-mana cost (a forage, a sacrifice), so the
	// generator scripts a decline-only ask's "no" only when this is set.
	TriggerCostMana bool `json:"trigger_cost_mana,omitempty"`
	// ManaColours is the offered colour set of a mana ask whose XMage side is
	// an AddManaInAnyCombinationEffect dialog (the engine's Produced$ "Combo
	// <colours>"): one multi-amount message per colour, in the set's own
	// order, whatever the unit count — a count-1 restricted set included,
	// which is the shape XMage's DynamicManaEffect (Produced$ Any) answers
	// with a plain colour dialog instead (agent 20261009T041408Z cluster C2).
	// Nil for every other ask, including a produced-Any one, so the
	// generator's existing routing is untouched.
	ManaColours []string `json:"mana_colours,omitempty"`
	// CostPosed marks a triggered optional cost's decline-only ask (option
	// kind "trigger_cost_decline" alone: gorge could not pay) whose cost XMage
	// still asks about. XMage's DoIfCostPaid skips its chooseUse only when the
	// cost's canPay is false, and a mana cost ("it's true all the time") or a
	// discard-your-hand cost always passes, so its AI answers yes and pays
	// what gorge declined. A decline-only ask over any other cost stays
	// unmarked: XMage's canPay is false there too and it poses nothing.
	CostPosed bool `json:"cost_posed,omitempty"`
}

// OracleFlip is one canonical coin-flip result the run's seeded RNG produced,
// read off the engine log's flip Note (effects.FlipNoteResult), in log order.
// Step is the scenario step whose window the flip fell in; -1 is the setup
// drive's window (a flip the drive walked into before the first gameplay
// step, e.g. Mana Crypt's turn-1 upkeep trigger). Turn is the turn the
// runner was on when the window's scan ran; Seat the flipper; Win the side
// (heads wins). The generator scripts each flip for XMage's choice queue, so
// the two engines do not each flip on their own seed.
type OracleFlip struct {
	Turn int  `json:"turn"`
	Step int  `json:"step"`
	Seat int  `json:"seat"`
	Win  bool `json:"win"`
}

const oracleLibraryTopN = 5

// oracleDecisionKind normalizes a decision kind to the cross-engine
// vocabulary; ok is false for kinds that are not compared.
func oracleDecisionKind(k decision.Kind) (string, bool) {
	switch k {
	case decision.KPriority, decision.KMulligan, decision.KStartingPlayer:
		return "", false
	case decision.KTriggerOptional, decision.KCommanderZone:
		return "yesno", true
	case decision.KModes:
		return "mode", true
	case decision.KChoose:
		return "choose_n", true
	case decision.KTriggerOrder, decision.KArrange, decision.KReplacement:
		return "order", true
	}
	return string(k), true
}

func (r *oracleRun) snapCounters(cs []state.Counter) map[string]int32 {
	if len(cs) == 0 {
		return nil
	}
	m := map[string]int32{}
	for _, c := range cs {
		m[normCounter(c.Kind)] += c.N
	}
	return m
}

func (r *oracleRun) zoneNames(z state.Zone, p state.PlayerID, sorted bool) []string {
	ids := r.e.G.Zone(z, p)
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		out = append(out, r.objName(r.e.G.Obj(id)))
	}
	if sorted {
		sort.Strings(out)
	}
	return out
}

// snapshot builds the checkpoint. Read-only: it calls only accessors.
func (r *oracleRun) snapshot(checkpoint string) OracleSnapshot {
	e, g := r.e, r.e.G
	s := OracleSnapshot{
		Checkpoint: checkpoint, Turn: g.Turn, Step: g.Step.String(),
		Active: int(g.Active), Priority: int(g.Priority), Over: g.Over,
		Players: []OracleSnapPlayer{}, Permanents: []OracleSnapPerm{}, Stack: []OracleSnapStack{},
	}
	if g.Over && !g.Draw {
		w := int(g.Winner)
		s.Winner = &w
	}
	for i := range g.Players {
		p := state.PlayerID(i)
		lib := g.Zone(state.ZLibrary, p)
		top := make([]string, 0, oracleLibraryTopN)
		for j := 0; j < len(lib) && j < oracleLibraryTopN; j++ {
			top = append(top, r.objName(g.Obj(lib[j])))
		}
		s.Players = append(s.Players, OracleSnapPlayer{
			Seat: i, Life: g.Players[i].Life, Counters: r.snapCounters(g.Players[i].Counters),
			Hand: r.zoneNames(state.ZHand, p, true), Graveyard: r.zoneNames(state.ZGraveyard, p, false),
			Exile: r.zoneNames(state.ZExile, p, true), Command: r.zoneNames(state.ZCommand, p, true),
			LibraryCount: len(lib), LibraryTop: top, Pool: poolString(g.Players[i].Pool),
			Lost: g.Players[i].Lost,
		})
	}
	var field []state.ObjID
	for i := range g.Players {
		field = append(field, g.Zone(state.ZBattlefield, state.PlayerID(i))...)
	}
	sort.Slice(field, func(a, b int) bool { return field[a] < field[b] })
	blocking := map[state.ObjID]bool{}
	for _, id := range field {
		for _, b := range g.Obj(id).BlockedBy {
			blocking[b] = true
		}
	}
	for _, id := range field {
		o := g.Obj(id)
		derived := e.Derived(id)
		types := append([]string(nil), derived.Types...)
		sort.Strings(types)
		kws := append([]string(nil), e.Keywords(id)...)
		sort.Strings(kws)
		p := OracleSnapPerm{
			Ref: r.objRef(o), Name: r.fieldName(o), Controller: int(o.Controller), Owner: int(o.Owner),
			Token: o.IsToken, Tapped: o.Tapped, FaceDown: o.FaceDown, Damage: o.Damage,
			Counters: r.snapCounters(o.Counters), Types: types, AllCreatureTypes: derived.AllCreatureTypes,
			Colors: e.Colors(id), Keywords: kws,
			Attacking: o.IsAttacking, Blocking: blocking[id] || o.IsBlocking,
		}
		if oracleHasFold(types, "Creature") {
			p.PT = fmt.Sprintf("%d/%d", e.Power(id), e.Toughness(id))
		}
		switch {
		case o.AttachedTo != 0:
			p.AttachedTo = r.objRef(g.Obj(o.AttachedTo))
		case o.HasAttachedPlayer:
			p.AttachedTo = fmt.Sprintf("p%d", o.AttachedPlayer)
		}
		s.Permanents = append(s.Permanents, p)
	}
	if d := e.Pending(); d != nil && d.Kind == decision.KPriority {
		s.Offered = oracleSnapshotOffers(r, d)
		sort.Slice(s.Offered, func(i, j int) bool {
			a, b := s.Offered[i], s.Offered[j]
			if a.Source != b.Source {
				return a.Source < b.Source
			}
			if a.Kind != b.Kind {
				return a.Kind < b.Kind
			}
			return a.Label < b.Label
		})
	}
	for i := len(g.Stack) - 1; i >= 0; i-- {
		o := g.Obj(g.Stack[i])
		if o == nil {
			continue
		}
		it := OracleSnapStack{Kind: "spell", Controller: int(o.Controller)}
		src := o
		if o.Ability != nil {
			it.Kind = "ability"
			src = g.Obj(o.Source)
			it.Trigger = stackTriggerSlot(src, o.Ability)
		}
		it.Source = stackSourceRef(r.objRef(src), src)
		if o.Ability != nil && src != nil && src.Zone == state.ZBattlefield && src.FaceDown {
			// An ability whose source is a face-down permanent (a granted
			// one: Etrata's turn-face-up grant) names no source card: the
			// face-down permanent has no name (CR 708.2a), as its own
			// snapshot line shows, so the ref keeps only its seat.
			if i := strings.IndexByte(it.Source, ':'); i >= 0 {
				it.Source = it.Source[:i+1]
			}
		}
		s.Stack = append(s.Stack, it)
	}
	return s
}

// stackTriggerSlot is the index of the face trigger whose compiled effect is
// sa (the pointer TriggerPush mints, as findTriggerForAbilityFace matches it),
// or "" when sa is not one of src's face triggers.
func stackTriggerSlot(src *state.Object, sa *cards.SA) string {
	// printedFaceTrigger also finds a back-face line of a double-faced source
	// that is already front face up again (a "when this dies" trigger pushed
	// from the look-back board, CR 712.8a).
	if _, _, _, idx, ok := printedFaceTrigger(src, sa); ok {
		return strconv.Itoa(idx)
	}
	return ""
}

// stackSourceRef is ref (objRef of o) for a stack item's source, except that a
// battlefield permanent standing on a face other than its first reports that
// face's name under its setup ref's seat prefix and "#n" ordinal: the setup
// ref names the front face, but the ability on the stack is the back face's
// (a "back_face" setup permanent), exactly as the XMage driver names it.
//
// A Room spell on the stack is the same case (CR 709.5, 601.2): it has only
// the characteristics of the door it was cast as, so the setup ref's whole
// "Left // Right" name is replaced by the cast door's name, whichever door
// that is (FaceIdx 0 included).
//
// An Adventure spell on the stack is the same case (CR 715.3b, 715.4), and an
// Omen spell is its Omen twin (CR 720.3b, 720.4): the card was cast as its
// alternate face and has only that face's characteristics, so XMage names the
// stack object by the alternate face's own name ("Stomp", "Charring Bite"),
// not the card's.
func stackSourceRef(ref string, o *state.Object) string {
	if o == nil || o.CopyFace != nil || o.Face() == nil {
		return ref
	}
	onStackRoom := o.Zone == state.ZStack && o.Ability == nil &&
		(chars.IsRoomCard(o) || adventureSpellFace(o) == o.Face() || omenSpellFace(o) == o.Face())
	if !onStackRoom && (o.Zone != state.ZBattlefield || o.FaceIdx == 0) {
		return ref
	}
	seat, nth := "", ""
	if i := strings.IndexByte(ref, ':'); i >= 0 {
		seat = ref[:i+1]
		ref = ref[i+1:]
	}
	if j := strings.LastIndexByte(ref, '#'); j >= 0 {
		nth = ref[j:]
	}
	return seat + o.Face().Name + nth
}

// objRef names o by the ref resolve maps back to o right now, so a snapshot
// ref is the object's identity, never its position on the battlefield:
//
//   - a card named at setup keeps its setup ref ("p1:Grizzly Bears#2") in
//     every zone, whoever controls it;
//   - any other card is "p<owner>:<name>[#k]", k its rank in ObjID order
//     among the owner's live objects of that name (resolve's own fallback;
//     card objects never cease, so k never changes);
//   - a token is "p<controller>:token:<name>[#k]", k its rank among that
//     seat's battlefield tokens whose name contains <name> (resolve's token
//     rule). Tokens are positional, as in scenarios; the comparator matches
//     them by characteristics (spec section 7);
//   - a PENDING ability object on the stack is
//     "p<controller>:ability:<source name>[#k]", k its rank in ObjID order
//     among that controller's pending ability objects with the same source
//     name (resolve's ability rule). Only a pending ability is offered as a
//     target (CR 115.5), so once the ability leaves the stack the ref names
//     nothing -- the same lifetime a spell ref's stack entry has.
func (r *oracleRun) objRef(o *state.Object) string {
	if o == nil {
		return ""
	}
	if o.Ability != nil {
		name := r.objName(r.e.G.Obj(o.Source))
		seat := o.Controller
		norm := cards.NormalizeName(name)
		k := 0
		for i := range r.e.G.Objs {
			c := &r.e.G.Objs[i]
			if c.Zone != state.ZStack || c.Ability == nil || c.Controller != seat ||
				cards.NormalizeName(r.objName(r.e.G.Obj(c.Source))) != norm {
				continue
			}
			k++
			if c.ID == o.ID {
				break
			}
		}
		base := fmt.Sprintf("p%d:ability:%s", seat, name)
		if k > 1 {
			return fmt.Sprintf("%s#%d", base, k)
		}
		return base
	}
	bound := ""
	for ref, id := range r.refs {
		if id == o.ID && (bound == "" || ref < bound) {
			bound = ref
		}
	}
	if bound != "" {
		return bound
	}
	name := r.objName(o)
	lower := strings.ToLower(name)
	seat, k := o.Owner, 0
	if o.IsToken {
		seat = o.Controller
	}
	for i := range r.e.G.Objs {
		c := &r.e.G.Objs[i]
		if c.Zone == state.ZCeased || c.Ability != nil || c.Face() == nil {
			continue
		}
		if o.IsToken {
			if !c.IsToken || c.Controller != seat || c.Zone != state.ZBattlefield || !strings.Contains(strings.ToLower(c.Face().Name), lower) {
				continue
			}
		} else if c.Owner != seat || c.Face().Name != name {
			continue
		}
		k++
		if c.ID == o.ID {
			break
		}
	}
	base := fmt.Sprintf("p%d:%s", seat, name)
	if o.IsToken {
		base = fmt.Sprintf("p%d:token:%s", seat, name)
	}
	ref := base
	if k > 1 {
		ref = fmt.Sprintf("%s#%d", base, k)
	}
	// A setup ref of the same spelling bound to another object would shadow
	// this one in resolve; "#1" is never bound at setup and still resolves
	// positionally.
	if id, ok := r.refs[ref]; ok && id != o.ID && k == 1 {
		ref = base + "#1"
	}
	return ref
}
