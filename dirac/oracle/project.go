package oracle

import (
	"hash/fnv"
	"reflect"
	"slices"
	"strings"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/kop"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/rules"
	"github.com/adams-shaun/gorge/engine/state"
)

// Projection limits.
const (
	MaxInst  = dirac.MaxObj // card instances (Ref 0..MaxInst-1)
	MaxStack = 48           // dirac.MaxStack: every kernel stack entry is compared
	MaxTgts  = 4            // kop.MaxTargets is 5, but a choose-two Command projects <= 3 targets
)

// Zone codes (gorge's state.Zone numbering).
const (
	ZoneLibrary uint8 = iota
	ZoneHand
	ZoneBattlefield
	ZoneGraveyard
	ZoneExile
	ZoneStack
	ZoneCommand
	ZoneCeased = 0x7F // a token or copy that left the game's zones
	ZoneNone   = 0xFF // instance unused
)

// CardProj flag bits.
const (
	FTapped uint16 = 1 << iota
	FSummonSick
	FAttacking
	FBlocking
	FFaceDown
	FToken
	FCopy
	FPhasedOut
	FMonstrous
	FSuspected
)

// Counter slots of CardProj.Counters.
const (
	CtrP1P1 = iota
	CtrM1M1
	CtrLoyalty
	CtrOther // sum of every other kind
	// CtrX is the first of the unfeaturized kinds' own slots (kcounters):
	// slot CtrX+i is kop.CtrCharge+i (gorge's name kop.CtrXNames[i]),
	// compared per kind rather than summed into CtrOther
	CtrX
	// CtrShieldC is the SHIELD counter's own slot (shield, CR 122.1c), kept
	// apart from gorge's "Shield" regeneration marker, which both sides sum
	// into CtrOther
	CtrShieldC = CtrX + kop.NumCounterKindsX
	NumCtr     = CtrShieldC + 1
)

// CardProj is one card instance.
type CardProj struct {
	NameHash   uint32 // FNV-1a 32 of the front face name (tokens included)
	Card       uint8  // deck table index (Matchup.Table); NoCardIndex for tokens and copies
	Face       uint8  // the face up (gorge's Object.FaceIdx; a multi-face card'sb)
	Zone       uint8
	Owner      uint8
	Controller uint8
	Pos        uint8 // position in an ordered zone: the library from the top (0 = next draw), the controller's battlefield list, the graveyard and exile in arrival order; 255 elsewhere
	Flags      uint16
	Damage     int16
	Power      int16 // battlefield creatures only; 0 elsewhere
	Toughness  int16
	Counters   [NumCtr]int16
	AttachedTo Ref
	Blocks     Ref // a blocker: the attacker it blocks (first); RefNone otherwise
	// Chars is the derived types, subtypes, colours and keywords
	// (battlefield only; projchars.go)
	Chars CharsProj
}

// StackProj is one stack object, bottom first.
type StackProj struct {
	Kind       uint8 // 0 spell, 1 activated, 2 triggered
	Source     Ref   // the spell's own instance, or the ability's source
	Ability    uint8 // activated: Face.Abilities index; triggered: 0x80|Face.Triggers index; 255 unknown
	Controller uint8
	NTargets   uint8
	Targets    [MaxTgts]Ref
	Modes      uint8 // chosen modes, a bit per index in the charm's Choices$ order
	X          int16 // the announced X
}

