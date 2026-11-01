package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The faceDown filter word: Cryptid Inspector's and
// Threats Around Every Corner's enters triggers (ValidCard$
// Permanent.faceDown+YouCtrl) and Tunnel Tipster's end-step count
// (ThisTurnEntered_Battlefield_Creature.faceDown+YouCtrl). gorge's word is a
// face-down BATTLEFIELD permanent, read where the object lives now: a
// creature that entered face down and was turned face up before the end step
// does not count. The face-down makers are manifest dread (Manifest Dread,
// Defiant Survivor, Innocuous Rat, Break Down the Door, Threats itself) and
// the morph / disguise cards of the facedown pool, paid from Moxen and
// Priests.

var kfacedownpredMorphDeck = []cardset.Entry{
	{Name: "Mox Emerald", Count: 3}, {Name: "Mox Jet", Count: 3}, {Name: "Mox Ruby", Count: 3}, {Name: "Mox Pearl", Count: 3},
	{Name: "Priest of Gix", Count: 4}, {Name: "Priest of Urabrask", Count: 4},
	{Name: "Aven Farseer", Count: 3}, {Name: "Kolaghan Stormsinger", Count: 2}, {Name: "Bolrac-Clan Basher", Count: 2},
	{Name: "Basilica Stalker", Count: 2}, {Name: "Zoetic Cavern", Count: 3}, {Name: "Concealed Weapon", Count: 4},
	{Name: "Tunnel Tipster", Count: 4},
}

var kfacedownpredDecks = [][2][]cardset.Entry{{
	// A: green, the manifest dread makers and the three carriers
	{
		{Name: "Cryptid Inspector", Count: 4}, {Name: "Threats Around Every Corner", Count: 4}, {Name: "Tunnel Tipster", Count: 4},
		{Name: "Manifest Dread", Count: 4}, {Name: "Defiant Survivor", Count: 3}, {Name: "Innocuous Rat", Count: 2},
		{Name: "Break Down the Door", Count: 2}, {Name: "Zoetic Cavern", Count: 3}, {Name: "Forest", Count: 14},
	},
	// B: the morph / disguise cards on Moxen and Priests, with carriers
	{
		{Name: "Mox Emerald", Count: 3}, {Name: "Mox Jet", Count: 3}, {Name: "Mox Ruby", Count: 2}, {Name: "Mox Pearl", Count: 2},
		{Name: "Priest of Gix", Count: 4}, {Name: "Priest of Urabrask", Count: 4},
		{Name: "Aven Farseer", Count: 3}, {Name: "Kolaghan Stormsinger", Count: 2}, {Name: "Basilica Stalker", Count: 2},
		{Name: "Zoetic Cavern", Count: 3}, {Name: "Concealed Weapon", Count: 2},
		{Name: "Cryptid Inspector", Count: 4}, {Name: "Tunnel Tipster", Count: 4}, {Name: "Forest", Count: 2},
	},
}, {
	// B against A, the seats swapped
	{
		{Name: "Mox Emerald", Count: 3}, {Name: "Mox Jet", Count: 3}, {Name: "Mox Ruby", Count: 2}, {Name: "Mox Pearl", Count: 2},
		{Name: "Priest of Gix", Count: 4}, {Name: "Priest of Urabrask", Count: 4},
		{Name: "Aven Farseer", Count: 3}, {Name: "Kolaghan Stormsinger", Count: 2}, {Name: "Basilica Stalker", Count: 2},
		{Name: "Zoetic Cavern", Count: 3}, {Name: "Concealed Weapon", Count: 2},
		{Name: "Cryptid Inspector", Count: 4}, {Name: "Tunnel Tipster", Count: 4}, {Name: "Forest", Count: 2},
	},
	{
		{Name: "Cryptid Inspector", Count: 4}, {Name: "Threats Around Every Corner", Count: 4}, {Name: "Tunnel Tipster", Count: 4},
		{Name: "Manifest Dread", Count: 4}, {Name: "Defiant Survivor", Count: 3}, {Name: "Innocuous Rat", Count: 2},
		{Name: "Break Down the Door", Count: 2}, {Name: "Zoetic Cavern", Count: 3}, {Name: "Forest", Count: 14},
	},
}, {
	// C against C: the facedown mana base and morph cards with Tunnel
	// Tipster, so a face-down creature enters and is turned up in one turn
	// under a Tipster
	kfacedownpredMorphDeck,
	kfacedownpredMorphDeck,
}}

var kfacedownpredTurnUp = func() uint8 { c, _ := specialCode("turn_face_up"); return c }()

