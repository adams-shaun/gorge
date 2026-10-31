package neutral

import (
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/bots/botpolicy"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/rules"
	"github.com/adams-shaun/gorge/engine/rules/combat"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/engine/view"
	"github.com/adams-shaun/gorge/server/spellbench/mapping"
	"github.com/adams-shaun/gorge/server/spellbench/protocol"
	"github.com/adams-shaun/gorge/server/spellbench/xview"
)

// Untranslated is the op of a candidate the neutral world cannot represent:
// a family it has no translation for, or a card whose printed facts its
// observation cannot rebuild (a token copy, say). No plan ever matches it.
const Untranslated = "untranslated"

// noOp is the op of a candidate no gorge option stands for (a mana
// activation no payment plan takes, or a family not translated yet).
var noOp = mapping.NativeOp{Op: Untranslated, Option: -1}

// native is the gorge decision the seat is answering, kept across the v2
// decisions that spend it: the substeps of a group, the later picks of a
// variable selection, or a cast's choose_cast_method.
type native struct {
	index   uint64
	key     string
	d       decision.Decision
	options map[string]int    // option key -> option index
	casts   map[string][]int  // priority: cast source v2 id -> its cast options
	names   map[string]string // priority: cast source v2 id -> its card name
	arrange *arrangement      // an arrangement's partition and order so far
	follow  map[string]decision.Decision
	pays    map[string]*planned                   // priority: payment action id -> its lowering
	paid    map[string]*decision.PaymentSelection // priority: card name -> the plan paying its cast
}

// arrangement is a scry-like native decision spent over 2n-1 v2 decisions.
type arrangement struct {
	cards   []string // looked-at card v2 ids, top first
	other   string   // the destination that is not the top
	dest    []string // the partition answered so far, per card
	placed  []string // cards placed by ordering picks
	native  []int    // dig: each card's option in the dig ask, -1 for none
	primary string   // dig: the v2 destination of the take
	rest    string   // dig: the v2 destination of the remainder
	fixed   bool     // dig: gorge's engine moves the cards without asking
	random  bool     // dig: RestRandomOrder$ shuffles the remainder, so no order ask
}

// Session translates one seat's decisions in one game.
type Session struct {
	B    *Builder
	seat state.PlayerID
	next uint64
	cur  *native
	last *protocol.SeatDecision
	// lastReasons is the per-candidate diagnosis of the last Payload, parallel
	// to its Ops: why the neutral world left a candidate untranslated.
	lastReasons []string
	Counts      map[string]int // untranslated decisions by first candidate kind (audit)
	// Approx counts decisions translated by an approximation rather than a
	// gorge ask (choose_pile's first-pile stand-in). It is deliberately
	// separate from Counts, which is the untranslated audit and the budget
	// TestNeutralUntranslatedBudget holds to 0.5%.
	Approx map[string]int
	// activated is the face ability index of each source's latest
	// activation, so its target ask can describe the ability.
	activated map[string]int
	// grants records each source's answered activations in order, oldest
	// first: the pure-grant keywords, or nil for an activation that was not a
	// pure grant. priority trims the list to the activations still on the
	// observed stack, so a repeated activation of any pending grant can be
	// reported as the engine's redundant-grant fact (decision.Grant). A source
	// whose ability has left the stack is dropped at the next priority
	// decision, the only kind whose Payload may prune: a cost-target or target
	// decision for a pending activation arrives before the ability is on the
	// stack, so pruning there would forget the grant.
	grants map[string][][]string
	// uses counts this turn's activations of each source, which gorge's
	// view carries as ActivatedThisTurn and its bot budgets: each activated
	// ability seen on the stack counts once for its source.
	uses     map[string]int32
	usesTurn uint32
	seen     map[string]bool // activated-ability stack ids already counted
	// pendingUses is the activation census's reversal check: a counted
	// activation whose source was tapped at the time. When its stack object
	// leaves and the source is untapped, the activation was reversed (CR
	// 733.1, an unpayable target/cost) and the engine's AbilityPush census
	// was folded back; the neutral count must follow.
	pendingUses map[string]pendingUse
	pend        *pending // a payment plan being lowered over priority decisions
	// autoPay is the host's declared engine_autopay (Section 7.1). A host
	// that does not declare it poses mana abilities as candidates and pays
	// them by the lowering on those candidates; it never takes a plan for a
	// plan-only cast. The zero value is the manual host, which is also what a
	// session built without a game_start models.
	autoPay bool
}

// pendingUse is one counted activated ability awaiting its stack departure.
type pendingUse struct {
	source string
	turn   uint32
	// held reports the stack object was observed at a priority decision: a
	// completed activation gives its controller priority with it on the
	// stack, while one reversed mid-announcement is seen only by the asks
	// that pay for it.
	held bool
}

// SetAutoPay records whether the host declared engine_autopay in its hello.
func (s *Session) SetAutoPay(autoPay bool) { s.autoPay = autoPay }

func NewSession(reg *cards.Registry) *Session {
	return &Session{B: NewBuilder(reg), Counts: map[string]int{}, Approx: map[string]int{}, activated: map[string]int{},
		grants: map[string][][]string{}, uses: map[string]int32{}, seen: map[string]bool{}, pendingUses: map[string]pendingUse{}}
}

// Picked tells the session which candidate of its last decision the seat
// answered. Later decisions of the same native decision depend on it: an
// arrangement's ordering picks follow its partition, and a target ask
// describes the ability its source just activated.
func (s *Session) Picked(i int) {
	if s.last == nil || i < 0 || i >= len(s.last.Candidates) {
		return
	}
	sem := s.last.Candidates[i].Semantic
	switch sem.Kind {
	case "activate_ability":
		if src, err := ref(sem, "source"); err == nil {
			if o, ok := s.abilityOption(src, num(sem, "ability_index")); ok {
				s.activated[src.ObjectID] = o.Ability
				var kw []string
				if f := s.abilityFace(src); f != nil && o.Ability >= 0 && o.Ability < len(f.Abilities) {
					kw = rules.PureGrantKeywords(f.Abilities[o.Ability])
				}
				s.grants[src.ObjectID] = append(s.grants[src.ObjectID], kw)
			} else {
				delete(s.activated, src.ObjectID) // a granted body names no printed ability
			}
		}
	case "arrange_card":
		if s.cur != nil && s.cur.arrange != nil {
			s.cur.arrange.dest = append(s.cur.arrange.dest, str(sem, "destination"))
		}
	case "order_pick":
		if s.cur != nil && s.cur.arrange != nil {
			if it, err := field[protocol.OrderItem](sem, "item"); err == nil && it.Object != nil {
				s.cur.arrange.placed = append(s.cur.arrange.placed, it.Object.ObjectID)
			}
		}
	}
}

// RebuildReason names why the last Payload could not reconstruct candidate
// i. It is empty when the candidate has a translation, so a caller can use
// it only for the candidates its ops mark Untranslated.
func (s *Session) RebuildReason(i int) string {
	if i < 0 || i >= len(s.lastReasons) {
		return ""
	}
	return s.lastReasons[i]
}

// untranslatedReason is the id-free diagnosis recorded for an op no plan
// matches: the decision family when the whole decision is unknown, or the
// candidate's own kind when only this offer has no translation.
func untranslatedReason(sem protocol.Semantic, decisionKind string) string {
	if decisionKind != "" {
		return "no neutral translation for decision kind " + decisionKind
	}
	return "no native option for candidate kind " + sem.Kind
}

// Payload translates a seat decision into what gorge's agent reads from
// x_gorge_view_v1 on gorge's own engine: the view, the native decision and
// one op per candidate. A family it cannot translate yields ops no plan
// matches, so the agent falls back; it is never an error.
func (s *Session) Payload(sd *protocol.SeatDecision) (xview.Payload, error) {
	v, err := s.B.View(&sd.Observation)
	if err != nil {
		return xview.Payload{}, err
	}
	s.seat = v.Viewer
	s.last = sd
	if sd.Observation.Turn != s.usesTurn {
		s.usesTurn, s.uses = sd.Observation.Turn, map[string]int32{}
		s.pendingUses = map[string]pendingUse{}
	}
	onStack := make(map[string]bool, len(sd.Observation.Stack))
	for _, e := range sd.Observation.Stack {
		onStack[e.ObjectID] = true
	}
	// A reversed activation leaves the stack with its source untapped (the
	// tap cost was undone). The observation carries no explicit census, so
	// this is the one observable signature of CR 733.1's fold.
	for id, p := range s.pendingUses {
		if onStack[id] {
			if sd.Context.Kind == "priority" {
				p.held = true
				s.pendingUses[id] = p
			}
			continue
		}
		// An activation the seat answered but never saw on the stack never
		// resolved: the engine reversed it while announcing it (CR 733.1) and
		// folded the cost back, so the turn's census must follow. One that
		// did reach a priority decision's stack is a completed activation
		// whatever its source looks like afterwards: a self-untap ability
		// leaves its source untapped by resolving, not by being folded back.
		if p.turn == s.usesTurn && !p.held {
			if s.uses[p.source] > 0 {
				s.uses[p.source]--
			}
		}
		delete(s.pendingUses, id)
	}
	for _, e := range sd.Observation.Stack {
		if e.StackKind == "activated_ability" && e.Source != nil && !s.seen[e.ObjectID] {
			s.seen[e.ObjectID] = true
			s.uses[e.Source.ObjectID]++
			s.pendingUses[e.ObjectID] = pendingUse{source: e.Source.ObjectID,
				turn: s.usesTurn, held: sd.Context.Kind == "priority"}
		}
	}
	for pi := range v.Players {
		for ci := range v.Players[pi].Battlefield {
			cv := &v.Players[pi].Battlefield[ci]
			cv.ActivatedThisTurn = s.uses[s.B.IDs.V2(cv.ID)]
		}
	}
	ops, n, err := s.translate(sd, &v)
	if err != nil {
		return xview.Payload{}, err
	}
	if n == nil {
		ops = make([]mapping.NativeOp, len(sd.Candidates))
		s.lastReasons = make([]string, len(sd.Candidates))
		for i := range ops {
			ops[i] = noOp
			s.lastReasons[i] = untranslatedReason(sd.Candidates[i].Semantic, sd.Candidates[0].Semantic.Kind)
		}
		s.Counts[sd.Candidates[0].Semantic.Kind]++
		s.cur = nil
		s.next++
		return xview.Payload{Version: 1, NativeIndex: s.next, View: v, Decision: decision.Decision{Player: s.seat}, Ops: ops}, nil
	}
	s.cur = n
	s.lastReasons = make([]string, len(ops))
	for i := range ops {
		if ops[i].Op == Untranslated {
			s.lastReasons[i] = untranslatedReason(sd.Candidates[i].Semantic, "")
		}
	}
	d := n.d
	d.Player = s.seat
	// Rebuild restores these policy facts from the payload; the neutral
	// world has no extension to read them from, so publish the decision's
	// own resume fields. Without the kind, a counter's unless-pay ask loses
	// its pay/decline arm and the bot pays a tax natively declined.
	facts := xview.Facts{ResumeKind: string(d.ResumeKind), EffectOptional: d.EffectOptional}
	if d.ResumeSA != nil {
		facts.ResumeAPI = d.ResumeSA.API
		facts.ResumeUnlessCost = d.ResumeSA.Params["UnlessCost"]
	}
	return xview.Payload{Version: 1, NativeIndex: n.index, View: v, Decision: d, Facts: facts, Ops: ops, Followups: n.follow}, nil
}

// open starts a new native decision under key.
func (s *Session) open(key string, d decision.Decision) *native {
	s.next++
	return &native{index: s.next, key: key, d: d, options: map[string]int{}}
}

// continuing reports whether the current native decision is the one under key.
func (s *Session) continuing(key string) *native {
	if s.cur != nil && s.cur.key == key {
		return s.cur
	}
	return nil
}

func (s *Session) translate(sd *protocol.SeatDecision, v *view.View) ([]mapping.NativeOp, *native, error) {
	first := sd.Candidates[0].Semantic
	if sd.Context.Kind == "priority" {
		return s.priority(sd, v)
	}
	switch first.Kind {
	case "choose_cast_method":
		return s.castMethod(sd)
	case "choose_target", "finish_target_selection":
		return s.targets(sd, v)
	case "select_object", "finish_selection":
		return s.selection(sd)
	case "choose_cost_target":
		return s.costTargets(sd)
	case "declare_attack":
		return s.attackers(sd, v)
	case "declare_block":
		return s.blockers(sd, v)
	case "choose_boolean", "optional_cast":
		return s.yesNo(sd)
	case "optional_cost":
		return s.optionalCost(sd)
	case "mulligan":
		return s.mulligan(sd)
	case "order_pick":
		return s.order(sd)
	case "arrange_card":
		return s.arrange(sd)
	case "choose_spell_mode":
		return s.modes(sd)
	case "choose_cost_option":
		return s.costOption(sd)
	case "choose_option":
		return s.option(sd)
	case "distribute":
		return s.distribute(sd, v)
	case "choose_pile":
		return s.pile(sd)
	case "choose_number":
		return s.numbers(sd)
	case "choose_color":
		return s.colors(sd)
	case "choose_replacement":
		return s.replacement(sd)
	}
	return nil, nil, nil
}

// --- semantic field readers ---

func field[T any](sem protocol.Semantic, name string) (T, error) {
	var out T
	raw, ok := sem.Fields[name].(json.RawMessage)
	if !ok {
		return out, fmt.Errorf("%s.%s is not an object", sem.Kind, name)
	}
	err := json.Unmarshal(raw, &out)
	return out, err
}

func ref(sem protocol.Semantic, name string) (protocol.ObjectRef, error) {
	return field[protocol.ObjectRef](sem, name)
}

// optRef reads a nullable object reference.
func optRef(sem protocol.Semantic, name string) (*protocol.ObjectRef, error) {
	if sem.Fields[name] == nil {
		return nil, nil
	}
	r, err := ref(sem, name)
	return &r, err
}

func target(sem protocol.Semantic, name string) (*protocol.TargetRef, error) {
	if sem.Fields[name] == nil {
		return nil, nil
	}
	t, err := field[protocol.TargetRef](sem, name)
	return &t, err
}

func num(sem protocol.Semantic, name string) int {
	n, _ := sem.Fields[name].(json.Number)
	i, _ := strconv.Atoi(string(n))
	return i
}

func str(sem protocol.Semantic, name string) string {
	s, _ := sem.Fields[name].(string)
	return s
}

func boolean(sem protocol.Semantic, name string) bool {
	b, _ := sem.Fields[name].(bool)
	return b
}

func targetKey(t protocol.TargetRef) string {
	if t.Player != nil {
		return "player:" + *t.Player
	}
	return t.Object.ObjectID
}

func name(r protocol.ObjectRef) string {
	if r.CardName == nil {
		return "a card"
	}
	return *r.CardName
}

// --- priority ---

// castActions names the special_action actions gorge offers as casts, by the
// word its option label starts with. The action is the option's Mode.
var castActions = map[string]string{"suspend": "Suspend", "plot": "Plot"}

// castModes maps a Section 7.4 method to gorge's cast option Mode.
var castModes = map[string]string{"normal": "", "flashback": "flashback", "escape": "escape", "madness": "madness",
	"plot": "plot_cast", "foretell": "foretell_cast", "adventure": "adventure_alt", "split_right": "split_alt",
	"mdfc_back": "modal_spell", "miracle": "miracle", "suspend": "suspend_cast", "evoke": "evoked"}

// hasOmenFace reports whether src names a card whose inset face is an Omen
// spell (CR 720). It is the guard for the protocol's catch-all "other"
// method: mapping.CastMethod maps gorge's omen_alt to "other" because CR 720
// has no Section 7.4 method of its own, so only such a card may invert that
// method back to omen_alt.
func (s *Session) hasOmenFace(src protocol.ObjectRef) bool {
	r := s.B.ref(src.CardName)
	if r.Card == nil || r.Card.AlternateMode != "Omen" || len(r.Card.Faces) != 2 {
		return false
	}
	f := r.Card.Faces[1]
	return (f.IsInstant() || f.IsSorcery()) && slices.Contains(f.Types, "Omen")
}

func (s *Session) priority(sd *protocol.SeatDecision, v *view.View) ([]mapping.NativeOp, *native, error) {
	n := s.open("priority", decision.Decision{Kind: decision.KPriority, Min: 1, Max: 1})
	n.casts, n.names, n.paid = map[string][]int{}, map[string]string{}, map[string]*decision.PaymentSelection{}
	pre := s.prepay(sd, v)
	// pendingGrant counts the activated abilities of each source on the stack
	// right now: the observation's only evidence for decision.Grant.Duplicate.
	// The stack resolves top-down, so when the count drops the records that
	// remain pending are the oldest ones. Prune the grants this priority
	// decision no longer sees here, not in Payload: the cost-target and target
	// decisions an activation poses come before its ability is on the stack,
	// and pruning there would drop the grant that a later priority decision
	// must still report.
	pendingGrant := map[string]int{}
	for _, e := range sd.Observation.Stack {
		if e.StackKind == "activated_ability" && e.Source != nil {
			pendingGrant[e.Source.ObjectID]++
		}
	}
	for id, recs := range s.grants {
		n := pendingGrant[id]
		switch {
		case n == 0:
			delete(s.grants, id)
		case n < len(recs):
			s.grants[id] = recs[:n]
		}
	}
	ops := make([]mapping.NativeOp, len(sd.Candidates))
	add := func(o decision.Option) int {
		o.Index = len(n.d.Options)
		n.d.Options = append(n.d.Options, o)
		return o.Index
	}
	for i, c := range sd.Candidates {
		sem := c.Semantic
		ops[i] = noOp
		switch sem.Kind {
		case "pass":
			ops[i] = mapping.NativeOp{Op: "choose", Option: add(decision.Option{Kind: "pass", Label: "Pass priority"})}
		case "play_land":
			src, err := ref(sem, "source")
			if err != nil {
				return nil, nil, err
			}
			o := decision.Option{Kind: "play_land", Label: "Play " + name(src), Obj: s.B.IDs.Of(src.ObjectID)}
			if num(sem, "face") == 1 {
				o.Mode = "modal_land"
			}
			ops[i] = mapping.NativeOp{Op: "choose", Option: add(o)}
		case "cast_spell":
			src, err := ref(sem, "source")
			if err != nil {
				return nil, nil, err
			}
			o := decision.Option{Kind: "cast", Label: "Cast " + name(src), Obj: s.B.IDs.Of(src.ObjectID)}
			method := str(sem, "method")
			switch {
			case sem.Fields["method"] == nil && src.Zone == "graveyard":
				o.Mode = "flashback" // a null method from a graveyard is the card's graveyard cast
			case sem.Fields["method"] == nil:
			case method == "alternative":
				o.AltCostIndex = 1
			default:
				mode, ok := castModes[method]
				if !ok {
					if method != "other" || !s.hasOmenFace(src) {
						continue
					}
					mode = "omen_alt"
				}
				o.Mode = mode
			}
			// An autopay host pays a planned cast by its plan: gorge's
			// decision then has no plain cast option, only the action.
			plan := pre.cover(o)
			if plan == nil && s.autoPay && ownNonMain(v) && o.Mode == "" && o.AltCostIndex == 0 {
				// On the seat's own turn outside a main phase the native
				// bot never consults the payment planner, so its decision
				// carries only plan-only casts. When the shadow planner
				// could not rebuild the plan, a plain cast here would let
				// the neutral bot cast a spell the native bot passed. Leave
				// the candidate untranslated unless the floating pool alone
				// would have offered it as a plain cast, or the card has a
				// contribution cast (Convoke and its siblings) the planner
				// never prices as a plan: the engine offers those as plain
				// casts at any time.
				if f := s.B.face(src.CardName, false, nil); f != nil && !s.poolPays(v, f) && !contributionCast(f) {
					continue
				}
			}
			idx := -1
			n.names[src.ObjectID] = name(src)
			if plan == nil || plan.poolOnly {
				idx = add(o)
				// One card can offer several cast methods at once (an
				// adventure and its creature), so accumulate instead of
				// replacing: castMethod matches a later method decision
				// against every option of the source.
				n.casts[src.ObjectID] = append(n.casts[src.ObjectID], idx)
				ops[i] = mapping.NativeOp{Op: "choose", Option: idx}
				if plan != nil {
					// The pool already pays: the plan tags the plain cast.
					plan.base = &idx
					ops[i].Payment = plan.sel
				}
			} else {
				n.paid[name(src)] = plan.sel
				ops[i] = mapping.NativeOp{Op: "payment", Option: -1, Payment: plan.sel}
			}
			if (sem.Fields["method"] == nil || method == "normal") && src.Zone == "hand" {
				// gorge offers the optional-cost variants (kicker and the
				// like) as casts of their own, beside the plain cast. The
				// v2 wire collapses them into this candidate and asks the
				// optional_cost follow-up, so the inverse must put them
				// back for the bot to rank them at priority.
				var covers []int
				if f := s.B.face(src.CardName, false, nil); f != nil && f.HasKeyword("Kicker") && s.kickerAffordable(v, f) {
					kicked := add(decision.Option{Kind: "cast", Label: "Cast " + name(src) + " (kicked)",
						Obj: o.Obj, Mode: "kicked"})
					n.casts[src.ObjectID] = append(n.casts[src.ObjectID], kicked)
					covers = append(covers, kicked)
				}
				if s.altCost(sd, src) {
					// gorge offers the alternative cost as a cast of its own,
					// right after the plain cast; the method decision follows.
					alt := add(decision.Option{Kind: "cast", Label: "Cast " + name(src) + " (alternative cost)",
						Obj: o.Obj, AltCostIndex: 1})
					n.casts[src.ObjectID] = append(n.casts[src.ObjectID], alt)
					covers = append(covers, alt)
				}
				if len(covers) > 0 {
					all := covers
					if idx >= 0 {
						all = append([]int{idx}, covers...)
					}
					ops[i] = mapping.NativeOp{Op: "cast", Option: -1, Covers: all, Payment: ops[i].Payment}
				}
			}
		case "special_action":
			src, err := ref(sem, "source")
			if err != nil {
				return nil, nil, err
			}
			// gorge offers suspending or plotting a card as a cast of its own
			// mode, which the bot ranks with the other casts.
			if label, ok := castActions[str(sem, "action")]; ok {
				ops[i] = mapping.NativeOp{Op: "choose", Option: add(decision.Option{Kind: "cast", Label: label + " " + name(src),
					Obj: s.B.IDs.Of(src.ObjectID), Mode: str(sem, "action")})}
			}
		case "activate_ability":
			src, err := ref(sem, "source")
			if err != nil {
				return nil, nil, err
			}
			// The rebuilt board's own offer knows what gorge's option carries
			// (a loyalty cost's facts, a grant the printed face lacks). The
			// printed face answers for a board that could not be rebuilt.
			idx := num(sem, "ability_index")
			o, offered := pre.offeredAbility(s.B.IDs.Of(src.ObjectID), idx)
			if printed, ok := s.abilityOption(src, idx); ok {
				if !offered {
					o = printed
				}
				o.Grant = s.grantFact(src, printed, v, pendingGrant)
			} else if !offered {
				continue
			}
			ops[i] = mapping.NativeOp{Op: "choose", Option: add(o)}
		}
		// activate_mana_ability stays untranslated here: gorge's bot never
		// floats mana by hand (its auto-pay policies drop mana activations).
		// pay tags the candidates a chosen payment plan takes.
	}
	s.finish(pre, n, ops)
	return ops, n, nil
}

