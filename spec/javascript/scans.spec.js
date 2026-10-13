// @vitest-environment jsdom
import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import path from 'node:path'
import { describe, it, beforeEach, expect, vi } from 'vitest'
import { parseListing, age, renderScans, start } from '../../www/scans.js'

const here = path.dirname(fileURLToPath(import.meta.url))
const piReply = readFileSync(path.join(here, '../webdav/fixtures/propfind-pi.xml'), 'utf8')
const flush = () => new Promise((resolve) => setTimeout(resolve, 0))

function multistatus (files) {
  const folder = '<D:response><D:href>/</D:href><D:propstat><D:prop>' +
    '<D:resourcetype><D:collection/></D:resourcetype></D:prop></D:propstat></D:response>'
  const entries = files.map(({ name, size = 7, modified }) =>
    `<D:response><D:href>/${encodeURIComponent(name)}</D:href><D:propstat><D:prop>` +
    `<D:displayname>${name}</D:displayname><D:getcontentlength>${size}</D:getcontentlength>` +
    `<D:getlastmodified>${modified}</D:getlastmodified><D:resourcetype/></D:prop></D:propstat></D:response>`)
  return `<D:multistatus xmlns:D="DAV:">${folder}${entries.join('')}</D:multistatus>`
}

describe('parseListing', () => {
  let xml, scans

  const subject = () => parseListing(xml)

  context('with the reply nginx gave on the Pi', () => {
    beforeEach(() => { xml = piReply; scans = subject() })

    it('skips the folder itself', () => {
      expect(scans).toHaveLength(1)
    })

    it("reads the file's name, size and address", () => {
      expect(scans[0]).toMatchObject({ name: '1791851911.pdf', size: 252184, href: '/scans/1791851911.pdf' })
    })

    it('reads the modified date', () => {
      expect(scans[0].modified.toISOString()).toBe('2026-10-13T00:38:33.000Z')
    })
  })

  context('with several files', () => {
    beforeEach(() => {
      xml = multistatus([
        { name: 'older.pdf', modified: 'Mon, 12 Oct 2026 21:48:35 GMT' },
        { name: 'newer.pdf', modified: 'Tue, 13 Oct 2026 00:38:33 GMT' }
      ])
      scans = subject()
    })

    it('lists them newest first', () => {
      expect(scans.map((scan) => scan.name)).toEqual(['newer.pdf', 'older.pdf'])
    })
  })

  context('when an entry has no displayname', () => {
    beforeEach(() => {
      xml = '<D:multistatus xmlns:D="DAV:"><D:response><D:href>/scan%20one.pdf</D:href>' +
        '<D:propstat><D:prop><D:resourcetype/></D:prop></D:propstat></D:response></D:multistatus>'
      scans = subject()
    })

    it('takes the name from the address', () => {
      expect(scans[0].name).toBe('scan one.pdf')
    })

    it('has no modified date', () => {
      expect(scans[0].modified).toBeNull()
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
  const now = new Date('2026-10-13T00:41:33Z')

  it("says how long ago the scan arrived, in humane-ts's words", () => {
    expect(age({ modified: new Date('2026-10-13T00:38:33Z') }, now)).toBe('3 minutes ago')
  })

  it('falls back when there is no date', () => {
    expect(age({ modified: null }, now)).toBe('an unknown time')
  })
})

describe('renderScans', () => {
  const now = new Date('2026-10-13T00:41:33Z')
  const scan = { href: '/1791851911.pdf', name: '1791851911.pdf', size: 225935, modified: new Date('2026-10-13T00:38:33Z') }
  let list

  beforeEach(() => {
    list = document.createElement('ul')
    list.append(document.createElement('li'))
    renderScans(list, [scan], now)
  })

  it('replaces the list with one item per scan', () => {
    expect(list.children).toHaveLength(1)
  })

  it('links to the scan, showing its size', () => {
    const link = list.querySelector('a')
    expect(link.getAttribute('href')).toBe('/1791851911.pdf')
    expect(link.textContent).toBe('📄 226 KB')
  })

  it('shows how long ago it arrived', () => {
    expect(list.querySelector('span').textContent).toBe('3 minutes ago')
  })

  it('gives the delete button the scan\'s address', () => {
    expect(list.querySelector('button.delete').dataset.href).toBe('/1791851911.pdf')
  })
})

describe('start', () => {
  let env, listing

  const subject = () => start(env)
  const list = () => document.getElementById('scans')
  const status = () => document.getElementById('status')
  const respond = (status, body = '') => Promise.resolve(new Response(body, { status }))

  beforeEach(() => {
    document.body.innerHTML = '<p id="status"></p><ul id="scans"></ul>'
    listing = multistatus([{ name: 'a.pdf', modified: new Date().toUTCString() }])
    env = {
      document,
      DOMParser,
      fetch: vi.fn((url, { method }) => method === 'PROPFIND' ? respond(207, listing) : respond(204)),
      confirm: vi.fn(() => true),
      alert: vi.fn(),
      setInterval: vi.fn()
    }
  })

  context('when the server lists one scan', () => {
    beforeEach(subject)

    it('sends PROPFIND with Depth: 1 to the root', () => {
      expect(env.fetch).toHaveBeenCalledWith('/', { method: 'PROPFIND', headers: { Depth: '1' } })
    })

    it('shows it', () => {
      expect(list().querySelectorAll('li')).toHaveLength(1)
      expect(status().textContent).toBe('')
    })

    it('refreshes the ages every 30 seconds', () => {
      expect(env.setInterval).toHaveBeenCalledWith(expect.any(Function), 30_000)
    })
  })

  context('when the folder is empty', () => {
    beforeEach(() => { listing = multistatus([]); return subject() })

    it('says so', () => {
      expect(status().textContent).toBe('No scans yet.')
    })
  })

  context('when the listing fails', () => {
    beforeEach(() => { env.fetch = vi.fn(() => respond(500)); return subject() })

    it('says what went wrong', () => {
      expect(status().textContent).toBe("Couldn't load the scans (status 500).")
    })
  })

  describe('clicking delete', () => {
    const click = async () => {
      list().querySelector('button.delete').click()
      await flush()
    }

    beforeEach(subject)

    it('asks first, naming when the scan arrived', async () => {
      await click()
      expect(env.confirm).toHaveBeenCalledWith('Delete this scan from less than a minute ago?')
    })

    context('when confirmed', () => {
      beforeEach(async () => {
        listing = multistatus([])
        await click()
      })

      it("sends DELETE to the scan's address", () => {
        expect(env.fetch).toHaveBeenCalledWith('/a.pdf', { method: 'DELETE' })
      })

      it('reloads the list', () => {
        expect(status().textContent).toBe('No scans yet.')
      })
    })

    context('when cancelled', () => {
      beforeEach(async () => {
        env.confirm = vi.fn(() => false)
        await click()
      })

      it('deletes nothing', () => {
        expect(env.fetch).not.toHaveBeenCalledWith('/a.pdf', expect.anything())
      })
    })

    context('when the server refuses', () => {
      beforeEach(async () => {
        env.fetch = vi.fn((url, { method }) => method === 'PROPFIND' ? respond(207, listing) : respond(423))
        await click()
      })

      it('says the delete failed', () => {
        expect(env.alert).toHaveBeenCalledWith('Delete failed.')
      })
    })

    context('when the scan is already gone', () => {
      beforeEach(async () => {
        env.fetch = vi.fn((url, { method }) => method === 'PROPFIND' ? respond(207, listing) : respond(404))
        await click()
      })

      it("doesn't complain", () => {
        expect(env.alert).not.toHaveBeenCalled()
      })
    })
  })
})
