package shadow_test

import (
	"math/rand/v2"
	"sort"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/dirac/oracle"
	"github.com/adams-shaun/gorge/dirac/shadow"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/view"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// The corpus pool of the attachment and token verifier: white and red cards
// dirac supports with evidence (kgen -supported mbc), none with an ability
// the staging refuses. Pacifism is an Aura (can't attack or block), Swiftfoot
// Boots an Equipment (equip {1}, haste and hexproof), Dragon Fodder a sorcery
// that makes two 1/1 Goblin tokens. The creatures have combat keywords only.
var (
	cardsLands     = []string{"Plains", "Mountain"}
	cardsCreatures = []string{
		"Fencing Ace", "Exultant Skymarcher", "Tormented Angel", "Charging Monstrosaur",
		"Wayward Giant", "Nimble Birdsticker", "Starlit Angel", "Shrike Force", "Faithbearer Paladin",
		"Knight of the Tusk",
	}
)

// cardsEnv is the mbc pool as a stageEnv. It skips the test with no corpus.
func cardsEnv(t *testing.T) *stageEnv {
	t.Helper()
	cardsettest.Require(t, "mbc")
	set, err := cardset.ByName("mbc")
	if err != nil {
		t.Fatal(err)
	}
	var list []cardset.Entry
	for _, n := range append(append([]string{"Pacifism", "Swiftfoot Boots", "Dragon Fodder", "Raise the Alarm"}, cardsLands...), cardsCreatures...) {
		list = append(list, cardset.Entry{Name: n, Count: 1})
	}
	tbl, _, _, err := set.Matchup(list, list)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]kop.CardID{}
	for i := range tbl.Cards {
		if tbl.Cards[i].Types&kop.TToken == 0 {
			id[tbl.Cards[i].Name] = kop.CardID(i)
		}
	}
	deck := func(r *rand.Rand) []kop.CardID {
		var d []kop.CardID
		add := func(n int, names ...string) {
			for i := 0; i < n; i++ {
				d = append(d, id[names[r.IntN(len(names))]])
			}
		}
		add(16, cardsLands...)
		add(3, "Pacifism")
		add(3, "Swiftfoot Boots")
		add(4, "Dragon Fodder")
		add(3, "Raise the Alarm")
		add(14, cardsCreatures...)
		return d
	}
	return &stageEnv{
		tbl: tbl, deck: deck, kernel: oracle.SetSide(set, tbl, oracle.Config{}),
		// a quarter of the games each wait for an attachment, a quarter for a token,
		// an eighth for a spell on the stack and an eighth for a stack entry with a target
		want: func(seed uint64, v *view.View, _ *decision.Decision) bool {
			switch seed % 8 {
			case 1, 5:
				return hasBF(v, func(c *view.CardView) bool { return c.AttachedTo != 0 })
			case 3, 7:
				return hasBF(v, func(c *view.CardView) bool { return c.IsToken })
			case 2:
				return len(v.Stack) != 0
			case 6:
				for i := range v.Stack {
					if len(v.Stack[i].Targets) != 0 {
						return true
					}
				}
				return false
			}
			return true
		},
	}
}

func hasBF(v *view.View, f func(*view.CardView) bool) bool {
	for _, p := range v.Players {
		for i := range p.Battlefield {
			if f(&p.Battlefield[i]) {
				return true
			}
		}
	}
	return false
}

// TestStageCardsMatchReplay is TestStageMatchesReplay on the corpus pool above:
// attached Auras and Equipment, tokens, and a token-making spell in hand,
// library and graveyard.
func TestStageCardsMatchReplay(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	env := cardsEnv(t)
	st := &stageStats{reasons: map[string]int{}, byStep: map[string]int{}, byKind: map[string]int{}}
	failed := 0
	for seed := uint64(1); seed <= stageGames; seed++ {
		if msg := stageGame(reg, env, seed, st, nil); msg != "" {
			t.Error(msg)
			if failed++; failed >= 5 {
				break
			}
		}
	}
	t.Logf("marks: %d staged positions held an attachment, %d a token (%d a ceased one), %d a spell on the stack", st.attached, st.tokens, st.ceased, st.spells)
	t.Logf("%d games: %d positions tried, %d staged (by step %v), %d continuation decisions; unstageable by reason %v", stageGames, st.tried, st.staged, st.byStep, st.contSteps, st.reasons)
	t.Logf("first detail of each reason: %v", st.firstDetail)
	t.Logf("%d staged positions held a targeted stack entry; %d positions refused %q", st.targeted, st.reasons[shadow.ReasonStack], shadow.ReasonStack)
	if st.targeted < 20 {
		t.Errorf("only %d staged positions held a targeted stack entry", st.targeted)
	}
	if st.reasons[shadow.ReasonStack] >= 200 {
		t.Errorf("%d positions refused %q", st.reasons[shadow.ReasonStack], shadow.ReasonStack)
	}
	if st.attached < 20 || st.tokens < 20 || st.spells < 20 {
		t.Errorf("the pool exercised too little: %d attachment and %d token positions staged", st.attached, st.tokens)
	}
	if st.ceased < 20 {
		t.Errorf("only %d staged positions held a ceased token, need at least 20", st.ceased)
	}
	if st.reasons[dirac.ReasonToken] >= 500 {
		t.Errorf("%d positions refused as a token, need fewer than 500", st.reasons[dirac.ReasonToken])
	}
	if st.staged < stageGames*8/10 { // a game that waits for a stack position may never see one
		t.Errorf("only %d of %d games staged a position", st.staged, stageGames)
	}
}

