package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
)

// The CopyPermanent copy rider lockstep: AddTypes$ (a card
// type, a supertype, a subtype, "A & B" lists), AddColors$ and
// PumpKeywords$. The first two are copiable values (CR 707.9b, 707.3): a
// Cackling Counterpart or Replication Technique copy of a Molten
// Duplication / Saheeli token is an artifact too (Master of Etherium counts
// it); PumpKeywords$ is not, so that copy has no haste. Deck A1 holds the
// red/blue copy spells, A2 the green/white ones (Adagia, Vaultborn Tyrant);
// the attackers include the hasty tokens made that turn.

var kcopyridersDecks = [][2][]cardset.Entry{{
	{
		{Name: "Molten Duplication", Count: 3}, {Name: "Saheeli, the Sun's Brilliance", Count: 3},
		{Name: "Devastating Onslaught", Count: 3}, {Name: "The Jolly Balloon Man", Count: 3},
		{Name: "Cackling Counterpart", Count: 4}, {Name: "Relm's Sketching", Count: 2},
		{Name: "Master of Etherium", Count: 3}, {Name: "Ornithopter", Count: 3}, {Name: "Bronze Sable", Count: 2},
		{Name: "Mountain", Count: 8}, {Name: "Island", Count: 6},
	},
	{
		{Name: "Lightning Bolt", Count: 4}, {Name: "Shock", Count: 3}, {Name: "Hill Giant", Count: 6},
		{Name: "Gray Ogre", Count: 6}, {Name: "Mogg Fanatic", Count: 3}, {Name: "Bronze Sable", Count: 2},
		{Name: "Mountain", Count: 16},
	},
}, {
	{
		{Name: "Vaultborn Tyrant", Count: 4}, {Name: "Adagia, Windswept Bastion", Count: 4},
		{Name: "Grizzly Bears", Count: 5}, {Name: "Giant Spider", Count: 4}, {Name: "Ornithopter", Count: 4},
		{Name: "Bonesplitter", Count: 3}, {Name: "Thran Golem", Count: 3},
		{Name: "Forest", Count: 8}, {Name: "Plains", Count: 5},
	},
	{
		{Name: "Grizzly Bears", Count: 6}, {Name: "Giant Spider", Count: 5}, {Name: "Hill Giant", Count: 5},
		{Name: "Gray Ogre", Count: 5}, {Name: "Lightning Bolt", Count: 3},
		{Name: "Forest", Count: 8}, {Name: "Mountain", Count: 8},
	},
}}

// kcopyridersCopiers are the copy cards with riders; kcopyridersPlain copy
// without one, so a copy of a rider token inherits what the rider copied.
var kcopyridersCopiers = map[string]bool{
	"Molten Duplication": true, "Saheeli, the Sun's Brilliance": true, "Devastating Onslaught": true,
	"The Jolly Balloon Man": true, "Vaultborn Tyrant": true, "Adagia, Windswept Bastion": true,
}

var kcopyridersPlain = map[string]bool{"Cackling Counterpart": true, "Relm's Sketching": true}

// kcopyridersCounts counts the policy's plays.
type kcopyridersCounts struct{ copiers, hasteAttacks, riderTurn int }

// kcopyridersPolicy plays a copy spell or ability whenever it can (else a
// random cast or land) and attacks with every creature, the tokens made
// this turn among them.
func kcopyridersPolicy(seed uint64, st *kcopyridersCounts) Policy {
	rnd := RandomPolicy(seed)
	var buf []Canon
	return PolicyFunc(func(c *PickCtx) Canon {
		buf = buf[:0]
		var cp, rider, tok []Canon
		for _, a := range c.D.Options {
			switch a.Kind() {
			case ActAttack:
				buf = append(buf, a)
				if o, ok := c.G.GorgeOption(a); ok {
					if ob := c.G.E.G.Obj(o.Obj); ob != nil && ob.IsToken && ob.EnteredThisTurn {
						st.hasteAttacks++
					}
				}
			case ActCast, ActActivate:
				n := optionFaceName(c, a)
				if kcopyridersCopiers[n] || kcopyridersPlain[n] && c.G != nil && int(c.G.E.G.Turn) == st.riderTurn {
					cp = append(cp, a)
					if kcopyridersCopiers[n] {
						rider = append(rider, a)
					}
				}
				buf = append(buf, a)
			case ActTarget:
				if o, ok := c.G.GorgeOption(a); ok {
					if ob := c.G.E.G.Obj(o.Obj); ob != nil && ob.IsToken {
						tok = append(tok, a)
					}
				}
				buf = append(buf, a)
			case ActPlayLand:
				buf = append(buf, a)
			}
		}
		pick := func(opts []Canon) Canon {
			d := *c.D
			d.Options = opts
			return rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
		}
		var a Canon
		switch {
		case len(tok) > 0:
			a = pick(tok) // copy a token the rider made
		case len(rider) > 0:
			st.copiers++
			a = pick(rider)
			if c.G != nil {
				st.riderTurn = int(c.G.E.G.Turn)
			}
		case len(cp) > 0:
			st.copiers++
			a = pick(cp)
		case len(buf) > 0:
			a = pick(buf)
		default:
			a = rnd.Pick(c)
		}
		return a
	})
}

