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
  {
    ignores: ['dist/', 'src/routeTree.gen.ts', 'src/api.gen.ts'],
  },
)