// Projection is the canonical state both sides are compared on after every
// committed action (doc.go, "Projection").
type Projection struct {
	NInst  uint8
	Cards  [MaxInst]CardProj
	Life   [2]int32
	Poison [2]int16
	Speed  [2]uint8 // CR 702.179
	// Initiative is the initiative's holder, -1 nobody (CR 726)
	Initiative int8
	// DayNight is the CR 731 designation: 0 neither, 1 day, 2 night
	DayNight uint8
	// Room is each player's venture marker: 0 no dungeon, else the room's
	// index + 1 in the dungeon's printed K:Dungeon order (CR 309.4)
	Room        [2]uint8
	LandsPlayed [2]uint8
	Pool        [2][6]int16 // WUBRGC
	HandCount   [2]uint8
	LibCount    [2]uint8
	NStack      uint8
	Stack       [MaxStack]StackProj
	Turn        int32
	Step        uint8 // gorge state.Step numbering
	Active      uint8
	Priority    uint8
	RNGDraws    uint64
	Over        bool
	Winner      int8 // -1 none/draw
	// CmdCasts is the times each seat's commander was cast from the command
	// zone (CR 903.8); CmdDamage[p][q] the combat damage player p was dealt
	// by seat q's commander (CR 903.10a). Zero outside a Commander game.
	// They stay last: ProjectionHash drops them while they are zero.
	CmdCasts  [2]uint8
	CmdDamage [2][2]uint8
	// Seats is a game of more than two seats' further seats (zero at two
	// seats). It stays last: ProjectionHash drops it while it is zero, so a
	// two-seat projection hashes as it did before it existed.
	Seats SeatsProj
}

// SeatsProj is the per-seat state of seats 2 and up, and every seat's loss.
type SeatsProj struct {
	NSeat uint8
	Lost  [MaxSeats]bool
	More  [MaxSeats - 2]SeatProj
}

// SeatProj is one further seat's Projection fields (Life .. LibCount, CmdCasts).
type SeatProj struct {
	Life        int32
	Poison      int16
	Speed       uint8
	Room        uint8
	LandsPlayed uint8
	Pool        [6]int16
	HandCount   uint8
	LibCount    uint8
	CmdCasts    uint8
}

// identity is a game's stable map between gorge objects and canonical card
// instances (doc.go, "Card identity"). gorge keeps an object's ObjID across
// zone changes (state.Object's doc), so an instance is bound to an ObjID once,
// when the object is first seen, and never rebound.
type identity struct {
	inst    []Ref         // by ObjID; RefNone for a non-card object (an ability, an emblem)
	obj     []state.ObjID // by instance
	scanned int           // objects scanned so far (len(G.Objs) at the last sync)
	err     string        // first overflow, if any
	// lay is the genesis card objects' sizes (deck 0, sideboard 0, deck 1,
	// sideboard 1) when a seat has a sideboard, and ncard the genesis card
	// objects bound so far: gorge deals each seat's deck then its sideboard,
	// where the kernel numbers both decks, then both sideboards (sync).
	lay   [4]int
	ncard int
	// gen is the genesis card objects (both decks and sideboards): the
	// instances past it are tokens, copies and dungeons (freeInst)
	gen int
}

// setLayout records m's genesis order for sync; a matchup with no sideboard
// leaves the plain deck order.
func (id *identity) setLayout(m *Matchup) {
	if m != nil {
		id.gen = len(m.Side[0]) + len(m.Side[1])
		for s := 0; s < m.NSeat(); s++ {
			id.gen += len(m.seatIx(s))
		}
	}
	if m == nil || len(m.Side[0])+len(m.Side[1]) == 0 {
		return
	}
	id.lay = [4]int{len(m.Deck[0]), len(m.Side[0]), len(m.Deck[1]), len(m.Side[1])}
}

// genesisInst is the instance of the n-th genesis card object (gorge's deal
// order: deck 0, sideboard 0, deck 1, sideboard 1) in the kernel's numbering
// (deck 0, deck 1, sideboard 0, sideboard 1).
func (id *identity) genesisInst(n int) int {
	d0, s0, d1 := id.lay[0], id.lay[1], id.lay[2]
	switch {
	case n < d0:
		return n
	case n < d0+s0:
		return d0 + d1 + (n - d0)
	case n < d0+s0+d1:
		return d0 + (n - d0 - s0)
	}
	return d0 + d1 + s0 + (n - d0 - s0 - d1)
}

