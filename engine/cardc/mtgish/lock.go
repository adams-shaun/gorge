package mtgish

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Lock is engine/cardc/MTGISH.lock: the pin of the mtgish data cardc reads.
type Lock struct {
	Repo, Upstream, Base, Ref string
	Files                     []LockFile
	Patches                   []LockPatch
}

// LockFile is one `sha256 <sum> <path>` line.
type LockFile struct{ Sum, Path string }

// LockPatch is one `patch <sha> <upstream-issue>` line.
type LockPatch struct {
	SHA   string
	Issue int
}

// ReadLock parses the lock file at path.
func ReadLock(path string) (Lock, error) {
	f, err := os.Open(path)
	if err != nil {
		return Lock{}, err
	}
	defer f.Close()
	return parseLock(bufio.NewScanner(f), path)
}

func parseLock(sc *bufio.Scanner, name string) (Lock, error) {
	var l Lock
	n := 0
	for sc.Scan() {
		n++
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		f := strings.Fields(line)
		bad := func(why string) error { return fmt.Errorf("%s:%d: %s", name, n, why) }
		switch f[0] {
		case "repo", "upstream", "base", "ref":
			if len(f) != 2 {
				return l, bad(f[0] + " takes one value")
			}
			switch f[0] {
			case "repo":
				l.Repo = f[1]
			case "upstream":
				l.Upstream = f[1]
			case "base":
				l.Base = f[1]
			default:
				l.Ref = f[1]
			}
		case "sha256":
			if len(f) != 3 {
				return l, bad("sha256 takes a sum and a path")
			}
			l.Files = append(l.Files, LockFile{Sum: f[1], Path: f[2]})
		case "patch":
			if len(f) != 3 {
				return l, bad("patch takes a sha and an issue number")
			}
			issue, err := strconv.Atoi(f[2])
			if err != nil {
				return l, bad("patch issue: " + err.Error())
			}
			l.Patches = append(l.Patches, LockPatch{SHA: f[1], Issue: issue})
		default:
			return l, bad("unknown line " + strconv.Quote(f[0]))
		}
	}
	return l, sc.Err()
}
