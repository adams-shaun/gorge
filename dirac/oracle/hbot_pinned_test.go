package oracle

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/dirac/search/hbot"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/deck"
	"github.com/adams-shaun/gorge/engine/state"
)

// A pinned hbot game is pinnedGames' recorded game for the hbot parity runs: gorge's
// heuristic bot chooses every action, so the record is the two decklists and the
// seed, not the actions. Nothing about it depends on how a synthetic pool deals its
// pairs, so lowering or refusing a card elsewhere in the pool cannot move it.
//
// Each call plays the games stored for the calling test (testdata/pins/<test
// name>.<n>.json, as pinnedGames names them), requires each game to satisfy every
// condition in reached at some decision, and fails on each disagreement wrong names. A
// game that no longer reaches a condition fails, and the games are recorded again with
//
//	PINS_RECORD=1 go test -count=1 -run '^TestName$' ./dirac/oracle
//
// which keeps the recorded games that still satisfy every condition, then scans the
// synthetic pool's pairs (dealt with the run's ScanSeed) for others that do, up to hbotPinGames.
const hbotPinGames = 2

// hbotPinDecisions bounds each pinned hbot game, as the pair runs did.
const hbotPinDecisions = 20000

// hbotPin is one recorded hbot game.
type hbotPin struct {
	Format string       `json:"format"` // "hbot-pin-1"
	Decks  [2]deck.File `json:"decks"`
	Seed   uint64       `json:"seed"`
}

// hbotCond is a condition a pinned hbot game must reach at some gorge decision: c is
// the decision as gorge asks it, gorge its heuristic bot's answer.
type hbotCond func(c *PickCtx, gorge []Canon) bool

// answersWith is the condition that gorge's bot answers a decision with an action of
// the choose kind named, as canon renders it ("choose:tapcost"), and that also meets
// each of also.
func answersWith(kind string, also ...hbotCond) hbotCond {
	return func(c *PickCtx, gorge []Canon) bool {
		if len(gorge) == 0 || !strings.HasPrefix(gorge[0].String(), kind+"(") {
			return false
		}
		for _, f := range also {
			if !f(c, gorge) {
				return false
			}
		}
		return true
	}
}

// promptIs is the condition that the decision's prompt is p.
func promptIs(p string) hbotCond {
	return func(c *PickCtx, _ []Canon) bool {
		d := c.G.E.Pending()
		return d != nil && d.Prompt == p
	}
}

// decidesWith is the condition that the deciding player controls a permanent named name.
func decidesWith(name string) hbotCond {
	return func(c *PickCtx, _ []Canon) bool {
		g := c.G.E.G
		d := c.G.E.Pending()
		if d == nil {
			return false
		}
		for _, id := range g.Zone(state.ZBattlefield, d.Player) {
			if o := g.Obj(id); o != nil && o.Face() != nil && o.Face().Name == name {
				return true
			}
		}
		return false
	}
}

// inHand is the condition that the deciding player holds a card of one of the names.
func inHand(names ...string) hbotCond {
	return func(c *PickCtx, _ []Canon) bool {
		g := c.G.E.G
		d := c.G.E.Pending()
		if d == nil {
			return false
		}
		for _, id := range g.Zone(state.ZHand, d.Player) {
			if o := g.Obj(id); o != nil && o.Face() != nil && slices.Contains(names, o.Face().Name) {
				return true
			}
		}
		return false
	}
}

// casts is the condition that gorge's bot answers by casting a card named name.
func casts(name string) hbotCond {
	return func(c *PickCtx, gorge []Canon) bool {
		if len(gorge) == 0 || gorge[0].Kind() != ActCast {
			return false
		}
		g := c.G.E.G
		for p := range g.Players {
			for _, z := range []state.Zone{state.ZHand, state.ZGraveyard, state.ZExile, state.ZLibrary} {
				for _, id := range g.Zone(z, state.PlayerID(p)) {
					if c.G.InstRef(id) != gorge[0].Src() {
						continue
					}
					if o := g.Obj(id); o != nil && o.Face() != nil && o.Face().Name == name {
						return true
					}
				}
			}
		}
		return false
	}
}

// hbotPinRun is how a pinned hbot game is played: the synthetic pool, the set its
// matchups are built from (nil: the pool's supported set), whether the kernel bot's
// facts read that set (else the FDN set's, as HBotFacts), and the seed the record
// scan deals the pool's pairs with.
type hbotPinRun struct {
	Pool         string
	Set          cardset.Set
	FactsFromSet bool
	ScanSeed     uint64
}

