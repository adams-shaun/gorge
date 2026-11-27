package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
)

// The hosted-Commander Play-shape lockstep (kgen lower_zone.go play's
// SorcerySpeed/ImprintPlayed/ValidSA$ creature-type arms, the kernel's
// freeImprint bit in dirac/cast_playimprint.go): Conduit of Worlds' targeted
// paid Play behind its SorcerySpeed$ activation, Rashmi and Ragavan's
// ImprintPlayed$ free cast of the exiled top card, Scarlet Witch's
// Spell.Hero,Spell.nonCreature filter over the cards exiled with her, and
// West Coast Expansion's Spell.Hero+YouOwn hand play. Each deck is built to
// reach one shape; the tallies fail the test if a run never posed it, so the
// lockstep cannot pass vacuously.
var khostedPlayDecks = [4][2][]cardset.Entry{
	// Conduit of Worlds: Entomb/Buried Alive put a nonland permanent in the
	// graveyard, then the artifact's {T} ability plays it paid
	// (SorcerySpeed$ True). The opponent is all lands so the game reaches it.
	{
		{
			{Name: "Conduit of Worlds", Count: 4}, {Name: "Entomb", Count: 4}, {Name: "Buried Alive", Count: 4},
			{Name: "Sakura-Tribe Elder", Count: 4}, {Name: "Birds of Paradise", Count: 4}, {Name: "Elvish Mystic", Count: 4},
			{Name: "Forest", Count: 8}, {Name: "Swamp", Count: 8},
		},
		{{Name: "Island", Count: 40}},
	},
	// Rashmi and Ragavan: the first spell each turn exiles the top card of
	// the opponent's library and the Treasure plus zero-cost artifacts let
	// the free ImprintPlayed$ cast be offered
	{
		{
			{Name: "Rashmi and Ragavan", Count: 4}, {Name: "Lotus Petal", Count: 4}, {Name: "Mishra's Bauble", Count: 4},
			{Name: "Urza's Bauble", Count: 4}, {Name: "Lightning Bolt", Count: 4}, {Name: "Gitaxian Probe", Count: 4},
			{Name: "Dark Ritual", Count: 4}, {Name: "Island", Count: 4}, {Name: "Mountain", Count: 4}, {Name: "Forest", Count: 4},
		},
		// nonland cards in the exiled top: the free cast needs a spell whose
		// mana value is below Rashmi's artifact count
		{{Name: "Island", Count: 20}, {Name: "Gitaxian Probe", Count: 20}},
	},
	// Scarlet Witch, Chaotic Avenger: the flying Hero deals combat damage,
	// exiles the top two and may cast a Hero or noncreature spell from among
	// them (Spell.Hero,Spell.nonCreature)
	{
		{
			{Name: "Scarlet Witch, Chaotic Avenger", Count: 4}, {Name: "The Wasp, Winsome Avenger", Count: 4},
			{Name: "Speed, Young Avenger", Count: 4}, {Name: "Spider-Punk", Count: 4},
			{Name: "Lightning Bolt", Count: 4}, {Name: "Gitaxian Probe", Count: 4},
			{Name: "Island", Count: 8}, {Name: "Mountain", Count: 4}, {Name: "Plains", Count: 4},
		},
		{{Name: "Island", Count: 40}},
	},
	// West Coast Expansion: a large X draws into a Hero in hand, then the
	// Spell.Hero+YouOwn free cast is offered
	{
		{
			{Name: "West Coast Expansion", Count: 4}, {Name: "The Wasp, Winsome Avenger", Count: 4},
			{Name: "Speed, Young Avenger", Count: 4}, {Name: "Spider-Punk", Count: 4},
			{Name: "Gitaxian Probe", Count: 4},
			{Name: "Island", Count: 10}, {Name: "Mountain", Count: 6}, {Name: "Plains", Count: 4},
		},
		{{Name: "Island", Count: 40}},
	},
}

// hpTally counts the Play asks this slice's four shapes posed and the cards
// their answers began.
type hpTally struct {
	asks  [4]int
	begun [4]int
}

// hpShape names the shape a pending Play ask belongs to, or -1.
func hpShape(sa *cards.SA) int {
	if sa == nil {
		return -1
	}
	switch {
	case sa.ParamStr(cards.PKSorcerySpeed) == "True":
		return 0
	case sa.ParamStr(cards.PKImprintPlayed) == "True":
		return 1
	case strings.Contains(sa.ParamStr(cards.PKValidSA), "Spell.Hero,Spell.nonCreature"):
		return 2
	case strings.Contains(sa.ParamStr(cards.PKValidSA), "Spell.Hero+YouOwn"):
		return 3
	}
	return -1
}

// hostedPlayPolicy biases the random policy toward progressing (casts,
// activations, land plays, attacks, modes), takes a card over Done at this
// slice's Play asks, and announces the largest X so West Coast Expansion's
// "X is 5 or more" gate can open.
func hostedPlayPolicy(t *hpTally) func(uint64) Policy {
	return func(seed uint64) Policy {
		rnd := RandomPolicy(seed)
		return PolicyFunc(func(c *PickCtx) Canon {
			if c.G == nil {
				return rnd.Pick(c)
			}
			pd := c.G.Pending()
			if pd != nil && pd.Kind == decision.KChoose && len(pd.Options) > 0 && pd.Options[0].Kind == "x" {
				var best Canon
				bestN, found := -1, false
				for _, a := range c.D.Options {
					if o, ok := c.G.GorgeOption(a); ok && o.Kind == "x" && o.Amount > bestN {
						best, bestN, found = a, o.Amount, true
					}
				}
				if found {
					return best
				}
			}
			if pd != nil && pd.ResumeKind == "play" {
				if shape := hpShape(pd.ResumeSA); shape >= 0 {
					t.asks[shape]++
					for _, a := range c.D.Options {
						if o, ok := c.G.GorgeOption(a); ok && o.Kind == "mode" && o.Obj != 0 {
							t.begun[shape]++
							return a
						}
					}
				}
			}
			buf := make([]Canon, 0, 8)
			for _, a := range c.D.Options {
				switch a.Kind() {
				case ActCast, ActCastFace, ActActivate, ActPlayLand, ActAttack, ActMode:
					buf = append(buf, a)
				}
			}
			if len(buf) > 0 && len(buf) < len(c.D.Options) {
				d := *c.D
				d.Options = buf
				return rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
			}
			return rnd.Pick(c)
		})
	}
}

func TestHostedCmdrPlayShapesLockstep(t *testing.T) {
	tally := new(hpTally)
	policy := hostedPlayPolicy(tally)
	for i, decks := range khostedPlayDecks {
		if n := lockstepRunPolicy(t, "hosted-cmdr", decks, 64, true, nil, nil, policy); n != 0 {
			t.Fatalf("deck %d: %d diverged", i, n)
		}
	}
	names := [...]string{"Conduit of Worlds", "Rashmi and Ragavan", "Scarlet Witch, Chaotic Avenger", "West Coast Expansion"}
	for i, name := range names {
		if tally.asks[i] == 0 {
			t.Errorf("%s: no Play ask posed: the lockstep proves nothing about the shape", name)
		}
		if tally.begun[i] == 0 {
			t.Errorf("%s: no Play answer began the card: the lockstep proves nothing about the resolution", name)
		}
	}
	t.Logf("play asks %v, begun %v", tally.asks, tally.begun)
}
