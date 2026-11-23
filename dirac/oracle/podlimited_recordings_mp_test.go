package oracle

import (
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The pod limits the kernel's StallTwoSeat once held, lifted for every seat
// count: speed, a battle's protector, the skip-untap marks, and four cards
// kgen once marked two-seat only for reading "the opponent" as one player
// (The Weaver King, Etrata, Ravenloft Adventurer, Vulcan Mind-Meld). Each
// mechanism's witness is a recorded four-seat game, testdata/repro/
// podlimited_<mechanism>_four_seat_pod.repro.json: pool "one-off", its decks'
// cards named, the table built from them alone (printedDelveSet), so lowering
// another card re-deals nothing. A witness was searched among a focus pool's
// pods (kfuzz -synth <pool> -seats 4 -focus <card>) for a clean game in which
// the mechanism fired, and is the shortest such game found.
//
// The replay serves the whole recorded script, requires no divergence and no
// stall, and then asks the witness's own question of gorge's finished game: a
// recording that no longer reaches its mechanism fails instead of passing as a
// different game.

// podWitnessTally counts what a game of a witness card showed.
type podWitnessTally struct {
	card                       string
	triggers, counters, played int
	protectors, farExilers     int
	speedSeat                  [4]int
	activations                int
}

// tallyPodGame reads gorge's finished game for card's mechanism events.
func tallyPodGame(g *GorgeSide, card, counter string) podWitnessTally {
	y := podWitnessTally{card: card}
	named := func(id state.ObjID) bool {
		o := g.E.G.Obj(id)
		return o != nil && o.Face() != nil && o.Face().Name == card
	}
	for j := range g.E.L.Events {
		e := &g.E.L.Events[j]
		switch {
		case e.Kind == events.TriggerPush && named(e.Obj):
			y.triggers++
		case e.Kind == events.AbilityPush && named(e.Obj):
			y.activations++
		case e.Kind == events.CounterChange && counter != "" && e.Counter == counter:
			y.counters++
		case e.Kind == events.Choose && e.Counter == "protector":
			y.protectors++ // CR 310.10
		case e.Kind == events.SpeedChange && int(e.Player) < len(y.speedSeat):
			y.speedSeat[e.Player]++
		case e.Kind == events.MoveZone && named(e.Obj) && e.To != state.ZHand && e.To != state.ZLibrary:
			y.played++
		case e.Kind == events.MoveZone && e.To == state.ZExile && strings.HasPrefix(e.Counter, "exiled_with_face_down"):
			// the looker (Amount) of a card the opponent exiled is the exiler: a seat
			// other than the one after the caster is a target a two-seat reading ("the
			// opponent" = the next seat) would miss
			if src := g.E.G.Obj(e.IDs[0]); e.Counter == "exiled_with_face_down_maylook" && src != nil &&
				int32(src.Owner) != e.Amount && (int32(src.Owner)+1)%4 != e.Amount {
				y.farExilers++
			}
		}
	}
	return y
}

// podWitnessCheck is what each recording's game must show; the key is the
// recording's file name without podlimited_ and _four_seat_pod.
var podWitnessCheck = map[string]struct {
	card, counter string
	ok            func(y podWitnessTally) bool
	what          string
}{
	"weaver_king": {"The Weaver King", "", func(y podWitnessTally) bool { return y.triggers > 0 }, "a Weaver King trigger"},
	"etrata": {"Etrata, the Silencer", "HIT", func(y podWitnessTally) bool { return y.triggers > 0 && y.counters > 0 },
		"an Etrata trigger exiling a creature with a hit counter"},
	"ravenloft": {"Ravenloft Adventurer", "HIT", func(y podWitnessTally) bool { return y.counters > 0 },
		"a creature exiled with a hit counter"},
	"vulcan": {"Vulcan Mind-Meld", "", func(y podWitnessTally) bool { return y.farExilers > 0 },
		"a card exiled face down by an opponent past the next seat"},
	"battle_protector": {"Invasion of Zendikar", "", func(y podWitnessTally) bool { return y.protectors > 0 && y.played > 0 },
		"a battle entering with a chosen protector"},
	"speed": {"Pride of the Road", "", func(y podWitnessTally) bool { return y.speedSeat[2] > 0 && y.speedSeat[3] > 0 },
		"speed starting at seats 2 and 3"},
	"skip_untap": {"Homarid Warrior", "", func(y podWitnessTally) bool { return y.played > 0 && y.activations > 0 },
		"Homarid Warrior's skip-untap ability activated"},
}

// podWitnessFiles is the recordings under testdata/repro for the witnesses.
func podWitnessFiles(t *testing.T) map[string]string {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join("testdata", "repro", "podlimited_*_four_seat_pod.repro.json"))
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]string{}
	for _, p := range paths {
		key := strings.TrimSuffix(strings.TrimPrefix(filepath.Base(p), "podlimited_"), "_four_seat_pod.repro.json")
		out[key] = p
	}
	return out
}

