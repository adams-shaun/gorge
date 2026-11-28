package rules

import "testing"

// longGameVerifiersOff turns off, until t ends, the four cache verifiers that
// cost the most over a long game (derivedmemo_verify_test.go and its siblings
// switch every verifier on for the whole test binary): each recomputes what a
// cache served, on every hit. The others stay on. The flags are process-wide,
// so t must be a sequential top-level test: one that never calls t.Parallel
// runs while every parallel test is still paused, so no other test reads a
// flag while it is off.
func longGameVerifiersOff(t *testing.T) {
	t.Helper()
	for _, flag := range []*bool{&derivedMemoVerify, &walkSkipVerify, &specDerivedVerify, &faceScanVerify} {
		prev := *flag
		*flag = false
		t.Cleanup(func() { *flag = prev })
	}
}

// kernelWorldsInLongGame is kernelWorldsInFuzzGame for the game too long to
// play under every verifier: sequential, so it may turn the costly ones off.
// Games 0-4 and the rest of this binary still play with them on.
func kernelWorldsInLongGame(t *testing.T, g, shards, shard int) {
	longGameVerifiersOff(t)
	kernelWorldsPlayGame(t, g, shards, shard)
}

// The helper's two promises: the four verifiers are off inside the test that
// asked, and back on for every test after it.
func TestLongGameVerifiersAreOffOnlyInTheirTest(t *testing.T) {
	flags := map[string]*bool{
		"derivedMemoVerify": &derivedMemoVerify, "walkSkipVerify": &walkSkipVerify,
		"specDerivedVerify": &specDerivedVerify, "faceScanVerify": &faceScanVerify,
	}
	on := func(want bool, when string) {
		for _, name := range []string{"derivedMemoVerify", "walkSkipVerify", "specDerivedVerify", "faceScanVerify"} {
			if *flags[name] != want {
				t.Errorf("%s: %s = %v, want %v", when, name, *flags[name], want)
			}
		}
	}
	on(true, "precondition: the rules test binary runs the verifiers")
	t.Run("long game", func(t *testing.T) {
		longGameVerifiersOff(t)
		on(false, "inside the long game")
		if !walkCacheVerify || !prioMemoVerify {
			t.Errorf("walkCacheVerify = %v, prioMemoVerify = %v: only the four costly verifiers go off", walkCacheVerify, prioMemoVerify)
		}
	})
	on(true, "after the long game")
}
