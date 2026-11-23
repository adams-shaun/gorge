package kgen

import (
	"fmt"
	"math/bits"
	"sync"

	"github.com/adams-shaun/gorge"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/dirac/manifest"
)

// Filter spill: a clause the kernel filter struct cannot hold (its one Pred
// slot already carries a comparison) keeps the struct for what fits and moves
// every comparison into a kop.PredProg program of literals, each a kop.FilterWord
// the kernel already has (dirac/filter_prog.go). Only words that exist are
// used: a comparison with no word still refuses as before ("two comparisons in
// one clause").

// cmpSpill is lClause.CmpStat of a clause whose comparisons all live in
// lClause.Spill (none in CmpOp/CmpN/CmpX/CmpCounter). It is an unknown
// comparison to every reader that does not know the spill, which therefore
// fail closed.
const cmpSpill uint8 = 204

// maxSpill is the literals one clause spills (lClause.Spill's length).
const maxSpill = 4

// FilterFlags2 has no free bit: all 32 constants are named (dirac/kop/
// filters.go, kernel filter_negations.go), so every boolean property a clause
// still needs is a FilterWord literal in the program (cmpLit), never a new
// Flags2 bit — the spill is also the Flags2-exhausted arm of the design.
// TestFilterFlags2Full pins the count.

// lSpillLit is one spilled literal, in LIR terms: the word, whether it is
// negated, and its operands (resolved to table indexes by cardBuilder.spillProg).
type lSpillLit struct {
	W       kop.FilterWord
	Neg     bool
	Op      uint8   // a comparison word's operator (opGE ...)
	N       int     // the bound, or a keyword bit
	R       kop.Ref // a referent word's referent (FwSharesCr, FwSharesName)
	Counter string  // FwCountersCmp's kind
	Sub     string  // FwSubtype's subtype
}

func (l lSpillLit) String() string {
	s := fmt.Sprintf("w%d", l.W)
	if l.Neg {
		s = "!" + s
	}
	return fmt.Sprintf("%s(%d %d %s %s)", s, l.Op, l.N, l.Counter, l.Sub)
}

// cmpLit is the literal a comparison lowers to, when a word exists for it:
// a constant power, toughness, mana value or counter bound, or a keyword the
// object has (or lacks).
func cmpLit(stat, op uint8, n int, x, counter string) (lSpillLit, bool) {
	if x != "" {
		return lSpillLit{}, false
	}
	switch stat {
	case cmpPower, cmpToughness, cmpCMC:
		if op < opGE || op > opNE || counter != "" {
			return lSpillLit{}, false
		}
		w := map[uint8]kop.FilterWord{cmpPower: kop.FwPowerCmp, cmpToughness: kop.FwToughnessCmp, cmpCMC: kop.FwCMCCmp}[stat]
		return lSpillLit{W: w, Op: op, N: n}, true
	case cmpCounters:
		// the ops counterPred admits (build.go): a GE bound up to 127, an LT
		// bound from 1, an LE bound to 126, an EQ 0; GT/NE have no kernel read
		if counter == "" || n < 0 || n > 127 {
			return lSpillLit{}, false
		}
		switch op {
		case opGE:
		case opLT:
			if n == 0 {
				return lSpillLit{}, false
			}
		case opLE:
			if n+1 > 127 {
				return lSpillLit{}, false
			}
		case opEQ:
			if n != 0 {
				return lSpillLit{}, false
			}
		default:
			return lSpillLit{}, false
		}
		return lSpillLit{W: kop.FwCountersCmp, Op: op, N: n, Counter: counter}, true
	case cmpHasKw, cmpLacksKw:
		return lSpillLit{W: kop.FwKeyword, Neg: stat == cmpLacksKw, N: n}, true
	}
	return lSpillLit{}, false
}

func (c *lClause) pushSpill(l lSpillLit) bool {
	if int(c.NSpill) >= maxSpill {
		return false
	}
	c.Spill[c.NSpill] = l
	c.NSpill++
	return true
}

// spillCmp is setCmp's answer to a second comparison: both go to the clause's
// program when a word exists for each, else the clause refuses as ever.
func (c *lClause) spillCmp(stat, op uint8, n int, x, counter string) error {
	errTwo := fmt.Errorf("two comparisons in one clause")
	nl, ok := cmpLit(stat, op, n, x, counter)
	if !ok {
		return errTwo
	}
	if c.CmpStat != cmpSpill {
		first, ok := cmpLit(c.CmpStat, c.CmpOp, c.CmpN, c.CmpX, c.CmpCounter)
		if !ok {
			return errTwo
		}
		c.pushSpill(first)
		c.CmpStat, c.CmpOp, c.CmpN, c.CmpX, c.CmpCounter = cmpSpill, 0, 0, "", ""
	}
	if !c.pushSpill(nl) {
		return errTwo
	}
	return nil
}

// withSubtype2Spilled is c with its second subtype (Subtype2, which takes the
// kernel filter's Pred slot) moved into the program beside its comparison; c
// itself when there is no comparison to share the slot with, or when the
// comparison has no word (the clause then refuses as before).
func (c lClause) withSubtype2Spilled() lClause {
	if c.Subtype2 == "" || c.CmpStat == cmpNone || c.Name != "" {
		return c
	}
	d := c
	if d.CmpStat != cmpSpill {
		first, ok := cmpLit(d.CmpStat, d.CmpOp, d.CmpN, d.CmpX, d.CmpCounter)
		if !ok {
			return c
		}
		d.pushSpill(first)
		d.CmpStat, d.CmpOp, d.CmpN, d.CmpX, d.CmpCounter = cmpSpill, 0, 0, "", ""
	}
	if !d.pushSpill(lSpillLit{W: kop.FwSubtype, Sub: d.Subtype2}) {
		return c
	}
	d.Subtype2 = ""
	return d
}

