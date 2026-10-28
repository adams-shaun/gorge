package gorged

import (
	"fmt"
	"net/http"
	"os"

	"github.com/adams-shaun/gorge/server/host"
	"github.com/adams-shaun/gorge/server/host/sbv2http"
)

// sbv2RoutePrefix is where the SpellBench v2 human-seat routes are mounted on
// the main listener's topMux, ahead of httpapi.NewHandler the same way
// /api/manabrew/v0/tables and /art/ are. With -sbv2 off nothing is registered
// under this prefix, so a request here falls through to httpapi's own "/"
// catch-all 404 exactly as ManaBrew does. TestSBV2OffMountsNothing pins it.
const sbv2RoutePrefix = "/api/sbv2/v1/tables"

// applySBV2 arms the registry's human-seat tap factory when -sbv2 is on and
// returns the hub the HTTP routes share. It must run before host.New, because
// that call copies Options; the hub itself needs no registry, so it can be
// built here while the registry that will use it is still being constructed.
// Off returns nil and leaves Options untouched, so an unmounted server is
// byte-identical to one built before this file existed.
func (c config) applySBV2(o *host.Options) *sbv2http.Hub {
	if !c.sbv2 {
		return nil
	}
	hub := sbv2http.NewHub()
	o.TapHuman = hub.Tap
	return hub
}

// mountSBV2 wires server/host/sbv2http onto topMux. It is a no-op when -sbv2
// is off. gate is non-nil whenever c.sbv2 is true -- serve's startup check
// refuses -sbv2 without -humans or -vsbot -- so the handler never sees a nil
// Options.Seat that would 403 every request.
func (c config) mountSBV2(topMux *http.ServeMux, r *host.Registry, gate *seatGate, hub *sbv2http.Hub) error {
	if !c.sbv2 {
		return nil
	}
	if gate == nil || hub == nil {
		return fmt.Errorf("-sbv2 requires a seat gate: pass -humans or -vsbot")
	}
	handler := sbv2http.NewHandler(r, hub, sbv2http.Options{Seat: gate.resolve})
	topMux.Handle(sbv2RoutePrefix+"/", http.StripPrefix(sbv2RoutePrefix, handler))
	fmt.Fprintf(os.Stderr, "gorged: sbv2 routes mounted at %s/\n", sbv2RoutePrefix)
	return nil
}
