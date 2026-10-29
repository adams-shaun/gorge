package neutral

import (
	"fmt"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/rules/cost"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/engine/view"
	"github.com/adams-shaun/gorge/server/spellbench/observe"
	"github.com/adams-shaun/gorge/server/spellbench/protocol"
)

// steps is Section 6.2's phase_step order, which is gorge's state.Step order.
var steps = []string{"untap", "upkeep", "draw", "precombat_main", "beginning_of_combat", "declare_attackers",
	"declare_blockers", "combat_damage", "end_of_combat", "postcombat_main", "end_step", "cleanup"}

// IDs numbers a seat's v2 object ids densely, in first-appearance order, for
// one game. v2 ids are fresh per incarnation (Section 5.3), so one gorge id
// is one object in one zone, which is all gorge's Board needs.
type IDs struct {
	m    map[string]state.ObjID
	back []string
}

func NewIDs() *IDs { return &IDs{m: map[string]state.ObjID{}, back: []string{""}} }

// Of returns id's gorge id, minting one for a new id. The empty id is 0.
func (t *IDs) Of(id string) state.ObjID {
	if id == "" {
		return 0
	}
	n, ok := t.m[id]
	if !ok {
		n = state.ObjID(len(t.back))
		t.m[id] = n
		t.back = append(t.back, id)
	}
	return n
}

// V2 returns the v2 id a gorge id stands for.
func (t *IDs) V2(n state.ObjID) string {
	if int(n) < len(t.back) {
		return t.back[n]
	}
	return ""
}

// Seat parses "p0" or "p1".
func Seat(s string) (state.PlayerID, error) {
	switch s {
	case "p0":
		return 0, nil
	case "p1":
		return 1, nil
	}
	return 0, fmt.Errorf("%q is not a seat", s)
}

// Builder turns observations into gorge views for one seat and game.
type Builder struct {
	Reg *cards.Registry
	IDs *IDs
	// refs caches Resolve by observed name; a miss (a token, a name the
	// registry lacks) caches the zero Ref.
	refs map[string]Ref
	// tokens lists the registry's token scripts by face name, stems sorted.
	tokens map[string][]string
	// blocked and blockTurn are rememberBlocks' per-combat memory: each
	// attacker's blocker count, keyed by its v2 id.
	blocked   map[string]blockMemo
	blockTurn uint32
	// cards is the previous View's per-object card views, keyed by protocol
	// object id. cachedCard.hash is a canonical fingerprint of the record
	// that produced card; View reuses an entry only when the new record
	// hashes the same and the zone's cost projection is unchanged. gen marks
	// the entries touched by the current View, so entries whose object left
	// the observation are deleted and the cache never grows past one
	// observation.
	cards map[string]cachedCard
	gen   uint64
}

// cachedCard is one cached card view: the canonical hash of the object record
// that built it, whether the zone lists ability costs, and the built view.
type cachedCard struct {
	hash  uint64
	costs bool
	gen   uint64
	card  view.CardView
}

// FNV-1a (64-bit). The hash is only a cache fingerprint: a collision rebuilds
// the card, never changes the view.
const (
	fnvOffset64 = 14695981039346656037
	fnvPrime64  = 1099511628211
)

// recordHash is a canonical, allocation-free encoding of a protocol record.
// Every length, field and presence marker is fed in, so two records that
// differ in any field hash apart (modulo the usual 64-bit collision). Map
// entries combine commutatively, so an equal map hashes equal whatever the
// iteration order.
type recordHash struct{ h uint64 }

func (r *recordHash) u64(v uint64) {
	for i := 0; i < 8; i++ {
		r.h ^= v & 0xff
		r.h *= fnvPrime64
		v >>= 8
	}
}

func (r *recordHash) boolean(b bool) {
	if b {
		r.u64(1)
	} else {
		r.u64(0)
	}
}

func (r *recordHash) str(s string) {
	r.u64(uint64(len(s)))
	for i := 0; i < len(s); i++ {
		r.h ^= uint64(s[i])
		r.h *= fnvPrime64
	}
}

func (r *recordHash) strp(s *string) {
	if s == nil {
		r.u64(0)
		return
	}
	r.u64(1)
	r.str(*s)
}

func (r *recordHash) strs(ss []string) {
	r.u64(uint64(len(ss)))
	for _, s := range ss {
		r.str(s)
	}
}

