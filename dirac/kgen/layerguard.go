package kgen

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// The CR 613 pool guard (reduced to its permanent list by the layer
// walk). It refused a matchup whose pool held two cards whose layer effects
// the kernel's fixed-order derivation could combine in the wrong order.
// The layer walk (dirac/static_layerwalk.go layers 1-5 and 7b, static_layer6.go
// layer 6, static_control.go layer 2, copy_exceptions.go layer 1) now applies
// every effect kgen lowers in CR 613 order, so every such row is lifted and
// layerConflicts is empty:
//   - two base-P/T setters (CR 613.4b), two type setters or a type setter
//     next to a type grant (613.1d), two colour setters (613.1e): override
//     records and override statics are merged by timestamp on every slot
//     two of them reach (perm.ovr.walk) or every slot (a global static);
//   - a static subtype grant next to a subtype setter (613.1d): the grant
//     is walked with the setter on every slot an override reaches
//     (fxNeedSubSet) and the walk takes every subtype grant;
//   - a subtype-reading grant next to a type change (613.8): every slot
//     walks in gorge's layer-4 dependency order (fxNeedSubGrant).
// Each lifted row has a gorge-pinned scenario (dirac/static_layerwalk_test.go,
// static_layer6_test.go, static_layerguards_test.go mirroring dirac/oracle
// layerscen_test.go).
//
// The shapes the walk does not implement are refused per CARD by the
// lowering, never per matchup, so they need no row here. The permanent
// refusal list (plan "What kgen keeps refusing"):
//   - layer 7d P/T switching: the Pump keyword "HIDDEN CARDNAME's power and
//     toughness are switched" (Twisted Image) is an unknown keyword;
//   - layer 3 text changes: api ChangeText (Artificial Evolution);
//   - dependency loops (Humility + Opalescence): a P/T set from a dynamic
//     value ("AffectedX") is refused, so Opalescence never lowers;
//   - a filter reading a characteristic another same-layer effect changes,
//     where gorge has no dependency rule: an override static whose
//     affected filter reads P/T or keywords (walkFilterOK), and a
//     keyword-gated layer-6 static that grants or removes the keyword it
//     reads (it could form a 613.8 cycle);
//   - global overrides the walk does not serve: SetName, a lose-all over
//     non-creatures (the mana planner does not walk lands: Toxicrene), a
//     layer-6 override with a condition other than the turn or the
//     source's own counters;
//   - CantHaveKeyword$ and a one-shot RemoveKeywords$ (Animate/AnimateAll);
//   - a copy exception with toughness below 1 (the SBA shortcut reads no
//     exception);
//   - at run time, a global lose-all static beside a subtype lord the
//     layer-4 walk reads stalls the game at its start (l6Stall).
// A new order-sensitive shape the walk cannot order is added back here as
// a row naming its classes, so a matchup refuses itself instead of
// mis-ordering silently.

type (
	layerClass    = cardset.LayerClass
	layerClasses  = cardset.LayerClasses
	layerConflict = cardset.LayerConflict
)

const (
	lcRecPT       = cardset.LcRecPT
	lcRecTypeSet  = cardset.LcRecTypeSet
	lcRecTypeAdd  = cardset.LcRecTypeAdd
	lcRecColor    = cardset.LcRecColor
	lcStatPT      = cardset.LcStatPT
	lcStatTypeSet = cardset.LcStatTypeSet
	lcStatTypeAdd = cardset.LcStatTypeAdd
	lcStatColor   = cardset.LcStatColor
	lcGrantSub    = cardset.LcGrantSub
	lcSubFilter   = cardset.LcSubFilter
	lcFreshType   = cardset.LcFreshType
	lcAnyTypeChg  = cardset.LcAnyTypeChg
)

// layerRefusal is cardset.LayerRefusal over layerConflicts.
func layerRefusal(names []string, cls []layerClasses) error {
	return cardset.LayerRefusal(names, cls, layerConflicts)
}

// layerConflicts is the guard's data: a matchup is refused when one card
// has a class in A and another card a class in B (see layerExempt for the
// pairs that can never meet on one permanent). Empty: the walk
// orders every pair the guard named (the header lists them).
var layerConflicts = []layerConflict{}

