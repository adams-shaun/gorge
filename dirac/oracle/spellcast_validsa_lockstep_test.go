package oracle

import (
	"math/rand/v2"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kgen"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// The SpellCast ValidSA$ words lockstep: FwModal, FwSingleTarget and
// FwTeamwork read the cast spell's own entry (gorge ValidSAMatches, a
// single-field clause being a plain spec filter over the spell).
//
//   - Modal Probe (an inline card: Riku of Many Paths, the one corpus carrier
//     of Spell.Modal, is refused for its CharmNum$ X) fires on Izzet Charm, not
//     on Shock, Opt, Blink of an Eye or a creature.
//   - Single Probe (an inline card: Spinerock Tyrant, the corpus carrier of
//     Instant.singleTarget,Sorcery.singleTarget, is refused for its
//     CopySpellAbility RememberNewCard$) and Spell Probe (Captured by the
//     Consulate, the corpus carrier of Spell.singleTarget, is refused for its
//     OrderDuplicates$; the probe reads any opposing spell too) fire on a one-target cast (Shock,
//     Blink of an Eye, a Forked Bolt or Izzet Charm pick with one) and not on
//     Opt (none) or a two-target Forked Bolt.
//   - Virtual Assistant (Spell.Teamwork) fires on a teamworked Team Tactics,
//     not on a plain one nor on a kicked Blink of an Eye (the kernel marks a
//     kicker payment with the same stack-entry bit it marks teamwork with).
//
// B holds no carrier, so A's triggers read B's casts too (Captured by the
// Consulate's ValidActivatingPlayer$ Opponent, Riku's You).
const spellcastModalProbe = `Name:Modal Probe
ManaCost:U
Types:Creature Wizard
PT:1/1
T:Mode$ SpellCast | ValidActivatingPlayer$ You | ValidSA$ Spell.Modal | Execute$ ProbeDraw | TriggerZones$ Battlefield | TriggerDescription$ Whenever you cast a modal spell, draw a card.
SVar:ProbeDraw:DB$ Draw | NumCards$ 1
Oracle:Whenever you cast a modal spell, draw a card.`

const spellcastSingleProbe = `Name:Single Probe
ManaCost:U
Types:Creature Wizard
PT:1/1
T:Mode$ SpellCast | ValidActivatingPlayer$ You | ValidSA$ Instant.singleTarget,Sorcery.singleTarget | Execute$ ProbeDraw | TriggerZones$ Battlefield | TriggerDescription$ Whenever you cast an instant or sorcery spell with a single target, draw a card.
SVar:ProbeDraw:DB$ Draw | NumCards$ 1
Oracle:Whenever you cast an instant or sorcery spell with a single target, draw a card.`

const spellcastSpellProbe = `Name:Spell Probe
ManaCost:U
Types:Creature Wizard
PT:1/1
T:Mode$ SpellCast | ValidActivatingPlayer$ Opponent | ValidSA$ Spell.singleTarget | Execute$ ProbeDraw | TriggerZones$ Battlefield | TriggerDescription$ Whenever an opponent casts a spell, if it has a single target, draw a card.
SVar:ProbeDraw:DB$ Draw | NumCards$ 1
Oracle:Whenever an opponent casts a spell, if it has a single target, draw a card.`

var spellcastValidSADecks = [2][]cardset.Entry{
	{ // A: the carriers
		{Name: "Modal Probe", Count: 3}, {Name: "Virtual Assistant", Count: 3},
		{Name: "Single Probe", Count: 2}, {Name: "Spell Probe", Count: 2},
		{Name: "Izzet Charm", Count: 3}, {Name: "Team Tactics", Count: 3}, {Name: "Blink of an Eye", Count: 3},
		{Name: "Shock", Count: 2}, {Name: "Forked Bolt", Count: 2}, {Name: "Opt", Count: 2},
		{Name: "Hill Giant", Count: 3}, {Name: "Burning-Tree Emissary", Count: 3},
		{Name: "Mountain", Count: 3}, {Name: "Island", Count: 3}, {Name: "Forest", Count: 1}, {Name: "Plains", Count: 2},
	},
	{ // B: spells only
		{Name: "Izzet Charm", Count: 4}, {Name: "Team Tactics", Count: 3}, {Name: "Blink of an Eye", Count: 3},
		{Name: "Shock", Count: 4}, {Name: "Forked Bolt", Count: 3}, {Name: "Opt", Count: 3}, {Name: "Lightning Bolt", Count: 2},
		{Name: "Hill Giant", Count: 4}, {Name: "Grizzly Bears", Count: 4},
		{Name: "Mountain", Count: 4}, {Name: "Island", Count: 3}, {Name: "Plains", Count: 3},
	},
}

// spellcastValidSATally counts, over gorge's finished logs, the trigger
// fires of each carrier (TriggerPush by its source) and the casts of each
// card (PutOnStack): the preconditions that both arms of every word were met.
type spellcastValidSATally struct {
	fires, casts map[string]int
}

func (c *spellcastValidSATally) finish(g *GorgeSide) {
	name := func(id state.ObjID) string {
		if o := g.E.G.Obj(id); o != nil && o.Face() != nil {
			return o.Face().Name
		}
		return ""
	}
	for i := range g.E.L.Events {
		switch e := &g.E.L.Events[i]; e.Kind {
		case events.TriggerPush:
			c.fires[name(e.Obj)]++
		case events.PutOnStack:
			c.casts[name(e.Obj)]++
		case events.CastInfo:
			if f := events.FlagsFrom(e.Counter); f&state.FlagTeamworkPaid != 0 {
				c.casts["teamworked "+name(e.Obj)]++
			} else if f&state.FlagKicked != 0 {
				c.casts["kicked "+name(e.Obj)]++
			}
		}
	}
}

// spellcastValidSAPolicy is the random policy, except that Team Tactics is cast
// teamworked half the time it is offered that way (the plain cast of the same
// card otherwise), and the tap election takes one creature, then Done.
func spellcastValidSAPolicy(seed uint64) Policy {
	rnd := RandomPolicy(seed)
	prng := rand.New(rand.NewPCG(seed, 0x76616c69647361)) // "validsa"
	inElect := false
	return PolicyFunc(func(p *PickCtx) Canon {
		if p.D.Kind == DChoose {
			if code, ok := chooseCode("teamwork"); ok {
				var picks []Canon
				done := -1
				for i, a := range p.D.Options {
					switch {
					case a.Kind() == ActChoose+CanonKind(code):
						picks = append(picks, a)
					case a.Kind() == ActDone:
						done = i
					}
				}
				if len(picks) > 0 {
					if inElect && done >= 0 {
						inElect = false
						return p.D.Options[done]
					}
					inElect = true
					return picks[prng.IntN(len(picks))]
				}
			}
		}
		inElect = false
		if p.D.Kind == DPriority && prng.IntN(2) == 0 {
			// the ETB mana first: a teamworked cast is offered only once the
			// floating pool prices it (teamwork_test.go's kteamworkPay)
			for _, a := range p.D.Options {
				if a.Kind() == ActCast && refCardName(p.G, a.Src()) == "Burning-Tree Emissary" {
					return a
				}
			}
			for i, a := range p.D.Options {
				if a.Kind() == ActCast && a.Mode()>>4 == 0 && a.Mode()&0xf == castModeOther && refCardName(p.G, a.Src()) == "Team Tactics" {
					return p.D.Options[i]
				}
			}
		}
		return rnd.Pick(p)
	})
}

func TestSpellcastValidSALockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	corpus := testutil.CorpusRegistry(t) // first: without .cards the test skips before the corpus-built set loads
	reg := cards.NewRegistry()           // the corpus registry is imaged, so the probe goes into a fresh one
	for k, v := range corpus.Tokens {
		reg.Tokens[k] = v
	}
	for _, src := range []string{spellcastModalProbe, spellcastSingleProbe, spellcastSpellProbe} {
		probe, diags := cards.ParseBytes("probe.txt", []byte(src))
		if len(diags) != 0 {
			t.Fatalf("probe: %v", diags)
		}
		probe.Link()
		for _, fc := range probe.Faces {
			fc.ApplyIntrinsics()
		}
		reg.Add(probe)
	}
	var decks [2][]*cards.Card
	var pool []*cards.Card
	seen := map[string]bool{}
	for s, d := range spellcastValidSADecks {
		for _, e := range d {
			cd, ok := reg.Lookup(e.Name)
			if !ok {
				if cd, ok = corpus.Lookup(e.Name); !ok {
					t.Fatalf("%s not in the corpus", e.Name)
				}
				reg.Add(cd)
			}
			if !seen[e.Name] {
				seen[e.Name] = true
				pool = append(pool, cd)
			}
			for i := 0; i < e.Count; i++ {
				decks[s] = append(decks[s], cd)
			}
		}
		if len(decks[s]) != 40 {
			t.Fatalf("deck %d: %d cards", s, len(decks[s]))
		}
	}
	pd, err := kgen.TableFromCards(reg, pool, "spellcastvalidsa-oracle")
	if err != nil {
		t.Fatalf("table: %v", err)
	}
	set := cardset.Raw(cardset.NewDataSet(pd))
	tbl, _, _, err := set.Matchup(spellcastValidSADecks[0], spellcastValidSADecks[1])
	if err != nil {
		t.Fatal(err)
	}
	tally := &spellcastValidSATally{fires: map[string]int{}, casts: map[string]int{}}
	for seed := uint64(1); seed <= 60; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"validsa-a", "validsa-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		res := Lockstep(m, seed, k, spellcastValidSAPolicy(seed), RunConfig{Config: cfg, MaxDecisions: 2500, MaxTurns: 24, Finish: tally.finish})
		CloseSide(k)
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			t.Errorf("seed %d: kernel stall %s", seed, res.Stall)
		}
		if d := res.Div; d != nil {
			lo := max(d.At-12, 0)
			t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:d.At]))
			t.Fatalf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
		}
	}
	t.Logf("fires %v casts %v", tally.fires, tally.casts)
	// each word read both ways: the trigger fired, and spells it must not
	// fire on were cast beside it
	for _, carrier := range []string{"Modal Probe", "Virtual Assistant", "Single Probe", "Spell Probe"} {
		if tally.fires[carrier] == 0 {
			t.Errorf("%s never fired", carrier)
		}
	}
	for _, spell := range []string{"Izzet Charm", "Team Tactics", "Blink of an Eye", "Shock", "Forked Bolt", "Opt", "teamworked Team Tactics", "kicked Blink of an Eye"} {
		if tally.casts[spell] == 0 {
			t.Errorf("%s never cast", spell)
		}
	}
}
