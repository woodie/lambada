import { defineConfig } from 'vitest/config'

export default defineConfig({
  test: {
    setupFiles: ['./spec/javascript/setup.js'],
    include: ['spec/javascript/**/*.spec.js']
  },
  resolve: {
    alias: [
      // www/lib/ is copied from node_modules by `make vendor`; tests use the original.
      { find: /^\.\/lib\/humane\/index\.js$/, replacement: 'humane-ts' }
    ]
  }
})