// pinnedHBotGames plays this call's recorded hbot games of run on cfg, requiring
// each to satisfy every condition in reached, and fails on each disagreement wrong
// names in a game's records. The record scan keeps only games wrong names none in,
// as pinnedGames keeps only games that play clean.
func pinnedHBotGames(t *testing.T, run hbotPinRun, cfg Config, wrong func(ParityRec) bool, reached ...hbotCond) {
	t.Helper()
	path := pinPath(t)
	reg := synthReg(t)
	if run.Set == nil {
		run.Set = cardsettest.Synth(t, run.Pool)
	}
	if os.Getenv("PINS_RECORD") == "1" {
		if err := recordHBotPins(t, path, run, reg, cfg, wrong, reached); err != nil {
			t.Fatal(err)
		}
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("no recorded games at %s (%v): record them with PINS_RECORD=1", path, err)
	}
	var games []hbotPin
	if err := json.Unmarshal(b, &games); err != nil || len(games) == 0 {
		t.Fatalf("%s: %v", path, err)
	}
	for i, g := range games {
		what := fmt.Sprintf("%s game %d (%s v %s seed %d)", path, i, g.Decks[0].Name, g.Decks[1].Name, g.Seed)
		res, hit, err := playHBotPin(reg, run, g, cfg, reached)
		if err != nil {
			t.Fatalf("%s: %v", what, err)
		}
		for j, h := range hit {
			if !h {
				t.Errorf("%s never satisfies condition %d of %d: record the games again (PINS_RECORD=1)", what, j+1, len(reached))
			}
		}
		for _, r := range res.Recs {
			if wrong(r) {
				t.Errorf("%s: %s", what, r.Where)
			}
		}
	}
}

// playHBotPin plays the recorded game g with gorge's heuristic bot and the kernel's
// hbot port compared at every decision.
// It reports which of the conditions in reached the game reached.
func playHBotPin(reg *cards.Registry, run hbotPinRun, g hbotPin, cfg Config, reached []hbotCond) (HBotParityResult, []bool, error) {
	if g.Decks[0].Name == g.Decks[1].Name {
		return HBotParityResult{}, nil, fmt.Errorf("both decks are named %s", g.Decks[0].Name)
	}
	hit := make([]bool, len(reached))
	observe := func(c *PickCtx, gorge []Canon) {
		for i, f := range reached {
			if !hit[i] && f(c, gorge) {
				hit[i] = true
			}
		}
	}
	c := &Corpus{Reg: reg, decks: map[string][]*cards.Card{}}
	c.AddDecks(map[string]deck.File{g.Decks[0].Name: g.Decks[0], g.Decks[1].Name: g.Decks[1]})
	mk := func(tbl *kop.Table) KernelBot {
		if run.FactsFromSet {
			return hbot.New(HBotFactsSet(reg, tbl, nil, run.Set))
		}
		return hbot.New(HBotFacts(reg, tbl, nil))
	}
	res, err := HBotParityGameOpts(c, g.Decks[0].Name, g.Decks[1].Name, g.Seed, cfg, hbotPinDecisions, mk, HBotParityOptions{Set: run.Set, Observe: observe})
	return res, hit, err
}

// recordHBotPins writes to path the games already there that still satisfy every
// condition with nothing wrong, then those of the synthetic pool's pairs that do, up to hbotPinGames.
func recordHBotPins(t *testing.T, path string, run hbotPinRun, reg *cards.Registry, cfg Config, wrong func(ParityRec) bool, reached []hbotCond) error {
	t.Helper()
	ok := func(g hbotPin) bool {
		res, hit, err := playHBotPin(reg, run, g, cfg, reached)
		return err == nil && res.Res.Div == nil && !contains(hit, false) && !slices.ContainsFunc(res.Recs, wrong)
	}
	var games []hbotPin
	if b, err := os.ReadFile(path); err == nil {
		var old []hbotPin
		if json.Unmarshal(b, &old) == nil {
			for _, g := range old {
				if len(games) < hbotPinGames && ok(g) {
					games = append(games, g)
				}
			}
		}
	}
	kept := len(games)
	c := &Corpus{Reg: reg, decks: map[string][]*cards.Card{}}
	pl, files, err := SynthPool(reg, run.Set, 1, SynthDecks)
	if err != nil {
		return err
	}
	c.AddDecks(files)
	for pair := 0; pair < pinScan && len(games) < hbotPinGames; pair++ {
		p := PairingAt(pl, pair, run.ScanSeed)
		g := hbotPin{Format: "hbot-pin-1", Decks: [2]deck.File{files[p.A], files[p.B]}, Seed: p.Seed}
		g.Decks[0].Name, g.Decks[1].Name = p.A, p.B
		dup := false
		for _, h := range games {
			dup = dup || h.Seed == g.Seed && h.Decks[0].Name == g.Decks[0].Name && h.Decks[1].Name == g.Decks[1].Name
		}
		if dup || !ok(g) { // a pair the set refuses holds no game
			continue
		}
		games = append(games, g)
	}
	if len(games) == 0 {
		return fmt.Errorf("no pair of %s within %d satisfies every condition", run.Pool, pinScan)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, err := json.Marshal(games)
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, append(b, '\n'), 0o644); err != nil {
		return err
	}
	t.Logf("recorded %d games to %s, %d of them kept", len(games), path, kept)
	return nil
}
