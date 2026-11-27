package manifest

import (
	"fmt"
	"strings"
)

// Band is an inclusive range: values for an enum family, bit positions for a
// flag family.
type Band struct {
	Lo, Hi int64
	// Why says what the band is for (reserved bands only).
	Why string
}

func (b Band) has(v int64) bool { return v >= b.Lo && v <= b.Hi }

// Family is one Go type the manifest allocates codes of.
type Family struct {
	// Name is the Go type; an entry's directory.
	Name string
	// Pkg is the Go package the constants are generated into: "kop" (exported
	// names) or "dirac" (unexported).
	Pkg string
	// GoType is the type the constant is declared with when that is not Name
	// ("uint8" for the ask kinds, which are not a named type).
	GoType string
	// Prefix starts every entry name ("" for none).
	Prefix string
	// Bits marks a flag word: every value is one bit, and Min/Max/Alloc and
	// Reserved are bit positions.
	Bits bool
	// Min and Max bound an explicit value (the type's range, or a table's
	// capacity where that is smaller).
	Min, Max int64
	// Alloc is where a "next" entry takes its value: the lowest free one.
	Alloc Band
	// Reserved bands take a value only for an entry with an approval line.
	Reserved []Band
	// Seeded marks a family whose vocabulary the census pre-maps (kmanifest
	// seed); the others take entries one at a time.
	Seeded bool
	// Kgen is the kgen name registration helper for the type ("enum",
	// "mask"), or "" when kgen names it some other way or not at all.
	Kgen string
	// Cover, when not 0, is one past the values the kernel serves: an entry
	// at or above it is deferred or blocked (Validate), and the generator
	// writes no name registration for it (CounterKind: kop.NumCounterKinds3).
	Cover int64

	// register is the kop/dirac name registration for e, a statement or "".
	register func(e *Entry) string
	// check is the family's own entry rule.
	check func(e *Entry) error
}

// CounterKindCover is kop.NumCounterKinds3, frozen at the cutover: the
// counter kinds the kernel's per-kind tables and walks reach (ProjObj.CountersX,
// CtrXNames, kop.IsCounterKind). A kind at or above it waits on moving the
// cover, and a kind of 256 or more also on widening the Action mode byte and
// the CostSubAmong pick key (dirac ctrMode). dirac's
// TestManifestStatusMatchesCode holds the two equal.
const CounterKindCover = 242

// CoverWhy is the status reason of an entry at or above its family's Cover.
const CoverWhy = "cover:NumCounterKinds3"

// opHotBand keeps the free Op values [158,249] for hot-path ops: a new
// operation is otherwise an ExtOp, and an Op here needs an approval line.
var opHotBand = Band{158, 249, "hot-path ops, each by approval"}

