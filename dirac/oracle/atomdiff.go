package oracle

import (
	"errors"
	"fmt"
	"sort"
	"sync"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/rules"
	"github.com/adams-shaun/gorge/engine/state"
)

// Atom differential: at every decision both engines agree on, a registered set
// of atoms (a filter word, a Count$ head) is evaluated by gorge's own matcher
// and by the kernel over every live object, and the first disagreement is a
// divergence naming the atom. It proves a word for every object in many real
// positions, not only where some dealt card happened to read it.
//
// The kernel form of an atom is never written by hand: kgen's own lowering
// (kgen.AtomCompiler) turns the atom's spec into the literal filter or amount
// the kernel's probes (dirac.ProbeFilter, ProbeValue) evaluate over a table
// overlay, so the two sides cannot drift.

// AtomLowerer lowers a filter spec or a Count$ head into kernel tables laid
// over a game's own; kgen.AtomCompiler is the one implementation. dirac/kgen's
// tests import this package, so it cannot import kgen: whoever runs the
// harness sets NewAtomLowerer (atomdiff_lower_test.go for the tests; a kfuzz
// mode would do the same from its main).
type AtomLowerer interface {
	Filter(spec string, zone kop.Zone) (kop.FilterID, error)
	Count(head string) (kop.ValueRef, error)
	Tables() ([]kop.Filter, []kop.Expr)
	Progs() []kop.FilterProg // the filter-program table the lowered filters index (nil: the game's own)
	Missing() []string
}

// NewAtomLowerer builds the lowerer over a game's table.
var NewAtomLowerer func(t *kop.Table) AtomLowerer

// AtomZones is the set of zones an atom is defined in.
type AtomZones uint8

// The zones a filter atom may be read in; AtomAnyZone is every one the harness
// probes. A word the kernel reads on a permanent only (tapped) names
// AtomBattlefield: gorge reads it false anywhere else and the kernel's card
// matcher is never asked.
const (
	AtomBattlefield AtomZones = 1 << iota
	AtomHand
	AtomGraveyard
	AtomExile
	AtomLibrary
	AtomStack
	AtomAnyZone = AtomBattlefield | AtomHand | AtomGraveyard | AtomExile | AtomLibrary | AtomStack
)

// atomZoneList pairs each AtomZones bit with the two engines' zone names.
var atomZoneList = [...]struct {
	bit AtomZones
	k   kop.Zone
	g   state.Zone
}{
	{AtomBattlefield, kop.ZoneBattlefield, state.ZBattlefield},
	{AtomHand, kop.ZoneHand, state.ZHand},
	{AtomGraveyard, kop.ZoneGraveyard, state.ZGraveyard},
	{AtomExile, kop.ZoneExile, state.ZExile},
	{AtomLibrary, kop.ZoneLibrary, state.ZLibrary},
	{AtomStack, kop.ZoneStack, state.ZStack},
}

// FilterAtom is one filter word under test. GorgeSpec is the spec gorge's
// matcher reads ("Card.attacking"); KernelSpec, empty for the same spec, is
// what kgen lowers for the kernel (only a deliberately wrong atom differs).
// Ctx is the zones the word is defined in; Needs the banks it reads.
type FilterAtom struct {
	Name, GorgeSpec, KernelSpec string
	Ctx                         AtomZones
	Needs                       dirac.ProbeBanks // state the game's table must maintain for the kernel to answer it
	// Skip names an object the kernel models differently on purpose, so the
	// word is not compared on it; each user cites its ticket (nil: none).
	Skip func(o *state.Object, d rules.Derived) bool
}

// CountAtom is one Count$ head under test: GorgeHead is the expression gorge
// evaluates ("Count$Valid Creature.YouCtrl"), KernelHead, empty for the same,
// what kgen lowers.
type CountAtom struct {
	Name, GorgeHead, KernelHead string
	Needs                       dirac.ProbeBanks // as FilterAtom.Needs
	TwoSeat                     bool             // kgen lowers the head for two seats only (every opponent is the one): not compared in a pod
}

