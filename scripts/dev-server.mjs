// Works on the scan page with no nginx and no Pi: serves www/ the way
// service/lambada.nginx.conf does (index.html at /, the rest under
// /.lambada/), and answers the three WebDAV requests the page makes --
// PROPFIND (Depth: 1), GET and DELETE -- from a local folder of scans.
// The real nginx config is tested separately (spec/webdav, CI).
//
//   npm run serve                       # scans from ./scans-dev
//   LAMBADA_DEV_SCANS=~/Desktop/scans npm run serve
import http from 'node:http'
import { createReadStream } from 'node:fs'
import { readdir, stat, rm, mkdir } from 'node:fs/promises'
import path from 'node:path'

const root = path.resolve(import.meta.dirname, '..')
const www = path.join(root, 'www')
const scans = path.resolve(process.env.LAMBADA_DEV_SCANS || path.join(root, 'scans-dev'))
const port = Number(process.env.PORT || 8000)
const types = { '.html': 'text/html; charset=utf-8', '.js': 'text/javascript', '.css': 'text/css', '.pdf': 'application/pdf' }

// Resolves a request path inside `dir`, or null if it would escape it.
function inside (dir, requestPath) {
  const full = path.join(dir, path.normalize(decodeURIComponent(requestPath)))
  return full.startsWith(dir + path.sep) ? full : null
}

async function sendFile (res, file) {
  try {
    const info = await stat(file)
    if (!info.isFile()) throw new Error('not a file')
    res.writeHead(200, { 'content-type': types[path.extname(file)] || 'application/octet-stream', 'content-length': info.size })
    createReadStream(file).pipe(res)
  } catch {
    res.writeHead(404).end()
  }
}

const escapeXml = (text) => text.replace(/[<>&'"]/g, (c) => `&#${c.charCodeAt(0)};`)

// The same shape nginx's dav-ext module sends (see spec/webdav/fixtures/propfind-pi.xml).
async function propfind (res) {
  const entries = ['<D:response><D:href>/</D:href><D:propstat><D:prop>' +
    '<D:resourcetype><D:collection/></D:resourcetype></D:prop></D:propstat></D:response>']
  for (const name of await readdir(scans)) {
    if (name.startsWith('.')) continue
    const info = await stat(path.join(scans, name))
    if (!info.isFile()) continue
    entries.push(`<D:response><D:href>/${encodeURIComponent(name)}</D:href><D:propstat><D:prop>` +
      `<D:displayname>${escapeXml(name)}</D:displayname>` +
      `<D:getcontentlength>${info.size}</D:getcontentlength>` +
      `<D:getlastmodified>${info.mtime.toUTCString()}</D:getlastmodified>` +
      '<D:resourcetype></D:resourcetype></D:prop>' +
      '<D:status>HTTP/1.1 200 OK</D:status></D:propstat></D:response>')
  }
  res.writeHead(207, { 'content-type': 'text/xml; charset=utf-8' })
  res.end(`<?xml version="1.0" encoding="utf-8" ?>\n<D:multistatus xmlns:D="DAV:">\n${entries.join('\n')}\n</D:multistatus>\n`)
}

async function remove (res, pathname) {
  const file = inside(scans, pathname)
  if (!file) return res.writeHead(403).end()
  try {
    if (!(await stat(file)).isFile()) return res.writeHead(403).end()
    await rm(file)
    res.writeHead(204).end()
  } catch {
    res.writeHead(404).end()
  }
}

await mkdir(scans, { recursive: true })

http.createServer(async (req, res) => {
  const { pathname } = new URL(req.url, 'http://localhost')
  if (req.method === 'GET' && pathname === '/') return sendFile(res, path.join(www, 'index.html'))
  if (req.method === 'GET' && pathname.startsWith('/.lambada/')) {
    const file = inside(www, pathname.slice('/.lambada'.length))
    return file ? sendFile(res, file) : res.writeHead(403).end()
  }
  if (req.method === 'PROPFIND' && pathname === '/') return propfind(res)
  if (req.method === 'GET') {
    const file = inside(scans, pathname)
    return file ? sendFile(res, file) : res.writeHead(403).end()
  }
  if (req.method === 'DELETE') return remove(res, pathname)
  res.writeHead(405).end()
}).listen(port, () => {
  console.log(`Scan page at http://localhost:${port}/ -- scans from ${scans}`)
})
