package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// The Defined$ set grammar (gorge engine/effects/context.go knownDefinedTargets
// and definedSpec) lowered onto kop.SetExpr. A value defined() already lowers
// never reaches this file: the wrapper in lower_effect.go tries it only at a
// refusal arm. The parser follows gorge's order:
//
//	Remembered.<filter>          SoBaseRef(RefRemembered) SoInter(F)
//	Targeted.<filter>            SoBaseRef(targets) SoInter(F)
//	<TriggeredCard>.<filter>     SoBaseRef(base) SoInter(F)
//	X & Y & ...                  SoUnion over each part, in script order
//	Non<ref>                     SoComplementPlayers
//	<ref>Controller/<ref>Owner   SoController / SoOwner
//	<ref>LKICopy                 SoLKI
//	Valid <filter>               SoBaseValid(Battlefield)
//	Valid<Zone> <filter>         SoBaseValid(zone)
//	<bare base>                  SoBaseRef(that ref)
//
// Attach bases (T08) and Spawner> (T08's SoSpawner) refuse with a guard class,
// as do the bases the kernel has no Ref for (ExiledWith/ReplacedCards).

// setOpKind is an lSet's op, one per kop.SetOpKind.
type setOpKind uint8

const (
	soBaseRef setOpKind = iota + 1
	soBaseValid
	soInter
	soUnion
	soController
	soOwner
	soComplementPlayers
	soLKI
)

// lSetOp is one op of an LIR set program's postfix form.
type lSetOp struct {
	Kind setOpKind
	R    lRef     // SoBaseRef
	Pl   bool     // SoBaseRef names players
	F    *lFilter // SoBaseValid and SoInter
	Zone int      // SoBaseValid: kop.Zone
}

// lSet is one SetExpr before kop emission. It is interned into Table.Sets at
// build time and dumped as a SET line so a card's LIR key covers the program.
type lSet struct {
	Ops     []lSetOp
	Players bool
	Order   kop.SetOrder
	Flags   uint8
	// MoveGate: the node's Origin$ gates a repeated move (setMoveOriginGate).
	// Lowering-time only: it is not dumped, the node's own params carry it.
	MoveGate bool
}

// String is the deterministic SET-line form.
func (s *lSet) String() string {
	var b strings.Builder
	b.WriteString("pl=")
	if s.Players {
		b.WriteByte('1')
	} else {
		b.WriteByte('0')
	}
	b.WriteString(" order=")
	b.WriteString(itoa(int(s.Order)))
	b.WriteString(" flags=")
	b.WriteString(itoa(int(s.Flags)))
	for _, op := range s.Ops {
		b.WriteByte(' ')
		switch op.Kind {
		case soBaseRef:
			b.WriteString("BaseRef(")
			b.WriteString(op.R.String())
			if op.Pl {
				b.WriteString(",pl")
			}
			b.WriteByte(')')
		case soBaseValid:
			b.WriteString("BaseValid(")
			b.WriteString(itoa(op.Zone))
			b.WriteByte(',')
			b.WriteString(op.F.String())
			b.WriteByte(')')
		case soInter:
			b.WriteString("Inter(")
			b.WriteString(op.F.String())
			b.WriteByte(')')
		case soUnion:
			b.WriteString("Union")
		case soController:
			b.WriteString("Controller")
		case soOwner:
			b.WriteString("Owner")
		case soComplementPlayers:
			b.WriteString("ComplementPlayers")
		case soLKI:
			b.WriteString("LKI")
		}
	}
	return b.String()
}

// itoa is a tiny base-10 formatter for the dump (no fmt allocation ordering
// concerns; the values are small non-negative ints).
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [12]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}

// definedSetAliases are gorge's definedAliases (context.go): exact
// compositions of modelled selectors, read only after the exact table misses.
var definedSetAliases = map[string]string{
	"TargetedOrController": "TargetedPlayer & TargetedController",
	"TargetedAndYou":       "TargetedPlayer & You",
	"DirectRemembered":     "RememberedLKI",
}