func (r *recordHash) i32p(v *int32) {
	if v == nil {
		r.u64(0)
		return
	}
	r.u64(1)
	r.u64(uint64(uint32(*v)))
}

func (r *recordHash) u32p(v *uint32) {
	if v == nil {
		r.u64(0)
		return
	}
	r.u64(1)
	r.u64(uint64(*v))
}

func (r *recordHash) ref(o *protocol.ObjectRef) {
	if o == nil {
		r.u64(0)
		return
	}
	r.u64(1)
	r.str(o.ObjectID)
	r.strp(o.CardName)
	r.str(o.OwnerSeat)
	r.str(o.ControllerSeat)
	r.str(o.Zone)
}

func (r *recordHash) refs(rs []protocol.ObjectRef) {
	r.u64(uint64(len(rs)))
	for i := range rs {
		r.ref(&rs[i])
	}
}

func (r *recordHash) target(t *protocol.TargetRef) {
	switch {
	case t == nil:
		r.u64(0)
	case t.Player != nil:
		r.u64(1)
		r.str(*t.Player)
	case t.Object != nil:
		r.u64(2)
		r.ref(t.Object)
	default:
		r.u64(3)
	}
}

func (r *recordHash) counters(m map[string]uint32) {
	r.u64(uint64(len(m)))
	var sum uint64
	for k, n := range m {
		var e recordHash
		e.h = fnvOffset64
		e.str(k)
		e.u64(uint64(n))
		sum += e.h
	}
	r.u64(sum)
}

func (r *recordHash) chosen(ms []map[string]string) {
	r.u64(uint64(len(ms)))
	for _, m := range ms {
		r.u64(uint64(len(m)))
		var sum uint64
		for k, v := range m {
			var e recordHash
			e.h = fnvOffset64
			e.str(k)
			e.str(v)
			sum += e.h
		}
		r.u64(sum)
	}
}

func (r *recordHash) characteristics(c *protocol.Characteristics) {
	if c == nil {
		r.u64(0)
		return
	}
	r.u64(1)
	r.strs(c.Supertypes)
	r.strs(c.Types)
	r.strs(c.Subtypes)
	r.strs(c.Colors)
	r.u64(uint64(c.ManaValue))
	r.i32p(c.Power)
	r.i32p(c.Toughness)
	r.strs(c.Keywords)
}

func (r *recordHash) permanent(p *protocol.Permanent) {
	if p == nil {
		r.u64(0)
		return
	}
	r.u64(1)
	r.boolean(p.Tapped)
	r.boolean(p.SummoningSick)
	r.u64(uint64(p.Damage))
	r.counters(p.Counters)
	r.target(p.AttachedTo)
	r.boolean(p.Attacking)
	r.target(p.AttackTarget)
	r.boolean(p.Blocking)
	r.refs(p.BlockedAttackers)
	r.boolean(p.PhasedOut)
	r.strs(p.Statuses)
	r.u32p(p.ClassLevel)
	r.chosen(p.Chosen)
}

// hashObjectRecord fingerprints every field of rec. Fields View does not read
// are hashed too, so the cache contract is the record's identity, not a
// list of the fields card happens to use today.
func hashObjectRecord(rec protocol.ObjectRecord) uint64 {
	r := recordHash{h: fnvOffset64}
	r.str(rec.ObjectID)
	r.strp(rec.CardName)
	r.str(rec.OwnerSeat)
	r.str(rec.ControllerSeat)
	r.str(rec.Zone)
	r.strp(rec.FullName)
	r.boolean(rec.FaceDown)
	r.boolean(rec.Token)
	r.boolean(rec.Copy)
	r.characteristics(rec.Characteristics)
	r.permanent(rec.Permanent)
	r.ref(rec.ExiledBy)
	return r.h
}

// blockMemo is one attacker's blocker count in the current combat and the
// attack target it had when the count was taken.
type blockMemo struct {
	n      int
	target string
}

func NewBuilder(reg *cards.Registry) *Builder {
	b := &Builder{Reg: reg, IDs: NewIDs(), refs: map[string]Ref{}, tokens: map[string][]string{},
		blocked: map[string]blockMemo{}, cards: map[string]cachedCard{}}
	for stem, c := range reg.Tokens {
		if len(c.Faces) > 0 {
			b.tokens[c.Faces[0].Name] = append(b.tokens[c.Faces[0].Name], stem)
		}
	}
	for _, stems := range b.tokens {
		sort.Strings(stems)
	}
	return b
}

