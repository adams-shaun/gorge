package cardset

import (
	"fmt"
	"math/bits"
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
	Defs                []kop.CardDef // Name blank: Names holds them; PrintedSubtypes blank: CardPrinted holds them
	CardSubs            []PoolSubMask
	CardTokens          [][]uint16
	CardRefSubs         [][]uint8
	AbilityOrigin       [][]uint8
	CreatureTypes       []string
	StaticSubs          [][]AbSubs
	OverrideSubs        []OvSubs
	Filters             []kop.Filter
	FilterProgs         []kop.FilterProg // the programs of PredProg filters; [0] unused, nil when no filter is a program
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
	// CardPrinted is a changeling's printed subtypes (pool indexes), before
	// kgen adds every pool creature type to its CardSubs; nil for any other
	// card. A matchup renumbers them into CardDef.PrintedSubtypes.
	CardPrinted [][]uint8
}

// UnmodeledSupertype reports whether w is a supertype the kernel's type mask
// does not model (CR 205.4a: Snow, World and Ongoing; Basic and Legendary have
// type bits). The table carries one as a subtype slot only when a filter or
// effect of the pool names it, and a type-setting override keeps that slot
// (CR 205.1a) without being a reason to give it one.
func UnmodeledSupertype(w string) bool {
	switch w {
	case "Snow", "World", "Ongoing":
		return true
	}
	return false
}

// PoolMaxSubtypes bounds a pool table's subtype list (PoolData.Subtypes). The
// pool data carries a subtype as a byte index (CardRefSubs, StaticSubs,
// OverrideSubs, CardPrinted) and a filter's Sub, NotSub and NotSubs as the
// index+1 below the matchup sentinel NoSubtype, so a pool holds 254. A
// matchup's table holds at most kop.MaxSubtypes: it keeps only the subtypes its
// own cards print or name (matchup), so a pool wider than one table (MBC's
// 246) plays every game whose cards fit, and a game that needs more is refused
// (a SubtypeRefusal).
const PoolMaxSubtypes = 254

// PoolSubMask is a set of pool subtype indexes (PoolData.Subtypes).
type PoolSubMask [4]uint64

// Has reports whether pool subtype index i is in m.
func (m PoolSubMask) Has(i int) bool { return uint(i) < 256 && m[i>>6]&(1<<(i&63)) != 0 }

// With is m with pool subtype index i added (out of range: m).
func (m PoolSubMask) With(i int) PoolSubMask {
	if uint(i) < 256 {
		m[i>>6] |= 1 << (i & 63)
	}
	return m
}

// Or is the union of m and o.
func (m PoolSubMask) Or(o PoolSubMask) PoolSubMask {
	return PoolSubMask{m[0] | o[0], m[1] | o[1], m[2] | o[2], m[3] | o[3]}
}

// Count is the number of indexes in m.
func (m PoolSubMask) Count() int {
	return bits.OnesCount64(m[0]) + bits.OnesCount64(m[1]) + bits.OnesCount64(m[2]) + bits.OnesCount64(m[3])
}

// SubtypeRefusal refuses a matchup whose cards need more subtypes than one
// table holds (kop.MaxSubtypes): a declared capacity of the per-game subtype
// table, reason "subtypes". The decks are fine alone.
func SubtypeRefusal(n int) error {
	return &InteractionRefusal{Reason: "subtypes", Why: fmt.Sprintf("the matchup's cards print or name %d subtypes, a table holds %d", n, kop.MaxSubtypes)}
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
	// Pool holds them as inert fallbacks and Inert says so. A face entry
	// counts with its main card (nameEntries). A matchup table is
	// unaffected (it compacts its CardIDs). nil when none.
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
	t, ids, err := s.matchup([][]Entry{a, b}, matchupGame)
	if err != nil {
		return nil, nil, nil, err
	}
	return t, ids[0], ids[1], nil
}

func (s dataSet) MatchupN(decks [][]Entry) (*kop.Table, [][]kop.CardID, error) {
	return s.matchup(decks, matchupGame)
}

