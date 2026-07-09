import { defineConfig } from 'vitest/config'

export default defineConfig({
  test: {
    setupFiles: ['./spec/javascript/setup.js'],
    include: ['spec/javascript/**/*.spec.js']
  }
})
