package cards

import (
	"os"
	"strings"
)

// gitTerminalPromptOff pins credential prompts off for every child git
// process this package starts. A fetch that needs credentials it does not
// have must fail with an error, not block a non-interactive build on a
// prompt it cannot answer.
const gitTerminalPromptOff = "GIT_TERMINAL_PROMPT=0"

// GitEnv returns the environment a child git process started by this package
// (or by its tests) must run with: os.Environ() with every inherited GIT_*
// variable removed, plus GIT_TERMINAL_PROMPT=0.
//
// fetchRepo builds git invocations that always name their repository
// explicitly (`git init <dir>` / `git -C <dir> ...`), so no inherited GIT_*
// variable can mean what its name suggests: GIT_DIR, GIT_INDEX_FILE and
// GIT_WORK_TREE all override -C and silently redirect a child at whatever
// repository the caller had checked out or staged. A cards.Fetch that runs
// under a git hook, from `go generate` under git, or from any wrapper that
// exports GIT_* would otherwise rewrite that caller's repository instead of
// the throwaway `work` directory it created: this shipped bug once replaced
// an enclosing repository's index with 34,542 Forge card paths.
//
// Transport and credential variables (GIT_SSH_COMMAND, GIT_SSL_*,
// GIT_PROXY_COMMAND, GIT_ASKPASS, ...) are stripped with the rest. They
// configure HOW to reach a network endpoint, which fetchRepo's explicit
// repository addressing never needs, and a caller configures them as config
// (http.proxy / http.ssl* in .gitconfig, ~/.ssh/config, curl's http_proxy
// environment) far more often than as GIT_* variables; keeping a transport
// allowlist would reintroduce exactly the class of quietly-redirected child
// this scrub exists to kill. GIT_TERMINAL_PROMPT=0 is set separately below
// so a fetch can never block on a credential prompt in a non-interactive
// build — it fails fast instead, and the user can retry with a configured
// credential helper or terminal prompt enabled.
func GitEnv() []string {
	env := make([]string, 0, len(os.Environ())+1)
	for _, kv := range os.Environ() {
		k, _, _ := strings.Cut(kv, "=")
		if scrubGitEnv(k) {
			continue
		}
		env = append(env, kv)
	}
	return append(env, gitTerminalPromptOff)
}

// scrubGitEnv reports whether an inherited environment variable must be
// stripped from a child git process. Everything git exports — GIT_DIR,
// GIT_INDEX_FILE, GIT_WORK_TREE, GIT_PREFIX, ... — is repository addressing
// that outranks fetchRepo's explicit init/<dir> and -C <dir> arguments and
// cannot be right for a process that names its own repository. There is
// deliberately no denylist of specific names: git defines more GIT_*
// variables with every version, and one that is easy to forget
// (GIT_INDEX_FILE) is exactly the one that rewrote this repository's index.
func scrubGitEnv(key string) bool {
	return strings.HasPrefix(key, "GIT_")
}