func (b *Builder) ref(name *string) Ref {
	if name == nil {
		return Ref{}
	}
	r, ok := b.refs[*name]
	if !ok {
		r, _ = Resolve(b.Reg, *name)
		b.refs[*name] = r
	}
	return r
}

// face is the printed face an observed object shows: a card's face by name
// (a copy, token or not, shows the card it copies), or for any other token
// the token script of that name whose colors and printed keywords agree
// with what is observed. It is nil when neither is known.
func (b *Builder) face(name *string, token bool, c *protocol.Characteristics) *cards.Face {
	if name == nil {
		return nil
	}
	if !token {
		r := b.ref(name)
		if r.Card == nil {
			return nil
		}
		return r.Card.Faces[r.Face]
	}
	var best *cards.Face
	for _, stem := range b.tokens[*name] {
		f := b.Reg.Tokens[stem].Faces[0]
		if c != nil && f.Colors != strings.Join(c.Colors, ",") {
			continue
		}
		if c != nil && !slices.ContainsFunc(f.Keywords, func(l string) bool {
			k := observe.Keywords([]string{l})
			return len(k) == 1 && !slices.Contains(c.Keywords, k[0])
		}) {
			return f
		}
		if best == nil {
			best = f
		}
	}
	return best
}

// View builds the seat's gorge view from its observation. Every fact comes
// from the observation, except the printed facts gorge's Board reads off a
// card (mana cost, type line, spell API, mana production), which come from
// the registry by the card's name.
func (b *Builder) View(o *protocol.Observation) (view.View, error) {
	viewer, err := Seat(o.Viewer)
	if err != nil {
		return view.View{}, err
	}
	v := view.View{Viewer: viewer, Visibility: "seat", Turn: int32(o.Turn), Round: roundOf(o.Turn),
		Players: []view.PlayerView{}, Stack: []view.StackView{}, Pending: []view.PendingView{}}
	// day_night: the observation spells out CR 731's designation. A null
	// field means the host declared the flag absent; the view then keeps its
	// zero (neither), never inventing a day/night state.
	if o.DayNight != nil {
		v.DayNight = *o.DayNight
	}
	// pregame is gorge's untap step of turn 0.
	i := 0
	if o.PhaseStep != "pregame" {
		if i = slices.Index(steps, o.PhaseStep); i < 0 {
			return view.View{}, fmt.Errorf("phase_step %q", o.PhaseStep)
		}
	}
	v.Step, v.Phase = state.Step(i).String(), view.PhaseOf(state.Step(i))
	if o.ActiveSeat != nil {
		if v.Active, err = Seat(*o.ActiveSeat); err != nil {
			return view.View{}, err
		}
	}
	if o.PrioritySeat != nil {
		if v.Priority, err = Seat(*o.PrioritySeat); err != nil {
			return view.View{}, err
		}
	}
	blockers := map[string][]state.ObjID{} // attacker v2 id -> its blockers
	for _, p := range o.Players {
		for _, r := range p.Battlefield {
			if r.Permanent != nil {
				for _, a := range r.Permanent.BlockedAttackers {
					blockers[a.ObjectID] = append(blockers[a.ObjectID], b.IDs.Of(r.ObjectID))
				}
			}
		}
	}
	b.rememberBlocks(o, state.Step(i), blockers)
	b.gen++
	for _, p := range o.Players {
		seat, err := Seat(p.Seat)
		if err != nil {
			return view.View{}, err
		}
		pool := p.ManaPool
		// Pool is always non-nil, but native view's poolView keeps only the
		// colours holding mana: an empty pool is {}, never six zero keys. The
		// observation sends all six amounts, so the rebuild drops the zeros.
		pv := view.PlayerView{ID: seat, Name: p.Seat, Life: p.Life, LibrarySize: int(p.LibraryCount),
			HandSize: int(p.HandCount), GraveyardSize: len(p.Graveyard),
			Pool:          map[string]int32{},
			LandDropSpent: p.LandsPlayedThisTurn > 0,
			Command:       []view.CardView{}, Commanders: []view.CardView{}, CommanderCasts: []int32{}}
		for _, m := range [...]struct {
			sym string
			n   uint32
		}{{"W", pool.W}, {"U", pool.U}, {"B", pool.B}, {"R", pool.R}, {"G", pool.G}, {"C", pool.C}} {
			if m.n > 0 {
				pv.Pool[m.sym] = int32(m.n)
			}
		}
		// player counters: view.PlayerView.Counters is the same map native
		// builds from state.Player.Counters. Poison is split into its own
		// protocol field and folded back into the same "POISON" key native
		// uses. A positive poison is sent only in the field; a zero-valued
		// poison counter (which state retains after a removal) is also sent
		// in counters as its presence marker, so it rebuilds the zero entry
		// native keeps. A null counters/poison (flag false) leaves the map
		// nil, so a host that sends nothing cannot make the seat look
		// poisoned.
		ctr := map[string]int32{}
		for k, n := range p.Counters {
			ctr[nativeCounter(k)] = int32(n)
		}
		if p.Poison != nil && *p.Poison > 0 {
			ctr["POISON"] = int32(*p.Poison)
		}
		if len(ctr) > 0 {
			pv.Counters = ctr
		}
		// designations: view.PlayerView's only designation field is
		// HasInitiative (CR 726.1). The monarch has no view.View field native
		// uses, so a "monarch" entry is deliberately not turned into an
		// invented field here; the observation still carries it for other
		// clients. A null designations array stays "nothing declared".
		if slices.Contains(p.Designations, "initiative") {
			pv.HasInitiative = true
		}
		zone := func(rs []protocol.ObjectRecord, costs bool) ([]view.CardView, error) {
			out := make([]view.CardView, 0, len(rs))
			for _, r := range rs {
				cv, err := b.card(r, blockers, costs)
				if err != nil {
					return nil, err
				}
				out = append(out, cv)
			}
			return out, nil
		}
		if seat == viewer {
			if pv.Hand, err = zone(p.Hand, true); err != nil {
				return view.View{}, err
			}
		}
		for _, z := range []struct {
			dst   *[]view.CardView
			src   []protocol.ObjectRecord
			costs bool
		}{{&pv.Battlefield, p.Battlefield, true}, {&pv.Graveyard, p.Graveyard, false},
			{&pv.Exile, p.Exile, false}, {&pv.Command, p.Command, false}} {
			if *z.dst, err = zone(z.src, z.costs); err != nil {
				return view.View{}, err
			}
		}
		v.Players = append(v.Players, pv)
	}
	for _, e := range o.Stack {
		sv, err := b.stack(e)
		if err != nil {
			return view.View{}, err
		}
		v.Stack = append(v.Stack, sv)
	}
	// Map iteration is only pruning: the view above was built in the
	// observation's record order, never in cache order.
	for id, c := range b.cards {
		if c.gen != b.gen {
			delete(b.cards, id)
		}
	}
	return v, nil
}