// grantFact reconstructs decision.Option.Grant (the bot's A1 no-op fact) for
// an activated ability whose whole effect is a pure, self-aimed keyword grant.
// The engine fills it natively from the ability it offers (rules.abilityGrant);
// the neutral world has no engine walk, so it reads the printed ability and
// the observation: Already from the projected keyword list, Duplicate from the
// recorded pending activations the session answered for this source.
// A nil result means "not a pure keyword grant", exactly the native nil.
func (s *Session) grantFact(src protocol.ObjectRef, o decision.Option, v *view.View, pending map[string]int) *decision.Grant {
	f := s.abilityFace(src)
	if f == nil || o.Ability < 0 || o.Ability >= len(f.Abilities) {
		return nil
	}
	kw := rules.PureGrantKeywords(f.Abilities[o.Ability])
	if len(kw) == 0 {
		return nil
	}
	g := &decision.Grant{Keywords: kw}
	g.Already = cardHasKeywords(v, o.Obj, kw)
	g.Duplicate = pending[src.ObjectID] > 0 && grantRecorded(s.grants[src.ObjectID], kw)
	return g
}

// grantRecorded reports whether recs holds a keyword set equal to kw. It
// mirrors rules.grantSetsEqual: order-independent and case-insensitive.
func grantRecorded(recs [][]string, kw []string) bool {
	for _, rec := range recs {
		if len(rec) != len(kw) {
			continue
		}
		all := true
		for _, x := range rec {
			found := false
			for _, y := range kw {
				if strings.EqualFold(x, y) {
					found = true
					break
				}
			}
			if !found {
				all = false
				break
			}
		}
		if all {
			return true
		}
	}
	return false
}

// requirementFace resolves the printed face of a battlefield record the
// combat-requirement reconstruction reads (a self-scoped MustAttack static).
// A token uses the token-script lookup its observed characteristics select,
// the same way buildCard picks the shown face.
func (s *Session) requirementFace(r protocol.ObjectRecord) *cards.Face {
	if r.CardName == nil {
		return nil
	}
	return s.B.face(r.CardName, r.Token && !r.Copy, r.Characteristics)
}

// PreferTokens narrows a shared token face name to the scripts this game's
// decks can create (Builder.PreferTokens).
func (s *Session) PreferTokens(stems []string) { s.B.PreferTokens(stems) }

// staticFace resolves the printed face a permanent's statics live on: a
// token's script when the permanent is a token, otherwise the card. A token
// registry lookup by name alone never finds a same-named card (the old
// card-only lookup missed every token static: the "can't block" Rat token
// read as blockable and the board-derived block list offered it).
func (s *Session) staticFace(cv view.CardView) *cards.Face {
	return s.B.face(&cv.Name, cv.IsToken, nil)
}

// abilityFace resolves the face an activated ability's source shows, matching
// abilityOption's card-then-token lookup.
func (s *Session) abilityFace(src protocol.ObjectRef) *cards.Face {
	if f := s.B.face(src.CardName, false, nil); f != nil {
		return f
	}
	return s.B.face(src.CardName, true, nil)
}

// cardHasKeywords reports whether the projected card id already carries every
// keyword in kw, the neutral reading of the engine's HasKeyword loop. The
// projected Keyword list is the engine's derived list, so this is the same
// fact rules.abilityGrant reads.
func cardHasKeywords(v *view.View, id state.ObjID, kw []string) bool {
	for _, p := range v.Players {
		for _, cv := range p.Battlefield {
			if cv.ID != id {
				continue
			}
			for _, want := range kw {
				found := false
				for _, have := range cv.Keywords {
					if strings.EqualFold(cards.KeywordHead(have), want) {
						found = true
						break
					}
				}
				if !found {
					return false
				}
			}
			return true
		}
	}
	return false
}

// kickerAffordable reports whether the printed cost plus the kicker cost
// fits the mana already in this seat's pool. gorge's cast offer prices the
// kicked variant against the floating pool, not against untapped sources it
// would have to float first (the player can activate those separately), so a
// pool-only test is the exact gate: adding producible mana would invent a
// kicked option the engine does not offer.
func (s *Session) kickerAffordable(v *view.View, f *cards.Face) bool {
	kick, ok := f.KeywordParam("Kicker")
	if !ok {
		return false
	}
	need := f.ManaValue() + rules.ParseCost(kick).CMC()
	var avail int32
	for _, p := range v.Players {
		if p.ID != s.seat {
			continue
		}
		for _, n := range p.Pool {
			avail += n
		}
	}
	return avail >= need
}

// altCost reports whether src's own AlternativeCost static offers its cost
// now. Land Grant's condition (no land cards in hand) is read off the hand;
// any other condition is not offered.
func (s *Session) altCost(sd *protocol.SeatDecision, src protocol.ObjectRef) bool {
	f := s.B.face(src.CardName, false, nil)
	if f == nil {
		return false
	}
	for _, st := range f.Statics {
		if st.Mode != "AlternativeCost" || st.Params["ValidCard"] != "Card.Self" {
			continue
		}
		// The engine's alternativeCostScopeOK prices a spell cast only: a
		// ValidSA$ naming no Spell scope (Heart of Kiran's Activated.Crew)
		// is an ability's cost, not a cast the neutral may offer.
		if vs := strings.TrimSpace(st.Params["ValidSA"]); vs != "" {
			ok := false
			for alt := range strings.SplitSeq(vs, ",") {
				alt = strings.TrimSpace(alt)
				kind, constraint := alt, ""
				if i := strings.IndexByte(alt, '.'); i >= 0 {
					kind, constraint = alt[:i], alt[i+1:]
				}
				if kind != "Spell" {
					continue
				}
				switch strings.TrimSpace(constraint) {
				case "", "Self":
					ok = true // the neutral offers self-cast statics only
				}
			}
			if !ok {
				continue
			}
		}
		check := st.Params["CheckSVar"]
		if check == "" {
			return true
		}
		if !strings.HasPrefix(f.SVars[check], "Count$ValidHand Land") || st.Params["SVarCompare"] != "EQ0" {
			return false
		}
		for _, p := range sd.Observation.Players {
			if p.Seat != sd.ActingSeat {
				continue
			}
			for _, o := range p.Hand {
				if o.Characteristics != nil && slices.Contains(o.Characteristics.Types, "land") {
					return false
				}
			}
		}
		return true
	}
	return false
}

// abilityOption is gorge's "ability" option for a permanent's non-mana
// activated ability number idx (Section 7.2 order: printed non-mana
// abilities in face order).
func (s *Session) abilityOption(src protocol.ObjectRef, idx int) (decision.Option, bool) {
	f := s.B.face(src.CardName, false, nil)
	if f == nil {
		f = s.B.face(src.CardName, true, nil) // a token's ability
	}
	if f == nil {
		return decision.Option{}, false
	}
	k := 0
	for i, a := range f.Abilities {
		if a.Kind != "AB" || manaAbility(a) {
			continue
		}
		if k == idx {
			return decision.Option{Kind: "ability", Label: f.Name + ": " + a.Params["SpellDescription"],
				Obj: s.B.IDs.Of(src.ObjectID), Ability: i, Cost: a.Params["Cost"], Attach: a.API == "Attach"}, true
		}
		k++
	}
	// idx >= k is a granted, gained or keyword-granted ability. The plain
	// observation carries no ability list (ObjectRecord has no abilities
	// field), so the neutral world cannot reconstruct its body or rank it.
	// The v2 candidate is still posed -- the seat answers candidate ids -- but
	// this world reports it opaque rather than guessing another ability.
	return decision.Option{}, false
}

// manaAbility mirrors mapping's split of gorge's activated abilities.
func manaAbility(a *cards.SA) bool {
	// The priority offer walk skips an AB only when cards.IsManaAbilitySA
	// says it is a mana ability and it is not a loyalty ability. A targeting
	// or library-moving AB$ Mana is offered on the ordinary path (CR 605.1a),
	// so it must count as a non-mana activated ability here too.
	if !cards.IsManaAbilitySA(a) {
		return false
	}
	if v, ok := a.Params["Planeswalker"]; ok && strings.EqualFold(strings.TrimSpace(v), "True") {
		return false
	}
	return !strings.Contains(strings.ToUpper(a.Params["Cost"]), "LOYALTY")
}

// castMethod answers a cast's method from the priority plan: the method of
// the cast option the plan chose.
func (s *Session) castMethod(sd *protocol.SeatDecision) ([]mapping.NativeOp, *native, error) {
	n := s.continuing("priority")
	if n == nil {
		return nil, nil, nil
	}
	ops := make([]mapping.NativeOp, len(sd.Candidates))
	for i, c := range sd.Candidates {
		ops[i] = noOp
		src, err := ref(c.Semantic, "source")
		if err != nil {
			return nil, nil, err
		}
		method := str(c.Semantic, "method")
		opts, ok := n.casts[src.ObjectID]
		if !ok {
			// The spell moved to the stack under a new id: its cast is one
			// of those of the same card name, whichever the plan chose.
			for _, id := range slices.Sorted(maps.Keys(n.casts)) {
				if n.names[id] == name(src) {
					opts = append(opts, n.casts[id]...)
				}
			}
		}
		var covers []int
		for _, opt := range opts {
			o := n.d.Options[opt]
			want := "normal"
			switch {
			case o.AltCostIndex > 0:
				want = "alternative"
			case o.Mode != "":
				for m, mode := range castModes {
					if mode == o.Mode {
						want = m
					}
				}
			}
			if method == want {
				covers = append(covers, opt)
			}
		}
		if len(covers) > 0 {
			ops[i] = mapping.NativeOp{Op: "cast", Option: -1, Covers: covers}
		}
		if sel := n.paid[name(src)]; sel != nil && method == "normal" {
			// The plain cast was paid by a plan: the method decision that
			// follows it is the plan's own.
			ops[i].Op, ops[i].Payment = "payment", sel
			if len(covers) == 0 {
				ops[i].Option = -1
			}
		}
	}
	return ops, n, nil
}

// --- targets and selections ---

// pick is a target or object selection spent over one or more v2
// decisions: a fixed group, or variable picks each closed by a finish.
func (s *Session) pick(sd *protocol.SeatDecision, key string, d decision.Decision, choice func(protocol.Semantic) (*protocol.TargetRef, error),
	opt func(protocol.TargetRef) (decision.Option, error), finish string) ([]mapping.NativeOp, *native, error) {
	first := sd.Candidates[0].Semantic
	n := s.continuing(key)
	started := sd.Group.SubstepIndex > 0 || num(first, "selected_count") > 0
	if n == nil || !started {
		n = s.open(key, d)
		for _, c := range sd.Candidates {
			t, err := choice(c.Semantic)
			if err != nil {
				return nil, nil, err
			}
			if t == nil {
				continue
			}
			o, err := opt(*t)
			if err != nil {
				return nil, nil, err
			}
			o.Index = len(n.d.Options)
			n.options[targetKey(*t)] = o.Index
			n.d.Options = append(n.d.Options, o)
		}
	}
	ops := make([]mapping.NativeOp, len(sd.Candidates))
	for i, c := range sd.Candidates {
		ops[i] = noOp
		if c.Semantic.Kind == finish {
			ops[i] = mapping.NativeOp{Op: "finish", Option: -1}
			continue
		}
		t, err := choice(c.Semantic)
		if err != nil {
			return nil, nil, err
		}
		if t == nil {
			continue
		}
		if k, ok := n.options[targetKey(*t)]; ok {
			ops[i] = mapping.NativeOp{Op: "choose", Option: k}
		}
	}
	return ops, n, nil
}

// targetOption is gorge's target option for t.
func (s *Session) targetOption(v *view.View, t protocol.TargetRef) (decision.Option, error) {
	if t.Player != nil {
		p, err := Seat(*t.Player)
		return decision.Option{Kind: "player", Label: *t.Player, Player: p, Controller: p}, err
	}
	r := *t.Object
	c, err := Seat(r.ControllerSeat)
	if err != nil {
		return decision.Option{}, err
	}
	kind := "permanent"
	switch r.Zone {
	case "stack":
		kind = "spell"
	case "graveyard", "exile", "hand", "library":
		kind = "card"
	}
	return decision.Option{Kind: kind, Label: name(r), Obj: s.B.IDs.Of(r.ObjectID), Player: c, Controller: c}, nil
}

func (s *Session) targets(sd *protocol.SeatDecision, v *view.View) ([]mapping.NativeOp, *native, error) {
	first := sd.Candidates[0].Semantic
	src, err := ref(first, "source")
	if err != nil {
		return nil, nil, err
	}
	slot := num(first, "slot")
	lo, hi := num(first, "minimum"), num(first, "maximum")
	if first.Kind == "finish_target_selection" {
		for _, c := range sd.Candidates {
			if c.Semantic.Kind == "choose_target" {
				lo, hi = num(c.Semantic, "minimum"), num(c.Semantic, "maximum")
				break
			}
		}
	}
	d := decision.Decision{Kind: decision.KTarget, Min: lo, Max: hi, Source: s.B.IDs.Of(src.ObjectID),
		Prompt: "Choose a target", TargetEffect: s.targetEffect(&sd.Observation, src, slot)}
	// A per-controller target restriction (TargetsForEachPlayer$ or
	// TargetsWithDifferentControllers$) is gorge's Option.Group: the wire
	// carries no group, but the source's targeting SA names the shape, and
	// every candidate's controller is on the wire. Rebuild the same label the
	// engine attaches so the bot's GroupAdmits pacing picks one target per
	// controller instead of a second one from a controller already chosen.
	// The same SA carries the cumulative caps (MaxTotalTargetPower$ /
	// MaxTotalTargetCMC$); the wire names neither the cap nor each option's
	// price, so the bot's Clamp would otherwise return a set the engine's
	// budget rejects.
	exclusive := false
	powerCap, powerCapped := 0, false
	cmcCap, cmcCapped := 0, false
	if _, _, sa, _ := s.targetAsk(&sd.Observation, src, slot); sa != nil {
		exclusive = effects.TargetsOf(sa).Flags&(effects.TgtForEachPlayer|effects.TgtDifferentControllers) != 0
		powerCap, powerCapped = literalTargetCap(sa.Params["MaxTotalTargetPower"])
		cmcCap, cmcCapped = literalTargetCap(sa.Params["MaxTotalTargetCMC"])
	}
	switch {
	case powerCapped && cmcCapped:
		d.MaxSum, d.Budgeted, d.MaxSum2, d.Budgeted2 = powerCap, true, cmcCap, true
	case powerCapped:
		d.MaxSum, d.Budgeted = powerCap, true
	case cmcCapped:
		d.MaxSum, d.Budgeted = cmcCap, true
	}
	key := fmt.Sprintf("target|%s|%d", src.ObjectID, slot)
	return s.pick(sd, key, d, func(sem protocol.Semantic) (*protocol.TargetRef, error) {
		if sem.Kind != "choose_target" {
			return nil, nil
		}
		return target(sem, "target")
	}, func(t protocol.TargetRef) (decision.Option, error) {
		o, err := s.targetOption(v, t)
		if err == nil && exclusive {
			o.Group = "target-controller-" + strconv.Itoa(int(o.Controller))
		}
		if err == nil && t.Object != nil && (powerCapped || cmcCapped) {
			// The cap is over non-player targets; a face is free, exactly as
			// the engine leaves its Value at zero.
			var power int32
			if powerCapped {
				power = targetPower(&sd.Observation, s, *t.Object)
				o.Value = int(power)
			}
			if cmcCapped {
				mv := int(targetManaValue(s, t.Object.CardName))
				if powerCapped {
					o.Value2 = mv
				} else {
					o.Value = mv
				}
			}
		}
		return o, err
	}, "finish_target_selection")
}

// literalTargetCap reads a cumulative target cap written as a plain integer.
// A dynamic cap (X, an SVar name, a Count$ body) is not on the wire, so it
// stays unknown rather than being invented from the observation.
func literalTargetCap(raw string) (int, bool) {
	n, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || n < 0 {
		return 0, false
	}
	return n, true
}

// targetPower is the engine's derived power for a target option, read off the
// observation's permanent, with the printed face as the fallback.
func targetPower(o *protocol.Observation, s *Session, r protocol.ObjectRef) int32 {
	if p, ok := observedPower(o, r.ObjectID); ok {
		return int32(p)
	}
	if f := s.B.face(r.CardName, false, nil); f != nil {
		return int32(f.Power())
	}
	return 0
}

// targetManaValue is the printed mana value of a target card, the same face
// field the engine publishes for a budget's Value.
func targetManaValue(s *Session, name *string) int32 {
	if f := s.B.face(name, false, nil); f != nil {
		return botpolicy.CmcOf(f.ManaCost)
	}
	return 0
}

// selectKinds maps a select_object purpose to gorge's KChoose option kind.
var selectKinds = map[string]string{"discard": "discard", "sacrifice": "sacrifice", "exile": "exile",
	"search": "search", "return_to_hand": "returncost", "tap": "tapcost", "untap": "untap", "keep": "keep",
	"delve": "exile", "put_into_hand": "dig", "put_onto_battlefield": "hidden_pick", "put_into_graveyard": "hidden_pick",
	"reveal": "reveal", "legend_rule": "keep", "convoke": "convoke", "vote": "vote_card", "other": "card"}

