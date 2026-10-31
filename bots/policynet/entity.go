package policynet

import (
	"fmt"
	"math/rand/v2"
	"strings"
	"sync"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/engine/view"
)

// The entity encoding (ticket pn14, FeaturesEntity). The mz set is a
// zone-aggregated hashed bag: "some creature is tapped", "some creature is a
// 3/3 flyer" — it cannot say WHICH creature. The entity set binds properties
// to individual cards: every visible card becomes one fixed-width vector
// (EntityCard: EntityRawWidth raw scalars plus hashed name/API rows into the
// shared embedding table), a learned per-card encoder turns it into
//
//	h_c = relu(EntW · [raw_c ‖ Σ_(r,v) E[r]·v] + EntB) ∈ R^EntK
//
// and the model reads those vectors twice:
//
//   - pooled into the state trunk: per group (my battlefield, the opponents'
//     battlefields, my hand, the stack) the scaled sum and the elementwise max
//     of h_c, P ∈ R^(EntityGroups·2·EntK), projected s += EntPᵀ·P. The max is
//     what lets "the opponent has one untapped 4/4 deathtoucher" survive the
//     pool, which a sum of raw features cannot express;
//   - per option: the hidden input gains [h_EntA ‖ h_EntB] (2·EntK floats),
//     the option's own card (Obj) and its related card (the attacker a
//     blocker would block, the planeswalker or battle an attacker attacks).
//
// The opponent's hand enters only as its count (the v1 dense block already
// carries it). Every read is from the redacted view.
//
// Determinism: the card list is built by walking the view's zone lists in a
// fixed order; no map is ranged.

// EntityGroups is the number of pooled card groups.
const EntityGroups = 4

// Entity groups (EntityCard.Group).
const (
	entGroupBfMe = iota
	entGroupBfOpp
	entGroupHand
	entGroupStack
)

// entSumScale scales the per-group sum pool (a board of ~10 cards should not
// dwarf the max pool).
const entSumScale = 0.25

// Raw entity layout (EntityRawWidth scalars; pinned by entityHashSuffix).
const (
	entTypeOff    = 0  // 10: entityTypes
	entPower      = 10 // power/4
	entToughness  = 11 // (toughness − damage)/4
	entDamaged    = 12
	entKwOff      = 13 // 31 keywordBits + 1 other
	entTapped     = 45
	entSick       = 46
	entCanAttack  = 47
	entCanBlock   = 48
	entAttacking  = 49
	entBlocking   = 50
	entBlocked    = 51
	entCtrPlus    = 52 // +1/+1 counters / 2
	entCtrMinus   = 53 // -1/-1 counters / 2
	entCtrLoyalty = 54 // loyalty / 4
	entCtrOther   = 55 // every other counter / 2
	entAttached   = 56 // attached to something
	entAttachMine = 57 // attached to a permanent I control
	entHosts      = 58 // number of attachments on it / 2
	entMine       = 59 // controller is the viewer
	entZoneBf     = 60
	entZoneHand   = 61
	entZoneStack  = 62
	entCastable   = 63 // hand, non-land, mana value <= available
	entManaValue  = 64 // mv / 5
	entInstant    = 65 // instant-speed
	entFaceDown   = 66
	entOne        = 67 // constant 1: a card's presence

	// Relational slots (entity format 2: what the stack targets and, on a
	// stack card, what it targets; format 3 adds the combat slots 78-82).
	entTgtByMe      = 68 // viewer-controlled stack entries targeting it
	entTgtByOpp     = 69 // other seats' stack entries targeting it
	entTgtPermMe    = 70 // its entry's object targets on my battlefield
	entTgtPermOpp   = 71 // its entry's object targets on their battlefield
	entTgtPlayerMe  = 72 // its entry's player targets naming me
	entTgtPlayerOpp = 73 // its entry's player targets naming another seat
	entTgtStack     = 74 // its entry's targets that are stack objects
	entTgtGrave     = 75 // its entry's targets that are graveyard cards
	entStackX       = 76 // X / 5
	entStackKicked  = 77 // kicked at least once
	entCombatFoes   = 78 // foes / 2
	entCombatFoePow = 79 // sum of foe power / 4
	entCombatFoeTgh = 80 // sum of foe remaining toughness / 4
	entCombatDies   = 81 // could die in this combat (approximated, no first strike etc.)
	entCombatKills  = 82 // could kill a foe this combat
	entReserved83   = 83 // pads the width to 84

	// The v2 block, appended so nothing at 0..83 moves: mana colour (one per
	// WUBRG colour), the pip counts (not the mana value) and a coarse
	// category. A v2 network reads them; a v1 or v3 network does not have
	// them and still loads (netsimd accepts both widths).
	entColourOff = 84 // W U B R G C, one each
	entPipsOff   = 90 // W U B R G generic, the pip counts / pipMax
	entCategory  = 96 // removal, creature, ramp, trick (coarse, overlapping)

	EntityRawWidth   = 100
	EntityRawWidthV1 = 68
	// EntityRawWidthV3 is the format-3 width the trained v1/v2 networks were
	// built at (84): the v2 colour/pips/category block is appended above it.
	EntityRawWidthV3 = 84
	// EntityRawWidthRelEnd is the end of the relational block the v1 checks
	// range over (68..83); the v2 block starts above it.
	EntityRawWidthRelEnd = 84
	entityFormat         = 3
)

