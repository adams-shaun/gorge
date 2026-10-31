package conformance

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/game"
	"github.com/adams-shaun/gorge/engine/game/gorgegame"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

const deckDir = "../../cmd/enginebench/testdata/decks"

// fdnPairs are enginebench's two workload pairs, both ways round.
var fdnPairs = [][2]string{
	{"FDN_top_04956_UG", "FDN_top_20626_WG"},
	{"FDN_top_07961_WR", "FDN_top_02581_UR"},
	{"FDN_top_20626_WG", "FDN_top_04956_UG"},
	{"FDN_top_02581_UR", "FDN_top_07961_WR"},
}

const (
	selfCheckGames = 20
	// minFrames is far below any game played to a result. A wire that ended a
	// game early would agree with itself, so the self-check holds it to this.
	minFrames = 100
)

func TestNativeConformanceGorgeSelf(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	for g := range selfCheckGames {
		t.Run(fmt.Sprintf("game%02d", g), func(t *testing.T) {
			t.Parallel()
			pair := fdnPairs[g%len(fdnPairs)]
			c := game.Config{
				Seed:   uint64(1000 + g),
				Names:  []string{pair[0], pair[1]},
				Decks:  [][]*cards.Card{fdnDeck(t, reg, pair[0]), fdnDeck(t, reg, pair[1])},
				Tokens: reg.Tokens,
			}
			frames := 0
			random := Random(c.Seed)
			pick := func(f Frame) string {
				frames++
				if frames == 1 {
					assertFrameHasContent(t, f)
				}
				return random(f)
			}
			if d := Run(t, Native{}, gorgegame.Engine{}, gorgegame.Engine{}, c, pick); d != nil {
				t.Fatal(d)
			}
			if frames < minFrames {
				t.Fatalf("the game finished after %d frames, want at least %d", frames, minFrames)
			}
			t.Logf("%s vs %s: %d frames", pair[0], pair[1], frames)
		})
	}
}

// assertFrameHasContent fails on a frame a client could not play from: no
// answers, or a state missing a seat.
func assertFrameHasContent(t *testing.T, f Frame) {
	t.Helper()
	if len(f.Answers) == 0 {
		t.Fatalf("frame %s has no answers", f.Key)
	}
	for _, viewer := range []string{"seat0", "seat1", "spectator"} {
		v, _ := f.State[viewer].(map[string]any)
		players, _ := v["players"].([]any)
		if len(players) != 2 {
			t.Fatalf("frame %s: %s sees %d players", f.Key, viewer, len(players))
		}
	}
}