// kfacedownpredStats is what the tally counts across the games.
type kfacedownpredStats struct {
	turnUps    int // turn-face-up special actions taken
	fdEntries  int // face-down entries under a controller who has Cryptid Inspector or Threats out
	pushes     int // Cryptid Inspector / Threats trigger pushes (enters and turn-ups)
	tipTrue    int // Tunnel Tipster end steps whose trigger put a counter
	tipFalse   int // Tunnel Tipster end steps with no counter
	tipFalseUp int // ... where a face-down creature had entered under that player this turn but was no longer face down
	tipBad     int // end steps whose outcome disagreed with the face-down creature on the battlefield
}

type kfacedownpredTip struct {
	id  state.ObjID
	exp bool
	got bool
}

type kfacedownpredTally struct {
	evAt  int
	turn  int
	act   state.PlayerID
	inEnd bool
	tips  []kfacedownpredTip
	fdEnt map[state.PlayerID]int // controller -> the turn a face-down permanent last entered under them
	upEnt bool                   // a tracked entry exists for the active player this turn
}

func (p *kfacedownpredTally) finish(st *kfacedownpredStats) {
	for _, tp := range p.tips {
		if tp.got != tp.exp {
			st.tipBad++
		}
		switch {
		case tp.got:
			st.tipTrue++
		default:
			st.tipFalse++
			if p.fdEnt[p.act] == p.turn {
				st.tipFalseUp++
			}
		}
	}
	p.tips, p.inEnd = p.tips[:0], false
}

// kfacedownpredCtrl lists the battlefield objects pl controls (the zone is
// kept by owner).
func kfacedownpredCtrl(g *state.Game, pl state.PlayerID) []state.ObjID {
	var out []state.ObjID
	for own := state.PlayerID(0); own < 2; own++ {
		for _, id := range g.Zone(state.ZBattlefield, own) {
			if o := g.Obj(id); o != nil && o.Controller == pl {
				out = append(out, id)
			}
		}
	}
	return out
}

// kfacedownpredCarrier reports whether controller pl has Cryptid Inspector or
// Threats on the battlefield (other than o).
func kfacedownpredCarrier(g *state.Game, pl state.PlayerID, o state.ObjID) bool {
	for _, id := range kfacedownpredCtrl(g, pl) {
		if ob := g.Obj(id); id == o || ob == nil || ob.FaceDown {
			continue // a face-down carrier has no abilities
		}
		switch chName(g, id) {
		case "Cryptid Inspector", "Threats Around Every Corner":
			return true
		}
	}
	return false
}

func (p *kfacedownpredTally) scan(ref *GorgeSide, st *kfacedownpredStats) {
	g := ref.E.G
	evs := ref.E.L.Events
	if len(evs) < p.evAt {
		p.evAt = 0
	}
	for j := p.evAt; j < len(evs); j++ {
		e := &evs[j]
		switch e.Kind {
		case events.TurnChange:
			p.finish(st)
			p.turn++
			p.act = e.Player
		case events.StepChange:
			if p.inEnd {
				p.finish(st)
			}
			if e.Step != state.StepEnd {
				break
			}
			p.inEnd = true
			mine := kfacedownpredCtrl(g, p.act)
			for _, id := range mine {
				if o := g.Obj(id); o == nil || o.FaceDown || chName(g, id) != "Tunnel Tipster" {
					continue // a face-down Tunnel Tipster has no abilities
				}
				// the face-down creature now on the battlefield that entered this turn
				exp := false
				for _, oid := range mine {
					if o := g.Obj(oid); o != nil && o.FaceDown && o.EnteredThisTurn && o.EffectiveIsCreature() {
						exp = true
					}
				}
				p.tips = append(p.tips, kfacedownpredTip{id: id, exp: exp})
			}
		case events.CounterChange:
			if p.inEnd && e.Amount > 0 {
				for i := range p.tips {
					if p.tips[i].id == e.Obj {
						p.tips[i].got = true
					}
				}
			}
		case events.TriggerPush:
			switch chName(g, e.Obj) {
			case "Cryptid Inspector", "Threats Around Every Corner":
				st.pushes++
			}
		case events.MoveZone:
			if e.To != state.ZBattlefield {
				break
			}
			if o := g.Obj(e.Obj); o != nil && o.FaceDown {
				if p.fdEnt == nil {
					p.fdEnt = map[state.PlayerID]int{}
				}
				if o.EffectiveIsCreature() {
					p.fdEnt[o.Controller] = p.turn
				}
				if kfacedownpredCarrier(g, o.Controller, e.Obj) {
					st.fdEntries++
				}
			}
		}
	}
	p.evAt = len(evs)
}