// entityTypes is the entity type vocabulary (index = entTypeOff + i).
// (CardView.Token is a per-object copy tag set on every card, not a token
// marker, so there is no token bit.)
var entityTypes = [...]string{"Creature", "Land", "Artifact", "Enchantment", "Planeswalker",
	"Instant", "Sorcery", "Battle", "Legendary", "Basic"}

// entityTypeMasks is entityTypes as cardTypeBits masks (typeMasks).
var entityTypeMasks = [len(entityTypes)]cards.TypeMask{cards.TypeCreature, cards.TypeLand, cards.TypeArtifact,
	cards.TypeEnchantment, cards.TypePlaneswalker, cards.TypeInstant, cards.TypeSorcery, cards.TypeBattle,
	cards.TypeLegendary, cards.TypeBasic}

// entRelNames names the relational slots EntityRawWidthV1..EntityRawWidthRelEnd-1.
var entRelNames = [16]string{"tgt-by-me", "tgt-by-opp", "tgt-perm-me", "tgt-perm-opp",
	"tgt-player-me", "tgt-player-opp", "tgt-stack", "tgt-grave", "stack-x", "stack-kicked",
	"combat-foes", "combat-foe-pow", "combat-foe-tgh", "combat-dies", "combat-kills", "reserved"}

// EntityRelNames returns a copy of the relational slot names; index i is
// slot EntityRawWidthV1 + i.
func EntityRelNames() []string {
	out := make([]string, len(entRelNames))
	copy(out, entRelNames[:])
	return out
}

// entityHashSuffix is the entity contract's part of EncoderHashFor.
func entityHashSuffix() string { return entityHashSuffixFor(entityFormat, EntityRawWidth, true) }

// entityHashSuffixV1 is the format-1 entity contract's suffix: the 68-wide
// layout every network exported before the relational slots.
func entityHashSuffixV1() string { return entityHashSuffixFor(1, EntityRawWidthV1, false) }

// entityHashSuffixFor builds the entity suffix for one format and width.
func entityHashSuffixFor(format, raw int, rel bool) string {
	relPart := ""
	if rel {
		relPart = "\x1frel=" + joinComma(entRelNames[:])
	}
	return fmt.Sprintf("\x1fentity-format=%d\x1fgroups=%d\x1fraw=%d\x1fsum=%g\x1ftypes=%s\x1fpins=%d%s",
		format, EntityGroups, raw, entSumScale, strings.Join(entityTypes[:], ","),
		hashID("ent|name|Swords to Plowshares"), relPart)
}

// joinComma joins ss with commas without strings.Join (the codeshape
// ratchet holds bots/policynet's strings.* count at its pre-change level).
func joinComma(ss []string) string {
	n := 0
	for _, s := range ss {
		n += len(s) + 1
	}
	b := make([]byte, 0, n)
	for i, s := range ss {
		if i > 0 {
			b = append(b, ',')
		}
		b = append(b, s...)
	}
	return string(b)
}