// rememberBlocks restores blocks the observation no longer shows. An attacker
// stays blocked when its blocker leaves the battlefield or phases out (CR
// 509.1h), and gorge's view keeps a zero tombstone in BlockedBy for each such
// blocker; a v2 observation lists blocks only from the blocker's side, so a
// departed blocker drops out of it. The builder remembers each attacker's
// blocker count across one combat's observations and pads the live list with
// zero tombstones up to it. The memory resets on a new turn and at any step
// outside declare_blockers..end_of_combat, and an attacker's entry goes when
// it stops attacking or its attack target changes (gorge clears a re-pointed
// attacker's blocks).
func (b *Builder) rememberBlocks(o *protocol.Observation, step state.Step, blockers map[string][]state.ObjID) {
	if o.Turn != b.blockTurn || step < state.StepDeclareBlockers || step > state.StepEndCombat {
		clear(b.blocked)
		b.blockTurn = o.Turn
		if step < state.StepDeclareBlockers || step > state.StepEndCombat {
			return
		}
	}
	attacking := map[string]bool{}
	for _, p := range o.Players {
		for _, r := range p.Battlefield {
			if r.Permanent == nil || !r.Permanent.Attacking {
				continue
			}
			id, target := r.ObjectID, attackTargetKey(r.Permanent.AttackTarget)
			attacking[id] = true
			m, ok := b.blocked[id]
			if ok && m.target != target {
				ok = false
			}
			live := len(blockers[id])
			switch {
			case ok && m.n > live:
				blockers[id] = append(blockers[id], make([]state.ObjID, m.n-live)...)
			case live > 0:
				b.blocked[id] = blockMemo{n: live, target: target}
			default:
				delete(b.blocked, id)
			}
		}
	}
	for id := range b.blocked {
		if !attacking[id] {
			delete(b.blocked, id)
		}
	}
}

