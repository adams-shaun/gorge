package oracle

import (
	"strings"
	"sync"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// The lockstep: constructed matchups over the generated freeplay
// pool -- the four api:Play free-cast spellings (P1 remembered exile, P2
// targeted graveyard, P3 hand, P4 untargeted graveyard) and the view-only
// top-of-library MayLookAt statics (L1). Run as its own shard
// (-run '^TestFreePlaySpellingsLockstep$'), not inside the remainder Lockstep$ shard.
var m11Decks = [][2][]cardset.Entry{{
	{
		{Name: "Kotis, the Fangkeeper", Count: 3}, {Name: "The Key to the Vault", Count: 3},
		{Name: "Buster Sword", Count: 3}, {Name: "Daredevil, Man Without Fear", Count: 3},
		{Name: "Iron Lad, Diverging Destiny", Count: 3},
		{Name: "Shock", Count: 3}, {Name: "Opt", Count: 3}, {Name: "Divination", Count: 1},
		{Name: "Grizzly Bears", Count: 2},
		{Name: "Island", Count: 5}, {Name: "Swamp", Count: 3}, {Name: "Forest", Count: 2},
		{Name: "Mountain", Count: 3}, {Name: "Plains", Count: 3},
	},
	{
		{Name: "Daring Waverider", Count: 3}, {Name: "Seifer Almasy", Count: 3},
		{Name: "The Dawning Archaic", Count: 1}, {Name: "Diviner of Mist", Count: 3},
		{Name: "Baral's Expertise", Count: 2},
		{Name: "Refute", Count: 2}, {Name: "Shock", Count: 3}, {Name: "Opt", Count: 3},
		{Name: "Divination", Count: 2}, {Name: "Grizzly Bears", Count: 3},
		{Name: "Island", Count: 8}, {Name: "Mountain", Count: 3}, {Name: "Swamp", Count: 2},
		{Name: "Forest", Count: 2},
	},
}, {
	{
		{Name: "Daring Waverider", Count: 3}, {Name: "Seifer Almasy", Count: 3},
		{Name: "The Dawning Archaic", Count: 1}, {Name: "Diviner of Mist", Count: 3},
		{Name: "Baral's Expertise", Count: 2},
		{Name: "Refute", Count: 2}, {Name: "Shock", Count: 3}, {Name: "Opt", Count: 3},
		{Name: "Divination", Count: 2}, {Name: "Grizzly Bears", Count: 3},
		{Name: "Island", Count: 8}, {Name: "Mountain", Count: 3}, {Name: "Swamp", Count: 2},
		{Name: "Forest", Count: 2},
	},
	{
		{Name: "Kotis, the Fangkeeper", Count: 3}, {Name: "The Key to the Vault", Count: 3},
		{Name: "Buster Sword", Count: 3}, {Name: "Daredevil, Man Without Fear", Count: 3},
		{Name: "Iron Lad, Diverging Destiny", Count: 3},
		{Name: "Shock", Count: 3}, {Name: "Opt", Count: 3}, {Name: "Divination", Count: 1},
		{Name: "Grizzly Bears", Count: 2},
		{Name: "Island", Count: 5}, {Name: "Swamp", Count: 3}, {Name: "Forest", Count: 2},
		{Name: "Mountain", Count: 3}, {Name: "Plains", Count: 3},
	},
}}

// m11Tally counts the four Play spellings from gorge's own answers and
// events, never from a kernel-side counter: the policy sees the pending
// "play" ask (ResumeKind) with its zone, ValidTgts, Amount and
// ReplaceGraveyard parameters, and the chosen option's card zone.
type m11Tally struct {
	p1, p2, p3, p4 int // begun free casts: exile, targeted grave, hand, untargeted grave
	one            int // Amount$ 1 asks that actually capped >1 candidates
	gy             int // begun graveyard plays whose SA has ReplaceGraveyard$ Exile
	skip           int // graveyard asks with an unofferable Refute
	plays          map[string]int
	l1             int // games with an L1 carrier on the battlefield
}

func newM11Tally() *m11Tally { return &m11Tally{plays: map[string]int{}} }

// add sums o's counts into t; the per-game l1_game flag is taken from o only
// when last (o holds the run's last game).
func (t *m11Tally) add(o *m11Tally, last bool) {
	t.p1, t.p2, t.p3, t.p4 = t.p1+o.p1, t.p2+o.p2, t.p3+o.p3, t.p4+o.p4
	t.one, t.gy, t.skip, t.l1 = t.one+o.one, t.gy+o.gy, t.skip+o.skip, t.l1+o.l1
	for k, v := range o.plays {
		if k == "l1_game" {
			if last {
				t.plays[k] = v
			}
			continue
		}
		t.plays[k] += v
	}
}

// m11Policy casts or activates whenever it can and takes a free-cast pick
// (ActMode) over stopping (ActDone), so the games reach the carriers. It
// counts the carrier plays and the games a view-only L1 carrier was on the
// battlefield (a game-level flag: the static changes nothing, so the count
// proves presence, not a mechanic).
func m11Policy(seed uint64, t *m11Tally) Policy {
	rnd := RandomPolicy(seed)
	var buf []Canon
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.G != nil {
			onField := false
			for _, ob := range c.G.E.G.Objs {
				if ob.Face() == nil || ob.Zone != state.ZBattlefield {
					continue
				}
				switch ob.Face().Name {
				case "Daredevil, Man Without Fear", "Iron Lad, Diverging Destiny":
					onField = true
				}
			}
			if onField && t.plays["l1_game"] == 0 {
				t.plays["l1_game"] = 1
				t.l1++
			}
		}
		buf = buf[:0]
		hasDone, hasMode := false, false
		for _, a := range c.D.Options {
			switch a.Kind() {
			case ActDone:
				hasDone = true
			case ActMode:
				hasMode = true
			case ActCast, ActActivate:
				buf = append(buf, a)
			}
		}
		var a Canon
		switch {
		case hasDone && hasMode:
			for _, o := range c.D.Options {
				if o.Kind() == ActMode {
					a = o
					break
				}
			}
		case len(buf) > 0:
			d := *c.D
			d.Options = buf
			a = rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
		default:
			a = rnd.Pick(c)
		}
		if c.G != nil {
			m11Count(c, a, t)
		}
		return a
	})
}