// EntityCard is one visible card's entity input.
type EntityCard struct {
	Group uint8     // entGroup*
	Raw   []float32 // EntityRawWidth scalars
	Rows  []Feature // hashed identity rows (name, spell API) into the shared table
}

// entityCards builds the view's entity list and the ObjID → list index map.
// Order: my battlefield, every other battlefield (seat order), my hand, the
// stack (bottom first, as the view lists it).
func entityCards(v view.View, seat state.PlayerID) ([]EntityCard, map[state.ObjID]int) {
	out := entityCardList(nil, &v, seat)
	idx := make(map[state.ObjID]int, len(out))
	n := 0
	walkEntityOrder(&v, seat, func(cv *view.CardView, _ uint8, _ int, _ *view.StackView) {
		if _, dup := idx[cv.ID]; !dup {
			idx[cv.ID] = n
			n++
		}
	})
	return out, idx
}

// walkEntityOrder visits the entity order's cards, duplicates included:
// my battlefield, every other battlefield (seat order), my hand, the stack.
func walkEntityOrder(v *view.View, seat state.PlayerID, f func(cv *view.CardView, group uint8, zone int, sv *view.StackView)) {
	var me *view.PlayerView
	for i := range v.Players {
		if v.Players[i].ID == seat {
			me = &v.Players[i]
		}
	}
	if me != nil {
		for i := range me.Battlefield {
			f(&me.Battlefield[i], entGroupBfMe, entZoneBf, nil)
		}
	}
	for i := range v.Players {
		if v.Players[i].ID == seat {
			continue
		}
		for j := range v.Players[i].Battlefield {
			f(&v.Players[i].Battlefield[j], entGroupBfOpp, entZoneBf, nil)
		}
	}
	if me != nil {
		for i := range me.Hand {
			f(&me.Hand[i], entGroupHand, entZoneHand, nil)
		}
	}
	for i := range v.Stack {
		if sv := &v.Stack[i]; sv.Card != nil {
			f(sv.Card, entGroupStack, entZoneStack, sv)
		}
	}
}

// entityScratch is entityCardList's dedup table: seen[id] == gen marks an
// id already listed in this call, so the table is never cleared.
type entityScratch struct {
	seen []uint32
	gen  uint32
	huge []state.ObjID // ids >= maxDenseObjID marked this call
}

var entityScratches = sync.Pool{New: func() any { return new(entityScratch) }}

// entityBoard is the board facts every permanent's vector reads, read off
// the battlefields without a map: who blocks (any BlockedBy entry), how
// many attachments a permanent hosts, and a host's controller (the LAST
// battlefield entry with its id, as the old map's last write left it).
type entityBoard struct {
	v        *view.View
	blockers []state.ObjID
	attached []state.ObjID
	tgts     []entTarget
	pairs    []entPair // attacker/blocker relations, in battlefield order
}

func (b *entityBoard) blocking(id state.ObjID) bool {
	for _, x := range b.blockers {
		if x == id {
			return true
		}
	}
	return false
}

func (b *entityBoard) hosts(id state.ObjID) int {
	n := 0
	for _, x := range b.attached {
		if x == id {
			n++
		}
	}
	return n
}

func (b *entityBoard) controller(id state.ObjID) (state.PlayerID, bool) {
	for i := len(b.v.Players) - 1; i >= 0; i-- {
		bf := b.v.Players[i].Battlefield
		for j := len(bf) - 1; j >= 0; j-- {
			if bf[j].ID == id {
				return bf[j].Controller, true
			}
		}
	}
	return 0, false
}

