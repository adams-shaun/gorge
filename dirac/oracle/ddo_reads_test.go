package oracle

import (
	"os"
	"strconv"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// Expedited Inheritance (MKM): "Whenever a creature is dealt damage, its
// controller may exile that many cards from the top of their library. They
// may play those cards until the end of their next turn." Its
// DamageDoneOnce body reads TriggeredTargetController (the damaged
// creature's controller) for the Dig, and the Effect that grants the play
// is that player's (EffectOwner$ RememberedController), not the trigger's
// controller's. Shock deals exactly two, so the damaged creature's
// controller exiles two and may play one on their turn; both decks carry the
// Inheritance and creatures, so either seat can be the damaged controller.
// The expected answer at every decision is gorge's.
var kddoreadsDecks = [2][]cardset.Entry{
	{
		{Name: "Expedited Inheritance", Count: 4}, {Name: "Shock", Count: 8},
		{Name: "Grizzly Bears", Count: 6}, {Name: "Hill Giant", Count: 2},
		{Name: "Mountain", Count: 12}, {Name: "Forest", Count: 8},
	},
	{
		{Name: "Expedited Inheritance", Count: 4}, {Name: "Shock", Count: 8},
		{Name: "Grizzly Bears", Count: 2}, {Name: "Hill Giant", Count: 6},
		{Name: "Mountain", Count: 10}, {Name: "Forest", Count: 10},
	},
}

// kddoreadsPolicy never attacks or blocks, so Shock is the only damage: one
// damaged referent per batch, the single-trigger shape the card is here to
// prove. It says yes to the Inheritance's take-all election and otherwise
// prefers plays and casts, falling back to random.
func kddoreadsPolicy(seed uint64) Policy {
	rnd := RandomPolicy(seed)
	var buf []Canon
	return PolicyFunc(func(c *PickCtx) Canon {
		buf = buf[:0]
		for _, a := range c.D.Options {
			switch a.Kind() {
			case ActAttack, ActBlock:
				continue
			case ActYes:
				return a
			}
			buf = append(buf, a)
		}
		if len(buf) == 0 {
			return rnd.Pick(c)
		}
		d := *c.D
		d.Options = buf
		return rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
	})
}

func TestDamageDoneOnceReadsLockstep(t *testing.T) {
	t.Parallel()
	seeds := uint64(400)
	if v := os.Getenv("KDDOREADS_SEEDS"); v != "" {
		if n, err := strconv.ParseUint(v, 10, 64); err == nil {
			seeds = n
		}
	}
	lockstepRunPolicy(t, "ddo_reads", kddoreadsDecks, seeds, true, nil, nil, kddoreadsPolicy)
}
