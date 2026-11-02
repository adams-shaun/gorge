package main

import "github.com/adams-shaun/gorge/dirac/oracle"

// v2TranscriptOpts is one -v2 transcript run (implemented in v2transcript.go).
type v2TranscriptOpts struct {
	Corpus       *oracle.Corpus
	PoolPath     string // pool.tsv
	Split        string // train, eval or all
	DeckDir      string // the pool's decks directory
	FirstGame    int    // first pool pair index (the repro anchor)
	Games        int    // clean games per answerer
	Seed         uint64 // base seed; pair p's decks and game seed are PairingAt(pool, p, seed)
	Answerers    []string
	Workers      int
	MaxDecisions int
	Quiet        bool
	Verbose      bool
	// NoChance is the negative control: the gorge host keeps gamecfg's
	// secret-stream shuffles instead of dealing the dirac kernel's chance. An
	// unfollowed run must diff; a clean one would mean the chance following
	// (and the note the summary prints) no longer describes the result.
	NoChance bool
	// RandFull is accepted for the old spelling and does nothing: a rand game
	// is always compared whole now that the gorge host follows the kernel's
	// chance (it used to stop at the first independent shuffle).
	RandFull bool
}