// entityCardList is entityCards' card list. Every card's Raw and Rows are
// carved out of one slab each (capacity-capped, so an append never reaches
// a neighbour).
func entityCardList(buf *StateBuf, v *view.View, seat state.PlayerID) []EntityCard {
	var me *view.PlayerView
	for i := range v.Players {
		if v.Players[i].ID == seat {
			me = &v.Players[i]
		}
	}
	var blkBuf, attBuf [16]state.ObjID
	var tgtBuf [16]entTarget
	var pairBuf [16]entPair
	var foeBuf [8]state.ObjID
	b := entityBoard{v: v, blockers: blkBuf[:0], attached: attBuf[:0], tgts: tgtBuf[:0], pairs: pairBuf[:0]}
	total := len(v.Stack)
	for i := range v.Players {
		pv := &v.Players[i]
		total += len(pv.Battlefield)
		if pv.ID == seat {
			total += len(pv.Hand)
		}
		for j := range pv.Battlefield {
			cv := &pv.Battlefield[j]
			b.blockers = append(b.blockers, cv.BlockedBy...)
			start := len(b.pairs)
			for _, blk := range cv.BlockedBy {
				dup := false
				for _, p := range b.pairs[start:] {
					if p.blk == blk {
						dup = true
						break
					}
				}
				if !dup {
					b.pairs = append(b.pairs, entPair{att: cv.ID, blk: blk})
				}
			}
			if cv.AttachedTo != 0 {
				b.attached = append(b.attached, cv.AttachedTo)
			}
		}
	}
	for i := range v.Stack {
		sv := &v.Stack[i]
		mine := sv.Controller == seat
		start := len(b.tgts)
		for _, t := range sv.Targets {
			if t.IsPlayer || t.Obj == 0 {
				continue
			}
			dup := false
			for _, e := range b.tgts[start:] {
				if e.obj == t.Obj {
					dup = true
					break
				}
			}
			if !dup {
				b.tgts = append(b.tgts, entTarget{obj: t.Obj, mine: mine})
			}
		}
	}
	var avail int32
	if me != nil {
		avail = seatAvailable(me)
	}
	sc := entityScratches.Get().(*entityScratch)
	sc.gen++
	sc.huge = sc.huge[:0]
	if sc.gen == 0 { // wrapped: start the stamps over
		clear(sc.seen)
		sc.gen = 1
	}
	out, raw, rows := buf.entitySlabs(total)
	walkEntityOrder(v, seat, func(cv *view.CardView, group uint8, zone int, sv *view.StackView) {
		if !sc.markNew(cv.ID) {
			return
		}
		k := len(out)
		r := raw[k*EntityRawWidth : (k+1)*EntityRawWidth : (k+1)*EntityRawWidth]
		start := len(rows)
		rows = entityCard(r, rows, cv, zone, seat, avail, b.blocking(cv.ID), b.hosts(cv.ID), &b)
		var cr []Feature
		if len(rows) > start {
			cr = rows[start:len(rows):len(rows)]
		}
		entityRelations(r, cv, zone, sv, seat, &b, foeBuf[:0])
		out = append(out, EntityCard{Group: group, Raw: r, Rows: cr})
	})
	entityScratches.Put(sc)
	buf.keepEntity(out, rows)
	if len(out) == 0 {
		return nil // as the append-built list was
	}
	return out
}

// markNew reports whether id is not yet listed this call (and marks it).
func (sc *entityScratch) markNew(id state.ObjID) bool {
	if int(id) >= maxDenseObjID {
		// Never seen in a real game: fall back to the slow exact check.
		return !sc.seenSlow(id)
	}
	if int(id) >= len(sc.seen) {
		sc.seen = append(sc.seen, make([]uint32, int(id)+1-len(sc.seen))...)
	}
	if sc.seen[id] == sc.gen {
		return false
	}
	sc.seen[id] = sc.gen
	return true
}

