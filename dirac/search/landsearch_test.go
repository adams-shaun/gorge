package search

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/dirac/search/hbot"
)

// landMatchup is an FDN two-colour deck with Evolving Wilds, a gain land
// and basics (seat 0) against a red-green one.
func landMatchup(t *testing.T) (*kop.Table, []dirac.CardID, []dirac.CardID) {
	t.Helper()
	a := []cardset.Entry{
		{Name: "Evolving Wilds", Count: 6},
		{Name: "Thornwood Falls", Count: 6},
		{Name: "Forest", Count: 6},
		{Name: "Island", Count: 6},
		{Name: "Mountain", Count: 4},
		{Name: "Llanowar Elves", Count: 8},
		{Name: "Think Twice", Count: 8},
		{Name: "Giant Growth", Count: 8},
		{Name: "Burst Lightning", Count: 8},
	}
	b := []cardset.Entry{
		{Name: "Mountain", Count: 18},
		{Name: "Forest", Count: 18},
		{Name: "Llanowar Elves", Count: 12},
		{Name: "Burst Lightning", Count: 12},
	}
	set := cardsettest.Set(t, "fdn")
	tbl, da, db, err := set.Matchup(a, b)
	if err != nil {
		t.Fatalf("matchup: %v", err)
	}
	return tbl, da, db
}

// landWalkGames plays random games of landMatchup and calls visit at every
// seat-0 decision (a copy of the state), until visit has returned true n
// times or the seeds run out.
func landWalkGames(t *testing.T, n int, visit func(s *dirac.State, d *dirac.Decision) bool) int {
	t.Helper()
	tbl, da, db := landMatchup(t)
	hits := 0
	for seed := uint64(1); seed <= 60 && hits < n; seed++ {
		s := dirac.NewConfig(tbl, da, db, seed, dirac.Config{})
		pol := dirac.NewRandomPolicy(seed ^ 0x6c616e64)
		var d dirac.Decision
		for i := 0; i < 3000 && hits < n; i++ {
			s.StepInto(&d)
			if d.Kind == dirac.DkNone {
				break
			}
			if d.Player == 0 {
				var c dirac.State
				c.CopyFrom(s)
				if visit(&c, &d) {
					hits++
				}
			}
			s.Apply(pol.Pick(&d))
		}
	}
	return hits
}

func landSearcher(tbl *kop.Table, mod func(o *Options)) *Searcher {
	o := DefaultOptions()
	o.Sims, o.Limit = 24, 16
	mod(&o)
	return NewSearcher(o, Uniform{}, hbot.New(hbot.NewFacts(tbl)))
}

func hasLandPlay(d *dirac.Decision) bool {
	for i := 0; i < int(d.N); i++ {
		if d.Options[i].Kind() == dirac.AkPlayLand {
			return true
		}
	}
	return false
}

