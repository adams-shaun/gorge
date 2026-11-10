package kgen

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// adhocKey stands in for a key directory's name in a table written outside one.
const adhocKey = "ad-hoc"

// generate lowers and builds the pool and writes <poolName>.ktab,
// <poolName>.coverage.md and ids.tsv into dir, which it creates. idsIn is the
// CardID index to extend (idsFile; missing = none yet).
func generate(p *pool, poolName, dir, idsIn string) (coverage, error) {
	prevIDs, err := readIDs(idsIn)
	if err != nil {
		return coverage{}, err
	}
	if layerCensusOut != nil {
		pb, err := buildForOutput(p, prevIDs)
		if err != nil {
			return coverage{}, err
		}
		names := make([]string, len(pb.all))
		for i, c := range pb.all {
			names[i] = c.Name
		}
		// a deck's tokens come with the card that makes them
		withTokens := append([]layerClasses{}, pb.cls...)
		for i := range pb.lw.cards {
			for _, ti := range pb.bs[i].tokens {
				withTokens[i].Any |= pb.cls[ti].Any
				withTokens[i].Other |= pb.cls[ti].Other
			}
		}
		n := len(pb.lw.cards)
		writeLayerCensus(layerCensusOut, names[:n], withTokens[:n], p.decks)
		return pb.cv, nil
	}
	d, cv, idx, err := tableOf(p, poolName, prevIDs)
	if err != nil {
		return cv, err
	}
	var md bytes.Buffer
	writeCoverage(&md, poolName, cv)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return cv, err
	}
	for _, f := range []tableFile{
		{poolName + ".ktab", cardset.EncodeTable(d, adhocKey)},
		{poolName + ".coverage.md", md.Bytes()},
		{idsFile, idsBytes(idx)},
	} {
		if err := os.WriteFile(filepath.Join(dir, f.name), f.data, 0o644); err != nil {
			return cv, err
		}
	}
	return cv, nil
}

// poolBuild is a pool lowered and built, CardIDs assigned, before anything is
// rendered.
type poolBuild struct {
	lw  *lowered
	all []*lCard
	bs  []built
	pt  *poolTables
	ids []int
	idx []idEntry
	cv  coverage
	cls []layerClasses
}

// buildForOutput lowers and builds p and assigns CardIDs under the index prev.
func buildForOutput(p *pool, prev []idEntry) (*poolBuild, error) {
	lw := lowerPool(p)
	all, bs, pt, err := buildPool(lw)
	if err != nil {
		return nil, err
	}
	ids, idx, err := assignIDs(prev, all)
	if err != nil {
		return nil, err
	}
	for i := range bs {
		bs[i].refSubs = refSubs(pt, &bs[i].def, bs[i].ovVals)
	}
	cls := make([]layerClasses, len(all))
	for i := range bs {
		cls[i] = classifyBuilt(pt, &bs[i])
	}
	return &poolBuild{lw: lw, all: all, bs: bs, pt: pt, ids: ids, idx: idx, cv: computeCoverage(p, lw), cls: cls}, nil
}

// guardLists are the pool indexes the matchup guards read: the Room cards, the
// cards that read a name, the ab-reduce statics and the mana-costed mana abilities.
func guardLists(bs []built, filters []kop.Filter) (rooms, readers, abRed []int, abRedFloor []uint8, abRedTypes []uint16, abMana []int, abManaMV []uint8, abManaTypes []uint16) {
	for i := range bs {
		if bs[i].def.FaceMode == kop.FaceRoom {
			rooms = append(rooms, i)
		}
		if bs[i].readsName {
			readers = append(readers, i)
		}
		// A reducer of activated abilities reaches a mana ability's mana cost
		// (CR 601.2f, 602.2b); one that lowers only an Exhaust, Power-up,
		// Equip or Room unlock never does. Its MinMana$ floor (0 none) is what the
		// cost keeps.
		for _, a := range bs[i].def.Abilities {
			if a.Kind == kop.AbStatic && a.Static.Kind == kop.StatReduceAbility && a.Static.B&(1|kop.ReduceAbUnlock|kop.ReduceAbPowerUp|kop.ReduceAbEquip) == 0 {
				abRed = append(abRed, i)
				abRedFloor = append(abRedFloor, uint8(a.Static.B>>kop.ReduceAbMinShift)&15)
				abRedTypes = append(abRedTypes, reducerTypes(filters, a.Static.Filter))
				break
			}
		}
		// The dearest mana cost among a card's mana abilities the kernel
		// does not price itself: a filter's fee is priced at the slot after
		// the reducers (dirac.FilterTapMana, filtFee), any other costed mana
		// ability is not.
		mv, any := 0, false
		for j := range bs[i].def.Abilities {
			a := &bs[i].def.Abilities[j]
			if a.Kind == kop.AbMana && (a.Cost.Mana.MV() > 0 || a.Cost.Mana.X > 0) && !dirac.FilterTapMana(&bs[i].def, a) {
				any = true
				mv = max(mv, int(a.Cost.Mana.MV())+int(a.Cost.Mana.X))
			}
		}
		if any {
			abMana = append(abMana, i)
			abManaMV = append(abManaMV, uint8(min(max(mv, 1), 255)))
			abManaTypes = append(abManaTypes, uint16(bs[i].def.Types))
		}
	}
	return rooms, readers, abRed, abRedFloor, abRedTypes, abMana, abManaMV, abManaTypes
}