// definedSetQualifiedBases are the spellings gorge's triggeredCardQualified
// takes a dotted qualifier over (context.go:3405).
var definedSetQualifiedBases = map[string]bool{
	"TriggeredCard":            true,
	"TriggeredCardLKICopy":     true,
	"TriggeredAttacker":        true,
	"TriggeredAttackerLKICopy": true,
	"Self":                     true,
	"Imprinted":                true,
	"ParentTarget":             true,
	"ParentTargeted":           true,
	"Exiled":                   true,
}

// setPlayerAPIs are the effect APIs whose Defined$ subject is a players set:
// an ambiguous target ref then reads as players (kop.SetExpr.Players).
var setPlayerAPIs = map[string]bool{
	"Draw":     true,
	"GainLife": true,
	"LoseLife": true,
	"Discard":  true,
}

// setOps is kgen's mirror of the kernel's wired RefSet list (dirac/
// value_setops.go execSet). An LIR op not here refuses with
// other:Defined$ set on <Op>, a guard counted under M3.
var setOps = map[string]bool{
	"PutCounter":   true,
	"PumpUntilEOT": true,
	"Destroy":      true,
	"Tap":          true,
	"Untap":        true,
	"Exile":        true,
	"Bounce":       true,
	"ToLibrary":    true,
	"GainControl":  true,
	"GainLife":     true,
	"LoseLife":     true,
	"Draw":         true,
	"Discard":      true,
}

// setOpPlayers is the kind of SetExpr each wired op reads, mirroring the
// kernel's execSet split (execSetPlayers versus the object-op arms).
var setOpPlayers = map[string]bool{
	"Draw":     true,
	"GainLife": true,
	"LoseLife": true,
	"Discard":  true,
}

// setSuspOps are the RefSet ops the kernel marks suspension-capable (dirac/
// value_setiter.go setSuspOp): a member body can park a decision, so an
// OrderList set cannot be replayed from membership bits.
var setSuspOps = map[string]bool{
	"PutCounter": true,
	"Destroy":    true,
	"Exile":      true,
	"Bounce":     true,
	"ToLibrary":  true,
}

// setUnionOps are the object ops a SoUnion set may feed. The kernel's union
// de-duplicates a member two parts both name; gorge's plain " & " path
// (context.go knownDefinedTargets) concatenates the parts, so an object named
// twice is acted on twice. These ops do nothing the second time (the object
// is already tapped or controlled). PutCounter, PumpUntilEOT and Destroy (a
// regeneration shield the first pass used is gone for the second) differ, so a
// union on them refuses; so does Untap (gorge's TryUntap spends a stun counter
// on the first pass and untaps on the second). A players union is admitted on
// every players op: gorge's consumers read it through playerIDsFromTargets,
// which de-duplicates like the kernel.
var setUnionOps = map[string]bool{
	"Tap":         true,
	"GainControl": true,
}

// setUnionMoveOps are the zone-move ops a union may feed only when the node's
// Origin$ gates the second move (setMoveOriginGate): gorge's mover
// (changeZoneMoveSkip) skips an already-moved object only when Origin$ is
// present and excludes the destination, and object ids persist across zones,
// so without it a twice-named object is moved twice.
var setUnionMoveOps = map[string]bool{
	"Exile":     true,
	"Bounce":    true,
	"ToLibrary": true,
}

// setMoveOriginGate reports whether the node is a ChangeZone whose Origin$
// excludes its Destination$, so an object moved once fails the Origin$ gate
// the second time.
func setMoveOriginGate(sa *cards.SA, ps *params) bool {
	if sa.API != "ChangeZone" {
		return false
	}
	origin, dest := strings.TrimSpace(ps.m["Origin"]), strings.TrimSpace(ps.m["Destination"])
	if origin == "" || dest == "" {
		return false
	}
	for _, o := range strings.Split(origin, ",") {
		o = strings.TrimSpace(o)
		if o == "" || strings.EqualFold(o, "Any") || strings.EqualFold(o, dest) {
			return false
		}
	}
	return true
}

