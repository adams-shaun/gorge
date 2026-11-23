package oracle

import (
	"hash/crc32"
	"testing"
)

// The manifest replays every recording that says "named": true, one parallel subtest each, named
// by the recording's file name. It is the one test a recording needs when no glob test replays its
// pool: add the JSON, not a Go function (docs/agents/do-not.md).
//
// A recording whose pool a glob test replays is never marked named (the glob replays it, and a
// second replay is a double); the manifest fails such a mark. A recording marked "served" is
// replayed by stdReproNamedServed, which also requires the whole recorded prefix to be served;
// the glob tests honour the same mark (replayRecorded), so a globbed recording keeps that
// check.
//
// It is reproManifestShards top-level tests, TestReproManifest0 to TestReproManifest7. A recording
// belongs to shard crc32(name) % reproManifestShards, so adding one moves no other. As one test the
// manifest was 595 recordings in 663 cost-s (slowrepros.txt's 11-20 anchor run), past the 1-minute
// budget on a loaded box (AGENTS.md: split, never trim). `-run 'TestReproManifest/<recording>'`
// still selects one recording: the unanchored prefix matches every shard, and only the shard that
// holds it has the subtest. Shard 0 also checks the marks and the floor.
const reproManifestShards = 8

// A change to reproManifestShards fails to compile until the TestReproManifestN functions match it,
// so no shard's recordings go unreplayed.
var _ = [1]struct{}{}[reproManifestShards-8]

func TestReproManifest0(t *testing.T) { reproManifestShard(t, 0) }
func TestReproManifest1(t *testing.T) { reproManifestShard(t, 1) }
func TestReproManifest2(t *testing.T) { reproManifestShard(t, 2) }
func TestReproManifest3(t *testing.T) { reproManifestShard(t, 3) }
func TestReproManifest4(t *testing.T) { reproManifestShard(t, 4) }
func TestReproManifest5(t *testing.T) { reproManifestShard(t, 5) }
func TestReproManifest6(t *testing.T) { reproManifestShard(t, 6) }
func TestReproManifest7(t *testing.T) { reproManifestShard(t, 7) }

// reproManifestShardOf is the shard that replays the named recording.
func reproManifestShardOf(name string) int {
	return int(crc32.ChecksumIEEE([]byte(name)) % reproManifestShards)
}

func reproManifestShard(t *testing.T, shard int) {
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
			if shard == 0 {
				t.Errorf("%s: marked named, but a glob test replays pool %q: drop \"named\" (it plays twice)", name, r.Pool)
			}
			continue
		}
		named++
		if reproManifestShardOf(name) != shard {
			continue
		}
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
	if shard == 0 && named < reproManifestFloor {
		t.Errorf("%d named recordings, want at least %d: a recording lost its mark", named, reproManifestFloor)
	}
}

// reproManifestFloor is the number of recordings marked named when the manifest replaced their
// one-line Go wrappers. A recording may be added; losing one would drop its replay unseen.
const reproManifestFloor = 595