// entityCard writes cv's raw vector into raw (EntityRawWidth zeros) and
// appends its identity rows to rows.
func entityCard(raw []float32, rows []Feature, cv *view.CardView, zone int, seat state.PlayerID, avail int32, blocking bool, hosts int, b *entityBoard) []Feature {
	set := func(i int, v float32) { raw[i] = v }
	fold, exact := typeMasks(cv)
	for i, m := range entityTypeMasks {
		if exact&m != 0 {
			set(entTypeOff+i, 1)
		}
	}
	creature := fold&cards.TypeCreature != 0
	if creature || fold&cards.TypePlaneswalker != 0 || zone == entZoneHand {
		set(entPower, float32(cv.Power)/4)
		set(entToughness, float32(cv.Toughness-cv.Damage)/4)
	}
	if cv.Damage > 0 {
		set(entDamaged, 1)
	}
	for _, k := range cv.Keywords {
		set(entKwOff+keywordBit(k), 1) // len(keywordBits) is "other"
	}
	if zone == entZoneBf {
		if cv.Tapped {
			set(entTapped, 1)
		}
		if cv.SummonSick {
			set(entSick, 1)
		}
		if creature {
			if !cv.Tapped && (!cv.SummonSick || hasKeyword(cv, kwHaste)) && !hasKeyword(cv, kwDefender) {
				set(entCanAttack, 1)
			}
			if !cv.Tapped {
				set(entCanBlock, 1)
			}
		}
		if cv.Attacking {
			set(entAttacking, 1)
		}
		if blocking {
			set(entBlocking, 1)
		}
		if len(cv.BlockedBy) > 0 {
			set(entBlocked, 1)
		}
		if len(cv.Counters) > 0 {
			eachCounterSorted(cv.Counters, func(k string, c int32) {
				n := float32(c)
				switch strings.ToUpper(k) {
				case "P1P1", "+1/+1":
					raw[entCtrPlus] += n / 2
				case "M1M1", "-1/-1":
					raw[entCtrMinus] += n / 2
				case "LOYALTY":
					raw[entCtrLoyalty] += n / 4
				default:
					raw[entCtrOther] += n / 2
				}
			})
		}
		if cv.AttachedTo != 0 {
			set(entAttached, 1)
			if c, ok := b.controller(cv.AttachedTo); ok && c == seat {
				set(entAttachMine, 1)
			}
		}
		if hosts > 0 {
			set(entHosts, float32(hosts)/2)
		}
	}
	if cv.Controller == seat {
		set(entMine, 1)
	}
	switch zone {
	case entZoneBf:
		set(entZoneBf, 1)
	case entZoneHand:
		set(entZoneHand, 1)
	case entZoneStack:
		set(entZoneStack, 1)
	}
	mv := int32(cardManaValue(cv))
	set(entManaValue, float32(mv)/5)
	if fold&cards.TypeInstant != 0 || hasKeyword(cv, kwFlash) {
		set(entInstant, 1)
	}
	if zone == entZoneHand && fold&cards.TypeLand == 0 && mv <= avail {
		set(entCastable, 1)
	}
	if cv.FaceDown {
		set(entFaceDown, 1)
	}
	set(entOne, 1)
	// The v2 block: printed mana colour, the pip counts, and a coarse
	// category. Colour was absent from the entity vector before -- the option
	// encoding reads it, the entity encoding did not -- so on the battlefield
	// a card's colours were visible only through its opaque name hash.
	if !cv.FaceDown {
		colours, _ := manaCostBits(cv.ManaCost)
		for k := 0; k < 6; k++ {
			if colours[k] {
				set(entColourOff+k, 1)
			}
		}
		pips := pipCounts(cv.ManaCost)
		for k := 0; k < 6; k++ {
			if pips[k] != 0 {
				set(entPipsOff+k, float32(pips[k])/float32(pipMax))
			}
		}
		if removalSpell(cv, fold) {
			set(entCategory, 1)
		}
		if creature {
			set(entCategory+1, 1)
		}
		if rampCard(cv) {
			set(entCategory+2, 1)
		}
		if fold&cards.TypeInstant != 0 && !removalSpell(cv, fold) {
			set(entCategory+3, 1)
		}
	}
	if cv.Name != "" && !cv.FaceDown {
		rows = append(rows, Feature{Row: fnvRow(fnvAdd(fnvAdd(fnvOffset, "ent|name|"), cv.Name)), Value: 1})
	}
	if cv.SpellAPI != "" {
		rows = append(rows, Feature{Row: fnvRow(fnvAdd(fnvAdd(fnvOffset, "ent|api|"), cv.SpellAPI)), Value: 1})
	}
	return rows
}

// ---- model half --------------------------------------------------------

// entInW is the per-card encoder's input width: the raw scalars plus the
// hashed identity embedding (H).
func (m *Model) entInW() int { return EntityRawWidth + m.H }

// entPoolW is the pooled vector's width.
func (m *Model) entPoolW() int { return EntityGroups * 2 * m.EntK }