// sync binds every object created since the last call. Genesis creates the
// deck cards owner by owner in decklist order, so deck s's i-th card is
// instance offset(s)+i; every later card object (a token, a spell or
// permanent copy) takes the next instance in creation (ObjID) order. Once
// all MaxInst instances are bound, a new object takes the lowest instance
// freeInst reports, the kernel's object id reuse (dirac zone_objreuse.go).
func (id *identity) sync(g *state.Game) {
	for id.scanned < len(g.Objs) {
		o := &g.Objs[id.scanned]
		id.scanned++
		for len(id.inst) <= int(o.ID) {
			id.inst = append(id.inst, RefNone)
		}
		if o.Card == nil {
			continue
		}
		if len(id.obj) >= MaxInst {
			if inst := id.freeInst(g); inst >= 0 {
				id.inst[id.obj[inst]] = RefNone
				id.inst[o.ID] = Ref(inst)
				id.obj[inst] = o.ID
				continue
			}
			if id.err == "" {
				id.err = "more than MaxInst card instances"
			}
			continue
		}
		if n := id.lay[0] + id.lay[1] + id.lay[2] + id.lay[3]; id.ncard < n {
			inst := id.genesisInst(id.ncard)
			id.ncard++
			for len(id.obj) <= inst {
				id.obj = append(id.obj, 0)
			}
			id.inst[o.ID] = Ref(inst)
			id.obj[inst] = o.ID
			continue
		}
		id.inst[o.ID] = Ref(len(id.obj))
		id.obj = append(id.obj, o.ID)
	}
}

// freeInst is the lowest instance the kernel would mint again, -1 none: a
// token, copy or dungeon instance (past genesis) whose object ceased to
// exist in an earlier turn (ZCeased, not entered this turn: the kernel's
// zoneTurn) and that no delayed trigger names. Instances bound by this same
// sync entered their zone this turn, so none is handed out twice.
func (id *identity) freeInst(g *state.Game) int {
	for inst := id.gen; inst < len(id.obj); inst++ {
		o := g.Obj(id.obj[inst])
		if o == nil || o.Zone != state.ZCeased || o.EnteredThisTurn || delayedNames(g, o.ID) {
			continue
		}
		return inst
	}
	return -1
}

// delayedNames reports whether a delayed trigger names oid as its source,
// creator or a remembered object.
func delayedNames(g *state.Game, oid state.ObjID) bool {
	for i := range g.Delayed {
		d := &g.Delayed[i]
		if d.Source == oid || d.Creator == oid {
			return true
		}
		for _, t := range d.Remembered {
			if !t.IsPlayer && t.Obj == oid {
				return true
			}
		}
	}
	return false
}

// ref is the canonical reference of object oid in g.
func (id *identity) ref(g *state.Game, oid state.ObjID) Ref {
	if oid == 0 {
		return RefNone
	}
	if pl, ok := oid.PlayerRef(); ok {
		return RefPlayer + Ref(pl)
	}
	if int(oid) < len(id.inst) && id.inst[oid] != RefNone {
		return id.inst[oid]
	}
	for i, s := range g.Stack {
		if s == oid && i < int(RefPlayer-RefStack) {
			return RefStack + Ref(i)
		}
	}
	return RefNone
}

// target is a state.Target's canonical reference.
func (id *identity) target(g *state.Game, t state.Target) Ref {
	if t.IsPlayer {
		return RefPlayer + Ref(t.Player)
	}
	return id.ref(g, t.Obj)
}

func nameHash(s string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(s))
	return h.Sum32()
}

