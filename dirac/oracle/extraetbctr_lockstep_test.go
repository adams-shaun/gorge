package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/engine/events"
)

// Zameck Guildmage's turn-long rider (kop.TurnReplEtbCreatureCtr): "each
// creature you control enters with an additional +1/+1 counter on it" until
// end of turn, played decision by decision against gorge. The first seat
// activates Zameck, then makes tokens and casts creatures (some entering with
// their own counter, so the entry is staged and the rider folds into the
// entry's one put) under Hardened Scales and Prairie Dog's "that many plus
// one" replacements; the second seat only plays creatures and removal. The
// rider applies once per activation, one put per activation when the entry is
// unstaged and folded into the one put when it is staged.

func extraEtbCtrDecks() [2][]string {
	cat := func(ds ...[]string) []string {
		var o []string
		for _, d := range ds {
			o = append(o, d...)
		}
		return o
	}
	return [2][]string{
		cat(deckOf(4, "Zameck Guildmage"), deckOf(3, "Hardened Scales"), deckOf(2, "Prairie Dog"),
			deckOf(3, "Servant of the Scale"), deckOf(3, "Raise the Alarm"), deckOf(3, "Llanowar Elves"),
			deckOf(2, "Grizzly Bears"), deckOf(2, "Soul Summons"), deckOf(8, "Forest"), deckOf(5, "Plains"), deckOf(5, "Island")),
		cat(deckOf(4, "Doom Blade"), deckOf(8, "Grizzly Bears"), deckOf(14, "Forest"), deckOf(14, "Swamp")),
	}
}

// extraEtbCtrPolicy: the first seat casts Zameck when it can, then alternates
// (on the seed's own stream) between activating Zameck and casting a token
// maker or creature, so the pool reaches entries under several activations;
// the second seat plays at random through the shared cast-first policy.
func extraEtbCtrPolicy(m *Matchup, seed uint64) Policy {
	base := castFirstPolicy(seed)
	x := seed*2862933555777941757 + 3037000493
	coin := func() bool {
		x = x*6364136223846793005 + 1442695040888963407
		return x>>63 != 0
	}
	cast := func(p *PickCtx, want string) (Canon, bool) {
		for _, o := range p.D.Options {
			if o.Kind() == ActCast && m.InstanceCard(int(o.Src())) == want {
				return o, true
			}
		}
		return 0, false
	}
	activate := func(p *PickCtx) (Canon, bool) {
		for _, o := range p.D.Options {
			if o.Kind() == ActActivate && m.InstanceCard(int(o.Src())) == "Zameck Guildmage" {
				return o, true
			}
		}
		return 0, false
	}
	return PolicyFunc(func(p *PickCtx) Canon {
		if p.D.Kind == DPriority && p.D.Player == 0 {
			if o, ok := cast(p, "Zameck Guildmage"); ok {
				return o
			}
			if !coin() {
				if o, ok := activate(p); ok {
					return o
				}
			}
			for _, want := range []string{"Raise the Alarm", "Servant of the Scale", "Hardened Scales", "Soul Summons", "Grizzly Bears", "Prairie Dog", "Llanowar Elves"} {
				if o, ok := cast(p, want); ok {
					return o
				}
			}
			if o, ok := activate(p); ok {
				return o
			}
		}
		return base.Pick(p)
	})
}

// extraEtbCtrReached: Zameck was activated, a token its controller controls
// entered that turn after the activation, and a +1/+1 counter was placed (the
// entry path, not a later placement).
func extraEtbCtrReached(g *GorgeSide) bool {
	active, token, counter := false, false, false
	for _, e := range g.E.L.Events {
		switch {
		case e.Kind == events.TurnChange:
			active = false
		case e.Kind == events.AbilityPush && sideObjName(g, e.Obj) == "Zameck Guildmage":
			active = true
		case active && e.Kind == events.TokenCreate && e.Player == 0:
			token = true
		case active && e.Kind == events.CounterChange && e.Counter == "P1P1" && e.Amount > 0:
			counter = true
		}
	}
	return token && counter
}