func attackTargetKey(t *protocol.TargetRef) string {
	switch {
	case t == nil:
		return ""
	case t.Player != nil:
		return "p:" + *t.Player
	case t.Object != nil:
		return "o:" + t.Object.ObjectID
	}
	return ""
}

// card returns the view of one object record, reusing the previous view when
// the record and the zone's cost projection are unchanged. BlockedBy is
// applied after the cache lookup because it depends on other objects'
// records, not this one's.
func (b *Builder) card(r protocol.ObjectRecord, blockers map[string][]state.ObjID, costs bool) (view.CardView, error) {
	if b.cards == nil {
		b.cards = map[string]cachedCard{}
	}
	h := hashObjectRecord(r)
	if c, ok := b.cards[r.ObjectID]; ok && c.hash == h && c.costs == costs {
		c.gen = b.gen
		b.cards[r.ObjectID] = c
		cv := c.card
		applyBlocks(&cv, r, blockers)
		return cv, nil
	}
	cv, err := b.buildCard(r, costs)
	if err != nil {
		return view.CardView{}, err
	}
	b.cards[r.ObjectID] = cachedCard{hash: h, costs: costs, gen: b.gen, card: cv}
	applyBlocks(&cv, r, blockers)
	return cv, nil
}

// applyBlocks sets the tombstone-aware BlockedBy list. A face-down card whose
// name the viewer may not see is projected with no permanent facts at all, as
// buildCard returns it before reading the permanent.
func applyBlocks(cv *view.CardView, r protocol.ObjectRecord, blockers map[string][]state.ObjID) {
	if r.CardName == nil || r.Permanent == nil {
		return
	}
	if bs := blockers[r.ObjectID]; len(bs) > 0 {
		cv.BlockedBy = slices.Clone(bs)
	} else {
		cv.BlockedBy = nil
	}
}

func (b *Builder) buildCard(r protocol.ObjectRecord, costs bool) (view.CardView, error) {
	controller, err := Seat(r.ControllerSeat)
	if err != nil {
		return view.CardView{}, err
	}
	owner, err := Seat(r.OwnerSeat)
	if err != nil {
		return view.CardView{}, err
	}
	id := b.IDs.Of(r.ObjectID)
	cv := view.CardView{ID: id, Controller: controller, Owner: owner, FaceDown: r.FaceDown,
		IsToken: r.Token, IsCopy: r.Copy, Token: "#" + strconv.FormatUint(uint64(id), 10)}
	if r.CardName == nil {
		// A face-down card the viewer may not look at: gorge projects it
		// with every printed field blank.
		return cv, nil
	}
	f := b.face(r.CardName, r.Token && !r.Copy, r.Characteristics)
	b.printed(&cv, f, *r.CardName, r.Characteristics)
	b.frontName(&cv, r.CardName, r.Token)
	if costs && f != nil {
		cv.AbilityCosts = printedAbilityCosts(f)
	}
	if c := r.Characteristics; c != nil {
		if c.Power != nil {
			cv.Power = *c.Power
		}
		if c.Toughness != nil {
			cv.Toughness = *c.Toughness
		}
		cv.Keywords = keywordLines(f, c.Keywords)
	}
	if p := r.Permanent; p != nil {
		cv.Tapped, cv.Damage, cv.SummonSick, cv.Attacking = p.Tapped, int32(p.Damage), p.SummoningSick, p.Attacking
		if p.AttachedTo != nil && p.AttachedTo.Object != nil {
			cv.AttachedTo = b.IDs.Of(p.AttachedTo.Object.ObjectID)
		}
		if p.Attacking && p.AttackTarget != nil && p.AttackTarget.Player != nil {
			who, err := Seat(*p.AttackTarget.Player)
			if err != nil {
				return view.CardView{}, err
			}
			cv.AttackingPlayer = &who
		}
		// permanent counters: the host normalises counter names (Section
		// 6.10); native view.View.Counters holds the engine's own spellings
		// ("P1P1", "Loyalty"). A null counters map (permanent_details false)
		// leaves the CardView without counters rather than an empty map.
		if len(p.Counters) > 0 {
			cv.Counters = map[string]int32{}
			for k, n := range p.Counters {
				cv.Counters[nativeCounter(k)] = int32(n)
			}
		}
		// statuses: view.CardView.Flags is native's bit set for the status
		// designations it models (suspected, renowned, monstrous, harnessed,
		// solved, cloaked, the Ring-bearer, morph/disguise, unlocked Room
		// doors). A status outside that set (goaded) has no view field and is
		// not invented into one.
		for _, s := range p.Statuses {
			cv.Flags |= statusFlag(s)
		}
		// chosen and class_level have no field in view.CardView: native's
		// projection carries neither a permanent's chosen name/type nor its
		// CR 716 level, so the neutral rebuild reads the fields (validating
		// their presence is the host's job) and maps them to nothing. Adding
		// a field here would make neutral richer than native, not equal.
	}
	return cv, nil
}