// m11Count classifies one answer against the pending gorge decision.
func m11Count(c *PickCtx, a Canon, t *m11Tally) {
	pd := c.G.Pending()
	if pd == nil || pd.ResumeKind != "play" || pd.ResumeSA == nil {
		return
	}
	sa := pd.ResumeSA
	opt, ok := c.G.GorgeOption(a)
	if !ok || opt.Kind != "mode" || opt.Obj == 0 {
		return
	}
	ob := c.G.E.G.Obj(opt.Obj)
	if ob == nil || ob.Face() == nil {
		return
	}
	t.plays[ob.Face().Name]++
	switch ob.Zone {
	case state.ZExile:
		t.p1++
	case state.ZHand:
		t.p3++
	case state.ZGraveyard:
		if strings.TrimSpace(sa.ParamStr(cards.PKValidTgts)) != "" {
			t.p2++
		} else {
			t.p4++
		}
		if strings.EqualFold(strings.TrimSpace(sa.ParamStr(cards.PKReplaceGraveyard)), "Exile") {
			t.gy++
		}
	}
	if pd.Max == 1 && len(pd.Options) > 1 {
		t.one++
	}
	zone := strings.TrimSpace(sa.ParamStr(cards.PKValidZone))
	if zone == "" {
		zone = strings.TrimSpace(sa.ParamStr(cards.PKTgtZone))
	}
	if zone != "Graveyard" {
		return
	}
	offered := make(map[state.ObjID]bool, len(pd.Options))
	for _, o := range pd.Options {
		offered[o.Obj] = true
	}
	for _, id := range c.G.E.G.Zone(state.ZGraveyard, state.PlayerID(pd.Player)) {
		if offered[id] {
			continue
		}
		if o := c.G.E.G.Obj(id); o != nil && o.Face() != nil && o.Face().Name == "Refute" {
			t.skip++
		}
	}
}

