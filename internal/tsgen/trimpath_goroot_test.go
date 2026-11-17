package tsgen

import (
	"go/build"
	"testing"
)

// TestNewLoaderRecoversGOROOTUnderTrimpath holds NewLoader to working when the
// binary was built with -trimpath: runtime.GOROOT() is then "", so
// build.Default.GOROOT is "" and the source importer has no stdlib to resolve.
func TestNewLoaderRecoversGOROOTUnderTrimpath(t *testing.T) {
	old := build.Default.GOROOT
	build.Default.GOROOT = ""
	t.Cleanup(func() { build.Default.GOROOT = old })
	if build.Default.GOROOT != "" {
		t.Fatal("precondition: build.Default.GOROOT is not empty")
	}

	l := NewLoader()
	if _, err := l.Lookup("github.com/adams-shaun/gorge/server/protocol", "Frame"); err != nil {
		t.Fatalf("NewLoader cannot load protocol with an empty build.Default.GOROOT: %v", err)
	}
}