// replacement translates a CR 616.1 replacement-order ask: v2 names each
// applicable replacement by its index and source permanent, and gorge asks a
// KReplacement whose options are the same sources. The candidate order is the
// engine's deterministic scan order, so the bot's worth ranking sees the same
// options in the same order.
func (s *Session) replacement(sd *protocol.SeatDecision) ([]mapping.NativeOp, *native, error) {
	d := decision.Decision{Kind: decision.KReplacement, Min: 1, Max: 1, Prompt: "Choose a replacement effect to apply next"}
	n := s.open("replacement", d)
	ops := make([]mapping.NativeOp, len(sd.Candidates))
	for i, c := range sd.Candidates {
		ops[i] = noOp
		if c.Semantic.Kind != "choose_replacement" {
			continue
		}
		src, err := optRef(c.Semantic, "replacement_source")
		if err != nil {
			return nil, nil, err
		}
		if src == nil {
			continue
		}
		o := decision.Option{Index: len(n.d.Options), Kind: "replacement", Label: name(*src), Obj: s.B.IDs.Of(src.ObjectID)}
		n.d.Options = append(n.d.Options, o)
		ops[i] = mapping.NativeOp{Op: "choose", Option: o.Index}
	}
	return ops, n, nil
}

func (s *Session) selection(sd *protocol.SeatDecision) ([]mapping.NativeOp, *native, error) {
	first := sd.Candidates[0].Semantic
	purpose := str(first, "purpose")
	src, err := optRef(first, "source")
	if err != nil {
		return nil, nil, err
	}
	if purpose == "other" && src != nil {
		if sa := s.chainSA(*src, func(sa *cards.SA) bool {
			return sa.API == "ChangeZone" && strings.EqualFold(sa.Params["Origin"], "Hand") && strings.EqualFold(sa.Params["Destination"], "Library")
		}); sa != nil {
			return s.handToLibrary(sd, *src, sa)
		}
	}
	// A hand-origin ChangeZone is gorge's hand_move ask, whatever its
	// destination: the wire purpose names the destination only, and the
	// bot's hand_move arm (the engine's deterministic first-Max take) is not
	// the exile arm the generic "exile" purpose would select. The visible
	// (own-hand) shape keeps the engine's option order; a foreign hand is a
	// look-keyed list the wire sorts, so it stays on the generic purpose.
	sa := (*cards.SA)(nil)
	if src != nil {
		sa = s.handMoveSA(*src, &sd.Observation, purpose)
	}
	kind, ok := selectKinds[purpose]
	if sa != nil && ownHandOptions(sd) {
		kind, ok = "hand_move", true
	}
	if ok && src != nil && purpose == "other" && s.hasAPI(*src, &sd.Observation, "Proliferate") {
		// mapping routes a Proliferate pick through the protocol's "other"
		// purpose, but the bot's proliferate arm takes the first Max options
		// (Min 0 included), unlike the generic "other" first-Min fill.
		kind = "proliferate"
	}
	if !ok {
		return nil, nil, nil
	}
	lo, hi := num(first, "minimum"), num(first, "maximum")
	for _, c := range sd.Candidates {
		if c.Semantic.Kind == "select_object" {
			lo, hi = num(c.Semantic, "minimum"), num(c.Semantic, "maximum")
			break
		}
	}
	d := decision.Decision{Kind: decision.KChoose, Min: lo, Max: hi, Prompt: "Choose " + purpose}
	if src != nil && purpose == "tap" && lo == 0 && sd.Observation.PhaseStep == "declare_attackers" {
		// The wire poses Enlist as an optional tap; gorge's ask is its own.
		if f := s.abilityFace(*src); f != nil && f.HasKeyword("Enlist") {
			kind = "enlist"
			d.Options = []decision.Option{{Kind: kind, Label: "Don't enlist for " + name(*src)}}
		}
	}
	if src != nil {
		if sa := s.hiddenPickSA(*src, &sd.Observation, purpose); sa != nil {
			// gorge asks a public-origin Hidden$ ChangeZone as a hidden_pick
			// of the zone's cards, whatever the destination.
			kind, d.ResumeKind, d.ResumeSA = "hidden_pick", "hidden_pick", sa
		}
	}
	key := "select|" + purpose
	if purpose == "discard" && (src != nil || sd.Observation.PhaseStep != "cleanup") {
		d.Kind = decision.KModes // gorge asks an effect's discard as a modal pick
	}
	if src != nil {
		d.Source = s.B.IDs.Of(src.ObjectID)
		key += "|" + src.ObjectID
		if purpose == "tap" {
			// A tap cost with a total-power floor publishes the floor as
			// Decision.MinSum and each candidate's power as its Value
			// (Crew 3, tapXType<Any/...withTotalPowerGE3>), so the bot's
			// shared Validate-driven fill paces the election.
			d.MinSum = s.tapFloor(*src)
		}
	}
	ops, n, err := s.pick(sd, key, d, func(sem protocol.Semantic) (*protocol.TargetRef, error) {
		if sem.Kind != "select_object" {
			return nil, nil
		}
		return target(sem, "choice")
	}, func(t protocol.TargetRef) (decision.Option, error) {
		if t.Object == nil {
			p, err := Seat(*t.Player)
			okind := kind
			if okind == "card" {
				okind = "player" // the protocol's "other" purpose covers a named player too
			}
			return decision.Option{Kind: okind, Label: *t.Player, Player: p}, err
		}
		o := decision.Option{Kind: kind, Label: name(*t.Object), Obj: s.B.IDs.Of(t.Object.ObjectID)}
		if kind == "hidden_pick" {
			// The pick's option names the card's owner, as effHiddenPick's does.
			p, err := Seat(t.Object.OwnerSeat)
			if err != nil {
				return o, err
			}
			o.Player = p
		}
		if purpose == "tap" {
			if p, ok := observedPower(&sd.Observation, t.Object.ObjectID); ok {
				o.Value = p
			} else {
				f := s.B.face(t.Object.CardName, false, nil)
				if f == nil {
					f = s.B.face(t.Object.CardName, true, nil) // a token's printed face
				}
				if f != nil {
					o.Value = f.Power()
				}
			}
		}
		return o, nil
	}, "finish_selection")
	if err == nil && purpose == "search" && src != nil {
		s.groupSearch(sd, n, *src)
	}
	if kind == "enlist" {
		for i := range ops {
			if ops[i].Op == "finish" {
				ops[i] = mapping.NativeOp{Op: "choose", Option: 0} // gorge offers the decline as an option
			}
		}
	}
	return ops, n, err
}

// hiddenPickDestinations is the select_object purpose a Hidden$ ChangeZone's
// destination zone names (mapping's newHiddenPick).
var hiddenPickDestinations = map[string]string{"exile": "exile", "hand": "put_into_hand",
	"battlefield": "put_onto_battlefield", "graveyard": "put_into_graveyard", "library": "search"}

// handMovePurposes is mapping's handDestPurpose inverted: the wire purpose a
// hand-origin ChangeZone's destination names.
var handMovePurposes = map[string]string{"hand": "put_into_hand", "battlefield": "put_onto_battlefield",
	"graveyard": "put_into_graveyard", "exile": "exile"}

// ownHandOptions reports whether every select_object candidate names a card
// in the acting seat's own hand: the visible shape whose candidate order is
// the engine's option order. A foreign hand's candidates are look ids sorted
// by (name, id) on the wire, so its order is not recoverable.
func ownHandOptions(sd *protocol.SeatDecision) bool {
	seen := false
	for _, c := range sd.Candidates {
		t, err := target(c.Semantic, "choice")
		if err != nil || t == nil || t.Object == nil {
			continue
		}
		seen = true
		if t.Object.Zone != "hand" || t.Object.OwnerSeat != sd.ActingSeat {
			return false
		}
	}
	return seen
}

// handMoveSA is the source's ChangeZone picking from a hand, when purpose is
// the wire purpose that move's destination has. It is the hand_move sibling of
// hiddenPickSA: the ask the native engine poses as KChoose with option kind
// "hand_move" (effects/zone_hand.go).
func (s *Session) handMoveSA(src protocol.ObjectRef, o *protocol.Observation, purpose string) *cards.SA {
	f, roots := s.abilityRoots(src, o)
	for _, root := range roots {
		if root == nil {
			continue
		}
		for _, sa := range chain(f, root) {
			if sa.API != "ChangeZone" || !strings.Contains(strings.ToLower(sa.Params["Origin"]), "hand") {
				continue
			}
			dest := strings.ToLower(strings.TrimSpace(sa.Params["Destination"]))
			if handMovePurposes[dest] == purpose {
				return sa
			}
		}
	}
	return nil
}

// hiddenPickSA is the Hidden$ ChangeZone of the source's ability that asks a
// pick for purpose from public zones (a graveyard, exile, the battlefield):
// gorge's effHiddenPick, whose decision is a hidden_pick. An origin that
// holds hidden information (a library search, a hand move) has its own ask.
func (s *Session) hiddenPickSA(src protocol.ObjectRef, o *protocol.Observation, purpose string) *cards.SA {
	f, roots := s.abilityRoots(src, o)
	for _, root := range roots {
		if root == nil {
			continue
		}
		for _, sa := range chain(f, root) {
			if sa.API != "ChangeZone" || !strings.EqualFold(sa.Params["Hidden"], "True") {
				continue
			}
			origin := strings.ToLower(sa.Params["Origin"])
			if origin == "" || strings.Contains(origin, "hand") || strings.Contains(origin, "library") ||
				hiddenPickDestinations[strings.ToLower(strings.TrimSpace(sa.Params["Destination"]))] != purpose {
				continue
			}
			return sa
		}
	}
	return nil
}

// groupSearch restores the Group structure of a hidden-library search. gorge
// offers one option per eligible card with the spec's ordinal in Option.Group,
// and the bot answers one card per group (a "search for a Zombie and a Swamp"
// takes one of each; "four creatures with different powers" takes one per
// power). The v2 select_object semantic carries no Group, so the grouping is
// rebuilt from the source's own ChangeType/DifferentPower spec and the
// candidates' printed faces; it is not card-specific.
func (s *Session) groupSearch(sd *protocol.SeatDecision, n *native, src protocol.ObjectRef) {
	if n == nil {
		return
	}
	sa := s.librarySearchSA(src)
	if sa == nil {
		return
	}
	spec := strings.TrimSpace(sa.Params["ChangeType"])
	each := strings.HasPrefix(spec, "EACH ")
	differentPower := strings.EqualFold(strings.TrimSpace(sa.Params["DifferentPower"]), "True")
	if !each && !differentPower {
		return
	}
	for _, c := range sd.Candidates {
		if c.Semantic.Kind != "select_object" {
			continue
		}
		t, err := target(c.Semantic, "choice")
		if err != nil || t == nil || t.Object == nil {
			continue
		}
		idx, ok := n.options[targetKey(*t)]
		if !ok || idx < 0 || idx >= len(n.d.Options) {
			continue
		}
		f := s.B.face(t.Object.CardName, false, nil)
		if f == nil {
			continue
		}
		switch {
		case each:
			n.d.Options[idx].Group = strconv.Itoa(eachFaceGroup(spec, f))
		default:
			n.d.Options[idx].Group = strconv.Itoa(f.Power())
		}
	}
	// ChangeNum$ above 1 raises each EACH group's cap; the engine carries it
	// in GroupLimit, and the bot's group-aware fill reads the same field. A
	// DifferentPower search's ChangeNum is the total pick ceiling (already
	// Max), not a per-power cap, so it must leave GroupLimit at one-per-group.
	if each {
		if per, err := strconv.Atoi(strings.TrimSpace(sa.Params["ChangeNum"])); err == nil && per > 1 {
			n.d.GroupLimit = per
		}
	}
}

// librarySearchSA is the source's first ChangeZone ability from a library. A
// spell's own ability chain is read first; a permanent's printed abilities
// (an activated search) follow.
func (s *Session) librarySearchSA(src protocol.ObjectRef) *cards.SA {
	f := s.B.face(src.CardName, false, nil)
	if f == nil {
		return nil
	}
	roots := append([]*cards.SA{}, f.Abilities...)
	if root := f.SpellAbility(); root != nil {
		roots = append(roots, root)
	}
	for _, root := range roots {
		for _, sa := range chain(f, root) {
			if sa.API == "ChangeZone" && strings.EqualFold(strings.TrimSpace(sa.Params["Origin"]), "Library") {
				return sa
			}
		}
	}
	return nil
}

// attackRestricted reports whether a printed restriction on a permanent
// attached to this attacker forbids attacking the given defender. The Vow
// cycle is the common shape: an Aura's CantAttack static with
// ValidCard$ Creature.EnchantedBy and Target$ You,Planeswalker.YouCtrl.
func (s *Session) attackRestricted(o *protocol.Observation, a protocol.ObjectRef, def protocol.TargetRef) bool {
	defController := ""
	switch {
	case def.Player != nil:
		defController = *def.Player
	case def.Object != nil:
		defController = def.Object.ControllerSeat
	default:
		return false
	}
	for _, p := range o.Players {
		for _, r := range p.Battlefield {
			if r.Permanent == nil || r.Permanent.AttachedTo == nil || r.Permanent.AttachedTo.Object == nil ||
				r.Permanent.AttachedTo.Object.ObjectID != a.ObjectID {
				continue
			}
			f := s.B.face(r.CardName, false, nil)
			if f == nil {
				continue
			}
			for _, st := range f.Statics {
				if !slices.Contains(strings.Split(st.Mode, ","), "CantAttack") {
					continue
				}
				valid := strings.TrimSpace(st.Params["ValidCard"])
				if !strings.Contains(valid, "EnchantedBy") && !strings.Contains(valid, "IsRemembered") {
					continue
				}
				target := strings.TrimSpace(st.Params["Target"])
				if target == "" {
					// A blanket restriction (Cessation, Pacifism) names no
					// defender: it forbids every attack.
					return true
				}
				// "You" is the attachment's controller; a planeswalker
				// target is restricted when its controller is that seat.
				if slices.Contains(strings.Split(target, ","), "You") && strings.EqualFold(defController, r.ControllerSeat) {
					return true
				}
				if strings.Contains(target, "Planeswalker.YouCtrl") && def.Object != nil && strings.EqualFold(defController, r.ControllerSeat) {
					return true
				}
			}
		}
	}
	return false
}

// isBattle reports whether the observed permanent's type line includes the
// battle card type.
func (s *Session) isBattle(id string) bool {
	if s.last == nil {
		return false
	}
	for _, p := range s.last.Observation.Players {
		for _, r := range p.Battlefield {
			if r.ObjectID == id && r.Characteristics != nil && slices.Contains(r.Characteristics.Types, "battle") {
				return true
			}
		}
	}
	return false
}

// contributionCast reports whether a spell can be paid by the contribution
// keywords (Convoke, Harmonize, Improvise, Waterbend) that the engine poses
// as a select_object payment rather than a payment plan. Such a cast stays a
// plain cast option in every priority window, so the autopay shadow's missing
// plan is not evidence that the engine withheld it.
func contributionCast(f *cards.Face) bool {
	for _, kw := range []string{"Convoke", "Harmonize", "Improvise", "Waterbend"} {
		if f.HasKeyword(kw) {
			return true
		}
	}
	return false
}

// ownNonMain reports whether the deciding seat is the active player and the
// step is not a main phase -- exactly the window where the native bot skips
// its payment planner.
func ownNonMain(v *view.View) bool {
	return v.Viewer == v.Active && v.Phase != "main1" && v.Phase != "main2"
}

// poolPays reports whether the deciding seat's floating pool alone covers the
// card's mana cost. A pool-only cast keeps its plain cast option even when
// the shadow plan is missing; an unpayable one does not.
func (s *Session) poolPays(v *view.View, f *cards.Face) bool {
	if f == nil || f.ManaCost == "" {
		return false
	}
	cost := rules.ParseCost(f.ManaCost)
	if cost.X > 0 || len(cost.Hybrid) > 0 || len(cost.Phyrexian) > 0 || cost.Snow > 0 || cost.Twobrid != nil {
		return false
	}
	var pool view.PlayerView
	for _, p := range v.Players {
		if p.ID == v.Viewer {
			pool = p
			break
		}
	}
	for i, c := range []string{"W", "U", "B", "R", "G"} {
		if i < len(cost.Colored) && int(cost.Colored[i]) > int(pool.Pool[c]) {
			return false
		}
	}
	total := int32(0)
	for _, n := range pool.Pool {
		total += n
	}
	return total >= cost.Generic+cost.Colored.Total()
}

// observedPower is the object's current projected power, the fact the
// engine's tap election publishes as Option.Value. A token's printed script
// is ambiguous (many share one name); the observation's characteristics are
// not.
func observedPower(o *protocol.Observation, id string) (int, bool) {
	for _, p := range o.Players {
		for _, r := range p.Battlefield {
			if r.ObjectID == id && r.Characteristics != nil && r.Characteristics.Power != nil {
				return int(*r.Characteristics.Power), true
			}
		}
	}
	return 0, false
}

// tapFloor is the total-power floor a source's tap cost imposes (Crew 3, or a
// tapXType<...withTotalPowerGE3> part), read with the same cost parser the
// engine uses. Zero means no set-level floor.
func (s *Session) tapFloor(src protocol.ObjectRef) int {
	f := s.B.face(src.CardName, false, nil)
	if f == nil {
		return 0
	}
	// Only the ability that is actually being cast/activated can impose a
	// tap-cost floor. An effect that asks the seat to tap permanents (a
	// spell's "tap up to N creatures") has no such cost; scanning every
	// printed ability would invent one.
	var root *cards.SA
	if src.Zone == "battlefield" {
		if i, ok := s.activated[src.ObjectID]; ok && i >= 0 && i < len(f.Abilities) {
			root = f.Abilities[i]
		}
	} else if s.last != nil {
		// The tap election's source may be the activated ability object on
		// the stack (Crew); its Source is the permanent whose ability Picked
		// recorded.
		for _, e := range s.last.Observation.Stack {
			if e.ObjectID != src.ObjectID || e.StackKind != "activated_ability" || e.Source == nil {
				continue
			}
			if sf := s.B.face(e.Source.CardName, false, nil); sf != nil {
				f = sf
				if i, ok := s.activated[e.Source.ObjectID]; ok && i >= 0 && i < len(f.Abilities) {
					root = f.Abilities[i]
				}
			}
			break
		}
	}
	if root == nil && src.Zone != "battlefield" {
		root = f.SpellAbility()
	}
	if root == nil {
		return 0
	}
	for _, sa := range chain(f, root) {
		cost := rules.ParseCost(sa.Params["Cost"])
		for _, part := range cost.TapPermanent {
			if part.MinPower > 0 {
				return int(part.MinPower)
			}
		}
	}
	return 0
}

// chainSA is the first ability in the source spell's chain (its spell
// ability and SubAbilities) that want accepts, or nil.
func (s *Session) chainSA(src protocol.ObjectRef, want func(*cards.SA) bool) *cards.SA {
	f := s.B.face(src.CardName, false, nil)
	if f == nil {
		return nil
	}
	for _, sa := range chain(f, f.SpellAbility()) {
		if want(sa) {
			return sa
		}
	}
	return nil
}

// hasAPI reports whether the source's asking ability (or, for a triggered
// ability the observation cannot single out, any of its triggers) reaches an
// SA with the given API. It is how the neutral world recovers the native
// option kind a lossy protocol purpose (the "other" select_object) hides.
func (s *Session) hasAPI(src protocol.ObjectRef, o *protocol.Observation, api string) bool {
	f, roots := s.abilityRoots(src, o)
	for _, root := range roots {
		if root == nil {
			continue
		}
		for _, sa := range chain(f, root) {
			if sa.API == api {
				return true
			}
		}
	}
	return false
}

// chain is root and the SubAbilities it leads to, in order.
func chain(f *cards.Face, root *cards.SA) []*cards.SA {
	var out []*cards.SA
	for sa := root; sa != nil && len(out) < 16; {
		out = append(out, sa)
		sub := sa.Params["SubAbility"]
		if sub == "" {
			break
		}
		sa = cards.ResolveSVar(f.SVars, sub)
	}
	return out
}

