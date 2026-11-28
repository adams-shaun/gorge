package gorged

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/adams-shaun/gorge/server/host"
	"github.com/adams-shaun/gorge/server/protocol"
)

// healthz answers 200 "ok" as soon as the listener serves. It reads nothing.
func healthz(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte("ok\n"))
}

// metricsHandler serves the Prometheus text exposition (-metrics): counts
// only, read from the registry, in a fixed series order.
func metricsHandler(r *host.Registry) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		byState := map[string]int{}
		for _, ti := range r.Tables() {
			byState[ti.State]++
		}
		var b strings.Builder
		b.WriteString("# HELP gorged_tables Tables by state.\n# TYPE gorged_tables gauge\n")
		for _, s := range []string{protocol.TableLive, protocol.TableCooldown, protocol.TableIdle, protocol.TableHalted} {
			fmt.Fprintf(&b, "gorged_tables{state=%q} %d\n", s, byState[s])
		}
		gauge := func(name, help string, v int) {
			fmt.Fprintf(&b, "# HELP %s %s\n# TYPE %s gauge\n%s %d\n", name, help, name, name, v)
		}
		gauge("gorged_on_demand_tables", "Tables created by POST /api/games that are still registered.", r.OnDemandTables())
		gauge("gorged_matches_stored", "Matches listed across all tables.", len(r.AllMatches()))
		gauge("gorged_search_slots_busy", "Searched decisions holding a search slot now.", r.SearchSlotsBusy())
		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		_, _ = w.Write([]byte(b.String()))
	}
}