var atomReg struct {
	sync.Mutex
	filters []FilterAtom
	counts  []CountAtom
}

// RegisterFilterAtom adds a to the registry the three atomdiff tests run.
// Each family's atomdiff_fw_<family>.go calls it from init, so no shared list
// grows.
func RegisterFilterAtom(a FilterAtom) {
	atomReg.Lock()
	defer atomReg.Unlock()
	atomReg.filters = append(atomReg.filters, a)
}

// RegisterCountAtom is RegisterFilterAtom for a Count$ head.
func RegisterCountAtom(a CountAtom) {
	atomReg.Lock()
	defer atomReg.Unlock()
	atomReg.counts = append(atomReg.counts, a)
}

// RegisteredAtoms is the registry, in name order (init order is file order,
// which a rename must not move).
func RegisteredAtoms() ([]FilterAtom, []CountAtom) {
	atomReg.Lock()
	defer atomReg.Unlock()
	f := append([]FilterAtom(nil), atomReg.filters...)
	c := append([]CountAtom(nil), atomReg.counts...)
	sort.Slice(f, func(i, j int) bool { return f[i].Name < f[j].Name })
	sort.Slice(c, func(i, j int) bool { return c[i].Name < c[j].Name })
	return f, c
}

// AtomStats counts, per atom, its evaluations and how many gorge read true
// and false. An atom with no true or no false reading over a seed set is not
// proven by it (TestAtomCoverage).
type AtomStats struct {
	Seed      uint64 // named in a divergence's detail; set by the caller
	Decisions int
	Evals     map[string]int
	True      map[string]int
	False     map[string]int
}

// NewAtomStats is an empty AtomStats.
func NewAtomStats() *AtomStats {
	return &AtomStats{Evals: map[string]int{}, True: map[string]int{}, False: map[string]int{}}
}

// Add folds o's counts into s.
func (s *AtomStats) Add(o *AtomStats) {
	s.Decisions += o.Decisions
	for k, v := range o.Evals {
		s.Evals[k] += v
	}
	for k, v := range o.True {
		s.True[k] += v
	}
	for k, v := range o.False {
		s.False[k] += v
	}
}

// atomSet is the registry lowered into one pair of kernel tables.
type atomSet struct {
	filters []kop.Filter
	exprs   []kop.Expr
	progs   []kop.FilterProg
	fa      []FilterAtom
	fid     [][len(atomZoneList)]kop.FilterID // per filter atom, per zone of its Ctx
	skipF   []bool                            // per filter atom: it names a subtype this game\'s table lacks
	skipC   []bool                            // per count atom, the same
	ca      []CountAtom
	cv      []kop.ValueRef
	err     error
	ps      *dirac.ProbeSet
}

func compileAtoms(fa []FilterAtom, ca []CountAtom, tbl *kop.Table) *atomSet {
	as := &atomSet{fa: fa, ca: ca}
	if NewAtomLowerer == nil {
		as.err = errors.New("oracle.NewAtomLowerer is not set: the caller of the atom harness must install a lowerer (kgen.NewAtomCompilerOver; see atomdiff_lower_test.go)")
		return as
	}
	comp := NewAtomLowerer(tbl)
	as.fid = make([][len(atomZoneList)]kop.FilterID, len(fa))
	as.skipF, as.skipC = make([]bool, len(fa)), make([]bool, len(ca))
	for i, a := range fa {
		spec := a.KernelSpec
		if spec == "" {
			spec = a.GorgeSpec
		}
		for zi, z := range atomZoneList {
			if a.Ctx&z.bit == 0 {
				continue
			}
			id, err := comp.Filter(spec, z.k)
			if err != nil {
				as.err = errors.Join(as.err, fmt.Errorf("filter atom %s (%s) in zone %d: kgen refuses: %v", a.Name, spec, z.k, err))
			}
			as.fid[i][zi] = id
			as.skipF[i] = as.skipF[i] || len(comp.Missing()) > 0
		}
	}
	as.cv = make([]kop.ValueRef, len(ca))
	for i, a := range ca {
		head := a.KernelHead
		if head == "" {
			head = a.GorgeHead
		}
		v, err := comp.Count(head)
		if err != nil {
			as.err = errors.Join(as.err, fmt.Errorf("count atom %s (%s): kgen refuses: %v", a.Name, head, err))
		}
		as.cv[i] = v
		as.skipC[i] = len(comp.Missing()) > 0
	}
	as.filters, as.exprs = comp.Tables()
	as.progs = comp.Progs()
	return as
}