func TestFreePlaySpellingsLockstep(t *testing.T) {
	t.Parallel()

	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("freeplay")
	if err != nil {
		t.Fatal(err)
	}
	tally := newM11Tally()
	games := 0
	matchupChunks(t, len(m11Decks), 1, 512, 128, func(t *testing.T, c seedChunk, mu *sync.Mutex) {
		mi, md := c.mi, m11Decks[c.mi]
		var decks [2][]*cards.Card
		for s, d := range md {
			for _, e := range d {
				card, ok := reg.Lookup(e.Name)
				if !ok {
					t.Fatalf("%s not in the corpus", e.Name)
				}
				for i := 0; i < e.Count; i++ {
					decks[s] = append(decks[s], card)
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
		part := newM11Tally()
		for seed := c.lo; seed <= c.hi; seed++ {
			m, err := NewMatchup(reg, [2]string{"m11-a", "m11-b"}, decks, 1)
			if err != nil {
				t.Fatal(err)
			}
			cfg := Config{GorgeAutopaySurface: seed%2 == 0}
			k, err := SetSide(set, tbl, cfg)(m, seed)
			if err != nil {
				t.Fatal(err)
			}
			part.plays["l1_game"] = 0
			res := Lockstep(m, seed, k, m11Policy(seed, part), RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40})
			CloseSide(k)
			if strings.HasPrefix(res.Stall, KernelStallPrefix) {
				t.Errorf("matchup %d seed %d: kernel stalled: %s", mi, seed, res.Stall)
			}
			if d := res.Div; d != nil {
				lo := max(d.At-12, 0)
				t.Logf("matchup %d seed %d last actions %v", mi, seed, actionStrings(res.Actions[lo:d.At]))
				t.Errorf("matchup %d seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", mi, seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
		}
		mu.Lock()
		defer mu.Unlock()
		games += int(c.hi - c.lo + 1)
		// l1_game is the game's flag: the run's is its last game's
		tally.add(part, c.mi == len(m11Decks)-1 && c.hi == 512)
	})
	t.Logf("%d games, carrier plays %v, L1 battlefield games %d, free [%d %d %d %d] one %d gy %d skip %d",
		games, tally.plays, tally.l1, tally.p1, tally.p2, tally.p3, tally.p4,
		tally.one, tally.gy, tally.skip)
	if tally.p1 < 50 {
		t.Errorf("P1 free casts %d, want at least 50", tally.p1)
	}
	if tally.p2 < 50 {
		t.Errorf("P2 free casts %d, want at least 50", tally.p2)
	}
	if tally.p3 < 30 {
		t.Errorf("P3 free casts %d, want at least 30", tally.p3)
	}
	if tally.p4 < 10 {
		t.Errorf("P4 free casts %d, want at least 10", tally.p4)
	}
	if tally.one < 20 {
		t.Errorf("Amount$ 1 caps %d, want at least 20", tally.one)
	}
	if tally.gy < 20 {
		t.Errorf("ReplaceGraveyard plays %d, want at least 20", tally.gy)
	}
	if tally.skip < 5 {
		t.Errorf("target-unfulfillable skips %d, want at least 5", tally.skip)
	}
	if tally.l1 < 50 {
		t.Errorf("L1 carriers on the battlefield in %d games, want at least 50", tally.l1)
	}
}

// TestFreePlaySpellingsMutation flips the Amount$ 1 reader (offer every candidate)
// and then the ReplaceGraveyard reader (normal graveyard) and requires the
// lockstep to diverge.
func TestFreePlaySpellingsMutation(t *testing.T) {
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("freeplay")
	if err != nil {
		t.Fatal(err)
	}
	md := m11Decks[0]
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
	}
	tbl, _, _, err := set.Matchup(md[0], md[1])
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		on   func(bool)
	}{
		{"AmountOne", func(b bool) { dirac.DebugM11IgnoreOne = b }},
		{"ReplaceGY", func(b bool) { dirac.DebugM11NoReplaceGY = b }},
	} {
		tc.on(true)
		diverged := false
		for seed := uint64(1); seed <= 256 && !diverged; seed++ {
			m, err := NewMatchup(reg, [2]string{"m11-a", "m11-b"}, decks, 1)
			if err != nil {
				t.Fatal(err)
			}
			cfg := Config{GorgeAutopaySurface: seed%2 == 0}
			k, err := SetSide(set, tbl, cfg)(m, seed)
			if err != nil {
				t.Fatal(err)
			}
			res := Lockstep(m, seed, k, m11Policy(seed, newM11Tally()), RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40})
			CloseSide(k)
			if res.Div != nil {
				diverged = true
			}
		}
		tc.on(false)
		if !diverged {
			t.Errorf("%s mutation: no divergence in 256 games", tc.name)
		}
	}
}