// project fills p from gorge engine e (doc.go, "Projection"). It reads
// state.Game and the engine's derived characteristics (Power/Toughness, the
// layer-applied values engine/view projects) directly: view.ProjectInto
// carries the same fields as strings and maps, and the oracle runs at every
// decision, so it reads the source the view itself reads.
func project(e *rules.Engine, id *identity, m *Matchup, p *Projection) (charErr string) {
	g := e.G
	id.sync(g)
	*p = Projection{}
	p.NInst = uint8(len(id.obj))
	for i := range p.Cards {
		p.Cards[i] = CardProj{Card: NoCardIndex, Zone: ZoneNone, Pos: 255, AttachedTo: RefNone, Blocks: RefNone}
	}
	nDeck := 0
	if m != nil {
		nDeck = id.gen
	}
	for inst, oid := range id.obj {
		o := g.Obj(oid)
		c := &p.Cards[inst]
		if o == nil {
			continue
		}
		c.NameHash = nameHash(cardName(o.Card))
		c.Face = o.FaceIdx
		if inst < nDeck && !o.IsToken && !o.IsCopy {
			c.Card = uint8(m.instanceTable(inst))
		}
		c.Zone = uint8(o.Zone)
		if o.Zone == state.ZCeased {
			c.Zone = ZoneCeased
		}
		c.Owner, c.Controller = uint8(o.Owner), uint8(o.Controller)
		var f uint16
		if o.Tapped {
			f |= FTapped
		}
		if o.SummonSick {
			f |= FSummonSick
		}
		if o.IsAttacking {
			f |= FAttacking
		}
		if o.IsBlocking {
			f |= FBlocking
		}
		if o.FaceDown {
			f |= FFaceDown
		}
		if o.IsToken {
			f |= FToken
		}
		if o.IsCopy {
			f |= FCopy
		}
		if o.Zone != state.ZBattlefield {
			f &^= FTapped | FSummonSick | FAttacking | FBlocking
		}
		if o.PhasedOut {
			f |= FPhasedOut
		}
		if o.Monstrous {
			f |= FMonstrous
		}
		if o.Suspected {
			f |= FSuspected
		}
		if o.Zone != state.ZBattlefield {
			// Permanent status is not tracked off the battlefield (gorge
			// keeps a stale summoning-sick bit on a dead creature).
			f &= FToken | FCopy | FFaceDown
		}
		c.Flags = f
		if o.Zone == state.ZBattlefield {
			c.Damage = int16(min(o.Damage, 255)) // the kernel saturates damage at 255 (dmgPerm); no toughness reaches it, so lethality agrees
			if e.IsCreature(oid) {
				c.Power, c.Toughness = int16(e.Power(oid)), int16(e.Toughness(oid))
			}
			if m != nil && m.chars != nil {
				var ce string
				if c.Chars, ce = gorgeChars(e, m.chars, oid); ce != "" && charErr == "" {
					charErr = ce
				}
			}
			c.AttachedTo = id.ref(g, o.AttachedTo)
			for _, b := range o.BlockedBy {
				if bi := id.ref(g, b); bi < MaxInst && p.Cards[bi].Blocks == RefNone {
					p.Cards[bi].Blocks = Ref(inst)
				}
			}
		}
		for _, ct := range o.Counters {
			if o.Zone == state.ZHand || o.Zone == state.ZGraveyard || o.Zone == state.ZLibrary {
				// gorge keeps a dead creature's counters on the card (CR
				// 122.2 removes them); the kernel cannot, so they are not
				// compared off the battlefield (an exiled card keeps a
				// stash counter: Tinybones)
				break
			}
			slot := CtrOther
			switch ct.Kind {
			case "P1P1":
				slot = CtrP1P1
			case "M1M1":
				slot = CtrM1M1
			case "LOYALTY":
				slot = CtrLoyalty
			case "SHIELD":
				slot = CtrShieldC
			default:
				for x, n := range kop.CtrXNames {
					if ct.Kind == n {
						slot = CtrX + x
					}
				}
			}
			n := int16(ct.N)
			if ct.Kind == "Deathtouched" && n > 2 {
				// The kernel keeps a two-event deathtouch mark
				// (pfDeathtouched, pfDtMore; FeatDeathtouchMark 1|2) while
				// gorge adds one "Deathtouched" counter per event. Any mark
				// is lethal at the next SBA, so this is a feature-view
				// representation difference, not gameplay: clamp the gorge side.
				n = 2
			}
			c.Counters[slot] += n
		}
	}
	p.DayNight = gorgeDayNight(g)
	p.Initiative = -1
	if g.HasInitiative {
		p.Initiative = int8(g.Initiative)
	}
	if n := len(g.Players); n > 2 {
		p.Seats.NSeat = uint8(n)
	}
	for s := 0; s < MaxSeats && s < len(g.Players); s++ {
		pl := &g.Players[s]
		sp := p.seat(s)
		if p.Seats.NSeat != 0 {
			p.Seats.Lost[s] = pl.Lost
		}
		*sp.life = pl.Life
		*sp.speed = uint8(pl.Speed)
		if do := g.Obj(pl.DungeonObj); pl.DungeonObj != 0 && do != nil && do.Face() != nil {
			*sp.room = dungeonRoomIndex(do.Face(), pl.DungeonRoom)
		}
		if state.PlayerID(s) == g.Active {
			// Only the active player's count is game state: gorge resets
			// a player's count at their own turn, the kernel at every turn.
			*sp.lands = uint8(pl.LandsPlayed)
		}
		for k := 0; k < 6; k++ {
			sp.pool[k] = int16(pl.Pool[k])
		}
		for _, ct := range pl.Counters {
			if ct.Kind == "POISON" {
				*sp.poison += int16(ct.N)
			}
		}
		lib := g.Zone(state.ZLibrary, state.PlayerID(s))
		*sp.lib = uint8(len(lib))
		*sp.hand = uint8(len(g.Zone(state.ZHand, state.PlayerID(s))))
		for i, oid := range lib {
			if r := id.ref(g, oid); r < MaxInst && i < 255 {
				p.Cards[r].Pos = uint8(i)
			}
		}
		for _, z := range [3]state.Zone{state.ZBattlefield, state.ZGraveyard, state.ZExile} {
			for i, oid := range g.Zone(z, state.PlayerID(s)) {
				if r := id.ref(g, oid); r < MaxInst && i < 255 {
					p.Cards[r].Pos = uint8(i)
				}
			}
		}
	}
	for i, sid := range g.Stack {
		if i >= MaxStack {
			break
		}
		o := g.Obj(sid)
		if o == nil {
			continue
		}
		st := &p.Stack[i]
		st.Controller = uint8(o.Controller)
		st.Ability = 255
		st.Modes = chosenModeMask(o)
		st.X = int16(o.X)
		for k := range st.Targets {
			st.Targets[k] = RefNone
		}
		if o.Ability != nil && o.Card == nil {
			st.Kind = 1
			if o.StackKind == state.StackKindTriggered {
				st.Kind = 2
			}
			st.Source = id.ref(g, o.Source)
			if src := g.Obj(o.Source); src != nil {
				st.Ability = abilityIndex(src.Face(), o.Ability)
			}
		} else {
			st.Source = id.ref(g, sid)
		}
		// Every target of the whole chain: the main ability's, then its
		// sub-abilities' (Prey Upon's fight target).
		for _, ts := range [2][]state.Target{o.Targets, o.SubTargets} {
			for _, t := range ts {
				if st.NTargets >= MaxTgts {
					break
				}
				st.Targets[st.NTargets] = id.target(g, t)
				st.NTargets++
			}
		}
		p.NStack++
	}
	p.Turn = g.Turn
	p.Step = uint8(g.Step)
	p.Active, p.Priority = uint8(g.Active), uint8(g.Priority)
	p.RNGDraws = e.RNGDraws()
	projectCommander(g, p)
	p.Over, p.Winner = g.Over, -1
	if g.Over && !g.Draw {
		p.Winner = int8(g.Winner)
	}
	return charErr
}

