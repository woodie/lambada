// @vitest-environment jsdom
import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import path from 'node:path'
import { describe, it, beforeEach, afterEach, expect, vi } from 'vitest'
import { parseListing, age, renderScans, start } from '../../src/main.ts'

const here = path.dirname(fileURLToPath(import.meta.url))
const piReply = readFileSync(path.join(here, '../webdav/fixtures/propfind-pi.xml'), 'utf8')
const indexHtml = readFileSync(path.join(here, '../../public/index.html'), 'utf8')

// Fixed "now" for every test, so ages are exact rather than timing-dependent.
const NOW = new Date('2026-10-13T00:41:33Z')
const minutesAgo = (minutes) => new Date(NOW.getTime() - minutes * 60_000)

// Lets pending promise callbacks (fetch, then render) run. setTimeout stays
// real; only Date and setInterval are faked below.
const flush = () => new Promise((resolve) => setTimeout(resolve, 0))

// A PROPFIND reply in the shape nginx sends: the folder, then one entry per file.
function multistatus (files) {
  const folder = '<D:response><D:href>/</D:href><D:propstat><D:prop>' +
    '<D:resourcetype><D:collection/></D:resourcetype></D:prop></D:propstat></D:response>'
  const entries = files.map(({ name, size = 7, modified }) =>
    `<D:response><D:href>/${encodeURIComponent(name)}</D:href><D:propstat><D:prop>` +
    `<D:displayname>${name.replace(/&/g, '&amp;').replace(/</g, '&lt;')}</D:displayname>` +
    `<D:getcontentlength>${size}</D:getcontentlength>` +
    `<D:getlastmodified>${modified.toUTCString()}</D:getlastmodified>` +
    '<D:resourcetype/></D:prop></D:propstat></D:response>')
  return `<D:multistatus xmlns:D="DAV:">${folder}${entries.join('')}</D:multistatus>`
}

beforeEach(() => {
  vi.useFakeTimers({ toFake: ['Date', 'setInterval', 'clearInterval'] })
  vi.setSystemTime(NOW)
})

afterEach(() => {
  vi.useRealTimers()
})

describe('parseListing', () => {
  let xml

  const subject = () => parseListing(xml)

  context('with the reply nginx gave on the Pi', () => {
    beforeEach(() => { xml = piReply })

    it('skips the folder itself', () => {
      expect(subject()).toHaveLength(1)
    })

    it("reads the file's name, size and address", () => {
      expect(subject()[0]).toMatchObject({ name: '1791851911.pdf', size: 252184, href: '/scans/1791851911.pdf' })
    })

    it('reads the modified date', () => {
      expect(subject()[0].modified).toEqual(new Date('2026-10-13T00:38:33Z'))
    })
  })

  context('with several files', () => {
    beforeEach(() => {
      xml = multistatus([
        { name: 'older.pdf', modified: minutesAgo(180) },
        { name: 'newest.pdf', modified: minutesAgo(3) },
        { name: 'oldest.pdf', modified: minutesAgo(9 * 1440) }
      ])
    })

    it('lists them newest first', () => {
      expect(subject().map((scan) => scan.name)).toEqual(['newest.pdf', 'older.pdf', 'oldest.pdf'])
    })
  })

  context('with a name that needs encoding', () => {
    beforeEach(() => { xml = multistatus([{ name: 'scan & one.pdf', modified: minutesAgo(3) }]) })

    it('keeps the plain name for display', () => {
      expect(subject()[0].name).toBe('scan & one.pdf')
    })

    it('keeps the encoded address for requests', () => {
      expect(subject()[0].href).toBe('/scan%20%26%20one.pdf')
    })
  })

  context('when one entry has no date', () => {
    beforeEach(() => {
      xml = multistatus([{ name: 'dated.pdf', modified: minutesAgo(9 * 1440) }])
        .replace('</D:multistatus>', '<D:response><D:href>/undated.pdf</D:href>' +
          '<D:propstat><D:prop><D:resourcetype/></D:prop></D:propstat></D:response></D:multistatus>')
    })

    it('lists it last', () => {
      expect(subject().map((scan) => scan.name)).toEqual(['dated.pdf', 'undated.pdf'])
    })
  })

  context('when an entry has no displayname or date', () => {
    beforeEach(() => {
      xml = '<D:multistatus xmlns:D="DAV:"><D:response><D:href>/scan%20one.pdf</D:href>' +
        '<D:propstat><D:prop><D:resourcetype/></D:prop></D:propstat></D:response></D:multistatus>'
    })

    it('takes the name from the address', () => {
      expect(subject()[0].name).toBe('scan one.pdf')
    })

    it('has no modified date', () => {
      expect(subject()[0].modified).toBeNull()
    })
  })

  context('with a different namespace prefix', () => {
    beforeEach(() => { xml = multistatus([{ name: 'a.pdf', modified: minutesAgo(3) }]).replace(/D:/g, 'd:').replace('xmlns:D=', 'xmlns:d=') })

    it('still reads it, since only the DAV: namespace matters', () => {
      expect(subject().map((scan) => scan.name)).toEqual(['a.pdf'])
    })
  })

  context("when the reply isn't XML", () => {
    beforeEach(() => { xml = '<html><body>405 Not Allowed' })

    it('throws', () => {
      expect(subject).toThrow('unreadable listing')
    })
  })
})

