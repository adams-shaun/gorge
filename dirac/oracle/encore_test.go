package oracle

import (
	"os"
	"strconv"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/state"
)

// The Encore lockstep (CR 702.141a; gorge kw_encore.go's expansion, lowered
// by dirac/kgen/lower_encore.go): a creature dies, its encore ability is
// activated from the graveyard, the token copy attacks its opponent this
// turn if able (the mark the kernel auto-declares, ExtAttacksIfAble) and
// the whole activation's group is sacrificed at the beginning of the next
// end step (CpSacAtEndDly, gorge's one delayed trigger, CR 603.10a). The
// six corpus cards carrying K:Encore are the pool; no other card here
// activates from the graveyard, so every graveyard activation offered is
// one card's encore, and every Required attacker naming one of the six
// cards is its minted copy.
var kencoreCards = map[string]bool{
	"Impulsive Pilferer":  true,
	"Amphin Mutineer":     true,
	"Angel of Indemnity":  true,
	"Phyrexian Triniform": true,
	"Belonging":           true,
	"Broodmate Tyrant":    true,
}

var kencoreDecks = [2][]cardset.Entry{
	{
		{Name: "Impulsive Pilferer", Count: 8}, {Name: "Amphin Mutineer", Count: 4},
		{Name: "Belonging", Count: 2}, {Name: "Broodmate Tyrant", Count: 2},
		{Name: "Angel of Indemnity", Count: 2}, {Name: "Phyrexian Triniform", Count: 1},
		{Name: "Grizzly Bears", Count: 1}, {Name: "Murder", Count: 3},
		{Name: "Mountain", Count: 5}, {Name: "Island", Count: 4}, {Name: "Plains", Count: 4},
		{Name: "Swamp", Count: 2}, {Name: "Forest", Count: 2},
	},
	{
		{Name: "Impulsive Pilferer", Count: 6}, {Name: "Amphin Mutineer", Count: 4},
		{Name: "Belonging", Count: 2}, {Name: "Broodmate Tyrant", Count: 2},
		{Name: "Angel of Indemnity", Count: 2}, {Name: "Phyrexian Triniform", Count: 1},
		{Name: "Raging Goblin", Count: 3}, {Name: "Hill Giant", Count: 2}, {Name: "Murder", Count: 3},
		{Name: "Mountain", Count: 5}, {Name: "Island", Count: 3}, {Name: "Plains", Count: 3},
		{Name: "Swamp", Count: 2}, {Name: "Forest", Count: 2},
	},
}

type kencoreStats struct {
	acts  map[string]int // per card: priority decisions offering its graveyard encore activation
	marks int            // attackers decisions offering a Required encore copy
}

// kencorePolicy casts, activates and plays lands whenever it can (so the
// creatures die, reach the graveyard and their encore is activated), else
// random; it counts, from gorge's options, the activations and the copies'
// attack requirements.
func kencorePolicy(st *kencoreStats) func(uint64) Policy {
	return func(seed uint64) Policy {
		rnd := RandomPolicy(seed)
		counted := map[int]string{} // canonical decision -> card counted this game
		return PolicyFunc(func(c *PickCtx) Canon {
			if c.G != nil && c.D.Kind == DAttackers {
				for _, a := range c.D.Options {
					if a.Kind() != ActAttack {
						continue
					}
					o, ok := c.G.GorgeOption(a)
					if !ok || !o.Required {
						continue
					}
					if ob := c.G.E.G.Obj(o.Obj); ob != nil && ob.Face() != nil && kencoreCards[ob.Face().Name] {
						st.marks++
						break
					}
				}
			}
			if c.G != nil && c.D.Kind == DPriority && counted[c.Seq] == "" {
				for _, a := range c.D.Options {
					if a.Kind() != ActActivate {
						continue
					}
					o, ok := c.G.GorgeOption(a)
					if !ok || o.Kind != "ability" {
						continue
					}
					ob := c.G.E.G.Obj(o.Obj)
					if ob == nil || ob.Face() == nil || ob.Zone != state.ZGraveyard || !kencoreCards[ob.Face().Name] {
						continue
					}
					name := ob.Face().Name
					if st.acts == nil {
						st.acts = map[string]int{}
					}
					st.acts[name]++
					counted[c.Seq] = name
					break
				}
			}
			buf := make([]Canon, 0, 8)
			for _, a := range c.D.Options {
				switch a.Kind() {
				case ActCast, ActCastFace, ActActivate, ActPlayLand:
					buf = append(buf, a)
				}
			}
			if len(buf) == 0 || len(buf) == len(c.D.Options) {
				return rnd.Pick(c)
			}
			d := *c.D
			d.Options = buf
			return rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
		})
	}
}

