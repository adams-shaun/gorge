package oracle

import (
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/rules"
	"github.com/adams-shaun/gorge/engine/state"
)

// Set atoms (design 2026-11-19-dirac-operand-algebras K1/K3): a kernel set
// expression (kop.SetExpr, dirac evalSet) beside gorge's evaluation of the
// equivalent Defined$ (effects.DefinedSpec), at every decision both engines
// agree on. Each atom is read for every living seat as the controller (with
// its first permanent as the source, as the count atoms are) and, when both
// stacks hold an entry, in the top entry's context: its source, controller,
// targets and trigger referents (gorge's trigger capture rides the stack
// object's Remembered, which its Triggered* referents read).

// SetAtom is one set expression under test. Prog is the kernel program; a
// SetAndFilter step's Arg indexes Filters, the filter specs the atom's
// lowerer compiles for it. Unordered compares the members as sets: gorge's
// " & " concatenates its parts, the kernel's union walks in index order.
type SetAtom struct {
	Name, GorgeDefined string
	Prog               kop.SetExpr
	Filters            []string
	Unordered          bool
	// Bound, when set, is whether gorge's context binds the atom's referent;
	// the atom is compared only there (nil: everywhere). The kernel's trigger
	// player is the event's player for the trigger modes that name one, and
	// what a lowering emits only for those (TriggeredPlayer on another mode
	// falls back to gorge's Remembered players).
	Bound func(c *effects.Ctx) bool
}

var setAtomReg struct {
	atoms []SetAtom
}

// RegisterSetAtom adds a to the set-atom registry; each family's
// atomdiff_set_<family>.go calls it from init.
func RegisterSetAtom(a SetAtom) {
	atomReg.Lock()
	defer atomReg.Unlock()
	setAtomReg.atoms = append(setAtomReg.atoms, a)
}

// RegisteredSetAtoms is the set-atom registry, in name order.
func RegisteredSetAtoms() []SetAtom {
	atomReg.Lock()
	defer atomReg.Unlock()
	a := append([]SetAtom(nil), setAtomReg.atoms...)
	sort.Slice(a, func(i, j int) bool { return a[i].Name < a[j].Name })
	return a
}

// setAtomSet is the set atoms lowered over one game's table.
type setAtomSet struct {
	atoms []SetAtom
	progs []kop.SetExpr // the atoms' programs, SetAndFilter args rewritten to the overlay's filter ids
	ps    *dirac.ProbeSet
	err   error
}

func newSetAtomSet(r *realSide, atoms []SetAtom) *setAtomSet {
	as := &setAtomSet{atoms: atoms}
	if NewAtomLowerer == nil {
		as.err = errors.New("oracle.NewAtomLowerer is not set (see atomdiff_lower_test.go)")
		return as
	}
	comp := NewAtomLowerer(r.tbl)
	for _, a := range atoms {
		prog := slices.Clone(a.Prog)
		for i, op := range prog {
			if op.Kind != kop.SetAndFilter {
				continue
			}
			id, err := comp.Filter(a.Filters[op.Arg], kop.ZoneBattlefield)
			if err != nil {
				as.err = errors.Join(as.err, fmt.Errorf("set atom %s filter %q: kgen refuses: %v", a.Name, a.Filters[op.Arg], err))
			}
			prog[i].Arg = uint16(id)
		}
		as.progs = append(as.progs, prog)
	}
	if as.err != nil {
		return as
	}
	filters, exprs := comp.Tables()
	as.ps, as.err = r.s.NewProbeSetProgs(filters, exprs, comp.Progs())
	return as
}

// SetAtomCheck is a RunConfig.Check evaluating atoms at every decision both
// sides agree on.
func SetAtomCheck(st *AtomStats, atoms []SetAtom) func(ref *GorgeSide, k Side) *Divergence {
	var as *setAtomSet
	var asTbl *kop.Table
	return func(ref *GorgeSide, k Side) *Divergence {
		r, ok := k.(*realSide)
		if !ok || ref.fr == nil || len(ref.fr.chosen) > 0 {
			return nil
		}
		if as == nil || asTbl != r.tbl {
			as, asTbl = newSetAtomSet(r, atoms), r.tbl
		}
		if as.err != nil {
			return &Divergence{What: "atom", Field: "lowering", Detail: as.err.Error()}
		}
		st.Decisions++
		return as.check(ref, r, st)
	}
}