// fdnDeck reads a Forge .dck main deck: "N [SET:num] Name" lines under a NAME:
// header.
func fdnDeck(t *testing.T, reg *cards.Registry, name string) []*cards.Card {
	t.Helper()
	f, err := os.Open(filepath.Join(deckDir, name+".dck"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	var deck []*cards.Card
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "NAME:") {
			continue
		}
		count, rest, ok := strings.Cut(line, " ")
		if !ok {
			t.Fatalf("%s: bad line %q", name, line)
		}
		n, err := strconv.Atoi(count)
		if err != nil {
			t.Fatalf("%s: bad count in %q", name, line)
		}
		_, card, ok := strings.Cut(rest, "] ")
		if !ok {
			t.Fatalf("%s: bad line %q", name, line)
		}
		c, ok := reg.Lookup(card)
		if !ok {
			t.Fatalf("%s: %q is not in the corpus", name, card)
		}
		for range n {
			deck = append(deck, c)
		}
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
	return deck
}

// stubWire plays scripted frames, moving one life total on the second session
// opened.
type stubWire struct{ opened int }

func (w *stubWire) Name() string { return "stub" }

func (w *stubWire) Open(testing.TB, game.Engine, game.Config) Session {
	w.opened++
	return &stubSession{lifeDelta: w.opened - 1}
}

type stubSession struct {
	frame     int
	lifeDelta int
}

func (s *stubSession) Next() (Frame, bool) {
	s.frame++
	life := 20.0
	if s.frame == 3 {
		life += float64(s.lifeDelta)
	}
	return Frame{
		Key:     "seat0/priority",
		Answers: []string{"pass/-/0/"},
		State: map[string]any{"spectator": map[string]any{
			"players": []any{map[string]any{"life": 20.0}, map[string]any{"life": life}},
		}},
	}, s.frame < 5
}

func (s *stubSession) Answer(string) error { return nil }

func TestDiffNamesTheFirstPath(t *testing.T) {
	c := game.Config{Seed: 4242}
	d := Run(t, &stubWire{}, nil, nil, c, First())
	if d == nil {
		t.Fatal("a moved life total produced no diff")
	}
	const path = "$.state.spectator.players[1].life"
	if d.Path != path || d.Frame != 2 {
		t.Fatalf("diff at %s frame %d, want %s frame 2", d.Path, d.Frame, path)
	}
	text := d.String()
	for _, want := range []string{path, "a=20", "b=21", "seed 4242"} {
		if !strings.Contains(text, want) {
			t.Errorf("%q does not name %q", text, want)
		}
	}
}

// tokenFrame is a seat's view of a token, under the given object id, enchanted
// and blocking, with the priority decision that can act on it.
func tokenFrame(token int) nativeRaw {
	seat := fmt.Sprintf(`{"viewer":0,"players":[
		{"seat":0,"life":20,"hand":[{"id":5,"token":"#5","name":"Bolt","tapped":false,"summon_sick":false,"damage":0,"attacking":false}],
		 "battlefield":[
			{"id":%[1]d,"token":"#%[1]d","name":"Goblin","tapped":true,"summon_sick":true,"damage":1,"attacking":false},
			{"id":6,"token":"#6","name":"Aura","attached_to":%[1]d,"tapped":false}],
		 "graveyard":[],"exile":[],"command":[],"commanders":[],
		 "cmd_damage":{"%[1]d":3},
		 "potential_actions":[{"kind":"ability","obj":%[1]d,"ability":1,"label":"Goblin: pump"}]},
		{"seat":1,"life":20,"hand":null,"battlefield":[{"id":7,"blocked_by":[%[1]d]}],"graveyard":[],"exile":[],"command":[],"commanders":[]}],
		"stack":[],"pending":[],
		"decision":{"seq":99,"player":0,"kind":"priority","prompt":"Act","min":1,"max":1,"options":[]}}`, token)
	pending := fmt.Sprintf(`{"seq":99,"player":0,"kind":"priority","prompt":"Act","min":1,"max":1,"options":[
		{"index":0,"kind":"pass","label":"Pass","player":0},
		{"index":1,"kind":"concede","label":"Concede","player":0},
		{"index":2,"kind":"activate","label":"Tap Goblin","player":0,"obj":%[1]d},
		{"index":3,"kind":"ability","label":"Goblin: sacrifice","player":0,"obj":%[1]d},
		{"index":4,"kind":"cast","label":"Cast Bolt","player":0,"obj":5}]}`, token)
	return nativeRaw{Seats: [][]byte{[]byte(seat), []byte(`{"viewer":1,"players":[],"stack":[],"pending":[]}`)},
		Spectator: []byte(`{"viewer":255,"players":[],"stack":[],"pending":[]}`), Pending: []byte(pending)}
}

func TestNormaliseRenamesTokens(t *testing.T) {
	low, _, err := normaliseNative(tokenFrame(9))
	if err != nil {
		t.Fatal(err)
	}
	high, _, err := normaliseNative(tokenFrame(4000))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(low, high) {
		t.Fatalf("a token's object id survived normalising:\n%s\n%s", frameJSON(t, low), frameJSON(t, high))
	}
	want := Frame{
		Key: "seat0/priority",
		Answers: []string{
			"ability/o2/0/",
			"ability/o2/1/",
			"cast/o1/0/",
			"pass/-/0/",
		},
		State: map[string]any{
			"seat0": map[string]any{
				"viewer": 0.0,
				"players": []any{
					map[string]any{"seat": 0.0, "life": 20.0,
						"hand": []any{map[string]any{"id": "o1", "token": "#o1", "name": "Bolt"}},
						"battlefield": []any{
							map[string]any{"id": "o2", "token": "#o2", "name": "Goblin", "tapped": true, "summon_sick": true, "damage": 1.0, "attacking": false},
							map[string]any{"id": "o3", "token": "#o3", "name": "Aura", "attached_to": "o2", "tapped": false},
						},
						"graveyard": []any{}, "exile": []any{}, "command": []any{}, "commanders": []any{},
						"cmd_damage": map[string]any{"o2": 3.0},
					},
					map[string]any{"seat": 1.0, "life": 20.0, "hand": nil,
						"battlefield": []any{map[string]any{"id": "o4", "blocked_by": []any{"o2"}}},
						"graveyard":   []any{}, "exile": []any{}, "command": []any{}, "commanders": []any{},
					},
				},
				"stack": []any{}, "pending": []any{},
				"decision": map[string]any{"player": 0.0, "kind": "priority", "min": 1.0, "max": 1.0},
			},
			"seat1":     map[string]any{"viewer": 1.0, "players": []any{}, "stack": []any{}, "pending": []any{}},
			"spectator": map[string]any{"viewer": 255.0, "players": []any{}, "stack": []any{}, "pending": []any{}},
		},
	}
	if !reflect.DeepEqual(low, want) {
		t.Fatalf("normalised frame:\n%s\nwant:\n%s", frameJSON(t, low), frameJSON(t, want))
	}
}

// optionFrame is a choice between a named value and an object with a per-object
// group, under the given object id.
func optionFrame(value, label string, id int) nativeRaw {
	d := fmt.Sprintf(`{"seq":1,"player":0,"kind":"choose","prompt":"Pick","min":1,"max":1,"options":[
		{"index":0,"kind":"name","label":%q,"player":0},
		{"index":1,"kind":"block","label":%q,"player":0,"obj":%d,"group":"blocker:%d"},
		{"index":2,"kind":"choice","label":"Other","player":0,"obj":%d,"group":"power:3"}]}`, value, label, id, id, id)
	view := func(viewer int, decision string) []byte {
		return []byte(fmt.Sprintf(`{"viewer":%d,"players":[],"stack":[],"pending":[]%s}`, viewer, decision))
	}
	return nativeRaw{Seats: [][]byte{view(0, `,"decision":`+d), view(1, "")}, Spectator: view(255, ""), Pending: []byte(d)}
}

func TestNormaliseKeepsWhatAnOptionOffers(t *testing.T) {
	norm := func(raw nativeRaw) map[string]any {
		f, _, err := normaliseNative(raw)
		if err != nil {
			t.Fatal(err)
		}
		return frameTree(f)
	}
	base := norm(optionFrame("Goblin", "Block", 9))
	if path, va, vb, found := firstDifference(base, norm(optionFrame("Goblin", "Block with it", 4000)), "$"); found {
		t.Errorf("an object's id or display label survived normalising: %s: %v, %v", path, va, vb)
	}
	path, _, _, found := firstDifference(base, norm(optionFrame("Elf", "Block", 9)), "$")
	if want := "$.state.seat0.decision.options[0].label"; !found || path != want {
		t.Errorf("a different named value diffed at %q, want %q", path, want)
	}
	options := base["state"].(map[string]any)["seat0"].(map[string]any)["decision"].(map[string]any)["options"].([]any)
	if got := options[2].(map[string]any)["group"]; got != "power:3" {
		t.Errorf("a group naming no object became %v", got)
	}
}

func frameJSON(t *testing.T, f Frame) []byte {
	t.Helper()
	b, err := json.MarshalIndent(f, "", " ")
	if err != nil {
		t.Fatal(err)
	}
	return b
}