describe('age', () => {
  let modified

  const subject = () => age({ modified }, NOW)

  context('for a scan from 3 minutes ago', () => {
    beforeEach(() => { modified = minutesAgo(3) })

    it("uses humane-ts's wording", () => {
      expect(subject()).toBe('3 minutes ago')
    })
  })

  context("when the server's clock is ahead of the browser's", () => {
    beforeEach(() => { modified = minutesAgo(-3) })

    it('says "in 3 minutes" rather than "ago"', () => {
      expect(subject()).toBe('in 3 minutes')
    })
  })

  context('with no date', () => {
    beforeEach(() => { modified = null })

    it('falls back to "an unknown time"', () => {
      expect(subject()).toBe('an unknown time')
    })
  })
})

describe('renderScans', () => {
  let list, scans

  const subject = () => renderScans(list, scans, NOW)

  beforeEach(() => {
    list = document.createElement('ul')
    list.append(document.createElement('li'))
    scans = [{ href: '/1791851911.pdf', name: '1791851911.pdf', size: 225_935, modified: minutesAgo(3) }]
  })

  it('replaces whatever the list held with one item per scan', () => {
    subject()
    expect(list.children).toHaveLength(1)
  })

  it("links to the scan's address", () => {
    subject()
    expect(list.querySelector('a').getAttribute('href')).toBe('/1791851911.pdf')
  })

  it('shows the name in the link', () => {
    subject()
    expect(list.querySelector('a').textContent).toBe('📄 1791851911.pdf')
  })

  it('shows the size in Finder\'s wording', () => {
    subject()
    expect(list.querySelector('span.size').textContent).toBe('226 KB')
  })

  it('shows how long ago the scan arrived', () => {
    subject()
    expect(list.querySelector('span.age').textContent).toBe('3 minutes ago')
  })

  it("gives the delete button the scan's address", () => {
    subject()
    expect(list.querySelector('button.delete').dataset.href).toBe('/1791851911.pdf')
  })

  context('with no scans', () => {
    beforeEach(() => { scans = [] })

    it('empties the list', () => {
      subject()
      expect(list.children).toHaveLength(0)
    })
  })

  context('with a name that looks like HTML', () => {
    beforeEach(() => { scans[0].name = '<img src=x onerror=alert(1)>.pdf' })

    it('treats it as plain text', () => {
      subject()
      expect(list.querySelector('img')).toBeNull()
      expect(list.querySelector('a').textContent).toBe('📄 <img src=x onerror=alert(1)>.pdf')
      expect(list.querySelector('button.delete').title).toBe('Delete <img src=x onerror=alert(1)>.pdf')
    })
  })
})

