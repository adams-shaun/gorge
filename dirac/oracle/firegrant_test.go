package oracle

import (
	"math/rand/v2"
	"slices"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// The granted-Firebending lockstep (CR 702.189a, CR 113.2c): Fire Nation
// Cadets' own static grant (while a Lesson is in the graveyard), Fire Nation
// Palace's and Fire Nation Turret's until-end-of-turn grants. gorge queues one
// trigger per derived Firebending:N entry beyond the printed ones, so a grant
// beside a printed Firebending, or two grants, fire the creature twice. The
// decks cover a creature holding Cadets' static and a Palace grant (the
// derived entries are listed in layer-6 timestamp order), an artifact creature
// (Memnite) as the grant's target (gorge tags its granted mana typed), and
// Palace's {1}{R},{T} activation with a firebending unit floating (gorge's
// planned payment refuses the pool, so the witness taps lands, never the
// Palace itself).

var firegrantDecks = [2][]cardset.Entry{
	{ // A: R/W
		{Name: "Fire Nation Cadets", Count: 4}, {Name: "Firebending Lesson", Count: 4}, {Name: "Fire Nation Palace", Count: 3},
		{Name: "Firebending Student", Count: 3}, {Name: "Memnite", Count: 3}, {Name: "Shock", Count: 3},
		{Name: "Lightning Strike", Count: 2}, {Name: "Hill Giant", Count: 1},
		{Name: "Mountain", Count: 12}, {Name: "Plains", Count: 5},
	},
	{ // B: R/G
		{Name: "Fire Nation Turret", Count: 3}, {Name: "Fire Nation Palace", Count: 2}, {Name: "Firebending Student", Count: 3},
		{Name: "Fire Nation Cadets", Count: 3}, {Name: "Memnite", Count: 3}, {Name: "Grizzly Bears", Count: 2}, {Name: "Giant Growth", Count: 2},
		{Name: "Shock", Count: 2}, {Name: "Firebending Lesson", Count: 3},
		{Name: "Mountain", Count: 11}, {Name: "Forest", Count: 6},
	},
}

type firegrantStats struct {
	firebendStats
	granted       int // granted-Firebending triggers seen on the stack in a declaration
	multi         int // declarations where one creature had 2 or more Firebending triggers on the stack
	activate      int // Fire Nation Palace activations chosen
	activateFloat int // ... with a firebending unit floating
	artTgt        int // Palace/Turret grants aimed at an artifact creature
	mixed         int // creatures firing Cadets' static grant and a Palace grant together (grant order)
	artGranted    int // granted-Firebending triggers of an artifact creature (typed mana) seen on the stack
}

// firegrantProbe is the firebendProbe bias plus: activate Fire Nation Palace
// when offered, and tally the Firebending triggers on the stack.
type firegrantProbe struct {
	firebendProbe
	st       *firegrantStats
	seen     map[[4]int]bool // granted triggers already counted this declaration
	multiSrc map[dirac.Obj]bool
	mixSrc   map[dirac.Obj]bool
}

// fbTrigger reports whether stack entry i is an Attacks trigger whose first
// instruction adds persistent mana (a Firebending trigger), and whether the
// ability is a latent (granted) one.
func (p *firegrantProbe) fbTrigger(i int) (src dirac.Obj, card dirac.CardID, ab uint8, granted, ok bool) {
	kind, src, card, _ := p.rs.s.StackEntry(i)
	if kind != 2 {
		return
	}
	ab = p.rs.s.StackAbility(i)
	if int(ab) >= len(p.tbl.Cards[card].Abilities) {
		return
	}
	a := &p.tbl.Cards[card].Abilities[ab]
	if a.Kind != kop.AbTriggered || a.Trig.Event != kop.TrigAttacks || len(a.Prog) == 0 ||
		a.Prog[0].Op != kop.OpAddMana || a.Prog[0].B != kop.Const(1) {
		return
	}
	return src, card, ab, a.AFlags&kop.AfLatent != 0, true
}

func (p *firegrantProbe) pick(c *PickCtx) Canon {
	n := p.rs.s.StackLen()
	any := false
	per := map[dirac.Obj]int{}
	kinds := map[dirac.Obj]uint8{} // bit 0: Cadets' grant, bit 1: Palace's, on the stack for the creature
	occ := map[[3]int]int{}
	for i := 0; i < n; i++ {
		src, card, ab, granted, ok := p.fbTrigger(i)
		if !ok {
			continue
		}
		any = true
		per[src]++
		if granted {
			switch p.tbl.Cards[card].Name {
			case "Fire Nation Cadets":
				kinds[src] |= 1
			case "Fire Nation Palace":
				kinds[src] |= 2
			}
			k := [3]int{int(src), int(card), int(ab)}
			occ[k]++
			key := [4]int{k[0], k[1], k[2], occ[k]}
			if !p.seen[key] {
				p.seen[key] = true
				p.st.granted++
				if p.cardName(Ref(src)) == "Memnite" {
					p.st.artGranted++
				}
			}
		}
	}
	for src, c := range per {
		if c >= 2 && !p.multiSrc[src] {
			p.multiSrc[src] = true
			p.st.multi++
		}
	}
	for src, m := range kinds {
		if m == 3 && !p.mixSrc[src] {
			p.mixSrc[src] = true
			p.st.mixed++
		}
	}
	if !any {
		clear(p.seen)
		clear(p.multiSrc)
		clear(p.mixSrc)
	}
	res := p.firebendProbe.pick(c)
	for _, o := range c.D.Options {
		if o.Kind() == ActActivate && o.Mode() == 1 && p.cardName(o.Src()) == "Fire Nation Palace" && p.r.IntN(4) != 0 {
			p.st.activate++
			if p.rs.s.FirebendRed()[p.rs.s.Controller(dirac.Obj(o.Src()))] != 0 {
				p.st.activateFloat++ // {1}{R},{T} with a persistent firebending unit floating (PlanPoolOK)
			}
			return o
		}
	}
	// a Palace/Turret grant aimed at an artifact creature: the granted trigger's
	// mana is typed (gorge effects.ManaProducerTag)
	if n := p.rs.s.StackLen(); n > 0 && res.Kind() == ActTarget {
		_, _, card, _ := p.rs.s.StackEntry(n - 1)
		if nm := p.tbl.Cards[card].Name; (nm == "Fire Nation Palace" || nm == "Fire Nation Turret") &&
			int(res.Tgt()) < p.rs.s.NumObjs() && p.cardName(res.Tgt()) == "Memnite" {
			p.st.artTgt++
		}
	}
	return res
}

func (p *firegrantProbe) cardName(r Ref) string {
	return p.tbl.Cards[p.rs.s.ObjCard(dirac.Obj(r))].Name
}

// firegrantRun plays seeds 1..seeds over firegrantDecks. With mutate the
// Palace's granted trigger adds 3 red instead of 4 (a table-only mutation).
func firegrantRun(t *testing.T, seeds uint64, report, mutate bool) (diverged bool, stalls int, st firegrantStats) {
	t.Helper()
	reg, set, tbl, decks := firebendSetupDecks(t, firegrantDecks)
	if mutate {
		mt := *tbl
		mt.Cards = slices.Clone(tbl.Cards)
		hit := false
		for i := range mt.Cards {
			if mt.Cards[i].Name != "Fire Nation Palace" {
				continue
			}
			mt.Cards[i].Abilities = slices.Clone(mt.Cards[i].Abilities)
			for j := range mt.Cards[i].Abilities {
				a := &mt.Cards[i].Abilities[j]
				if a.AFlags&kop.AfLatent == 0 || len(a.Prog) == 0 || a.Prog[0].Op != kop.OpAddMana || a.Prog[0].A != kop.Const(4) {
					continue
				}
				a.Prog = slices.Clone(a.Prog)
				a.Prog[0].A = kop.Const(3)
				hit = true
			}
		}
		if !hit {
			t.Fatal("no Palace latent AddMana 4 to mutate")
		}
		tbl = &mt
	}
	for seed := uint64(1); seed <= seeds; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"firegrant-a", "firegrant-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		probe := &firegrantProbe{st: &st, seen: map[[4]int]bool{}, multiSrc: map[dirac.Obj]bool{}, mixSrc: map[dirac.Obj]bool{}}
		probe.firebendProbe = firebendProbe{st: &st.firebendStats, base: RandomPolicy(seed), r: rand.New(rand.NewPCG(seed, 0x66676e74)), rs: k.(*realSide), tbl: tbl}
		res := Lockstep(m, seed, k, PolicyFunc(probe.pick), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20})
		CloseSide(k)
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			stalls++
		}
		if d := res.Div; d != nil {
			diverged = true
			if report {
				lo := max(d.At-12, 0)
				t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:d.At]))
				t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
			return
		}
	}
	return
}

