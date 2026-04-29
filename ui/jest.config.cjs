/* eslint-env node */
module.exports = {
  testEnvironment: 'jsdom',
  transform: {
    '^.+\\.[jt]sx?$': ['@swc/jest', {
      jsc: {
        parser: { syntax: 'typescript', tsx: true },
        transform: { react: { runtime: 'automatic' } },
      },
    }],
  },
  transformIgnorePatterns: [
    '/node_modules/.pnpm/(?!(react-markdown|remark-|unified|mdast-|micromark|devlop|unist-|decode-named-character-reference|character-entities|property-information|hast-util-|comma-separated-tokens|space-separated-tokens|vfile|ccount|escape-string-regexp|markdown-table|zwitch|longest-streak|trim-lines|estree-util-|bail))',
  ],
  setupFilesAfterEnv: ['<rootDir>/src/test-setup.ts'],
  testMatch: ['<rootDir>/src/**/*.test.ts', '<rootDir>/src/**/*.test.tsx'],
}
