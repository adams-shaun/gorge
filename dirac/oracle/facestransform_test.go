package oracle

import (
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
)

// The facestransform lockstep: constructed matchups over the generated
// facestransform pool -- the non-token transforming DFCs' three lifted
// routes (SetState Transform, ChangeZone Transformed$ True, Mode$
// Transformed) plus Norn's Inquisitor over an Incubator token and Tyrant's
// Scorn's CMC filter for CR 712.8e. The frozen FDN and BLB pair pools
// exercise none of these arms.

var ftDecks = [][2][]cardset.Entry{
	// NEO sagas: chapter III exiles the Saga and returns it transformed.
	{{
		{Name: "Befriending the Moths", Count: 2}, {Name: "Boseiju Reaches Skyward", Count: 2},
		{Name: "Era of Enlightenment", Count: 4}, // (Life of Toshiro Umezawa left the pool:, its mana ability)
		{Name: "Michiko's Reign of Truth", Count: 2}, {Name: "Okiba Reckoner Raid", Count: 2},
		{Name: "Tales of Master Seshiro", Count: 2}, {Name: "Teachings of the Kirin", Count: 2},
		{Name: "The Modern Age", Count: 2}, {Name: "The Restoration of Eiganjo", Count: 2},
		{Name: "The Shattered States Era", Count: 2},
		{Name: "Plains", Count: 5}, {Name: "Island", Count: 4}, {Name: "Swamp", Count: 3},
		{Name: "Mountain", Count: 3}, {Name: "Forest", Count: 3},
	}, {
		{Name: "Befriending the Moths", Count: 2}, {Name: "Boseiju Reaches Skyward", Count: 2},
		{Name: "Era of Enlightenment", Count: 4}, // (Life of Toshiro Umezawa left the pool:, its mana ability)
		{Name: "Michiko's Reign of Truth", Count: 2}, {Name: "Okiba Reckoner Raid", Count: 2},
		{Name: "Tales of Master Seshiro", Count: 2}, {Name: "Teachings of the Kirin", Count: 2},
		{Name: "The Modern Age", Count: 2}, {Name: "The Restoration of Eiganjo", Count: 2},
		{Name: "The Shattered States Era", Count: 2},
		{Name: "Plains", Count: 5}, {Name: "Island", Count: 4}, {Name: "Swamp", Count: 3},
		{Name: "Mountain", Count: 3}, {Name: "Forest", Count: 3},
	}},
	// FIN/LCI: battlefield transforms, graveyard returns and the CMC filter.
	{{
		{Name: "Cecil, Dark Knight", Count: 2}, {Name: "Exdeath, Void Warlock", Count: 2},
		{Name: "Sidequest: Card Collection", Count: 2}, {Name: "Sidequest: Hunt the Mark", Count: 2},
		{Name: "Vincent Valentine", Count: 2}, {Name: "Dion, Bahamut's Dominant", Count: 2},
		{Name: "Garland, Knight of Cornelia", Count: 2}, {Name: "Jecht, Reluctant Guardian", Count: 2},
		{Name: "Dowsing Device", Count: 2}, {Name: "Grasping Shadows", Count: 2}, {Name: "Norn's Inquisitor", Count: 2},
		{Name: "Converter Beast", Count: 2}, {Name: "Tyrant's Scorn", Count: 2},
		{Name: "Plains", Count: 3}, {Name: "Island", Count: 5}, {Name: "Swamp", Count: 3},
		{Name: "Mountain", Count: 1}, {Name: "Forest", Count: 2},
	}, {
		{Name: "Cecil, Dark Knight", Count: 2}, {Name: "Exdeath, Void Warlock", Count: 2},
		{Name: "Sidequest: Card Collection", Count: 2}, {Name: "Sidequest: Hunt the Mark", Count: 2},
		{Name: "Vincent Valentine", Count: 2}, {Name: "Dion, Bahamut's Dominant", Count: 2},
		{Name: "Garland, Knight of Cornelia", Count: 2}, {Name: "Jecht, Reluctant Guardian", Count: 2},
		{Name: "Dowsing Device", Count: 2}, {Name: "Grasping Shadows", Count: 2}, {Name: "Norn's Inquisitor", Count: 2},
		{Name: "Converter Beast", Count: 2}, {Name: "Tyrant's Scorn", Count: 2},
		{Name: "Plains", Count: 3}, {Name: "Island", Count: 5}, {Name: "Swamp", Count: 3},
		{Name: "Mountain", Count: 1}, {Name: "Forest", Count: 2},
	}},
	// Chocobo/Norn: a token transforms under Norn, and Chocobo's back-face
	// trigger follows its own flip.
	{{
		{Name: "Sidequest: Raise a Chocobo", Count: 4}, {Name: "Norn's Inquisitor", Count: 4},
		{Name: "Converter Beast", Count: 4}, {Name: "Dowsing Device", Count: 4},
		{Name: "Plains", Count: 8}, {Name: "Forest", Count: 8}, {Name: "Mountain", Count: 4}, {Name: "Island", Count: 4},
	}, {
		{Name: "Sidequest: Raise a Chocobo", Count: 4}, {Name: "Norn's Inquisitor", Count: 4},
		{Name: "Converter Beast", Count: 4}, {Name: "Dowsing Device", Count: 4},
		{Name: "Plains", Count: 8}, {Name: "Forest", Count: 8}, {Name: "Mountain", Count: 4}, {Name: "Island", Count: 4},
	}},
	// Mixed: the CMC filter meets transformed Sagas and the graveyard return.
	{{
		{Name: "Tyrant's Scorn", Count: 4}, {Name: "Dowsing Device", Count: 4},
		{Name: "Okiba Reckoner Raid", Count: 4}, {Name: "The Restoration of Eiganjo", Count: 4},
		{Name: "Garland, Knight of Cornelia", Count: 4},
		{Name: "Plains", Count: 5}, {Name: "Island", Count: 5}, {Name: "Swamp", Count: 5}, {Name: "Mountain", Count: 3}, {Name: "Forest", Count: 2},
	}, {
		{Name: "Tyrant's Scorn", Count: 4}, {Name: "Dowsing Device", Count: 4},
		{Name: "Okiba Reckoner Raid", Count: 4}, {Name: "The Restoration of Eiganjo", Count: 4},
		{Name: "Garland, Knight of Cornelia", Count: 4},
		{Name: "Plains", Count: 5}, {Name: "Island", Count: 5}, {Name: "Swamp", Count: 5}, {Name: "Mountain", Count: 3}, {Name: "Forest", Count: 2},
	}},
}

