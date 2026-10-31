package gorged

import (
	"flag"
	"fmt"
	"io"
)

// ServeFlagSet returns a fresh copy of the flag set Main parses, set to
// ContinueOnError with no output, so a caller outside this module can check
// an argument list against the real flags and read back the parsed values by
// name (fs.Lookup(name).Value): a deployment's test can prove that what its
// script passes is accepted by the server it starts.
func ServeFlagSet() *flag.FlagSet { return ServeFlagSetFor(Extensions{}) }

// ServeFlagSetFor is ServeFlagSet for a binary that links ext: its flag set
// carries the extensions' own flags (Extensions.Flags) too.
func ServeFlagSetFor(ext Extensions) *flag.FlagSet {
	fs, _ := flagsFor(ext)
	fs.Init(fs.Name(), flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	return fs
}

// flagsFor is serveFlags with ext linked in: the config carries ext and the
// flag set the extensions' own flags.
func flagsFor(ext Extensions) (*flag.FlagSet, *config) {
	fs, c := serveFlags()
	c.ext = ext
	if ext.Flags != nil {
		ext.Flags(fs)
	}
	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "Usage of %s:\n", fs.Name())
		fs.PrintDefaults()
		printBotOptionHelp(fs.Output())
	}
	return fs, c
}