// handToLibrary is Brainstorm's "put cards from your hand on top": gorge's
// KChoose of hand_move options in hand order, whose answer order is the
// order the cards go on top. v2 selects the cards (select_object), then
// orders them (order_pick library_top).
func (s *Session) handToLibrary(sd *protocol.SeatDecision, src protocol.ObjectRef, sa *cards.SA) ([]mapping.NativeOp, *native, error) {
	first := sd.Candidates[0].Semantic
	key := "handmove|" + src.ObjectID
	n := s.continuing(key)
	if n == nil || (sd.Group.SubstepIndex == 0 && num(first, "selected_count") == 0) {
		lo, hi := num(first, "minimum"), num(first, "maximum")
		n = s.open(key, decision.Decision{Kind: decision.KChoose, Min: lo, Max: hi, Source: s.B.IDs.Of(src.ObjectID),
			Prompt: "Put cards on top", ResumeSA: sa})
		offered := map[string]bool{}
		for _, c := range sd.Candidates {
			if t, err := target(c.Semantic, "choice"); err == nil && t != nil && t.Object != nil {
				offered[t.Object.ObjectID] = true
			}
		}
		for _, p := range sd.Observation.Players {
			if p.Seat != sd.ActingSeat {
				continue
			}
			for _, o := range p.Hand {
				if offered[o.ObjectID] {
					n.options[o.ObjectID] = len(n.d.Options)
					n.d.Options = append(n.d.Options, decision.Option{Index: len(n.d.Options), Kind: "hand_move",
						Label: name(o.ObjectRef), Obj: s.B.IDs.Of(o.ObjectID), Player: s.seat})
				}
			}
		}
	}
	ops := make([]mapping.NativeOp, len(sd.Candidates))
	for i, c := range sd.Candidates {
		ops[i] = noOp
		if c.Semantic.Kind != "select_object" {
			continue
		}
		t, err := target(c.Semantic, "choice")
		if err != nil {
			return nil, nil, err
		}
		if t != nil && t.Object != nil {
			if k, ok := n.options[t.Object.ObjectID]; ok {
				ops[i] = mapping.NativeOp{Op: "choose", Option: k}
			}
		}
	}
	return ops, n, nil
}

// costKinds maps choose_cost_target's cost_kind to gorge's cost option kind.
var costKinds = map[string]string{"sacrifice": "sacrifice", "discard": "discard", "tap": "tapcost",
	"return_to_hand": "returncost", "exile": "exile_cost"}

func (s *Session) costTargets(sd *protocol.SeatDecision) ([]mapping.NativeOp, *native, error) {
	first := sd.Candidates[0].Semantic
	kind, ok := costKinds[str(first, "cost_kind")]
	if !ok {
		return nil, nil, nil
	}
	src, err := ref(first, "source")
	if err != nil {
		return nil, nil, err
	}
	d := decision.Decision{Kind: decision.KChoose, Min: num(first, "minimum"), Max: num(first, "maximum"),
		Source: s.B.IDs.Of(src.ObjectID), Prompt: "Pay " + kind}
	return s.pick(sd, "cost|"+src.ObjectID+"|"+kind, d, func(sem protocol.Semantic) (*protocol.TargetRef, error) {
		r, err := ref(sem, "candidate")
		if err != nil {
			return nil, err
		}
		t := protocol.ObjectTarget(r)
		return &t, nil
	}, func(t protocol.TargetRef) (decision.Option, error) {
		return decision.Option{Kind: kind, Label: name(*t.Object), Obj: s.B.IDs.Of(t.Object.ObjectID)}, nil
	}, "")
}

// --- combat ---

func hasKeyword(cv view.CardView, k string) bool {
	return slices.ContainsFunc(cv.Keywords, func(l string) bool { return strings.EqualFold(cards.KeywordHead(l), k) })
}

func battlefield(v *view.View, seat state.PlayerID) []view.CardView {
	for _, p := range v.Players {
		if p.ID == seat {
			return p.Battlefield
		}
	}
	return nil
}

func isCreature(cv view.CardView) bool {
	// CR 708.2: a face-down permanent on the battlefield is a 2/2 creature.
	// Native's view still prints the visible face's type line for it (see
	// Builder.printed), so the type words alone would miss a face-down land.
	if cv.FaceDown {
		return true
	}
	return slices.ContainsFunc(strings.Fields(cv.Types), func(t string) bool { return strings.EqualFold(t, "Creature") })
}

// attackKey names one (attacker, defender) pair.
func attackKey(attacker string, def *protocol.TargetRef) string {
	switch {
	case def == nil:
		return attacker + ">none"
	case def.Player != nil:
		return attacker + ">player:" + *def.Player
	case def.Object != nil:
		return attacker + ">object:" + def.Object.ObjectID
	}
	return attacker + ">?"
}

// attackOption builds gorge's attacker option for one (attacker, defender)
// pair. The v2 declare_attack semantic names a player or a planeswalker, and
// the native decision carries one option per pair: Player is the defender
// and Battle is the planeswalker's object (0 for a player).
func (s *Session) attackOption(n *native, a protocol.ObjectRef, def protocol.TargetRef, label string, required bool) (int, bool) {
	key := attackKey(a.ObjectID, &def)
	if i, ok := n.options[key]; ok {
		return i, true
	}
	o := decision.Option{Index: len(n.d.Options), Kind: "attacker", Label: label,
		Obj: s.B.IDs.Of(a.ObjectID), Required: required}
	switch {
	case def.Player != nil:
		p, err := Seat(*def.Player)
		if err != nil {
			return 0, false
		}
		o.Player = p
	case def.Object != nil:
		p, err := Seat(def.Object.ControllerSeat)
		if err != nil {
			return 0, false
		}
		// A battle is protected by an opponent of its controller (CR
		// 310.8); the v2 object names only the battle, whose controller can
		// be the attacker's own seat. The defending player is the other
		// seat in a two-player game.
		if s.isBattle(def.Object.ObjectID) {
			p = 1 - p
		}
		o.Player = p
		o.Battle = s.B.IDs.Of(def.Object.ObjectID)
	default:
		return 0, false
	}
	n.options[key] = o.Index
	n.d.Options = append(n.d.Options, o)
	return o.Index, true
}

// attackers builds gorge's declaration from the v2 candidate stream and the
// observed board. The current substep's candidates are authoritative for the
// unit under decision (every legal defender, a player or a planeswalker);
// every other legal attacker gets its player option so the whole-declaration
// bot sees the future substeps too.
func (s *Session) attackers(sd *protocol.SeatDecision, v *view.View) ([]mapping.NativeOp, *native, error) {
	opp := 1 - s.seat
	attached := map[state.ObjID][]view.CardView{}
	for _, p := range v.Players {
		for _, cv := range p.Battlefield {
			if cv.AttachedTo != 0 {
				attached[cv.AttachedTo] = append(attached[cv.AttachedTo], cv)
			}
		}
	}
	n := s.continuing("attack")
	required := map[string]bool{}
	if n == nil || sd.Group.SubstepIndex == 0 {
		n = s.open("attack", decision.Decision{Kind: decision.KAttackers, Min: 0, Prompt: "declare attackers"})
		// CR 508.1d requirements: a goaded or self-"attacks each combat if
		// able" creature must attack. gorge marks those options Required;
		// the v2 declare_attack semantic does not carry the flag, but the
		// observation reports the permanent's "goaded" status and the
		// printed static registry carries a self-scoped MustAttack.
		for _, p := range sd.Observation.Players {
			for _, r := range p.Battlefield {
				if r.Permanent != nil && slices.Contains(r.Permanent.Statuses, "goaded") {
					required[r.ObjectID] = true
				}
				// A self-scoped printed MustAttack (Volatile Rig's "CARDNAME
				// attacks each combat if able") is not a keyword and never
				// reaches the observation's keyword list, so the printed face
				// is the only source. A conditional static is left to the
				// engine's own offer.
				if f := s.requirementFace(r); f != nil {
					for _, st := range f.Statics {
						if selfSpec(mustAttackScope(st)) {
							required[r.ObjectID] = true
						}
					}
				}
				// A continuous Goad$ True static on an attached Aura or
				// Equipment (Coercive Impetus) never becomes a Goads entry,
				// so the statuses alone miss it. Its goad applies to the
				// creature it is attached to.
				if r.Permanent == nil || r.Permanent.AttachedTo == nil || r.Permanent.AttachedTo.Object == nil {
					continue
				}
				f := s.B.face(r.CardName, false, nil)
				if f == nil {
					continue
				}
				for _, st := range f.Statics {
					if strings.EqualFold(strings.TrimSpace(st.Params["Goad"]), "True") || attachedMustAttack(st) {
						required[r.Permanent.AttachedTo.Object.ObjectID] = true
						break
					}
				}
			}
		}
		s.typeScopedRequirements(&sd.Observation, sd.ActingSeat, required)
		// The native decision carries one unit per group substep. At the
		// first substep the whole declaration is known from the board; the
		// current substep's candidates are authoritative for the unit under
		// decision, and the remaining units are the first SubstepCount legal
		// attackers in the engine's battlefield order.
		player := protocol.PlayerTarget("p" + strconv.Itoa(int(opp)))
		var defenders []protocol.TargetRef
		// A battle is attacked by a player other than its protector (CR
		// 310.8): the acting seat's own battles are the attackable ones, and
		// an opponent's battle is one this seat protects. Planeswalkers are
		// the reverse -- only an opponent's can be attacked.
		for _, p := range sd.Observation.Players {
			for _, r := range p.Battlefield {
				if r.Characteristics == nil {
					continue
				}
				attackable := slices.Contains(r.Characteristics.Types, "planeswalker")
				if p.Seat == sd.ActingSeat {
					attackable = slices.Contains(r.Characteristics.Types, "battle")
				}
				if attackable {
					defenders = append(defenders, protocol.ObjectTarget(r.ObjectRef))
				}
			}
		}
		var legal []protocol.ObjectRef
		for _, p := range sd.Observation.Players {
			if p.Seat != sd.ActingSeat {
				continue
			}
			for _, r := range p.Battlefield {
				cv, ok := findCard(battlefield(v, s.seat), s.B.IDs.Of(r.ObjectID))
				if !ok || !isCreature(cv) || cv.Tapped ||
					(hasKeyword(cv, "Defender") && !s.mayAttackThroughDefender(r.ObjectID, cv)) ||
					(cv.SummonSick && !hasKeyword(cv, "Haste")) || s.cantAttack(cv, attached[cv.ID]) ||
					s.barredByType(&sd.Observation, r, "CantAttack") {
					continue
				}
				legal = append(legal, r.ObjectRef)
			}
		}
		nUnits := int(sd.Group.SubstepCount)
		if nUnits < 1 {
			nUnits = 1
		}
		addUnit := func(a protocol.ObjectRef) bool {
			added := false
			if !s.attackRestricted(&sd.Observation, a, player) {
				if _, ok := s.attackOption(n, a, player, "Attack with "+name(a)+" at p"+strconv.Itoa(int(opp)), required[a.ObjectID]); ok {
					added = true
				}
			}
			for _, def := range defenders {
				if s.attackRestricted(&sd.Observation, a, def) {
					continue
				}
				label := "Attack with " + name(a)
				if def.Object != nil {
					label += " (planeswalker: " + name(*def.Object) + ")"
				}
				if _, ok := s.attackOption(n, a, def, label, required[a.ObjectID]); ok {
					added = true
				}
			}
			return added
		}
		if nUnits == 1 {
			// A one-unit declaration: only the current unit's own offer.
			for _, c := range sd.Candidates {
				a, err := ref(c.Semantic, "attacker")
				if err != nil {
					return nil, nil, err
				}
				def, err := target(c.Semantic, "defender")
				if err != nil {
					return nil, nil, err
				}
				if def == nil {
					continue
				}
				label := "Attack with " + name(a)
				if def.Player != nil {
					label += " at " + *def.Player
				} else if def.Object != nil {
					label += " (planeswalker: " + name(*def.Object) + ")"
				}
				s.attackOption(n, a, *def, label, required[a.ObjectID])
			}
		} else {
			// The group's substeps follow the engine's own battlefield order:
			// native's newDeclare walks its options in order and keeps the
			// first-seen unit, and those options are the board-order legal
			// attackers. `legal` is built in that same order, so the current
			// unit must keep its board position rather than moving first.
			if nUnits > len(legal) {
				nUnits = len(legal)
			}
			seen := map[string]bool{}
			added := 0
			for i := 0; i < len(legal) && added < nUnits; i++ {
				if !addUnit(legal[i]) {
					continue // every defender is restricted for this attacker
				}
				seen[legal[i].ObjectID] = true
				added++
			}
			// The current unit is always part of the native declaration; a
			// board-order miss appends it without dropping another unit.
			for _, c := range sd.Candidates {
				a, err := ref(c.Semantic, "attacker")
				if err != nil {
					return nil, nil, err
				}
				if !seen[a.ObjectID] {
					addUnit(a)
					seen[a.ObjectID] = true
				}
			}
		}
		// Any candidate pair the board-derived shape did not name is still
		// part of the native offer.
		for _, c := range sd.Candidates {
			a, err := ref(c.Semantic, "attacker")
			if err != nil {
				return nil, nil, err
			}
			def, err := target(c.Semantic, "defender")
			if err != nil {
				return nil, nil, err
			}
			if def == nil {
				continue
			}
			label := "Attack with " + name(a)
			if def.Player != nil {
				label += " at " + *def.Player
			} else if def.Object != nil {
				label += " (planeswalker: " + name(*def.Object) + ")"
			}
			s.attackOption(n, a, *def, label, required[a.ObjectID])
		}
		n.d.Max = len(n.d.Options)
	}
	ops := make([]mapping.NativeOp, len(sd.Candidates))
	for i, c := range sd.Candidates {
		ops[i] = noOp
		a, err := ref(c.Semantic, "attacker")
		if err != nil {
			return nil, nil, err
		}
		def, err := target(c.Semantic, "defender")
		if err != nil {
			return nil, nil, err
		}
		unit := s.B.IDs.Of(a.ObjectID)
		switch {
		case def == nil:
			ops[i] = mapping.NativeOp{Op: "none", Option: -1, Unit: unit}
		case def.Player != nil:
			k, ok := n.options[attackKey(a.ObjectID, def)]
			if !ok {
				label := "Attack with " + name(a) + " at " + *def.Player
				k, ok = s.attackOption(n, a, *def, label, required[a.ObjectID])
			}
			if ok {
				ops[i] = mapping.NativeOp{Op: "choose", Option: k, Unit: unit}
			}
		case def.Object != nil:
			k, ok := n.options[attackKey(a.ObjectID, def)]
			if !ok {
				label := "Attack with " + name(a) + " (planeswalker: " + name(*def.Object) + ")"
				k, ok = s.attackOption(n, a, *def, label, required[a.ObjectID])
			}
			if ok {
				ops[i] = mapping.NativeOp{Op: "choose", Option: k, Unit: unit}
			}
		}
	}
	n.d.Max = len(n.d.Options)
	return ops, n, nil
}

func findCard(cvs []view.CardView, id state.ObjID) (view.CardView, bool) {
	for _, cv := range cvs {
		if cv.ID == id {
			return cv, true
		}
	}
	return view.CardView{}, false
}

// protected reports whether attacker a has protection from a blocker of
// these colors (Guardian of the Guildpact's "from monocolored", or from a
// color), read off its Forge keyword lines.
func protected(a view.CardView, colors []string) bool {
	for _, k := range a.Keywords {
		k = strings.ToLower(k)
		if !strings.HasPrefix(k, "protection") {
			continue
		}
		if strings.Contains(k, "monocolor") && len(colors) == 1 {
			return true
		}
		for _, c := range colors {
			if strings.Contains(k, "card."+c) || strings.HasSuffix(k, " "+c) || strings.HasSuffix(k, ":"+c) {
				return true
			}
		}
	}
	return false
}

// blockKey names one (blocker, attacker) pair.
func blockKey(blocker, attacker string) string { return blocker + ">" + attacker }

// blockOption builds gorge's block option for one pair.
func (s *Session) blockOption(v *view.View, n *native, b, a protocol.ObjectRef, label string, required bool) int {
	key := blockKey(b.ObjectID, a.ObjectID)
	if i, ok := n.options[key]; ok {
		if required {
			n.d.Options[i].Required = true
			n.d.Options[i].BlockMust = true
		}
		return i
	}
	o := decision.Option{Index: len(n.d.Options), Kind: "block", Label: label,
		Obj: s.B.IDs.Of(b.ObjectID), Attacker: s.B.IDs.Of(a.ObjectID), Player: s.seat,
		Group: "blocker:" + b.ObjectID, Required: required, BlockMust: required}
	o.MinBlockers, o.MaxBlockers = s.blockBounds(a, v)
	n.options[key] = o.Index
	n.d.Options = append(n.d.Options, o)
	return o.Index
}

// blockers builds gorge's block declaration from the v2 candidate stream and
// the observed board. The current substep's candidates are authoritative for
// the blocker under decision; other untapped blockers get their board-derived
// pairs so the whole-declaration bot sees the future substeps.
func (s *Session) blockers(sd *protocol.SeatDecision, v *view.View) ([]mapping.NativeOp, *native, error) {
	n := s.continuing("block")
	if n == nil || sd.Group.SubstepIndex == 0 {
		n = s.open("block", decision.Decision{Kind: decision.KBlockers, Min: 0, Prompt: "declare blockers"})
		var attackers []view.CardView
		for _, cv := range battlefield(v, 1-s.seat) {
			if cv.Attacking {
				attackers = append(attackers, cv)
			}
		}
		colors := map[string][]string{}
		refs := map[string]protocol.ObjectRef{}
		records := map[string]protocol.ObjectRecord{}
		for _, p := range sd.Observation.Players {
			for _, o := range p.Battlefield {
				refs[o.ObjectID] = o.ObjectRef
				records[o.ObjectID] = o
				if o.Characteristics != nil {
					colors[o.ObjectID] = o.Characteristics.Colors
				}
			}
		}
		// attached maps a permanent's gorge id to the permanents attached to
		// it, so a blocker's granted statics (Copper Carapace's "equipped
		// creature can't block") are visible where the view has no static
		// registry.
		attached := map[state.ObjID][]view.CardView{}
		for _, p := range v.Players {
			for _, cv := range p.Battlefield {
				if cv.AttachedTo != 0 {
					attached[cv.AttachedTo] = append(attached[cv.AttachedTo], cv)
				}
			}
		}
		first, err := ref(sd.Candidates[0].Semantic, "blocker")
		if err != nil {
			return nil, nil, err
		}
		cur := first.ObjectID
		// A non-public block restriction (a CantBlockBy effect with no printed
		// static, e.g. an "can't be blocked this turn" rider) is invisible to
		// the observation, but it is readable from this substep's authoritative
		// offer: an attacker the current blocker can block by every public
		// fact whose pair the engine did NOT offer is restricted for this
		// declaration. Mark it so the derived pairs for the OTHER blockers do
		// not invent a pair the engine never offers. Only when the current
		// blocker got at least one pair: an all-none offer is equally the
		// signature of a blocker that cannot block, which says nothing about
		// the attackers.
		offered := map[state.ObjID]bool{}
		for _, c := range sd.Candidates {
			if c.Semantic.Kind != "declare_block" {
				continue
			}
			if a, err := optRef(c.Semantic, "attacker"); err == nil && a != nil {
				offered[s.B.IDs.Of(a.ObjectID)] = true
			}
		}
		restricted := map[state.ObjID]bool{}
		if len(offered) > 0 {
			if curCV, ok := findCard(battlefield(v, s.seat), s.B.IDs.Of(cur)); ok {
				for _, a := range attackers {
					if !offered[a.ID] && s.canBlockPair(v, curCV, a, colors) {
						restricted[a.ID] = true
					}
				}
			}
		}
		// The group's substeps follow the engine's battlefield order: native's
		// newDeclare walks its options in order and keeps the first-seen unit.
		// The current blocker keeps its board position (its candidate stream is
		// authoritative for its own pairs); every other untapped, able blocker
		// gets its board-derived pairs. The gates are combat.CanBlock's
		// public-fact half (see canBlockPair), so the bot's whole-declaration
		// view does not see a pair the engine's own offer omitted.
		for _, b := range battlefield(v, s.seat) {
			if !isCreature(b) || b.Tapped {
				continue
			}
			bv2 := s.B.IDs.V2(b.ID)
			bref, ok := refs[bv2]
			if !ok {
				continue
			}
			if bv2 == cur {
				// The current substep's candidates: exactly the native
				// decision's offered pairs for this blocker.
				for _, c := range sd.Candidates {
					if c.Semantic.Kind != "declare_block" {
						continue
					}
					cb, err := ref(c.Semantic, "blocker")
					if err != nil {
						return nil, nil, err
					}
					if cb.ObjectID != cur {
						continue
					}
					a, err := optRef(c.Semantic, "attacker")
					if err != nil {
						return nil, nil, err
					}
					if a == nil {
						continue
					}
					s.blockOption(v, n, cb, *a, name(cb)+" blocks "+name(*a), s.mustBlock(b, attached[b.ID]))
				}
				continue
			}
			if s.cantBlock(b, attached[b.ID]) || s.barredByType(&sd.Observation, records[bv2], "CantBlock") {
				continue
			}
			for _, a := range attackers {
				if restricted[a.ID] || !s.canBlockPair(v, b, a, colors) {
					continue
				}
				aref, ok := refs[s.B.IDs.V2(a.ID)]
				if !ok {
					continue
				}
				s.blockOption(v, n, bref, aref, b.Name+" blocks "+a.Name, s.mustBlock(b, attached[b.ID]))
			}
		}
		n.d.Max = len(n.d.Options)
	}
	ops := make([]mapping.NativeOp, len(sd.Candidates))
	for i, c := range sd.Candidates {
		ops[i] = noOp
		b, err := ref(c.Semantic, "blocker")
		if err != nil {
			return nil, nil, err
		}
		a, err := optRef(c.Semantic, "attacker")
		if err != nil {
			return nil, nil, err
		}
		unit := s.B.IDs.Of(b.ObjectID)
		if a == nil {
			ops[i] = mapping.NativeOp{Op: "none", Option: -1, Unit: unit}
			continue
		}
		k, ok := n.options[blockKey(b.ObjectID, a.ObjectID)]
		if !ok {
			cv, _ := s.cardByV2(v, b.ObjectID)
			k = s.blockOption(v, n, b, *a, name(b)+" blocks "+name(*a), s.mustBlock(cv, nil))
		}
		ops[i] = mapping.NativeOp{Op: "choose", Option: k, Unit: unit}
	}
	n.d.Max = len(n.d.Options)
	return ops, n, nil
}