// reducerTypes is the card types a StatReduceAbility static's filter can
// match (kop.Filter.Types, 0 for any): every type when the filter has an
// alternative (Or) or names none.
func reducerTypes(filters []kop.Filter, id kop.FilterID) uint16 {
	if int(id) >= len(filters) {
		return 0
	}
	f := &filters[id]
	if f.Or != 0 {
		return 0
	}
	return uint16(f.Types)
}

// validPoolName reports whether name is letters, digits and underscores only.
func validPoolName(name string) bool {
	if name == "" {
		return false
	}
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '_':
		default:
			return false
		}
	}
	return true
}

func init() { _ = sort.Strings }

// refSubs walks every filter a card's abilities reach (targets, costs,
// triggers, statics, replacements, instruction filters, the filters inside
// its exprs, conds and override values, and the emblems it creates) and returns
// the pool subtypes they name, sorted.
func refSubs(pt *poolTables, d *kop.CardDef, ovVals []kop.ValueRef) []uint8 {
	set := map[uint8]bool{}
	seenF := map[kop.FilterID]bool{}
	var filt func(id kop.FilterID)
	var val func(v kop.ValueRef)
	filt = func(id kop.FilterID) {
		for id != 0 && !seenF[id] {
			seenF[id] = true
			f := pt.filters[id]
			if f.Sub > 0 {
				set[f.Sub-1] = true
			}
			if f.NotSub > 0 {
				set[f.NotSub-1] = true
			}
			for _, n := range f.NotSubs {
				if n > 0 {
					set[n-1] = true
				}
			}
			if f.Pred == kop.PredHistoric && f.PredN > 0 {
				// PredN is the Saga subtype index+1; the kernel reads it by index,
				// and the matchup must keep that subtype or its remap zeroes the clause
				set[uint8(f.PredN-1)] = true
			}
			if f.Pred == kop.PredOutlaw {
				// the kernel reads the five outlaw types by name
				// (genPrep.outlawSub), so the matchup keeps them
				for _, w := range [...]string{"Assassin", "Mercenary", "Pirate", "Rogue", "Warlock"} {
					if i, ok := pt.subIdx[w]; ok {
						set[uint8(i)] = true
					}
				}
			}
			val(f.PredV)
			id = f.Or
		}
	}
	val = func(v kop.ValueRef) {
		switch v.K {
		case kop.VCountFilter:
			filt(kop.FilterID(v.N))
		case kop.VExpr:
			e := pt.exprs[v.N]
			if e.Kind == kop.ExSacrificedThisTurn && e.N == kop.SacrificedFood {
				// the kernel tallies the Food cards by subtype name
				// (prepTurnTallies), so the matchup keeps it
				if i, ok := pt.subIdx["Food"]; ok {
					set[uint8(i)] = true
				}
			}
			filt(e.F)
			val(e.Sub[0])
			val(e.Sub[1])
		}
	}
	var cond func(c kop.CondID)
	cond = func(c kop.CondID) {
		k := pt.conds[c]
		if k.Kind == kop.CondAnd {
			cond(k.Sub[0])
			cond(k.Sub[1])
		}
		filt(k.F)
		val(k.A)
		val(k.B)
	}
	var abil func(ab *kop.Ability)
	abil = func(ab *kop.Ability) {
		filt(ab.Cost.Filter)
		for _, t := range ab.Targets {
			filt(t.Filter)
			val(t.MinV) // TargetMax$ Count$Valid <subtype>.YouCtrl (Agility Bobblehead)
			val(t.MaxV)
		}
		filt(ab.Trig.Filter)
		filt(ab.Trig.Filter2)
		if ab.Trig.Flags3&kop.Tf3ManaFromTreasure != 0 {
			// the kernel marks the Treasure cards by subtype name
			// (markTreasureSrc), so the matchup keeps it
			if i, ok := pt.subIdx["Treasure"]; ok {
				set[uint8(i)] = true
			}
		}
		cond(ab.Trig.Cond)
		filt(ab.Static.Filter)
		filt(ab.Static.Filter2)
		cond(ab.Static.Cond2)
		val(ab.Static.VA)
		val(ab.Static.VB)
		filt(ab.Repl.Filter)
		cond(ab.Repl.Cond)
		filt(ab.ManaRestrict)
		filt(ab.ManaRestrictAb)
		filt(ab.ManaAmong)
		filt(ab.ManaHasteValid) // AddsKeywords$ Haste's Spell.Dragon+Creature
		val(ab.ManaV)
		val(ab.OwnReduce) // ReduceCost$ (a subtype filter in its count)
		for _, in := range ab.Prog {
			filt(in.F)
			val(in.A)
			val(in.B)
			if in.Op == kop.OpSkipUnless {
				cond(kop.CondID(in.C))
			}
			if in.Op == kop.OpCreateEmblem {
				for i := range pt.emblems[in.C].Abilities {
					abil(&pt.emblems[in.C].Abilities[i])
				}
			}
		}
	}
	filt(d.Enchant.Filter)
	filt(d.BestowTgt.Filter)
	filt(d.SpellCost.Filter)
	filt(d.AltAddCost.Filter)
	filt(d.KickerCost.Filter)
	filt(d.BeholdF)
	filt(d.WardSac)
	filt(d.CastBF)   // the frozen count's filter (CardDef.CastBF)
	filt(d.ProtFrom) // K:Protection:Coyote can't name a subtype the matchup then drops
	filt(d.Landwalk)
	filt(d.OptCost.Filter)
	filt(d.FbCost.Filter)
	val(d.ModesPickV) // a dynamic triggered charm's count filter (the matchup keeps the subtype it names)
	val(d.MorphReduce)
	for i := range d.Abilities {
		abil(&d.Abilities[i])
	}
	for _, v := range ovVals {
		val(v)
	}
	var out []uint8
	for s := range set {
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// buildPool builds every lowered card and token of lw against the pool's
// subtype table (two passes) and appends each build refusal to its card,
// and a token's refusal to every card that makes it: after it, a card with
// no refusal is one the generated pool supports (the build-level count,
// TestCorpusBuildAcceptance).
func buildPool(lw *lowered) ([]*lCard, []built, *poolTables, error) {
	all := append(append([]*lCard{}, lw.cards...), lw.tokens...)
	// Pass 1 finds the subtypes effects and filters name beyond the
	// printed ones; pass 2 builds against the complete subtype table.
	pt := newPoolTables(lw, nil)
	for _, c := range all {
		buildCard(pt, c)
	}
	pt = newPoolTables(lw, pt.missing)
	bs := make([]built, len(all))
	for i, c := range all {
		bs[i] = buildCard(pt, c)
		c.Refusals = append(c.Refusals, bs[i].refusals...)
	}
	if len(pt.missing) > 0 {
		return nil, nil, nil, fmt.Errorf("subtype table incomplete after two passes: %v", pt.missing)
	}
	// A card whose token was refused at build time is refused too.
	for i, c := range lw.cards {
		for _, ti := range bs[i].tokens {
			if len(all[ti].Refusals) > 0 && len(c.Refusals) == 0 {
				c.Refusals = append(c.Refusals, refusal{Card: c.Name, Node: "token " + all[ti].Name, Reason: "token refused"})
			}
		}
		if len(c.Refusals) > 0 {
			bs[i].def.Abilities = nil
		}
	}
	sup, err := embeddedSupport()
	if err != nil {
		return nil, nil, nil, err
	}
	refuseByName(lw, sup)
	return all, bs, pt, nil
}
