package gorged

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/adams-shaun/gorge/dirac/adapter"
	"github.com/adams-shaun/gorge/engine/deck"
	"github.com/adams-shaun/gorge/engine/view"
	"github.com/adams-shaun/gorge/server/protocol"
)

// errServeExited says serve() returned before it served the table under test.
var errServeExited = errors.New("serve exited")

// diracPollClient bounds a poll: a serve that exited leaves its listener open,
// so an unbounded GET would wait on a connection nobody answers.
var diracPollClient = &http.Client{Timeout: time.Second}

// commanderDeckDir writes two legal 100-card Commander decks over fdn: Giada and
// 99 Plains each. fdn is a set dirac holds, so a served table can deal them.
func commanderDeckDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for i, id := range []string{"fdn-command-a", "fdn-command-b"} {
		f := deck.File{
			Name:      fmt.Sprintf("fdn-command-%c", 'a'+i),
			Format:    "commander",
			Commander: "Giada, Font of Hope",
			Cards: []deck.Entry{
				{Name: "Giada, Font of Hope", Count: 1},
				{Name: "Plains", Count: 99},
			},
		}
		raw, err := json.Marshal(f)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, id+".json"), raw, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// pollJSON GETs url until it answers 200 and decodes the body into out. A serve
// that has exited is returned wrapped in errServeExited, so a startup refusal
// ends the poll instead of looking like a slow start.
func pollJSON(done <-chan error, url string, out any) error {
	select {
	case err := <-done:
		return fmt.Errorf("%w: %v", errServeExited, err)
	default:
	}
	resp, err := diracPollClient.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GET %s: %s", url, resp.Status)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

// TestDiracServeCommanderTable drives serve() itself on a two-seat Commander
// table of two fdn decks: the commander pool must survive the dirac deck check
// and reach host.AddTable, the table comes up live on dirac, and its opening
// view (seq 0) has both seats at 40 life with the commander in the command
// zone. It fails if serve drops the commander pool before dealing the tables.
func TestDiracServeCommanderTable(t *testing.T) {
	url, cancel, done := startServe(t, config{
		engine: adapter.Name, diracSets: "fdn", decks: commanderDeckDir(t), decksGiven: true,
		formatsRaw: "commander", tables: 1, seats: 2, pace: 0,
		autoMana: true, botAutoPayMana: true,
	})
	defer cancel()

	var tables []protocol.TableInfo
	deadline := time.Now().Add(10 * time.Second)
	for {
		err := pollJSON(done, url+"/api/tables", &tables)
		if errors.Is(err, errServeExited) {
			t.Fatal(err)
		}
		if err == nil && len(tables) == 1 && tables[0].State == protocol.TableLive {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("no live commander table served: %v %+v", err, tables)
		}
		time.Sleep(time.Second / 40)
	}
	if got := tables[0]; got.Format != "commander" || got.Engine != adapter.Name || got.Seats != 2 {
		t.Fatalf("table %+v, want a two-seat commander table on dirac", got)
	}

	var v view.View
	deadline = time.Now().Add(10 * time.Second)
	for {
		err := pollJSON(done, url+"/api/tables/t1/matches/1/view?seq=0", &v)
		if errors.Is(err, errServeExited) {
			t.Fatal(err)
		}
		if err == nil && len(v.Players) == 2 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("no opening view for the commander match: %v", err)
		}
		time.Sleep(time.Second / 40)
	}
	for _, p := range v.Players {
		if p.Life != 40 {
			t.Errorf("seat %s starts at %d life, want 40", p.Name, p.Life)
		}
		if len(p.Command) != 1 || len(p.Commanders) != 1 {
			t.Errorf("seat %s command zone %v, roster %v, want its commander in the zone", p.Name, p.Command, p.Commanders)
			continue
		}
		if got := p.Command[0].Name; got != "Giada, Font of Hope" {
			t.Errorf("seat %s command zone holds %q, want Giada, Font of Hope", p.Name, got)
		}
	}
}