// ftCarriers names every card whose lifted route the test counts.
var ftCarriers = []string{
	"Befriending the Moths", "Boseiju Reaches Skyward", "Era of Enlightenment",
	"Michiko's Reign of Truth", "Okiba Reckoner Raid",
	"Tales of Master Seshiro", "Teachings of the Kirin", "The Modern Age",
	"The Restoration of Eiganjo", "The Shattered States Era",
	"Cecil, Dark Knight", "Exdeath, Void Warlock", "Sidequest: Card Collection",
	"Sidequest: Hunt the Mark", "Vincent Valentine", "Dion, Bahamut's Dominant",
	"Garland, Knight of Cornelia", "Jecht, Reluctant Guardian", "Dowsing Device",
	"Sidequest: Raise a Chocobo", "Grasping Shadows",
}

// ftTally counts the flipped front faces from gorge's event log. The kernel
// debug counters count the routes the event log does not name (MvTransformed
// entries, queued TrigTransformed triggers, CMC reads on a back face).
type ftTally struct {
	evAt     int
	flips    map[string]int
	token    int
	nonToken int
	norn     int
}

func newFTTally() *ftTally { return &ftTally{flips: map[string]int{}} }

func (t *ftTally) reset() { t.evAt = 0 }

func (t *ftTally) check(ref *GorgeSide, _ Side) *Divergence {
	evs := ref.E.L.Events
	if len(evs) < t.evAt {
		t.evAt = 0
	}
	for j := t.evAt; j < len(evs); j++ {
		e := &evs[j]
		if e.Kind == events.PutOnStack && e.Text == "Norn's Inquisitor" {
			t.norn++
			continue
		}
		if e.Kind != events.FlipFace || e.Text != "Transformed" {
			continue
		}
		ob := ref.E.G.Obj(e.Obj)
		if ob == nil || ob.Card == nil || len(ob.Card.Faces) == 0 {
			continue
		}
		name := ob.Card.Faces[0].Name
		t.flips[name]++
		if ob.IsToken {
			t.token++
		} else {
			t.nonToken++
		}
	}
	t.evAt = len(evs)
	return nil
}

// ftPrep is one matchup's decks and kernel table, built once: the table is
// read-only across games, so 1024 games do not rebuild it.
type ftPrep struct {
	md    [2][]cardset.Entry
	decks [2][]*cards.Card
	tbl   *kop.Table
}

func prepareFT(t *testing.T, reg *cards.Registry, set cardset.Set) []ftPrep {
	t.Helper()
	out := make([]ftPrep, len(ftDecks))
	for mi, md := range ftDecks {
		p := ftPrep{md: md}
		for s, d := range md {
			for _, e := range d {
				c, ok := reg.Lookup(e.Name)
				if !ok {
					t.Fatalf("%s not in the corpus", e.Name)
				}
				for j := 0; j < e.Count; j++ {
					p.decks[s] = append(p.decks[s], c)
				}
			}
			if len(p.decks[s]) != 40 {
				t.Fatalf("matchup %d deck %d: %d cards", mi, s, len(p.decks[s]))
			}
		}
		tbl, _, _, err := set.Matchup(md[0], md[1])
		if err != nil {
			t.Fatal(err)
		}
		p.tbl = tbl
		out[mi] = p
	}
	return out
}

