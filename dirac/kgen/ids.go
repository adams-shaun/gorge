package kgen

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// idsFile is the checked-in CardID index of the pool table (cardset.Set.Pool):
// one line per CardID ever assigned, "<id>\t<card|token>\t<name>". A pool
// card or token keeps its CardID across regenerations however the pool
// changes; a new one takes the next free CardID (cards before tokens, each
// sorted by name); one that left the pool keeps its CardID as an absent,
// inert placeholder. The first assignment is the pool order itself
// (cards sorted by name, then tokens), so CardID == pool index until the
// pool changes.
const idsFile = "ids.tsv"

type idEntry struct {
	id    int
	token bool
	name  string
}

func idKey(token bool, name string) string {
	if token {
		return "token\t" + name
	}
	return "card\t" + name
}

// readIDs reads an index; a missing file is an empty index.
func readIDs(path string) ([]idEntry, error) {
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return parseIDs(b, path)
}

// parseIDs parses an index's bytes; path names it in errors.
func parseIDs(b []byte, path string) ([]idEntry, error) {
	var out []idEntry
	sc := bufio.NewScanner(bytes.NewReader(b))
	for n := 1; sc.Scan(); n++ {
		line := sc.Text()
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		f := strings.SplitN(line, "\t", 3)
		if len(f) != 3 || (f[1] != "card" && f[1] != "token") {
			return nil, fmt.Errorf("%s:%d: malformed line %q", path, n, line)
		}
		id, err := strconv.Atoi(f[0])
		if err != nil || id != len(out) {
			return nil, fmt.Errorf("%s:%d: id %q out of sequence", path, n, f[0])
		}
		out = append(out, idEntry{id: id, token: f[1] == "token", name: f[2]})
	}
	return out, sc.Err()
}

// assignIDs gives every pool entry (all: the cards, then the tokens) its
// CardID under the index prev, and returns the CardIDs per pool index and
// the extended index.
func assignIDs(prev []idEntry, all []*lCard) ([]int, []idEntry, error) {
	byKey := map[string]int{}
	for _, e := range prev {
		byKey[idKey(e.token, e.name)] = e.id
	}
	idx := append([]idEntry(nil), prev...)
	ids := make([]int, len(all))
	var fresh []int
	for i, c := range all {
		if id, ok := byKey[idKey(c.Token, c.Name)]; ok {
			ids[i] = id
			continue
		}
		fresh = append(fresh, i)
	}
	sort.SliceStable(fresh, func(a, b int) bool {
		ca, cb := all[fresh[a]], all[fresh[b]]
		if ca.Token != cb.Token {
			return !ca.Token
		}
		return ca.Name < cb.Name
	})
	for _, i := range fresh {
		ids[i] = len(idx)
		idx = append(idx, idEntry{id: len(idx), token: all[i].Token, name: all[i].Name})
	}
	if len(idx) >= int(kop.NoCard) {
		return nil, nil, fmt.Errorf("%d CardIDs exceed kop.NoCard", len(idx))
	}
	return ids, idx, nil
}

func idsBytes(idx []idEntry) []byte {
	var b bytes.Buffer
	b.WriteString("# CardID index of the pool table, maintained by dirac/kgen: an id never moves.\n")
	for _, e := range idx {
		k := "card"
		if e.token {
			k = "token"
		}
		fmt.Fprintf(&b, "%d\t%s\t%s\n", e.id, k, e.name)
	}
	return b.Bytes()
}