// newAtomSet lowers the atoms over r's table and builds the probe overlay. An
// atom whose own lowering turns on state the game's table never allocated (a
// damage history, a ledger: NewProbeSet refuses the layout) is left out, as an
// atom reading a bank the table lacks is (Needs), and so reads as unevaluated.
func newAtomSet(r *realSide, fa []FilterAtom, ca []CountAtom) *atomSet {
	as := compileAtoms(fa, ca, r.tbl)
	if as.err != nil {
		return as
	}
	var err error
	if as.ps, err = r.s.NewProbeSetProgs(as.filters, as.exprs, as.progs); err == nil {
		return as
	}
	var okF []FilterAtom
	var okC []CountAtom
	for _, a := range fa {
		if one := compileAtoms([]FilterAtom{a}, nil, r.tbl); one.err != nil {
			return one
		} else if _, e := r.s.NewProbeSetProgs(one.filters, one.exprs, one.progs); e == nil {
			okF = append(okF, a)
		}
	}
	for _, a := range ca {
		if one := compileAtoms(nil, []CountAtom{a}, r.tbl); one.err != nil {
			return one
		} else if _, e := r.s.NewProbeSetProgs(one.filters, one.exprs, one.progs); e == nil {
			okC = append(okC, a)
		}
	}
	as = compileAtoms(okF, okC, r.tbl)
	if as.err == nil {
		if as.ps, err = r.s.NewProbeSetProgs(as.filters, as.exprs, as.progs); err != nil {
			as.err = err
		}
	}
	return as
}

// AtomCheck is a RunConfig.Check evaluating the registered atoms at every
// decision both sides agree on (a multi-pick frame is compared at its first
// pick, as FeatCheck does).
func AtomCheck(st *AtomStats) func(ref *GorgeSide, k Side) *Divergence {
	fa, ca := RegisteredAtoms()
	return AtomCheckFor(st, fa, ca)
}

// AtomCheckFor is AtomCheck over an explicit atom list (a test's own atom).
func AtomCheckFor(st *AtomStats, fa []FilterAtom, ca []CountAtom) func(ref *GorgeSide, k Side) *Divergence {
	var as *atomSet
	var asTbl *kop.Table
	return func(ref *GorgeSide, k Side) *Divergence {
		r, ok := k.(*realSide)
		if !ok || ref.fr == nil || len(ref.fr.chosen) > 0 {
			return nil
		}
		if as == nil || asTbl != r.tbl {
			// the atoms are lowered over this game's table, as a card of its matchup would be
			as, asTbl = newAtomSet(r, fa, ca), r.tbl
		}
		if as.err != nil {
			return &Divergence{What: "atom", Field: "lowering", Detail: as.err.Error()}
		}
		st.Decisions++
		return as.check(ref, r, st)
	}
}

// atomCtx is one evaluation context: the source's controller and the source,
// the permanent each atom is read relative to (none for a free-standing read).
type atomCtx struct {
	you state.PlayerID
	gs  state.ObjID
	ks  dirac.Obj
}

func (as *atomSet) contexts(ref *GorgeSide, r *realSide) []atomCtx {
	g := ref.E.G
	var out []atomCtx
	for p := 0; p < int(r.n); p++ {
		if p < len(g.Players) && g.Players[p].Lost {
			continue
		}
		out = append(out, atomCtx{you: state.PlayerID(p), ks: dirac.NoObj})
		for _, id := range g.Zone(state.ZBattlefield, state.PlayerID(p)) {
			if inst := ref.id.ref(g, id); inst != RefNone && int(inst) < MaxInst {
				out = append(out, atomCtx{you: state.PlayerID(p), gs: id, ks: dirac.Obj(inst)})
				break
			}
		}
	}
	return out
}