// mustBlock reports whether the blocker's printed statics make blocking a
// duty (Watchdog's "blocks each combat if able"). The observation carries no
// static registry and the view has no such field, so the printed face is the
// source. Attached grants use the same mode with an EnchantedBy/EquippedBy
// scope; a conditional static is not read (the engine's offer remains the
// authority there).
func (s *Session) mustBlock(cv view.CardView, attached []view.CardView) bool {
	check := func(host view.CardView, scoped bool) bool {
		f := s.staticFace(host)
		if f == nil {
			return false
		}
		for _, st := range f.Statics {
			if !slices.Contains(strings.Split(st.Mode, ","), "MustBlock") {
				continue
			}
			valid := strings.TrimSpace(st.Params["ValidCreature"])
			if scoped {
				if !strings.Contains(valid, "EnchantedBy") && !strings.Contains(valid, "EquippedBy") {
					continue
				}
			} else if valid != "" && !strings.Contains(valid, "Self") {
				continue
			}
			if unresolvedStatic(st) {
				continue
			}
			return true
		}
		return false
	}
	if check(cv, false) {
		return true
	}
	for i := range attached {
		if check(attached[i], true) {
			return true
		}
	}
	return false
}

// blockBounds is the blocker-count bounds an attacker's block options publish,
// zero for no bound: the literal Min$ and Max$ of its printed MinMaxBlocker
// statics, with Menace folded in as a floor of two. The bot declines its combat
// simulation on any published bound.
func (s *Session) blockBounds(a protocol.ObjectRef, v *view.View) (lo, hi int) {
	if f := s.abilityFace(a); f != nil {
		for _, st := range f.Statics {
			if !slices.Contains(strings.Split(st.Mode, ","), "MinMaxBlocker") || !selfSpec(st.Params["ValidCard"]) || unresolvedStatic(st) {
				continue
			}
			if n, err := strconv.Atoi(strings.TrimSpace(st.Params["Min"])); err == nil && n > lo {
				lo = n
			}
			if n, err := strconv.Atoi(strings.TrimSpace(st.Params["Max"])); err == nil && (hi == 0 || n < hi) {
				hi = n
			}
		}
	}
	if cv, ok := s.cardByV2(v, a.ObjectID); ok && lo < 2 && hasKeyword(cv, "Menace") {
		lo = 2
	}
	return lo, hi
}

// canBlockPair is combat.CanBlock's per-pair rule rebuilt from the seat's view
// and the observation's current colours: the gates that read only public
// battlefield facts. The candidate stream is authoritative for the blocker
// under decision; this keeps the board-derived pairs for the OTHER blockers
// inside the same legality, so the whole-declaration bot cannot choose a pair
// the engine's own offer would have omitted.
func (s *Session) canBlockPair(v *view.View, b, a view.CardView, colors map[string][]string) bool {
	bv2, av2 := s.B.IDs.V2(b.ID), s.B.IDs.V2(a.ID)
	bc, ac := colors[bv2], colors[av2]
	if hasKeyword(a, "Flying") && !hasKeyword(b, "Flying") && !hasKeyword(b, "Reach") {
		return false
	}
	// CR 702.28: shadow pairs only with shadow, in either direction.
	if hasKeyword(a, "Shadow") != hasKeyword(b, "Shadow") {
		return false
	}
	// CR 702.31b: horsemanship is attacker-keyed (a horsemanship creature may
	// block a creature without it, but not the other way round).
	if hasKeyword(a, "Horsemanship") && !hasKeyword(b, "Horsemanship") {
		return false
	}
	artifact := hasType(b, "Artifact")
	// CR 702.36: Fear -- only artifact or black creatures block it.
	if hasKeyword(a, "Fear") && !artifact && !slices.Contains(bc, "black") {
		return false
	}
	// CR 702.13a: Intimidate -- artifact creatures and creatures sharing a
	// colour with the attacker.
	if hasKeyword(a, "Intimidate") && !artifact && !sharesColour(bc, ac) {
		return false
	}
	// CR 702.110a: Skulk -- a blocker with greater power cannot block.
	if hasKeyword(a, "Skulk") && b.Power > a.Power {
		return false
	}
	// CR 702.157b: a suspected creature can't block.
	if b.Flags&view.FlagSuspected != 0 {
		return false
	}
	// CR 702.86: an unleashed creature with a +1/+1 counter can't block.
	if hasKeyword(b, "Unleash") && b.Counters["P1P1"] > 0 {
		return false
	}
	if protected(a, bc) || s.cantBlockAttacker(b, a) {
		return false
	}
	// CR 702.14: landwalk -- the attacker is unblockable while the defending
	// player controls a land its filter names.
	if s.landwalkEvades(v, a) {
		return false
	}
	return true
}

// landwalkEvades reports whether a's Landwalk filter matches a land this seat
// controls. The filter is read with combat.LandwalkSpec, the same parser the
// engine's evasion uses, and matched against the view's derived type words.
func (s *Session) landwalkEvades(v *view.View, a view.CardView) bool {
	for _, line := range a.Keywords {
		spec, ok := combat.LandwalkSpec(line)
		if !ok || spec == "" {
			continue
		}
		for _, land := range battlefield(v, s.seat) {
			if matchesLandSpec(land, spec) {
				return true
			}
		}
	}
	return false
}

// matchesLandSpec reports whether a land's (derived) type line satisfies a
// landwalk filter: the land itself, then every dot-separated qualifier ("Snow",
// "nonBasic", "Legendary", a basic land subtype).
func matchesLandSpec(cv view.CardView, spec string) bool {
	words := strings.Fields(cv.Types)
	has := func(w string) bool {
		return slices.ContainsFunc(words, func(t string) bool { return strings.EqualFold(t, w) })
	}
	if !has("Land") {
		return false
	}
	for _, part := range strings.Split(spec, ".") {
		part = strings.TrimSpace(part)
		switch {
		case part == "" || strings.EqualFold(part, "Land"):
		case strings.EqualFold(part, "nonBasic"):
			if has("Basic") {
				return false
			}
		default:
			if !has(part) {
				return false
			}
		}
	}
	return true
}

func hasType(cv view.CardView, t string) bool {
	return slices.ContainsFunc(strings.Fields(cv.Types), func(w string) bool { return strings.EqualFold(w, t) })
}

func sharesColour(a, b []string) bool {
	for _, c := range a {
		if slices.Contains(b, c) {
			return true
		}
	}
	return false
}

// cantBlock reports whether a printed static forbids this creature from
// blocking: its own unconditional "CARDNAME can't block" (Aesthir Glider), or
// a CantBlock static on a permanent attached to it (Copper Carapace's
// "equipped creature gets +2/+2 and can't block"). The v2 view carries no
// static registry, so the printed faces are the only source.
func (s *Session) cantBlock(cv view.CardView, attached []view.CardView) bool {
	// A derived keyword grant ("CARDNAME can't block." / the compound
	// "CARDNAME can't attack or block.") is a layer-6 fact the view already
	// carries; native's BlockRestricted reads the same derived list, so the
	// board-derived pairs must honour it too.
	for _, k := range cv.Keywords {
		switch cards.HiddenKeywordOf(k) {
		case cards.HiddenKeywordCantBlock, cards.HiddenKeywordCantAttackOrBlock:
			return true
		}
	}
	if f := s.staticFace(cv); f != nil {
		for _, st := range f.Statics {
			if !slices.Contains(strings.Split(st.Mode, ","), "CantBlock") {
				continue
			}
			if unresolvedStatic(st) {
				return true // a conditional gate we cannot evaluate is not proven legal
			}
			valid := strings.TrimSpace(st.Params["ValidCard"])
			if valid == "" || selfSpec(valid) {
				return true
			}
		}
	}
	for i := range attached {
		f := s.staticFace(attached[i])
		if f == nil {
			continue
		}
		for _, st := range f.Statics {
			if !slices.Contains(strings.Split(st.Mode, ","), "CantBlock") {
				continue
			}
			if unresolvedStatic(st) {
				return true // a conditional gate we cannot evaluate is not proven legal
			}
			valid := strings.TrimSpace(st.Params["ValidCard"])
			if strings.Contains(valid, "EquippedBy") || strings.Contains(valid, "EnchantedBy") {
				return true
			}
		}
	}
	return false
}

// selfSpec reports whether a Forge ValidCard$/ValidCreature$ spec names the
// static's own host. Forge spells it Card.Self, Creature.Self or CARDNAME;
// any dot-separated "Self" qualifier is the same anchor.
func selfSpec(spec string) bool {
	for _, part := range strings.Split(strings.ToLower(strings.TrimSpace(spec)), ".") {
		if part == "self" || part == "cardname" {
			return true
		}
	}
	return false
}

// attackRestriction reports whether st's printed CantAttack gate applies,
// and whether the observation proves it. A condition the reconstruction
// cannot evaluate is unknown, not unconditional: the caller drops a derived
// whole-declaration option rather than inventing a legal attacker. The two
// families the observation does carry are IsPresent$/PresentCompare$ (a
// battlefield census) and UnlessDefender$ (the defending seat's board).
func (s *Session) attackRestriction(st cards.Static, id state.ObjID) (applies, known bool) {
	if target := strings.TrimSpace(st.Params["Target"]); target != "" {
		// The observation carries no attacked-player history. The engine's
		// own offer is authoritative for a player-scoped Target gate (Port
		// Razer's "can't attack a player it has already attacked this turn");
		// do not invent a restriction it did not apply.
		if strings.EqualFold(target, "Player.attackedBySourceThisTurn") {
			return false, true
		}
		return false, false
	}
	for _, p := range []string{"CheckSVar", "SVarCompare", "CheckSecondSVar", "SecondSVarCompare",
		"IsPresent2", "PresentZone", "Condition", "ConditionCompare"} {
		if strings.TrimSpace(st.Params[p]) != "" {
			return false, false
		}
	}
	if spec := strings.TrimSpace(st.Params["IsPresent"]); spec != "" {
		if s.last == nil {
			return false, false
		}
		cmp := strings.TrimSpace(st.Params["PresentCompare"])
		if cmp == "" {
			cmp = "GE1"
		}
		n := 0
		for _, p := range s.last.Observation.Players {
			for _, r := range p.Battlefield {
				if s.presentSpec(spec, r, s.last.ActingSeat) {
					n++
				}
			}
		}
		return compareCount(n, cmp), true
	}
	if spec := strings.TrimSpace(st.Params["UnlessDefender"]); spec != "" {
		if s.last == nil {
			return false, false
		}
		ok, known := s.defenderHas(spec)
		if !known {
			return false, false
		}
		return !ok, true
	}
	return true, true
}

