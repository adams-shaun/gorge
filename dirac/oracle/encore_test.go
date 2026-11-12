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
// end step (CpSacAtEndDly, gorge's one delayed trigger, CR 603.10a).
//
// The pool is every corpus card carrying K:Encore that kgen lowers (22 of
// the 26; the other four are refused for unrelated mechanisms), so every
// card Encore newly built is played here. No other card in the pool
// activates from the graveyard, carries a this-turn attack duty or a copy's
// end-step sacrifice, so every graveyard activation offered is one card's
// encore, every Required attacker naming one of these cards is its minted
// copy, and the mutation tests below drop only Encore's marks.
var kencoreNames = []string{
	"Amphin Mutineer", "Angel of Indemnity", "Belonging", "Briarblade Adept",
	"Broodmate Tyrant", "Coastline Marauders", "Elvish Dreadlord",
	"Exquisite Huntmaster", "Fathom Fleet Swordjack", "Fin-Clade Fugitives",
	"Impulsive Pilferer", "Impulsivity", "Jubilation", "Kangee's Lieutenant",
	"Kinsbaile Courier", "Lamentation", "Mist Dancer", "Phyrexian Triniform",
	"Soul of Eternity", "Spellbinding Soprano", "Subterfuge", "Trove Tracker",
}

var kencoreCards = func() map[string]bool {
	m := make(map[string]bool, len(kencoreNames))
	for _, n := range kencoreNames {
		m[n] = true
	}
	return m
}()

// kencoreGroup is one focused lockstep over a few lowered Encore cards, four
// copies of each, so each is drawn, cast, killed and activated often enough
// across the seeds, with a land base that reaches the expensive encores.
// All 22 cards appear in exactly one group.
type kencoreGroup struct {
	name  string
	cards []string
	lands []cardset.Entry
}

var kencoreGroups = []kencoreGroup{
	{
		name: "big",
		cards: []string{
			"Amphin Mutineer", "Angel of Indemnity", "Belonging", "Broodmate Tyrant",
		},
		lands: []cardset.Entry{
			{Name: "Plains", Count: 6}, {Name: "Island", Count: 6},
			{Name: "Swamp", Count: 4}, {Name: "Mountain", Count: 4}, {Name: "Forest", Count: 4},
		},
	},
	{
		// Its {9} cast and {12} encore are the hardest to reach, so it gets
		// its own mana-flooded deck rather than a few slots in another group.
		name:  "triniform",
		cards: []string{"Phyrexian Triniform"},
		lands: []cardset.Entry{{Name: "Forest", Count: 36}},
	},
	{
		// Its {6}{R} cast and {7}{R}{R} encore are the red group's hardest,
		// so it gets its own mana-flooded deck rather than finishing last
		// in a shared one.
		name:  "impulsivity",
		cards: []string{"Impulsivity"},
		lands: []cardset.Entry{{Name: "Mountain", Count: 36}},
	},
	{
		name: "red",
		cards: []string{
			"Impulsive Pilferer", "Spellbinding Soprano", "Coastline Marauders",
			"Fathom Fleet Swordjack",
		},
		lands: []cardset.Entry{{Name: "Mountain", Count: 24}},
	},
	{
		name:  "black",
		cards: []string{"Briarblade Adept", "Elvish Dreadlord", "Exquisite Huntmaster", "Lamentation"},
		lands: []cardset.Entry{{Name: "Swamp", Count: 24}},
	},
	{
		name:  "greenwhite",
		cards: []string{"Fin-Clade Fugitives", "Jubilation", "Kangee's Lieutenant", "Kinsbaile Courier"},
		lands: []cardset.Entry{{Name: "Plains", Count: 12}, {Name: "Forest", Count: 12}},
	},
	{
		name:  "bluewhite",
		cards: []string{"Mist Dancer", "Soul of Eternity", "Subterfuge", "Trove Tracker"},
		lands: []cardset.Entry{{Name: "Plains", Count: 12}, {Name: "Island", Count: 12}},
	},
}

// kencoreDecks builds the 40-card deck both seats play for one group.
func kencoreDecks(g kencoreGroup) [2][]cardset.Entry {
	d := make([]cardset.Entry, 0, 40)
	for _, n := range g.cards {
		d = append(d, cardset.Entry{Name: n, Count: 4})
	}
	d = append(d, g.lands...)
	return [2][]cardset.Entry{append([]cardset.Entry(nil), d...), append([]cardset.Entry(nil), d...)}
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
	seeds := uint64(200)
	if v := os.Getenv("KENCORE_SEEDS"); v != "" {
		if n, err := strconv.ParseUint(v, 10, 64); err == nil && n > 0 {
			seeds = n
		}
	}
	st := &kencoreStats{}
	skipped := 0
	for _, g := range kencoreGroups {
		t.Run(g.name, func(t *testing.T) {
			defer func() {
				if t.Skipped() {
					skipped++
				}
			}()
			lockstepRunPolicy(t, "encore", kencoreDecks(g), seeds, true, nil, nil, kencorePolicy(st))
		})
	}
	// Without a corpus every group skips; the counts below would then judge games that never ran (fresh-clone-check).
	if skipped == len(kencoreGroups) {
		t.Skip("every encore group skipped (no corpus)")
	}
	t.Logf("%d encore activations, %d attack requirements on encore copies", len(st.acts), st.marks)
	for _, name := range kencoreNames {
		t.Logf("%s: encore activated in %d games", name, st.acts[name])
		if st.acts[name] < 1 {
			t.Errorf("%s: encore activated in %d games, want >= 1", name, st.acts[name])
		}
	}
	if st.marks < 20 {
		t.Errorf("only %d attack requirements on encore copies, want >= 20", st.marks)
	}
}

// TestEncoreLockstepMutationAttack: with the copies' attack duty dropped,
// some seed diverges.
func TestEncoreLockstepMutationAttack(t *testing.T) {
	t.Parallel()
	n := lockstepRun(t, "encore", kencoreDecks(kencoreGroups[1]), 150, false, kencoreDropAttack)
	if n == 0 {
		t.Fatal("no seed diverged with the copies' attack duty dropped")
	}
}

// TestEncoreLockstepMutationSac: with the group's end-step sacrifice
// dropped (CpSacAtEndDly; the pool's only copy rider of that kind is
// Encore's), some seed diverges.
func TestEncoreLockstepMutationSac(t *testing.T) {
	t.Parallel()
	n := lockstepRun(t, "encore", kencoreDecks(kencoreGroups[1]), 150, false, kencoreDropSac)
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
