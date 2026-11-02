package gorged

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/adams-shaun/gorge/dirac/adapter"
	"github.com/adams-shaun/gorge/server/host/httpapi"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

const benchDecks = "../../dirac/testdata/bench/fdn"

func diracConfig() config {
	return config{engine: adapter.Name, diracSets: "fdn", seats: 2, autoMana: true, botAutoPayMana: true, formatsRaw: "constructed", decks: benchDecks, decksGiven: true}
}

func TestDiracRequiresTwoSeats(t *testing.T) {
	c := diracConfig()
	c.seats = 4
	if err := c.checkDiracFlags(); err == nil || !strings.Contains(err.Error(), "-seats 2") {
		t.Fatalf("four seats: %v", err)
	}
}

func TestDiracFlagsRefuseCommander(t *testing.T) {
	for _, tc := range []struct {
		name string
		set  func(*config)
		want string
	}{
		{"commander", func(c *config) { c.formatsRaw = "constructed,commander" }, "-format commander"},
		{"manual mana", func(c *config) { c.autoMana = false }, "-auto-mana=false"},
		{"a bot's manual mana", func(c *config) { c.botAutoPayMana = false }, "-bot-auto-mana=false"},
		{"events", func(c *config) { c.events = true }, "-events"},
		{"network bot", func(c *config) { c.nets = []netPolicy{{policy: "az", net: "n"}} }, "-az-net"},
	} {
		c := diracConfig()
		tc.set(&c)
		if err := c.checkDiracFlags(); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: %v, want it to name %s", tc.name, err, tc.want)
		}
	}
	if err := diracConfig().checkDiracFlags(); err != nil {
		t.Fatalf("a servable config: %v", err)
	}
}

func TestDiracDecksDefaultToBench(t *testing.T) {
	c := diracConfig()
	c.decks, c.decksGiven, c.diracSets = defaultDecks, false, "blb,fdn"
	if got, want := c.diracDecks(), filepath.Join("dirac", "testdata", "bench", "blb"); got != want {
		t.Fatalf("default decks %s, want %s", got, want)
	}
	c.decks, c.decksGiven = "elsewhere", true
	if got := c.diracDecks(); got != "elsewhere" {
		t.Fatalf("named decks read as %s", got)
	}
}

func TestMissingTablesFailsAtStart(t *testing.T) {
	testutil.CorpusRegistry(t)
	cards := testutil.CorpusDirOrSkip(t)
	t.Setenv("GORGE_CARDS", t.TempDir())
	t.Setenv("DIRAC_TABLES", t.TempDir())
	c := diracConfig()
	c.diracSets = "blb"
	c.cards = cards
	c.spectator, c.dir = "public", t.TempDir()
	err := serve(t.Context(), c, nil)
	if err == nil || !strings.Contains(err.Error(), "make dirac-tables") {
		t.Fatalf("no tables: %v", err)
	}
}

func TestEmptyCatalogueFailsAtStart(t *testing.T) {
	c := diracConfig()
	c.decks = "../../testkit/testutil/decks"
	c.cards = testutil.CorpusDirOrSkip(t)
	c.spectator, c.dir = "public", t.TempDir()
	err := serve(t.Context(), c, nil)
	if err == nil || !strings.Contains(err.Error(), "no deck in") || !strings.Contains(err.Error(), "is not in the pool") {
		t.Fatalf("nothing playable: %v", err)
	}
}

// oneRefused is a bench deck directory with a deck dirac cannot play beside two it can.
func oneRefused(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, f := range []string{"000_FDN_top_00001_WB.json", "001_FDN_top_00032_WU.json"} {
		raw, err := os.ReadFile(filepath.Join(benchDecks, f))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, f), raw, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	raw, err := os.ReadFile("../../testkit/testutil/decks/dimir-tempo.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "dimir-tempo.json"), raw, 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func fetchJSON(t *testing.T, url string, into any) {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if err := json.NewDecoder(resp.Body).Decode(into); err != nil {
		t.Fatal(err)
	}
}

