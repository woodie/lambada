import { readdirSync, readFileSync, rmSync, writeFileSync, existsSync } from 'node:fs'
import path from 'node:path'
import { describe, it, beforeEach, expect, inject } from 'vitest'
import { propfind, emailScan } from './helpers.js'

const { baseUrl, attachmentsDir, smtpPort } = inject('webdav')
const scan = Buffer.alloc(225_935, 7)
const onDisk = (name) => existsSync(path.join(attachmentsDir, name))

const lockBody = '<?xml version="1.0" encoding="utf-8"?><D:lockinfo xmlns:D="DAV:">' +
  '<D:lockscope><D:exclusive/></D:lockscope><D:locktype><D:write/></D:locktype></D:lockinfo>'

describe('service/lambada.nginx.conf', () => {
  beforeEach(() => {
    for (const name of readdirSync(attachmentsDir)) rmSync(path.join(attachmentsDir, name), { recursive: true })
    writeFileSync(path.join(attachmentsDir, '1791851911.pdf'), scan)
    writeFileSync(path.join(attachmentsDir, 'scan with spaces.pdf'), 'x')
    writeFileSync(path.join(attachmentsDir, '.hidden'), 'x')
  })

  describe('a browser visiting the root', () => {
    const subject = () => fetch(`${baseUrl}/`)

    it('gets the scan page', async () => {
      const response = await subject()
      expect(response.status).toBe(200)
      expect(await response.text()).toContain('<title>Available Scans</title>')
    })
  })

  describe("the scan page's own files", () => {
    for (const file of ['scans.js', 'vendor/humane/index.js', 'style.css']) {
      it(`serves /.lambada/${file}`, async () => {
        const response = await fetch(`${baseUrl}/.lambada/${file}`)
        expect(response.status).toBe(200)
        expect(response.headers.get('content-type')).toMatch(file.endsWith('.js') ? 'javascript' : 'css')
      })
    }
  })

  describe('OPTIONS', () => {
    const subject = () => fetch(`${baseUrl}/`, { method: 'OPTIONS' })

    it('advertises WebDAV with locking', async () => {
      const response = await subject()
      expect(response.headers.get('dav')).toContain('2')
    })
  })

  describe('listing the root (PROPFIND, Depth: 1)', () => {
    const subject = () => propfind(`${baseUrl}/`)
    const files = async () => (await subject()).entries.filter((entry) => !entry.isFolder)

    it('responds with a multistatus', async () => {
      expect((await subject()).status).toBe(207)
    })

    it('lists each file with its name and size', async () => {
      const listed = (await files()).map(({ name, size }) => ({ name, size }))
      expect(listed).toEqual(expect.arrayContaining([
        { name: '1791851911.pdf', size: 225_935 },
        { name: 'scan with spaces.pdf', size: 1 }
      ]))
    })

    it('gives each file a modified date', async () => {
      for (const entry of await files()) expect(Number.isNaN(entry.modified.getTime())).toBe(false)
    })

    it('leaves out hidden files', async () => {
      expect((await files()).map((entry) => entry.name)).not.toContain('.hidden')
    })
  })

  describe('downloading a file (GET)', () => {
    const subject = () => fetch(`${baseUrl}/1791851911.pdf`)

    it("returns the file's bytes", async () => {
      const response = await subject()
      expect(response.status).toBe(200)
      expect(Buffer.from(await response.arrayBuffer()).equals(scan)).toBe(true)
    })
  })

  describe('deleting a file (DELETE)', () => {
    let name
    const subject = () => fetch(`${baseUrl}/${encodeURIComponent(name)}`, { method: 'DELETE' })

    context('when the file exists', () => {
      beforeEach(() => { name = 'scan with spaces.pdf' })

      it('responds 204 and removes it', async () => {
        expect((await subject()).status).toBe(204)
        expect(onDisk(name)).toBe(false)
      })
    })

    context('when the file is already gone', () => {
      beforeEach(() => { name = 'missing.pdf' })

      it('responds 404', async () => {
        expect((await subject()).status).toBe(404)
      })
    })

    context('when another client has the file locked', () => {
      beforeEach(async () => {
        name = '1791851911.pdf'
        await fetch(`${baseUrl}/${name}`, { method: 'LOCK', headers: { 'Content-Type': 'application/xml' }, body: lockBody })
      })

      it('refuses with 423 and keeps the file', async () => {
        expect((await subject()).status).toBe(423)
        expect(onDisk(name)).toBe(true)
      })
    })
  })

  describe('requests that are refused', () => {
    let method, url, body
    const subject = () => fetch(url, { method, body })

    context('deleting the whole folder', () => {
      beforeEach(() => { method = 'DELETE'; url = `${baseUrl}/`; body = undefined })

      it('responds 403 and keeps every file', async () => {
        expect((await subject()).status).toBe(403)
        expect(onDisk('1791851911.pdf')).toBe(true)
      })
    })

    context('uploading a file', () => {
      beforeEach(() => { method = 'PUT'; url = `${baseUrl}/upload.pdf`; body = 'x' })

      it('responds 403 and creates nothing', async () => {
        expect((await subject()).status).toBe(403)
        expect(onDisk('upload.pdf')).toBe(false)
      })
    })

    context("changing the page's own files", () => {
      beforeEach(() => { method = 'PUT'; url = `${baseUrl}/.lambada/index.html`; body = 'x' })

      it('responds 403', async () => {
        expect((await subject()).status).toBe(403)
      })
    })

    context('creating a folder', () => {
      beforeEach(() => { method = 'MKCOL'; url = `${baseUrl}/folder/`; body = undefined })

      it('responds 403', async () => {
        expect((await subject()).status).toBe(403)
      })
    })

    context('copying a file', () => {
      beforeEach(() => { method = 'COPY'; url = `${baseUrl}/1791851911.pdf`; body = undefined })

      it('responds 403', async () => {
        expect((await subject()).status).toBe(403)
      })
    })
  })

  describe("Finder's hidden files", () => {
    const subject = () => fetch(`${baseUrl}/.DS_Store`, { method: 'PUT', body: 'finder' })

    it('accepts them', async () => {
      expect((await subject()).status).toBe(201)
    })

    it('never lists them', async () => {
      await subject()
      const names = (await propfind(`${baseUrl}/`)).entries.map((entry) => entry.name)
      expect(names).not.toContain('.DS_Store')
    })
  })

  describe('a scan emailed to lambada-mta', () => {
    const emailed = Buffer.from('%PDF-1.4 emailed scan')
    const subject = async () => {
      await emailScan({ port: smtpPort, filename: 'scan.pdf', content: emailed })
      for (let tries = 0; tries < 50; tries++) {
        const newFile = (await propfind(`${baseUrl}/`)).entries.find((entry) => entry.size === emailed.length)
        if (newFile) return newFile
        await new Promise((resolve) => setTimeout(resolve, 100))
      }
      return undefined
    }

    it('shows up in the WebDAV listing', async () => {
      const newFile = await subject()
      expect(newFile?.name).toMatch(/^\d+\.pdf$/)
    })

    it('downloads with the same bytes that were emailed', async () => {
      const newFile = await subject()
      const response = await fetch(`${baseUrl}/${newFile.name}`)
      expect(Buffer.from(await response.arrayBuffer()).equals(emailed)).toBe(true)
      expect(readFileSync(path.join(attachmentsDir, newFile.name)).equals(emailed)).toBe(true)
    })
  })
})