// podWitnessReplay plays the recording at path on the table its own four
// decks need, serving the whole script, and returns the result with gorge's
// finished game's tally.
func podWitnessReplay(t *testing.T, path, key string) (Result, podWitnessTally, *Repro) {
	t.Helper()
	reg := synthReg(t)
	r, err := ReadRepro(path)
	if err != nil {
		t.Fatal(err)
	}
	if r.Pool != "one-off" || len(r.Cards) != 2 || len(r.MoreCards) != 2 || r.Stall {
		t.Fatalf("pool %q, %d+%d decks, stall %v: want a clean one-off four-seat pod", r.Pool, len(r.Cards), len(r.MoreCards), r.Stall)
	}
	m, err := r.Matchup(reg)
	if err != nil {
		t.Fatal(err)
	}
	lists := [][]string{r.Cards[0], r.Cards[1], r.MoreCards[0], r.MoreCards[1]}
	var all []string
	for _, l := range lists {
		all = append(all, l...)
	}
	set := cardset.Raw(printedDelveSet(t, [2][]string{all, nil}))
	es := make([][]cardset.Entry, len(lists))
	for i, l := range lists {
		es[i] = ReproEntries(reg, set, l)
	}
	tbl, _, err := set.MatchupN(es)
	if err != nil {
		t.Fatal(err)
	}
	rc := r.RunConfig()
	rc.MaxDecisions = len(r.Actions) + 200
	k, err := SetSide(set, tbl, rc.Config)(m, r.Seed)
	if err != nil {
		t.Fatal(err)
	}
	defer CloseSide(k)
	g, err := NewGorgeSide(m, r.Seed, rc.Config, nil)
	if err != nil {
		t.Fatal(err)
	}
	pol := &ScriptPolicy{Actions: r.Actions}
	res := LockstepLed(m, r.Seed, g, k, pol, rc)
	if msg := reproSkipFailure(r, pol); msg != "" {
		t.Fatal(msg)
	}
	w := podWitnessCheck[key]
	return res, tallyPodGame(g, w.card, w.counter), r
}

// TestPodLimitedRecordingsLockstep replays each mechanism's four-seat witness
// in lockstep, and requires its mechanism to show in the game.
func TestPodLimitedRecordingsLockstep(t *testing.T) {
	files := podWitnessFiles(t)
	keys := make([]string, 0, len(podWitnessCheck))
	for k := range podWitnessCheck {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if files[key] == "" {
			t.Errorf("no recording podlimited_%s_four_seat_pod.repro.json", key)
		}
	}
	for key := range files {
		if _, ok := podWitnessCheck[key]; !ok {
			t.Errorf("%s has no witness check: add it to podWitnessCheck", files[key])
		}
	}
	for _, key := range keys {
		path := files[key]
		if path == "" {
			continue
		}
		t.Run(key, func(t *testing.T) {
			t.Parallel()
			res, y, _ := podWitnessReplay(t, path, key)
			if res.Div != nil {
				t.Fatalf("divergence at %d: %s %s %v %s", res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff, res.Div.Detail)
			}
			if res.Stall != "" {
				t.Fatalf("stall %s", res.Stall)
			}
			if w := podWitnessCheck[key]; !w.ok(y) {
				t.Fatalf("the game shows no %s: %+v", w.what, y)
			}
		})
	}
}
