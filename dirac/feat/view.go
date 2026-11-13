package feat

import (
	"math/bits"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// view.go builds the kernel-side analogue of gorge's view.View for one
// seat: the cards policynet reads, in the zone-list orders the view lists
// them, from the kernel's Projection plus its feature accessors.

// Option zone classes (policynet's zoneBattlefield..zoneExile); zStack is
// the stack (policynet's -1 class).
const (
	zBattlefield = 0
	zHand        = 1
	zGraveyard   = 2
	zExile       = 3
	zStack       = 4
)

// noObj is the encoder's no-object byte. The encoders hold objects in a byte
// (policynet's two-seat views). Under dirac_mp (MaxObj past 255) an object
// at noObj or past it has no encoding: a declared limit, counted per build
// (ObjOverflow), never a silent truncation.
const noObj = uint8(dirac.NoObj & 0xff)

// ObjOverflow counts the encodes (full and incremental) whose position held
// an object the encoders cannot name: an index at noObj or past it, which
// only the dirac_mp size class reaches. Such an encode names those objects
// wrongly, so a caller that compares or learns from its output skips it.
// The default class never counts one (MaxObj is under noObj there).
func (e *Encoder) ObjOverflow() int { return e.objOverflow }

// vcard is one visible card (policynet's view.CardView) as one encode
// lists it: its content (vcore, shared from the content cache, never
// written through) and the facts the encode sets on top -- the zone class
// it is listed in (a stack band shows a permanent as on the stack), the
// attacking and blocking bits a declaration in progress clears, and
// whether it is blocked.
type vcard struct {
	*vcore
	obj       uint8
	zone      uint8
	attacking bool
	blocked   bool
	blocks    uint8 // a blocker: the attacker it blocks (noObj none)
	rel       entRel
}

// kwReps is a card view's repeated keyword grants (vcore.kwRep), kept
// beside its cache entry: they are rare, and a view stays small.
type kwReps = dirac.FeatKwReps

// vcore is a card view's content: a pure function of its content cache key.
type vcore struct {
	cid       kop.CardID
	c         *card
	ctrl      uint8
	tapped    bool
	sick      bool
	catt      bool // attacking
	damage    int32
	power     int32
	tough     int32
	ctr       [dirac.FeatCtrKinds]uint16 // the kernel's kinds, then gorge's deathtouch mark
	nctr      uint8                      // kinds in gorge's counter list (ctrListed)
	ctrListed uint16                     // counter kinds gorge lists, a removed one at 0 (kernel FeatCountersListed)
	kw        kop.KwMask                 // granted (non-printed) keyword bits with a spelling
	kwRep     *kwReps                    // granted bits listed again: kwRep[b] more times (nil: none)
	printedKw bool
	manaSrc   bool // a permanent kernel FeatManaSources counts
	attached  uint8
	cblocks   uint8  // the attacker it blocks (noObj none)
	acts      int32  // non-mana activations this turn
	key       uint64 // the content cache key it was built for (cachedCard)
	// tyv is the derived type line gorge's view shows when a layer-4 effect
	// changed the permanent's card types (nil: the printed line, c's)
	tyv *typeView
}

// The type line's facts: the derived line when there is one.
func (c *vcore) creature() bool {
	if c.tyv != nil {
		return c.tyv.creature
	}
	return c.c.creature
}

func (c *vcore) land() bool {
	if c.tyv != nil {
		return c.tyv.land
	}
	return c.c.land
}

func (c *vcore) fold() uint16 {
	if c.tyv != nil {
		return c.tyv.fold
	}
	return c.c.fold
}

func (c *vcore) exact() uint16 {
	if c.tyv != nil {
		return c.tyv.exact
	}
	return c.c.exact
}

func (c *vcore) stackTypes(side int) []uint16 {
	if c.tyv != nil {
		return c.tyv.stackTy[side]
	}
	return c.c.stackTypes[side]
}

func (c *vcore) typeRows(zi int) []uint16 {
	if c.tyv != nil {
		return c.tyv.typeRows[zi]
	}
	return c.c.typeRows[zi]
}

// vplayer is one seat's public facts and zone lists (indices into cards).
type vplayer struct {
	life, handN, libN, graveN int32
	pool                      [6]int32
	avail                     int32 // pool + available, policynet seatAvailable
	bf, hand, grave, exile    []int32
}

// vstack is one stack entry (bottom first): the name its card id labels,
// and its card view (-1 for none).
type vstack struct {
	name *card
	cv   int32
	ctrl uint8
}

// vview is the seat's view of one position.
type vview struct {
	seat     uint8
	turn     int32
	step     int8 // policynet stepNames index; -1 none
	phase    int8
	active   uint8
	priority uint8
	cards    []vcard
	n        uint8 // the game's seats, a departed seat included (it keeps its place)
	players  [dirac.MaxSeats]vplayer
	// others is every seat but the viewer's as one side, in ascending seat
	// order (policynet's everyOther), built at more than two seats: what
	// the encodings read as "the opponent" there (sides).
	others vplayer
	stack  []vstack
	hosts  [256]uint8 // obj -> how many battlefield permanents are attached to it
}

// stepIndex maps kop.Step onto gorge's 12 step names (the kernel splits
// combat damage in two).
var stepIndex = [...]int8{
	kop.StepUntap: 0, kop.StepUpkeep: 1, kop.StepDraw: 2, kop.StepMain1: 3, kop.StepBeginCombat: 4,
	kop.StepAttackers: 5, kop.StepBlockers: 6, kop.StepFirstStrikeDamage: 7, kop.StepDamage: 7,
	kop.StepEndCombat: 8, kop.StepMain2: 9, kop.StepEnd: 10, kop.StepCleanup: 11,
}

// phaseOf is view.PhaseOf over the step index.
func phaseOf(step int8) int8 {
	switch {
	case step < 0:
		return -1
	case step <= 2:
		return 0
	case step == 3:
		return 1
	case step <= 8:
		return 2
	case step == 9:
		return 3
	default:
		return 4
	}
}

// viewBase is a search's base State (SetBase): its epoch, and every
// object's content hash then (Encoder.hb), which a State carrying the
// epoch reads for each object the kernel has not marked changed.
type viewBase struct {
	epoch uint32
}

// SetBase records s, whose epoch a search has just set, as the base the
// encoder's views are keyed against: an object the kernel has not marked
// changed since hashes as it did then. The output is unchanged
// (TestBaseViewMatchesFull).
func (e *Encoder) SetBase(s *dirac.State) {
	e.oroot = [dirac.MaxObj]cardHint{}
	b := e.vbase()
	b.epoch = s.Epoch()
	for o := 0; o < s.NumObjs(); o++ {
		e.hb[o] = s.ObjHash(dirac.Obj(o))
	}
}

// NoBase forgets the base (every object's hash is read from the line
// cache or computed).
func (e *Encoder) NoBase() {
	if e.vb != nil {
		e.vb.epoch = 0
	}
}

func (e *Encoder) vbase() *viewBase {
	if e.vb == nil {
		e.vb = new(viewBase)
	}
	return e.vb
}

// BaseHashes is the base's object content hashes when s carries its
// epoch (SetBase), else nil: kernel ViewHash's base argument.
func (e *Encoder) BaseHashes(s *dirac.State) *[dirac.MaxObj]uint64 {
	if e.vb != nil && e.vb.epoch != 0 && s.Epoch() == e.vb.epoch {
		return &e.hb
	}
	return nil
}

// HashCache is the line cache of changed objects' content hashes the
// encoder reads (kernel ViewHashCached's cache argument).
func (e *Encoder) HashCache() *dirac.ObjHashCache { return &e.ohc }

// po is object o's projection, written on first use in a build
// (kernel FeatProjObj: its zone position is not, FeatZoneIndex is).
func (e *Encoder) po(o uint8) *dirac.ProjObj {
	if e.kpg[o] != e.kgen {
		e.st0.FeatProjObj(dirac.Obj(o), &e.kp.Objs[o])
		e.kpg[o] = e.kgen
	}
	return &e.kp.Objs[o]
}

// sourceHidden: seat can't tell which card an ability's source is, so the ability
// can't carry its name: a library is hidden from every seat (CR 401.2), a hand from
// all but its owner (CR 402.3) and a face-down object from all but its controller
// (CR 708.5). A token that left the battlefield has ceased to exist, in no such zone
// whatever its Zone reads (CR 111.8).
func sourceHidden(po *dirac.ProjObj, seat uint8) bool {
	switch {
	case po.Flags&dirac.ProjFaceDown != 0:
		return po.Ctrl != seat
	case po.Token:
		return false
	case po.Zone == kop.ZoneLibrary:
		return true
	case po.Zone == kop.ZoneHand:
		return po.Owner != seat
	}
	return false
}

// build fills e.v from s for seat. Each card view comes from the content
// cache (cachedCard), keyed by the object's content hash -- the base's
// for an object unmarked since it, else the line cache's or computed.
func (e *Encoder) build(s *dirac.State, seat uint8, dk dirac.DecisionKind, decider uint8) {
	v := &e.v
	kp := &e.kp
	e.st0 = s
	e.kgen++
	if e.kgen == 0 {
		e.kpg = [dirac.MaxObj]uint32{}
		e.kgen = 1
	}
	s.FeatProjGlobals(kp)
	if int(kp.NObj) > int(noObj) {
		e.objOverflow++
	}
	if e.hbase = e.BaseHashes(s); e.hbase != nil {
		e.IncStats.BaseBuilds++
	}
	// the derived context is one value per build (cachedCard mixes it into
	// every permanent's and creature card's key); read it once here so a
	// card hint can compare it before deciding to recompute a key.
	e.ctx, e.ctxOK = s.FeatDerivedContext(e.hbase, &e.ohc), true
	if e.vc == nil {
		e.vc = new(vcCache)
	}
	e.vc.start, e.vc.novf = e.vc.use+1, 0
	v.cards = v.cards[:0]
	v.stack = v.stack[:0]
	v.seat = seat
	v.turn = int32(kp.Turn)
	v.step = -1
	if int(kp.Step) < len(stepIndex) {
		v.step = stepIndex[kp.Step]
	}
	v.phase = phaseOf(v.step)
	v.active, v.priority = kp.Active, kp.Prio
	v.n = s.NumSeats()
	for p := 0; p < int(v.n); p++ {
		pl := &v.players[p]
		pl.life = int32(kp.Life[p])
		pl.handN, pl.libN, pl.graveN = int32(kp.HandN[p]), int32(kp.LibN[p]), int32(kp.GraveN[p])
		for k := range pl.pool {
			pl.pool[k] = int32(kp.Pool[p][k])
		}
		pl.bf, pl.hand, pl.grave, pl.exile = pl.bf[:0], pl.hand[:0], pl.grave[:0], pl.exile[:0]
	}
	// Battlefields in entry order, per controller.
	for p := uint8(0); p < v.n; p++ {
		e.objs = s.FeatBattlefield(p, e.objs[:0])
		pl := &v.players[p]
		for _, o := range e.objs {
			pl.bf = append(pl.bf, e.addCard(s, uint8(o), zBattlefield))
		}
	}
	for p := 0; p < int(v.n); p++ {
		for _, ci := range v.players[p].bf {
			v.hosts[v.cards[ci].obj] = 0
		}
	}
	for p := 0; p < int(v.n); p++ {
		for _, ci := range v.players[p].bf {
			if a := v.cards[ci].attached; a != noObj {
				v.hosts[a]++
			}
		}
	}
	// A declaration in progress is not yet game state in gorge, which poses
	// it as one decision: the attackers (blockers) picked so far are not
	// attacking (blocking) until AkDone. At more than two seats each
	// defending player declares in turn (CR 509.1, 802.4): an earlier
	// defender's blocks are game state, only the decider's are in progress.
	if dk == dirac.DkAttackers || dk == dirac.DkBlockers {
		for i := range v.cards {
			c := &v.cards[i]
			if dk == dirac.DkAttackers {
				c.attacking = false
			} else if v.n <= 2 || c.ctrl == decider {
				c.blocks = noObj
			}
		}
	}
	// Blocked attackers (policynet: len(BlockedBy) > 0, which keeps a
	// blocker that has died); while blockers are declared, an attacker an
	// earlier defender blocked.
	if dk != dirac.DkBlockers {
		for p := 0; p < int(v.n); p++ {
			for _, bi := range v.players[p].bf {
				c := &v.cards[bi]
				c.blocked = s.FeatBlocked(dirac.Obj(c.obj))
			}
		}
	} else if v.n > 2 {
		for i := range v.cards {
			if b := v.cards[i].blocks; b != noObj {
				for j := range v.cards {
					if v.cards[j].obj == b {
						v.cards[j].blocked = true
					}
				}
			}
		}
	}
	// Hand (the viewer's), graveyards and exiles in arrival order: one pass
	// buckets the objects, each bucket sorted by zone position.
	for z := 0; z < 3; z++ {
		for p := 0; p < int(v.n); p++ {
			if z == 0 && uint8(p) != seat {
				continue
			}
			e.objs = s.FeatZoneList(uint8(p), [3]kop.Zone{kop.ZoneHand, kop.ZoneGraveyard, kop.ZoneExile}[z], e.objs[:0])
			pl := &v.players[p]
			for _, x := range e.objs {
				idx := e.addCard(s, uint8(x), uint8(zHand+z))
				switch z {
				case 0:
					pl.hand = append(pl.hand, idx)
				case 1:
					pl.grave = append(pl.grave, idx)
				default:
					pl.exile = append(pl.exile, idx)
				}
			}
		}
	}
	// The stack, bottom first.
	for i := 0; i < int(kp.NStack); i++ {
		se := &kp.Stack[i]
		st := vstack{name: e.pool.card(se.Card), cv: -1, ctrl: se.Ctrl}
		src := uint8(se.Source)
		if se.Kind == 0 {
			st.cv = e.addStackCard(s, src)
		} else if src != noObj && int(src) < int(kp.NObj) {
			// An ability is named by its source object's face in any zone
			// (gorge view abilityName: a ceased token dying with Fake Your
			// Own Death's granted trigger names the token, not the granting
			// card) the seat can see (sourceHidden), and its band shows the
			// source while the source is in a public zone (not a hidden one,
			// not a ceased token).
			po := e.po(src)
			st.name = e.pool.card(po.Card)
			if sourceHidden(po, seat) {
				st.name = hiddenAbility
			}
			switch po.Zone {
			case kop.ZoneBattlefield, kop.ZoneGraveyard, kop.ZoneExile, kop.ZoneStack:
				st.cv = e.addStackCard(s, src)
			}
		}
		v.stack = append(v.stack, st)
	}
	e.buildRel(s)
	// Available mana: the pool plus every untapped source that can tap.
	for p := 0; p < int(v.n); p++ {
		pl := &v.players[p]
		t := int32(0)
		for _, n := range pl.pool {
			t += n
		}
		for _, ci := range pl.bf {
			c := &v.cards[ci]
			if c.tapped {
				continue
			}
			w := c.c.avail
			if !c.printedKw {
				// its abilities are lost: only a granted mana ability counts
				// (a {T}: Add {C}, one unit)
				w = 0
				if e.st0.FeatManaGranted(dirac.Obj(c.obj)) {
					w = 1
				}
			} else if gw, ok := e.st0.FeatGrantAvail(dirac.Obj(c.obj)); ok {
				// a granted mana ability (Springleaf Parade's token): its
				// colours times its amount; none beside the card's own
				// (gorge AvailableMana omits a source with two free abilities)
				w = gw
			}
			if w == 0 {
				continue
			}
			if c.sick && c.creature() && !c.hasHaste() {
				continue
			}
			t += w
		}
		pl.avail = t
	}
	if v.n > 2 {
		v.mergeOthers()
	}
}

// sides is the viewer's player block and the side the encodings read as
// "the opponent": the other seat at two seats, every other seat merged
// (others) at more, as policynet's everyOther reads them.
func (v *vview) sides() (me, opp *vplayer) {
	if v.n <= 2 {
		return &v.players[v.seat&1], &v.players[1-v.seat&1]
	}
	return &v.players[v.seat], &v.others
}

// mergeOthers builds others: every seat but the viewer's, ascending (the
// order policynet's bags and entity lists walk everyOther). Its lists are
// the seats' lists concatenated; its counts and available mana are summed
// (policynet's "opp|avail" sums every other seat); its life is the first
// other seat's (policynet's mz life rows read others[0]).
func (v *vview) mergeOthers() {
	o := &v.others
	o.bf, o.hand, o.grave, o.exile = o.bf[:0], o.hand[:0], o.grave[:0], o.exile[:0]
	o.life, o.handN, o.libN, o.graveN, o.avail = 0, 0, 0, 0, 0
	o.pool = [6]int32{}
	first := true
	for p := uint8(0); p < v.n; p++ {
		if p == v.seat {
			continue
		}
		pl := &v.players[p]
		if first {
			o.life, first = pl.life, false
		}
		o.handN += pl.handN
		o.libN += pl.libN
		o.graveN += pl.graveN
		o.avail += pl.avail
		for k := range o.pool {
			o.pool[k] += pl.pool[k]
		}
		o.bf = append(o.bf, pl.bf...)
		o.grave = append(o.grave, pl.grave...)
		o.exile = append(o.exile, pl.exile...)
	}
}

// relPlayer is the seat at relative seat r (policynet's relPlayer: turn
// order from the viewer, wrapping).
func (v *vview) relPlayer(r int) uint8 { return v.seatFrom(v.seat, r) }

// seatFrom is the seat r places after p in turn order, wrapping.
func (v *vview) seatFrom(p uint8, r int) uint8 {
	if v.n <= 2 {
		return (p + uint8(r)) & 1
	}
	return uint8((int(p) + r) % int(v.n))
}

// lastToPass is the seat a priority round that began with the active
// player ends with (CR 117.4): the last seat before it in turn order still
// in the game; the non-active player at two seats.
func (v *vview) lastToPass(s *dirac.State) uint8 {
	if v.n <= 2 {
		return 1 - v.active
	}
	p := v.active
	for i := uint8(1); i < v.n; i++ {
		p = (p + v.n - 1) % v.n
		if !s.FeatLost(p) {
			return p
		}
	}
	return v.active
}

// relSeat is seat p relative to the viewer in turn order (policynet
// relSeat): 0 the viewer, 1 the next seat to take a turn, and so on.
func (v *vview) relSeat(p uint8) int {
	if v.n <= 2 {
		return int((p - v.seat) & 1)
	}
	return (int(p) - int(v.seat) + int(v.n)) % int(v.n)
}

// addCard appends obj's card view in zone z and returns its index.
func (e *Encoder) addCard(s *dirac.State, o uint8, z uint8) int32 {
	vc := e.cardView(s, o, z)
	e.v.cards = append(e.v.cards, vcard{vcore: vc, obj: o, zone: z, attacking: vc.catt, blocks: vc.cblocks})
	return int32(len(e.v.cards) - 1)
}

// cardView is obj's view in zone z: the last view the object had on this
// line of play when the kernel has not written it since (and the build's
// derived context and zone are the same), else the content cache's. The
// hint names a content-cache entry, checked by its key: an entry another
// build evicted falls back to cachedCard.
func (e *Encoder) cardView(s *dirac.State, o uint8, z uint8) *vcore {
	obj := dirac.Obj(o)
	line := s.Line()
	h := &e.ohint[o]
	if line != 0 && h.line == line && h.z == z && h.ctx == e.ctx && h.vc != nil &&
		!s.ChangedSince(obj, h.at) {
		if vc := e.useView(h.key); vc != nil {
			h.vc = vc
			return vc
		}
	}
	if e.hbase != nil {
		r := &e.oroot[o]
		if r.epoch == s.Epoch() && r.z == z && r.ctx == e.ctx && r.vc != nil &&
			!s.Changed(obj) {
			if vc := e.useView(r.key); vc != nil {
				r.vc = vc
				if line != 0 {
					h.line, h.at, h.ctx, h.key, h.vc, h.z = line, s.Touches(), e.ctx, vc.key, vc, z
				}
				return vc
			}
		}
	}
	vc := e.cachedCard(s, o, z)
	if line != 0 {
		h.line, h.at, h.ctx, h.key, h.vc, h.z = line, s.Touches(), e.ctx, vc.key, vc, z
	}
	if e.hbase != nil && !s.Changed(obj) {
		r := &e.oroot[o]
		r.epoch, r.ctx, r.key, r.vc, r.z = s.Epoch(), e.ctx, vc.key, vc, z
	}
	return vc
}

// useView marks the content cache entry holding key used by the current
// build and returns its view. A hint (or an epoch-root view) that names an
// entry does not otherwise count as a use, and cachedCard must not evict a
// view a listed card still points at: only a way used since the build's
// start is safe. It returns nil when the cache no longer holds key --
// evicted, or an overflow view, which is a build-local buffer and may be
// reused.
func (e *Encoder) useView(key uint64) *vcore {
	c := &e.vc.set[key>>(64-vcSetBits)]
	for i := range c.key {
		if c.key[i] == key {
			e.vc.use++
			c.use[i] = e.vc.use
			return &c.vc[i]
		}
	}
	return nil
}

// The content cache is set-associative: vcSets sets of vcWays views,
// a view's set chosen by its key's top bits.
const (
	vcSetBits = 9
	vcSets    = 1 << vcSetBits
	vcWays    = 4
)

// vcCache keeps card views keyed by content: the object's content hash
// (kernel ObjHash, which names no object: two copies of a card in a hand
// hash alike, and so does a card drawn as another object in another
// search world), its zone class and, on the battlefield, the objects it
// is attached to and blocks; for a view a derived characteristic reaches
// (a permanent, a creature card), the derived context too (kernel
// FeatDerivedContext). A view is a pure function of that key.
type vcCache struct {
	use   uint32
	start uint32 // use at the current build's start: a way used since holds a listed view
	// ovf holds the views a build lists beyond what a set can keep
	ovf [16]struct {
		vc  vcore
		rep kwReps
	}
	novf int
	set  [vcSets]struct {
		key [vcWays]uint64
		use [vcWays]uint32
		vc  [vcWays]vcore
		rep [vcWays]kwReps
	}
}

func (c *vcCache) clear() {
	for i := range c.set {
		c.set[i].key = [vcWays]uint64{}
		c.set[i].use = [vcWays]uint32{}
	}
}

// cachedCard is obj's card view in zone z from the content cache, built
// (mkCard) on a miss.
func (e *Encoder) cachedCard(s *dirac.State, o uint8, z uint8) *vcore {
	key := mix64(s.FeatObjHash(dirac.Obj(o), e.hbase, &e.ohc), uint64(z)+1)
	card, att, blk := s.FeatObjLinks(dirac.Obj(o))
	if z == zBattlefield {
		// the projection names the attached-to and blocked objects by
		// object; the hash has their slots
		key = mix64(key, uint64(att)|uint64(blk)<<16)
	} else if g, rep := s.FeatGrantedOffRep(dirac.Obj(o)); g != 0 {
		// a keyword granted off the battlefield (not in the hash), and how
		// many times it was granted
		key = mix64(key, uint64(g)|uint64(rep[trailing(uint64(kop.KwFlashback))])<<32)
	}
	if z == zStack {
		// a spell's controller is its caster, which the hash (owner) does
		// not name: Etali casting the opponent's exiled Empyrean Eagle
		key = mix64(key, uint64(e.po(o).Ctrl)+0x5354)
	}
	if z == zBattlefield || e.pool.card(card).creature {
		if !e.ctxOK {
			e.ctx, e.ctxOK = s.FeatDerivedContext(e.hbase, &e.ohc), true
		}
		key = mix64(key, e.ctx)
	}
	key |= 1
	c := &e.vc.set[key>>(64-vcSetBits)]
	e.vc.use++
	j := 0
	for i := range c.key {
		if c.key[i] == key {
			c.use[i] = e.vc.use
			e.IncStats.CardHits++
			return &c.vc[i]
		}
		if c.use[i] < c.use[j] {
			j = i
		}
	}
	e.IncStats.CardBuilds++
	e.IncStats.BuildsZ[z]++
	if c.key[j] != 0 && c.use[j] >= e.vc.start {
		// every way holds a view this build listed: build this one
		// beside the cache, for this build only
		x := &e.vc.ovf[e.vc.novf%len(e.vc.ovf)]
		e.vc.novf++
		x.vc = e.mkCard(s, e.po(o), o, z, &x.rep)
		x.vc.key = key
		return &x.vc
	}
	c.key[j], c.use[j] = key, e.vc.use
	c.vc[j] = e.mkCard(s, e.po(o), o, z, &c.rep[j])
	c.vc[j].key = key
	return &c.vc[j]
}

func mix64(h, x uint64) uint64 {
	h ^= x
	h *= 0x9e3779b97f4a7c15
	return h ^ h>>29
}

// mkCard is obj's card view in zone z, read from s and its projection ob.
func (e *Encoder) mkCard(s *dirac.State, ob *dirac.ProjObj, o uint8, z uint8, reps *kwReps) vcore {
	if reps != nil {
		*reps = kwReps{}
	}
	c := vcore{cid: ob.Card, c: e.pool.card(ob.Card), ctrl: ob.Ctrl, attached: noObj, cblocks: noObj}
	if z == zBattlefield {
		if nc, ok := s.FeatNameCard(dirac.Obj(o)); ok {
			c.c = e.renamed(ob.Card, nc) // renamed by an override: gorge's view names it so
		}
		if ty, subs, all, setSub, ok := s.FeatViewTypes(dirac.Obj(o)); ok {
			c.tyv = e.typeLine(c.c, ty, subs, all, setSub)
		}
		c.tapped = ob.Flags&dirac.ProjTapped != 0
		c.sick = ob.Flags&dirac.ProjSick != 0
		c.catt = ob.Flags&dirac.ProjAttacking != 0
		c.damage = int32(ob.Damage)
		c.power, c.tough = int32(ob.Power), int32(ob.Tough)
		if !c.creature() {
			// gorge derives a noncreature permanent's P/T too (an
			// imprisoned 2/2 with its +1/+1 counters, an uncrewed
			// vehicle); the kernel's projection says 0
			p, t := s.FeatPT(dirac.Obj(o))
			c.power, c.tough = int32(p), int32(t)
		}
		copy(c.ctr[:], ob.Counters[:])
		c.ctr[dirac.FeatCtrDeathtouched] = uint16(s.FeatDeathtouchMark(dirac.Obj(o)))
		c.ctrListed = s.FeatCountersListed(dirac.Obj(o), &ob.Counters)
		c.nctr = uint8(bits.OnesCount16(c.ctrListed))
		c.attached = uint8(ob.Attached)
		c.cblocks = uint8(ob.Blocks)
		g, rep, printed := s.FeatGrantedRep(dirac.Obj(o))
		c.printedKw = printed
		c.manaSrc = s.FeatManaActivatable(dirac.Obj(o))
		c.kw = g & e.pool.kwSpelled
		if e.spelledReps(&rep, reps) {
			c.kwRep = reps
		}
	} else {
		c.power, c.tough = c.c.power, c.c.tough
		// a card's counters off the battlefield: Tinybones' stash counter
		// on an exiled card (gorge's option counts it)
		copy(c.ctr[:], ob.Counters[:])
		c.ctrListed = s.FeatCountersListed(dirac.Obj(o), &ob.Counters)
		c.nctr = uint8(bits.OnesCount16(c.ctrListed))
		c.printedKw = true
		g, rep := s.FeatGrantedOffRep(dirac.Obj(o))
		c.kw = g & e.pool.kwSpelled
		if e.spelledReps(&rep, reps) {
			c.kwRep = reps
		}
		c.sick = s.FeatSickOff(dirac.Obj(o))
		if c.c.creature {
			// a card's own characteristic-defining P/T applies in any zone
			if p, t, ok := s.FeatCDAPT(dirac.Obj(o)); ok {
				c.power, c.tough = int32(p), int32(t)
			}
		}
	}
	c.acts = int32(s.FeatActivations(dirac.Obj(o)))
	return c
}

// addStackCard appends the card view a stack band shows (the spell itself,
// or an ability's source) with the stack's zone class.
func (e *Encoder) addStackCard(s *dirac.State, o uint8) int32 {
	if e.po(o).Zone == kop.ZoneBattlefield {
		// The source's own permanent view (cardView of the source id).
		idx := e.addCard(s, o, zBattlefield)
		e.v.cards[idx].zone = zStack
		return idx
	}
	return e.addCard(s, o, zStack)
}

// hasHaste and friends are policynet hasKeyword over the derived list.
func (c *vcore) hasHaste() bool    { return c.printedKw && c.c.haste || c.kw&kop.KwHaste != 0 }
func (c *vcore) hasFlying() bool   { return c.printedKw && c.c.flying || c.kw&kop.KwFlying != 0 }
func (c *vcore) hasDefender() bool { return c.printedKw && c.c.defender || c.kw&kop.KwDefender != 0 }
func (c *vcore) hasFlash() bool    { return c.printedKw && c.c.flash || c.kw&kop.KwFlash != 0 }

// eachKw calls f for every keyword of c in gorge's order: the printed list,
// then the granted bits in bit order, then the bits a second and a third
// effect grants again (gorge lists a keyword once per granting effect).
func (e *Encoder) eachKw(c *vcard, f func(k *kwInfo)) {
	if c.printedKw {
		for i := range c.c.kws {
			f(&c.c.kws[i])
		}
	}
	for m := c.kw; m != 0; m &= m - 1 {
		f(&e.pool.kw[trailing(uint64(m))])
	}
	if c.kwRep == nil {
		return
	}
	for b, n := range c.kwRep {
		for ; n != 0; n-- {
			f(&e.pool.kw[b])
		}
	}
}

// spelledReps merges rep's spelled keywords into dst (saturating); false
// when none repeats. Two calls merge: a granted-off repeat and a static
// pump's repeat both count.
func (e *Encoder) spelledReps(rep, dst *kwReps) bool {
	any := false
	for b := range rep {
		n := rep[b]
		if n == 0 || e.pool.kwSpelled&(kop.KwMask(1)<<b) == 0 {
			continue
		}
		if v := int(dst[b]) + int(n); v > 255 {
			dst[b] = 255
		} else {
			dst[b] = uint8(v)
		}
		any = true
	}
	return any
}

func trailing(x uint64) int { return bits.TrailingZeros64(x) }

func sortU16(a []uint16) {
	for i := 1; i < len(a); i++ {
		for j := i; j > 0 && a[j] < a[j-1]; j-- {
			a[j], a[j-1] = a[j-1], a[j]
		}
	}
}
