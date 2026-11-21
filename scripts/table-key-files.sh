#!/usr/bin/env bash
# table-key-files.sh: print, one per line from the repository root, every file whose bytes can decide dirac's card
# tables: each non-test Go file and each embedded file (go:embed, which carries tablesrc.go's pool lists, set lists,
# support data and manifest entries) of every package in this module that ./dirac/cmd/kgen is built from.
#
# 11-21: ci-corpus.sh keyed a box's table directory by tablesrc.go's embeds alone, but kgen also compiles in packages
# outside them (engine/rules/pay, engine/rules/chars, dirac), which changed on 11-20 without moving that key. A box
# then served tables an older tree had built: batch #3110 passed TestMBCGreatHallOfStarnheimReprosLockstep on them,
# and the same tree failed it with fresh tables (main went red; revert #3141). The list comes from go list, so a new
# kgen dependency joins the key by itself (scripts/table_key_files_test.go).
set -euo pipefail
cd "$(git rev-parse --show-toplevel)"
mod=$(go list -m)
go list -deps -f '{{if .Module}}{{if eq .Module.Path "'"$mod"'"}}{{$d := .Dir}}{{range .GoFiles}}{{$d}}/{{.}}
{{end}}{{range .EmbedFiles}}{{$d}}/{{.}}
{{end}}{{end}}{{end}}' ./dirac/cmd/kgen | sed '/^$/d' | sed "s#^$PWD/##" | sort -u