// seatFields points at seat s's per-seat Projection fields.
type seatFields struct {
	life               *int32
	poison             *int16
	speed, room, lands *uint8
	pool               *[6]int16
	hand, lib, casts   *uint8
}

// seat is seat s's fields: the two-seat arrays for seats 0 and 1, Seats.More
// past them.
func (p *Projection) seat(s int) seatFields {
	if s < 2 {
		return seatFields{&p.Life[s], &p.Poison[s], &p.Speed[s], &p.Room[s], &p.LandsPlayed[s], &p.Pool[s], &p.HandCount[s], &p.LibCount[s], &p.CmdCasts[s]}
	}
	m := &p.Seats.More[s-2]
	return seatFields{&m.Life, &m.Poison, &m.Speed, &m.Room, &m.LandsPlayed, &m.Pool, &m.HandCount, &m.LibCount, &m.CmdCasts}
}

// projectCommander fills p's commander bookkeeping from g. A seat's
// CmdDamage is indexed by the match-wide dense commander index, every
// seat's Commanders in seat order: the slot is mapped to its owner's seat.
func projectCommander(g *state.Game, p *Projection) {
	var owner []int
	for s := range g.Players {
		for range g.Players[s].Commanders {
			owner = append(owner, s)
		}
	}
	for s := 0; s < MaxSeats && s < len(g.Players); s++ {
		pl := &g.Players[s]
		var n int32
		for _, c := range pl.CmdCasts {
			n += c
		}
		*p.seat(s).casts = uint8(min(max(n, 0), 255))
		if s >= 2 {
			continue // CmdDamage is the two-seat game's
		}
		for k, d := range pl.CmdDamage {
			if k < len(owner) && owner[k] < 2 {
				q := owner[k]
				p.CmdDamage[s][q] = uint8(min(max(int32(p.CmdDamage[s][q])+d, 0), 255))
			}
		}
	}
}