// MatchupInert is Matchup with the matchup guards skipped: it builds the table
// the game would have played had the guards not refused it, with the game's
// compact CardIDs. A diagnostic run uses it to observe the divergence or
// unmodelled stall an interaction refusal hides. A card the set does not play
// is still a *RefusedError: this is not a by-name bypass (see Raw).
func (s dataSet) MatchupInert(a, b []Entry) (*kop.Table, []kop.CardID, []kop.CardID, error) {
	t, ids, err := s.matchup([][]Entry{a, b}, matchupSkipGuards)
	if err != nil {
		return nil, nil, nil, err
	}
	return t, ids[0], ids[1], nil
}

// MatchupNInert is MatchupInert for any number of decks.
func (s dataSet) MatchupNInert(decks [][]Entry) (*kop.Table, [][]kop.CardID, error) {
	return s.matchup(decks, matchupSkipGuards)
}

func (s dataSet) Pool() (*kop.Table, error) {
	all := make([]Entry, s.d.NumCards)
	for i := range all {
		all[i] = Entry{Name: s.d.Names[i], Count: 1}
	}
	t, _, err := s.matchup([][]Entry{all, nil}, matchupInert)
	return t, err
}

func (s dataSet) Inert(i int) bool {
	if i >= s.d.NumIDs {
		return false
	}
	if s.nameInert != nil && s.nameInert[i] {
		return true
	}
	// the plays arm stays a card entry's: a token or face entry
	// ([NumCards, NumIDs)) is inert only when nameInert says so
	return i < s.d.NumCards && !s.plays[i]
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

// keptSubtypeNeedsSlot reports whether a subtype an override keeps earns a
// table slot: some pool card prints it. A supertype slot (UnmodeledSupertype)
// is kept only when another reader gave it one, never a slot of its own.
func keptSubtypeNeedsSlot(dd *PoolData, pool []int, s uint8) bool {
	if UnmodeledSupertype(dd.Subtypes[s]) {
		return false
	}
	for _, i := range pool {
		if dd.CardSubs[i].Has(int(s)) {
			return true
		}
	}
	return false
}

// matchupOverrides is the matchup's copy of the pool's override list, its
// subtype masks and NameCard remapped. The list is the pool's, so a CR 305.7
// land-type setter of a card outside this matchup (Blood Moon beside a deck
// that has none) is in it too, and the kernel reads the flag table-wide
// (prepLandSet, prepLandGrant: a land-type model, and an attack tax's refusal
// beside it). A setter nothing here reaches (usesOverride, plus every emblem
// ability) is an empty record: its index stays.
func matchupOverrides(dd *PoolData, usesOverride map[int]bool, mask func([]uint8) kop.SubMask, id map[int]kop.CardID) []kop.Override {
	for _, e := range dd.Emblems {
		for _, ab := range e.Abilities {
			if ab.Static.Override > 0 {
				usesOverride[int(ab.Static.Override)-1] = true
			}
			for _, in := range ab.Prog {
				if in.Op == kop.OpOverride || in.Op == kop.OpOverrideAll {
					for n := 0; n < 5; n++ {
						usesOverride[int(in.C)+n] = true // a chosen-type group's five records
					}
				}
			}
		}
	}
	out := make([]kop.Override, len(dd.Overrides))
	copy(out, dd.Overrides)
	for k := range out {
		if out[k].SetLandSub && !usesOverride[k] {
			out[k] = kop.Override{}
			continue
		}
		out[k].SetSub = mask(dd.OverrideSubs[k].Set)
		out[k].AddSub = mask(dd.OverrideSubs[k].Add)
		out[k].KeepSub = mask(dd.OverrideSubs[k].Keep)
		if nc := out[k].NameCard; nc != 0 {
			out[k].NameCard = 0
			if j, ok := id[int(nc)-1]; ok {
				out[k].NameCard = uint16(j) + 1
			}
		}
	}
	return out
}

// matchupMode says which table a matchup builds: a game's, the whole pool's,
// or a refused pair's replayed without the guards.
type matchupMode uint8

const (
	matchupGame       matchupMode = iota // a game's table: compact CardIDs, every guard on
	matchupInert                         // Pool's table: stable pool CardIDs, inert cards, no guards
	matchupSkipGuards                    // a refused pair replayed: compact CardIDs, no guards
)

func (m matchupMode) inert() bool  { return m == matchupInert }
func (m matchupMode) guards() bool { return m == matchupGame }

func (s dataSet) matchup(decks [][]Entry, mode matchupMode) (*kop.Table, [][]kop.CardID, error) {
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
			if !s.plays[i] && !mode.inert() {
				return nil, nil, &RefusedError{Card: e.Name, Status: dd.Status[i], Reason: dd.Reason[i]}
			}
			add(i)
		}
	}
	sort.Ints(pool)
	if mode.guards() {
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
		if mode.inert() {
			id[i] = kop.CardID(dd.PoolID[i]) // the pool table's stable CardIDs
		}
	}
	// The matchup's subtypes are the rules-relevant ones: every subtype a
	// filter of an included card names and every subtype an included
	// card's effect grants. Printed subtypes nothing reads are dropped from
	// the masks -- unless a card chooses a creature type, when every
	// printed subtype is kept (any of them may be chosen), or the kernel
	// reads one by name (Role below, Army for amass).
	var subSet PoolSubMask
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
					markBasicLandChoice(dd, &in, mark)
				case kop.OpOverride, kop.OpOverrideAll:
					// (OpOverrideAll: gorge's AnimateAll type grants are compared
					// by the projection too -- Breach the Multiverse's Phyrexian)
					usesOverride[int(in.C)] = true
					if in.Op == kop.OpOverride && in.B.N == kop.OvChosenType {
						for n := 1; n < 5; n++ {
							usesOverride[int(in.C)+n] = true // the other basic land types' records
						}
					}
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
			if keptSubtypeNeedsSlot(dd, pool, s) {
				mark(s)
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
		if f := &dd.Filters[i]; f.Pred == kop.PredWord7 && f.PredN&0xff == kop.W7SharesCrTrigObj || f.Pred == kop.PredSharesCrCmdr {
			choosesType = true // sharing a creature type with the attacker or a commander reads every printed one
		}
	}
	if choosesType {
		all := subSet
		for _, i := range pool {
			all = all.Or(dd.CardSubs[i])
		}
		if all.Count() > kop.MaxSubtypes {
			// A changeling's subtypes are every pool creature type (kgen
			// changelingSubs), which a wide pool's (MBC's) cannot fit in one
			// game's table. Then a changeling keeps its printed ones only: the
			// kernel reads a type outside the table as one an all-creature-types
			// object has and nothing else does (outsideSub, subOutside), which
			// is what each dropped type is, since no other card here prints or
			// names it. A matchup that fits keeps every one.
			all = subSet
			for _, i := range pool {
				if dd.Defs[i].AllCreatureTypes {
					for _, s := range dd.CardPrinted[i] {
						all = all.With(int(s))
					}
				} else {
					all = all.Or(dd.CardSubs[i])
				}
			}
			if mode.inert() && all.Count() > kop.MaxSubtypes {
				// The whole-pool table (Pool: the names, stable CardIDs and the
				// net's vocabulary, never a game) of a pool wider than one table
				// keeps the subtypes its cards name or grant; a game's table
				// keeps the printed ones its type choices read.
				all = subSet
			}
		}
		subSet = all
	}
	// A static that grants a basic land type (Urborg's Swamp) makes the kernel
	// tell a land's printed mana ability from the intrinsic one its own basic
	// types give (blgPrintedMask): keep the basic land types the included
	// cards print, so a Plateau's {R} and {W} are not read as printed.
	keepPrintedBasicLandTypes(dd, pool, &subSet)
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
		return nil, nil, SubtypeRefusal(len(t.Subtypes))
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
	t.Filters = slices.Clone(dd.Filters)
	t.FilterProgs = remapFilterProgs(dd.FilterProgs, subMap)
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
	t.Overrides = matchupOverrides(dd, usesOverride, mask, id)
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
					if in.Ext() == kop.ExtPreventNext && in.C&kop.PrevNextTokens != 0 {
						// the rider's token (prevTokInstr, repl_prevnext.go):
						// B is the pool token index kgen stored (instr_damage.go
						// opPreventNext), which the kernel mints as a CardID
						in.B.N = int16(id[int(in.B.N)])
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
	if mode.inert() {
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
		if len(dd.CardPrinted) > i && len(dd.CardPrinted[i]) > 0 {
			d.PrintedSubtypes = mask(dd.CardPrinted[i])
		}
		d.Abilities = remap(d.Abilities)
		if len(d.Faces) > 0 {
			fs := make([]kop.CardID, len(d.Faces))
			for n, f := range d.Faces {
				fs[n] = id[int(f)]
			}
			d.Faces = fs
		}
		if d.FuseDef != 0 {
			d.FuseDef = id[int(d.FuseDef)-1] + 1 // the fused spell's def (dirac/face_fuse.go)
		}
		if mode.inert() && s.Inert(i) {
			d = inertDef(d)
		}
		for _, as := range dd.StaticSubs[i] {
			if as.Ab < len(d.Abilities) {
				d.Abilities[as.Ab].Static.AddSub = mask(as.Subs)
			}
		}
		t.Cards[j] = d
	}
	if mode.guards() {
		if err := s.interactionGuard(t, pool); err != nil {
			return nil, nil, err
		}
	}
	if mode.guards() {
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

// remapFilterProgs copies the pool's filter programs for a matchup's table,
// renumbering each FwSubtype literal's pool subtype index as remapSubtypePreds
// does PredSubtype's: a subtype no object of the matchup has maps past every
// subtype, so the literal never holds.
func remapFilterProgs(progs []kop.FilterProg, subMap map[uint8]uint8) []kop.FilterProg {
	if progs == nil {
		return nil
	}
	out := make([]kop.FilterProg, len(progs))
	for i, p := range progs {
		out[i].Lits = append([]kop.FLit(nil), p.Lits...)
		for j := range out[i].Lits {
			if l := &out[i].Lits[j]; l.W == kop.FwSubtype {
				if m, ok := subMap[uint8(l.Arg)]; ok {
					l.Arg = int16(m)
				} else {
					l.Arg = NoSubtype - 1
				}
			}
		}
	}
	return out
}

// basicLandTypeNames are gorge's Type$ Basic Land option labels
// (effects.TypeChoiceLabels("Basic Land")).
var basicLandTypeNames = map[string]bool{"Forest": true, "Island": true, "Mountain": true, "Plains": true, "Swamp": true}

// markBasicLandChoice marks the five basic land subtypes when in is a ChooseType
// Basic Land: its answer is a table subtype whichever land the matchup prints
// (Realmwright's AddChosen grant reads it; gorge's answer may be any of the
// five).
func markBasicLandChoice(dd *PoolData, in *kop.Instr, mark func(uint8)) {
	if in.C != 0 || in.B.N != kop.ChooseTypeBasicLand {
		return
	}
	for s, n := range dd.Subtypes {
		if basicLandTypeNames[n] {
			mark(uint8(s))
		}
	}
}

// keepPrintedBasicLandTypes adds to subSet the basic land types some pool
// card prints, when a static of an included card grants one.
func keepPrintedBasicLandTypes(dd *PoolData, pool []int, subSet *PoolSubMask) {
	granted := false
	for _, i := range pool {
		for _, as := range dd.StaticSubs[i] {
			for _, s := range as.Subs {
				granted = granted || basicLandTypeNames[dd.Subtypes[s]]
			}
		}
	}
	if !granted {
		return
	}
	for s, n := range dd.Subtypes {
		if !basicLandTypeNames[n] {
			continue
		}
		for _, i := range pool {
			if dd.CardSubs[i].Has(s) {
				*subSet = subSet.With(s)
				break
			}
		}
	}
}
