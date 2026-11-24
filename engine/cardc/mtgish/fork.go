package mtgish

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
)

// ForkPatch is one row of engine/cardc/testdata/mtgish-fork.tsv, the register of
// patches the mtgish fork carries.
type ForkPatch struct {
	SHA    string
	Issue  int
	Class  string
	Status string
	Cards  []string
}

const forkHeader = "sha\tupstream_issue\tclass\tstatus\tcards"

var (
	forkSHARe   = regexp.MustCompile(`^[0-9a-f]{40}$`)
	forkClassRe = regexp.MustCompile(`^[a-z]+(-[a-z]+)*$`)
)

// ReadForkRegister parses the fork register at path. The header row is required;
// a bad row is an error naming its line.
func ReadForkRegister(path string) ([]ForkPatch, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out []ForkPatch
	sc := bufio.NewScanner(f)
	n, header := 0, false
	for sc.Scan() {
		n++
		line := strings.TrimRight(sc.Text(), "\r")
		if strings.TrimSpace(line) == "" || strings.HasPrefix(line, "#") {
			continue
		}
		bad := func(why string) error { return fmt.Errorf("%s:%d: %s", path, n, why) }
		if !header {
			if line != forkHeader {
				return nil, bad("header row must be " + strconv.Quote(forkHeader))
			}
			header = true
			continue
		}
		c := strings.Split(line, "\t")
		if len(c) != 5 {
			return nil, bad(fmt.Sprintf("want 5 tab-separated columns, got %d", len(c)))
		}
		p := ForkPatch{SHA: c[0], Class: c[2], Status: c[3]}
		if !forkSHARe.MatchString(p.SHA) {
			return nil, bad("sha must be 40 lower-case hex digits")
		}
		issue, err := strconv.Atoi(c[1])
		if err != nil || issue <= 0 {
			return nil, bad("upstream_issue must be a number above 0")
		}
		p.Issue = issue
		if !forkClassRe.MatchString(p.Class) {
			return nil, bad("class must be a lower-case hyphenated token")
		}
		switch p.Status {
		case "carried", "fixed-upstream", "dropped":
		default:
			return nil, bad("status must be carried, fixed-upstream or dropped")
		}
		if c[4] == "" {
			return nil, bad("cards must be comma-separated names, or -")
		}
		if c[4] != "-" {
			for _, name := range strings.Split(c[4], ",") {
				name = strings.TrimSpace(name)
				if name == "" {
					return nil, bad("cards has an empty name")
				}
				p.Cards = append(p.Cards, name)
			}
		}
		out = append(out, p)
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	if !header {
		return nil, fmt.Errorf("%s: header row missing", path)
	}
	return out, nil
}