// Families are the manifest's families, sorted by name. Untyped and int16
// operand codes are not here: they stay hand-written in their feature files.
var Families = []*Family{
	flags("AbilityFlags", "Af", 16, "mask"),
	enum8("CmpOp", "Cmp", "enum"),
	flags("ColorMask", "", 8, "mask"),
	enum8("CondKind", "Cond", "enum"),
	{
		// CostAtomKind: one atom of a cost program (kop.CostAtom, Table.CostProgs;
		// dirac/cost_prog.go). Zero is no atom. Only the parts gorge's
		// pay/classify.go prices are atoms: cost:unknown, cost:withheld and
		// cost:twobrid have none.
		Name: "CostAtomKind", Pkg: "kop", Prefix: "CostAtom", Min: 1, Max: 255, Alloc: Band{Lo: 1, Hi: 255},
	},
	flags("CostFlags", "Cost", 32, "mask"),
	{
		// perm.ck is 16 bits wide, so every uint16 value is a legal counter
		// kind, but the kernel serves the kinds below CounterKindCover only
		// (kop.IsCounterKind): the seed defers a kind at or above it. The
		// name registration fills CtrXNames, gorge's counter name, from the
		// first forge token.
		Name: "CounterKind", Pkg: "kop", Prefix: "Ctr", Min: 0, Max: 65535,
		Alloc: Band{Lo: 0, Hi: 65535}, Seeded: true, Kgen: "enum", Cover: CounterKindCover,
		register: func(e *Entry) string {
			return fmt.Sprintf("CtrXNames[%s-CtrCharge] = %q", e.Name, e.Forge[0])
		},
		check: func(e *Entry) error {
			if len(e.Forge) == 0 {
				return fmt.Errorf("a CounterKind needs a forge line: its counter's name")
			}
			return nil
		},
	},
	// ExprKind widens to uint16 before the manifest is seeded.
	{Name: "ExprKind", Pkg: "kop", Prefix: "Ex", Max: 65535, Alloc: Band{Lo: 0, Hi: 65535}, Seeded: true, Kgen: "enum"},
	{
		// NumExtOps (321) is the sub-op tables' capacity.
		Name: "ExtOp", Pkg: "kop", Prefix: "Ext", Min: 0, Max: 320, Alloc: Band{Lo: 1, Hi: 320}, Seeded: true,
		register: func(e *Entry) string {
			return fmt.Sprintf("extOpNames[%s] = %q", e.Name, strings.TrimPrefix(e.Name, "Ext"))
		},
	},
	flags("FilterFlags", "F", 16, "mask"),
	flags("FilterFlags2", "F2", 32, ""),
	{
		// FilterWord: a literal of a filter program (kop.FLit, PredProg). Zero
		// is no word; the kernel's fwTable is indexed by the value, with one
		// handler per word (//kmanifest:fword).
		Name: "FilterWord", Pkg: "kop", Prefix: "Fw", Min: 1, Max: 65535, Alloc: Band{Lo: 1, Hi: 65535},
		check: checkFilterWord,
	},
	// KwID 0..31 are the KwMask bit positions, with no constants of their own.
	{Name: "KwID", Pkg: "kop", Prefix: "Kw", Min: 32, Max: 65535, Alloc: Band{Lo: 32, Hi: 65535}, Seeded: true, Kgen: "enum"},
	flags("KwMask", "Kw", 32, "mask"),
	{
		// The frozen iota list ends at OpCount (156) and the high ops start at
		// OpHighFirst (250); kgen names ops through Op.Known.
		Name: "Op", Pkg: "kop", Prefix: "Op", Min: 0, Max: 255,
		Alloc: Band{Lo: opHotBand.Lo, Hi: opHotBand.Hi}, Reserved: []Band{opHotBand},
		register: func(e *Entry) string {
			return fmt.Sprintf("opNames[%s] = %q", e.Name, strings.TrimPrefix(e.Name, "Op"))
		},
	},
	enum8("Pred", "Pred", "enum"),
	enum8("Ref", "Ref", "enum"),
	seeded(enum8("ReplKind", "Repl", "enum")),
	flags("SpecialMask", "Spm", 8, ""),
	flags("StaticFlags", "St", 16, "mask"),
	{Name: "StaticKind", Pkg: "kop", Prefix: "Stat", Max: 65535, Alloc: Band{Lo: 0, Hi: 65535}, Seeded: true, Kgen: "enum"},
	flags("TargetFlags", "Tgf", 32, "mask"),
	enum8("TargetKind", "Tgt", "enum"),
	seeded(enum8("TrigEvent", "Trig", "enum")),
	flags("TrigFlags", "Tf", 8, "mask"),
	flags("TrigFlags2", "Tf2", 32, ""),
	flags("TrigFlags3", "Tf3", 8, ""),
	flags("TrigFlags4", "Tf4", 8, ""),
	flags("TrigFlags5", "Tf5", 8, ""),
	flags("TypeMask", "T", 16, "mask"),
	enum8("VKind", "V", "enum"),
	seeded(enum8("Zone", "Zone", "enum")),
	{
		// ask: a resolution ask kind (resCtx.ask, dirac/interp_gen.go). The
		// hand constants are frozen in testdata/asks.tsv; applyResolve hands
		// only an ask at or above raGenBase (100) to the generated chain.
		Name: "ask", Pkg: "dirac", Prefix: "ra", GoType: "uint8", Min: 100, Max: 255, Alloc: Band{Lo: 146, Hi: 245},
	},
	// key: a bounded State key, the keyed arena's index.
	{Name: "key", Pkg: "dirac", Prefix: "key", Max: 65535, Alloc: Band{Lo: 0, Hi: 65535}, Seeded: true},
	{
		// mech: an arena mechanic (dirac/arena.go). The legacy 55 hold 0..54,
		// a new one takes 55 up; arenaNone is 0xFF. The width comes from the
		// state line: slotbit is a slot mask, objbit an object mask. gen
		// also writes numMech, one past the highest.
		Name: "mech", Pkg: "dirac", Prefix: "mech", Min: 0, Max: 254, Alloc: Band{Lo: 55, Hi: 254},
		register: func(e *Entry) string {
			return fmt.Sprintf("mechWidth[%s] = %s\nmechName[%s] = %q", e.Name, mechWidth(e), e.Name, lowerCamel(strings.TrimPrefix(e.Name, "mech")))
		},
		check: func(e *Entry) error {
			if mechWidth(e) == "" {
				return fmt.Errorf("a mech needs a state line of slotbit or objbit, not %q", e.State)
			}
			return nil
		},
	},
}

// goType is the type an entry's constant is declared with.
func (f *Family) goType() string {
	if f.GoType != "" {
		return f.GoType
	}
	return f.Name
}

// lowerCamel lowers n's leading capitals, all but the last of a run that a
// lower-case letter follows: FooBar is fooBar, FBLo fbLo, SBAChain sbaChain,
// HexUB hexUB.
func lowerCamel(n string) string {
	r := 0
	for r < len(n) && n[r] >= 'A' && n[r] <= 'Z' {
		r++
	}
	if r > 1 && r < len(n) && n[r] >= 'a' && n[r] <= 'z' {
		r--
	}
	return strings.ToLower(n[:r]) + n[r:]
}

func mechWidth(e *Entry) string {
	switch cat, _, _ := strings.Cut(e.State, " "); cat {
	case "slotbit":
		return "wSlot"
	case "objbit":
		return "wObj"
	}
	return ""
}

func enum8(name, prefix, kgen string) *Family {
	return &Family{Name: name, Pkg: "kop", Prefix: prefix, Max: 255, Alloc: Band{Lo: 0, Hi: 255}, Kgen: kgen}
}

func seeded(f *Family) *Family { f.Seeded = true; return f }

func flags(name, prefix string, width int64, kgen string) *Family {
	return &Family{Name: name, Pkg: "kop", Prefix: prefix, Bits: true, Max: width - 1, Alloc: Band{Lo: 0, Hi: width - 1}, Kgen: kgen}
}

// FamilyNamed is the family called name, or nil.
func FamilyNamed(name string) *Family {
	for _, f := range Families {
		if f.Name == name {
			return f
		}
	}
	return nil
}

// reserved is the reserved band holding v (a value, or a bit position for a
// flag family), or nil.
func (f *Family) reserved(v int64) *Band {
	for i := range f.Reserved {
		if f.Reserved[i].has(v) {
			return &f.Reserved[i]
		}
	}
	return nil
}

// bitOf is v's bit position, or -1 when v is not a single bit.
func bitOf(v int64) int64 {
	if v <= 0 || v&(v-1) != 0 {
		return -1
	}
	var b int64
	for v > 1 {
		v >>= 1
		b++
	}
	return b
}