// atomObj is a live object both engines hold in the same zone.
type atomObj struct {
	gid state.ObjID
	k   dirac.Obj
	zi  int // index in atomZoneList
}

func (as *atomSet) objects(ref *GorgeSide, r *realSide, buf []atomObj) []atomObj {
	g := ref.E.G
	buf = buf[:0]
	for inst, oid := range ref.id.obj {
		o := g.Obj(oid)
		if o == nil || o.PhasedOut || ref.id.inst[oid] != Ref(inst) {
			continue
		}
		kz := r.s.ProbeObjZone(dirac.Obj(inst))
		for zi, z := range atomZoneList {
			if o.Zone == z.g && kz == z.k {
				buf = append(buf, atomObj{gid: oid, k: dirac.Obj(inst), zi: zi})
			}
		}
	}
	return buf
}

func (as *atomSet) check(ref *GorgeSide, r *realSide, st *AtomStats) *Divergence {
	ctxs := as.contexts(ref, r)
	objs := as.objects(ref, r, nil)
	g := ref.E
	fail := func(name string, ctx atomCtx, obj, gv, kv any) *Divergence {
		return &Divergence{What: "atom", Field: name, Detail: fmt.Sprintf("seed %d decision %d obj %v (you %d, source %d): gorge %v kernel %v", st.Seed, st.Decisions, obj, ctx.you, ctx.gs, gv, kv)}
	}
	banks := r.s.ProbeBanks()
	for i, a := range as.fa {
		if as.skipF[i] || a.Needs&^banks != 0 {
			continue
		}
		spec := a.GorgeSpec
		for _, ob := range objs {
			if a.Ctx&atomZoneList[ob.zi].bit == 0 {
				continue
			}
			if a.Skip != nil && a.Skip(g.G.Obj(ob.gid), g.Derived(ob.gid)) {
				continue
			}
			fid := as.fid[i][ob.zi]
			for _, c := range ctxs {
				gv := rules.MatchesSpecAtZone(g, spec, ob.gid, c.gs, c.you, atomZoneList[ob.zi].g)
				kv := r.s.ProbeFilter(as.ps, fid, ob.k, c.ks, uint8(c.you))
				st.Evals[a.Name]++
				if gv {
					st.True[a.Name]++
				} else {
					st.False[a.Name]++
				}
				if gv != kv {
					return fail(a.Name, c, as.describe(ref, r, ob), gv, kv)
				}
			}
		}
	}
	for i, a := range as.ca {
		if as.skipC[i] || a.Needs&^banks != 0 || a.TwoSeat && r.n > 2 {
			continue
		}
		for _, c := range ctxs {
			gv := int(effects.EvalCount(g, effects.NewCtxPtr(c.gs, c.you, effects.CtxInit{}), a.GorgeHead))
			kv := r.s.ProbeValue(as.ps, as.cv[i], c.ks, uint8(c.you))
			st.Evals[a.Name]++
			if gv != 0 {
				st.True[a.Name]++
			} else {
				st.False[a.Name]++
			}
			if gv != kv {
				return fail(a.Name, c, "-", gv, kv)
			}
		}
	}
	return nil
}

// describe names an object for a divergence's detail: both engines' ids, its
// card and zone, and each side's current power and toughness.
func (as *atomSet) describe(ref *GorgeSide, r *realSide, ob atomObj) string {
	name := "?"
	if o := ref.E.G.Obj(ob.gid); o != nil && o.Card != nil {
		name = o.Face().Name
	}
	d := ref.E.Derived(ob.gid)
	kp := &r.kp.Objs[ob.k]
	kpw, kt := r.s.ProbePT(ob.k)
	return fmt.Sprintf("%d(gorge %d %q zone %d; gorge %d/%d kernel %d/%d matcher %d/%d)", ob.k, ob.gid, name, ob.zi, d.Power, d.Toughness, kp.Power, kp.Tough, kpw, kt)
}