describe('start', () => {
  let env, listing, deleteStatus, putStatus, visibility

  const subject = async () => { await start(env); await flush() }
  const list = () => document.getElementById('scans')
  const status = () => document.getElementById('status')
  const reply = (code, body = null) => Promise.resolve(new Response(body, { status: code }))
  const requests = (method) => env.fetch.mock.calls.filter(([, options]) => options.method === method)
  const clickDelete = async () => { list().querySelector('button.delete').click(); await flush() }

  beforeEach(() => {
    // The real page, so these tests break if index.html and scans.js drift apart.
    document.documentElement.innerHTML = indexHtml.replace(/^<!DOCTYPE html>\s*<html[^>]*>|<\/html>\s*$/g, '')
    listing = multistatus([{ name: 'scan one.pdf', size: 225_935, modified: minutesAgo(3) }])
    deleteStatus = 204
    putStatus = 201
    visibility = 'visible'
    Object.defineProperty(document, 'visibilityState', { configurable: true, get: () => visibility })
    env = {
      document,
      DOMParser,
      setInterval,
      fetch: vi.fn((url, { method }) => {
        if (method === 'PROPFIND') return reply(207, listing)
        return reply(method === 'PUT' ? putStatus : deleteStatus)
      }),
      confirm: vi.fn(() => true),
      alert: vi.fn()
    }
  })

  context('when the server lists a scan', () => {
    beforeEach(subject)

    it('sends PROPFIND with Depth: 1 to the root', () => {
      expect(env.fetch).toHaveBeenCalledWith('/', { method: 'PROPFIND', headers: { Depth: '1' } })
    })

    it('shows it', () => {
      expect(list().querySelector('li').textContent).toBe('🗑️📄 scan one.pdf226 KB3 minutes ago')
    })

    it('shows no status message', () => {
      expect(status().textContent).toBe('')
    })

    context('and a minute passes', () => {
      beforeEach(() => vi.advanceTimersByTime(60_000))

      it('updates the age without asking the server again', () => {
        expect(list().querySelector('span.age').textContent).toBe('4 minutes ago')
        expect(requests('PROPFIND')).toHaveLength(1)
      })
    })
  })

  context('when the folder is empty', () => {
    beforeEach(() => { listing = multistatus([]); return subject() })

    it('says so', () => {
      expect(status().textContent).toBe('No scans yet.')
    })
  })

  context('when the server answers with an error', () => {
    beforeEach(() => {
      env.fetch = vi.fn(() => reply(500))
      return subject()
    })

    it('says what went wrong', () => {
      expect(status().textContent).toBe("Couldn't load the scans (status 500).")
    })
  })

  context("when the server can't be reached", () => {
    beforeEach(() => {
      env.fetch = vi.fn(() => Promise.reject(new TypeError('Failed to fetch')))
      return subject()
    })

    it('says what went wrong', () => {
      expect(status().textContent).toBe("Couldn't load the scans (Failed to fetch).")
    })
  })

  describe('coming back to the tab', () => {
    const switchTo = async (state) => {
      visibility = state
      document.dispatchEvent(new Event('visibilitychange'))
      await flush()
    }

    beforeEach(subject)

    it('reloads the list', async () => {
      listing = multistatus([])
      await switchTo('visible')
      expect(status().textContent).toBe('No scans yet.')
    })

    it("doesn't reload while the tab is hidden", async () => {
      await switchTo('hidden')
      expect(requests('PROPFIND')).toHaveLength(1)
    })

    context('after a failed load', () => {
      beforeEach(async () => {
        env.fetch.mockImplementationOnce(() => reply(500))
        await switchTo('visible')
      })

      it('clears the error once the server answers again', async () => {
        await switchTo('visible')
        expect(status().textContent).toBe('')
      })
    })
  })

  describe('clicking delete', () => {
    beforeEach(subject)

    it('asks first, naming the file', async () => {
      await clickDelete()
      expect(env.confirm).toHaveBeenCalledWith('Delete scan one.pdf?')
    })

    context('when cancelled', () => {
      beforeEach(async () => {
        env.confirm.mockReturnValue(false)
        await clickDelete()
      })

      it('deletes nothing', () => {
        expect(requests('DELETE')).toHaveLength(0)
      })
    })

    context('when confirmed', () => {
      beforeEach(async () => {
        listing = multistatus([])
        await clickDelete()
      })

      it("sends DELETE to the scan's encoded address", () => {
        expect(env.fetch).toHaveBeenCalledWith('/scan%20one.pdf', { method: 'DELETE' })
      })

      it("doesn't alert", () => {
        expect(env.alert).not.toHaveBeenCalled()
      })

      it('reloads the list', () => {
        expect(status().textContent).toBe('No scans yet.')
      })
    })

    context('when the server refuses (for example, locked in Finder)', () => {
      beforeEach(async () => {
        deleteStatus = 423
        await clickDelete()
      })

      it('says the delete failed', () => {
        expect(env.alert).toHaveBeenCalledWith('Delete failed.')
      })

      it('reloads the list anyway, so it shows what is really there', () => {
        expect(requests('PROPFIND')).toHaveLength(2)
      })
    })

    context("when the server can't be reached", () => {
      beforeEach(async () => {
        env.fetch.mockImplementation((url, { method }) =>
          method === 'PROPFIND' ? reply(207, listing) : Promise.reject(new TypeError('Failed to fetch')))
        await clickDelete()
      })

      it('says the delete failed', () => {
        expect(env.alert).toHaveBeenCalledWith('Delete failed.')
      })
    })

    context('when someone else already deleted it', () => {
      beforeEach(async () => {
        deleteStatus = 404
        await clickDelete()
      })

      it("doesn't complain, since it's gone either way", () => {
        expect(env.alert).not.toHaveBeenCalled()
      })
    })
  })

  context('clicking somewhere other than a delete button', () => {
    beforeEach(async () => {
      await subject()
      list().querySelector('span.age').click()
      await flush()
    })

    it('does nothing', () => {
      expect(env.confirm).not.toHaveBeenCalled()
    })
  })

  describe('dropping files on the page', () => {
    let files
    const file = (name, size = 7) => ({ name, size })
    const dragEvent = (type, props = {}) => {
      const event = new Event(type, { cancelable: true })
      for (const [key, value] of Object.entries(props)) Object.defineProperty(event, key, { value })
      return event
    }
    const drop = async () => {
      const event = dragEvent('drop', { dataTransfer: { files } })
      document.dispatchEvent(event)
      await flush()
      return event
    }

    beforeEach(async () => {
      files = [file('sample.png')]
      await subject()
    })

    it('takes the drop instead of letting the browser open the file', async () => {
      expect((await drop()).defaultPrevented).toBe(true)
    })

    it('PUTs the file under its own name', async () => {
      await drop()
      expect(env.fetch).toHaveBeenCalledWith('/sample.png', { method: 'PUT', body: files[0] })
    })

    it('reloads the list', async () => {
      await drop()
      expect(requests('PROPFIND')).toHaveLength(2)
    })

    it("doesn't alert", async () => {
      await drop()
      expect(env.alert).not.toHaveBeenCalled()
    })

    context('with a name that needs encoding', () => {
      beforeEach(() => { files = [file('my file #2.pdf')] })

      it('encodes it in the address', async () => {
        await drop()
        expect(requests('PUT')[0][0]).toBe('/my%20file%20%232.pdf')
      })
    })

    context('with several files', () => {
      beforeEach(() => { files = [file('a.txt'), file('b.txt')] })

      it('uploads each, then reloads once', async () => {
        await drop()
        expect(requests('PUT').map(([url]) => url)).toEqual(['/a.txt', '/b.txt'])
        expect(requests('PROPFIND')).toHaveLength(2)
      })
    })

    context("with a scan's name", () => {
      beforeEach(() => { files = [file('1791851911.pdf')] })

      it("doesn't send it, and says why", async () => {
        await drop()
        expect(requests('PUT')).toHaveLength(0)
        expect(env.alert).toHaveBeenCalledWith("Couldn't upload 1791851911.pdf (that name is kept for scans).")
      })
    })

    context('over 25 MB', () => {
      beforeEach(() => { files = [file('big.zip', 25 * 1024 * 1024 + 1)] })

      it("doesn't send it, and says why", async () => {
        await drop()
        expect(requests('PUT')).toHaveLength(0)
        expect(env.alert).toHaveBeenCalledWith("Couldn't upload big.zip (over 25 MB).")
      })
    })

    context('with the name of a file already listed', () => {
      beforeEach(() => { files = [file('scan one.pdf')] })

      it('asks before replacing it', async () => {
        await drop()
        expect(env.confirm).toHaveBeenCalledWith('Replace scan one.pdf?')
      })

      context('when cancelled', () => {
        beforeEach(() => env.confirm.mockReturnValue(false))

        it('sends nothing', async () => {
          await drop()
          expect(requests('PUT')).toHaveLength(0)
        })
      })

      context('when confirmed', () => {
        it('replaces it', async () => {
          await drop()
          expect(requests('PUT')).toHaveLength(1)
        })
      })
    })

    context('when the server refuses', () => {
      beforeEach(() => { putStatus = 403 })

      it('says which file failed and why', async () => {
        await drop()
        expect(env.alert).toHaveBeenCalledWith("Couldn't upload sample.png (status 403).")
      })
    })

    context("when the server can't be reached", () => {
      beforeEach(() => {
        env.fetch.mockImplementation((url, { method }) =>
          method === 'PROPFIND' ? reply(207, listing) : Promise.reject(new TypeError('Failed to fetch')))
      })

      it('says which file failed and why', async () => {
        await drop()
        expect(env.alert).toHaveBeenCalledWith("Couldn't upload sample.png (Failed to fetch).")
      })
    })

    context('when nothing usable is dropped', () => {
      it('does nothing', async () => {
        document.dispatchEvent(dragEvent('drop'))
        await flush()
        expect(requests('PUT')).toHaveLength(0)
        expect(requests('PROPFIND')).toHaveLength(1)
      })
    })

    describe('the outline while dragging', () => {
      const dropping = () => document.body.classList.contains('dropping')

      beforeEach(() => document.dispatchEvent(dragEvent('dragover')))

      it('shows over the page', () => {
        expect(dropping()).toBe(true)
      })

      it('stays while moving between elements on the page', () => {
        document.dispatchEvent(dragEvent('dragleave', { relatedTarget: list() }))
        expect(dropping()).toBe(true)
      })

      it('goes when the drag leaves the window', () => {
        document.dispatchEvent(dragEvent('dragleave', { relatedTarget: null }))
        expect(dropping()).toBe(false)
      })

      it('goes on drop', async () => {
        await drop()
        expect(dropping()).toBe(false)
      })
    })
  })

  describe('the upload button', () => {
    const picker = () => document.getElementById('upload')

    beforeEach(subject)

    it('opens the file picker', () => {
      const click = vi.spyOn(picker(), 'click').mockImplementation(() => {})
      document.getElementById('upload-button').click()
      expect(click).toHaveBeenCalled()
    })

    context('when files are picked', () => {
      const pick = async (files) => {
        Object.defineProperty(picker(), 'files', { configurable: true, value: files })
        picker().dispatchEvent(new Event('change'))
        await flush()
      }

      it('uploads them', async () => {
        await pick([{ name: 'picked.pdf', size: 7 }])
        expect(requests('PUT')[0][0]).toBe('/picked.pdf')
      })

      it('clears the picker, so the same file can be picked again', async () => {
        await pick([{ name: 'picked.pdf', size: 7 }])
        expect(picker().value).toBe('')
      })

      it('does nothing without a file list', async () => {
        await pick(null)
        expect(requests('PUT')).toHaveLength(0)
      })
    })
  })
})
