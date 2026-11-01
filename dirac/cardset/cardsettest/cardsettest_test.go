package cardsettest

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge"
	"github.com/adams-shaun/gorge/dirac/cardset"
)

const digest = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

// outcome is how a call on a recording TB ended.
type outcome struct {
	skipped, failed bool
	msg             string
}

// recorder stops the goroutine the way Skipf and Fatalf do, and records which ran.
type recorder struct {
	testing.TB
	o outcome
}

func (r *recorder) Helper() {}
func (r *recorder) Skipf(f string, a ...any) {
	r.o = outcome{skipped: true, msg: fmtf(f, a...)}
	panic(r)
}
func (r *recorder) Fatalf(f string, a ...any) {
	r.o = outcome{failed: true, msg: fmtf(f, a...)}
	panic(r)
}

func fmtf(f string, a ...any) string {
	var b strings.Builder
	b.WriteString(f)
	for _, x := range a {
		if e, ok := x.(error); ok {
			b.WriteString(" " + e.Error())
		}
	}
	return b.String()
}

// run calls f on a recorder and returns how it ended.
func run(f func(testing.TB)) (o outcome) {
	r := &recorder{}
	defer func() {
		if x := recover(); x != nil && x != any(r) {
			panic(x)
		}
		o = r.o
	}()
	f(r)
	return r.o
}

// corpus sets up a corpus with a cards.lock, tables written by write if it is non-nil.
func corpus(t *testing.T, write func(dir, key string)) {
	t.Helper()
	c := t.TempDir()
	if err := os.WriteFile(filepath.Join(c, "cards.lock"), []byte(`{"digest":"`+digest+`"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GORGE_CARDS", c)
	t.Setenv("DIRAC_TABLES", "")
	if write != nil {
		key := digest[:16] + "-" + gorge.TableSourceHash()[:16]
		dir := filepath.Join(c, cardset.TablesSubdir, key)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		write(dir, key)
	}
}

func writePool(t *testing.T, dir, key, pool string, damage bool) {
	t.Helper()
	b := cardset.EncodeTable(&cardset.PoolData{Name: pool}, key)
	sum, err := cardset.TableSum(b)
	if err != nil {
		t.Fatal(err)
	}
	if damage {
		b[len(b)-1] ^= 0xFF
	}
	if err := os.WriteFile(filepath.Join(dir, pool+".ktab"), b, 0o644); err != nil {
		t.Fatal(err)
	}
	m := cardset.EncodeManifest([]cardset.ManifestRow{{Pool: pool, Sum: sum}})
	if err := os.WriteFile(filepath.Join(dir, cardset.ManifestFile), m, 0o644); err != nil {
		t.Fatal(err)
	}
}

// Each row of the door's contract, through both Set and Require: only no corpus skips.
func TestDoor(t *testing.T) {
	doors := map[string]func(testing.TB){
		"Set":     func(tb testing.TB) { Set(tb, "p") },
		"Require": func(tb testing.TB) { Require(tb, "p") },
	}
	for _, door := range []string{"Set", "Require"} {
		f := doors[door]
		t.Run(door, func(t *testing.T) {
			t.Run("no corpus skips", func(t *testing.T) {
				t.Setenv("GORGE_CARDS", "")
				t.Setenv("DIRAC_TABLES", "")
				t.Chdir(t.TempDir())
				if _, err := cardset.CorpusDir(); !errors.Is(err, cardset.ErrNoCorpus) {
					t.Skipf("the temp directory has a corpus above it: %v", err)
				}
				if o := run(f); !o.skipped || o.failed {
					t.Fatalf("%+v, want a skip", o)
				}
			})
			t.Run("no tables fails naming the command", func(t *testing.T) {
				corpus(t, nil)
				o := run(f)
				if !o.failed || o.skipped || !strings.Contains(o.msg, "make dirac-tables") {
					t.Fatalf("%+v, want a failure naming make dirac-tables", o)
				}
			})
			t.Run("damaged table fails", func(t *testing.T) {
				corpus(t, func(dir, key string) { writePool(t, dir, key, "p", true) })
				if o := run(f); !o.failed || o.skipped {
					t.Fatalf("%+v, want a failure", o)
				}
			})
			t.Run("unknown pool fails", func(t *testing.T) {
				corpus(t, func(dir, key string) { writePool(t, dir, key, "other", false) })
				if o := run(f); !o.failed || o.skipped {
					t.Fatalf("%+v, want a failure", o)
				}
			})
			t.Run("present passes", func(t *testing.T) {
				corpus(t, func(dir, key string) { writePool(t, dir, key, "p", false) })
				if o := run(f); o.failed || o.skipped {
					t.Fatalf("%+v, want no failure", o)
				}
			})
		})
	}
}