// kfacedownpredPolicy prefers casting, playing and activating (so the
// carriers and the face-down makers are reached), else random, and counts the
// carrier events the gorge side logs.
func kfacedownpredPolicy(seed uint64, st *kfacedownpredStats) Policy {
	rnd := RandomPolicy(seed)
	var buf []Canon
	var tl kfacedownpredTally
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.G != nil {
			tl.scan(c.G, st)
		}
		buf = buf[:0]
		// a turn-up first (it fires Cryptid Inspector and leaves a Tunnel
		// Tipster's face-down entry no longer face down), then a face-down cast
		for _, tier := range [...]func(a Canon) bool{
			func(a Canon) bool { return a.Kind() == ActSpecial && a.Mode() == kfacedownpredTurnUp },
			func(a Canon) bool { return a.Kind() == ActCastFace },
			func(a Canon) bool {
				switch a.Kind() {
				case ActCast, ActActivate, ActSpecial, ActPlayLand:
					return true
				}
				return false
			},
		} {
			for _, a := range c.D.Options {
				if tier(a) {
					buf = append(buf, a)
				}
			}
			if len(buf) > 0 {
				if tier(Canon(buf[0])) && buf[0].Kind() == ActSpecial && buf[0].Mode() == kfacedownpredTurnUp {
					st.turnUps++
				}
				break
			}
		}
		if len(buf) == 0 {
			return rnd.Pick(c)
		}
		d := *c.D
		d.Options = buf
		return rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
	})
}

// runFacedownpred plays the constructed matchups and returns the games, the
// divergences, the stalls and the carrier counts. mutate makes the kernel's
// PredFaceDown answer face up (the mutation must diverge).
func runFacedownpred(t *testing.T, seeds int, mutate bool) (games, divs, stalls int, st kfacedownpredStats) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("facedownpred")
	if err != nil {
		t.Fatal(err)
	}
	old := dirac.DebugFaceDownPredUp
	dirac.DebugFaceDownPredUp = mutate
	defer func() { dirac.DebugFaceDownPredUp = old }()
	for mi, md := range kfacedownpredDecks {
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
			m, err := NewMatchup(reg, [2]string{"kfacedownpred-a", "kfacedownpred-b"}, decks, 1)
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
			res := LockstepLed(m, seed, g, k, kfacedownpredPolicy(seed, &st), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 25})
			CloseSide(k)
			games++
			if len(res.Stall) >= len(KernelStallPrefix) && res.Stall[:len(KernelStallPrefix)] == KernelStallPrefix {
				stalls++
				if !mutate {
					t.Errorf("matchup %d seed %d: kernel stall %q", mi, seed, res.Stall)
				}
			}
			if d := res.Div; d != nil {
				divs++
				if !mutate {
					t.Errorf("matchup %d seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", mi, seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
				}
			}
		}
	}
	return games, divs, stalls, st
}

func TestFacedownpredLockstep(t *testing.T) {
	if isolate(t) {
		return
	}

	games, divs, stalls, st := runFacedownpred(t, 256, false)
	t.Logf("%d games, %d divergences, %d stalls", games, divs, stalls)
	t.Logf("%+v", st)
	if divs != 0 || stalls != 0 {
		t.Fatalf("%d divergences, %d stalls", divs, stalls)
	}
	if st.fdEntries < 15 {
		t.Errorf("only %d face-down entries under a Cryptid Inspector / Threats controller (want >= 15)", st.fdEntries)
	}
	if st.tipTrue < 5 || st.tipFalse < 5 {
		t.Errorf("Tunnel Tipster end steps: %d true, %d false (want >= 5 each)", st.tipTrue, st.tipFalse)
	}
	// gorge reads the spec on the object where it lives now: a creature that
	// entered face down and was turned face up before the end step does not
	// count
	if st.tipBad != 0 {
		t.Errorf("%d Tunnel Tipster end steps disagreed with the face-down creature on the battlefield", st.tipBad)
	}
	if st.tipFalseUp < 1 {
		t.Errorf("no Tunnel Tipster end step with a face-down entry that was no longer face down (want >= 1)")
	}
}

// TestFacedownpredMutation makes the kernel's PredFaceDown answer face up:
// gorge's word is a face-down battlefield permanent, so the games must
// diverge. A green mutation means the lockstep never reaches a decision the
// word shapes.
func TestFacedownpredMutation(t *testing.T) {
	games, divs, _, _ := runFacedownpred(t, 128, true)
	if divs == 0 {
		t.Fatalf("mutation answered face up in %d games with no divergence", games)
	}
	t.Logf("mutation diverged in %d of %d games", divs, games)
}
