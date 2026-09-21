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
  {
    rules: {
      '@typescript-eslint/no-explicit-any': 'error',
      // core no-undef is off for TS, which is typescript-eslint's own
      // documented guidance: tsc already reports an undefined identifier, and
      // no-undef cannot see TYPE-position names, so it false-positives on
      // every DOM lib type used only in an annotation or assertion (here:
      // FeedbackButton's `as DisplayMediaStreamOptions`, which exists only
      // because the shipped TS lib lags the spec). Leaving it on would push
      // authors to delete correct type assertions to satisfy a linter that
      // does not understand them. svelte-check/tsc remain the real gate.
      'no-undef': 'off',
      // An argument named with a leading underscore is deliberately unused:
      // it documents a parameter kept for call-shape parity with a sibling
      // (castable.ts's `_decision`, which exists so the signature matches
      // autopilot.ts's actionables). Vars and caught errors stay strict --
      // only the args pattern is relaxed, so a genuinely dead local is still
      // an error.
      '@typescript-eslint/no-unused-vars': ['error', { argsIgnorePattern: '^_' }],
    },
  },
  // Browser tests must attach to the global setup fixture. This prevents a
  // copied test from silently adding another Vite server or Chromium process.
  {
    files: ['src/**/*.test.ts'],
    rules: {
      'no-restricted-imports': ['error', {
        paths: [
          { name: 'vite', importNames: ['createServer'], message: 'use the shared browser test fixture' },
          { name: 'playwright', importNames: ['chromium'], message: 'use sharedBrowser from src/test/browser' },
        ],
      }],
    },
  },
  { ignores: ['dist/', 'node_modules/', '../cmd/gorged/webdist/', 'src/protocol.ts'] },
);