// roundOf derives the board's round from the observation's turn. The v2
// observation is exactly two seats (Section 6's players is two records), so
// one round is two turns and the first turn opens round 1:
// round = 1 + (turn-1)/2. view.RoundOf counts the same boundary from the
// event log, including eliminated seats it can no longer reach; a two-seat
// game ends when one seat loses, so no turn after that exists to diverge.
// Turn 0 (pregame) is round 1, as RoundOf's empty-log guard has it.
func roundOf(turn uint32) int32 {
	if turn == 0 {
		return 1
	}
	return int32((turn-1)/2) + 1
}

// frontName fills CardView.CardName the way native's cardView does: the
// resolved card's printed front-face name when it differs from the object's
// shown name (a transformed back face, a copy, a SetName$ rename). A token's
// registry lookup would find a same-named card rather than the token script,
// and its one face is the shown name anyway, so tokens keep it empty.
func (b *Builder) frontName(cv *view.CardView, name *string, token bool) {
	if name == nil || token {
		return
	}
	r := b.ref(name)
	if r.Card == nil || len(r.Card.Faces) == 0 || r.Card.Faces[0] == nil {
		return
	}
	if front := r.Card.Faces[0].Name; front != cv.Name {
		cv.CardName = front
	}
}

// printedAbilityCosts is the no-rules fallback native's zoneCard uses when its
// Chars is absent: each non-mana activated ability's printed Cost$, in face
// order. The observation carries no offer-time cost, so the plain rebuild can
// only recover the printed one; a live RaiseCost/ReduceCost is a protocol gap
// the degradation census records.
func printedAbilityCosts(f *cards.Face) []string {
	var out []string
	for _, a := range f.Abilities {
		if a.Kind == "AB" && !cards.IsManaAbilitySA(a) {
			out = append(out, cost.FormatCost(cost.ParseCost(a.Params["Cost"])))
		}
	}
	return out
}

// printed fills a card's printed facts. Observed Supertypes/Types/Subtypes
// win when the observation carries them (the other engine may have applied
// continuous effects the registry's printed face does not know); a card with
// no observed types falls back to its registry face. The remaining printed
// facts (mana cost, spell API, mana production) come from the face, except
// that an observed mana value of 0 clears a printed cost.
func (b *Builder) printed(cv *view.CardView, f *cards.Face, name string, c *protocol.Characteristics) {
	cv.Name = name
	cv.Printing = view.Printing{Name: name}
	if c != nil && (len(c.Supertypes) > 0 || len(c.Types) > 0 || len(c.Subtypes) > 0) {
		var words []string
		for _, list := range [][]string{c.Supertypes, c.Types, c.Subtypes} {
			for _, w := range list {
				words = append(words, title(w))
			}
		}
		cv.Types = strings.Join(words, " ")
	}
	if f == nil {
		return
	}
	cv.Name, cv.Printing.Name = f.Name, f.Name
	if cv.Types == "" {
		cv.Types = strings.Join(f.Types, " ")
	} else if sameTypeLine(c, f.Types) {
		// The observation normalises each type word (lower case, punctuation
		// folded to "_"), losing "Urza's" and "Assembly-Worker" spelling. When
		// the observed set is the printed set, the registry's line is the same
		// fact with its original punctuation and casing.
		cv.Types = strings.Join(f.Types, " ")
	}
	cv.ManaCost = f.ManaCost
	cv.PrintedPT = f.PT
	cv.ColourIdentity = cards.ColourMask(f.ColourIdentity())
	cv.PlayMain1 = f.PlayMain1Hint()
	cv.RemoveDeck = f.RemoveDeck != ""
	// An observed mana value of 0 on a face whose printed cost is not 0 is
	// a copy that has no mana cost (CR 707.9b: Embalm, Eternalize).
	if c != nil && c.ManaValue == 0 && f.Cmc() > 0 {
		cv.ManaCost = ""
	}
	if sa := f.SpellAbility(); sa != nil {
		// The spell-ability projection: the same facts view.cardView reads
		// off the printed face, so the heuristic's cast and timing gates see
		// the same card through the rebuilt view as through native's. The
		// pump facts are load-bearing: without them the A6 PumpInCombat rule
		// cannot recognise a combat trick as a timed pump and holds it
		// nowhere (Violent Urge/Thwip! were cast in the precombat main where
		// native passes).
		cv.SpellAPI = sa.API
		cv.RemovalKind = effects.CastRemovalKind(sa)
		cv.RemovalTargets = effects.CastRemovalTargets(sa)
		cv.RemovalDamage = effects.CastRemovalDamage(sa)
		cv.PumpPower, cv.PumpToughness = effects.CastPumpBonus(sa)
		cv.PumpOwn = effects.CastPumpOwnCombatants(sa)
		cv.CombatTrick = effects.CastCombatTrick(sa)
		cv.AuraLogic = effects.CastAuraLogic(f)
	}
	if p := f.ManaProduction(); !p.IsZero() {
		cv.Produces = &p
	}
}

