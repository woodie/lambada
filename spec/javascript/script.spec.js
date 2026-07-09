import vm from 'node:vm'
import { readFileSync } from 'fs'
import { fileURLToPath } from 'url'
import path from 'path'
import sinon from 'sinon'
import { describe, it, beforeEach, afterEach, expect } from 'vitest'

// script.js is a plain browser global (served via <script src="script.js">
// in views/listing.html.tmpl, embedded into the binary with go:embed),
// not a module -- so it's evaluated as-is into a sandboxed global context
// rather than imported, leaving production code untouched. A bare vm
// context (rather than jsdom) is used because window.location.reload is
// non-configurable in jsdom and can't be stubbed.
const scriptPath = path.join(path.dirname(fileURLToPath(import.meta.url)), '../../cmd/lambada-web/static/script.js')
const scriptSource = readFileSync(scriptPath, 'utf8')

const flush = () => new Promise((resolve) => setTimeout(resolve, 0))

describe('deleteFile', () => {
  let sandbox, name, confirmMessage, confirmStub, fetchStub, alertStub, reloadStub

  beforeEach(() => {
    confirmStub = sinon.stub()
    fetchStub = sinon.stub()
    alertStub = sinon.stub()
    reloadStub = sinon.stub()

    sandbox = {
      confirm: confirmStub,
      fetch: fetchStub,
      alert: alertStub,
      location: { reload: reloadStub }
    }
    vm.createContext(sandbox)
    vm.runInContext(scriptSource, sandbox)

    name = 'scan with spaces.pdf'
    confirmMessage = 'Delete this scan from 3 minutes ago?'
  })

  afterEach(() => {
    sinon.restore()
  })

  it('asks for confirmation with the given message', async () => {
    confirmStub.returns(false)
    sandbox.deleteFile(name, confirmMessage)
    await flush()
    expect(confirmStub.calledWith(confirmMessage)).toBe(true)
  })

  context('when the user cancels the confirmation', () => {
    beforeEach(() => {
      confirmStub.returns(false)
    })

    it('does not send a delete request', async () => {
      sandbox.deleteFile(name, confirmMessage)
      await flush()
      expect(fetchStub.called).toBe(false)
    })
  })

  context('when the user confirms', () => {
    beforeEach(() => {
      confirmStub.returns(true)
    })

    context('when the delete request succeeds', () => {
      beforeEach(() => {
        fetchStub.resolves({ ok: true })
      })

      it('sends a DELETE request to the URL-encoded download path', async () => {
        sandbox.deleteFile(name, confirmMessage)
        await flush()
        expect(fetchStub.calledWith('/download/scan%20with%20spaces.pdf', { method: 'DELETE' })).toBe(true)
      })

      it('reloads the page', async () => {
        sandbox.deleteFile(name, confirmMessage)
        await flush()
        expect(reloadStub.called).toBe(true)
      })

      it('does not show an alert', async () => {
        sandbox.deleteFile(name, confirmMessage)
        await flush()
        expect(alertStub.called).toBe(false)
      })
    })

    context('when the delete request responds with a non-ok status', () => {
      beforeEach(() => {
        fetchStub.resolves({ ok: false })
      })

      it('shows a delete-failed alert', async () => {
        sandbox.deleteFile(name, confirmMessage)
        await flush()
        expect(alertStub.calledWith('Delete failed.')).toBe(true)
      })

      it('does not reload the page', async () => {
        sandbox.deleteFile(name, confirmMessage)
        await flush()
        expect(reloadStub.called).toBe(false)
      })
    })

    context('when the fetch call rejects', () => {
      beforeEach(() => {
        fetchStub.rejects(new Error('network error'))
      })

      it('shows a delete-failed alert', async () => {
        sandbox.deleteFile(name, confirmMessage)
        await flush()
        expect(alertStub.calledWith('Delete failed.')).toBe(true)
      })

      it('does not reload the page', async () => {
        sandbox.deleteFile(name, confirmMessage)
        await flush()
        expect(reloadStub.called).toBe(false)
      })
    })
  })
})
