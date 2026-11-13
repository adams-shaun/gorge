package oracle

import (
	"math/rand/v2"
	"sync"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// The Bargain lockstep (CR 702.166): constructed matchups over the generated
// bargain pool -- the nine WOE Bargain carriers kgen lowers, the two
// Condition$ Bargain cards, Ornithopter (a free artifact), Curious Pair's
// Treats to Share adventure (a Food token), Iridescent Tiger (the floating
// WUBRG the surface prices a coloured carrier from), Burning-Tree Emissary
// (the RG counterpart) and basics. The frozen FDN and BLB pair pools carry
// no Bargain card, so the expected answer at every decision is gorge's.
//
// The two decks are different: A carries the Count$Bargained carriers and a
// Condition$ pair, B the ETB riders, so the seats do not play the same 40
// and every family card is in a deck. The aggregate test counts the mechanic
// itself: the bargained cast chosen (ActCast with the cast-mode nibble full,
// the way gorge's "bargained" mode canonicalises), a plain cast of a Bargain
// card while the bargained option was offered, the election answered (gorge's
// "bargain" choose kind), a token sacrificed, the bargained ETB riders and
// both arms of a Count$Bargained read. Per-carrier coverage (all nine must
// set and clear sfKicked) is TestBargainCarrierFlags in package kernel,
// because the pool-pays surface cannot float UU/BB or a combat-timed W for
// every carrier here.
//
// The last three family carriers are in deck A: Brave the
// Wilds (a target count read from Count$Bargained), Realm-Scorcher Hellkite
// (DB$ Mana "Combo Any") and Torch the Tower (ReplaceDyingDefined Remembered).
// A bargained Brave fizzles in gorge (FlagBargained is set at payment, after
// the target ask) and the kernel follows (GORGE-FINDINGS.md). The surface cannot
// reach a bargained Hellkite: only a floating pool offers the bargained mode
// (gorge's payment actions plan plain casts) and the ETB mana never floats six
// with {R}{R}; its plain cast is exercised here and its colour ask by
// TestBargainHellkiteManaChoice in package kernel.
//
// Four workers run the 256 seeds; MaxTurns 60 and MaxDecisions 4000 keep
// every game from ending at the harness cap, so any Stall is a real failure.

const kbargainCastOther = castModeOther // the "bargained" cast mode's nibble (castModes is full)

var kbargainETB = map[string]bool{
	"Agatha's Champion":       true,
	"Diminisher Witch":        true,
	"High Fae Negotiator":     true,
	"Tenacious Tomeseeker":    true,
	"Realm-Scorcher Hellkite": true,
	"Troublemaker Ouphe":      true,
}

var kbargainCount = map[string]bool{
	"Brave the Wilds":      true,
	"Candy Grapple":        true,
	"Farsight Ritual":      true,
	"Kellan's Lightblades": true,
	"Stonesplitter Bolt":   true,
	"Torch the Tower":      true,
}

// kbargainCarrier is every card in the decks that has Bargain. The surface
// cannot offer every one of them (UU/BB need two Tigers; Kellan's needs a
// combat-timed W), but the policy still prefers them when they are offered.
var kbargainCarrier = map[string]bool{
	"Agatha's Champion":       true,
	"Archon's Glory":          true,
	"Brave the Wilds":         true,
	"Candy Grapple":           true,
	"Diminisher Witch":        true,
	"Farsight Ritual":         true,
	"High Fae Negotiator":     true,
	"Kellan's Lightblades":    true,
	"Realm-Scorcher Hellkite": true,
	"Rowan's Grim Search":     true,
	"Stonesplitter Bolt":      true,
	"Tenacious Tomeseeker":    true,
	"Torch the Tower":         true,
	"Troublemaker Ouphe":      true,
}

// kbargainPay is the ETB mana the decks run: Iridescent Tiger adds one of
// each colour when it is cast, Burning-Tree Emissary adds {R}{G}. They are
// the floating pool the pool-pays surface prices a coloured carrier from.
var kbargainPay = map[string]bool{
	"Iridescent Tiger":      true,
	"Burning-Tree Emissary": true,
}

// The nine family carriers, split over the two decks,
// so the seats play different 40s and each carrier is exercised.
var kbargainDecks = [2][]cardset.Entry{
	{ // A: the Count$Bargained carriers and the Condition$ pair
		{Name: "Stonesplitter Bolt", Count: 2}, {Name: "Troublemaker Ouphe", Count: 2},
		{Name: "Candy Grapple", Count: 1}, {Name: "Kellan's Lightblades", Count: 1},
		{Name: "Farsight Ritual", Count: 2},
		{Name: "Archon's Glory", Count: 1}, {Name: "Rowan's Grim Search", Count: 1},
		{Name: "Brave the Wilds", Count: 2}, {Name: "Realm-Scorcher Hellkite", Count: 2},
		{Name: "Torch the Tower", Count: 2},
		{Name: "Ornithopter", Count: 2}, {Name: "Curious Pair", Count: 3},
		{Name: "Bumbleflower's Sharepot", Count: 1},
		{Name: "Iridescent Tiger", Count: 4}, {Name: "Burning-Tree Emissary", Count: 4},
		{Name: "Mountain", Count: 4}, {Name: "Forest", Count: 4},
		{Name: "Swamp", Count: 1}, {Name: "Plains", Count: 1},
	},
	{ // B: the ETB riders and the remaining Count carriers
		{Name: "Agatha's Champion", Count: 2}, {Name: "Diminisher Witch", Count: 2},
		{Name: "High Fae Negotiator", Count: 2}, {Name: "Tenacious Tomeseeker", Count: 2},
		{Name: "Candy Grapple", Count: 2}, {Name: "Troublemaker Ouphe", Count: 2},
		{Name: "Ornithopter", Count: 2}, {Name: "Curious Pair", Count: 4},
		{Name: "Bumbleflower's Sharepot", Count: 2},
		{Name: "Iridescent Tiger", Count: 4}, {Name: "Burning-Tree Emissary", Count: 4},
		{Name: "Island", Count: 4}, {Name: "Swamp", Count: 4},
		{Name: "Plains", Count: 2}, {Name: "Mountain", Count: 1}, {Name: "Forest", Count: 1},
	},
}

// kbargainTally counts the mechanic through the decisions (the policy) and
// gorge's live stack (RunConfig.Check).
type kbargainTally struct {
	castsBargain int
	offers       int
	castNames    map[string]int
	plainNames   map[string]int
	castsPlain   int
	elections    int
	tokenSacs    int
	etbRiders    int // a bargained carrier's ETB trigger left the stack
	countYes     int // a Count$Bargained carrier resolved with FlagBargained
	countNo      int // the same with the plain cast

	seenSpell    map[state.ObjID]bool
	seenTrig     map[state.ObjID]bool
	pending      map[state.ObjID]bool
	pendingSpell map[state.ObjID]bool
	pendingFlag  map[state.ObjID]bool
}

func newKbargainTally() *kbargainTally {
	return &kbargainTally{castNames: map[string]int{}, plainNames: map[string]int{}}
}

func (c *kbargainTally) reset() {
	c.seenSpell = map[state.ObjID]bool{}
	if c.castNames == nil {
		c.castNames = map[string]int{}
	}
	if c.plainNames == nil {
		c.plainNames = map[string]int{}
	}
	c.seenTrig = map[state.ObjID]bool{}
	c.pending = map[state.ObjID]bool{}
	c.pendingSpell = map[state.ObjID]bool{}
	c.pendingFlag = map[state.ObjID]bool{}
}

// kbargainIsToken reports whether the canonical object a sacrifice election
// offers is a token.
func kbargainIsToken(g *GorgeSide, ref Ref) bool {
	if g == nil || ref >= RefStack || int(ref) >= len(g.id.obj) {
		return false
	}
	o := g.E.G.Obj(g.id.obj[ref])
	return o != nil && o.IsToken
}

// kbargainCastOption reports the canonical index of a bargained cast option
// in d: an ActCast with the full cast-mode nibble (gorge's "bargained" mode
// canonicalises to castModeOther) whose card has Bargain.
func kbargainCastOption(g *GorgeSide, d *CDecision) int {
	return kbargainCastWhere(g, d, func(string) bool { return true })
}

// kbargainCastWhere is kbargainCastOption over a card-name class.
func kbargainCastWhere(g *GorgeSide, d *CDecision, want func(string) bool) int {
	for i, a := range d.Options {
		if a.Kind() == ActCast && a.Mode()>>4 == 0 && a.Mode()&0xf == kbargainCastOther &&
			want(refCardName(g, a.Src())) {
			return i
		}
	}
	return -1
}

// kbargainArm answers the bargained offer at index pref with the bargained
// cast half the time and the plain cast of the same card the other half,
// counting the arm it chose.
func kbargainArm(p *PickCtx, pref int, c *kbargainTally, prng *rand.Rand) Canon {
	src := p.D.Options[pref].Src()
	name := refCardName(p.G, src)
	plain := -1
	for i, a := range p.D.Options {
		if a.Kind() == ActCast && a.Mode() == 0 && a.Src() == src {
			plain = i
			break
		}
	}
	if plain < 0 || prng.IntN(2) == 0 {
		c.castsBargain++
		c.castNames[name]++
		return p.D.Options[pref]
	}
	c.castsPlain++
	c.plainNames[name]++
	return p.D.Options[plain]
}

// kbargainPolicy takes the bargained cast half the time and, on the other
// half, the plain cast of the same card (so both arms are exercised), the
// bargain election (preferring a token half the time) and otherwise a
// cast/activation/land when one is offered. It casts the ETB mana first:
// the pool-pays surface offers a coloured carrier only once its mana is
// floating.
func kbargainPolicy(seed uint64, c *kbargainTally) Policy {
	rnd := RandomPolicy(seed)
	prng := rand.New(rand.NewPCG(seed, 0x6261726761696e)) // "bargain"
	var buf []Canon
	return PolicyFunc(func(p *PickCtx) Canon {
		if p.D.Kind == DChoose {
			if code, ok := chooseCode("bargain"); ok {
				var picks []Canon
				for _, a := range p.D.Options {
					if a.Kind() == ActChoose+CanonKind(code) {
						picks = append(picks, a)
					}
				}
				if len(picks) > 0 {
					if prng.IntN(3) != 0 {
						for _, a := range picks {
							if kbargainIsToken(p.G, a.Src()) {
								picks = []Canon{a}
								break
							}
						}
					}
					a := picks[prng.IntN(len(picks))]
					c.elections++
					if kbargainIsToken(p.G, a.Src()) {
						c.tokenSacs++
					}
					return a
				}
			}
		}
		if p.D.Kind == DAttackers {
			var atk []Canon
			for _, a := range p.D.Options {
				if a.Kind() == ActAttack {
					atk = append(atk, a)
				}
			}
			if len(atk) > 0 {
				return atk[prng.IntN(len(atk))]
			}
		}
		if p.D.Kind == DTarget {
			for _, a := range p.D.Options {
				if a.Kind() == ActTarget && a.Tgt() >= RefPlayer {
					return a
				}
			}
		}
		if p.D.Kind == DPriority {
			// The ETB mana first: a Tiger's five mana (or an Emissary's
			// two) is the floating pool the surface prices a coloured
			// carrier from.
			for _, a := range p.D.Options {
				if a.Kind() == ActCast && kbargainPay[refCardName(p.G, a.Src())] {
					return a
				}
			}
			for _, a := range p.D.Options {
				if a.Kind() == ActCastFace && a.Mode()&0xf == faceCodeAdventure &&
					refCardName(p.G, a.Src()) == "Curious Pair" {
					return a // Treats to Share: the Food token candidate
				}
			}
			for _, a := range p.D.Options {
				if a.Kind() == ActCast && refCardName(p.G, a.Src()) == "Ornithopter" {
					return a // a free artifact candidate
				}
			}
			// Every carrier offered, preferring the one with the fewest
			// counts so the reachable family cards spread over both arms.
			var offers []int
			for i, a := range p.D.Options {
				if a.Kind() == ActCast && a.Mode()>>4 == 0 && a.Mode()&0xf == kbargainCastOther &&
					kbargainCarrier[refCardName(p.G, a.Src())] {
					offers = append(offers, i)
				}
			}
			if len(offers) > 0 {
				c.offers++
				score := func(i int) (int, bool) {
					n := refCardName(p.G, p.D.Options[i].Src())
					return c.castNames[n] + c.plainNames[n], kbargainETB[n]
				}
				pref := offers[0]
				best, bestETB := score(pref)
				for _, i := range offers[1:] {
					got, etb := score(i)
					if got < best || (got == best && etb && !bestETB) {
						best, bestETB, pref = got, etb, i
					}
				}
				return kbargainArm(p, pref, c, prng)
			}
			for _, a := range p.D.Options {
				if a.Kind() == ActPlayLand {
					return a
				}
			}
			buf = buf[:0]
			for _, a := range p.D.Options {
				switch a.Kind() {
				case ActCast, ActCastCasualty, ActActivate, ActPlayLand:
					buf = append(buf, a)
				}
			}
			if len(buf) > 0 {
				d := *p.D
				d.Options = buf
				return rnd.Pick(&PickCtx{D: &d, G: p.G, Seq: p.Seq})
			}
		}
		return rnd.Pick(p)
	})
}

// check counts what only the live gorge stack shows: the two arms of a
// Count$Bargained read and the bargained ETB riders as they resolve.
func (c *kbargainTally) check(ref *GorgeSide, _ Side) *Divergence {
	g := ref.E.G
	onStack := map[state.ObjID]bool{}
	for _, id := range g.Stack {
		onStack[id] = true
		o := g.Obj(id)
		if o == nil {
			continue
		}
		if o.Ability == nil {
			if o.Card == nil || len(o.Card.Faces) == 0 || !kbargainCount[o.Card.Faces[0].Name] {
				continue
			}
			// The flag is paid at cast completion, after the build stages
			// that also pose decisions while the spell is on the stack;
			// remember the last value seen and count the arm when it leaves.
			if !c.seenSpell[id] {
				c.seenSpell[id] = true
				c.pendingSpell[id] = true
			}
			if o.CastFlags&state.FlagBargained != 0 {
				c.pendingFlag[id] = true
			}
			continue
		}
		if o.Source == 0 || c.seenTrig[id] {
			continue
		}
		src := g.Obj(o.Source)
		if src == nil || src.Card == nil || len(src.Card.Faces) == 0 ||
			!kbargainETB[src.Card.Faces[0].Name] || src.CastFlags&state.FlagBargained == 0 {
			continue
		}
		c.seenTrig[id] = true
		c.pending[id] = true
	}
	for id := range c.pending {
		if !onStack[id] {
			delete(c.pending, id)
			c.etbRiders++
		}
	}
	for id := range c.pendingSpell {
		if onStack[id] {
			continue
		}
		delete(c.pendingSpell, id)
		if c.pendingFlag[id] {
			c.countYes++
		} else {
			c.countNo++
		}
		delete(c.pendingFlag, id)
	}
	return nil
}

func (c *kbargainTally) add(o *kbargainTally) {
	c.castsBargain += o.castsBargain
	c.offers += o.offers
	c.castsPlain += o.castsPlain
	c.elections += o.elections
	c.tokenSacs += o.tokenSacs
	c.etbRiders += o.etbRiders
	c.countYes += o.countYes
	c.countNo += o.countNo
	if c.castNames == nil {
		c.castNames = map[string]int{}
	}
	if c.plainNames == nil {
		c.plainNames = map[string]int{}
	}
	for k, v := range o.castNames {
		c.castNames[k] += v
	}
	for k, v := range o.plainNames {
		c.plainNames[k] += v
	}
}

// TestBargainLockstep runs the 256-seed sweep in four workers (the test
// budget's four CPUs): every game is independent, and the tally merges.
func TestBargainLockstep(t *testing.T) {
	t.Parallel()

	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("bargain")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, d := range kbargainDecks {
		for _, e := range d {
			card, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("deck %d: %s not in the corpus", s, e.Name)
			}
			for i := 0; i < e.Count; i++ {
				decks[s] = append(decks[s], card)
			}
		}
		if len(decks[s]) != 40 {
			t.Fatalf("deck %d: %d cards", s, len(decks[s]))
		}
	}

	const workers = 4
	tallies := make([]*kbargainTally, workers)
	gameCounts := make([]int, workers)
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			tbl, _, _, err := set.Matchup(kbargainDecks[0], kbargainDecks[1])
			if err != nil {
				t.Errorf("worker %d: matchup: %v", w, err)
				return
			}
			c := newKbargainTally()
			for seed := uint64(w + 1); seed <= 256; seed += workers {
				cfg := Config{GorgeAutopaySurface: seed%2 == 0}
				m, err := NewMatchup(reg, [2]string{"kbargain-a", "kbargain-b"}, decks, 1)
				if err != nil {
					t.Errorf("seed %d: matchup: %v", seed, err)
					return
				}
				k, err := SetSide(set, tbl, cfg)(m, seed)
				if err != nil {
					t.Errorf("seed %d: side: %v", seed, err)
					return
				}
				c.reset()
				res := Lockstep(m, seed, k, kbargainPolicy(seed, c),
					RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 60, Check: c.check})
				CloseSide(k)
				gameCounts[w]++
				if res.Div != nil {
					lo := res.Div.At - 12
					if lo < 0 {
						lo = 0
					}
					t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:res.Div.At]))
					t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v",
						seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff, res.Div.Detail,
						res.Div.RefOptions, res.Div.KerOptions)
				}
				if res.Stall != "" {
					t.Errorf("seed %d: stalled: %s after %d turns", seed, res.Stall, res.Turns)
				}
			}
			tallies[w] = c
		}(w)
	}
	wg.Wait()
	c := newKbargainTally()
	games := 0
	for w, wc := range tallies {
		if wc != nil {
			c.add(wc)
		}
		games += gameCounts[w]
	}
	t.Logf("%d games, %+v castNames %v plainNames %v", games, *c, c.castNames, c.plainNames)
	for _, a := range []struct {
		name string
		got  int
		want int
	}{
		{"bargained casts", c.castsBargain, 30},
		{"plain casts with a candidate", c.castsPlain, 30},
		{"bargain elections", c.elections, 30},
		{"bargained ETB riders resolved", c.etbRiders, 10},
		{"Count$Bargained yes arm", c.countYes, 10},
		{"Count$Bargained no arm", c.countNo, 10},
		{"token sacrifices", c.tokenSacs, 5},
	} {
		if a.got < a.want {
			t.Errorf("%s: %d, want >= %d", a.name, a.got, a.want)
		}
	}
}