// TestLandSearchSearchesTheLandDrop: off, a land-play bot answer is played
// unsearched; on, the decision is searched with that land as candidate 0
// and no other land (without :landpick); with LandPick every distinct land
// in hand is a candidate, once.
func TestLandSearchSearchesTheLandDrop(t *testing.T) {
	tbl, _, _ := landMatchup(t)
	off := landSearcher(tbl, func(o *Options) {})
	on := landSearcher(tbl, func(o *Options) { o.LandSearch = true })
	pick := landSearcher(tbl, func(o *Options) { o.LandSearch, o.LandPick = true, true })
	multi := 0
	hits := landWalkGames(t, 40, func(s *dirac.State, d *dirac.Decision) bool {
		if d.Kind != dirac.DkPriority || !hasLandPlay(d) {
			return false
		}
		r0 := *off.Search(s, 5)
		if r0.Stats.Searched != 0 || r0.Acts[0].Kind() != dirac.AkPlayLand {
			t.Fatalf("off: searched %d, answer kind %d (want the land unsearched)", r0.Stats.Searched, r0.Acts[0].Kind())
		}
		r1 := *on.Search(s, 5)
		if r1.Stats.Searched != 1 || r1.Stats.LandSearched != 1 || on.RootCand(0)[0] != uint32(r0.Acts[0]) {
			t.Fatalf("on: searched %d landsearched %d cand0 %x (want the bot's land %x)", r1.Stats.Searched, r1.Stats.LandSearched, on.RootCand(0)[0], uint32(r0.Acts[0]))
		}
		for i := 1; i < r1.N; i++ {
			if dirac.Action(on.RootCand(i)[0]).Kind() == dirac.AkPlayLand {
				t.Fatalf("on without landpick: candidate %d is a land play", i)
			}
		}
		if (r1.Stats.LandDeferred == 1) != (dirac.Action(on.RootCand(r1.Choice)[0]).Kind() != dirac.AkPlayLand) {
			t.Fatalf("LandDeferred %d for choice %d", r1.Stats.LandDeferred, r1.Choice)
		}
		r2 := *pick.Search(s, 5)
		if r2.Stats.Searched != 1 || pick.RootCand(0)[0] != uint32(r0.Acts[0]) {
			t.Fatalf("landpick: searched %d cand0 %x", r2.Stats.Searched, pick.RootCand(0)[0])
		}
		var want, got [64]bool
		nw := 0
		for i := 0; i < int(d.N); i++ {
			if a := d.Options[i]; a.Kind() == dirac.AkPlayLand {
				c := s.ObjCard(dirac.Obj(a.Source()))
				if !want[c%64] {
					nw++
				}
				want[c%64] = true
			}
		}
		ng := 0
		for i := 0; i < r2.N; i++ {
			a := dirac.Action(pick.RootCand(i)[0])
			if a.Kind() != dirac.AkPlayLand {
				continue
			}
			c := s.ObjCard(dirac.Obj(a.Source()))
			if got[c%64] {
				t.Fatalf("landpick: card %d offered twice", c)
			}
			got[c%64] = true
			ng++
		}
		if ng != nw || got != want {
			t.Fatalf("landpick: %d land candidates, want %d distinct lands", ng, nw)
		}
		if nw > 1 {
			multi++
		}
		return true
	})
	if hits < 20 || multi == 0 {
		t.Fatalf("%d land decisions, %d with two distinct lands: the walk found too few", hits, multi)
	}
}

// TestLandSearchUnmirroredOpponent: an unmirrored opponent's land play is
// never a point (its uniform prior would skip drops its bot never skips).
func TestLandSearchUnmirroredOpponent(t *testing.T) {
	tbl, _, _ := landMatchup(t)
	sr := landSearcher(tbl, func(o *Options) { o.LandSearch, o.OpponentMirror = true, false })
	sr.seat = 0
	var d dirac.Decision
	d.Kind, d.Player, d.N = dirac.DkPriority, 1, 2
	d.Options[0] = dirac.MakeAction(dirac.AkPlayLand, 3, 0, 0)
	d.Options[1] = dirac.MakeAction(dirac.AkPass, 0, 0, 0)
	if sr.landOn(&d) {
		t.Fatal("an unmirrored opponent's land play is a point")
	}
	sr.Opts.OpponentMirror = true
	if !sr.landOn(&d) {
		t.Fatal("a mirrored opponent's land play is not a point")
	}
}

// TestLandDraw: under Sample a land root's draw stands only for a land or
// a line with strictly more visits than the land.
func TestLandDraw(t *testing.T) {
	var sr Searcher
	pt := &sr.rootPt
	pt.addSeq([]uint32{uint32(dirac.MakeAction(dirac.AkPlayLand, 3, 0, 0))})
	pt.addSeq([]uint32{uint32(dirac.MakeAction(dirac.AkPass, 0, 0, 0))})
	pt.addSeq([]uint32{uint32(dirac.MakeAction(dirac.AkPlayLand, 4, 0, 0))})
	played := [MaxCands]int32{5, 5, 1}
	if c := sr.landDraw(1, &played); c != 0 {
		t.Fatalf("a tie with the land: %d", c)
	}
	played[1] = 6
	if c := sr.landDraw(1, &played); c != 1 {
		t.Fatalf("more visits than the land: %d", c)
	}
	if c := sr.landDraw(2, &played); c != 2 {
		t.Fatalf("another land: %d", c)
	}
}

