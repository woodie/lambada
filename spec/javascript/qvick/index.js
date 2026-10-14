// qvick: Quick's justBeforeEach for Vitest. Copied from github.com/woodie/qvick until it's on npm.
import { beforeEach } from 'vitest'
import { getFn, setFn } from '@vitest/runner'

const queues = new WeakMap()

export function justBeforeEach (step) {
  beforeEach((context) => {
    let queue = queues.get(context.task)
    if (!queue) {
      queue = []
      queues.set(context.task, queue)
      const test = getFn(context.task)
      setFn(context.task, async () => {
        for (const queued of queue) await queued()
        return test()
      })
    }
    queue.push(() => step(context))
    return () => { queue.length = 0 }
  })
}
