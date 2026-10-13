import { defineConfig } from 'vitest/config'

export default defineConfig({
  test: {
    setupFiles: ['./spec/javascript/setup.js'],
    globalSetup: ['./spec/webdav/global-setup.js'],
    include: ['spec/webdav/**/*.spec.js'],
    fileParallelism: false
  }
})
