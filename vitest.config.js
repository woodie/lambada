import { defineConfig } from 'vitest/config'

export default defineConfig({
  test: {
    setupFiles: ['./spec/javascript/setup.js'],
    include: ['spec/javascript/**/*.spec.js'],
    // With --coverage (npm run test-js, CI): the scan page is small enough to keep fully covered.
    coverage: {
      include: ['src/main.ts'],
      thresholds: { 100: true }
    }
  }
})
