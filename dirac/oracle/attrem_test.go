package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The lockstep: api:Attach Object$ Remembered |
// Defined$ Self over a one-object remembered writer. U.S.Agent, John Walker
// creates a Sturdy Shield and attaches it to itself; Tony Stark's back face
// (The Invincible Iron Man) puts an artifact from its controller's hand onto
// the battlefield at the beginning of combat and attaches it only when it is
// an Equipment. Deck A carries Tony Stark, Equipment and other artifacts
// (Sol Ring and Mind Stone ramp it to the transform); deck B carries
// U.S.Agent and vanilla bodies (no instant removal: gorge reads a trigger's
// may-ness from its dead source's current face, so a Tony Stark back face
// killed with its trigger on the stack diverges; see the report). Deck B also
// carries token doublers (Anointed Procession, Parallel Lives): the
// Shield's resolution then remembers several tokens and each attaches.

var kattremDecks = [2][]cardset.Entry{
	{
		{Name: "Tony Stark", Count: 4}, {Name: "Sol Ring", Count: 2}, {Name: "Mind Stone", Count: 3}, {Name: "Wayfarer's Bauble", Count: 2},
		{Name: "Bonesplitter", Count: 3}, {Name: "Short Sword", Count: 2}, {Name: "Leather Armor", Count: 2}, {Name: "Lightning Bolt", Count: 2},
		{Name: "Island", Count: 10}, {Name: "Mountain", Count: 10},
	},
	{
		{Name: "U.S.Agent, John Walker", Count: 4}, {Name: "Grizzly Bears", Count: 5}, {Name: "Hill Giant", Count: 4}, {Name: "Llanowar Elves", Count: 3},
		{Name: "Anointed Procession", Count: 2}, {Name: "Parallel Lives", Count: 1},
		{Name: "Swamp", Count: 8}, {Name: "Plains", Count: 9}, {Name: "Forest", Count: 4},
	},
}

var (
	kattremEquip    = map[string]bool{"Bonesplitter": true, "Short Sword": true, "Bone Saw": true}
	kattremNonEquip = map[string]bool{"Sol Ring": true, "Mind Stone": true, "Wayfarer's Bauble": true}
)

// kattremTally counts, from gorge's events: Sturdy Shield attached to
// U.S.Agent, Equipment attached to the transformed Tony Stark, and
// non-Equipment artifacts it put onto the battlefield from hand (left
// unattached).
type kattremTally struct {
	evAt                    int
	shield, equip, nonEquip int
	equipMoved              int
	pending                 state.ObjID // an Equipment just put from hand, until its resolution ends
}

func (y *kattremTally) reset() { y.evAt, y.pending = 0, 0 }

func (y *kattremTally) check(ref *GorgeSide, _ Side) *Divergence {
	g := ref.E.G
	evs := ref.E.L.Events
	if len(evs) < y.evAt {
		y.evAt = 0
	}
	for j := y.evAt; j < len(evs); j++ {
		e := &evs[j]
		switch e.Kind {
		case events.Resolve:
			y.pending = 0
		case events.Attach:
			if len(e.IDs) != 1 {
				break
			}
			eq, host := chName(g, e.Obj), chName(g, e.IDs[0])
			if eq == "Sturdy Shield" && strings.HasPrefix(host, "U.S.Agent") {
				y.shield++
			}
			if e.Obj == y.pending && kattremEquip[eq] && strings.Contains(host, "Iron Man") {
				y.equip++
			}
		case events.MoveZone:
			if e.From != state.ZHand || e.To != state.ZBattlefield {
				break
			}
			n := chName(g, e.Obj)
			if kattremNonEquip[n] {
				y.nonEquip++
			}
			if kattremEquip[n] {
				y.equipMoved++
				y.pending = e.Obj
			}
		}
	}
	y.evAt = len(evs)
	return nil
}

func kattremSetup(t *testing.T) (*cards.Registry, cardset.Set) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("attrem")
	if err != nil {
		t.Fatal(err)
	}
	return reg, set
}