// chosenModeMask is stack object o's ChosenModes as a bit per mode's index in
// its charm's Choices$ order.
func chosenModeMask(o *state.Object) uint8 {
	if len(o.ChosenModes) == 0 {
		return 0
	}
	sa := o.Ability
	if sa == nil {
		sa = o.Face().SpellAbility()
	}
	var m uint8
	for i, name := range effects.CharmOf(sa).Modes {
		if i < 8 && slices.Contains(o.ChosenModes, name) {
			m |= 1 << i
		}
	}
	return m
}

// abilityIndex is sa's index in face: Abilities i, or 0x80|i for the
// trigger whose Effect it is; 255 when it is neither (a delayed trigger, an
// SVar-minted or granted ability).
func abilityIndex(face *cards.Face, sa *cards.SA) uint8 {
	if face == nil || sa == nil {
		return 255
	}
	for i, a := range face.Abilities {
		if a == sa && i < 0x80 {
			return uint8(i)
		}
	}
	for i, t := range face.Triggers {
		if t.Effect == sa && i < 0x7F {
			return 0x80 | uint8(i)
		}
	}
	return 255
}

// dungeonRoomIndex is room key's index + 1 in the dungeon face's printed
// K:Dungeon order (0: no marker room; 255: a key the order lacks).
func dungeonRoomIndex(f *cards.Face, room string) uint8 {
	if room == "" {
		return 0
	}
	list, _ := f.KeywordParam("Dungeon")
	for i, r := range strings.Split(list, ",") {
		if strings.TrimSpace(r) == room {
			return uint8(i + 1)
		}
	}
	return 255
}

// gorgeDayNightField is the index of state.Game's DayNight field (gorge
// cr-daybound-nightbound, 2e8eb2156), nil on a gorge pin before it: read
// by reflection so the oracle builds against either pin.
var gorgeDayNightField = func() []int {
	if f, ok := reflect.TypeOf(state.Game{}).FieldByName("DayNight"); ok && f.Type.Kind() == reflect.Uint8 {
		return f.Index
	}
	return nil
}()

// gorgeDayNight is g's CR 731 designation (0 neither, 1 day, 2 night).
func gorgeDayNight(g *state.Game) uint8 {
	if gorgeDayNightField == nil {
		return 0
	}
	return uint8(reflect.ValueOf(g).Elem().FieldByIndex(gorgeDayNightField).Uint())
}