// sameTypeLine reports whether observed's normalised type words are exactly
// face's printed type words, so the registry line can supply the punctuation
// and casing the observation folded away. A type-changing effect leaves the
// sets unequal and the observed line stands.
func sameTypeLine(c *protocol.Characteristics, face []string) bool {
	if c == nil || len(face) == 0 {
		return false
	}
	words := 0
	for _, list := range [][]string{c.Supertypes, c.Types, c.Subtypes} {
		words += len(list)
	}
	if words == 0 {
		return false
	}
	// Build the same joined form strings.Join would, without the temporary
	// []string allocations: this runs once per rebuilt card, and its
	// allocations dominated an uncached View.
	var obs, printed strings.Builder
	first := true
	for _, list := range [][]string{c.Supertypes, c.Types, c.Subtypes} {
		for _, w := range list {
			if !first {
				obs.WriteByte('_')
			}
			first = false
			writeTypeWord(&obs, w)
		}
	}
	for i, w := range face {
		if i > 0 {
			printed.WriteByte('_')
		}
		writeTypeWord(&printed, w)
	}
	return obs.String() == printed.String()
}

// writeTypeWord writes observe.Normalize(w), with a fast path for the words
// Normalize leaves unchanged (the common printed type word). Only the
// characters Normalize folds send a word down the slow path, so the two forms
// are byte-identical.
func writeTypeWord(b *strings.Builder, w string) {
	for i := 0; i < len(w); i++ {
		c := w[i]
		if c >= 'A' && c <= 'Z' || c <= ' ' || c == '\'' || c == '-' || c >= 0x80 {
			b.WriteString(observe.Normalize(w))
			return
		}
	}
	b.WriteString(w)
}

func (b *Builder) stack(e protocol.StackEntry) (view.StackView, error) {
	controller, err := Seat(e.ControllerSeat)
	if err != nil {
		return view.StackView{}, err
	}
	sv := view.StackView{ID: b.IDs.Of(e.ObjectID), Controller: controller, Targets: []view.TargetView{}}
	if e.CardName != nil {
		sv.Name = *e.CardName
	}
	for _, t := range e.Targets {
		switch {
		case t == nil:
		case t.Player != nil:
			who, err := Seat(*t.Player)
			if err != nil {
				return view.StackView{}, err
			}
			sv.Targets = append(sv.Targets, view.TargetView{Player: who, IsPlayer: true})
		case t.Object != nil:
			sv.Targets = append(sv.Targets, view.TargetView{Obj: b.IDs.Of(t.Object.ObjectID)})
		}
	}
	switch e.StackKind {
	case "spell":
		sv.Kind = "spell"
		owner, err := Seat(e.OwnerSeat)
		if err != nil {
			return view.StackView{}, err
		}
		if e.CardName != nil {
			cv := view.CardView{ID: sv.ID, Controller: controller, Owner: owner, Token: "#" + strconv.FormatUint(uint64(sv.ID), 10)}
			f := b.face(e.CardName, false, e.Characteristics)
			b.printed(&cv, f, *e.CardName, e.Characteristics)
			b.frontName(&cv, e.CardName, false)
			cv.IsCopy = e.Copy
			if c := e.Characteristics; c != nil {
				if c.Power != nil {
					cv.Power = *c.Power
				}
				if c.Toughness != nil {
					cv.Toughness = *c.Toughness
				}
				cv.Keywords = keywordLines(f, c.Keywords)
			}
			sv.Name = cv.Name
			sv.Card = &cv
		}
	case "triggered_ability":
		sv.Kind = "trigger"
	default:
		sv.Kind = "ability"
	}
	if e.Source != nil {
		sv.Source = b.IDs.Of(e.Source.ObjectID)
	}
	return sv, nil
}

