package oracle

import (
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// The Partner with [name] lockstep (CR 702.124j): every corpus carrier dirac
// supports (the partnerwith pool) enters with gorge kwPartnerWith's trigger, "target player may search
// their library for a card named [name], reveal it, put it into their hand,
// then shuffle". The controller targets either seat; the targeted player
// answers the search confirmation (Optional$ True) and the pick over their
// own library (DefinedPlayer$ and Chooser$ Targeted); a partner the pool does
// not hold matches nothing. Each deck is a colour group of carriers, both
// halves of a pair where both lower, and both seats play it, so the searched
// library holds the partner whichever seat is targeted.

// pwDecks are the carriers by colour group, both halves of a pair in one
// deck; a carrier whose partner the pool does not hold finds nothing.
var pwDecks = map[string][]cardset.Entry{
	"wur": {
		{Name: "Alisaie Leveilleur", Count: 2}, {Name: "Alphinaud Leveilleur", Count: 2},
		{Name: "Brallin, Skyshark Rider", Count: 2}, {Name: "Shabraz, the Skyshark", Count: 2},
		{Name: "Chakram Retriever", Count: 2}, {Name: "Chakram Slinger", Count: 2},
		{Name: "Impetuous Protege", Count: 2}, {Name: "Proud Mentor", Count: 2},
		{Name: "Jenny Flint", Count: 2}, {Name: "Evie Frye", Count: 2},
		{Name: "Timin, Youthful Geist", Count: 2}, {Name: "Toothy, Imaginary Friend", Count: 2},
		{Name: "Island", Count: 6}, {Name: "Plains", Count: 5}, {Name: "Mountain", Count: 5},
	},
	"bwr": {
		{Name: "Blaring Captain", Count: 3}, {Name: "Blaring Recruiter", Count: 3},
		{Name: "Kamber, the Plunderer", Count: 3}, {Name: "Laurine, the Diversion", Count: 3},
		{Name: "Silvar, Devourer of the Free", Count: 3}, {Name: "Trynn, Champion of Freedom", Count: 3},
		{Name: "Nikara, Lair Scavenger", Count: 3}, {Name: "Virtus the Veiled", Count: 3},
		{Name: "Swamp", Count: 7}, {Name: "Plains", Count: 5}, {Name: "Mountain", Count: 4},
	},
	"gub": {
		{Name: "Cazur, Ruthless Stalker", Count: 3}, {Name: "Ukkima, Stalking Shadow", Count: 3},
		{Name: "Ley Weaver", Count: 3}, {Name: "Lore Weaver", Count: 3},
		{Name: "Soulblade Corrupter", Count: 3}, {Name: "Soulblade Renewer", Count: 3},
		{Name: "Rocksteady, Mutant Marauder", Count: 3}, {Name: "Llanowar Elves", Count: 3},
		{Name: "Forest", Count: 7}, {Name: "Island", Count: 5}, {Name: "Swamp", Count: 4},
	},
	"gwb": {
		{Name: "Merry, Warden of Isengard", Count: 4}, {Name: "Pippin, Warden of Isengard", Count: 4},
		{Name: "Sam, Loyal Attendant", Count: 4}, {Name: "Llanowar Elves", Count: 4}, {Name: "Grizzly Bears", Count: 7},
		{Name: "Forest", Count: 7}, {Name: "Plains", Count: 5}, {Name: "Swamp", Count: 5},
	},
}

// pwPartner is each carrier's named partner, read from its K:Partner with
// line (the full printed name before any short-name alias).
func pwPartner(c *cards.Card) string {
	for _, k := range c.Faces[0].Keywords {
		if cards.KeywordHead(k) != "Partner with" {
			continue
		}
		_, p, _ := strings.Cut(k, ":")
		name, _, _ := strings.Cut(p, ":")
		return strings.TrimSpace(name)
	}
	return ""
}

// pwTally is a run's per-carrier counts: casts from the hand and from the
// command zone, search confirmations posed (and to the opponent of the
// source's controller, and declined), and picks of the named partner.
type pwTally struct {
	cast, cmdCast, confirm, oppConfirm, declined, found map[string]int
}

func newPwTally() *pwTally {
	return &pwTally{cast: map[string]int{}, cmdCast: map[string]int{}, confirm: map[string]int{},
		oppConfirm: map[string]int{}, declined: map[string]int{}, found: map[string]int{}}
}

// pwPolicy is eagerPolicy's cast-first random policy that tallies the
// Partner-with decisions as gorge poses them: the search confirmation and the
// search whose source is a carrier, the asked seat, and the answer.
func pwPolicy(seed uint64, partner map[string]string, n *pwTally) Policy {
	inner := eagerPolicy(seed, map[string]int{})
	return PolicyFunc(func(c *PickCtx) Canon {
		a := inner.Pick(c)
		if c.G == nil {
			return a
		}
		d := c.G.Pending()
		if d == nil {
			return a
		}
		o, ok := c.G.GorgeOption(a)
		if !ok {
			return a
		}
		g := c.G.E.G
		if o.Kind == "cast" {
			if ob := g.Obj(o.Obj); ob != nil && ob.Face() != nil && partner[ob.Face().Name] != "" {
				switch ob.Zone {
				case state.ZHand:
					n.cast[ob.Face().Name]++
				case state.ZCommand:
					n.cmdCast[ob.Face().Name]++
				}
			}
			return a
		}
		src := g.Obj(d.Source)
		if src == nil || src.Face() == nil || partner[src.Face().Name] == "" {
			return a
		}
		name := src.Face().Name
		switch d.ResumeKind {
		case "search_confirm":
			n.confirm[name]++
			if d.Player != src.Controller {
				n.oppConfirm[name]++
			}
			if o.Kind == "no" {
				n.declined[name]++
			}
		case "search":
			if ob := g.Obj(o.Obj); o.Obj != 0 && ob != nil && ob.Face() != nil {
				if ob.Face().Name != partner[name] {
					panic(fmt.Sprintf("%s's search picked %q, not its partner %q", name, ob.Face().Name, partner[name]))
				}
				n.found[name]++
			}
		}
		return a
	})
}

// pwRun is one run over the partnerwith pool: a deck a seat, and, for a
// Commander duel, each seat's commander pair by name (the first copy of
// each in its deck).
type pwRun struct {
	decks [2][]cardset.Entry
	pairs *[2][2]string
	seeds uint64
	// mutate, when set, edits the kernel table first; the run then stops at
	// the first divergence.
	mutate func(*testing.T, *kop.Table)
}

// play runs the seeded games and returns how many were played and diverged,
// the tally and each carrier's named partner.
func (r pwRun) play(t *testing.T) (games, divs int, n *pwTally, partner map[string]string) {
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("partnerwith")
	if err != nil {
		t.Fatal(err)
	}
	partner = map[string]string{}
	var decks [2][]*cards.Card
	cmd := [2][2]int{{-1, -1}, {-1, -1}}
	for s := range decks {
		for _, e := range r.decks[s] {
			c, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s not in the corpus", e.Name)
			}
			if p := pwPartner(c); p != "" {
				partner[e.Name] = p
			}
			for h := 0; r.pairs != nil && h < 2; h++ {
				if r.pairs[s][h] == e.Name && cmd[s][h] < 0 {
					cmd[s][h] = len(decks[s])
				}
			}
			for i := 0; i < e.Count; i++ {
				decks[s] = append(decks[s], c)
			}
		}
		if len(decks[s]) != 40 {
			t.Fatalf("deck %d: %d cards", s, len(decks[s]))
		}
		if r.pairs != nil && (cmd[s][0] < 0 || cmd[s][1] < 0) {
			t.Fatalf("seat %d: commanders %v not in its deck", s, r.pairs[s])
		}
	}
	tbl, _, _, err := set.Matchup(r.decks[0], r.decks[1])
	if err != nil {
		t.Fatal(err)
	}
	if r.mutate != nil {
		r.mutate(t, tbl)
	}
	n = newPwTally()
	for seed := uint64(1); seed <= r.seeds; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"kpw-a", "kpw-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		if r.pairs != nil {
			m.Commander = true
			m.Commanders, m.Partners = [2]int{cmd[0][0], cmd[1][0]}, [2]int{cmd[0][1], cmd[1][1]}
		}
		lead, err := NewGorgeSide(m, seed, cfg, nil)
		if err != nil {
			t.Fatal(err)
		}
		for s := 0; r.pairs != nil && s < 2; s++ {
			// CR 702.124j: gorge seats a mutual Partner-with pair (deck.IsPartnerPair);
			// a refused pair would play commander-less
			pl := &lead.E.G.Players[s]
			if len(pl.Commanders) != 2 || lead.E.G.Obj(pl.Commanders[0]).Zone != state.ZCommand || lead.E.G.Obj(pl.Commanders[1]).Zone != state.ZCommand {
				t.Fatalf("seed %d seat %d: gorge seated %v, want %v in the command zone", seed, s, pl.Commanders, r.pairs[s])
			}
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		res := LockstepLed(m, seed, lead, k, pwPolicy(seed, partner, n), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 25})
		CloseSide(k)
		games++
		if d := res.Div; d != nil {
			divs++
			if r.mutate != nil {
				return
			}
			lo := d.At - 12
			if lo < 0 {
				lo = 0
			}
			t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:d.At]))
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			return
		}
	}
	return
}

