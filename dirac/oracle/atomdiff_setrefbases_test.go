package oracle

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/state"
)

// refAtomSources bounds the boards one decision generates.
const refAtomSources = 24

// setRefAtomCheck is a RunConfig.Check reading atoms at every decision both
// sides agree on, on boards generated over that position: sources on the
// battlefield and in the graveyards (a departed Aura keeps its last bearer),
// each beside another object and a seat that rotate with the decision.
// nonEmpty counts, per atom, the boards where gorge named a member.
func setRefAtomCheck(atoms []SetRefAtom, nonEmpty map[string]int) func(ref *GorgeSide, k Side) *Divergence {
	var as *setAtomSet
	var asTbl *kop.Table
	plain := make([]SetAtom, len(atoms))
	for i, a := range atoms {
		plain[i] = a.SetAtom
	}
	decision := 0
	return func(ref *GorgeSide, k Side) *Divergence {
		r, ok := k.(*realSide)
		if !ok || ref.fr == nil || len(ref.fr.chosen) > 0 {
			return nil
		}
		if as == nil || asTbl != r.tbl {
			as, asTbl = newSetAtomSet(r, plain), r.tbl
		}
		if as.err != nil {
			return &Divergence{What: "atom", Field: "lowering", Detail: as.err.Error()}
		}
		decision++
		g := ref.E.G
		var objs []state.ObjID
		kobj := map[state.ObjID]dirac.Obj{}
		for p := 0; p < int(r.n); p++ {
			for _, z := range []state.Zone{state.ZBattlefield, state.ZGraveyard} {
				for _, id := range g.Zone(z, state.PlayerID(p)) {
					o := g.Obj(id)
					if inst := ref.id.ref(g, id); o != nil && !o.PhasedOut && inst != RefNone && int(inst) < MaxInst {
						objs = append(objs, id)
						kobj[id] = dirac.Obj(inst)
					}
				}
			}
		}
		if len(objs) < 2 {
			return nil
		}
		for i := 0; i < min(len(objs), refAtomSources); i++ {
			// every permanent in turn is a source over the decisions
			src := objs[(decision*refAtomSources+i)%len(objs)]
			b := RefBoard{Src: src, Obj: objs[(decision+3*i+1)%len(objs)],
				Ctrl: g.Obj(src).Controller, Player: state.PlayerID((decision + i) % int(r.n))}
			kb := dirac.ProbeRefBinding{Src: kobj[b.Src], TrigObj: kobj[b.Obj], Chosen: kobj[b.Obj], Ctrl: uint8(b.Ctrl),
				TrigPl: uint8(b.Player), DefSeat: uint8(b.Player), ChosenPl: uint8(b.Player) + 1}
			for j, a := range atoms {
				c := effects.NewCtxPtr(b.Src, b.Ctrl, effects.CtxInit{})
				if a.Bind != nil && !a.Bind(ref.E, c, b) {
					continue
				}
				gv := gorgeMembers(ref, effects.DefinedSpec(ref.E, c, a.GorgeDefined))
				kv := kernelMembers(r.s.ProbeSetResolving(as.ps, as.progs[j], kb))
				if a.Unordered {
					gv, kv = setCompact(gv), setCompact(kv)
				}
				if len(gv) > 0 {
					nonEmpty[a.Name]++
				}
				if !slices.Equal(gv, kv) {
					return &Divergence{What: "atom", Field: a.Name, Detail: fmt.Sprintf(
						"decision %d source %d (zone %d) object %d seat %d: gorge %s [%s] kernel [%s]", decision, kobj[b.Src],
						g.Obj(b.Src).Zone, kobj[b.Obj], b.Player, a.GorgeDefined, strings.Join(gv, " "), strings.Join(kv, " "))}
				}
			}
		}
		return nil
	}
}