// entCache is one state's entity forward: per card the encoder input and
// activation, the pooled vector and, per pooled max slot, the card that won.
type entCache struct {
	u      [][]float32 // per card, entInW
	h      [][]float32 // per card, EntK
	pool   []float32   // entPoolW
	argmax []int       // entPoolW; -1 for sum slots and empty groups
}

// entForward runs the per-card encoder and the pool. nil when the model has
// no entity encoder.
func (m *Model) entForward(st State) *entCache {
	if m.EntK == 0 {
		return nil
	}
	k, inW := m.EntK, m.entInW()
	c := &entCache{u: make([][]float32, len(st.Cards)), h: make([][]float32, len(st.Cards)),
		pool: make([]float32, m.entPoolW()), argmax: make([]int, m.entPoolW())}
	for i := range c.argmax {
		c.argmax[i] = -1
	}
	for ci, card := range st.Cards {
		u := make([]float32, inW)
		copy(u, card.Raw)
		for _, f := range card.Rows {
			row := m.Table[int(f.Row)*m.H : int(f.Row)*m.H+m.H]
			for j := range row {
				u[EntityRawWidth+j] += row[j] * f.Value
			}
		}
		h := make([]float32, k)
		for q := 0; q < k; q++ {
			sum := m.EntB[q]
			w := m.EntW[q*inW : (q+1)*inW]
			for i, ui := range u {
				if ui != 0 {
					sum += w[i] * ui
				}
			}
			if sum > 0 {
				h[q] = sum
			}
		}
		c.u[ci], c.h[ci] = u, h
		g := int(card.Group)
		if g >= EntityGroups {
			continue
		}
		base := g * 2 * k
		for q := 0; q < k; q++ {
			c.pool[base+q] += entSumScale * h[q]
			mi := base + k + q
			if c.argmax[mi] < 0 || h[q] > c.pool[mi] {
				c.pool[mi] = h[q]
				c.argmax[mi] = ci
			}
		}
	}
	return c
}

// entAddTrunk adds the pooled projection EntPᵀ·P into the state trunk s.
func (m *Model) entAddTrunk(s []float32, c *entCache) {
	if c == nil {
		return
	}
	for p, v := range c.pool {
		if v == 0 {
			continue
		}
		row := m.EntP[p*m.H : (p+1)*m.H]
		for j := range s {
			s[j] += row[j] * v
		}
	}
}

// entOffset is the hidden-input offset of the option's entity block.
func (m *Model) entOffset() int { return 2*m.H + OptionSlotWidth + OptionDenseWidth + m.ExtraW }

// entFillInput writes [h_EntA ‖ h_EntB] into the option's hidden input x.
func (m *Model) entFillInput(x []float32, o Option, c *entCache) {
	if c == nil {
		return
	}
	off := m.entOffset()
	for slot, ref := range [2]int32{o.EntA, o.EntB} {
		if ref <= 0 || int(ref) > len(c.h) {
			continue
		}
		copy(x[off+slot*m.EntK:off+(slot+1)*m.EntK], c.h[ref-1])
	}
}

