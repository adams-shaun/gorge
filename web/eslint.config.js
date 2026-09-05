import js from '@eslint/js';
import ts from 'typescript-eslint';
import svelte from 'eslint-plugin-svelte';
import globals from 'globals';

export default ts.config(
  js.configs.recommended,
  ...ts.configs.recommended,
  ...svelte.configs['flat/recommended'],
  { languageOptions: { globals: { ...globals.browser } } },
  { files: ['**/*.svelte'], languageOptions: { parserOptions: { parser: ts.parser } } },
  { rules: { '@typescript-eslint/no-explicit-any': 'error' } },
  { ignores: ['dist/', 'node_modules/', '../cmd/gorged/webdist/', 'src/protocol.ts'] },
);
