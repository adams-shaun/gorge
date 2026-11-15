package oracle

import "testing"

// Torrent Elemental's "{3}{B/G}{B/G}: Put it onto the battlefield tapped" is
// activated from exile (CR 113.6k-style zone-restricted ability). The recording
// is the first divergence of tail-54-c001 pair 6 (autopay, mulligans 0): at
// decision 264 gorge offered activate(Torrent Elemental) at priority and the
// kernel did not.
func TestMBCTorrentElementalReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "torrent_elemental_exile_activation")
}