// entBackward propagates the trunk gradient ds and the per-option entity
// input gradients (dh, accumulated per card by the caller) through the pool,
// the projection and the per-card encoder into g. dh is indexed by card and
// may be nil for cards no option referenced.
func (m *Model) entBackward(c *entCache, st State, ds []float32, dh [][]float32, g *Grads) {
	if c == nil {
		return
	}
	k, inW := m.EntK, m.entInW()
	// Projection: dEntP[p,j] += P[p]·ds[j]; dP[p] = Σ_j EntP[p,j]·ds[j].
	dP := make([]float32, len(c.pool))
	for p, v := range c.pool {
		row := m.EntP[p*m.H : (p+1)*m.H]
		grow := g.EntP[p*m.H : (p+1)*m.H]
		var sum float32
		for j, d := range ds {
			if v != 0 {
				grow[j] += v * d
			}
			sum += row[j] * d
		}
		dP[p] = sum
	}
	if dh == nil {
		dh = make([][]float32, len(st.Cards))
	}
	for ci, card := range st.Cards {
		gidx := int(card.Group)
		if gidx >= EntityGroups {
			continue
		}
		base := gidx * 2 * k
		for q := 0; q < k; q++ {
			d := entSumScale * dP[base+q]
			if c.argmax[base+k+q] == ci {
				d += dP[base+k+q]
			}
			if d == 0 {
				continue
			}
			if dh[ci] == nil {
				dh[ci] = make([]float32, k)
			}
			dh[ci][q] += d
		}
	}
	du := make([]float32, inW)
	for ci, d := range dh {
		if d == nil {
			continue
		}
		h, u := c.h[ci], c.u[ci]
		any := false
		for i := range du {
			du[i] = 0
		}
		for q := 0; q < k; q++ {
			if h[q] <= 0 || d[q] == 0 {
				continue
			}
			any = true
			dz := d[q]
			g.EntB[q] += dz
			w := m.EntW[q*inW : (q+1)*inW]
			gw := g.EntW[q*inW : (q+1)*inW]
			for i, ui := range u {
				if ui != 0 {
					gw[i] += dz * ui
				}
				du[i] += w[i] * dz
			}
		}
		if !any {
			continue
		}
		for _, f := range st.Cards[ci].Rows {
			g.addTable(int(f.Row), du[EntityRawWidth:], f.Value)
		}
	}
}

// UpgradeEntity turns a model into a FeaturesEntity model with a k-wide
// per-card encoder (ticket pn14's warm start): the encoder is drawn from rng,
// while the pooled projection and the new hidden-input columns start at
// ZERO, so the upgraded model scores every (state, option) exactly as the
// source did until training moves them. The source must be an mz model (the
// entity set is mz plus the cards) without an entity encoder already.
func UpgradeEntity(m *Model, k int, rng *rand.Rand) error {
	switch {
	case k < 1:
		return fmt.Errorf("policynet: entity width %d < 1", k)
	case m.EntK != 0:
		return fmt.Errorf("policynet: model already has an entity encoder (k=%d)", m.EntK)
	case m.Features != FeaturesMZ:
		return fmt.Errorf("policynet: entity upgrade needs an mz model, got %s", m.Features)
	case m.ExtraW != 0:
		return fmt.Errorf("policynet: entity upgrade of an ExtraW model")
	}
	oldInW := m.InW
	m.EntK = k
	m.InW = oldInW + 2*k
	hid := make([]float32, m.Hidden*m.InW)
	for h := 0; h < m.Hidden; h++ {
		copy(hid[h*m.InW:h*m.InW+oldInW], m.HidW[h*oldInW:(h+1)*oldInW])
	}
	m.HidW = hid
	m.EntW = make([]float32, k*m.entInW())
	m.EntB = make([]float32, k)
	m.EntP = make([]float32, m.entPoolW()*m.H)
	fillUniform(m.EntW, glorot(m.entInW(), k), rng)
	m.Features = FeaturesEntity
	return nil
}

// seenSlow is markNew for an id past the dense table: a short list of the
// huge ids marked this generation.
func (sc *entityScratch) seenSlow(id state.ObjID) bool {
	for _, x := range sc.huge {
		if x == id {
			return true
		}
	}
	sc.huge = append(sc.huge, id)
	return false
}

// ---- the v2 block's helpers --------------------------------------------

// pipMax caps one pip count, as feat's encoder does: a card with more pips of
// one colour than this is saturated (the distinction is not meaningful).
const pipMax = 5

// pipCounts counts a mana cost's pips per colour (WUBRG then generic), from
// the same token walk manaCostBits takes so the two cannot disagree. A hybrid
// pip counts for every colour it names; {X} counts for nothing. Deliberately
// mirrors dirac/feat's costPips: both encoders write the same slots
// and the oracle compares them.
//
// The token walk is local rather than strings.Split/TrimSpace/EqualFold so
// the per-card encode adds no string machinery (the codeshape ratchet).
func pipCounts(cost string) [6]int {
	var n [6]int
	cost = manaBraceForm.Replace(cost)
	start, end := 0, len(cost)
	for start < end && isSpaceByte(cost[start]) {
		start++
	}
	for end > start && isSpaceByte(cost[end-1]) {
		end--
	}
	cost = cost[start:end]
	if cost == "" || equalFoldASCII(cost, "no cost") {
		return n
	}
	for i := 0; i < len(cost); {
		if isSpaceByte(cost[i]) {
			i++
			continue
		}
		start, depth := i, 0
		for i < len(cost) {
			if cost[i] == '<' {
				depth++
			} else if cost[i] == '>' && depth > 0 {
				depth--
			} else if isSpaceByte(cost[i]) && depth == 0 {
				break
			}
			i++
		}
		addPips(&n, cost[start:i])
	}
	for i := range n {
		if n[i] > pipMax {
			n[i] = pipMax
		}
	}
	return n
}