// pwMin is each carrier's floor of casts and of search confirmations.
const pwMin = 3

func TestPartnerWithLockstep(t *testing.T) {
	t.Parallel()
	names := make([]string, 0, len(pwDecks))
	for k := range pwDecks {
		names = append(names, k)
	}
	sort.Strings(names)
	for _, dn := range names {
		deck := pwDecks[dn]
		t.Run(dn, func(t *testing.T) {
			t.Parallel()
			games, divs, n, partner := pwRun{decks: [2][]cardset.Entry{deck, deck}, seeds: 60}.play(t)
			if divs != 0 {
				return
			}
			var carriers []string
			for c := range partner {
				carriers = append(carriers, c)
			}
			sort.Strings(carriers)
			inDeck := map[string]bool{}
			for _, e := range deck {
				inDeck[e.Name] = true
			}
			opp, declined := 0, 0
			var line []string
			for _, c := range carriers {
				line = append(line, fmt.Sprintf("%s cast=%d confirm=%d opp=%d declined=%d found=%d",
					c, n.cast[c], n.confirm[c], n.oppConfirm[c], n.declined[c], n.found[c]))
				opp += n.oppConfirm[c]
				declined += n.declined[c]
				if n.cast[c] < pwMin || n.confirm[c] < pwMin {
					t.Errorf("%s: cast %d, confirmations %d, want at least %d each", c, n.cast[c], n.confirm[c], pwMin)
				}
				if inDeck[partner[c]] && n.found[c] == 0 {
					t.Errorf("%s: its partner %s never found", c, partner[c])
				}
				if !inDeck[partner[c]] && n.found[c] != 0 {
					t.Errorf("%s: found %d of %s, which the deck does not hold", c, n.found[c], partner[c])
				}
			}
			t.Logf("%d games:\n  %s", games, strings.Join(line, "\n  "))
			if opp == 0 || declined == 0 {
				t.Errorf("an opponent's search %d, a declined search %d: want both", opp, declined)
			}
		})
	}
}