func runCopyriders(t *testing.T, seeds int, mutate func(), reset func()) (games, divs, stalls int, st dirac.CopyRidersStats, pc kcopyridersCounts) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("copyriders")
	if err != nil {
		t.Fatal(err)
	}
	dirac.DebugCopyRidersStats = &st
	defer func() { dirac.DebugCopyRidersStats = nil }()
	if mutate != nil {
		mutate()
		defer reset()
	}
	for mi, md := range kcopyridersDecks {
		var decks [2][]*cards.Card
		for s, d := range md {
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
				t.Fatalf("matchup %d deck %d: %d cards", mi, s, len(decks[s]))
			}
		}
		tbl, _, _, err := set.Matchup(md[0], md[1])
		if err != nil {
			t.Fatal(err)
		}
		for seed := uint64(1); seed <= uint64(seeds); seed++ {
			cfg := Config{GorgeAutopaySurface: seed%2 == 0}
			m, err := NewMatchup(reg, [2]string{"kcopyriders-a", "kcopyriders-b"}, decks, 1)
			if err != nil {
				t.Fatal(err)
			}
			k, err := SetSide(set, tbl, cfg)(m, seed)
			if err != nil {
				t.Fatal(err)
			}
			g, err := NewGorgeSide(m, seed, cfg, nil)
			if err != nil {
				t.Fatal(err)
			}
			res := LockstepLed(m, seed, g, k, kcopyridersPolicy(seed, &pc), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20})
			CloseSide(k)
			games++
			if strings.HasPrefix(res.Stall, KernelStallPrefix) {
				stalls++
				t.Logf("matchup %d seed %d: %s", mi, seed, res.Stall)
			}
			if d := res.Div; d != nil {
				divs++
				if mutate == nil {
					t.Errorf("matchup %d seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", mi, seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
				}
			}
		}
	}
	return games, divs, stalls, st, pc
}

func TestCopyridersLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	games, divs, stalls, st, pc := runCopyriders(t, 256, nil, nil)
	t.Logf("%d games, %d divergences, %d stalls, copy-spell picks %d, copies with an exception record %d, copies of a copy with an AddTypes/AddColors template %d, PumpKeywords grants %d, attacks by tokens made that turn %d",
		games, divs, stalls, pc.copiers, st.Own, st.Inherited, st.Pumped, pc.hasteAttacks)
	if divs != 0 {
		t.Fatalf("%d divergences", divs)
	}
	if stalls != 0 {
		t.Errorf("%d kernel stalls", stalls)
	}
	if st.Own < 40 {
		t.Errorf("only %d copies made with an exception record (need 40)", st.Own)
	}
	if st.Inherited < 10 {
		t.Errorf("only %d copies of a copy with an AddTypes/AddColors template (need 10)", st.Inherited)
	}
	if pc.hasteAttacks < 10 {
		t.Errorf("only %d attacks by a token made that turn (need 10)", pc.hasteAttacks)
	}
}

// TestCopyridersMutation makes the PumpKeywords$ haste copiable
// (dirac.DebugCopyRidersPumpCopiable): a copy of a hasty token then has
// haste in the kernel only, so the games must diverge.
func TestCopyridersMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	games, divs, _, _, _ := runCopyriders(t, 256,
		func() { dirac.DebugCopyRidersPumpCopiable = true },
		func() { dirac.DebugCopyRidersPumpCopiable = false })
	if divs == 0 {
		t.Fatalf("mutation made the haste grant copiable in %d games with no divergence", games)
	}
	t.Logf("mutation diverged in %d of %d games", divs, games)
}

// TestCopyridersMutationNoTypes ignores a copy exception's AddTypes
// (dirac.DebugCopyRidersNoTypes): the tokens are no artifacts, so Master of
// Etherium's count and the copies of the copies differ and the games must
// diverge.
func TestCopyridersMutationNoTypes(t *testing.T) {
	if isolate(t) {
		return
	}
	games, divs, _, _, _ := runCopyriders(t, 256,
		func() { dirac.DebugCopyRidersNoTypes = true },
		func() { dirac.DebugCopyRidersNoTypes = false })
	if divs == 0 {
		t.Fatalf("mutation ignored AddTypes in %d games with no divergence", games)
	}
	t.Logf("mutation diverged in %d of %d games", divs, games)
}
