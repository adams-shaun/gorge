package policynet

import (
	"fmt"
	"math/rand/v2"
	"sort"
	"strings"

	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/state"
	"github.com/adams-shaun/gorge/view"
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

	EntityRawWidth = 68
)

// entityTypes is the entity type vocabulary (index = entTypeOff + i).
// (CardView.Token is a per-object copy tag set on every card, not a token
// marker, so there is no token bit.)
var entityTypes = [...]string{"Creature", "Land", "Artifact", "Enchantment", "Planeswalker",
	"Instant", "Sorcery", "Battle", "Legendary", "Basic"}

// entityHashSuffix is the entity contract's part of EncoderHashFor.
func entityHashSuffix() string {
	return fmt.Sprintf("\x1fentity-format=1\x1fgroups=%d\x1fraw=%d\x1fsum=%g\x1ftypes=%s\x1fpins=%d",
		EntityGroups, EntityRawWidth, entSumScale, strings.Join(entityTypes[:], ","), hashID("ent|name|Swords to Plowshares"))
}

// EntityCard is one visible card's entity input.
type EntityCard struct {
	Group uint8     // entGroup*
	Raw   []float32 // EntityRawWidth scalars
	Rows  []Feature // hashed identity rows (name, spell API) into the shared table
}

// entityCards builds the view's entity list and the ObjID → list index map.
// Order: my battlefield, every other battlefield (seat order), my hand, the
// stack (top first, as the view lists it).
func entityCards(v view.View, seat state.PlayerID) ([]EntityCard, map[state.ObjID]int) {
	var me *view.PlayerView
	for i := range v.Players {
		if v.Players[i].ID == seat {
			me = &v.Players[i]
		}
	}
	// Board facts every permanent's vector reads: who blocks, who hosts
	// attachments, who controls the host.
	blocking := map[state.ObjID]bool{}
	hosts := map[state.ObjID]int{}
	ctrl := map[state.ObjID]state.PlayerID{}
	for i := range v.Players {
		for _, cv := range v.Players[i].Battlefield {
			ctrl[cv.ID] = cv.Controller
			for _, b := range cv.BlockedBy {
				blocking[b] = true
			}
			if cv.AttachedTo != 0 {
				hosts[cv.AttachedTo]++
			}
		}
	}
	var avail int32
	if me != nil {
		avail = seatAvailable(me)
	}
	var out []EntityCard
	idx := map[state.ObjID]int{}
	add := func(cv view.CardView, group uint8, zone int) {
		if _, dup := idx[cv.ID]; dup {
			return
		}
		idx[cv.ID] = len(out)
		out = append(out, entityCard(cv, group, zone, seat, avail, blocking[cv.ID], hosts[cv.ID], ctrl))
	}
	if me != nil {
		for _, cv := range me.Battlefield {
			add(cv, entGroupBfMe, entZoneBf)
		}
	}
	for i := range v.Players {
		if v.Players[i].ID == seat {
			continue
		}
		for _, cv := range v.Players[i].Battlefield {
			add(cv, entGroupBfOpp, entZoneBf)
		}
	}
	if me != nil {
		for _, cv := range me.Hand {
			add(cv, entGroupHand, entZoneHand)
		}
	}
	for i := range v.Stack {
		if sv := &v.Stack[i]; sv.Card != nil {
			add(*sv.Card, entGroupStack, entZoneStack)
		}
	}
	return out, idx
}

func entityCard(cv view.CardView, group uint8, zone int, seat state.PlayerID, avail int32, blocking bool, hosts int, ctrl map[state.ObjID]state.PlayerID) EntityCard {
	raw := make([]float32, EntityRawWidth)
	set := func(i int, v float32) { raw[i] = v }
	types := strings.Fields(cv.Types)
	for i, t := range entityTypes {
		for _, w := range types {
			if w == t {
				set(entTypeOff+i, 1)
			}
		}
	}
	creature := hasTypeWord(cv.Types, "Creature")
	if creature || hasTypeWord(cv.Types, "Planeswalker") || zone == entZoneHand {
		set(entPower, float32(cv.Power)/4)
		set(entToughness, float32(cv.Toughness-cv.Damage)/4)
	}
	if cv.Damage > 0 {
		set(entDamaged, 1)
	}
	for _, k := range cv.Keywords {
		hit := false
		for i, w := range keywordBits {
			if strings.EqualFold(k, w) {
				set(entKwOff+i, 1)
				hit = true
				break
			}
		}
		if !hit {
			set(entKwOff+len(keywordBits), 1)
		}
	}
	if zone == entZoneBf {
		if cv.Tapped {
			set(entTapped, 1)
		}
		if cv.SummonSick {
			set(entSick, 1)
		}
		if creature {
			if !cv.Tapped && (!cv.SummonSick || hasKeyword(cv, "Haste")) && !hasKeyword(cv, "Defender") {
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
		keys := make([]string, 0, len(cv.Counters))
		for k := range cv.Counters {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			n := float32(cv.Counters[k])
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
		}
		if cv.AttachedTo != 0 {
			set(entAttached, 1)
			if c, ok := ctrl[cv.AttachedTo]; ok && c == seat {
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
	mv := int32(mvOf(cv.ManaCost))
	set(entManaValue, float32(mv)/5)
	if hasTypeWord(cv.Types, "Instant") || hasKeyword(cv, "Flash") {
		set(entInstant, 1)
	}
	if zone == entZoneHand && !hasTypeWord(cv.Types, "Land") && mv <= avail {
		set(entCastable, 1)
	}
	if cv.FaceDown {
		set(entFaceDown, 1)
	}
	set(entOne, 1)
	var rows []Feature
	if cv.Name != "" && !cv.FaceDown {
		rows = append(rows, Feature{Row: hashID("ent|name|" + cv.Name), Value: 1})
	}
	if cv.SpellAPI != "" {
		rows = append(rows, Feature{Row: hashID("ent|api|" + cv.SpellAPI), Value: 1})
	}
	return EntityCard{Group: group, Raw: raw, Rows: rows}
}

// entityOptionRefs resolves an option's entity references (Option.EntA/B):
// its own card and its related card, as 1 + State.Cards index (0 = none).
func entityOptionRefs(v view.View, seat state.PlayerID, o decision.Option) (int32, int32) {
	_, idx := entityCards(v, seat)
	ref := func(id state.ObjID) int32 {
		if id == 0 {
			return 0
		}
		if i, ok := idx[id]; ok {
			return int32(i + 1)
		}
		return 0
	}
	rel := o.Attacker
	if rel == 0 {
		rel = o.Battle
	}
	return ref(o.Obj), ref(rel)
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