// classifyLayers reads a built card's layer-changing effects.
func classifyLayers(pt *poolTables, d *kop.CardDef) layerClasses {
	var lc layerClasses
	mark := func(c layerClass, self bool) {
		lc.Any |= c
		if !self {
			lc.Other |= c
		}
	}
	// override marks override i: rec selects the record classes, else the
	// static ones; fresh marks a record on an entering object.
	override := func(i int, self, rec, fresh bool) {
		if i < 0 || i >= len(pt.overrides) {
			return
		}
		o := &pt.overrides[i]
		set := o.HasTypes || i < len(pt.ovSubs) && pt.ovSubs[i].set != nil || o.HasSub || o.RemoveCreatureSub || o.SetLandSub
		add := o.AddTypes != 0 || o.AllCreatureTypes || i < len(pt.ovSubs) && len(pt.ovSubs[i].add) > 0
		if fresh {
			if set || add {
				mark(lcFreshType, self)
			}
			return
		}
		pc, sc, ac, cc := lcRecPT, lcRecTypeSet, lcRecTypeAdd, lcRecColor
		if !rec {
			pc, sc, ac, cc = lcStatPT, lcStatTypeSet, lcStatTypeAdd, lcStatColor
			if o.LoseAbilities {
				lc.Strips = true
			}
			if o.HasTypes && o.SetTypes&kop.TCreature == 0 {
				lc.Uncreature = true
			}
		}
		if o.HasPT {
			mark(pc, self)
		}
		if set {
			mark(sc, self)
		}
		if add {
			mark(ac, self)
		}
		if o.HasColors {
			mark(cc, self)
		}
	}
	filterSelf := func(f kop.FilterID) bool {
		return int(f) < len(pt.filters) && pt.filters[f].Flags&kop.FSelf != 0
	}
	if f := d.Enchant.Filter; f != 0 && int(f) < len(pt.filters) && pt.filters[f].Types == kop.TCreature && pt.filters[f].Or == 0 {
		lc.OnCreature = true
	}
	for ai := range d.Abilities {
		a := &d.Abilities[ai]
		if a.Kind == kop.AbStatic {
			st := &a.Static
			switch st.Kind {
			case kop.StatOverride:
				override(int(st.Override)-1, filterSelf(st.Filter), false, false)
			case kop.StatContinuous:
				if st.AddSub != (kop.SubMask{}) || st.AddChosen {
					self := filterSelf(st.Filter)
					mark(lcGrantSub, self)
					if filterReadsSub(pt, st.Filter) {
						mark(lcSubFilter, self)
					}
				}
			}
		}
		var returned [8]kop.Ref // objects an earlier ReturnEx of this program moved
		nr := 0
		for _, in := range a.Prog {
			self := in.Obj == kop.RefSelf
			fresh := false
			for _, r := range returned[:nr] {
				fresh = fresh || r == in.Obj
			}
			switch in.Op {
			case kop.OpReturnEx:
				if nr < len(returned) {
					returned[nr] = in.Obj
					nr++
				}
			case kop.OpSetPT:
				if !fresh {
					mark(lcRecPT, self)
				}
			case kop.OpAnimate:
				if fresh {
					mark(lcFreshType, self)
				} else {
					mark(lcRecPT|lcRecTypeAdd, self)
				}
			case kop.OpSetTypes:
				switch {
				case fresh:
					mark(lcFreshType, self)
				case in.A.N == 1:
					mark(lcRecTypeAdd, self)
				default:
					mark(lcRecTypeSet, self)
				}
			case kop.OpOverride, kop.OpEarthbend:
				// Earthbend applies an override record (animateGrant) to its
				// target land: the same layer class as OpOverride.
				override(int(in.C), self, true, fresh || in.Obj == kop.RefRemembered && nr > 0)
			}
		}
	}
	return lc
}

// classifyBuilt is classifyLayers plus the static subtype grants a built
// card carries as pool indexes (stAddSub: AddSub is a matchup mask).
func classifyBuilt(pt *poolTables, b *built) layerClasses {
	lc := classifyLayers(pt, &b.def)
	for ai, subs := range b.stAddSub {
		if ai >= len(b.def.Abilities) {
			continue
		}
		st := &b.def.Abilities[ai].Static
		if len(subs) == 0 && !st.AddChosen {
			continue
		}
		self := int(st.Filter) < len(pt.filters) && pt.filters[st.Filter].Flags&kop.FSelf != 0
		lc.Any |= lcGrantSub
		if !self {
			lc.Other |= lcGrantSub
		}
		if filterReadsSub(pt, st.Filter) {
			lc.Any |= lcSubFilter
			if !self {
				lc.Other |= lcSubFilter
			}
		}
	}
	return lc
}

