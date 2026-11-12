package cardset

import (
	"fmt"
	"slices"
	"sort"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// AbsentID is a CardID whose card left the pool (ids.tsv keeps it).
type AbsentID struct {
	ID    uint16
	Name  string
	Token bool
}

// AbSubs is one static ability's granted subtypes (pool subtype indexes).
type AbSubs struct {
	Ab   int
	Subs []uint8
}

// OvSubs is an override's subtype sets (pool subtype indexes).
type OvSubs struct {
	Set, Add, Keep []uint8
}

// LayerEntry is a pool index with an order-sensitive layer effect and its
// classes (dirac/kgen layerguard.go: the interim CR 613 guard).
type LayerEntry struct {
	I int
	C LayerClasses
}

// PoolData is one pool as plain data, what a binary table holds. Filters, Exprs,
// Conds, Overrides, Emblems and LayerConflicts are never nil; the per-card
// CardRefSubs, AbilityOrigin, StaticSubs and CardTokens entries and OverrideSubs'
// Set and Add are nil when empty. The codec keeps the difference.
type PoolData struct {
	Name                string
	NumCards, NumTokens int
	NumIDs              int
	Names               []string
	PoolID              []uint16
	Absent              []AbsentID
	Status              []Status
	Reason              []string      // why an entry is not StatusSupported; "" when it is
	Defs                []kop.CardDef // Name blank: Names holds them
	CardSubs            []kop.SubMask
	CardTokens          [][]uint16
	CardRefSubs         [][]uint8
	AbilityOrigin       [][]uint8
	CreatureTypes       []string
	StaticSubs          [][]AbSubs
	OverrideSubs        []OvSubs
	Filters             []kop.Filter
	Exprs               []kop.Expr
	Conds               []kop.Cond
	Overrides           []kop.Override
	Emblems             []kop.Emblem
	Subtypes            []string
	LayerCls            []LayerEntry
	LayerConflicts      []LayerConflict
	RoomCards           []int
	NameReaders         []int
	AbReducers          []int
	AbReducerFloor      []uint8  // AbReducers' MinMana$ floors, 0 none
	AbReducerTypes      []uint16 // AbReducers' filter card types, 0 any
	AbManaCosted        []int
	AbManaCostedMV      []uint8  // AbManaCosted's dearest mana-ability mana cost
	AbManaCostedTypes   []uint16 // AbManaCosted's card types
}

// NoSubtype is the matchup filter sentinel for a subtype no object in the
// matchup can have (Filter.Sub/NotSub; it is beyond len(Table.Subtypes)).
const NoSubtype = 255

// dataSet is a Set over a PoolData. plays is the view: which entries Matchup and
// Pool play, indexed like d.Status.
type dataSet struct {
	d     *PoolData
	plays []bool
	// nameInert marks the pool-table cards that choose a card name when the
	// pool's table cannot hold the choice (pooldata_names.go poolNamesFit):
	// Pool holds them as inert fallbacks and Inert says so. A matchup table
	// is unaffected (it compacts its CardIDs). nil when none.
	nameInert []bool
}

var _ Set = (*dataSet)(nil)

// NewDataSet returns the Set over d, playing what a game may play. The set keeps d
// and never writes to it.
func NewDataSet(d *PoolData) Set { return viewOf(d, gated(EnforceEvidence)) }

func (s dataSet) Name() string { return s.d.Name }

func (s dataSet) Lookup(name string) (int, bool) {
	d := s.d
	i := sort.Search(d.NumCards, func(i int) bool { return d.Names[i] >= name })
	if i < d.NumCards && d.Names[i] == name {
		return i, true
	}
	return 0, false
}

func (s dataSet) Supported() []bool { return s.plays }

func (s dataSet) Status(i int) Status { return s.d.Status[i] }

func (s dataSet) Reason(i int) string { return s.d.Reason[i] }

func (s dataSet) AbilityOrigin(i, ab int) uint8 {
	if ab >= len(s.d.AbilityOrigin[i]) {
		return 0xFF
	}
	return s.d.AbilityOrigin[i][ab]
}

func (s dataSet) Matchup(a, b []Entry) (*kop.Table, []kop.CardID, []kop.CardID, error) {
	t, ids, err := s.matchup([][]Entry{a, b}, false)
	if err != nil {
		return nil, nil, nil, err
	}
	return t, ids[0], ids[1], nil
}

func (s dataSet) MatchupN(decks [][]Entry) (*kop.Table, [][]kop.CardID, error) {
	return s.matchup(decks, false)
}

func (s dataSet) Pool() (*kop.Table, error) {
	all := make([]Entry, s.d.NumCards)
	for i := range all {
		all[i] = Entry{Name: s.d.Names[i], Count: 1}
	}
	t, _, err := s.matchup([][]Entry{all, nil}, true)
	return t, err
}

func (s dataSet) Inert(i int) bool {
	return i < s.d.NumCards && (!s.plays[i] || s.nameInert != nil && s.nameInert[i])
}

func (s dataSet) PoolID(i int) kop.CardID { return kop.CardID(s.d.PoolID[i]) }

func (s dataSet) InertID(id kop.CardID) bool {
	for i := range s.d.PoolID {
		if s.d.PoolID[i] == uint16(id) {
			return s.Inert(i)
		}
	}
	return int(id) < s.d.NumIDs
}

func (s dataSet) NumIDs() int        { return s.d.NumIDs }
func (s dataSet) Subtypes() []string { return s.d.Subtypes }
func (s dataSet) Names() []string    { return s.d.Names }
func (s dataSet) NumCards() int      { return s.d.NumCards }
func (s dataSet) NumTokens() int     { return s.d.NumTokens }

func (s dataSet) Def(i int) *kop.CardDef { return &s.d.Defs[i] }

// inertDef is an unsupported card's vanilla shape.
func inertDef(d kop.CardDef) kop.CardDef {
	return kop.CardDef{Name: d.Name, Cost: d.Cost, Types: d.Types, Subtypes: d.Subtypes, Colors: d.Colors,
		Power: d.Power, Toughness: d.Toughness, Loyalty: d.Loyalty, Enchant: d.Enchant}
}

// layerGuard refuses a matchup pool (sorted pool indexes) holding two
// cards whose layer effects the kernel's fixed order can mis-order.
func (s dataSet) layerGuard(pool []int) error {
	d := s.d
	var names []string
	var cls []LayerClasses
	for _, e := range d.LayerCls {
		for _, i := range pool {
			if i == e.I {
				names = append(names, d.Names[i])
				cls = append(cls, e.C)
				break
			}
		}
	}
	if len(names) < 2 {
		return nil
	}
	return LayerRefusal(names, cls, d.LayerConflicts)
}

// roomNameGuard refuses a matchup pool holding both a two-door Room and a
// card that reads a name (RoomNameRefusal).
func (s dataSet) roomNameGuard(pool []int) error {
	d := s.d
	var rooms, readers []string
	for _, i := range pool {
		for _, r := range d.RoomCards {
			if i == r {
				rooms = append(rooms, d.Names[i])
			}
		}
		for _, r := range d.NameReaders {
			if i == r {
				readers = append(readers, d.Names[i])
			}
		}
	}
	return RoomNameRefusal(rooms, readers)
}

// abReduceGuard refuses a matchup pool holding an activated-ability cost
// reducer and a mana ability whose mana cost the reducer would lower
// (AbReduceRefusal).
func (s dataSet) abReduceGuard(pool []int) error {
	d := s.d
	var reducers, costed []string
	for _, i := range pool {
		for k, r := range d.AbManaCosted {
			if i != r {
				continue
			}
			for j, rd := range d.AbReducers {
				if !slices.Contains(pool, rd) || int(d.AbManaCostedMV[k]) <= int(d.AbReducerFloor[j]) ||
					d.AbReducerTypes[j] != 0 && d.AbReducerTypes[j]&d.AbManaCostedTypes[k] == 0 {
					continue
				}
				costed = append(costed, d.Names[i])
				reducers = append(reducers, d.Names[rd])
			}
		}
	}
	return AbReduceRefusal(reducers, costed)
}

func (s dataSet) matchup(decks [][]Entry, inert bool) (*kop.Table, [][]kop.CardID, error) {
	dd := s.d
	var pool []int
	seen := map[int]bool{}
	var add func(i int)
	add = func(i int) {
		if seen[i] {
			return
		}
		seen[i] = true
		pool = append(pool, i)
		for _, t := range dd.CardTokens[i] {
			add(int(t))
		}
	}
	for _, d := range decks {
		for _, e := range d {
			i, ok := s.Lookup(e.Name)
			if !ok {
				return nil, nil, fmt.Errorf("card %q is not in the pool", e.Name)
			}
			if !s.plays[i] && !inert {
				return nil, nil, &RefusedError{Card: e.Name, Status: dd.Status[i], Reason: dd.Reason[i]}
			}
			add(i)
		}
	}
	sort.Ints(pool)
	if !inert {
		if err := s.layerGuard(pool); err != nil {
			return nil, nil, err
		}
		if err := s.roomNameGuard(pool); err != nil {
			return nil, nil, err
		}
		if err := s.abReduceGuard(pool); err != nil {
			return nil, nil, err
		}
	}
	if len(pool) >= int(kop.NoCard) {
		return nil, nil, fmt.Errorf("matchup needs %d cards", len(pool))
	}
	id := map[int]kop.CardID{}
	for j, i := range pool {
		id[i] = kop.CardID(j)
		if inert {
			id[i] = kop.CardID(dd.PoolID[i]) // the pool table's stable CardIDs
		}
	}
	// The matchup's subtypes are the rules-relevant ones: every subtype a
	// filter of an included card names and every subtype an included
	// card's effect grants. Printed subtypes nothing reads are dropped from
	// the masks -- unless a card chooses a creature type, when every
	// printed subtype is kept (any of them may be chosen), or the kernel
	// reads one by name (Role below, Army for amass).
	var subSet kop.SubMask
	mark := func(s uint8) { subSet = subSet.With(int(s)) }
	choosesType := false
	usesOverride := map[int]bool{}
	for _, i := range pool {
		for _, s := range dd.CardRefSubs[i] {
			mark(s)
		}
		for _, as := range dd.StaticSubs[i] {
			for _, s := range as.Subs {
				mark(s)
			}
		}
		for _, ab := range dd.Defs[i].Abilities {
			if ab.Static.Override > 0 {
				usesOverride[int(ab.Static.Override)-1] = true
			}
			for ti := 0; ti < int(ab.NTargets); ti++ {
				if ab.Targets[ti].TFlags&kop.TgfSharedCrType != 0 {
					choosesType = true // a shared creature type slot reads every printed one
				}
			}
			for _, in := range ab.Prog {
				switch in.Op {
				case kop.OpChooseType:
					choosesType = true
				case kop.OpOverride, kop.OpOverrideAll:
					// (OpOverrideAll: gorge's AnimateAll type grants are compared
					// by the projection too -- Breach the Multiverse's Phyrexian)
					usesOverride[int(in.C)] = true
				case kop.OpEmpower:
					if ov := int(uint16(in.B.N) & 0xfff); ov > 0 {
						usesOverride[ov-1] = true // the Jace subtype grant
					}
				case kop.OpAmass:
					// The "also a <Type>" grant rides B.N, exactly like Empower's;
					// without this the override's AddSub is masked to zero.
					if ov := int(uint16(in.B.N) & 0xfff); ov > 0 {
						usesOverride[ov-1] = true
					}
					// The kernel's amass finds the Army by name in Table.Subtypes and
					// mints the token at C; keep the token's printed subtypes (Army +
					// its Type$ word) so that lookup and the CardDef.Subtypes.Has(army)
					// check both work.
					if x := int(in.C); x >= 0 && x < len(dd.CardSubs) {
						subSet = subSet.Or(dd.CardSubs[x])
					}
				case kop.OpCopyAsTokenEx:
					if in.C > 0 {
						usesOverride[int(in.C)-1] = true // the copy exceptions
					}
				case kop.OpExt:
					if x := kop.ExtOp(in.A.N); (x == kop.ExtCopyAllEach || x == kop.ExtCopyExiledWith || x == kop.ExtCloneCopy || x == kop.ExtCopyPicked) && in.C > 0 {
						usesOverride[int(in.C)-1] = true // the copy exceptions
					} else if x == kop.ExtCopyNamed && uint16(in.C)>>kop.CopyNamedExcShift > 0 {
						usesOverride[int(uint16(in.C)>>kop.CopyNamedExcShift)-1] = true
					}
				case kop.OpBecomeCopy:
					if x := in.C >> 1; x > 0 {
						usesOverride[int(x)-1] = true
					}
				}
			}
		}
	}
	for o := range usesOverride {
		for _, s := range append(append([]uint8{}, dd.OverrideSubs[o].Set...), dd.OverrideSubs[o].Add...) {
			mark(s)
		}
		// A printed subtype an override keeps stays on the derived type line;
		// the view drops printed subtypes outside the table once an override
		// has set them.
		for _, s := range dd.OverrideSubs[o].Keep {
			for _, i := range pool {
				if dd.CardSubs[i].Has(int(s)) {
					mark(s)
					break
				}
			}
		}
	}
	pwTypes := false
	for i := range dd.Exprs {
		if x := &dd.Exprs[i]; x.Kind == kop.ExValidAgg && x.N == kop.AggCreatureTypes {
			choosesType = true // a count of creature types reads every printed one
		} else if x.Kind == kop.ExCount && x.N == 1 {
			pwTypes = true // a count of planeswalker types reads every walker's printed ones
		}
	}
	if pwTypes {
		for _, i := range pool {
			if dd.Defs[i].Types&kop.TPlaneswalker != 0 {
				subSet = subSet.Or(dd.CardSubs[i])
			}
		}
	}
	for i := range dd.Filters {
		if f := &dd.Filters[i]; f.Pred == kop.PredWord7 && f.PredN&0xff == kop.W7SharesCrTrigObj {
			choosesType = true // sharing a creature type with the attacker reads every printed one
		}
	}
	if choosesType {
		for _, i := range pool {
			subSet = subSet.Or(dd.CardSubs[i])
		}
	}
	// The kernel's Role exclusivity keys on the printed Role subtype by
	// name (dirac/gen.go prepRoles), not on a filter, so keep Role in the
	// table whenever an included card prints it. Without this the mask()
	// remap below drops the bit from every CardDef and the sweep never arms.
	for s, n := range dd.Subtypes {
		if n != "Role" {
			continue
		}
		for _, i := range pool {
			if dd.CardSubs[i].Has(int(s)) {
				mark(uint8(s))
				break
			}
		}
		break
	}
	subMap := map[uint8]uint8{}
	t := &kop.Table{Exprs: dd.Exprs, Conds: dd.Conds}
	for s := 0; s < len(dd.Subtypes); s++ {
		if subSet.Has(int(s)) {
			subMap[uint8(s)] = uint8(len(t.Subtypes))
			t.Subtypes = append(t.Subtypes, dd.Subtypes[s])
		}
	}
	t.TypeVocab = make([]uint8, len(dd.CreatureTypes))
	for v, w := range dd.CreatureTypes {
		t.TypeVocab[v] = 255
		for si, sw := range t.Subtypes {
			if sw == w {
				t.TypeVocab[v] = uint8(si)
			}
		}
	}
	if len(t.Subtypes) > kop.MaxSubtypes {
		return nil, nil, fmt.Errorf("matchup needs %d subtypes", len(t.Subtypes))
	}
	// A filter naming a subtype no object in this matchup can have keeps
	// the sentinel NoSubtype: Sub never matches, NotSub always passes.
	subOf := func(s uint8) uint8 {
		if m, ok := subMap[s]; ok {
			return m + 1
		}
		return NoSubtype
	}
	mask := func(subs []uint8) kop.SubMask {
		var m kop.SubMask
		for _, s := range subs {
			if b, ok := subMap[s]; ok {
				m = m.With(int(b))
			}
		}
		return m
	}
	t.Filters = make([]kop.Filter, len(dd.Filters))
	copy(t.Filters, dd.Filters)
	for k := range t.Filters {
		f := &t.Filters[k]
		if f.Sub > 0 {
			f.Sub = subOf(f.Sub - 1)
		}
		if f.NotSub > 0 {
			f.NotSub = subOf(f.NotSub - 1)
		}
		for i, n := range f.NotSubs {
			if n > 0 {
				f.NotSubs[i] = subOf(n - 1)
			}
		}
		remapSubtypePreds(f, subMap)
		if f.Pred == kop.PredNamed {
			// PredN is a pool index; the table's CardIDs are the matchup's
			// compact ids (inert: the pool's stable PoolID), so the
			// card-zone matcher's int(c) == n comparison needs the id the
			// named card has HERE (findings r2). A named-other clause's
			// PredN may be -1 (no pool entry has the name) or a card the
			// matchup does not hold: no object here has that name, so it
			// stays / becomes -1 and matches nothing
			// (gorge sharesName). A named<self> clause is literal too, and
			// its carrier is in the matchup whenever its table is built.
			if j, ok := id[int(f.PredN)]; ok && f.PredN >= 0 {
				f.PredN = int16(j)
			} else {
				f.PredN = -1
			}
		}
	}
	t.Overrides = make([]kop.Override, len(dd.Overrides))
	copy(t.Overrides, dd.Overrides)
	for k := range t.Overrides {
		t.Overrides[k].SetSub = mask(dd.OverrideSubs[k].Set)
		t.Overrides[k].AddSub = mask(dd.OverrideSubs[k].Add)
		t.Overrides[k].KeepSub = mask(dd.OverrideSubs[k].Keep)
		if nc := t.Overrides[k].NameCard; nc != 0 {
			t.Overrides[k].NameCard = 0
			if j, ok := id[int(nc)-1]; ok {
				t.Overrides[k].NameCard = uint16(j) + 1
			}
		}
	}
	remap := func(abs []kop.Ability) []kop.Ability {
		out := make([]kop.Ability, len(abs))
		for k, ab := range abs {
			out[k] = ab
			if ab.Kind == kop.AbReplacement && (ab.Repl.Kind == kop.ReplRewriteTokens || ab.Repl.Kind == kop.ReplAddToken) {
				out[k].Repl.C = int16(id[int(ab.Repl.C)]) // the token it makes or adds
			}
			out[k].Prog = make([]kop.Instr, len(ab.Prog))
			copy(out[k].Prog, ab.Prog)
			for n := range out[k].Prog {
				in := &out[k].Prog[n]
				switch in.Op {
				case kop.OpCreateToken, kop.OpCreateTokenEx, kop.OpAmass, kop.OpEmpower, kop.OpIncubate, kop.OpTakeInitiative:
					in.C = int16(id[int(in.C)])
				case kop.OpExt:
					if in.Ext() == kop.ExtEndure {
						in.B.N = int16(id[int(in.B.N)]) // the Spirit token
					}
					if in.Ext() == kop.ExtCopyNamed {
						in.B.N = int16(id[int(in.B.N)]) // the named card's entry, a table token
					}
				}
			}
		}
		return out
	}
	t.Emblems = make([]kop.Emblem, len(dd.Emblems))
	for k, e := range dd.Emblems {
		t.Emblems[k] = e // keeps UntilEOT (the copy dropped it)
		t.Emblems[k].Abilities = remap(e.Abilities)
	}
	t.Cards = make([]kop.CardDef, len(pool))
	if inert {
		t.Cards = make([]kop.CardDef, dd.NumIDs)
		for _, a := range dd.Absent {
			t.Cards[a.ID] = kop.CardDef{Name: a.Name}
		}
	}
	for _, i := range pool {
		j := id[i]
		d := dd.Defs[i]
		d.Name = dd.Names[i]
		var subs []uint8
		for s := 0; s < len(dd.Subtypes); s++ {
			if dd.CardSubs[i].Has(int(s)) {
				subs = append(subs, uint8(s))
			}
		}
		d.Subtypes = mask(subs)
		d.Abilities = remap(d.Abilities)
		if len(d.Faces) > 0 {
			fs := make([]kop.CardID, len(d.Faces))
			for n, f := range d.Faces {
				fs[n] = id[int(f)]
			}
			d.Faces = fs
		}
		if inert && s.Inert(i) {
			d = inertDef(d)
		}
		for _, as := range dd.StaticSubs[i] {
			if as.Ab < len(d.Abilities) {
				d.Abilities[as.Ab].Static.AddSub = mask(as.Subs)
			}
		}
		t.Cards[j] = d
	}
	if !inert {
		if err := s.interactionGuard(t, pool); err != nil {
			return nil, nil, err
		}
	}
	if !inert {
		if err := copyNamedGuard(t); err != nil { // pooldata_names.go
			return nil, nil, err
		}
	}
	if err := s.nameChoices(t, pool, id); err != nil { // pooldata_names.go
		return nil, nil, err
	}
	deck := func(d []Entry) []kop.CardID {
		var out []kop.CardID
		for _, e := range d {
			i, _ := s.Lookup(e.Name)
			for c := 0; c < e.Count; c++ {
				out = append(out, id[i])
			}
		}
		return out
	}
	out := make([][]kop.CardID, len(decks))
	for p, d := range decks {
		out[p] = deck(d)
	}
	return t, out, nil
}

// remapSubtypePreds renumbers the pool subtype index a filter's Pred slot
// carries into the matchup's compact subtype table. PredSubtype's PredN is the
// pool index (a clause's second subtype); one no object here can have maps past
// every subtype (NoSubtype-1, as Sub's sentinel). PredHistoric's PredN is the
// pool's Saga subtype index+1; the compact table renumbers subtypes, so the
// pool index would test an arbitrary other subtype (an all-creature-types
// permanent satisfies any creature type). A matchup with no Saga keeps 0: the
// clause is artifact or legendary only.
func remapSubtypePreds(f *kop.Filter, subMap map[uint8]uint8) {
	switch {
	case f.Pred == kop.PredSubtype:
		if m, ok := subMap[uint8(f.PredN)]; ok {
			f.PredN = int16(m)
		} else {
			f.PredN = NoSubtype - 1
		}
	case f.Pred == kop.PredHistoric && f.PredN > 0:
		if m, ok := subMap[uint8(f.PredN-1)]; ok {
			f.PredN = int16(m) + 1
		} else {
			f.PredN = 0
		}
	}
}