// keywordLines turns Section 6.10 keyword names back into gorge's keyword
// lines: the printed face's own line when it has one for that keyword (so a
// parameter such as a flashback cost survives), else the name in gorge's
// title case ("first_strike" -> "First Strike"). The face's lines that name
// no CR 702 keyword (gorge's internal ones, such as etbCounter) are kept
// first, as gorge's derived list carries them.
func keywordLines(f *cards.Face, names []string) []string {
	var out []string
	if f != nil {
		for _, l := range f.Keywords {
			if len(observe.Keywords([]string{l})) == 0 {
				out = append(out, l)
			}
		}
	}
	for _, k := range names {
		line := ""
		if f != nil {
			for _, l := range f.Keywords {
				if got := observe.Keywords([]string{l}); len(got) == 1 && got[0] == k {
					line = l
					break
				}
			}
		}
		if line == "" {
			words := strings.Split(k, "_")
			for i, w := range words {
				words[i] = title(w)
			}
			line = strings.Join(words, " ")
		}
		out = append(out, line)
	}
	return out
}

var ptCounter = regexp.MustCompile(`^[pm]\d+[pm]\d+$`)

// nativeTitleCounters are the engine's non-keyword counter kinds whose
// canonical spelling is title case. The corpus writes these player counters
// that way, and state.CanonicalCounterKind does not fold them.
var nativeTitleCounters = map[string]string{"contract": "Contract", "experience": "Experience"}

// nativeCounter reverses observe.Counter's Section 6.10 normalisation for the
// engine spellings view.View.Counters carries. The P/T counters are upper-case
// ("P1P1"/"M1M1"); CR 122.1b keyword counters are title case ("Flying",
// "First Strike"), which cards.CounterKeyword recognises case-insensitively;
// the two engine player counters above are also title case; every other kind
// is upper-case ("loyalty" -> "LOYALTY", "ticket" -> "TICKET"). A kind whose
// engine spelling matches none of these is lossy, which is why the census
// test guards the round trip on real games.
func nativeCounter(k string) string {
	if ptCounter.MatchString(strings.ToLower(k)) {
		return strings.ToUpper(k)
	}
	if kind, ok := nativeTitleCounters[k]; ok {
		return kind
	}
	words := strings.Split(k, "_")
	for i, w := range words {
		words[i] = title(w)
	}
	if kind, ok := cards.CounterKeyword(strings.Join(words, " ")); ok {
		return kind
	}
	return strings.ToUpper(k)
}

// statusFlag maps a Section 6.10 status name to the CardFlag bit native's
// view.cardFlags sets for it. Names the view cannot represent return 0.
func statusFlag(s string) view.CardFlag {
	switch s {
	case "suspected":
		return view.FlagSuspected
	case "renowned":
		return view.FlagRenowned
	case "monstrous":
		return view.FlagMonstrous
	case "harnessed":
		return view.FlagHarnessed
	case "solved":
		return view.FlagSolved
	case "cloaked":
		return view.FlagCloaked
	case "ring_bearer":
		return view.FlagRingBearer
	case "morphed":
		return view.FlagMorphed
	case "disguised":
		return view.FlagDisguised
	case "left_door":
		return view.FlagLeftDoor
	case "right_door":
		return view.FlagRightDoor
	}
	return 0
}

func title(w string) string {
	if w == "" {
		return w
	}
	return strings.ToUpper(w[:1]) + w[1:]
}
