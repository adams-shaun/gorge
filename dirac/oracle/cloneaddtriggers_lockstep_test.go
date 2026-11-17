package oracle

import (
	"maps"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// Phantasmal Image, Copycrook and Progenitor Mimic: "You may have this enter
// as a copy of any creature on the battlefield, except it has <a trigger>".
// gorge folds the AddTriggers$ trigger into the copy's copiable values
// (CR 707.9b; effects/clone.go foldCopyExcept); the kernel grants it to the
// copy as a latent trigger in the as-enters grant record (kgen
// lower_cloneentergrant.go cloneGrantTrigger). The expected answer at every
// decision is gorge's: the copy election, the granted trigger's queueing and
// its resolution (Image sacrificed when targeted, Copycrook connives on
// attacking, Mimic's upkeep token copy).
//
// A copy of a copy has the trigger twice (gorge appends the line to the copy
// face each time, CR 707.3 and 707.9b): the kernel keeps an AddTriggers$
// grant per application (copy_grants.go copyGrantOnce), and a token copy
// carries its template's grants (interp_gen.go copyGrantsAdd), so a Mimic
// copying a Mimic token shows both triggers in the CR 603.3b order ask.
//
// Seat 1's Shocks target the copies; seat 0 plays the three carriers over
// Grizzly Bears to copy.
var kcloneAddTriggersDecks = [2][]cardset.Entry{
	{
		{Name: "Phantasmal Image", Count: 6}, {Name: "Copycrook", Count: 5}, {Name: "Progenitor Mimic", Count: 5},
		{Name: "Grizzly Bears", Count: 6}, {Name: "Island", Count: 14}, {Name: "Forest", Count: 8},
	},
	{
		{Name: "Shock", Count: 8}, {Name: "Grizzly Bears", Count: 8}, {Name: "Mountain", Count: 12}, {Name: "Forest", Count: 8},
	},
}

// kcloneAddTriggersCarrier is the carrier name of o's printed card ("" when
// it is not one).
func kcloneAddTriggersCarrier(o *state.Object) string {
	if o == nil || o.Card == nil || len(o.Card.Faces) == 0 {
		return ""
	}
	switch n := o.Card.Faces[0].Name; n {
	case "Phantasmal Image", "Copycrook", "Progenitor Mimic":
		return n
	}
	return ""
}

func TestCloneAddTriggersLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	var names [2][]string
	var gd [2][]*cards.Card
	for s, d := range kcloneAddTriggersDecks {
		for _, e := range d {
			card, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s not in the corpus", e.Name)
			}
			names[s] = append(names[s], e.Name)
			for i := 0; i < e.Count; i++ {
				gd[s] = append(gd[s], card)
			}
		}
	}
	set := cardset.Raw(printedDelveSet(t, names))
	tbl, _, _, err := set.Matchup(kcloneAddTriggersDecks[0], kcloneAddTriggersDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	const n = 60
	copies, fires := map[string]int{}, map[string]int{}
	twice, div := 0, 0
	for seed := uint64(1); seed <= n; seed++ {
		m, err := NewMatchup(reg, [2]string{"mp-a", "mp-b"}, gd, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, Config{})(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		seen := 0
		check := func(ref *GorgeSide, _ Side) *Divergence {
			evs := ref.E.L.Events
			if len(evs) < seen {
				seen = 0
			}
			for ; seen < len(evs); seen++ {
				ev := &evs[seen]
				o := ref.E.G.Obj(ev.Obj)
				c := kcloneAddTriggersCarrier(o)
				if c == "" || o.Face() == nil || o.Face().Name == c {
					continue
				}
				switch ev.Kind {
				case events.MoveZone:
					if ev.To == state.ZBattlefield {
						copies[c]++
						// a copy of a copy has the same granted trigger
						// twice (two instances of one AddTriggers$ line);
						// two different triggers are not that
					out:
						for a := 1; a < len(o.Face().Triggers); a++ {
							for b := 0; b < a; b++ {
								if maps.Equal(o.Face().Triggers[a].Params, o.Face().Triggers[b].Params) {
									twice++
									break out
								}
							}
						}
					}
				case events.TriggerPush:
					fires[c]++
				}
			}
			return nil
		}
		res := Lockstep(m, seed, k, kcloneAddTriggersPolicy(seed), RunConfig{Config: Config{}, MaxDecisions: 2500, MaxTurns: 20, Check: check})
		CloseSide(k)
		if d := res.Div; d != nil {
			div++
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
		}
	}
	t.Logf("%d games: %d divergences, copies %v, granted triggers pushed %v, %d copies with two triggers", n, div, copies, fires, twice)
	for _, c := range []string{"Phantasmal Image", "Copycrook", "Progenitor Mimic"} {
		if copies[c] < 3 {
			t.Errorf("%s entered as a copy %d times: want at least 3 (the grant must be exercised)", c, copies[c])
		}
	}
	if fires["Copycrook"] < 3 || fires["Progenitor Mimic"] < 3 {
		t.Errorf("granted triggers pushed %v: want Copycrook and Progenitor Mimic at least 3 each", fires)
	}
	if twice < 1 {
		t.Errorf("no copy of a copy entered with the trigger twice: the copy-of-copy grant is not exercised")
	}
}

// kcloneAddTriggersPolicy casts and plays lands when it can (so the carriers
// enter as copies, get Shocked and attack), else random.
func kcloneAddTriggersPolicy(seed uint64) Policy {
	rnd := RandomPolicy(seed)
	var buf []Canon
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.D.Kind != DPriority {
			return rnd.Pick(c)
		}
		buf = buf[:0]
		for _, o := range c.D.Options {
			switch o.Kind() {
			case ActCast, ActPlayLand:
				buf = append(buf, o)
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