// kattremRetarget returns a copy of t whose every OpAttachObj attaches Self
// (its C object Ref becomes RefSelf): nothing attaches. Only the Cards and
// the touched abilities' programs are copied.
func kattremRetarget(t *kop.Table) *kop.Table {
	m := *t
	m.Cards = append([]kop.CardDef(nil), t.Cards...)
	for i := range m.Cards {
		m.Cards[i].Abilities = append([]kop.Ability(nil), t.Cards[i].Abilities...)
		for j := range m.Cards[i].Abilities {
			a := &m.Cards[i].Abilities[j]
			a.Prog = append([]kop.Instr(nil), a.Prog...)
			for k := range a.Prog {
				if a.Prog[k].Op == kop.OpAttachObj {
					a.Prog[k].C = int16(kop.RefSelf)
				}
			}
		}
	}
	return &m
}

// kattremRun plays seeds 1..seeds and returns the first divergence's seed
// (0 none). mutate rewrites the built table.
func kattremRun(t *testing.T, reg *cards.Registry, set cardset.Set, seeds uint64, mutate bool, y *kattremTally) (games int, divSeed uint64, stalls int) {
	t.Helper()
	var decks [2][]*cards.Card
	for s, d := range kattremDecks {
		for _, e := range d {
			c, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s not in the corpus", e.Name)
			}
			for i := 0; i < e.Count; i++ {
				decks[s] = append(decks[s], c)
			}
		}
		if len(decks[s]) != 40 {
			t.Fatalf("deck %d: %d cards", s, len(decks[s]))
		}
	}
	tbl, _, _, err := set.Matchup(kattremDecks[0], kattremDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	if mutate {
		tbl = kattremRetarget(tbl)
	}
	for seed := uint64(1); seed <= seeds; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"kattrem-a", "kattrem-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		y.reset()
		res := Lockstep(m, seed, k, RandomPolicy(seed), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20, Check: y.check})
		CloseSide(k)
		games++
		if res.Stall != "" && len(res.Stall) >= len(KernelStallPrefix) && res.Stall[:len(KernelStallPrefix)] == KernelStallPrefix {
			stalls++
			t.Errorf("seed %d: kernel stall %s", seed, res.Stall)
		}
		if d := res.Div; d != nil {
			if !mutate {
				lo := max(d.At-12, 0)
				t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nlast actions %v\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, actionStrings(res.Actions[lo:d.At]), d.RefOptions, d.KerOptions)
			}
			return games, seed, stalls
		}
	}
	return games, 0, stalls
}

func TestAttremLockstep(t *testing.T) {
	t.Parallel()
	reg, set := kattremSetup(t)
	y := &kattremTally{}
	games, div, stalls := kattremRun(t, reg, set, 256, false, y)
	t.Logf("%d games, diverged seed %d, stalls %d; %d Sturdy Shield attaches to U.S.Agent, %d of %d Equipment put from hand attached to Tony Stark's back face, %d non-Equipment put and left unattached",
		games, div, stalls, y.shield, y.equip, y.equipMoved, y.nonEquip)
	if div != 0 || stalls != 0 {
		return
	}
	if y.shield < 20 {
		t.Errorf("%d Sturdy Shield attaches, want at least 20", y.shield)
	}
	if y.equip != y.equipMoved {
		t.Errorf("%d of %d Equipment put from hand attached", y.equip, y.equipMoved)
	}
	if y.equip < 3 {
		t.Errorf("%d Equipment attached to Tony Stark, want at least 3", y.equip)
	}
	if y.nonEquip < 3 {
		t.Errorf("%d non-Equipment left unattached, want at least 3", y.nonEquip)
	}
}

// TestAttremMutation: a table whose OpAttachObj attaches Self (nothing)
// must diverge from gorge, which attaches the remembered object.
func TestAttremMutation(t *testing.T) {
	reg, set := kattremSetup(t)
	_, div, _ := kattremRun(t, reg, set, 64, true, &kattremTally{})
	if div == 0 {
		t.Fatalf("retargeted OpAttachObj: no divergence in 64 seeds")
	}
	t.Logf("diverged at seed %d", div)
}
