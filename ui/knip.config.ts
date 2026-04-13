import type { KnipConfig } from 'knip'

const config: KnipConfig = {
  entry: ['src/routes/**/*.tsx'],
  project: ['src/**/*.{ts,tsx}'],
  ignore: ['src/api.gen.ts'],
  ignoreDependencies: [
    // Used at build time by the vite tanstackRouter plugin, not a direct import.
    '@tanstack/router-cli',
  ],
  ignoreExportsUsedInFile: true,
  // apiFetch/apiPost/token helpers are intentionally internal to api/index.ts.
  // They would be flagged as unused exports because knip sees them exported but
  // not imported elsewhere. The lint rule (eslint no-restricted-syntax) already
  // enforces they stay internal.
  // Tag them as @public in comments to suppress, or use ignoreExportsUsedInFile above.
}

export default config