// setRefAtomSeats plays games of the setbases fixture at n seats under check
// and returns each atom's non-empty readings.
func setRefAtomSeats(t *testing.T, n int, games uint64, atoms []SetRefAtom) map[string]int {
	t.Helper()
	reg, set, gd := setBaseTable(t)
	names := make([]string, n)
	decks := make([][]*cards.Card, n)
	es := make([][]cardset.Entry, n)
	for s := range names {
		names[s] = fmt.Sprintf("setbases-%d", s)
		decks[s], es[s] = gd[s%2], setBaseDecks[s%2]
	}
	tbl, _, err := set.MatchupN(es)
	if err != nil {
		t.Fatal(err)
	}
	nonEmpty := map[string]int{}
	cfg := Config{}
	for seed := uint64(1); seed <= games; seed++ {
		m, err := NewMatchupN(reg, names, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		if got := int(k.(*realSide).n); got != n {
			t.Fatalf("precondition: a %d-seat game, got %d", n, got)
		}
		res := Lockstep(m, seed, k, castFirstPolicy(seed), RunConfig{Config: cfg, MaxDecisions: atomMaxDecisions, MaxTurns: 40,
			Check: setRefAtomCheck(atoms, nonEmpty)})
		CloseSide(k)
		if d := res.Div; d != nil {
			t.Errorf("%d seats seed %d: decision %d: %s: %s %s", n, seed, d.At, d.What, d.Field, d.Detail)
		}
	}
	return nonEmpty
}

// TestAtomDiffSetRefBases: every Ref the kernel's SetRefBase admits has an
// atom, and each atom and the battlefield base agree with gorge's Defined$ on
// generated boards at two and four seats, reading a member at both.
func TestAtomDiffSetRefBases(t *testing.T) {
	if isolate(t) {
		return
	}
	atoms := SetRefAtoms()
	covered := map[kop.Ref]bool{}
	for _, a := range atoms {
		covered[a.Ref] = true
	}
	admitted := 0
	for r := 0; r < 256; r++ {
		if ref := kop.Ref(r); kop.SetRefBaseOK(ref) {
			admitted++
			if !covered[ref] {
				t.Errorf("admitted ref %d has no atom", r)
			}
		}
	}
	if admitted == 0 {
		t.Fatal("precondition: the kernel admits no ref")
	}
	for _, n := range []int{2, 4} {
		got := setRefAtomSeats(t, n, 6, atoms)
		for _, a := range atoms {
			t.Logf("%d seats %-36s non-empty %6d", n, a.Name, got[a.Name])
			if got[a.Name] == 0 {
				t.Errorf("%d seats: %s is not proven: gorge never named a member", n, a.Name)
			}
		}
	}
}

// TestAtomDiffSetRefBasesCatchesAWrongRef: the owner atom without the
// narrowing to a live attachment diverges from gorge.
func TestAtomDiffSetRefBasesCatchesAWrongRef(t *testing.T) {
	if isolate(t) {
		return
	}
	atoms := SetRefAtoms()
	i := slices.IndexFunc(atoms, func(a SetRefAtom) bool { return a.Ref == kop.RefAttached && a.GorgeDefined == "EnchantedOwner" })
	if i < 0 || len(atoms[i].Prog) != 3 || atoms[i].Prog[1].Kind != kop.SetAndFilter {
		t.Fatal("precondition: the EnchantedOwner atom narrows to a live attachment")
	}
	// without the narrowing a departed Aura names its last bearer's owner,
	// where gorge names nobody
	wrong := atoms[i]
	wrong.Prog = kop.SetExpr{wrong.Prog[0], wrong.Prog[2]}
	wrong.Filters = nil
	reg, set, gd := setBaseTable(t)
	tbl, _, _, err := set.Matchup(setBaseDecks[0], setBaseDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	for seed := uint64(1); seed <= 16; seed++ {
		m, err := NewMatchup(reg, [2]string{"setbases-a", "setbases-b"}, gd, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, Config{})(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		res := Lockstep(m, seed, k, castFirstPolicy(seed), RunConfig{MaxDecisions: atomMaxDecisions, MaxTurns: 40,
			Check: setRefAtomCheck([]SetRefAtom{wrong}, map[string]int{})})
		CloseSide(k)
		if d := res.Div; d != nil {
			if d.What != "atom" || d.Field != wrong.Name {
				t.Fatalf("seed %d diverged elsewhere: %+v", seed, d)
			}
			t.Logf("seed %d: %s", seed, d.Detail)
			return
		}
	}
	t.Fatal("the unnarrowed owner agreed with gorge on every board of 16 games")
}