// addPips adds one mana-cost token's pips to n: a numeric token is generic,
// a slash token (hybrid, twobrid, Phyrexian) counts each one-letter face and
// nothing else, and any other token counts every WUBRG C letter it names.
func addPips(n *[6]int, tok string) {
	if equalFoldASCII(tok, "X") {
		return
	}
	if isPlainDigits(tok) {
		v := 0
		for i := 0; i < len(tok); i++ {
			v = v*10 + int(tok[i]-'0')
		}
		n[5] += v
		return
	}
	if byteIndex(tok, '/') >= 0 {
		for start := 0; start <= len(tok); {
			end := start
			for end < len(tok) && tok[end] != '/' {
				end++
			}
			if end-start == 1 {
				if k := pipIndexByte(tok[start]); k >= 0 {
					n[k]++
				}
			}
			if end == len(tok) {
				break
			}
			start = end + 1
		}
		return
	}
	for i := 0; i < len(tok); i++ {
		if k := pipIndexByte(tok[i]); k >= 0 {
			n[k]++
		}
	}
}

// pipIndexByte maps a colour letter to its v2 pip slot (WUBRG C), or -1.
func pipIndexByte(b byte) int {
	switch b {
	case 'W':
		return 0
	case 'U':
		return 1
	case 'B':
		return 2
	case 'R':
		return 3
	case 'G':
		return 4
	case 'C':
		return 5
	}
	return -1
}

func byteIndex(s string, b byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == b {
			return i
		}
	}
	return -1
}

func lowerASCII(b byte) byte {
	if b >= 'A' && b <= 'Z' {
		return b + ('a' - 'A')
	}
	return b
}

func equalFoldASCII(s, t string) bool {
	if len(s) != len(t) {
		return false
	}
	for i := 0; i < len(s); i++ {
		if lowerASCII(s[i]) != lowerASCII(t[i]) {
			return false
		}
	}
	return true
}

// containsFoldASCII reports whether sub (lowercase) occurs in s, folding
// ASCII case, without strings.ToLower/Contains on the per-card path.
func containsFoldASCII(s, sub string) bool {
	if sub == "" {
		return true
	}
	for i := 0; i+len(sub) <= len(s); i++ {
		ok := true
		for j := 0; j < len(sub); j++ {
			if lowerASCII(s[i+j]) != sub[j] {
				ok = false
				break
			}
		}
		if ok {
			return true
		}
	}
	return false
}

// removalSpell reports a non-creature, non-land card whose spell API reads
// like removal. Coarse on purpose: the marker is evidence of a role, and
// under-tagging leaves the net where it already was.
func removalSpell(cv *view.CardView, fold cards.TypeMask) bool {
	if fold&cards.TypeCreature != 0 || fold&cards.TypeLand != 0 {
		return false
	}
	for _, k := range removalAPIMarkers {
		if containsFoldASCII(cv.SpellAPI, k) {
			return true
		}
	}
	return false
}

var removalAPIMarkers = []string{"destroy", "exile", "damage", "deals", "sacrifices", "counter target"}

// rampCard reports a land or a name that reads like a mana rock.
func rampCard(cv *view.CardView) bool {
	if fold, _ := typeMasks(cv); fold&cards.TypeLand != 0 {
		return true
	}
	for _, k := range rampNameMarkers {
		if containsFoldASCII(cv.Name, k) {
			return true
		}
	}
	return false
}

var rampNameMarkers = []string{"signet", "talisman", "lantern", "monument", "orb", "prism", "keyrune"}