// TestDedupSearchOneCandidatePerCard: a library search with more matching
// cards than Limit is unsearched off and searched on, one candidate per
// distinct card (the bot's first), keyed by card.
func TestDedupSearchOneCandidatePerCard(t *testing.T) {
	tbl, _, _ := landMatchup(t)
	off := landSearcher(tbl, func(o *Options) { o.Limit = 6 })
	on := landSearcher(tbl, func(o *Options) { o.Limit, o.DedupSearch = 6, true })
	big := 0
	hits := landWalkGames(t, 12, func(s *dirac.State, d *dirac.Decision) bool {
		if !libSearch(d) || d.N < 2 {
			return false
		}
		distinct := 0
		for i := 0; i < int(d.N); i++ {
			if !dupBefore(s, d, 0, i) {
				distinct++
			}
		}
		r0 := *off.Search(s, 9)
		if int(d.N) > 6 {
			big++
			if r0.Stats.Searched != 0 {
				t.Fatalf("off: a %d-option search was searched", d.N)
			}
		}
		r1 := *on.Search(s, 9)
		if distinct < 2 {
			return false
		}
		if r1.Stats.Searched != 1 || r1.Stats.DedupSearched != 1 || r1.N != distinct {
			t.Fatalf("on: searched %d dedup %d, %d candidates for %d distinct cards (%d options)", r1.Stats.Searched, r1.Stats.DedupSearched, r1.N, distinct, d.N)
		}
		if b := d.Options[on.Bot.Pick(s, d)]; on.RootCand(0)[0] != uint32(b) {
			t.Fatalf("on: candidate 0 %x is not the bot's answer %x", on.RootCand(0)[0], uint32(b))
		}
		for i := 0; i < r1.N; i++ {
			for j := 0; j < i; j++ {
				if optCard(s, dirac.Action(on.RootCand(i)[0])) == optCard(s, dirac.Action(on.RootCand(j)[0])) {
					t.Fatalf("on: candidates %d and %d are the same card", j, i)
				}
			}
		}
		return true
	})
	if hits == 0 || big == 0 {
		t.Fatalf("%d library searches, %d over the limit: the walk found too few", hits, big)
	}
}

// TestLandAbNowCracksAtOnce: on, a priority decision offering Evolving
// Wilds' crack plays it unsearched; off, it is the bot's or the search's.
func TestLandAbNowCracksAtOnce(t *testing.T) {
	tbl, _, _ := landMatchup(t)
	on := landSearcher(tbl, func(o *Options) { o.LandAbNow = true })
	off := landSearcher(tbl, func(o *Options) {})
	differ := 0
	hits := landWalkGames(t, 20, func(s *dirac.State, d *dirac.Decision) bool {
		i := landAbility(s, d)
		if i < 0 {
			return false
		}
		if tbl.Cards[s.ObjCard(dirac.Obj(d.Options[i].Source()))].Name != "Evolving Wilds" {
			t.Fatalf("landAbility found %q", tbl.Cards[s.ObjCard(dirac.Obj(d.Options[i].Source()))].Name)
		}
		r := *on.Search(s, 3)
		if r.Stats.Searched != 0 || r.Stats.LandAbNow != 1 || r.NActs != 1 || r.Acts[0] != d.Options[i] {
			t.Fatalf("on: searched %d landabnow %d answer %x (want the crack %x)", r.Stats.Searched, r.Stats.LandAbNow, uint32(r.Acts[0]), uint32(d.Options[i]))
		}
		if r0 := off.Search(s, 3); r0.Acts[0] != d.Options[i] {
			differ++
		}
		return true
	})
	if hits < 5 || differ == 0 {
		t.Fatalf("%d crack decisions, %d answered otherwise off: the walk found too few", hits, differ)
	}
}

// TestLandKeysZeroAllocs: a search with every land key on allocates
// nothing.
func TestLandKeysZeroAllocs(t *testing.T) {
	tbl, _, _ := landMatchup(t)
	sr := landSearcher(tbl, func(o *Options) {
		o.LandSearch, o.LandPick, o.DedupSearch, o.LandAbNow, o.Noise, o.Sample = true, true, true, true, true, true
	})
	var states []*dirac.State
	landWalkGames(t, 30, func(s *dirac.State, d *dirac.Decision) bool {
		if d.Kind == dirac.DkPriority && hasLandPlay(d) || libSearch(d) {
			c := new(dirac.State)
			c.CopyFrom(s)
			states = append(states, c)
			return true
		}
		return false
	})
	for i := range states {
		sr.Search(states[i], 1) // warm
	}
	if a := testing.AllocsPerRun(3, func() {
		for i := range states {
			sr.Search(states[i], 1)
		}
	}); a != 0 {
		t.Fatalf("%v allocations per run", a)
	}
}
