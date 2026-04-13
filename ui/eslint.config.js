import js from '@eslint/js'
import tseslint from 'typescript-eslint'
import reactHooks from 'eslint-plugin-react-hooks'

export default tseslint.config(
  js.configs.recommended,
  ...tseslint.configs.recommended,
  {
    plugins: { 'react-hooks': reactHooks },
    rules: {
      ...reactHooks.configs.recommended.rules,
      // Enforce barrel imports for MUI — prevents deep path imports like
      // `@mui/material/Button` or `@mui/icons-material/Home` which break
      // tree-shaking awareness and cause the "missing type declarations" error.
      'no-restricted-imports': ['error', {
        patterns: [
          {
            group: ['@mui/material/*'],
            message: 'Use barrel import: import { ... } from "@mui/material"',
          },
          {
            group: ['@mui/icons-material/*'],
            message: 'Use barrel import: import { HomeIcon } from "@mui/icons-material"',
          },
        ],
      }],
      '@typescript-eslint/no-explicit-any': 'warn',
      '@typescript-eslint/no-unused-vars': ['warn', { argsIgnorePattern: '^_', varsIgnorePattern: '^_' }],
    },
  },
  // Ban raw apiFetch/apiPost outside the api module — use typed client hooks instead.
  {
    files: ['src/**/*.ts', 'src/**/*.tsx'],
    ignores: ['src/api/index.ts'],
    rules: {
      'no-restricted-syntax': [
        'error',
        {
          selector: 'CallExpression[callee.name="apiFetch"]',
          message: 'Add a typed hook in src/api/index.ts instead of calling apiFetch directly.',
        },
        {
          selector: 'CallExpression[callee.name="apiPost"]',
          message: 'Add a typed hook in src/api/index.ts instead of calling apiPost directly.',
        },
      ],
    },
  },
  {
    ignores: ['dist/', 'src/routeTree.gen.ts', 'src/api.gen.ts'],
  },
)