func TestEncoreLockstep(t *testing.T) {
	t.Parallel()
	seeds := uint64(500)
	if v := os.Getenv("KENCORE_SEEDS"); v != "" {
		if n, err := strconv.ParseUint(v, 10, 64); err == nil && n > 0 {
			seeds = n
		}
	}
	st := &kencoreStats{}
	lockstepRunPolicy(t, "encore", kencoreDecks, seeds, true, nil, nil, kencorePolicy(st))
	t.Logf("%d encore activations, %d attack requirements on encore copies", len(st.acts), st.marks)
	for _, name := range []string{
		"Impulsive Pilferer", "Amphin Mutineer", "Angel of Indemnity",
		"Phyrexian Triniform", "Belonging", "Broodmate Tyrant",
	} {
		if st.acts[name] < 1 {
			t.Errorf("%s: encore activated in %d games, want >= 1", name, st.acts[name])
		}
	}
	if st.marks < 5 {
		t.Errorf("only %d attack requirements on encore copies, want >= 5", st.marks)
	}
}

// TestEncoreLockstepMutationAttack: with the copies' attack duty dropped,
// some seed diverges.
func TestEncoreLockstepMutationAttack(t *testing.T) {
	t.Parallel()
	n := lockstepRun(t, "encore", kencoreDecks, 150, false, kencoreDropAttack)
	if n == 0 {
		t.Fatal("no seed diverged with the copies' attack duty dropped")
	}
}

// TestEncoreLockstepMutationSac: with the group's end-step sacrifice
// dropped (CpSacAtEndDly; the pool's only copy rider of that kind is
// Encore's), some seed diverges.
func TestEncoreLockstepMutationSac(t *testing.T) {
	t.Parallel()
	n := lockstepRun(t, "encore", kencoreDecks, 150, false, kencoreDropSac)
	if n == 0 {
		t.Fatal("no seed diverged with the copies' end-step sacrifice dropped")
	}
}

// kencoreDropAttack Nops every ExtAttacksIfAble mark (the copies' "attacks
// that opponent this turn if able").
func kencoreDropAttack(tbl *kop.Table) {
	kencoreWalk(tbl, func(p *kop.Instr) {
		if p.Op == kop.OpExt && p.Ext() == kop.ExtAttacksIfAble {
			*p = kop.Instr{Op: kop.OpNop}
		}
	})
}

// kencoreDropSac clears OpCopyAsTokenEx's CpSacAtEndDly rider (the copies'
// next-end-step sacrifice; the pool's only copy rider of that kind is
// Encore's).
func kencoreDropSac(tbl *kop.Table) {
	kencoreWalk(tbl, func(p *kop.Instr) {
		if p.Op == kop.OpCopyAsTokenEx && p.B.K == kop.VConst {
			cf := uint16(p.B.N)
			if cf&uint16(kop.CpSacAtEndDly) != 0 {
				p.B = kop.Const(int(int16(cf &^ uint16(kop.CpSacAtEndDly))))
			}
		}
	})
}

func kencoreWalk(tbl *kop.Table, f func(*kop.Instr)) {
	cards := append([]kop.CardDef(nil), tbl.Cards...)
	for i := range cards {
		abs := append([]kop.Ability(nil), cards[i].Abilities...)
		for j := range abs {
			p := append([]kop.Instr(nil), abs[j].Prog...)
			for x := range p {
				f(&p[x])
			}
			abs[j].Prog = p
		}
		cards[i].Abilities = abs
	}
	tbl.Cards = cards
}