// enterSetNode records the node facts the set builder reads (the effect's
// subject, and whether Origin$ gates a repeated move) and returns the restore.
func (lc *lowerCtx) enterSetNode(sa *cards.SA, ps *params) func() {
	players, gate := lc.setPlayers, lc.setMoveGate
	lc.setPlayers, lc.setMoveGate = setPlayerAPIs[sa.API], setMoveOriginGate(sa, ps)
	return func() { lc.setPlayers, lc.setMoveGate = players, gate }
}

// hasUnion reports whether the set program joins parts with SoUnion.
func (s *lSet) hasUnion() bool {
	for _, op := range s.Ops {
		if op.Kind == soUnion {
			return true
		}
	}
	return false
}

// setObj lowers an instruction's rSet Obj to kop.RefSet and its SetRef. pc is
// the instruction's index in the ability's program.
func (cb *cardBuilder) setObj(ki *kop.Instr, pc int, in lInstr) (*kop.SetRef, error) {
	if pc > 255 {
		return nil, refuse("Defined$ set at instruction %d", pc)
	}
	if !setOps[in.Op] {
		return nil, refuse("Defined$ set on %s", in.Op)
	}
	s := in.Obj.Set
	if s == nil || len(s.Ops) == 0 {
		return nil, refuse("Defined$ set on %s: no set", in.Op)
	}
	if s.Players != setOpPlayers[in.Op] {
		return nil, refuse("Defined$ set shape on %s", in.Op)
	}
	if !s.Players && s.hasUnion() && !setUnionOps[in.Op] && !(setUnionMoveOps[in.Op] && s.MoveGate) {
		return nil, refuse("Defined$ set union on %s", in.Op)
	}
	if s.Order == kop.OrderList && setSuspOps[in.Op] {
		return nil, refuse("Defined$ ordered set on a suspending op %s", in.Op)
	}
	id, err := cb.internSet(s)
	if err != nil {
		return nil, err
	}
	ki.Obj = kop.RefSet
	return &kop.SetRef{PC: uint8(pc), Set: id, Flags: s.Flags}, nil
}

// setKopShape refuses a RefSet instruction whose emitted kop op the kernel's
// execSet does not serve (dirac/value_setops.go): the LIR op name is not enough,
// since one LIR op lowers to several kop ops (an Exile that remembers is
// OpExileEx, which execSet panics on) and some wired ops serve only one shape
// (the others stall with StallSetShape).
func setKopShape(ki *kop.Instr) error {
	switch ki.Op {
	case kop.OpPutCounter, kop.OpPumpUntilEOT, kop.OpTap, kop.OpUntap, kop.OpDestroy,
		kop.OpExile, kop.OpGainControl, kop.OpGainLife, kop.OpLoseLife, kop.OpDraw:
		return nil
	case kop.OpBounce:
		if ki.C == 0 {
			return nil
		}
	case kop.OpToLibrary:
		if ki.C == 0 || ki.C == 1 {
			return nil
		}
	case kop.OpDiscard:
		if ki.F == 0 && ki.C == kop.ChooseSelf {
			return nil
		}
	}
	return refuse("Defined$ set on kop %s shape (C=%d F=%d)", ki.Op, ki.C, ki.F)
}

// internSet emits an LIR set to its kop.SetExpr and interns it in the pool.
func (cb *cardBuilder) internSet(s *lSet) (kop.SetID, error) {
	ex, err := cb.setExpr(s)
	if err != nil {
		return 0, err
	}
	return cb.pt.internSet(ex), nil
}

