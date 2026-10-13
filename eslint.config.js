import neostandard from 'neostandard'

export default [
  ...neostandard({ ts: true, env: ['browser'], globals: ['context'] }),
  { ignores: ['public/main.js', 'coverage', 'scans-dev'] }
]