// extraEtbCtrGames plays the pool on both option surfaces (autopay and full).
func extraEtbCtrGames(t *testing.T, seeds int, onGame func(seed uint64, surface string, res Result, reached bool, dEtb, dMerged int64)) {
	t.Helper()
	reg := synthReg(t)
	decks := extraEtbCtrDecks()
	set := printedDelveSet(t, decks)
	for _, surface := range []string{"autopay", "full"} {
		for seed := uint64(1); seed <= uint64(seeds); seed++ {
			c := effectLongCase{name: "extra_etb_ctr_" + surface, decks: decks, surface: surface, reached: extraEtbCtrReached}
			r := &Repro{Format: reproFormat, Decks: [2]string{"extra_etb_ctr_0", "extra_etb_ctr_1"},
				Cards: decks, Seed: seed, PassElision: true, Surface: surface, Kernel: "custom"}
			m, err := r.Matchup(reg)
			if err != nil {
				t.Fatal(err)
			}
			e0, m0 := dirac.TurnReplStats.Etb.Load(), dirac.TurnReplStats.EtbMerged.Load()
			res, reached := effectLongPlay(t, reg, c, r, set, extraEtbCtrPolicy(m, seed))
			onGame(seed, surface, res, reached, dirac.TurnReplStats.Etb.Load()-e0, dirac.TurnReplStats.EtbMerged.Load()-m0)
		}
	}
}

func TestExtraEtbCtrPoolLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	var etb, merged, games, reached int64
	fd0 := dirac.TurnReplStats.EtbFaceDown.Load()
	extraEtbCtrGames(t, 80, func(seed uint64, surface string, res Result, ok bool, dEtb, dMerged int64) {
		if res.Div != nil {
			t.Fatalf("%s seed %d: divergence at %d: %s %s %v", surface, seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
		}
		if res.StallUnmodelled {
			t.Fatalf("%s seed %d: the kernel stalled (%s): the game is not an agreed one", surface, seed, res.Stall)
		}
		games++
		etb += dEtb
		merged += dMerged
		if ok {
			reached++
		}
	})
	t.Logf("%d games agreed, %d creatures got the rider's counters, %d folded into an entry's one put", games, etb, merged)
	if games < 100 {
		t.Errorf("only %d games were modelled to their end", games)
	}
	if etb == 0 {
		t.Error("no creature entered with the rider's extra counter: the pool does not exercise it")
	}
	if merged == 0 {
		t.Error("no rider counter folded into a staged entry: the staged put is not exercised")
	}
	if etb <= merged {
		t.Error("every rider counter folded: the unstaged one-put-per-activation path is not exercised")
	}
	if dirac.TurnReplStats.EtbFaceDown.Load() == fd0 {
		t.Error("no face-down entrant got the rider's counters: the manifest path is not exercised")
	}
	if reached == 0 {
		t.Error("no game reached a token entering under the rider: the token path is not exercised")
	}
}

// TestExtraEtbCtrMutation: with the rider ignored (dirac.DebugEtbCreOff) the
// pool diverges, so the lockstep leans on the entry hook.
func TestExtraEtbCtrMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	reg := synthReg(t)
	decks := extraEtbCtrDecks()
	set := printedDelveSet(t, decks)
	dirac.DebugEtbCreOff = true
	defer func() { dirac.DebugEtbCreOff = false }()
	diverged := 0
	for seed := uint64(1); seed <= 40; seed++ {
		c := effectLongCase{name: "extra_etb_ctr_mutation", decks: decks, surface: "autopay", reached: func(*GorgeSide) bool { return false }}
		r := &Repro{Format: reproFormat, Decks: [2]string{"extra_etb_ctr_0", "extra_etb_ctr_1"},
			Cards: decks, Seed: seed, PassElision: true, Surface: "autopay", Kernel: "custom"}
		m, err := r.Matchup(reg)
		if err != nil {
			t.Fatal(err)
		}
		res, _ := effectLongPlay(t, reg, c, r, set, extraEtbCtrPolicy(m, seed))
		if res.Div != nil {
			diverged++
		}
	}
	if diverged == 0 {
		t.Fatal("no game diverged with the rider ignored: the pool does not exercise it")
	}
	t.Logf("%d of 40 games diverged with the rider ignored", diverged)
}