// cardsMutations are the staging features of the corpus pool, each dropped or
// bent: the verifier must fail on every one.
func cardsMutations(tbl *kop.Table) map[string]func(*dirac.World) {
	isCreature := func(w *dirac.World, i int) bool {
		o := &w.Objs[i]
		return o.Zone == kop.ZoneBattlefield && tbl.Cards[o.Card].Types&kop.TCreature != 0
	}
	return map[string]func(*dirac.World){
		"equipment attachment dropped": func(w *dirac.World) {
			for i := range w.Objs {
				if o := &w.Objs[i]; o.Attached && tbl.Cards[o.Card].Types&kop.TEquipment != 0 {
					o.Attached = false
				}
			}
		},
		"aura rehosted": func(w *dirac.World) {
			for i := range w.Objs {
				o := &w.Objs[i]
				if !o.Attached || tbl.Cards[o.Card].Types&kop.TAura == 0 {
					continue
				}
				for h := range w.Objs {
					if h != int(o.Host) && isCreature(w, h) {
						o.Host = dirac.Obj(h)
						break
					}
				}
			}
		},
		"tokens dropped": dropTokens(tbl),
		// each ceased token takes another token card of the table
		"ceased token's card swapped": func(w *dirac.World) {
			for i := range w.Objs {
				if o := &w.Objs[i]; o.Token && o.Zone == kop.ZoneNone {
					for c := range tbl.Cards {
						if d := &tbl.Cards[c]; d.Types == kop.TCreature|kop.TToken && len(d.Abilities) == 0 && d.Keywords&^kop.KwFlying == 0 && kop.CardID(c) != o.Card {
							o.Card = kop.CardID(c)
							break
						}
					}
				}
			}
		},
		// a ceased token comes back on its owner's battlefield, last in its list
		"ceased token revived": func(w *dirac.World) {
			for i := range w.Objs {
				if o := &w.Objs[i]; o.Token && o.Zone == kop.ZoneNone {
					n := uint8(0)
					for j := range w.Objs {
						if b := &w.Objs[j]; b.Zone == kop.ZoneBattlefield && b.Ctrl == o.Ctrl {
							n++
						}
					}
					o.Zone, o.Pos = kop.ZoneBattlefield, n
				}
			}
		},
		// the target moves to another creature on the same battlefield
		"stack target moved": func(w *dirac.World) {
			for i := range w.Stack {
				e := &w.Stack[i]
				for j := 0; j < int(e.NT); j++ {
					for c := range w.Objs {
						if t := &w.Objs[e.Tgt[j]]; dirac.Obj(c) != dirac.Obj(e.Tgt[j]) && isCreature(w, c) && w.Objs[c].Ctrl == t.Ctrl &&
							(e.Ability || dirac.Obj(c) != e.Obj) {
							e.Tgt[j] = dirac.TCode(c)
							break
						}
					}
				}
			}
		},
		"stack target dropped": func(w *dirac.World) {
			for i := range w.Stack {
				w.Stack[i].NT = 0
			}
		},
		// the spell goes to its owner's graveyard instead of staying on the stack
		"stack spell dropped": func(w *dirac.World) {
			var keep []dirac.WorldStack
			for _, e := range w.Stack {
				if e.Ability {
					keep = append(keep, e)
					continue
				}
				o := &w.Objs[e.Obj]
				o.Zone = kop.ZoneGraveyard
				for j := range w.Objs {
					if g := &w.Objs[j]; j != int(e.Obj) && g.Owner == o.Owner && g.Zone == kop.ZoneGraveyard && g.Pos >= o.Pos {
						o.Pos = g.Pos + 1
					}
				}
			}
			w.Stack = keep
		},
	}
}

// dropTokens removes the tokens from a world (they are its last objects) and
// closes the gaps in the battlefield order. An Aura on a token leaves the world
// as it was (the mutation is not applied there); an Equipment on one
// is left unattached.
func dropTokens(tbl *kop.Table) func(*dirac.World) {
	return func(w *dirac.World) {
		n := len(w.Objs)
		for n > 0 && w.Objs[n-1].Token {
			n--
		}
		for i := 0; i < n; i++ {
			if o := &w.Objs[i]; o.Attached && int(o.Host) >= n {
				if tbl.Cards[o.Card].Types&kop.TAura != 0 {
					return
				}
				o.Attached = false
			}
		}
		w.Objs = w.Objs[:n]
		// every battlefield object keeps its order; the places are renumbered densely
		for p := uint8(0); p < 2; p++ {
			var live []*dirac.WorldObj
			for i := range w.Objs {
				if o := &w.Objs[i]; o.Zone == kop.ZoneBattlefield && o.Owner == p {
					live = append(live, o)
				}
			}
			sort.SliceStable(live, func(a, b int) bool { return live[a].Pos < live[b].Pos })
			for pos, o := range live {
				o.Pos = uint8(pos)
			}
		}
	}
}