func TestFacesTransformLockstep(t *testing.T) {
	if isolate(t) {
		return
	}

	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("facestransform")
	if err != nil {
		t.Fatal(err)
	}
	prep := prepareFT(t, reg, set)
	games := 1024
	if testing.Short() {
		games = 64
	}
	tally := newFTTally()
	oldStats := dirac.DebugFaceTransformStats
	dirac.DebugFaceTransformStats = &dirac.FaceTransformStats{}
	defer func() { dirac.DebugFaceTransformStats = oldStats }()
	for mi := range prep {
		p := prep[mi]
		for i := mi; i < games; i += len(prep) {
			seed := uint64(i/len(prep) + 1)
			cfg := Config{GorgeAutopaySurface: seed%2 == 0}
			m, err := NewMatchup(reg, [2]string{"facestransform-a", "facestransform-b"}, p.decks, 1)
			if err != nil {
				t.Fatal(err)
			}
			k, err := SetSide(set, p.tbl, cfg)(m, seed)
			if err != nil {
				t.Fatal(err)
			}
			tally.reset()
			rc := RunConfig{Config: cfg, MaxDecisions: 3000, MaxTurns: 30, Check: tally.check}
			res := Lockstep(m, seed, k, castActivatePolicy(seed), rc)
			CloseSide(k)
			if d := res.Div; d != nil {
				r := NewRepro(m, seed, rc, &res)
				_ = r.Write(filepath.Join(t.TempDir(), "ft-repro.json"))
				lo := d.At - 40
				if lo < 0 {
					lo = 0
				}
				t.Logf("matchup %d seed %d last actions %v", mi, seed, actionStrings(res.Actions[lo:d.At]))
				t.Fatalf("matchup %d seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", mi, seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
		}
	}
	st := dirac.DebugFaceTransformStats
	t.Logf("%d games, non-token flips %d, token flips %d, MvTransformed %d, triggers %d, Norn triggers %d, CMC back-face reads %d",
		games, tally.nonToken, tally.token, st.Enters, st.Fires, tally.norn, st.PredMV)
	// OpTransform on a non-token permanent: the flips that are not the
	// entering-transformed return (which is MvTransformed, not OpTransform).
	if got := tally.nonToken - st.Enters; got < 100 {
		t.Errorf("OpTransform on a non-token permanent %d, want at least 100", got)
	}
	if st.Enters < 30 {
		t.Errorf("MvTransformed entries %d, want at least 30", st.Enters)
	}
	if st.Fires < 20 {
		t.Errorf("TrigTransformed triggers %d, want at least 20", st.Fires)
	}
	if tally.token < 5 {
		t.Errorf("token transforms %d, want at least 5", tally.token)
	}
	if tally.norn < 5 {
		t.Errorf("Norn's Inquisitor triggers %d, want at least 5", tally.norn)
	}
	if st.PredMV < 10 {
		t.Errorf("CMC reads on a back face %d, want at least 10", st.PredMV)
	}
	for _, n := range ftCarriers {
		if tally.flips[n] == 0 {
			t.Logf("carrier %s never transformed or entered transformed (0)", n)
		}
	}
	// Hunt the Mark needs an opponent's creature to die and three Treasures;
	// Raise a Chocobo needs four Birds. Both lower, but these random decks
	// rarely meet the condition; the unit tests drive them directly.
	for _, n := range []string{"Sidequest: Hunt the Mark", "Sidequest: Raise a Chocobo"} {
		if tally.flips[n] != 0 {
			t.Logf("optional carrier %s transformed %d times", n, tally.flips[n])
		}
	}
}

// TestFacesTransformMutation disables the ChangeZone Transformed$ entry and
// requires the lockstep to see it: without it a back face never enters, so
// the two engines diverge.
func TestFacesTransformMutation(t *testing.T) {
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("facestransform")
	if err != nil {
		t.Fatal(err)
	}
	md := ftDecks[1]
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
	dirac.DebugIgnoreTransformed = true
	defer func() { dirac.DebugIgnoreTransformed = false }()
	for seed := uint64(1); seed <= 8; seed++ {
		cfg := Config{}
		m, err := NewMatchup(reg, [2]string{"facestransform-a", "facestransform-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		res := Lockstep(m, seed, k, castActivatePolicy(seed), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 30})
		CloseSide(k)
		if res.Div != nil {
			t.Logf("seed %d: the mutation diverges at decision %d (%s %s)", seed, res.Div.At, res.Div.What, res.Div.Field)
			return
		}
	}
	t.Fatal("ignoring the transformed entry diverged in no game: the lockstep does not see the back face")
}
