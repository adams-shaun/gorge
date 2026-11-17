package oracle

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// Scratch hunt for the Glaive of the Guildpact cast-offer divergence
// (options options): the kernel offers cast(Glaive {2}) at p0's priority and
// gorge does not. Not committed: a search harness, deleted before the fix's
// commit. GORGE_GLAIVE_HUNT=1 arms it; GORGE_GLAIVE_INTER names one interactor
// (12 copies in p0's deck), GORGE_GLAIVE_BATCH a "|" batch played in every deck.

func glaiveDeck(name string, inter, glaive, land int) []string {
	d := make([]string, 0, inter+glaive+land)
	for i := 0; i < inter; i++ {
		d = append(d, name)
	}
	for i := 0; i < glaive; i++ {
		d = append(d, "Glaive of the Guildpact")
	}
	for i := 0; i < land/2; i++ {
		d = append(d, "Plains", "Kher Keep")
	}
	for len(d) < 40 {
		d = append(d, "Plains")
	}
	return d
}

func plainsDeck() []string {
	d := make([]string, 0, 40)
	for i := 0; i < 20; i++ {
		d = append(d, "Plains", "Island")
	}
	return d
}

func TestZZGlaiveHunt(t *testing.T) {
	if os.Getenv("GORGE_GLAIVE_HUNT") == "" {
		t.Skip("hunt env not set")
	}
	// worktree root: the test binary runs with cwd = the package dir
	w, _ := os.Getwd()
	w = filepath.Dir(filepath.Dir(w))
	reg := synthReg(t)
	set, err := cardset.SynthByName("tail-39")
	if err != nil {
		t.Fatal(err)
	}
	set = cardset.Raw(set)
	entries := func(names []string) []cardset.Entry {
		es := make([]cardset.Entry, len(names))
		for i, n := range names {
			es[i] = cardset.Entry{Name: n, Count: 1}
		}
		return es
	}
	log := func(format string, args ...any) {
		f, _ := os.OpenFile(w+"/.ds4/scratch/hunt.out", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if f != nil {
			fmt.Fprintf(f, format, args...)
			f.Close()
		}
	}
	play := func(deck0 []string, seed uint64) string {
		r := &Repro{
			Decks:   [2]string{"hunt0", "hunt1"},
			Cards:   [2][]string{deck0, plainsDeck()},
			Seed:    seed,
			Surface: "autopay",
		}
		m, err := r.Matchup(reg)
		if err != nil {
			t.Fatal(err)
		}
		tbl, _, _, err := set.Matchup(entries(r.Cards[0]), entries(r.Cards[1]))
		if err != nil {
			t.Fatal(err)
		}
		rc := RunConfig{Config: Config{PassElision: true, GorgeAutopaySurface: true}, MaxTurns: 40, MaxDecisions: 4000}
		k, err := SetSide(set, tbl, rc.Config)(m, r.Seed)
		if err != nil {
			t.Fatal(err)
		}
		res := Lockstep(m, r.Seed, k, RandomPolicy(r.Seed), rc)
		CloseSide(k)
		if res.Div != nil {
			return fmt.Sprintf("DIV at %d: %s %s %v", res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
		}
		if res.Stall != "" {
			return "stall " + res.Stall
		}
		return "clean"
	}
	if env := os.Getenv("GORGE_GLAIVE_MINI"); env != "" {
		minis := map[string][]string{
			"lashwrithe": {"Lashwrithe", "Lashwrithe", "Lashwrithe", "Lashwrithe", "Lashwrithe", "Lashwrithe",
				"Marshal of Zhalfir", "Marshal of Zhalfir", "Marshal of Zhalfir", "Marshal of Zhalfir", "Marshal of Zhalfir", "Marshal of Zhalfir",
				"Hall of Tagsin", "Hall of Tagsin", "Hall of Tagsin", "Hall of Tagsin",
				"Plains", "Plains", "Plains", "Plains", "Plains", "Plains", "Plains", "Plains", "Plains", "Plains", "Plains", "Plains",
				"Mountain", "Mountain", "Mountain", "Mountain", "Mountain", "Mountain", "Mountain", "Mountain", "Mountain", "Mountain", "Mountain", "Mountain"},
			"glaive": {"Glaive of the Guildpact", "Glaive of the Guildpact", "Glaive of the Guildpact", "Glaive of the Guildpact",
				"Glaive of the Guildpact", "Glaive of the Guildpact", "Glaive of the Guildpact", "Glaive of the Guildpact",
				"Plains", "Plains", "Plains", "Plains", "Plains", "Plains", "Plains", "Plains",
				"Kher Keep", "Kher Keep", "Kher Keep", "Kher Keep", "Kher Keep", "Kher Keep", "Kher Keep", "Kher Keep",
				"Shivan Gorge", "Shivan Gorge", "Shivan Gorge", "Shivan Gorge", "Shivan Gorge", "Shivan Gorge", "Shivan Gorge", "Shivan Gorge",
				"Plains", "Plains", "Plains", "Plains", "Plains", "Plains", "Plains", "Plains"},
		}
		deck, ok := minis[env]
		if !ok {
			t.Fatalf("unknown mini %q", env)
		}
		for _, seed := range []uint64{1, 2, 3, 4, 5, 6} {
			log("HUNT mini %-12s seed %d: %s\n", env, seed, play(deck, seed))
		}
		return
	}
	if bp := os.Getenv("GORGE_GLAIVE_BATCHES"); bp != "" {
		b, err := os.ReadFile(w + "/" + bp)
		if err != nil {
			t.Fatal(err)
		}
		for bi, line := range strings.Split(strings.TrimSpace(string(b)), "\n") {
			names := strings.Split(line, "|")
			for i := 0; i < 8; i++ {
				deck := make([]string, 0, 40)
				for _, n := range names {
					deck = append(deck, n, n)
				}
				for j := 0; j < 8; j++ {
					deck = append(deck, "Glaive of the Guildpact")
				}
				for len(deck) < 40 {
					deck = append(deck, "Plains")
				}
				seed := uint64(i + 1)
				log("HUNT batch %02d %-30s seed %d: %s\n", bi, names[0], seed, play(deck, seed))
			}
		}
		return
	}
	if batch := os.Getenv("GORGE_GLAIVE_BATCH"); batch != "" {
		names := strings.Split(batch, "|")
		for i := 0; i < 8; i++ {
			deck := make([]string, 0, 40)
			for _, n := range names {
				deck = append(deck, n, n)
			}
			for j := 0; j < 8; j++ {
				deck = append(deck, "Glaive of the Guildpact")
			}
			for len(deck) < 40 {
				deck = append(deck, "Plains")
			}
			seed := uint64(i + 1)
			log("HUNT batch %-30s seed %d: %s\n", names[0], seed, play(deck, seed))
		}
		return
	}
	inter := os.Getenv("GORGE_GLAIVE_INTER")
	if inter == "" {
		inter = "Salvaged Manaworker"
	}
	for _, seed := range []uint64{1, 2, 3, 4, 5, 6, 7, 8} {
		log("HUNT %-24s seed %d: %s\n", inter, seed, play(glaiveDeck(inter, 12, 10, 18), seed))
	}
}
