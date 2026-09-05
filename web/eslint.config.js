import js from '@eslint/js';
import ts from 'typescript-eslint';
import svelte from 'eslint-plugin-svelte';
import globals from 'globals';

export default ts.config(
  js.configs.recommended,
  ...ts.configs.recommended,
  ...svelte.configs['flat/recommended'],
  { languageOptions: { globals: { ...globals.browser } } },
  // eslint-plugin-svelte's recommended config also parses .svelte.ts/.svelte.js
  // "Svelte module" files (Svelte 5 rune stores) with svelte-eslint-parser, but
  // doesn't wire a nested TS parser for them the way it does for .svelte SFCs —
  // without this they fail to parse (e.g. `import type { ... }`). Task 19 is
  // the first to add such a file (session.svelte.ts, tables.svelte.ts).
  { files: ['**/*.svelte', '**/*.svelte.ts', '**/*.svelte.js'], languageOptions: { parserOptions: { parser: ts.parser } } },
  { rules: { '@typescript-eslint/no-explicit-any': 'error' } },
  { ignores: ['dist/', 'node_modules/', '../cmd/gorged/webdist/', 'src/protocol.ts'] },
);