func filterReadsSub(pt *poolTables, f kop.FilterID) bool {
	for n := 0; int(f) < len(pt.filters) && n < 64; n++ {
		fl := &pt.filters[f]
		if fl.Sub != 0 || fl.NotSub != 0 {
			return true
		}
		if fl.Or == 0 {
			break
		}
		f = fl.Or
	}
	return false
}

// writeLayerCensus reports the guard's reach over a pool: every classified
// card, and (for a deck pool) how many decks and deck pairs it refuses.
func writeLayerCensus(w io.Writer, names []string, cls []layerClasses, decks [][]string) {
	idx := map[string]int{}
	n := 0
	for i, nm := range names {
		idx[nm] = i
		if cls[i].Any != 0 {
			n++
			fmt.Fprintf(w, "class\t%s\t%s\tother=%s\tstrips=%v uncreature=%v oncreature=%v\n", nm, cls[i].Any, cls[i].Other, cls[i].Strips, cls[i].Uncreature, cls[i].OnCreature)
		}
	}
	fmt.Fprintf(w, "classified cards: %d of %d\n", n, len(names))
	// pool-wide pairs
	pairs := map[string]string{}
	for i := range names {
		for j := i + 1; j < len(names); j++ {
			if err := layerRefusal([]string{names[i], names[j]}, []layerClasses{cls[i], cls[j]}); err != nil {
				pairs[names[i]+" + "+names[j]] = err.Error()
			}
		}
	}
	var ps []string
	for p := range pairs {
		ps = append(ps, p)
	}
	sort.Strings(ps)
	for _, p := range ps {
		why := pairs[p]
		if k := strings.LastIndex(why, ": "); k >= 0 && strings.HasPrefix(why, "layer guard: ") {
			why = why[strings.Index(why[len("layer guard: "):], ": ")+len("layer guard: ")+2:]
		}
		fmt.Fprintf(w, "pair\t%s\t%s\n", p, why)
	}
	fmt.Fprintf(w, "conflicting card pairs in the pool: %d\n", len(ps))
	if len(decks) == 0 {
		return
	}
	// a deck reduced to its classified cards; decks with equal keys share
	// a verdict, so pairs are counted over distinct keys.
	key := map[string]int{}
	var keys [][]string
	selfRefused := 0
	for _, d := range decks {
		var k []string
		for _, nm := range d {
			if i, ok := idx[nm]; ok && cls[i].Any != 0 {
				k = append(k, nm)
			}
		}
		ks := strings.Join(k, "\x00")
		if _, ok := key[ks]; !ok {
			key[ks] = 0
			keys = append(keys, k)
		}
		key[ks]++
	}
	refusedKey := func(k []string) bool {
		c := make([]layerClasses, len(k))
		for i, nm := range k {
			c[i] = cls[idx[nm]]
		}
		return layerRefusal(k, c) != nil
	}
	var refusedPairs, totalPairs int64
	nd := int64(len(decks))
	totalPairs = nd * nd
	for _, a := range keys {
		ca := int64(key[strings.Join(a, "\x00")])
		if refusedKey(a) {
			selfRefused += int(ca)
		}
		for _, b := range keys {
			cb := int64(key[strings.Join(b, "\x00")])
			if refusedKey(append(append([]string{}, a...), b...)) {
				refusedPairs += ca * cb
			}
		}
	}
	fmt.Fprintf(w, "decks refused on their own: %d of %d\n", selfRefused, len(decks))
	fmt.Fprintf(w, "ordered deck pairs refused: %d of %d (%.2f%%)\n", refusedPairs, totalPairs, 100*float64(refusedPairs)/float64(totalPairs))
}

// layerCensusOut, when set (kgen -layer-census), makes generate print the
// guard's census instead of writing the table.
var layerCensusOut io.Writer
