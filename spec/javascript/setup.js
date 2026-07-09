import { describe } from 'vitest'

// Alias `context` to `describe`, matching the RSpec-style describe/context/it
// hierarchy used across this account's other projects (see docs/COWORK.md).
globalThis.context = describe
