package oracle

import (
	"sort"
	"strings"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The elided-mana mirror. Gorge applies a play's payment plan's mana
// activations at the play's submit; the kernel re-derives the same payment at
// the play's Apply. Between the two, the canonical menus are priced from
// different positions (gorge's post-activation, the kernel's from the raw
// position), so the kernel over-offers casts gorge withholds. The gorge side
// records every mana ability its engine resolved while processing a canonical
// answer (ManaAbilityHook, fired pre-resolution) and pairs it with the mana
// its resolution and the plan's machinery moved (the ManaAdd events of the
// span), and the lockstep drains the record and hands it to the kernel side
// (dirac.State.MirrorManaUse), which applies the activations and the plan's
// sub-cost spends out of band before its own processing of the same answer.

// ElidedDrains is a side that records the elided mana activations its engine
// resolved since the last drain (the gorge side).
type ElidedDrains interface {
	DrainElided() []dirac.ManaUse
}

// ElidedMirrors is a side that applies the elided mana activations of the
// other engine out of band (the kernel side). arm marks the uses as the
// payment of the answer the side is about to apply (the drain before its
// Apply); a drain before its Step arms nothing.
type ElidedMirrors interface {
	MirrorElided(uses []dirac.ManaUse, arm bool)
}

// mirrorElided drains the lead's elided mana uses into the follow, when the
// lead is the gorge side and the follow the kernel side. A kernel-led run
// mirrors nothing: the kernel processes an answer before gorge follows it, so
// there is no record to hand over in time. afterStep marks the drain after
// the lead's Step (the resolution-time payments gorge's engine funds there):
// the follow has already processed the answer's Apply by then, and where its
// own re-derivation of that payment already landed on gorge's position
// (reconcileElided) the uses are dropped.
func mirrorElided(lead, follow Side, afterStep bool) {
	rec, ok := lead.(ElidedDrains)
	if !ok {
		return
	}
	mir, ok := follow.(ElidedMirrors)
	if !ok {
		return
	}
	uses := rec.DrainElided()
	if len(uses) == 0 {
		return
	}
	if afterStep {
		uses = reconcileElided(lead, follow, uses)
		if len(uses) == 0 {
			return
		}
	}
	mir.MirrorElided(uses, !afterStep)
}

// reconcileElided drops, from a step drain's uses, those of a seat whose
// pool the kernel already holds at the lead's position: an answer whose cost
// gorge funds at its resolution (after this answer's Apply, which the
// kernel's own re-derivation runs against) leaves the kernel holding the
// same plan's floats by the time this drain reads, and re-applying the plan
// would double them (the kernel's over-float the pair-10 echo payment
// measured). A life use is held to the life too: the kernel's re-derivation
// can fund the floats from other sources while missing the plan's life cost
// (Myr Convert's Pay 2 life), and only the life is then applied.
func reconcileElided(lead, follow Side, uses []dirac.ManaUse) []dirac.ManaUse {
	var gp, kp Projection
	lead.Project(&gp)
	follow.Project(&kp)
	// Per seat: the units of the final cost payment gorge spent (a use's
	// Trimmed), and whether a life use meets a life the kernel has not paid.
	var trim [len(gp.Pool)]int32
	var lifeOff [len(gp.Pool)]bool
	for _, u := range uses {
		if int(u.Seat) < len(trim) {
			trim[u.Seat] += u.Trimmed
			lifeOff[u.Seat] = lifeOff[u.Seat] || u.Life != 0 && gp.Life[u.Seat] != kp.Life[u.Seat]
		}
	}
	out := uses[:0]
	for _, u := range uses {
		if u.Rider {
			continue // the kernel's own funding of this payment runs the rider (Step follows)
		}
		s := u.Seat
		if int(s) < len(gp.Pool) && gp.Pool[s] == kp.Pool[s] {
			switch {
			case !lifeOff[s]:
				continue // the kernel already holds this plan's floats
			case trim[s] == 0 && u.Life == 0:
				// the kernel holds the floats but not the plan's life: the
				// life alone is applied
				continue
			}
			// else gorge's pool is empty of the plan's floats (its payment
			// spent them) and the kernel's life is unpaid: the kernel's
			// payment has not run, and the plan is applied whole for it to
			// spend
		}
		out = append(out, u)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// elFire is one resolved mana ability: its player and source, the log
// position it resolved from (the hook fires pre-resolution), and whether its
// resolution has produced mana yet. A colour ask pauses the resolution after
// the hook: the answer's continuation re-fires the hook for the same ability
// and the two records are merged at the drain, so the activation is tapped
// once.
type elFire struct {
	p       state.PlayerID
	src     state.ObjID
	at      int
	found   bool
	ab      int            // the printed ability's index on the source (-1: granted or unknown)
	granted bool           // the ability is granted (not one of the source's printed ones)
	ctr     bool           // the activation changed counters on its source (a depletion land's or a Wall of Roots' cost)
	ctrN    int32          // the counters' net change (a variable cost: Dwarven Hold's "remove X storage counters")
	sac     bool           // the source was sacrificed as the activation's cost (a Treasure, a Lotus Petal)
	last    int            // the log position of the activation's last mana or life event
	oth     [2]state.ObjID // the other permanents its cost tapped (a Springleaf Drum's creature)
	noth    uint8
}

// abilityIndex is the index of sa among src's face abilities (the kernel's
// ability number for a printed one), or -1: an ability the source was granted
// (an effect's "has '...'"), or none.
func (g *GorgeSide) abilityIndex(src state.ObjID, sa *cards.SA) int {
	ob := g.E.G.Obj(src)
	if ob == nil || sa == nil {
		return -1
	}
	f := ob.Face()
	if f == nil {
		return -1
	}
	for i, a := range f.Abilities {
		if a == sa {
			return i
		}
	}
	return -1
}

// elideHook is the ManaAbilityHook the gorge side runs (chained under the
// coverage probe, which wraps whatever hook is installed): one record per
// resolved mana ability, pre-resolution. It emits nothing.
func (g *GorgeSide) elideHook(p state.PlayerID, src state.ObjID, sa *cards.SA) {
	if src != 0 && src == g.elPrimSrc {
		return // the canonical mana activation's own fire (any of its submits): not recorded
	}
	ab := g.abilityIndex(src, sa)
	g.elFires = append(g.elFires, elFire{p: p, src: src, at: len(g.E.L.Events), ab: ab, granted: sa != nil && ab < 0})
}

// armElided arms the lead's record when the follow side mirrors it.
func armElided(lead, follow Side) {
	if a, ok := lead.(interface{ ArmElided() }); ok {
		if _, ok := follow.(ElidedMirrors); ok {
			a.ArmElided()
		}
	}
}

// ArmElided installs the record hook on the engine, chained over any hook
// already there. Only a gorge-led lockstep whose follow side mirrors the
// record drains it (LockstepLed), so no other GorgeSide grows the record.
func (g *GorgeSide) ArmElided() {
	if g.elArmed || g.E == nil {
		return
	}
	g.elArmed = true
	prev := g.E.ManaAbilityHook
	g.E.ManaAbilityHook = func(p state.PlayerID, src state.ObjID, sa *cards.SA) {
		g.elideHook(p, src, sa)
		if prev != nil {
			prev(p, src, sa)
		}
	}
}

// elItem is one mirrored use at the log position it stands for.
type elItem struct {
	at int
	u  dirac.ManaUse
}

// elSpan is one drain's scan of the log window: the uses found so far, and
// the position of the scan relative to the recorded fires.
type elSpan struct {
	items      []elItem
	cand       []state.ObjID          // permanents tapped as a cost since the open fire's hook
	refs       []Ref                  // each fire's source, as the canon Ref (RefNone: not a permanent)
	cur        int                    // the fire whose resolution is open, or -1
	lastPos    int                    // the log position of the last positive mana
	pushed     bool                   // the play's PutOnStack was seen inside an open fire
	spent      [2][state.MC + 1]int32 // restricted units the final cost payment spent, by seat and column
	unmirrored bool                   // the span is left to the kernel's own re-derivation (below)
}

// DrainElided returns the elided mana uses recorded since the last drain and
// clears the record, in log order. A fire's tap and the positive ManaAdd
// events its resolution produced are the activation's footprint. The plan's
// machinery spends between activations -- a face-down land's flip cost, a
// filter's conversion -- and those spends (the span's negative ManaAdd
// events, before the play's PutOnStack) are mirrored too: the kernel's
// re-derivation, which skips the plan's sub-cost steps once the mirrored
// position prices the play, would otherwise leave the spend unpaid. After the
// push nothing is mirrored: the cost payment's spends are the kernel's own
// (its payFromPool prices from the pool the mirrored floats fill).
func (g *GorgeSide) DrainElided() []dirac.ManaUse {
	ev := g.E.L.Events
	if len(g.E.G.Players) != 2 {
		// the mirror's seat mapping and trigger groups are two-seat (the
		// kernel's multi-player pods keep their own re-derivation)
		g.elReset(len(ev))
		return nil
	}
	sc := g.elScanEvents(ev)
	if sc.unmirrored {
		// A stack object resolved inside the span: gorge's script passed
		// priority between the activations (a tap-another source's Taps
		// trigger resolves before the next activation, the mana still
		// floating). Those passes are no canonical decision, and the kernel's
		// own re-derivation at the play's Apply replays them (tapXPlay), so
		// the span is not mirrored: applying the activations out of band would
		// leave the trigger on the kernel's stack under the play. A span that
		// floated restricted mana an activated ability's cost may spend, or
		// asked a replacement order of a sacrificed source, is left to the
		// kernel too: its payment prices those itself.
		g.elReset(len(ev))
		return nil
	}
	sc.trimTrailingSpends()
	sc.plainSpentRestricted()
	g.mergeColourAsks()
	g.elScan = len(ev)
	g.id.sync(g.E.G)
	// The taps, in log order (gorge's Tap events; a fire's position stands
	// for its tap, which lands between its hook and its mana), then the mana
	// units and the plan's sub-cost spends, each at its own event.
	items := g.elFireItems(ev, sc.items)
	sort.SliceStable(items, func(i, j int) bool { return items[i].at < items[j].at })
	var uses []dirac.ManaUse
	for _, it := range items {
		uses = append(uses, it.u)
	}
	g.elFires = g.elFires[:0]
	return uses
}

// elReset drops the record and the scan position.
func (g *GorgeSide) elReset(to int) {
	g.elScan = to
	g.elFires = g.elFires[:0]
}

// elScanEvents pairs each fire and each event of the window: a fire opens at
// its hook position, an event joins the fire whose resolution was open.
func (g *GorgeSide) elScanEvents(ev []events.Event) *elSpan {
	sc := &elSpan{cur: -1, lastPos: -1}
	g.id.sync(g.E.G)
	sc.refs = make([]Ref, len(g.elFires))
	for i := range g.elFires {
		sc.refs[i] = RefNone
		if ref := g.id.ref(g.E.G, g.elFires[i].src); ref < RefStack && ref != RefNone {
			sc.refs[i] = ref
		}
	}
	fi := 0
	for i := g.elScan; i < len(ev); i++ {
		for fi < len(g.elFires) && g.elFires[fi].at <= i {
			sc.cur = fi
			fi++
			sc.cand = sc.cand[:0]
		}
		e := &ev[i]
		if e.Kind == events.Resolve {
			g.elRes = e.Obj
		} else if e.Kind == events.MoveZone && g.elRes != 0 && e.Obj == g.elRes {
			g.elRes = 0
		}
		if e.Kind == events.SpellCastReversed {
			// A cast gorge reversed (CR 733.1) kept its payment's taps and
			// floats; the kernel's own reversal keeps its own the same way.
			sc.unmirrored = true
		}
		if e.Kind == events.GrantTriggerPush && g.elFireSrc(e.Obj) {
			// A mana source's "when you spend this mana" trigger (Gilanra),
			// granted to the cast it paid. The kernel records that rider as
			// its own payment taps the source, so the span is left to it.
			sc.unmirrored = true
		}
		if sc.cur >= 0 {
			g.elEvent(sc, i, e)
		}
	}
	return sc
}

// elFireSrc reports whether src is the source of a recorded fire.
func (g *GorgeSide) elFireSrc(src state.ObjID) bool {
	for i := range g.elFires {
		if g.elFires[i].src == src {
			return true
		}
	}
	return false
}

// elEvent classifies one event of the window inside the open fire.
func (g *GorgeSide) elEvent(sc *elSpan, i int, e *events.Event) {
	f := &g.elFires[sc.cur]
	switch e.Kind {
	case events.PutOnStack:
		sc.pushed = true
	case events.MoveZone:
		g.elSacrifice(sc, f, i, e)
	case events.Resolve:
		sc.unmirrored = true
	case events.DecisionAsk:
		if e.Text == "replacement" {
			// a replacement order asked of a sacrificed source: the kernel's
			// own payment asks it in its lowering, in its own order
			sc.unmirrored = true
		}
	case events.DamageProvenance:
		g.elPain(sc, f, i, e)
	case events.Tap:
		g.elTap(sc, f, e)
	case events.CounterChange:
		if e.Obj == f.src && e.Amount != 0 {
			f.ctr = true
			f.ctrN += e.Amount
		}
	case events.ManaAdd:
		g.elManaAdd(sc, f, i, e)
	case events.LifeChange:
		g.elLife(sc, f, i, e)
	}
}

// elSacrifice records a sacrifice inside the open fire: the source's own (its
// cost), or another permanent the activation's cost paid before its mana
// (Krark-Clan Ironworks' "Sacrifice an artifact"); one after it is the
// play's own cost. A replacement may send the permanent elsewhere than the
// graveyard (an exile instead), and it was sacrificed all the same.
func (g *GorgeSide) elSacrifice(sc *elSpan, f *elFire, i int, e *events.Event) {
	if e.Text != "sacrificed" {
		return
	}
	if e.Obj == f.src {
		f.sac = true
		return
	}
	if f.found {
		return
	}
	if ref := g.id.ref(g.E.G, e.Obj); ref < RefStack && ref != RefNone {
		sc.items = append(sc.items, elItem{i, dirac.ManaUse{Obj: dirac.Obj(ref), Seat: uint8(f.p), Sac: true,
			Cease: g.ceased(e.Obj), Late: g.afterPush(i), Sba: true}})
	}
}

// elPain records the source's own pain rider (a painland's "deals 1 damage to
// you"): the damage event it follows, dealt to its controller.
func (g *GorgeSide) elPain(sc *elSpan, f *elFire, i int, e *events.Event) {
	ev := g.E.L.Events
	if e.Amount > 0 && e.Obj == f.src && i > 0 && ev[i-1].Kind == events.Damage && ev[i-1].Player == f.p {
		if ref := sc.refs[sc.cur]; ref != RefNone {
			sc.items = append(sc.items, elItem{i, dirac.ManaUse{Obj: dirac.Obj(ref), Seat: uint8(f.p), Pain: e.Amount}})
		}
	}
}

// elTap records a Tap inside the open fire: the source's own commits the cost
// taps seen before it; a permanent tapped as a cost is the activation's own
// when the source's tap follows it (tap-another-permanent mana pays it before
// its {T}), the play's (convoke) when it comes after the fires.
func (g *GorgeSide) elTap(sc *elSpan, f *elFire, e *events.Event) {
	switch {
	case e.Obj == f.src:
		g.commitCostTaps(sc.cur, &sc.cand)
	case !sc.pushed && e.Text == "tapped as a cost":
		if len(sc.cand) < 2 {
			sc.cand = append(sc.cand, e.Obj)
		}
	}
}

// elManaAdd records one ManaAdd of the open fire: a float, or a spend the
// plan's machinery made.
func (g *GorgeSide) elManaAdd(sc *elSpan, f *elFire, i int, e *events.Event) {
	if e.Amount > 0 {
		g.commitCostTaps(sc.cur, &sc.cand) // a source without its own {T}: the cost taps precede the mana
	}
	// Restricted mana (Carnelian Orb of Dragonkind's Dragon-only {R} that
	// gives the spell haste) floats restricted on the kernel side (a
	// restricted source unit spent with its rider, not plain pool mana); a
	// spend of it is taken from the batches.
	restr := strings.Contains(e.Text, "mana-restriction")
	if restr && strings.Contains(e.Text, "Activated") {
		// restricted mana an activated ability's cost may spend: the kernel's
		// own payment prices that spendability (and the activation's source)
		// itself
		sc.unmirrored = true
	}
	col := manaUseCol(e.Counter)
	if col > state.MC {
		return
	}
	// A spend the plan's machinery made is mirrored only before the play's
	// push: after it the cost payment's spends are the kernel's own.
	if e.Amount < 0 && sc.pushed {
		if restr {
			sc.spentR(e.Player, col, -e.Amount)
		}
		return
	}
	if e.Amount > 0 {
		f.found = true
		sc.lastPos = i
	}
	f.last = i
	sc.items = append(sc.items, elItem{i, dirac.ManaUse{
		Obj: dirac.Obj(sc.refs[sc.cur]), Seat: uint8(e.Player), Col: uint8(col), N: e.Amount, Restr: restr}})
}

// elLife records the activation's own life change: a cost's payment (Myr
// Convert's "{T}, Pay 2 life: Add one mana of any color") or a rider's gain,
// both the fire's resolution moved. A payment after the play's push is the
// cost payment's own -- the kernel's re-derivation pays it. (A span in which
// a stack object resolved is not mirrored at all: see elSpan.unmirrored.)
func (g *GorgeSide) elLife(sc *elSpan, f *elFire, i int, e *events.Event) {
	if e.Player != f.p {
		// the activation's rider on another player (Grove of the Burnwillows'
		// "each opponent gains 1 life"): the gain that follows the fire's mana
		ev := g.E.L.Events
		if e.Amount > 0 && i > 0 && ev[i-1].Kind == events.ManaAdd && ev[i-1].Amount > 0 && g.elRes == 0 {
			sc.items = append(sc.items, elItem{i, dirac.ManaUse{
				Obj: dirac.Obj(sc.refs[sc.cur]), Seat: uint8(e.Player), Life: e.Amount, Rider: true}})
		}
		return
	}
	if e.Amount < 0 && sc.pushed && f.found {
		return // after the fire's mana and the push: the play's own cost payment
	}
	if g.elRes != 0 && f.found {
		// after the fire's mana, inside a resolving stack object: that
		// object's effect (a trigger paid with this mana, then gaining
		// life), which the kernel's own resolution applies
		return
	}
	sc.items = append(sc.items, elItem{i, dirac.ManaUse{
		Obj: dirac.Obj(sc.refs[sc.cur]), Seat: uint8(f.p), Life: e.Amount}})
}

// spentR notes restricted units the play's final cost payment spent.
func (sc *elSpan) spentR(p state.PlayerID, col int, n int32) {
	if int(p) < len(sc.spent) && col < len(sc.spent[p]) {
		sc.spent[p][col] += n
	}
}

// plainSpentRestricted floats plain the restricted units gorge's cost payment
// spent: the kernel's own table may price the restriction as not spendable on
// that play, while gorge's payment took the mana; a plain unit pays it the
// same way and leaves no restricted batch behind.
func (sc *elSpan) plainSpentRestricted() {
	for i := range sc.items {
		it := &sc.items[i]
		if !it.u.Restr || it.u.N <= 0 || int(it.u.Seat) >= len(sc.spent) || int(it.u.Col) >= len(sc.spent[it.u.Seat]) {
			continue
		}
		if n := &sc.spent[it.u.Seat][it.u.Col]; *n >= it.u.N {
			*n -= it.u.N
			it.u.Restr = false
		}
	}
}

// trimTrailingSpends drops the cost payment's spends that trail the last
// float: a plan's sub-cost (a flip, a filter conversion) is followed by more
// activations, the final cost payment is not, and the kernel pays the cost
// itself from the pool the mirrored floats fill. A life payment in that
// position is the final cost payment's own for the same reason (a rider's
// gain keeps its amount: a cost never gains life).
func (sc *elSpan) trimTrailingSpends() {
	for i := range sc.items {
		it := &sc.items[i]
		if it.u.N < 0 && it.at > sc.lastPos {
			if it.u.Restr {
				sc.spentR(state.PlayerID(it.u.Seat), int(it.u.Col), -it.u.N)
			}
			it.u.Trimmed = -it.u.N
			it.u.N = 0 // dropped: the kernel's own cost payment
		}
		if it.u.Life < 0 && it.at > sc.lastPos {
			it.u.Life = 0
		}
	}
}

// mergeColourAsks folds the colour ask's continuation into the paused fire. A
// colour ask pauses the resolution after the first fire: the fire carries no
// mana, the answer's continuation re-fires the hook for the same ability and
// its ManaAdd pairs there. The earlier record is the one kept (its tap is the
// activation's own: a paused fire that in fact produced nothing still taps
// its source), the continuation's goes, so the activation is tapped once.
func (g *GorgeSide) mergeColourAsks() {
	for i := 0; i < len(g.elFires); i++ {
		f := &g.elFires[i]
		if f.found || i+1 >= len(g.elFires) {
			continue
		}
		n := &g.elFires[i+1]
		if n.p == f.p && n.src == f.src && n.found {
			f.found = true
			g.elFires = append(g.elFires[:i+1], g.elFires[i+2:]...)
			i--
		}
	}
}

// elFireItems appends, for each recorded fire whose source is a permanent,
// its counter cost, its tap (with the other permanents its cost tapped) and
// its sacrifice, each at the log position it stands for.
func (g *GorgeSide) elFireItems(ev []events.Event, items []elItem) []elItem {
	for i := range g.elFires {
		f := &g.elFires[i]
		ref := g.id.ref(g.E.G, f.src)
		if ref >= RefStack || ref == RefNone {
			continue
		}
		end := len(ev)
		if i+1 < len(g.elFires) {
			end = g.elFires[i+1].at
		}
		// A mana ability the engine resolved without tapping its source
		// (Forsaken Monument's "add an additional {C}" trigger, resolved as a
		// mana ability of the Monument) taps nothing.
		if f.ctr {
			items = append(items, elItem{f.at, dirac.ManaUse{Obj: dirac.Obj(ref), Seat: uint8(f.p), Ctr: true, CtrN: f.ctrN}})
		}
		if tapped := g.tappedIn(f.src, f.at, end); tapped || f.noth > 0 {
			items = append(items, elItem{f.at, g.elTapUse(f, ref, tapped)})
		} else if f.ab >= 0 && f.ab < 255 {
			// an activation that taps nothing (a "{1}: Add ..." filter): its
			// announcement and its once-each-turn mark are still the kernel's
			// to see
			items = append(items, elItem{f.at, dirac.ManaUse{Obj: dirac.Obj(ref), Seat: uint8(f.p), Ab: uint8(f.ab + 1), Late: g.afterPush(f.at)}})
		}
		if f.sac {
			// the cost's sacrifice follows the activation's mana in the
			// kernel's own order (the float, then fireSac and the move)
			at := f.at
			if f.last > at {
				at = f.last
			}
			items = append(items, elItem{at, dirac.ManaUse{Obj: dirac.Obj(ref), Seat: uint8(f.p), Sac: true,
				Cease: g.ceased(f.src), Late: g.afterPush(f.at), Sba: f.granted}})
		}
	}
	return items
}

// elTapUse is the fire's tap use: the source's own tap and the other
// permanents its cost tapped.
func (g *GorgeSide) elTapUse(f *elFire, ref Ref, tapped bool) dirac.ManaUse {
	u := dirac.ManaUse{Obj: dirac.Obj(ref), Seat: uint8(f.p), Tap: tapped, Late: g.afterPush(f.at)}
	if f.ab >= 0 && f.ab < 255 {
		u.Ab = uint8(f.ab + 1)
	}
	for _, o := range f.oth[:f.noth] {
		if r := g.id.ref(g.E.G, o); r < RefStack && r != RefNone {
			u.Others[u.NOthers] = dirac.Obj(r)
			u.NOthers++
		}
	}
	return u
}

// ceased reports whether gorge's state-based check has ceased the token o.
func (g *GorgeSide) ceased(o state.ObjID) bool {
	ob := g.E.G.Obj(o)
	return ob != nil && ob.Zone == state.ZCeased
}

// commitCostTaps attaches the permanents tapped as a cost since fire cur's hook
// to its activation (at most two), once: what follows the activation's own tap
// or mana is the play's (convoke).
func (g *GorgeSide) commitCostTaps(cur int, cand *[]state.ObjID) {
	f := &g.elFires[cur]
	for _, o := range *cand {
		if f.noth < 2 {
			f.oth[f.noth] = o
			f.noth++
		}
	}
	*cand = (*cand)[:0]
}

// tappedIn reports whether the log holds a Tap of src in [from, to).
func (g *GorgeSide) tappedIn(src state.ObjID, from, to int) bool {
	ev := g.E.L.Events
	for i := from; i < to && i < len(ev); i++ {
		if ev[i].Kind == events.Tap && ev[i].Obj == src {
			return true
		}
	}
	return false
}

// afterPush reports whether the mana ability whose hook fired at log position
// at ran after its play's push (a spell's PutOnStack, an ability's
// AbilityPush): the nearest earlier push comes before
// any priority answer (a play's submit) or resolution. Gorge's plan
// activations run before the push (cast_begin.go beginCastWithPayment); the
// cost payment's own follow it.
func (g *GorgeSide) afterPush(at int) bool {
	ev := g.E.L.Events
	for i := at - 1; i >= 0; i-- {
		switch e := &ev[i]; {
		case e.Kind == events.PutOnStack, e.Kind == events.AbilityPush:
			return true
		case e.Kind == events.Resolve, e.Kind == events.DecisionMade && strings.HasPrefix(e.Text, "priority:"):
			return false
		}
	}
	return false
}

// manaUseCol is the pool slot of a ManaAdd event's counter (engine/events
// manaAddSlot's order, over the exported forms).
func manaUseCol(counter string) int {
	if len(counter) == 2 && counter[0] == 'S' {
		return state.ManaIndex(counter[1])
	}
	if _, slot, ok := state.TypedManaCounter(counter); ok {
		return slot
	}
	if slot, ok := state.SnowTypedCounter(counter); ok {
		return slot
	}
	if counter != "" {
		return state.ManaIndex(counter[0])
	}
	return state.MC
}

// manaActivationPrimary reports whether the offered option at opt is a
// canonical activation of a mana ability (a granted AB$ Mana line's
// max-speed activation, or a printed mana ability's "ability" option): the
// engine resolves that primary itself at the answer's submit, and the kernel
// resolves the same canonical action itself, so its fire is not recorded --
// recording it would tap the source a second time. The plan-backed and
// cost-payment activations beside it are elided and mirrored.
func (g *GorgeSide) manaActivationPrimary(d *decision.Decision, opt int) bool {
	o := &d.Options[opt]
	switch o.Kind {
	case "granted":
		return strings.HasPrefix(o.SVar, "AB$ Mana")
	case "ability":
		obj := g.E.G.Obj(o.Obj)
		if obj == nil || o.Ability < 0 {
			return false
		}
		f := obj.Face()
		if f == nil || int(o.Ability) >= len(f.Abilities) {
			return false
		}
		return cards.IsManaAbilitySA(f.Abilities[o.Ability])
	}
	return false
}
