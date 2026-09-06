package cards_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/cards"
)

// A Forge card script is identifiable by content, not path: it begins with a
// Name: line and carries an Oracle: line. Checking content rather than
// directory means the guard survives someone vendoring the corpus somewhere
// unexpected.
func looksLikeForgeScript(blob string) bool {
	return strings.HasPrefix(blob, "Name:") && strings.Contains(blob, "\nOracle:")
}

// gitCmd builds a git command running with cards.GitEnv()'s environment:
// os.Environ() with every inherited GIT_* variable stripped. These tests
// init, add and commit throwaway repositories; a hook or wrapper that
// exported GIT_DIR or GIT_INDEX_FILE would otherwise redirect those writes
// into whatever repository the test runs under (the gitiso bug).
func gitCmd(args ...string) *exec.Cmd {
	cmd := exec.Command("git", args...)
	cmd.Env = cards.GitEnv()
	return cmd
}

// checkForgeScriptsInRepo inspects a git repository at repoDir for Forge card
// scripts in .txt files, checking the working tree first, then the staged index,
// then HEAD. This ordering catches scripts that are staged before commit,
// committed but then modified, and merely committed.
// Returns (offenders, error) where error is non-nil only if git itself is unavailable.
func checkForgeScriptsInRepo(t *testing.T, repoDir string) ([]string, error) {
	t.Helper()

	out, err := gitCmd("-C", repoDir, "ls-files", "-z", "--", "*.txt").Output()
	if err != nil {
		return nil, err
	}

	var offenders []string
	for _, path := range strings.Split(strings.TrimRight(string(out), "\x00"), "\x00") {
		if path == "" {
			continue
		}

		fullPath := filepath.Join(repoDir, path)
		var blob []byte

		// 1. Try working-tree file first
		if data, err := os.ReadFile(fullPath); err == nil {
			blob = data
		} else {
			// 2. Try staged blob in index
			if data, err := gitCmd("-C", repoDir, "show", ":"+path).Output(); err == nil {
				blob = data
			} else {
				// 3. Try HEAD
				if data, err := gitCmd("-C", repoDir, "show", "HEAD:"+path).Output(); err == nil {
					blob = data
				}
				// If none exist, skip this path
				if blob == nil {
					continue
				}
			}
		}

		if looksLikeForgeScript(string(blob)) {
			offenders = append(offenders, path)
		}
	}
	return offenders, nil
}

// repoRoot resolves the root of the git repository containing the current
// working directory. Test binaries run with cwd set to the package
// directory, so this always finds this repo's root regardless of how deep
// the package lives — unlike a hard-coded relative path (e.g. "../.."),
// which silently drifts, and drifted outside the repo entirely, when this
// package was transplanted to a new module root.
func repoRoot(t *testing.T) string {
	t.Helper()

	out, err := gitCmd("-C", ".", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		t.Fatalf("could not resolve git repo root from working directory: %v", err)
	}
	return strings.TrimSpace(string(out))
}

// TestNoForgeScriptsTracked enforces the GPL boundary from the design spec:
// gorge ships a compiler, never the scripts.
func TestNoForgeScriptsTracked(t *testing.T) {
	root := repoRoot(t)

	offenders, err := checkForgeScriptsInRepo(t, root)
	if err != nil {
		t.Fatalf("git unavailable while scanning %s: %v", root, err)
	}
	if len(offenders) > 0 {
		t.Fatalf("Forge card scripts are tracked in git, which breaks the GPL boundary:\n  %s",
			strings.Join(offenders, "\n  "))
	}
}