// presentSpec reports whether one battlefield record matches one comma
// alternative of an IsPresent$ spec: a type word (or "Permanent") plus the
// optional .YouCtrl/.OppCtrl scope. Subtypes count, as the filter grammar's
// type predicates do.
func (s *Session) presentSpec(spec string, r protocol.ObjectRecord, seat string) bool {
	for _, alt := range strings.Split(spec, ",") {
		alt = strings.TrimSpace(alt)
		if alt == "" || r.Characteristics == nil {
			continue
		}
		parts := strings.Split(alt, ".")
		scope := ""
		if len(parts) > 1 {
			scope = strings.Join(parts[1:], ".")
		}
		if strings.Contains(strings.ToLower(scope), "youctrl") && r.ControllerSeat != seat {
			continue
		}
		if strings.Contains(strings.ToLower(scope), "oppctrl") && r.ControllerSeat == seat {
			continue
		}
		words := strings.FieldsFunc(parts[0], func(r rune) bool { return r == '+' || r == '&' || r == ' ' })
		if len(words) == 0 {
			continue
		}
		match := true
		for _, w := range words {
			if strings.EqualFold(w, "Permanent") {
				continue
			}
			if !s.recordHasWord(r, w) {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

// defenderHas reports whether any opposing seat controls a permanent the
// UnlessDefender$ spec names: controls<Type> (the common land/enchantment
// cycle) or Permanent.enchanted. known is false for a form the observation
// cannot answer.
func (s *Session) defenderHas(spec string) (ok, known bool) {
	known = true
	for _, alt := range strings.Split(spec, ",") {
		alt = strings.TrimSpace(alt)
		if alt == "" {
			continue
		}
		lower := strings.ToLower(alt)
		switch {
		case strings.HasPrefix(lower, "controls"):
			word := alt[len("controls"):]
			if word == "" {
				known = false
				continue
			}
			for _, p := range s.last.Observation.Players {
				if p.Seat == s.last.ActingSeat {
					continue
				}
				for _, r := range p.Battlefield {
					if s.recordHasWord(r, word) {
						return true, true
					}
				}
			}
		case lower == "permanent.enchanted":
			opp := map[string]bool{}
			for _, p := range s.last.Observation.Players {
				if p.Seat != s.last.ActingSeat {
					opp[p.Seat] = true
				}
			}
			for _, p := range s.last.Observation.Players {
				for _, r := range p.Battlefield {
					if r.Permanent == nil || r.Permanent.AttachedTo == nil || r.Permanent.AttachedTo.Object == nil {
						continue
					}
					if !s.recordHasWord(r, "enchantment") {
						continue
					}
					if opp[r.Permanent.AttachedTo.Object.ControllerSeat] {
						return true, true
					}
				}
			}
		default:
			return false, false
		}
	}
	return false, known
}

// compareCount answers a Forge PresentCompare$/SVarCompare$ form ("EQ0",
// "GE1", ...) for a board count.
func compareCount(n int, cmp string) bool {
	cmp = strings.ToUpper(strings.TrimSpace(cmp))
	op, rest := "", ""
	for _, p := range []string{"EQ", "GE", "LE", "GT", "LT", "NE"} {
		if strings.HasPrefix(cmp, p) {
			op, rest = p, cmp[len(p):]
			break
		}
	}
	if op == "" {
		return false
	}
	want, err := strconv.Atoi(strings.TrimSpace(rest))
	if err != nil {
		return false
	}
	switch op {
	case "EQ":
		return n == want
	case "GE":
		return n >= want
	case "LE":
		return n <= want
	case "GT":
		return n > want
	case "LT":
		return n < want
	case "NE":
		return n != want
	}
	return false
}

// recordHasWord is characteristicsHasWord plus the printed face's words when
// the observation omits subtypes entirely (a lean projection or a synthetic
// fixture): the protocol splits subtypes out of Types, and a condition naming
// a creature or basic-land subtype must still find it there. A record that
// does carry a subtype list is authoritative -- a derived line that lost a
// printed subtype must not regain it.
func (s *Session) recordHasWord(r protocol.ObjectRecord, w string) bool {
	if characteristicsHasWord(r.Characteristics, w) {
		return true
	}
	if r.Characteristics == nil || len(r.Characteristics.Subtypes) != 0 {
		return false
	}
	f := s.B.face(r.CardName, r.Token && !r.Copy, r.Characteristics)
	if f == nil {
		return false
	}
	for _, t := range f.Types {
		if strings.EqualFold(t, w) {
			return true
		}
	}
	return false
}

// characteristicsHasWord reports whether an observation's full type line
// (supertypes, card types, subtypes) carries w. The protocol splits subtypes
// out of Types, so a condition naming a creature type or a basic land type
// must read all three lists.
func characteristicsHasWord(c *protocol.Characteristics, w string) bool {
	if c == nil {
		return false
	}
	for _, list := range [][]string{c.Supertypes, c.Types, c.Subtypes} {
		for _, t := range list {
			if strings.EqualFold(t, w) {
				return true
			}
		}
	}
	return false
}

// unresolvedStatic reports whether a static carries a printed condition whose
// live value the v2 observation does not carry. The derived attack/block lists
// for units the current substep's candidates do not name treat such a static
// as a restriction the seat cannot prove absent, so they omit the option: the
// engine's own offer is the authority, and inventing a pair it never offered
// cannot be repaired later in the declaration. A pair the current substep does
// offer is authoritative and bypasses these reads.
func unresolvedStatic(st cards.Static) bool {
	for _, param := range []string{"CheckSVar", "SVarCompare", "CheckSecondSVar", "SecondSVarCompare",
		"IsPresent", "IsPresent2", "PresentCompare", "PresentZone", "Condition",
		"ConditionCompare", "UnlessDefender", "Target"} {
		if strings.TrimSpace(st.Params[param]) != "" {
			return true
		}
	}
	return false
}

// cantAttack reports whether an unconditional printed static forbids this
// creature from attacking. A static carrying any condition parameter is not
// evaluated (the observation carries no live SVar or board gate), so the
// engine's own offer remains the authority there.
func (s *Session) cantAttack(cv view.CardView, attached []view.CardView) bool {
	// A derived keyword grant ("CARDNAME can't attack." / the compound
	// "CARDNAME can't attack or block.") is already in the view's keyword
	// list, the same derived fact native's AttackBlocked reads.
	for _, k := range cv.Keywords {
		switch cards.HiddenKeywordOf(k) {
		case cards.HiddenKeywordCantAttack, cards.HiddenKeywordCantAttackOrBlock:
			return true
		}
	}
	f := s.staticFace(cv)
	if f == nil {
		return false
	}
	for _, st := range f.Statics {
		if !slices.Contains(strings.Split(st.Mode, ","), "CantAttack") {
			continue
		}
		valid := strings.TrimSpace(st.Params["ValidCard"])
		if valid != "" && !selfSpec(valid) {
			continue
		}
		applies, known := s.attackRestriction(st, cv.ID)
		if !known {
			// The observation cannot prove this gate absent. The engine's
			// own offer remains authoritative for the current substep, but
			// a derived whole-declaration option must not invent a pair the
			// engine's conditional static excluded (Goblin Cohort,
			// Deep-Sea Serpent). With no observation at all (the rule's
			// unit-test shape) the static is not read as unconditional.
			if s.last != nil {
				return true
			}
			continue
		}
		if applies {
			return true
		}
	}
	// A printed CantAttack static on a permanent attached to this creature
	// (Recumbent Bliss: "Enchanted creature can't attack or block") is
	// unconditional when it carries no condition or Target. The Vow cycle's
	// defender-scoped aura carries Target$ and is handled by attackRestricted.
	for i := range attached {
		f := s.staticFace(attached[i])
		if f == nil {
			continue
		}
		for _, st := range f.Statics {
			if !slices.Contains(strings.Split(st.Mode, ","), "CantAttack") {
				continue
			}
			valid := strings.TrimSpace(st.Params["ValidCard"])
			if !strings.Contains(valid, "EnchantedBy") && !strings.Contains(valid, "EquippedBy") {
				continue
			}
			if unresolvedStatic(st) {
				continue
			}
			return true
		}
	}
	return false
}

// mustAttackScope is a printed MustAttack static's creature scope, "" for a
// static that is conditional or names a defender.
func mustAttackScope(st cards.Static) string {
	if !strings.Contains(st.Mode, "MustAttack") || unresolvedStatic(st) || strings.TrimSpace(st.Params["ValidPlayer"]) != "" {
		return ""
	}
	if valid := strings.TrimSpace(st.Params["ValidCreature"]); valid != "" {
		return valid
	}
	return strings.TrimSpace(st.Params["ValidCard"])
}

// attachedMustAttack reports whether the static of an Aura or Equipment makes
// the creature it is attached to attack (Infectious Bloodlust).
func attachedMustAttack(st cards.Static) bool {
	valid := mustAttackScope(st)
	return strings.Contains(valid, "EnchantedBy") || strings.Contains(valid, "EquippedBy")
}

// typeScopedRequirements marks the seat's creatures a printed MustAttack static
// on any permanent names by type and controller ("Creatures your opponents
// control attack each combat if able").
func (s *Session) typeScopedRequirements(o *protocol.Observation, seat string, required map[string]bool) {
	for _, p := range o.Players {
		for _, src := range p.Battlefield {
			f := s.requirementFace(src)
			if f == nil {
				continue
			}
			for _, st := range f.Statics {
				valid := mustAttackScope(st)
				for _, own := range o.Players {
					if own.Seat != seat {
						continue
					}
					for _, r := range own.Battlefield {
						if s.typeScoped(valid, r, src) {
							required[r.ObjectID] = true
						}
					}
				}
			}
		}
	}
}

// barredByType reports whether an unconditional printed static of the mode on
// a permanent names this creature by type (Doomed Artisan: "Sculptures you
// control can't attack or block"). A ValidCard$ scope other than who controls
// the creature is not read, so the engine's own offer stays the authority
// there.
func (s *Session) barredByType(o *protocol.Observation, r protocol.ObjectRecord, mode string) bool {
	for _, p := range o.Players {
		for _, src := range p.Battlefield {
			f := s.B.face(src.CardName, src.Token, nil)
			if f == nil {
				continue
			}
			for _, st := range f.Statics {
				if !slices.Contains(strings.Split(st.Mode, ","), mode) || unresolvedStatic(st) {
					continue
				}
				if s.typeScoped(strings.TrimSpace(st.Params["ValidCard"]), r, src) {
					return true
				}
			}
		}
	}
	return false
}

// typeScoped reports whether the static on src names the creature r by a
// ValidCard$ spec whose every alternative is a bare type word, scoped to at
// most who controls it, whether it is src itself and more type words. A scope
// the creature's type words do not hold reads as not naming it.
func (s *Session) typeScoped(valid string, r, src protocol.ObjectRecord) bool {
	if valid == "" {
		return false
	}
	for _, alt := range strings.Split(valid, ",") {
		_, scope, ok := strings.Cut(strings.TrimSpace(alt), ".")
		if !ok {
			continue
		}
		for _, q := range strings.Split(scope, "+") {
			switch q {
			case "YouCtrl", "OppCtrl":
			case "Other":
				if r.ObjectID == src.ObjectID {
					return false
				}
			default:
				if !s.recordHasWord(r, q) {
					return false
				}
			}
		}
	}
	return s.presentSpec(valid, r, src.ControllerSeat)
}

// mayAttackThroughDefender reports whether a printed-Defender creature has a
// live CanAttackDefender permission the observation carries no keyword for:
// the creature's own activated ability, activated this turn (the session's
// ActivatedThisTurn census), delivered an Effect whose StaticAbilities$ body
// is a self-scoped CanAttackDefender grant (Wall of One Thousand Cuts). The
// engine's continuous-effect registry is the authority; this reads the
// public half of it -- the printed ability and the activation census --
// exactly as cantAttack reads a printed static. A grant that is not
// self-scoped, is gated by a term the observation cannot evaluate, or whose
// last recorded activation is another ability is not admitted (the engine's
// own offer still is).
func (s *Session) mayAttackThroughDefender(v2ID string, cv view.CardView) bool {
	if cv.ActivatedThisTurn < 1 {
		return false
	}
	f := s.staticFace(cv)
	if f == nil {
		return false
	}
	i, ok := s.activated[v2ID]
	if !ok || i < 0 || i >= len(f.Abilities) {
		return false
	}
	ab := f.Abilities[i]
	// The permission may be the activated ability's own Effect body
	// (Wall of One Thousand Cuts) or a later link of its SubAbility chain
	// (Walking Wall's Pump into DB$ Effect | StaticAbilities$ CanAttack).
	for _, link := range chain(f, ab) {
		if link.API != "Effect" {
			continue
		}
		names := strings.FieldsFunc(effects.EffectOf(link).StaticAbilities, func(r rune) bool {
			return r == ',' || r == ' ' || r == '\t' || r == '\n'
		})
		for _, name := range names {
			body := strings.TrimSpace(f.SVars[name])
			if body == "" {
				continue
			}
			statics, ok := cards.ParseStaticLines(body)
			if !ok {
				continue
			}
			for _, st := range statics {
				if st.Mode != "CanAttackDefender" || !effects.CanAttackDefenderGrantParamsReadable(st.Params) {
					continue
				}
				if strings.TrimSpace(st.Params["ValidAttacked"]) != "" {
					continue // a defender-side gate the neutral cannot evaluate
				}
				spec := strings.TrimSpace(st.Params["ValidCard"])
				if spec == "" {
					spec = strings.TrimSpace(st.Params["ValidCards"])
				}
				switch strings.ToLower(spec) {
				case "card.effectsource", "card.self", "cardname":
					return true
				}
			}
		}
	}
	return false
}

// cantBlockAttacker reports whether a printed CantBlockBy static forbids this
// (blocker, attacker) pair. Forge spells the static on either side of the
// pair: a blocker's own "can't block <filter>" (Hinterland Drake:
// ValidAttacker$ Creature.Artifact | ValidBlocker$ Creature.Self) and an
// attacker's "can't be blocked by <filter>" (Azure Beastbinder:
// ValidAttacker$ Creature.Self | ValidBlocker$ Creature.powerGE2). Native's
// BlockRestricted reads ValidAttacker$ against the attacker and ValidBlocker$
// against the blocker with the static's host as the Self referent, so both
// host shapes are read here.
func (s *Session) cantBlockAttacker(b, a view.CardView) bool {
	return s.cantBlockByStatic(b, a, b) || s.cantBlockByStatic(b, a, a)
}

func (s *Session) cantBlockByStatic(b, a, host view.CardView) bool {
	f := s.staticFace(host)
	if f == nil {
		return false
	}
	for _, st := range f.Statics {
		if !slices.Contains(strings.Split(st.Mode, ","), "CantBlockBy") {
			continue
		}
		if unresolvedStatic(st) {
			continue // a conditional gate cannot be proven to apply; the engine's offer stays authoritative
		}
		attackerSpec := strings.TrimSpace(st.Params["ValidAttacker"])
		if attackerSpec == "" {
			attackerSpec = strings.TrimSpace(st.Params["ValidCard"])
		}
		// An empty attacker spec matches nothing in native's matcher, so the
		// static cannot be proven to apply.
		if attackerSpec == "" || !matchesSpec(attackerSpec, a, host) {
			continue
		}
		blockerSpec, ok := st.Params["ValidBlocker"]
		if !ok {
			return true // the attacker spec alone names the restriction
		}
		if matchesSpec(strings.TrimSpace(blockerSpec), b, host) {
			return true
		}
	}
	return false
}

// matchesSpec evaluates the dot-separated qualifiers this build can read from
// a Forge ValidAttacker$/ValidBlocker$ spec against one creature. Self is the
// static's host. An unknown qualifier matches nothing: a restriction that
// cannot be proven must not apply, the same fail-closed direction as
// unresolvedStatic.
func matchesSpec(spec string, cv, host view.CardView) bool {
	if spec == "" {
		return false
	}
	for _, part := range strings.Split(strings.ToLower(spec), ".") {
		part = strings.TrimSpace(part)
		switch {
		case part == "" || part == "card":
		case part == "creature":
			if !isCreature(cv) {
				return false
			}
		case part == "self" || part == "cardname":
			if cv.ID != host.ID {
				return false
			}
		case part == "artifact":
			if !hasType(cv, "Artifact") {
				return false
			}
		case strings.HasPrefix(part, "power"):
			if !matchesPower(part[len("power"):], cv.Power) {
				return false
			}
		default:
			return false
		}
	}
	return true
}

// matchesPower reads Forge's comparator suffix ("GE2", "LT3", ...) against a
// creature's power.
func matchesPower(cmp string, power int32) bool {
	switch {
	case strings.HasPrefix(cmp, "ge"):
		n, ok := atoi32(cmp[2:])
		return ok && power >= n
	case strings.HasPrefix(cmp, "gt"):
		n, ok := atoi32(cmp[2:])
		return ok && power > n
	case strings.HasPrefix(cmp, "le"):
		n, ok := atoi32(cmp[2:])
		return ok && power <= n
	case strings.HasPrefix(cmp, "lt"):
		n, ok := atoi32(cmp[2:])
		return ok && power < n
	case strings.HasPrefix(cmp, "eq"):
		n, ok := atoi32(cmp[2:])
		return ok && power == n
	}
	return false
}

func atoi32(s string) (int32, bool) {
	n, err := strconv.ParseInt(strings.TrimSpace(s), 10, 32)
	return int32(n), err == nil
}

// --- small choices ---

// yesNo is gorge's two-option ask for a yes/no choice: "yes" then "no", in
// the decision kind gorge poses for that purpose.
func (s *Session) yesNo(sd *protocol.SeatDecision) ([]mapping.NativeOp, *native, error) {
	first := sd.Candidates[0].Semantic
	d := decision.Decision{Kind: decision.KChoose, Min: 1, Max: 1, Prompt: "Choose",
		Options: []decision.Option{{Index: 0, Kind: "yes", Label: "Yes"}, {Index: 1, Kind: "no", Label: "No"}}}
	field := "value"
	switch {
	case first.Kind == "optional_cast":
		field, d.Kind, d.ResumeKind = "cast_it", decision.KTriggerOptional, str(first, "method")
	case first.Kind == "optional_cost":
		field = "pay"
		d.Options[0].Kind, d.Options[1].Kind = "trigger_cost_pay", "trigger_cost_decline"
	case str(first, "purpose") == "optional_trigger" || str(first, "purpose") == "may_ability":
		d.Kind = decision.KTriggerOptional
		if src, _ := optRef(first, "source"); src != nil && str(first, "purpose") == "optional_trigger" {
			d.EffectOptional = s.effectOptional(*src)
		}
		if src, _ := optRef(first, "source"); src != nil && str(first, "purpose") == "may_ability" {
			if sa := s.repeatOptionalSA(*src, &sd.Observation); sa != nil {
				// RepeatOptional$'s election is a plain choose with its own
				// wording, not a may-ability.
				d.Kind, d.Prompt, d.Source = decision.KChoose, "Repeat this process?", s.B.IDs.Of(src.ObjectID)
				d.ResumeKind, d.ResumeSA = "repeat_optional", sa
				d.Options[0].Label, d.Options[0].Player = "Repeat", s.seat
				d.Options[1].Label, d.Options[1].Player = "Stop", s.seat
			}
		}
	case str(first, "purpose") == "optional_replacement":
		d.Kind = decision.KReplacement
		d.Options[0].Kind, d.Options[1].Kind = "madness_exile", "madness_graveyard"
	}
	n := s.open("yesno", d)
	ops := make([]mapping.NativeOp, len(sd.Candidates))
	for i, c := range sd.Candidates {
		ops[i] = noOp
		if c.Semantic.Kind != first.Kind {
			continue // a mana activation offered beside a resolution payment
		}
		k := 1
		if boolean(c.Semantic, field) {
			k = 0
		}
		ops[i] = mapping.NativeOp{Op: "choose", Option: k}
	}
	return ops, n, nil
}

// effectOptional reports whether a source's optional trigger is one an Effect
// it created carries (an emblem's "you may"): the source prints no optional
// trigger, and an SVar body of its is one. gorge marks that election on the
// decision, and the bot declines it.
func (s *Session) effectOptional(src protocol.ObjectRef) bool {
	f := s.abilityFace(src)
	if f == nil {
		return false
	}
	for _, tr := range f.Triggers {
		if _, ok := tr.Params["OptionalDecider"]; ok {
			return false
		}
	}
	for _, name := range slices.Sorted(maps.Keys(f.SVars)) {
		if tr, ok := cards.ParseTriggerLine(f.SVars[name]); ok {
			if _, optional := tr.Params["OptionalDecider"]; optional {
				return true
			}
		}
	}
	return false
}

// repeatOptionalSA is the RepeatOptional$ Repeat of the ability the source
// names, nil when it has none.
func (s *Session) repeatOptionalSA(src protocol.ObjectRef, o *protocol.Observation) *cards.SA {
	f, roots := s.abilityRoots(src, o)
	for _, root := range roots {
		if root == nil {
			continue
		}
		for _, sa := range chain(f, root) {
			if sa.API == "Repeat" && strings.EqualFold(sa.Params["RepeatOptional"], "True") {
				return sa
			}
		}
	}
	return nil
}

// optionalCost is gorge's unless-cost ask (pay or decline) for
// unless_payment, and its pay-or-decline ask for other optional costs.
func (s *Session) optionalCost(sd *protocol.SeatDecision) ([]mapping.NativeOp, *native, error) {
	first := sd.Candidates[0].Semantic
	for _, c := range sd.Candidates {
		if c.Semantic.Kind == "optional_cost" {
			first = c.Semantic
			break
		}
	}
	src, err := ref(first, "source")
	if err != nil {
		return nil, nil, err
	}
	// The native decision publishes the unless-cost ability as ResumeSA.
	// The bot's unless-mana arm needs it to tell a payable tax from a
	// no-cost ask; the v2 semantic names only the source, so resolve the
	// ability that is on the stack or resolving and read its own printed
	// UnlessCost body. A "other" optional cost whose source carries an
	// UnlessCost elsewhere on its face is not this shape.
	resume := s.unlessCostSA(src, &sd.Observation)
	if str(first, "cost") != "unless_payment" && resume == nil {
		return s.yesNo(sd)
	}
	obj := s.unlessCostObject(src, &sd.Observation)
	n := s.open("unless", decision.Decision{Kind: decision.KModes, Min: 1, Max: 1, ResumeKind: "unless_pay",
		Prompt: "Pay the cost?", ResumeSA: resume,
		Options: []decision.Option{
			{Index: 0, Kind: "mode", Label: "Pay", Mode: decision.ModeUnlessPay,
				Obj: obj, Player: s.seat},
			{Index: 1, Kind: "mode", Label: "Decline", Mode: decision.ModeUnlessDecline,
				Obj: obj, Player: s.seat}}})
	ops := make([]mapping.NativeOp, len(sd.Candidates))
	for i, c := range sd.Candidates {
		ops[i] = noOp
		if c.Semantic.Kind != "optional_cost" {
			continue
		}
		k := 1
		if boolean(c.Semantic, "pay") {
			k = 0
		}
		ops[i] = mapping.NativeOp{Op: "choose", Option: k}
	}
	return ops, n, nil
}

// unlessCostObject is the object the native unless-cost decision names:
// the ability's Source permanent for a triggered or activated ability
// (rules' Ctx.Source is o.Source, Ruling T20-b), and the source itself
// for a spell (a counterspell tax names the spell on the stack). The v2
// semantic names the ability object on the stack, so the stack entry's
// Source is the one place the permanent is visible.
func (s *Session) unlessCostObject(src protocol.ObjectRef, o *protocol.Observation) state.ObjID {
	for i := range o.Stack {
		e := &o.Stack[i]
		if e.ObjectID != src.ObjectID || e.Source == nil {
			continue
		}
		if e.StackKind == "triggered_ability" || e.StackKind == "activated_ability" {
			return s.B.IDs.Of(e.Source.ObjectID)
		}
	}
	return s.B.IDs.Of(src.ObjectID)
}

// abilityRoots is the face and the root abilities of the source an ask names:
// the ability on the stack or resolving, the native decision's ResumeSA. The
// v2 source names the ability object, so the observation's stack entry (or
// the activation the session just recorded) picks the one ability; scanning
// the whole face would let an unrelated ability masquerade as this ask's.
func (s *Session) abilityRoots(src protocol.ObjectRef, o *protocol.Observation) (*cards.Face, []*cards.SA) {
	var entry *protocol.StackEntry
	for i := range o.Stack {
		if o.Stack[i].ObjectID == src.ObjectID {
			entry = &o.Stack[i]
		}
	}
	var f *cards.Face
	var roots []*cards.SA
	switch {
	case entry != nil && entry.StackKind == "spell":
		if f = s.B.face(entry.CardName, false, nil); f != nil {
			roots = addRoots(roots, f, entry, f.SpellAbility())
		}
	case entry != nil && entry.StackKind == "activated_ability" && entry.Source != nil:
		// The ask names the ability object on the stack; the activated
		// permanent is the entry's Source and Picked recorded which of its
		// printed abilities was chosen.
		if f = s.B.face(entry.Source.CardName, false, nil); f != nil {
			if i, ok := s.activated[entry.Source.ObjectID]; ok && i >= 0 && i < len(f.Abilities) {
				roots = append(roots, f.Abilities[i])
			} else {
				roots = append(roots, f.Abilities...)
			}
		}
	case entry != nil && entry.StackKind == "triggered_ability":
		// The observation does not say which printed trigger fired, so every
		// trigger's effect is a candidate; an unrelated activated ability is
		// not.
		if f = s.B.face(src.CardName, false, nil); f != nil {
			for _, tr := range f.Triggers {
				if tr.Effect != nil {
					roots = append(roots, tr.Effect)
				}
			}
		}
	case src.Zone == "battlefield":
		// An ability being activated is announced from its permanent.
		if f = s.B.face(src.CardName, false, nil); f != nil {
			if i, ok := s.activated[src.ObjectID]; ok && i >= 0 && i < len(f.Abilities) {
				roots = append(roots, f.Abilities[i])
			}
		}
	default:
		// The stack entry is gone (a resolving trigger) or the source is a
		// card the observation does not place: no ability can be singled
		// out, so keep the historic face-wide search.
		if f = s.B.face(src.CardName, false, nil); f != nil {
			for _, tr := range f.Triggers {
				if tr.Effect != nil {
					roots = addRoots(roots, f, entry, tr.Effect)
				}
			}
			roots = append(roots, f.Abilities...)
			if root := f.SpellAbility(); root != nil {
				roots = append(roots, root)
			}
		}
	}
	return f, roots
}

// unlessCostSA finds the printed unless-cost body of the ability that is on
// the stack or resolving, the native decision's ResumeSA (abilityRoots picks
// the one ability, so an unrelated ability's UnlessCost cannot masquerade as
// this ask's).
func (s *Session) unlessCostSA(src protocol.ObjectRef, o *protocol.Observation) *cards.SA {
	f, roots := s.abilityRoots(src, o)
	for _, root := range roots {
		if root == nil {
			continue
		}
		for _, sa := range chain(f, root) {
			if sa.Params["UnlessCost"] != "" {
				return sa
			}
		}
	}
	return nil
}

func (s *Session) mulligan(sd *protocol.SeatDecision) ([]mapping.NativeOp, *native, error) {
	first := sd.Candidates[0].Semantic
	taken := num(first, "mulligans_taken")
	// CR 103.5b's multiplayer free mulligan exempts a seat from the bottoming
	// PENALTY, not from the allowance. The round's fixed free count is derived
	// from the table size, so the observation's player list is enough.
	free := 0
	if len(sd.Observation.Players) >= 3 {
		free = 1
	}
	bottom := max(0, taken-free)
	// gorge's keep/mulligan prompt carries the bottoming penalty only in its
	// text (engine/rules/mulligan.go's keepMulliganPrompt); botpolicy's
	// keepBottomCount is the rule that reads it. Reproduce the same clause so
	// the neutral world answers the same keep/mulligan question.
	penalty := "put " + putCount(bottom) + " on the bottom of your library"
	if bottom == 0 && free > 0 {
		penalty = "keep all seven cards"
	}
	opts := make([]decision.Option, 0, len(sd.Candidates))
	at := map[bool]int{}
	for _, c := range sd.Candidates {
		keep := boolean(c.Semantic, "keep")
		kind, label := "mulligan", "mulligan"
		if keep {
			kind, label = "keep", "keep"
		}
		if _, ok := at[keep]; !ok {
			at[keep] = len(opts)
			opts = append(opts, decision.Option{Index: len(opts), Kind: kind, Label: label, Player: s.seat})
		}
	}
	_, canMulligan := at[false]
	prompt := "Keep your hand (" + penalty + ")"
	if canMulligan {
		prompt += " or take a mulligan?"
	}
	n := s.open("mulligan", decision.Decision{Kind: decision.KMulligan, Min: 1, Max: 1, Prompt: prompt, Options: opts})
	ops := make([]mapping.NativeOp, len(sd.Candidates))
	for i, c := range sd.Candidates {
		ops[i] = mapping.NativeOp{Op: "choose", Option: at[boolean(c.Semantic, "keep")]}
	}
	return ops, n, nil
}

// putCount is engine/rules/mulligan.go's human-facing card count.
func putCount(n int) string {
	if n == 1 {
		return "1 card"
	}
	return strconv.Itoa(n) + " cards"
}

func (s *Session) modes(sd *protocol.SeatDecision) ([]mapping.NativeOp, *native, error) {
	first := sd.Candidates[0].Semantic
	src, err := ref(first, "source")
	if err != nil {
		return nil, nil, err
	}
	key := "modes|" + src.ObjectID
	n := s.continuing(key)
	if n == nil || sd.Group.SubstepIndex == 0 {
		// gorge offers only the eligible modes, in printed order.
		n = s.open(key, decision.Decision{Kind: decision.KModes, Min: num(first, "minimum"), Max: num(first, "maximum"),
			Source: s.B.IDs.Of(src.ObjectID), Prompt: "Choose a mode"})
		var modes []int
		for _, c := range sd.Candidates {
			modes = append(modes, num(c.Semantic, "mode_index"))
		}
		slices.Sort(modes)
		for _, k := range slices.Compact(modes) {
			n.options[strconv.Itoa(k)] = len(n.d.Options)
			n.d.Options = append(n.d.Options, decision.Option{Index: len(n.d.Options), Kind: "mode", Label: fmt.Sprintf("Mode %d", k+1)})
		}
	}
	ops := make([]mapping.NativeOp, len(sd.Candidates))
	for i, c := range sd.Candidates {
		ops[i] = noOp
		if k, ok := n.options[strconv.Itoa(num(c.Semantic, "mode_index"))]; ok {
			ops[i] = mapping.NativeOp{Op: "choose", Option: k}
		}
	}
	return ops, n, nil
}

// costOption is a choice between a spell's optional costs (Highway
// Robbery: discard a card or sacrifice a land). gorge poses the same choice
// as the modes of the spell's GenericChoice, in printed order; each
// candidate is matched to the printed choice whose cost names its words.
func (s *Session) costOption(sd *protocol.SeatDecision) ([]mapping.NativeOp, *native, error) {
	src, err := ref(sd.Candidates[0].Semantic, "source")
	if err != nil {
		return nil, nil, err
	}
	var printed []string
	if f := s.B.face(src.CardName, false, nil); f != nil {
		if root := f.SpellAbility(); root != nil && root.API == "GenericChoice" {
			for _, c := range strings.Split(root.Params["Choices"], ",") {
				cost := ""
				if sa := cards.ResolveSVar(f.SVars, strings.TrimSpace(c)); sa != nil {
					cost = strings.ToLower(sa.Params["UnlessCost"] + " " + sa.Params["Cost"])
				}
				printed = append(printed, cost)
			}
		}
	}
	rank := func(choice string) int {
		for i, cost := range printed {
			for _, w := range strings.Split(choice, "_") {
				w = strings.TrimSuffix(strings.Replace(w, "sacrifice", "sac", 1), "s")
				if len(w) > 2 && strings.Contains(cost, w) {
					return i
				}
			}
		}
		return len(printed)
	}
	choices := make([]string, len(sd.Candidates))
	for i, c := range sd.Candidates {
		choices[i] = str(c.Semantic, "choice")
	}
	order := slices.Clone(choices)
	slices.SortStableFunc(order, func(a, b string) int { return rank(a) - rank(b) })
	n := s.open("costoption|"+src.ObjectID, decision.Decision{Kind: decision.KModes, Min: 1, Max: 1,
		Source: s.B.IDs.Of(src.ObjectID), Prompt: "Choose a cost"})
	for _, c := range order {
		n.d.Options = append(n.d.Options, decision.Option{Index: len(n.d.Options), Kind: "mode", Label: c})
	}
	ops := make([]mapping.NativeOp, len(sd.Candidates))
	for i, c := range choices {
		ops[i] = mapping.NativeOp{Op: "choose", Option: slices.Index(order, c)}
	}
	return ops, n, nil
}

// option is an effect's choice among numbered options (Winding Way's
// creature or land). gorge offers them in the same printed order and takes
// the first, whether it poses them as a type choice or as modes.
func (s *Session) option(sd *protocol.SeatDecision) ([]mapping.NativeOp, *native, error) {
	first := sd.Candidates[0].Semantic
	src, err := optRef(first, "source")
	if err != nil {
		return nil, nil, err
	}
	d := decision.Decision{Kind: decision.KModes, Min: 1, Max: 1, Prompt: "Choose an option"}
	kind := "mode"
	var labels []string
	if src != nil {
		d.Source = s.B.IDs.Of(src.ObjectID)
		if sa := s.chainSA(*src, func(sa *cards.SA) bool { return sa.API == "ChooseType" }); sa != nil {
			d.Kind, kind = decision.KChoose, "type"
			labels = strings.Split(sa.Params["ValidTypes"], ",")
		}
	}
	var idx []int
	for _, c := range sd.Candidates {
		idx = append(idx, num(c.Semantic, "option_index"))
	}
	slices.Sort(idx)
	idx = slices.Compact(idx)
	n := s.open("option", d)
	for _, k := range idx {
		label := fmt.Sprintf("Option %d", k+1)
		if k < len(labels) {
			label = strings.TrimSpace(labels[k])
		}
		n.d.Options = append(n.d.Options, decision.Option{Index: len(n.d.Options), Kind: kind, Label: label})
	}
	ops := make([]mapping.NativeOp, len(sd.Candidates))
	for i, c := range sd.Candidates {
		ops[i] = mapping.NativeOp{Op: "choose", Option: slices.Index(idx, num(c.Semantic, "option_index"))}
	}
	return ops, n, nil
}

func (s *Session) numbers(sd *protocol.SeatDecision) ([]mapping.NativeOp, *native, error) {
	first := sd.Candidates[0].Semantic
	kind := "number"
	if str(first, "purpose") == "x_value" {
		kind = "x"
	}
	n := s.open("number", decision.Decision{Kind: decision.KChoose, Min: 1, Max: 1, Prompt: "Choose a number"})
	ops := make([]mapping.NativeOp, len(sd.Candidates))
	for i, c := range sd.Candidates {
		val := num(c.Semantic, "value")
		n.d.Options = append(n.d.Options, decision.Option{Index: i, Kind: kind, Label: strconv.Itoa(val), Value: val})
		ops[i] = mapping.NativeOp{Op: "choose", Option: i}
	}
	return ops, n, nil
}

var colorSymbols = map[string]string{"white": "W", "blue": "U", "black": "B", "red": "R", "green": "G"}

func (s *Session) colors(sd *protocol.SeatDecision) ([]mapping.NativeOp, *native, error) {
	n := s.open("color", decision.Decision{Kind: decision.KChoose, Min: 1, Max: 1, Prompt: "Choose a color"})
	ops := make([]mapping.NativeOp, len(sd.Candidates))
	for i, c := range sd.Candidates {
		color := str(c.Semantic, "color")
		n.d.Options = append(n.d.Options, decision.Option{Index: i, Kind: "color", Label: title(color),
			ManaSymbol: colorSymbols[color]})
		ops[i] = mapping.NativeOp{Op: "choose", Option: i}
	}
	return ops, n, nil
}

// --- target effects ---

// chosenModeRoots expands a modal ability's chosen modes into their SVar
// bodies. The stack entry publishes the chosen indices among the root's
// Choices$ (modesAndX), so a Charm's mode body is recoverable even though it
// is not on the SubAbility chain: the mode's targeting SA (and its
// per-controller/target-budget parameters) lives there.
func chosenModeRoots(f *cards.Face, entry *protocol.StackEntry, root *cards.SA) []*cards.SA {
	if f == nil || entry == nil || root == nil || len(entry.Modes) == 0 || root.Params["Choices"] == "" {
		return nil
	}
	choices := strings.Split(root.Params["Choices"], ",")
	var out []*cards.SA
	for _, m := range entry.Modes {
		if int(m) >= len(choices) {
			continue
		}
		if sa := cards.ResolveSVar(f.SVars, strings.TrimSpace(choices[m])); sa != nil {
			out = append(out, sa)
		}
	}
	return out
}

// addRoots appends root and the chosen-mode bodies of a modal root.
func addRoots(roots []*cards.SA, f *cards.Face, entry *protocol.StackEntry, root *cards.SA) []*cards.SA {
	if root == nil {
		return roots
	}
	roots = append(roots, root)
	return append(roots, chosenModeRoots(f, entry, root)...)
}

// targetAsk locates the slot-th targeting ability of a target ask and the
// facts describing it: the stack entry that names the ask (nil for an
// ability announced from a battlefield permanent), the source face, the SA
// carrying ValidTgts, and the entry's published X value. targetEffect and
// the target ask's per-controller grouping both read it, so the slot walk
// has one home.
func (s *Session) targetAsk(o *protocol.Observation, src protocol.ObjectRef, slot int) (entry *protocol.StackEntry, f *cards.Face, sa *cards.SA, x *uint32) {
	for i := range o.Stack {
		if o.Stack[i].ObjectID == src.ObjectID {
			entry = &o.Stack[i]
		}
	}
	var roots []*cards.SA
	switch {
	case entry != nil && entry.StackKind == "spell":
		if f = s.B.face(entry.CardName, false, nil); f != nil {
			roots = addRoots(roots, f, entry, f.SpellAbility())
		}
	case entry != nil && entry.StackKind == "activated_ability" && entry.Source != nil:
		// The ask names the ability object on the stack; the activated
		// permanent is the entry's Source and Picked recorded which of its
		// printed abilities was chosen.
		if f = s.B.face(entry.Source.CardName, false, nil); f != nil {
			if i, ok := s.activated[entry.Source.ObjectID]; ok && i >= 0 && i < len(f.Abilities) {
				roots = addRoots(roots, f, entry, f.Abilities[i])
			}
		}
	case entry != nil && entry.StackKind == "triggered_ability":
		// A triggered ability's source is its permanent, and the ask names
		// the trigger's own ability object. The observation does not say
		// which printed trigger fired, so every root's chain is a candidate;
		// the slot picks the targeting ability within it.
		if f = s.B.face(src.CardName, false, nil); f != nil {
			// A trigger's Execute$ body is the effect that poses the ask;
			// the printed AB/spell roots are the fallback for a trigger the
			// compiler folded elsewhere.
			for _, tr := range f.Triggers {
				if tr.Effect != nil {
					roots = addRoots(roots, f, entry, tr.Effect)
				}
			}
			roots = append(roots, f.Abilities...)
			// An Effect-created trigger (The Sea Devils' "deals that much
			// damage") is an SVar body with Mode$ + Execute$, never a printed
			// T: trigger. Its Execute$ body is the root the ask lives in.
			for _, name := range slices.Sorted(maps.Keys(f.SVars)) {
				tr, ok := cards.ParseTriggerLine(f.SVars[name])
				if !ok {
					continue
				}
				exec := strings.TrimSpace(tr.Params["Execute"])
				if exec == "" {
					continue
				}
				if root := cards.ResolveSVar(f.SVars, exec); root != nil {
					roots = append(roots, root)
				}
			}
			if root := f.SpellAbility(); root != nil {
				roots = append(roots, root)
			}
		}
	case src.Zone == "battlefield":
		// An ability being activated is announced from its permanent.
		i, ok := s.activated[src.ObjectID]
		if f = s.B.face(src.CardName, false, nil); f != nil && ok && i < len(f.Abilities) {
			roots = append(roots, f.Abilities[i])
		}
	}
	for _, root := range roots {
		k := 0
		for _, c := range chain(f, root) {
			if _, ok := c.Params["ValidTgts"]; ok {
				if k == slot {
					sa = c
					break
				}
				k++
			}
		}
		if sa != nil {
			break
		}
	}
	if entry != nil {
		x = entry.XValue
	}
	return entry, f, sa, x
}

// targetEffect describes what a target ask's ability does, as gorge's
// engine does for its own target asks: the API of the slot-th targeting
// ability in the source's chain, its removal shape, and a literal or X
// damage amount.
func (s *Session) targetEffect(o *protocol.Observation, src protocol.ObjectRef, slot int) *decision.TargetEffect {
	entry, f, sa, x := s.targetAsk(o, src, slot)
	if sa == nil {
		return nil
	}
	// The counter a Count$CardCounters.<KIND> body reads lives on the
	// battlefield permanent. For an ability on the stack that permanent is
	// the entry's Source; a spell or an ability announced from the
	// permanent keeps src.
	anchor := src
	if entry != nil && entry.Source != nil {
		anchor = *entry.Source
	}
	out := &decision.TargetEffect{API: sa.API, Removal: removal(sa)}
	if sa.API == "Effect" {
		out.Statics = staticModesFromFace(f, sa)
	}
	if sa.API == "DealDamage" || sa.API == "DamageAll" {
		out.Damage = &decision.DamageEffect{}
		raw := strings.TrimSpace(sa.Params["NumDmg"])
		switch {
		case raw == "":
		case raw == "X" && x != nil && xValueIsTriggerAmount(entry, f, raw):
			n := int(*x)
			out.Damage.Amount = &n
		default:
			if n, err := strconv.Atoi(raw); err == nil && n >= 0 {
				out.Damage.Amount = &n
				break
			}
			// A dynamic amount (NumDmg$ X or another SVar name) is not on
			// the v2 wire; the observation's zones can still resolve the
			// common Count$ form, and an unresolvable body stays unknown.
			if f != nil {
				if v, ok := s.svarFromFace(o, &anchor, f, raw, 0); ok && v >= 0 {
					n := int(v)
					out.Damage.Amount = &n
				}
			}
		}
	}
	return out
}

// xValueIsTriggerAmount reports whether the stack entry's published x_value
// is the amount NumDmg$ X names. A spell's x_value is its mana cost's X. An
// ability's x_value is the causing event's magnitude, which only a
// TriggerCount$ body reads (The Sea Devils); a Count$ body (Niv-Mizzet,
// Guildpact's colour-pair count) must go to svarValue instead of borrowing
// the event's amount.
func xValueIsTriggerAmount(entry *protocol.StackEntry, f *cards.Face, raw string) bool {
	if entry == nil || entry.XValue == nil {
		return false
	}
	if entry.StackKind == "spell" {
		return true
	}
	return f != nil && strings.HasPrefix(strings.TrimSpace(f.SVars[raw]), "TriggerCount$")
}

// removal is gorge's targetRemoval (rules/stack.go), which is unexported.
func removal(sa *cards.SA) *decision.RemovalEffect {
	switch sa.API {
	case "Destroy", "DestroyAll":
		return &decision.RemovalEffect{Kind: "destroy"}
	case "Sacrifice", "SacrificeAll":
		return &decision.RemovalEffect{Kind: "sacrifice"}
	case "ChangeZone", "ChangeZoneAll":
		dest := strings.ToLower(strings.TrimSpace(sa.Params["Destination"]))
		kind := map[string]string{"exile": "exile", "hand": "bounce", "graveyard": "graveyard", "library": "library", "command": "command"}[dest]
		if kind == "" {
			return nil
		}
		return &decision.RemovalEffect{Kind: kind, Destination: dest}
	}
	return nil
}

// staticModesFromFace resolves an Effect SA's StaticAbilities$ names against
// the printed face's SVar table, returning each body's Mode$ in written
// order. It is the neutral mirror of rules.staticModesFromSVars: the v2
// observation has no live static registry, so the printed ability facts are
// the only source, and the native target ask publishes exactly these modes
// for the bot's grant polarity read.
func staticModesFromFace(f *cards.Face, sa *cards.SA) []string {
	if f == nil || sa == nil || sa.API != "Effect" {
		return nil
	}
	names := strings.FieldsFunc(sa.Params["StaticAbilities"], func(r rune) bool {
		return r == ',' || r == ' ' || r == '\t' || r == '\n'
	})
	var out []string
	for _, n := range names {
		body := strings.TrimSpace(f.SVars[n])
		if body == "" {
			continue
		}
		statics, ok := cards.ParseStaticLines(body)
		if !ok {
			continue
		}
		for _, st := range statics {
			if st.Mode != "" {
				out = append(out, st.Mode)
			}
		}
	}
	return out
}

// --- ordering and arrangement ---

// order is an order_pick block gorge asks as one ordered answer: trigger
// order, or the cards a mulligan puts on the bottom.
func (s *Session) order(sd *protocol.SeatDecision) ([]mapping.NativeOp, *native, error) {
	first := sd.Candidates[0].Semantic
	purpose := str(first, "purpose")
	if purpose == "arrangement" {
		return s.arrange(sd)
	}
	switch purpose {
	case "library_top", "other":
		return s.placed(sd)
	}
	var kind decision.Kind
	var optKind string
	switch purpose {
	case "triggers":
		kind, optKind = decision.KTriggerOrder, "trigger"
	case "mulligan_bottom":
		kind, optKind = decision.KMulligan, "bottom"
	default:
		return nil, nil, nil
	}
	key := "order|" + purpose
	n := s.continuing(key)
	pos := num(first, "position")
	if n == nil || pos == 0 {
		count := num(first, "count")
		n = s.open(key, decision.Decision{Kind: kind, Min: count, Max: count, Prompt: "Order"})
		for _, c := range sd.Candidates {
			it, err := field[json.RawMessage](c.Semantic, "item")
			if err != nil {
				return nil, nil, err
			}
			o := decision.Option{Index: len(n.d.Options), Kind: optKind, Label: "item"}
			var item protocol.OrderItem
			if json.Unmarshal(it, &item) == nil {
				switch {
				case item.Object != nil:
					o.Obj, o.Label = s.B.IDs.Of(item.Object.ObjectID), name(*item.Object)
				case item.Trigger != nil && item.Trigger.Source != nil:
					o.Obj, o.Label = s.B.IDs.Of(item.Trigger.Source.ObjectID), name(*item.Trigger.Source)
				}
			}
			n.options[string(it)] = o.Index
			n.d.Options = append(n.d.Options, o)
		}
		if kind == decision.KTriggerOrder {
			n.d.Min, n.d.Max = len(n.d.Options), len(n.d.Options)
		}
	}
	ops := make([]mapping.NativeOp, len(sd.Candidates))
	for i, c := range sd.Candidates {
		ops[i] = noOp
		it, err := field[json.RawMessage](c.Semantic, "item")
		if err != nil {
			return nil, nil, err
		}
		if k, ok := n.options[string(it)]; ok {
			ops[i] = mapping.NativeOp{Op: "list", Option: k, List: "choices", Position: pos}
		}
	}
	return ops, n, nil
}

// placed orders the cards a selection just chose: Brainstorm's cards going
// on top, or discarded cards going to the graveyard. gorge's answer order
// is their order. An ordering with no selection before it (a reveal sent
// to the graveyard) is one gorge's engine fixes without asking.
func (s *Session) placed(sd *protocol.SeatDecision) ([]mapping.NativeOp, *native, error) {
	ops := make([]mapping.NativeOp, len(sd.Candidates))
	n := s.cur
	if n == nil || !(strings.HasPrefix(n.key, "handmove|") || strings.HasPrefix(n.key, "select|")) {
		n = s.open("fixed", decision.Decision{})
		for i := range ops {
			ops[i] = noOp
		}
		ops[0] = mapping.NativeOp{Op: "fixed", Option: -1}
		return ops, n, nil
	}
	pos := num(sd.Candidates[0].Semantic, "position")
	for i, c := range sd.Candidates {
		ops[i] = noOp
		it, err := field[protocol.OrderItem](c.Semantic, "item")
		if err != nil || it.Object == nil {
			continue
		}
		if k, ok := n.options[it.Object.ObjectID]; ok {
			ops[i] = mapping.NativeOp{Op: "list", Option: k, List: "choices", Position: pos}
		}
	}
	return ops, n, nil
}

// lookedHow lists the acting seat's top count library cards the observation
// shows with the given how ("looked_at", "revealed"), top first, or nil when
// any is missing.
func lookedHow(sd *protocol.SeatDecision, count int, how string) []string {
	type card struct {
		id  string
		pos uint32
	}
	var cs []card
	for _, k := range sd.Observation.Known {
		if k.OwnerSeat == sd.ActingSeat && k.Zone == "library" && k.ObjectID != nil && k.PositionFromTop != nil &&
			k.How == how && int(*k.PositionFromTop) < count {
			cs = append(cs, card{*k.ObjectID, *k.PositionFromTop})
		}
	}
	slices.SortFunc(cs, func(a, b card) int { return int(a.pos) - int(b.pos) })
	if len(cs) != count {
		return nil
	}
	ids := make([]string, count)
	for i, c := range cs {
		if int(c.pos) != i {
			return nil
		}
		ids[i] = c.id
	}
	return ids
}

// otherDestination names the non-top destination an "other" arrangement
// offers, in candidate order.
func otherDestination(sd *protocol.SeatDecision) (string, bool) {
	for _, c := range sd.Candidates {
		if dst := str(c.Semantic, "destination"); dst != "" && dst != "top" {
			return dst, true
		}
	}
	return "", false
}

// explore translates the explore election: a KChoose whose options are the
// revealed card's destinations, state-changing first (graveyard option 0,
// top option 1), as gorge poses it.
func (s *Session) explore(sd *protocol.SeatDecision, first protocol.Semantic, ids []string) ([]mapping.NativeOp, *native, error) {
	d := decision.Decision{Kind: decision.KChoose, Min: 1, Max: 1, Prompt: "explore"}
	if src, err := optRef(first, "source"); err == nil && src != nil {
		d.Source = s.B.IDs.Of(src.ObjectID)
	}
	for i, dst := range []string{"graveyard", "top"} {
		d.Options = append(d.Options, decision.Option{Index: i, Kind: dst, Label: knownName(sd, ids[0]),
			Obj: s.B.IDs.Of(ids[0]), Player: s.seat})
	}
	n := s.open("explore", d)
	ops := make([]mapping.NativeOp, len(sd.Candidates))
	for i, c := range sd.Candidates {
		ops[i] = noOp
		if c.Semantic.Kind != "arrange_card" {
			continue
		}
		dst := str(c.Semantic, "destination")
		for _, o := range d.Options {
			if o.Kind == dst {
				ops[i] = mapping.NativeOp{Op: "choose", Option: o.Index}
				break
			}
		}
	}
	return ops, n, nil
}

// arrangeKinds maps an arrangement purpose to gorge's KArrange pile-B kind.
var arrangeKinds = map[string]string{"scry": "bottom", "surveil": "graveyard", "look_at_top": "bottom"}

// arrange is a scry, surveil or look-at-top arrangement: gorge's KArrange,
// whose answer is the cards kept on top in order (pile A) and, for scry,
// the order of the rest. The looked-at cards are in the observation's
// known list with their positions from the top.
func (s *Session) arrange(sd *protocol.SeatDecision) ([]mapping.NativeOp, *native, error) {
	first := sd.Candidates[0].Semantic
	if (first.Kind == "arrange_card" && str(first, "purpose") == "dig") || (s.cur != nil && s.cur.key == "dig") {
		return s.dig(sd)
	}
	if first.Kind == "arrange_card" && num(first, "card_index") == 0 && sd.Group.SubstepIndex == 0 {
		purpose := str(first, "purpose")
		other, ok := arrangeKinds[purpose]
		if !ok && purpose == "other" {
			// Explore ("put it back on top or into your graveyard") is the
			// one KChoose among the arrangements; its revealed card is not
			// a looked-at arrange. The other "other" shapes (a look whose
			// pile B is exile or hand) are KArrange with a destination the
			// candidates name.
			if ids := lookedHow(sd, num(first, "card_count"), "revealed"); ids != nil {
				return s.explore(sd, first, ids)
			}
			other, ok = otherDestination(sd)
		}
		if !ok {
			return nil, nil, nil
		}
		count := num(first, "card_count")
		type looked struct {
			id  string
			pos uint32
		}
		var cards []looked
		for _, k := range sd.Observation.Known {
			if k.OwnerSeat == sd.ActingSeat && k.Zone == "library" && k.ObjectID != nil && k.PositionFromTop != nil && k.How == "looked_at" {
				cards = append(cards, looked{*k.ObjectID, *k.PositionFromTop})
			}
		}
		slices.SortFunc(cards, func(a, b looked) int { return int(a.pos) - int(b.pos) })
		if len(cards) != count {
			return nil, nil, nil
		}
		d := decision.Decision{Kind: decision.KArrange, Min: 0, Max: count, Restable: other == "bottom", Prompt: purpose}
		if purpose == "look_at_top" {
			d.Min = count
		}
		if src, err := optRef(first, "source"); err == nil && src != nil {
			d.Source = s.B.IDs.Of(src.ObjectID)
		}
		n := s.open("arrange", d)
		n.arrange = &arrangement{other: other}
		for _, c := range cards {
			n.arrange.cards = append(n.arrange.cards, c.id)
		}
		for _, ci := range hiddenOrder(sd, n.arrange.cards) {
			id := n.arrange.cards[ci]
			o := decision.Option{Index: len(n.d.Options), Kind: other, Label: knownName(sd, id), Obj: s.B.IDs.Of(id)}
			n.options[id] = o.Index
			n.d.Options = append(n.d.Options, o)
		}
		s.cur = n
	}
	n := s.continuing("arrange")
	if n == nil || n.arrange == nil {
		return nil, nil, nil
	}
	a := n.arrange
	ops := make([]mapping.NativeOp, len(sd.Candidates))
	for i, c := range sd.Candidates {
		ops[i] = noOp
		switch c.Semantic.Kind {
		case "arrange_card":
			card, err := ref(c.Semantic, "card")
			if err != nil {
				return nil, nil, err
			}
			k, ok := n.options[card.ObjectID]
			dst := str(c.Semantic, "destination")
			if ok && (dst == "top" || dst == a.other) {
				ops[i] = mapping.NativeOp{Op: "dest", Option: k, List: dst}
			}
		case "order_pick":
			it, err := field[protocol.OrderItem](c.Semantic, "item")
			if err != nil || it.Object == nil {
				continue
			}
			k, ok := n.options[it.Object.ObjectID]
			ci := slices.Index(a.cards, it.Object.ObjectID)
			if !ok || ci < 0 || ci >= len(a.dest) {
				continue
			}
			dst := a.dest[ci]
			pos := 0
			for _, p := range a.placed {
				if j := slices.Index(a.cards, p); j >= 0 && j < len(a.dest) && a.dest[j] == dst {
					pos++
				}
			}
			list := "choices"
			if dst != "top" {
				list = "rest"
			}
			ops[i] = mapping.NativeOp{Op: "list", Option: k, List: list, Position: pos}
		}
	}
	return ops, n, nil
}

// looked lists the acting seat's top count library cards it now knows
// (looked at or revealed), top first, or nil when any is missing.
func looked(sd *protocol.SeatDecision, count int) []string {
	type card struct {
		id  string
		pos uint32
	}
	var cs []card
	for _, k := range sd.Observation.Known {
		if k.OwnerSeat == sd.ActingSeat && k.Zone == "library" && k.ObjectID != nil && k.PositionFromTop != nil &&
			(k.How == "looked_at" || k.How == "revealed") && int(*k.PositionFromTop) < count {
			cs = append(cs, card{*k.ObjectID, *k.PositionFromTop})
		}
	}
	slices.SortFunc(cs, func(a, b card) int { return int(a.pos) - int(b.pos) })
	if len(cs) != count {
		return nil
	}
	ids := make([]string, count)
	for i, c := range cs {
		if int(c.pos) != i {
			return nil
		}
		ids[i] = c.id
	}
	return ids
}

// knownName is the card name the observation's known list gives id.
func knownName(sd *protocol.SeatDecision, id string) string {
	for _, k := range sd.Observation.Known {
		if k.ObjectID != nil && *k.ObjectID == id {
			return k.CardName
		}
	}
	return "a card"
}

// hiddenOrder lists the indices of library cards ids in the order gorge's
// agent is shown them: by card name, then object id, so an option's place
// never tells the library order (xview's sortHidden).
func hiddenOrder(sd *protocol.SeatDecision, ids []string) []int {
	order := make([]int, len(ids))
	for i := range order {
		order[i] = i
	}
	slices.SortStableFunc(order, func(a, b int) int {
		return strings.Compare(knownName(sd, ids[a])+"\x00"+ids[a], knownName(sd, ids[b])+"\x00"+ids[b])
	})
	return order
}

// digEligible reports whether a card matches a Dig's ChangeValid$ filter and
// whether the filter is one this reconstruction understands. The engine has
// already narrowed the wire's options to eligible cards; the neutral world
// rebuilds the same native option list from the printed faces. The common
// spec is "Card" or "Card.nonCreature+nonLand": an optional Card prefix and
// + joined type words, each optionally negated by non. Anything with a scope
// clause or an unrecognised shape fails closed.
func digEligible(f *cards.Face, spec string) (match, known bool) {
	if f == nil {
		return false, true
	}
	for _, alt := range strings.Split(spec, ",") {
		alt = strings.TrimSpace(alt)
		if alt == "" {
			continue
		}
		m, ok := digAlt(f, alt)
		if !ok {
			return false, false
		}
		if m {
			return true, true
		}
	}
	return false, true
}

func digAlt(f *cards.Face, alt string) (bool, bool) {
	if strings.EqualFold(strings.TrimSpace(alt), "Card") {
		return true, true
	}
	if i := strings.IndexByte(alt, '.'); i >= 0 {
		if !strings.EqualFold(strings.TrimSpace(alt[:i]), "Card") {
			return false, false
		}
		alt = alt[i+1:]
		if strings.Contains(alt, ".") {
			return false, false // a scope clause (Card.YouCtrl) is not modelled
		}
	}
	if strings.TrimSpace(alt) == "" {
		return true, true
	}
	parts := strings.FieldsFunc(alt, func(r rune) bool { return r == '+' || r == '&' || r == ' ' })
	if len(parts) == 0 {
		return false, false
	}
	for _, p := range parts {
		neg := false
		if len(p) > 3 && strings.EqualFold(p[:3], "non") {
			neg, p = true, p[3:]
		}
		if p == "" || !cardTypeWords[strings.ToLower(p)] {
			// A predicate this filter does not model (Card.NamedCard,
			// Card.Historic, Card.cmcLE3) is not the same as a type the card
			// lacks: treating it as one would silently mark every card
			// ineligible, and a negated one ("Card.nonHistoric") eligible in
			// reverse.
			return false, false
		}
		has := slices.ContainsFunc(f.Types, func(t string) bool { return strings.EqualFold(t, p) })
		if has == neg {
			return false, true
		}
	}
	return true, true
}

// digSA finds the Dig ability that posed the ask. The source may be a spell
// on the stack or an activated ability (Orcish Librarian's "{R}, {T}: Look at
// the top eight..."), so it searches the same ability roots every other
// source-scoped reconstruction reads rather than only the spell ability.
func (s *Session) digSA(src protocol.ObjectRef, o *protocol.Observation) *cards.SA {
	f, roots := s.abilityRoots(src, o)
	for _, root := range roots {
		if root == nil {
			continue
		}
		for _, sa := range chain(f, root) {
			if sa.API == "Dig" {
				return sa
			}
		}
	}
	return nil
}

// dig is a Dig's look: v2 partitions the window between the primary
// destination and the remainder destination and then orders the remainder.
// gorge asks a KChoose of the eligible cards ("dig", in window order) and,
// when two or more remain, their order as a follow-up. The v2 partition is
// per card, so each candidate maps to a destination op: inside the bot's
// take for the primary destination, outside it for the remainder.
func (s *Session) dig(sd *protocol.SeatDecision) ([]mapping.NativeOp, *native, error) {
	first := sd.Candidates[0].Semantic
	if first.Kind == "arrange_card" && num(first, "card_index") == 0 && sd.Group.SubstepIndex == 0 {
		src, err := optRef(first, "source")
		if err != nil || src == nil {
			return nil, nil, err
		}
		sa := s.digSA(*src, &sd.Observation)
		ids := looked(sd, num(first, "card_count"))
		if sa == nil || ids == nil {
			return nil, nil, nil
		}
		param := func(k string) string { return strings.TrimSpace(sa.Params[k]) }
		primary := param("DestinationZone")
		if primary == "" {
			primary = "Hand"
		}
		rest := param("DestinationZone2")
		pos2 := param("LibraryPosition2")
		if rest == "" {
			rest, pos2 = "Library", "-1" // effects.compileDig's default
		} else if strings.EqualFold(rest, "Library") && pos2 == "" {
			pos2 = "-1" // an explicit Library with no position is the bottom
		}
		primaryV2 := strings.ToLower(primary)
		restV2 := strings.ToLower(rest)
		if strings.EqualFold(rest, "Library") {
			switch pos2 {
			case "0":
				restV2 = "top"
			case "-1":
				restV2 = "bottom"
			}
		}
		n := s.open("dig", decision.Decision{})
		n.arrange = &arrangement{cards: ids}
		switch {
		case strings.EqualFold(param("ChangeNum"), "All") || (rest != "" && !strings.EqualFold(rest, "Library")):
			n.arrange.fixed = true
		case primaryV2 == restV2:
			// Both piles name the same destination: the partition candidates
			// do not say which side a card is on.
			return nil, nil, nil
		default:
			spec := param("ChangeValid")
			if spec == "" {
				spec = "Card" // effects.compileDig's default
			}
			eligible := func(id string) (bool, bool) {
				name := ""
				for _, k := range sd.Observation.Known {
					if k.ObjectID != nil && *k.ObjectID == id {
						name = k.CardName
					}
				}
				return digEligible(s.B.face(&name, false, nil), spec)
			}
			d := decision.Decision{Kind: decision.KChoose, Prompt: "dig", ResumeSA: sa}
			d.Source = s.B.IDs.Of(src.ObjectID)
			n.arrange.native = make([]int, len(ids))
			for ci := range n.arrange.native {
				n.arrange.native[ci] = -1
			}
			for _, ci := range hiddenOrder(sd, ids) {
				ok, known := eligible(ids[ci])
				if !known {
					return nil, nil, nil
				}
				if ok {
					n.arrange.native[ci] = len(d.Options)
					d.Options = append(d.Options, decision.Option{Index: len(d.Options), Kind: "dig", Label: knownName(sd, ids[ci]), Obj: s.B.IDs.Of(ids[ci]), Player: s.seat})
				}
			}
			changeNum, err := strconv.Atoi(param("ChangeNum"))
			anyNum := strings.EqualFold(param("ChangeNum"), "Any")
			if err != nil && !anyNum {
				return nil, nil, nil
			}
			if anyNum {
				changeNum = len(d.Options)
			}
			d.Max = min(changeNum, len(d.Options))
			if !anyNum && !strings.EqualFold(param("Optional"), "True") {
				d.Min = d.Max
			}
			n.d = d
		}
		n.arrange.primary, n.arrange.rest = primaryV2, restV2
		n.arrange.random = strings.EqualFold(param("RestRandomOrder"), "True")
		s.cur = n
	}
	n := s.continuing("dig")
	if n == nil || n.arrange == nil {
		return nil, nil, nil
	}
	a := n.arrange
	ops := make([]mapping.NativeOp, len(sd.Candidates))
	for i := range ops {
		ops[i] = noOp
	}
	if a.fixed {
		ops[0] = mapping.NativeOp{Op: "fixed", Option: -1}
		return ops, n, nil
	}
	var bottom, top []int
	for _, ci := range hiddenOrder(sd, a.cards) {
		if ci >= len(a.dest) {
			continue
		}
		switch a.dest[ci] {
		case "bottom":
			bottom = append(bottom, ci)
		case "top":
			top = append(top, ci)
		}
	}
	if n.follow == nil {
		n.follow = map[string]decision.Decision{}
	}
	if len(a.dest) == len(a.cards) && len(bottom) >= 2 && !a.random {
		fd := decision.Decision{Kind: decision.KArrange, Min: len(bottom), Max: len(bottom), ResumeKind: "dig_arrange",
			ResumeSA: n.d.ResumeSA, Source: n.d.Source, Player: s.seat, Prompt: "Put the remaining cards on the bottom of your library in any order"}
		for j, ci := range bottom {
			fd.Options = append(fd.Options, decision.Option{Index: j, Kind: "dig_bottom", Label: knownName(sd, a.cards[ci]), Obj: s.B.IDs.Of(a.cards[ci]), Player: s.seat})
		}
		n.follow["dig_bottom"] = fd
	}
	if len(a.dest) == len(a.cards) && len(top) >= 2 && !a.random {
		fd := decision.Decision{Kind: decision.KArrange, Min: len(top), Max: len(top), ResumeKind: "dig_arrange",
			ResumeSA: n.d.ResumeSA, Source: n.d.Source, Player: s.seat, Prompt: "Put the remaining cards back on top of your library in any order"}
		for j, ci := range top {
			fd.Options = append(fd.Options, decision.Option{Index: j, Kind: "top", Label: knownName(sd, a.cards[ci]), Obj: s.B.IDs.Of(a.cards[ci]), Player: s.seat})
		}
		n.follow["dig_top"] = fd
	}
	for i, c := range sd.Candidates {
		switch c.Semantic.Kind {
		case "arrange_card":
			card, err := ref(c.Semantic, "card")
			if err != nil {
				return nil, nil, err
			}
			ci := slices.Index(a.cards, card.ObjectID)
			if ci < 0 {
				continue
			}
			dst := strings.ToLower(str(c.Semantic, "destination"))
			if dst != a.primary && dst != a.rest {
				continue
			}
			inside := dst == a.primary
			if inside && a.native[ci] < 0 {
				continue // an ineligible card has no take option
			}
			ops[i] = mapping.NativeOp{Op: "dest", Option: a.native[ci], List: dst, Inside: &inside, Position: ci}
		case "order_pick":
			it, err := field[protocol.OrderItem](c.Semantic, "item")
			if err != nil || it.Object == nil {
				continue
			}
			ci := slices.Index(a.cards, it.Object.ObjectID)
			if ci < 0 || ci >= len(a.dest) {
				continue
			}
			dst := a.dest[ci]
			if a.rest == "top" {
				// A top-remainder Dig's only order ask is that remainder; the
				// neutral's own partition answers may differ from the played
				// game's, so the ask's identity (not the divergent state) names
				// the pile.
				dst = "top"
			}
			pos := 0
			for _, p := range a.placed {
				if j := slices.Index(a.cards, p); j >= 0 && j < len(a.dest) && a.dest[j] == dst {
					pos++
				}
			}
			switch {
			case dst == a.rest && (a.random || (dst != "bottom" && dst != "top")):
				// A random remainder, or one that leaves the library, is moved
				// in library order: the engine poses no order ask, so each
				// ordering pick is the presentational dest mapping poses. The
				// taken pile keeps its Choices order (and its top/bottom ask).
				inside := false
				ops[i] = mapping.NativeOp{Op: "dest", Option: -1, List: dst, Inside: &inside}
			case dst == "hand":
				ops[i] = mapping.NativeOp{Op: "list", Option: a.native[ci], List: "choices", Position: pos}
			case dst == "bottom" && len(bottom) == 1:
				// gorge moves a lone remainder without asking dig_bottom.
				ops[i] = mapping.NativeOp{Op: "dest", Option: -1, List: "bottom"}
			case dst == "bottom":
				ops[i] = mapping.NativeOp{Op: "list", Option: slices.Index(bottom, ci), List: "followup:dig_bottom", Position: pos}
			case dst == "top" && len(top) == 1:
				// A lone remainder stays on top without an order ask.
				ops[i] = mapping.NativeOp{Op: "fixed", Option: -1}
			case dst == "top":
				ops[i] = mapping.NativeOp{Op: "list", Option: slices.Index(top, ci), List: "followup:dig_top", Position: pos}
			}
		}
	}
	return ops, n, nil
}
