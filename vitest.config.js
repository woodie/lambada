import { defineConfig } from 'vitest/config'

export default defineConfig({
  test: {
    setupFiles: ['./spec/javascript/setup.js'],
    include: ['spec/javascript/**/*.spec.js'],
    // With --coverage (npm run test-js, CI): the scan page is small enough to keep fully covered.
    coverage: {
      include: ['public/scans.js'],
      thresholds: { 100: true }
    }
  },
  resolve: {
    alias: [
      // public/lib/ is copied from node_modules by `make vendor`; tests use the original.
      { find: /^\.\/lib\/humane\/index\.js$/, replacement: 'humane-ts' }
    ]
  }
})