// setExpr maps an LIR set to its kop form.
func (cb *cardBuilder) setExpr(s *lSet) (kop.SetExpr, error) {
	out := kop.SetExpr{Players: s.Players, Order: kop.SetOrder(s.Order)}
	for _, op := range s.Ops {
		ko := kop.SetOp{}
		var err error
		switch op.Kind {
		case soBaseRef:
			ko.Kind = kop.SoBaseRef
			if op.Pl {
				ko.Arg = 1
			}
			if ko.R, err = cb.ref(op.R); err != nil {
				return out, err
			}
		case soBaseValid:
			ko.Kind = kop.SoBaseValid
			ko.Arg = int16(op.Zone)
			if ko.F, err = cb.filter(op.F, kop.Zone(op.Zone)); err != nil {
				return out, err
			}
		case soInter:
			ko.Kind = kop.SoInter
			if ko.F, err = cb.filter(op.F, kop.ZoneAny); err != nil {
				return out, err
			}
		case soUnion:
			ko.Kind = kop.SoUnion
		case soController:
			ko.Kind = kop.SoController
		case soOwner:
			ko.Kind = kop.SoOwner
		case soComplementPlayers:
			ko.Kind = kop.SoComplementPlayers
		case soLKI:
			ko.Kind = kop.SoLKI
		default:
			return out, refuse("Defined$ set op %d", op.Kind)
		}
		out.Ops = append(out.Ops, ko)
	}
	return out, nil
}

// definedSet lowers v onto an lSet. inSetParse suppresses the wrapper's
// fallback while the parser calls defined() for a base, so the two can never
// recurse.
func (lc *lowerCtx) definedSet(v string) (*lSet, error) {
	save := lc.inSetParse
	lc.inSetParse = true
	defer func() { lc.inSetParse = save }()
	s, err := lc.definedSetValue(strings.TrimSpace(v))
	if err != nil {
		return nil, err
	}
	if setDepth(s.Ops) > kop.SetMaxDepth {
		return nil, refuse("Defined$ %q: a set deeper than %d", v, kop.SetMaxDepth)
	}
	return s, nil
}

// setDepth is the deepest stack a postfix set program reaches.
func setDepth(ops []lSetOp) int {
	d, max := 0, 0
	for _, op := range ops {
		switch op.Kind {
		case soUnion:
			d--
		case soInter, soController, soOwner, soComplementPlayers, soLKI:
			// pop one, push one
		default:
			d++
			if d > max {
				max = d
			}
		}
	}
	return max
}

// definedSetValue parses one value in gorge's order.
func (lc *lowerCtx) definedSetValue(v string) (*lSet, error) {
	if v == "" {
		return nil, refuse("Defined$ \"\"")
	}
	// gorge's prefix selectors run before the " & " split (the prefix grammar
	// owns the whole tail, a filter or a qualifier).
	if q, ok := strings.CutPrefix(v, "Remembered."); ok {
		base := &lSet{Ops: []lSetOp{{Kind: soBaseRef, R: lRef{Kind: rRemembered}}}, Order: kop.OrderList}
		return lc.setInter(v, base, q)
	}
	if q, ok := strings.CutPrefix(v, "Targeted."); ok {
		base, err := lc.targetSetBase()
		if err != nil {
			return nil, err
		}
		return lc.setInter(v, base, q)
	}
	if q, ok := strings.CutPrefix(v, "ExiledWith."); ok {
		return nil, refuse("Defined$ %q: no base ref for ExiledWith", "ExiledWith."+q)
	}
	if q, ok := strings.CutPrefix(v, "ReplacedCards."); ok {
		return nil, refuse("Defined$ %q: no base ref for ReplacedCards", "ReplacedCards."+q)
	}
	if base, q, ok := strings.Cut(v, "."); ok && definedSetQualifiedBases[base] {
		b, err := lc.definedSetValue(base)
		if err != nil {
			return nil, err
		}
		return lc.setInter(v, b, q)
	}
	if strings.HasPrefix(v, "AttachedTo ") || strings.HasPrefix(v, "AttachedBy ") {
		return nil, refuse("Defined$ %q: attach base", v)
	}
	if strings.HasPrefix(v, "Spawner>") {
		return nil, refuse("Defined$ %q: spawner base", v)
	}
	if s, ok := lc.definedSetExact(v); ok {
		return s, nil
	}
	if alias, ok := definedSetAliases[v]; ok {
		return lc.definedSetValue(alias)
	}
	if !strings.Contains(v, " & ") {
		if s, ok := lc.definedSetValid(v); ok {
			return s, nil
		}
		if s, ok := lc.definedSetNon(v); ok {
			return s, nil
		}
		if s, ok := lc.definedSetSuffix(v); ok {
			return s, nil
		}
	}
	if strings.Contains(v, " & ") {
		return lc.definedSetUnion(v)
	}
	return nil, refuse("Defined$ %q", v)
}