// TestForgeScriptDetection verifies the guard catches Forge scripts in all states.
func TestForgeScriptDetection(t *testing.T) {
	forgeSignature := "Name: Test Card\nType: Creature\nOracle: Draw a card.\nPower/Toughness: 1/1"
	innocentText := "This is just a normal text file\nwith multiple lines\nbut no Forge signature"

	tests := []struct {
		name           string
		setup          func(t *testing.T, repoDir string)
		expectOffender bool
		reason         string
	}{
		{
			name: "Forge script staged but not committed",
			setup: func(t *testing.T, repoDir string) {
				path := filepath.Join(repoDir, "card.txt")
				if err := os.WriteFile(path, []byte(forgeSignature), 0644); err != nil {
					t.Fatalf("write failed: %v", err)
				}
				if err := gitCmd("-C", repoDir, "add", "card.txt").Run(); err != nil {
					t.Fatalf("git add failed: %v", err)
				}
			},
			expectOffender: true,
			reason:         "guard must catch scripts in staging area before commit",
		},
		{
			name: "Forge script committed",
			setup: func(t *testing.T, repoDir string) {
				path := filepath.Join(repoDir, "card.txt")
				if err := os.WriteFile(path, []byte(forgeSignature), 0644); err != nil {
					t.Fatalf("write failed: %v", err)
				}
				if err := gitCmd("-C", repoDir, "add", "card.txt").Run(); err != nil {
					t.Fatalf("git add failed: %v", err)
				}
				if err := gitCmd("-C", repoDir, "-c", "user.email=test@test.com", "-c", "user.name=Test",
					"commit", "-m", "test").Run(); err != nil {
					t.Fatalf("git commit failed: %v", err)
				}
			},
			expectOffender: true,
			reason:         "guard must catch committed scripts",
		},
		{
			name: "Innocuous text file committed",
			setup: func(t *testing.T, repoDir string) {
				path := filepath.Join(repoDir, "readme.txt")
				if err := os.WriteFile(path, []byte(innocentText), 0644); err != nil {
					t.Fatalf("write failed: %v", err)
				}
				if err := gitCmd("-C", repoDir, "add", "readme.txt").Run(); err != nil {
					t.Fatalf("git add failed: %v", err)
				}
				if err := gitCmd("-C", repoDir, "-c", "user.email=test@test.com", "-c", "user.name=Test",
					"commit", "-m", "test").Run(); err != nil {
					t.Fatalf("git commit failed: %v", err)
				}
			},
			expectOffender: false,
			reason:         "guard must not flag files without Name:/Oracle: signature",
		},
		{
			name: "Committed Forge script deleted from working tree",
			setup: func(t *testing.T, repoDir string) {
				path := filepath.Join(repoDir, "card.txt")
				if err := os.WriteFile(path, []byte(forgeSignature), 0644); err != nil {
					t.Fatalf("write failed: %v", err)
				}
				if err := gitCmd("-C", repoDir, "add", "card.txt").Run(); err != nil {
					t.Fatalf("git add failed: %v", err)
				}
				if err := gitCmd("-C", repoDir, "-c", "user.email=test@test.com", "-c", "user.name=Test",
					"commit", "-m", "test").Run(); err != nil {
					t.Fatalf("git commit failed: %v", err)
				}
				// Delete from working tree but leave in HEAD
				if err := os.Remove(path); err != nil {
					t.Fatalf("remove failed: %v", err)
				}
			},
			expectOffender: true,
			reason:         "guard checks HEAD as fallback when working tree absent; the script is still committed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()

			// Initialize a git repo with an initial commit
			if err := gitCmd("-C", tmpDir, "init").Run(); err != nil {
				t.Fatalf("git init failed: %v", err)
			}
			if err := gitCmd("-C", tmpDir, "-c", "user.email=test@test.com", "-c", "user.name=Test",
				"commit", "--allow-empty", "-m", "initial").Run(); err != nil {
				t.Fatalf("initial commit failed: %v", err)
			}

			tt.setup(t, tmpDir)
			offenders, err := checkForgeScriptsInRepo(t, tmpDir)
			if err != nil {
				t.Fatalf("git should be available in temp repo: %v", err)
			}

			if tt.expectOffender && len(offenders) == 0 {
				t.Errorf("expected to detect Forge script, but got none (reason: %s)", tt.reason)
			}
			if !tt.expectOffender && len(offenders) > 0 {
				t.Errorf("expected no detection, but got offenders: %v (reason: %s)", offenders, tt.reason)
			}
		})
	}
}
