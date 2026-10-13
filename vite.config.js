import { defineConfig } from 'vite'

// Library mode, like netpress: one fixed-name, minified file dropped into
// public/ next to the hand-edited index.html and style.css, and committed, so
// the Pi needs no npm. IIFE, so index.html can call Lambada.start().
export default defineConfig({
  publicDir: false,
  build: {
    outDir: 'public',
    emptyOutDir: false,
    lib: {
      entry: 'src/main.ts',
      formats: ['iife'],
      name: 'Lambada',
      fileName: () => 'main.js'
    }
  }
})
