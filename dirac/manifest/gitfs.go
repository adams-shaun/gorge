package manifest

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"strings"
	"testing/fstest"
)

// git runs git in repo and returns its stdout.
func git(repo string, stdin []byte, args ...string) ([]byte, error) {
	cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
	if stdin != nil {
		cmd.Stdin = bytes.NewReader(stdin)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git %s: %v: %s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return out, nil
}

// MergeBase is the merge base of ref and HEAD in the git repository at repo:
// the base Check holds a branch to, so an entry the branch never saw is not
// one it deleted.
func MergeBase(repo, ref string) (string, error) {
	out, err := git(repo, nil, "merge-base", ref, "HEAD")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// ReadGit reads (Read) the manifest and ordinals.tsv committed at ref in the
// git repository whose top level is repo, the module root.
func ReadGit(repo, ref string) (*Tree, error) {
	out, err := git(repo, nil, "rev-parse", "--verify", "--quiet", ref+"^{commit}")
	if err != nil {
		return nil, fmt.Errorf("%s is not a commit: %w", ref, err)
	}
	commit := strings.TrimSpace(string(out))
	out, err = git(repo, nil, "ls-tree", "-r", "-z", "--full-tree", commit, "--", Dir, OrdinalsPath)
	if err != nil {
		return nil, err
	}
	var paths []string
	var ids bytes.Buffer
	for _, rec := range strings.Split(strings.TrimSuffix(string(out), "\x00"), "\x00") {
		if rec == "" {
			continue
		}
		meta, p, ok := strings.Cut(rec, "\t")
		f := strings.Fields(meta)
		if !ok || len(f) != 3 {
			return nil, fmt.Errorf("git ls-tree: %q", rec)
		}
		if f[1] != "blob" {
			continue
		}
		paths = append(paths, p)
		ids.WriteString(f[2] + "\n")
	}
	m := fstest.MapFS{}
	if len(paths) > 0 {
		out, err = git(repo, ids.Bytes(), "cat-file", "--batch")
		if err != nil {
			return nil, err
		}
		r := bufio.NewReader(bytes.NewReader(out))
		for _, p := range paths {
			head, err := r.ReadString('\n')
			if err != nil {
				return nil, fmt.Errorf("git cat-file %s: %v", p, err)
			}
			f := strings.Fields(head)
			if len(f) != 3 || f[1] != "blob" {
				return nil, fmt.Errorf("git cat-file %s: %q", p, head)
			}
			n, err := strconv.Atoi(f[2])
			if err != nil {
				return nil, fmt.Errorf("git cat-file %s: %q", p, head)
			}
			b := make([]byte, n+1) // the content and its trailing newline
			if _, err := io.ReadFull(r, b); err != nil {
				return nil, fmt.Errorf("git cat-file %s: %v", p, err)
			}
			m[p] = &fstest.MapFile{Data: b[:n]}
		}
	}
	t, err := Read(m)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", ref, err)
	}
	return t, nil
}
