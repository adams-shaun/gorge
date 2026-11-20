package oracle

import "testing"

// TestReproManifest replays every recording that says "named": true, one parallel subtest each,
// named by the recording's file name. It is the one test a recording needs when no glob test
// replays its pool: add the JSON, not a Go function (docs/agents/do-not.md).
//
// A recording whose pool a glob test replays is never marked named (the glob replays it, and a
// second replay is a double); the manifest fails such a mark. A recording marked "served" is
// replayed by stdReproNamedServed, which also requires the whole recorded prefix to be served;
// the glob tests honour the same mark (replayRecorded), so a globbed recording keeps that
// check.
func TestReproManifest(t *testing.T) {
	t.Parallel()
	globbed := globbedPools(t)
	named := 0
	for _, path := range reproFiles(t) {
		r, err := ReadRepro(path)
		if err != nil {
			t.Fatal(err)
		}
		if !r.Named {
			continue
		}
		name := reproName(path)
		if globbed[r.Pool] {
			t.Errorf("%s: marked named, but a glob test replays pool %q: drop \"named\" (it plays twice)", name, r.Pool)
			continue
		}
		named++
		t.Run(name, func(t *testing.T) {
			if landGrantRecordingReplay(t, name) {
				return
			}
			if r.Served {
				stdReproNamedServed(t, name)
			} else {
				stdReproNamed(t, name)
			}
		})
	}
	if named < reproManifestFloor {
		t.Errorf("%d named recordings, want at least %d: a recording lost its mark", named, reproManifestFloor)
	}
}

// reproManifestFloor is the number of recordings marked named when the manifest replaced their
// one-line Go wrappers. A recording may be added; losing one would drop its replay unseen.
const reproManifestFloor = 595