func postGame(t *testing.T, base string, body map[string]string) (int, []byte) {
	t.Helper()
	raw, _ := json.Marshal(body)
	resp, err := http.Post(base+"/api/games", "application/json", bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	out, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, out
}

func TestDecksCarryRefusal(t *testing.T) {
	c := diracConfig()
	c.decks, c.vsbot = oneRefused(t), true
	base, cancel, _ := startServe(t, c)
	defer cancel()
	var decks []httpapi.DeckInfo
	fetchJSON(t, base+"/api/decks", &decks)
	var refused int
	for _, d := range decks {
		switch {
		case d.ID != "dimir-tempo" && d.Refused != "":
			refused++
		case d.ID == "dimir-tempo" && !strings.Contains(d.Refused, "Underground Sea"):
			t.Errorf("the refused deck reads %q", d.Refused)
		}
	}
	if len(decks) != 3 || refused != 0 {
		t.Fatalf("%d decks, %d others refused", len(decks), refused)
	}
	status, body := postGame(t, base, map[string]string{"format": "constructed", "human_deck": "dimir-tempo"})
	if status != http.StatusBadRequest || !strings.Contains(string(body), "Underground Sea") {
		t.Fatalf("a game with the refused deck: %d %s", status, body)
	}
}

func TestCapabilitiesSayNoManualMana(t *testing.T) {
	c := diracConfig()
	c.vsbot = true
	base, cancel, _ := startServe(t, c)
	defer cancel()
	var caps map[string]any
	fetchJSON(t, base+"/api/capabilities", &caps)
	if caps["engine"] != adapter.Name || caps["manual_mana"] != false {
		t.Fatalf("capabilities %v", caps)
	}
}

func TestGorgeIsByteIdenticalWhenOff(t *testing.T) {
	base, cancel, _ := startServe(t, config{vsbot: true, seats: 2, autoMana: true, formatsRaw: "constructed"})
	defer cancel()
	resp, err := http.Get(base + "/api/capabilities")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if got := strings.TrimSpace(string(raw)); got != `{"events":false}` {
		t.Fatalf("capabilities %s", got)
	}
	var decks []httpapi.DeckInfo
	fetchJSON(t, base+"/api/decks", &decks)
	for _, d := range decks {
		if d.Refused != "" {
			t.Fatalf("deck %s refused on gorge: %s", d.ID, d.Refused)
		}
	}
}

// TestServesADiracGame plays a human seat against a bot to the end over HTTP, undoing
// one answer, and reads the finished game back.
func TestServesADiracGame(t *testing.T) {
	c := diracConfig()
	c.vsbot, c.botPolicy = true, "bot"
	base, cancel, _ := startServe(t, c)
	defer cancel()
	status, body := postGame(t, base, map[string]string{"format": "constructed", "bot_policy": "bot"})
	if status != http.StatusOK {
		t.Fatalf("create: %d %s", status, body)
	}
	var g httpapi.CreateGameResponse
	if err := json.Unmarshal(body, &g); err != nil {
		t.Fatal(err)
	}
	match := base + "/api/tables/" + g.Table + "/matches/1"
	call := func(method, path string, in any) (int, []byte) {
		var rd io.Reader
		if in != nil {
			raw, _ := json.Marshal(in)
			rd = bytes.NewReader(raw)
		}
		req, _ := http.NewRequest(method, match+path, rd)
		req.Header.Set("Authorization", "Bearer "+g.Token)
		req.Header.Set("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		out, _ := io.ReadAll(resp.Body)
		return resp.StatusCode, out
	}
	type option struct {
		Index int    `json:"index"`
		Kind  string `json:"kind"`
	}
	type pending struct {
		Seq    uint64   `json:"seq"`
		Player int      `json:"player"`
		Kind   string   `json:"kind"`
		Min    int      `json:"min"`
		Opts   []option `json:"options"`
	}
	answers, undone, rewinding := 0, false, false
	deadline := time.Now().Add(40 * time.Second)
	for {
		if time.Now().After(deadline) {
			t.Fatalf("no end after %d answers", answers)
		}
		s, raw := call(http.MethodGet, "/view?seat=0", nil)
		var v struct {
			Over bool `json:"over"`
		}
		if s == http.StatusNotFound {
			time.Sleep(time.Second / 100)
			continue
		}
		if err := json.Unmarshal(raw, &v); err != nil {
			t.Fatalf("view %d: %s", s, raw)
		}
		if v.Over {
			break
		}
		s, raw = call(http.MethodGet, "/pending?seat=0", nil)
		var d pending
		if s != http.StatusOK || json.Unmarshal(raw, &d) != nil || d.Kind == "" || d.Player != 0 {
			time.Sleep(time.Second / 100)
			continue
		}
		choices := []int{}
		if d.Kind == "priority" {
			for _, o := range d.Opts {
				if o.Kind == "pass" {
					choices = []int{o.Index}
				}
			}
		} else {
			for i := 0; i < d.Min; i++ {
				choices = append(choices, d.Opts[i].Index)
			}
		}
		s, raw = call(http.MethodPost, "/intent?seat=0", map[string]any{"seq": d.Seq, "player": 0, "choices": choices})
		// An accepted undo is serviced after its 204, so the decision just read may be
		// the one it rewinds past, and the server is right to refuse the answer.
		if s == http.StatusConflict && rewinding {
			continue
		}
		if s != http.StatusNoContent {
			t.Fatalf("%s answer: %d %s", d.Kind, s, raw)
		}
		rewinding = false
		answers++
		if answers == 12 && !undone {
			undone = true
			if s, raw := call(http.MethodPost, "/undo?seat=0", struct{}{}); s != http.StatusNoContent {
				t.Fatalf("undo: %d %s", s, raw)
			}
			rewinding = true
		}
	}
	s, raw := call(http.MethodGet, "/events?seat=0", nil)
	var evs []struct {
		Line string `json:"line"`
	}
	if err := json.Unmarshal(raw, &evs); s != http.StatusOK || err != nil || len(evs) == 0 {
		t.Fatalf("events %d: %v", s, err)
	}
	if !strings.Contains(evs[len(evs)-1].Line, "win") && !strings.Contains(evs[len(evs)-1].Line, "draw") {
		t.Fatalf("the log ends on %q", evs[len(evs)-1].Line)
	}
	if s, raw := call(http.MethodGet, "/view?seat=0&seq=20", nil); s != http.StatusOK {
		t.Fatalf("a replayed view of the finished game: %d %s", s, raw)
	}
}

func TestUnknownEngineNamesEveryEngine(t *testing.T) {
	err := serve(t.Context(), config{engine: "x"}, nil)
	if err == nil || !strings.Contains(err.Error(), `engine "x" is not linked into this server (available: dirac, gorge)`) {
		t.Fatalf("-engine x: %v", err)
	}
}

func TestDecksFlagIsHonouredAtItsDefaultValue(t *testing.T) {
	parsed := func(args ...string) *config {
		t.Helper()
		fs, c := flagsFor(Extensions{})
		if err := fs.Parse(args); err != nil {
			t.Fatal(err)
		}
		return c
	}
	if got, want := parsed("-engine", "dirac").diracDecks(), filepath.Join("dirac", "testdata", "bench", "fdn"); got != want {
		t.Errorf("no -decks: %s, want %s", got, want)
	}
	if got := parsed("-engine", "dirac", "-decks", defaultDecks).diracDecks(); got != defaultDecks {
		t.Errorf("-decks %s: read as %s", defaultDecks, got)
	}
}