// setInter appends SoInter to base with the dotted qualifier's Card filter.
func (lc *lowerCtx) setInter(v string, base *lSet, q string) (*lSet, error) {
	if base.Players {
		return nil, refuse("Defined$ %q: a qualifier over a players set", v)
	}
	f, err := lc.filter("Card." + strings.TrimSpace(q))
	if err != nil {
		return nil, err
	}
	base.Ops = append(base.Ops, lSetOp{Kind: soInter, F: f})
	return base, nil
}

// targetSetBase is SoBaseRef over the chain's one single-object target slot.
// Several slots or a multi-object slot refuse, as they did before the set
// grammar (no lockstep play covers the kernel's slot-member reads yet).
func (lc *lowerCtx) targetSetBase() (*lSet, error) {
	if len(lc.chainT) != 1 {
		return nil, refuse("Defined$ Targeted over %d target slots", len(lc.chainT))
	}
	t := lc.lastTarget()
	if t < 0 {
		return nil, refuse("Defined$ Targeted with no ancestor target slot")
	}
	if err := lc.singleObjectSetSlot(t); err != nil {
		return nil, err
	}
	return lc.setFromRef(lRef{Kind: rTarget, N: t}), nil
}

// singleObjectSetSlot refuses a target slot that can hold more than one object.
// The kernel keeps only a slot's first object in its target cell, so only a
// slot that can never hold more than one reads as one member; every route that
// turns a target ref into a set base (the dotted form and a bare union part)
// goes through it.
func (lc *lowerCtx) singleObjectSetSlot(t int) error {
	tg := lc.ab.Targets[t]
	if !tg.Min.isConst() || !tg.Max.isConst() || tg.Min.N > 1 || tg.Max.N > 1 {
		return refuse("Defined$ Targeted over a multi-object target slot")
	}
	return nil
}

// definedSetExact lowers a whole value through defined(), reusing every
// context guard the single-ref lowering already has.
func (lc *lowerCtx) definedSetExact(v string) (*lSet, bool) {
	r, err := lc.defined(v)
	if err != nil {
		return nil, false
	}
	switch r.Kind {
	case rTarget, rTargetController, rTargetOwner:
		if lc.singleObjectSetSlot(r.N) != nil {
			return nil, false
		}
	}
	s := lc.setFromRef(r)
	if !s.Players && strings.HasSuffix(v, "LKICopy") {
		// defined() folds the spelling into the same live ref (a LKICopy
		// attacker is the trigger object); the set records the read.
		s.Ops = append(s.Ops, lSetOp{Kind: soLKI})
		s.Flags |= kop.SrLKI
	}
	return s, true
}

// setFromRef wraps a base ref. A players ref orders by seat from the
// controller; every object ref keeps its own order.
func (lc *lowerCtx) setFromRef(r lRef) *lSet {
	players := refIsPlayer(r.Kind)
	if !players && (r.Kind == rTarget || r.Kind == rTargetUnion) {
		// a target may name a player: the effect's subject decides
		players = lc.setPlayers
	}
	s := &lSet{Players: players}
	if players {
		s.Order = kop.OrderSeat
	} else {
		s.Order = kop.OrderList
	}
	s.Ops = append(s.Ops, lSetOp{Kind: soBaseRef, R: r, Pl: players})
	return s
}

// refIsPlayer reports whether a ref kind names players.
func refIsPlayer(k refKind) bool {
	switch k {
	case rYou, rEachOpp, rEachPlayer, rTriggeredPlayer, rDefending, rPromised,
		rChosenPlayer, rChosenPlayerRes, rResChosenPlayer, rTriggerOpps,
		rTargetController, rTargetOwner, rRememberedController, rRememberedOwner,
		rTriggeredOwner, rOppNonRemCtrl, rSrcOwner, rAttachedCtrl:
		return true
	}
	return false
}