// spillPred sets kf's PredProg for a clause whose comparisons are spilled.
func (cb *cardBuilder) spillPred(kf *kop.Filter, c *lClause, zone kop.Zone) error {
	if kf.Pred != kop.PredNone {
		return fmt.Errorf("spilled comparisons beside another predicate (Pred %d)", kf.Pred)
	}
	id, err := cb.spillProg(c, zone)
	if err != nil {
		return err
	}
	kf.Pred, kf.PredN = kop.PredProg, int16(id)
	return nil
}

// staticIn builds a static ability with its filters read in the layer walk
// as well (fwAllowed).
func (cb *cardBuilder) staticIn(s lStatic) (kop.Ability, []uint8, error) {
	defer func(old bool) { cb.inStatic = old }(cb.inStatic)
	cb.inStatic = true
	return cb.static(s)
}

// fwContexts is each filter word's declared matcher kinds (the manifest's
// contexts line), by the word's entry name. A build reads the embedded
// manifest, never the working tree; unreadable, no word is allowed.
var fwContexts = sync.OnceValue(func() map[string][]string {
	es, err := manifest.ReadEntries(gorge.TableSources())
	if err != nil {
		return nil
	}
	out := map[string][]string{}
	for _, e := range es {
		if e.Family == "FilterWord" {
			out[e.Name] = e.Contexts
		}
	}
	return out
})

// fwName is a word's manifest entry name.
var fwName = map[kop.FilterWord]string{
	kop.FwPowerCmp: "FwPowerCmp", kop.FwToughnessCmp: "FwToughnessCmp", kop.FwCMCCmp: "FwCMCCmp",
	kop.FwCountersCmp: "FwCountersCmp", kop.FwKeyword: "FwKeyword", kop.FwSubtype: "FwSubtype",
}

// fwReadIn is the matcher kinds a filter built for zone is read in: a
// battlefield filter reads permanents (and, in a static, the layer walk's
// "cur" view), the stack's spells, any other zone's cards, and a filter with
// no zone any of them.
func fwReadIn(zone kop.Zone, static bool) []string {
	switch {
	case static:
		return []string{"perm", "cur"}
	case zone == kop.ZoneBattlefield:
		return []string{"perm"}
	case zone == kop.ZoneStack:
		return []string{"spell"}
	case zone == kop.ZoneNone:
		return []string{"perm", "card", "spell"}
	}
	return []string{"card"}
}

// fwAllowed refuses a word read in a matcher kind its manifest entry does
// not declare: the word would fail closed there while gorge's would not
// (class other:filter word <W> in <context>).
func fwAllowed(w kop.FilterWord, zone kop.Zone, static bool) error {
	name := fwName[w]
	have := fwContexts()[name]
	for _, ctx := range fwReadIn(zone, static) {
		ok := false
		for _, h := range have {
			ok = ok || h == ctx
		}
		if !ok {
			return refuse("filter word %s in %s", name, ctx)
		}
	}
	return nil
}

// spillProg lowers the clause's spilled literals to a program of the table
// and returns its id.
func (cb *cardBuilder) spillProg(c *lClause, zone kop.Zone) (kop.FilterProgID, error) {
	lits := make([]kop.FLit, 0, c.NSpill)
	for _, l := range c.Spill[:c.NSpill] {
		fl := kop.FLit{W: l.W, Neg: l.Neg, Op: cmpOps[l.Op]} // build.go's canonical op map, not a positional cast
		switch l.W {
		case kop.FwPowerCmp, kop.FwToughnessCmp, kop.FwCMCCmp:
			fl.V = kop.Const(l.N)
		case kop.FwCountersCmp:
			k, err := counterKind(l.Counter)
			if err != nil {
				return 0, err
			}
			if k == kop.CtrStash || int(k) >= 256 {
				return 0, fmt.Errorf("counter compare %d on %s", l.Op, l.Counter)
			}
			fl.Arg, fl.V = int16(k), kop.Const(l.N)
		case kop.FwKeyword:
			if l.N == bits.TrailingZeros32(uint32(kop.KwFlash)) && zone == kop.ZoneBattlefield {
				return 0, refuse("hasKeywordFlash on the battlefield") // as the struct's PredHasKeyword
			}
			if l.N < 0 || l.N >= 32 {
				return 0, fmt.Errorf("keyword bit %d", l.N)
			}
			fl.Arg = int16(l.N)
		case kop.FwWithKeyword:
			fl.Arg = int16(l.N) // the KwID
		case kop.FwSharesCr, kop.FwSharesName:
			fl.R = l.R // the referent (filter_sharesreferents.go)
		case kop.FwSubtype:
			i, ok := cb.pt.sub(l.Sub)
			if !ok {
				return 0, fmt.Errorf("subtype %q", l.Sub)
			}
			fl.Arg = int16(i)
		}
		if err := fwAllowed(l.W, zone, cb.inStatic); err != nil {
			return 0, err
		}
		lits = append(lits, fl)
	}
	return cb.pt.internFilterProg(lits), nil
}

// internFilterProg interns a program of the table (index 0 is unused, as
// kop.Table.FilterProgs documents).
func (pt *poolTables) internFilterProg(lits []kop.FLit) kop.FilterProgID {
	if pt.progs == nil {
		pt.progs = []kop.FilterProg{{}}
		pt.progKey = map[string]kop.FilterProgID{}
	}
	key := fmt.Sprintf("%#v", lits)
	if id, ok := pt.progKey[key]; ok {
		return id
	}
	pt.progs = append(pt.progs, kop.FilterProg{Lits: lits})
	id := kop.FilterProgID(len(pt.progs) - 1)
	pt.progKey[key] = id
	return id
}
