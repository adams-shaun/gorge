## Summary

<!-- What changes and why. For a rules fix, cite the CR section and link the issue or report. -->

## Tests run

<!-- Paste the commands you ran and their result. A run without .cards/ skips the corpus tests; say so if that was the case. -->

```sh

```

## Checklist

- [ ] No Forge card scripts, or text derived from them, are committed.
- [ ] `CHANGELOG.md` has a line under `## [Unreleased]` (or this PR has no user-visible change).
- [ ] This PR changes a wire/file contract: the `contracts/registry.json` entry is updated (major bumped on a break, golden hashes refreshed), and `CHANGELOG.md` has a line under `### Contracts`.
- [ ] This PR breaks a stable package (`.github/stable-packages.txt`): the PR is labelled `breaking-change`, and `CHANGELOG.md` has a line under `### Breaking`.
- [ ] Golden chain heads (`rules/testdata/heads/`) moved: the cause is explained above.