// definedSetValid lowers Valid/Valid<Zone> <filter> onto SoBaseValid.
func (lc *lowerCtx) definedSetValid(v string) (*lSet, bool) {
	zones := [...]struct {
		prefix string
		zone   kop.Zone
	}{
		{"ValidGraveyard ", kop.ZoneGraveyard},
		{"ValidHand ", kop.ZoneHand},
		{"ValidLibrary ", kop.ZoneLibrary},
		{"ValidExile ", kop.ZoneExile},
		{"ValidBattlefield ", kop.ZoneBattlefield},
	}
	for _, z := range zones {
		if q, ok := strings.CutPrefix(v, z.prefix); ok {
			return lc.validSet(v, q, z.zone)
		}
	}
	if q, ok := strings.CutPrefix(v, "Valid "); ok {
		return lc.validSet(v, q, kop.ZoneBattlefield)
	}
	return nil, false
}

func (lc *lowerCtx) validSet(v, q string, zone kop.Zone) (*lSet, bool) {
	f, err := lc.filter(strings.TrimSpace(q))
	if err != nil || f.Players != 0 || len(f.Clauses) == 0 {
		return nil, false
	}
	return &lSet{
		Ops:   []lSetOp{{Kind: soBaseValid, F: f, Zone: int(zone)}},
		Order: kop.OrderObj,
	}, true
}

// definedSetNon lowers forge's general Non<referent> player complement. It
// runs after the exact table, as gorge's nonComplementTargets does.
func (lc *lowerCtx) definedSetNon(v string) (*lSet, bool) {
	rest, ok := strings.CutPrefix(v, "Non")
	if !ok || rest == "" {
		return nil, false
	}
	b, err := lc.definedSetValue(rest)
	if err != nil || !b.Players {
		return nil, false
	}
	b.Ops = append(b.Ops, lSetOp{Kind: soComplementPlayers})
	b.Order = kop.OrderSeat
	return b, true
}

// definedSetSuffix lowers <ref>Controller, <ref>Owner and <ref>LKICopy over a
// base this grammar can parse.
func (lc *lowerCtx) definedSetSuffix(v string) (*lSet, bool) {
	for _, suf := range [...]struct {
		name string
		kind setOpKind
	}{
		{"Controller", soController},
		{"Owner", soOwner},
		{"LKICopy", soLKI},
	} {
		if len(v) <= len(suf.name) || !strings.HasSuffix(v, suf.name) {
			continue
		}
		// A projection reads an object set: an ambiguous target base here is
		// the object the target names, never a player.
		savePlayers := lc.setPlayers
		lc.setPlayers = false
		b, err := lc.definedSetValue(strings.TrimSuffix(v, suf.name))
		lc.setPlayers = savePlayers
		if err != nil {
			continue
		}
		switch suf.kind {
		case soController, soOwner:
			if b.Players {
				continue
			}
			b.Ops = append(b.Ops, lSetOp{Kind: suf.kind})
			b.Players = true
			b.Order = kop.OrderSeat
		case soLKI:
			if b.Players {
				continue
			}
			b.Ops = append(b.Ops, lSetOp{Kind: soLKI})
			b.Flags |= kop.SrLKI
		}
		return b, true
	}
	return nil, false
}

// definedSetUnion lowers "A & B & ..." as SoUnion over each part, each part
// parsed recursively; the parts join in script order (gorge knownDefinedTargets).
func (lc *lowerCtx) definedSetUnion(v string) (*lSet, error) {
	parts := strings.Split(v, " & ")
	out := &lSet{Order: kop.OrderList, MoveGate: lc.setMoveGate}
	for i, p := range parts {
		s, err := lc.definedSetValue(strings.TrimSpace(p))
		if err != nil {
			return nil, err
		}
		if i == 0 {
			out.Players = s.Players
		} else if s.Players != out.Players {
			return nil, refuse("Defined$ %q: union of players and objects", v)
		}
		out.Ops = append(out.Ops, s.Ops...)
		out.Flags |= s.Flags
		if i > 0 {
			out.Ops = append(out.Ops, lSetOp{Kind: soUnion})
		}
	}
	return out, nil
}