// setAtomCtx is one evaluation context of a set atom.
type setAtomCtx struct {
	name string
	gc   *effects.Ctx
	ks   dirac.Obj
	you  uint8
	top  bool
}

func (as *setAtomSet) contexts(ref *GorgeSide, r *realSide) []setAtomCtx {
	g := ref.E.G
	var out []setAtomCtx
	for _, c := range (*atomSet)(nil).contexts(ref, r) {
		out = append(out, setAtomCtx{
			name: fmt.Sprintf("you %d source %d", c.you, c.gs),
			gc:   effects.NewCtxPtr(c.gs, c.you, effects.CtxInit{}), ks: c.ks, you: uint8(c.you),
		})
	}
	ksrc, kctrl, ok := r.s.ProbeStackTop()
	if !ok || len(g.Stack) == 0 {
		return out
	}
	id := g.Stack[len(g.Stack)-1]
	o := g.Obj(id)
	if o == nil {
		return out
	}
	src := o.Source
	if src == 0 {
		src = id
	}
	if inst := ref.id.ref(g, src); inst == RefNone || dirac.Obj(inst) != ksrc || uint8(o.Controller) != kctrl {
		return out // the two tops are not the same entry
	}
	gc := effects.NewCtxPtr(src, o.Controller, effects.CtxInit{
		Targets: append([]state.Target(nil), o.Targets...), Remembered: append([]state.Target(nil), o.Remembered...),
		TriggerContext: rules.StackTriggerContext(ref.E, id),
	})
	return append(out, setAtomCtx{name: fmt.Sprintf("stack top %d", id), gc: gc, ks: ksrc, you: kctrl, top: true})
}

// gorgeMembers renders gorge's targets as the kernel's member codes would
// read: "o<instance>" for an object, "p<seat>" for a player.
func gorgeMembers(ref *GorgeSide, ts []state.Target) []string {
	g := ref.E.G
	out := []string{}
	for _, t := range ts {
		switch {
		case t.IsPlayer:
			out = append(out, fmt.Sprintf("p%d", t.Player))
		case t.Obj == 0:
		default:
			if inst := ref.id.ref(g, t.Obj); inst != RefNone && int(inst) < MaxInst {
				out = append(out, fmt.Sprintf("o%d", inst))
			} else {
				out = append(out, fmt.Sprintf("gorge-only#%d", t.Obj))
			}
		}
	}
	return out
}

func kernelMembers(cs []dirac.TCode) []string {
	out := []string{}
	for _, c := range cs {
		if dirac.IsObjTarget(c) {
			out = append(out, fmt.Sprintf("o%d", c))
		} else {
			out = append(out, fmt.Sprintf("p%d", c-dirac.PlayerTarget0))
		}
	}
	return out
}

func setCompact(xs []string) []string {
	ys := slices.Clone(xs)
	slices.Sort(ys)
	return slices.Compact(ys)
}

func (as *setAtomSet) check(ref *GorgeSide, r *realSide, st *AtomStats) *Divergence {
	for _, c := range as.contexts(ref, r) {
		for i, a := range as.atoms {
			if a.Bound != nil && !a.Bound(c.gc) {
				continue
			}
			kc, ok := r.s.ProbeSetExpr(as.ps, as.progs[i], c.ks, c.you, c.top)
			if !ok {
				continue
			}
			gv := gorgeMembers(ref, effects.DefinedSpec(ref.E, c.gc, a.GorgeDefined))
			kv := kernelMembers(kc)
			if a.Unordered {
				gv, kv = setCompact(gv), setCompact(kv)
			}
			st.Evals[a.Name]++
			if len(gv) > 0 {
				st.True[a.Name]++
			} else {
				st.False[a.Name]++
			}
			if !slices.Equal(gv, kv) {
				return &Divergence{What: "atom", Field: a.Name, Detail: fmt.Sprintf("seed %d decision %d (%s): gorge %s [%s] kernel [%s]",
					st.Seed, st.Decisions, c.name, a.GorgeDefined, strings.Join(gv, " "), strings.Join(kv, " "))}
			}
		}
	}
	return nil
}
