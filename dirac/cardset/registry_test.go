package cardset

import (
	"errors"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// ByName and Names serve a key directory's tables.
func TestRegistry(t *testing.T) {
	isolate(t)
	c := makeCorpus(t, t.TempDir())
	t.Setenv("GORGE_CARDS", c)
	writeTables(t, filepath.Join(c, TablesSubdir, testKey()), testKey(), "ztest-alpha", "ztest-beta")

	s, err := ByName("ztest-alpha")
	if err != nil {
		t.Fatalf("ByName(ztest-alpha): %v", err)
	}
	if s.Name() != "ztest-alpha" {
		t.Fatalf("ByName served %q", s.Name())
	}
	if i, ok := s.Lookup("B Card"); !ok || i != 1 {
		t.Fatalf("Lookup through ByName: %d, %v", i, ok)
	}
	if _, ok := s.Lookup("No Card"); ok {
		t.Fatal("Lookup found an absent card")
	}

	_, err = ByName("ztest-missing")
	if err == nil {
		t.Fatal("ByName(unknown) returned no error")
	}
	for _, want := range []string{"ztest-missing", "ztest-alpha", "ztest-beta"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not name %q", err, want)
		}
	}

	if names := Names(); !slices.Equal(names, []string{"ztest-alpha", "ztest-beta"}) {
		t.Errorf("Names: %v", names)
	}
}

// Names has no error to return: tables it cannot read are nil, not a partial list.
func TestNamesWithoutTables(t *testing.T) {
	isolate(t)
	if names := Names(); names != nil {
		t.Errorf("no corpus: %v", names)
	}
	t.Setenv("GORGE_CARDS", makeCorpus(t, t.TempDir()))
	if names := Names(); names != nil {
		t.Errorf("no tables: %v", names)
	}
}

// MustByName panics with ByName's error.
func TestMustByName(t *testing.T) {
	isolate(t)
	defer func() {
		if err, _ := recover().(error); !errors.Is(err, ErrNoCorpus) {
			t.Fatalf("recovered %v, want ErrNoCorpus", err)
		}
	}()
	MustByName("fdn")
	t.Fatal("MustByName did not panic")
}
