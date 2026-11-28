package gorged

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/adams-shaun/gorge/server/protocol"
)

// demandLogMax is the size at which the demand log rotates: the live file
// and one older generation keep it under twice this on disk.
const demandLogMax = 8 << 20

// demandLog appends, for each checked deck, the card-pool names of the cards
// the engine cannot play to <dir>/demand/unsupported.jsonl (-deck-demand-log).
// A line carries the UTC day, the engine and those names, nothing else: no
// unknown name (free text a person typed), no deck, count, address or
// identity. Its clock only stamps a log line; it never reaches a game.
type demandLog struct {
	path string
	max  int64
	now  func() time.Time
	mu   sync.Mutex
	// warned is set once a write failed, so a broken disk logs one line.
	warned bool
}

func newDemandLog(dir string) *demandLog {
	return &demandLog{path: filepath.Join(dir, "demand", "unsupported.jsonl"), max: demandLogMax, now: time.Now}
}

// demandLog is the -deck-demand-log sink, nil when the flag is off.
func (c config) demandLog() *demandLog {
	if !c.deckDemandLog {
		return nil
	}
	return newDemandLog(c.dir)
}

type demandLine struct {
	Day         string   `json:"day"`
	Engine      string   `json:"engine"`
	Unsupported []string `json:"unsupported"`
}

// record appends one line per verdict in rep: gorge's unsupported names
// under rep.Engine, then, when rep carries a dirac verdict, dirac's under its
// engine, so each line names the engine whose support produced it. A nil log
// records nothing; a write error is logged once and never reaches the request.
func (l *demandLog) record(rep protocol.DeckReport) {
	if l == nil {
		return
	}
	day := l.now().UTC().Format("2006-01-02")
	names := make([]string, 0, len(rep.Unsupported))
	for _, u := range rep.Unsupported {
		names = append(names, u.Name)
	}
	lines := []demandLine{{Day: day, Engine: rep.Engine, Unsupported: names}}
	if rep.Dirac != nil {
		names := make([]string, 0, len(rep.Dirac.Unsupported))
		for _, u := range rep.Dirac.Unsupported {
			names = append(names, u.Name)
		}
		lines = append(lines, demandLine{Day: day, Engine: rep.Dirac.Engine, Unsupported: names})
	}
	var buf []byte
	for _, dl := range lines {
		sort.Strings(dl.Unsupported)
		line, err := json.Marshal(dl)
		if err != nil {
			l.fail(err)
			return
		}
		buf = append(append(buf, line...), '\n')
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if err := l.append(buf); err != nil {
		l.fail(err)
	}
}

// append writes line, first rotating a file already at max to path.1.
// Called with mu held.
func (l *demandLog) append(line []byte) error {
	if err := os.MkdirAll(filepath.Dir(l.path), 0o755); err != nil {
		return err
	}
	if st, err := os.Stat(l.path); err == nil && st.Size() >= l.max {
		if err := os.Rename(l.path, l.path+".1"); err != nil {
			return err
		}
	}
	f, err := os.OpenFile(l.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	_, werr := f.Write(line)
	if cerr := f.Close(); werr == nil {
		werr = cerr
	}
	return werr
}

func (l *demandLog) fail(err error) {
	if l.warned {
		return
	}
	l.warned = true
	fmt.Fprintf(os.Stderr, "gorged: deck demand log: %v (further write errors are not logged)\n", err)
}