func TestFiregrantLockstep(t *testing.T) {
	t.Parallel()
	div, stalls, st := firegrantRun(t, 256, true, false)
	t.Logf("256 games, firegrant %+v, %d divergences, %d stalls", st, b2i(div), stalls)
	if stalls != 0 {
		t.Errorf("%d stalled games", stalls)
	}
	if st.granted < 20 {
		t.Errorf("granted Firebending triggers: %d, want at least 20", st.granted)
	}
	if st.mixed < 3 {
		t.Errorf("creatures firing Cadets' and Palace's grants together: %d, want at least 3", st.mixed)
	}
	if st.artGranted < 20 || st.artTgt < 20 {
		t.Errorf("artifact-creature grant targets %d, granted triggers of one %d: want at least 20 of each", st.artTgt, st.artGranted)
	}
	if st.activateFloat < 5 {
		t.Errorf("Palace activations with firebending red floating: %d, want at least 5", st.activateFloat)
	}
	if st.multi < 3 {
		t.Errorf("declarations with a creature firing 2+ Firebending triggers: %d, want at least 3", st.multi)
	}
}

// TestFiregrantMutation: a granted trigger that adds the wrong amount must
// make the lockstep diverge.
func TestFiregrantMutation(t *testing.T) {
	t.Parallel()
	if diverged, _, _ := firegrantRun(t, 64, false, true); !diverged {
		t.Fatal("Palace granted AddMana 4 -> 3: no divergence in 64 seeds")
	}
}