// TestPartnerWithCommanderLockstep plays Commander duels led by mutual
// Partner-with pairs (CR 702.124j's first ability, the pair marker): gorge
// seats both halves (deck.IsPartnerPair) as the kernel seats Config's pair,
// each half is cast from the command zone, and its enters trigger searches a
// library whose other copies of the partner are the findable ones.
func TestPartnerWithCommanderLockstep(t *testing.T) {
	t.Parallel()
	pairs := [2][2]string{{"Kamber, the Plunderer", "Laurine, the Diversion"}, {"Merry, Warden of Isengard", "Pippin, Warden of Isengard"}}
	games, divs, n, _ := pwRun{decks: [2][]cardset.Entry{pwDecks["bwr"], pwDecks["gwb"]}, pairs: &pairs, seeds: 30}.play(t)
	if divs != 0 {
		return
	}
	t.Logf("%d games: command-zone casts %v, confirmations %v, found %v", games, n.cmdCast, n.confirm, n.found)
	for _, seat := range pairs {
		for _, c := range seat {
			if n.cmdCast[c] == 0 {
				t.Errorf("%s never cast from the command zone", c)
			}
		}
	}
}

// TestPartnerWithMutation proves the pool's search is the targeted player's:
// searching the trigger controller's library instead diverges.
func TestPartnerWithMutation(t *testing.T) {
	t.Parallel()
	games, divs, _, _ := pwRun{decks: [2][]cardset.Entry{pwDecks["bwr"], pwDecks["bwr"]}, seeds: 60, mutate: func(t *testing.T, tbl *kop.Table) {
		moved := 0
		for ci := range tbl.Cards {
			for ai := range tbl.Cards[ci].Abilities {
				for pi := range tbl.Cards[ci].Abilities[ai].Prog {
					in := &tbl.Cards[ci].Abilities[ai].Prog[pi]
					if in.Op == kop.OpSearchEx && in.Obj == kop.RefTarget0 {
						in.Obj = kop.RefController
						moved++
					}
				}
			}
		}
		if moved == 0 {
			t.Fatal("mutation found no targeted player's search in the pool")
		}
	}}.play(t)
	if divs == 0 {
		t.Fatalf("mutation in %d games with no divergence", games)
	}
	t.Logf("mutation diverged in game %d", games)
}
