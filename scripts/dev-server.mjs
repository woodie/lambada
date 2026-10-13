// Tries the scan page without nginx: serves www/ the way
// service/lambada.nginx.conf does (index.html at /, the rest under
// /.lambada/), and passes every other request -- the PROPFIND listing,
// downloads and deletes -- on to a real lambada server.
//
//   npm run serve                                  # the Pi at 10.0.1.111
//   LAMBADA_URL=http://scans.netpress.com npm run serve
import http from 'node:http'
import { readFile } from 'node:fs/promises'
import path from 'node:path'

const target = new URL(process.env.LAMBADA_URL || 'http://10.0.1.111')
const port = Number(process.env.PORT || 8000)
const www = path.resolve(import.meta.dirname, '../www')
const types = { '.html': 'text/html; charset=utf-8', '.js': 'text/javascript', '.css': 'text/css' }

async function servePage (pathname, res) {
  const file = pathname === '/' ? 'index.html' : pathname.slice('/.lambada/'.length)
  const full = path.join(www, path.normalize(file))
  if (!full.startsWith(www + path.sep)) return res.writeHead(403).end()
  try {
    const body = await readFile(full)
    res.writeHead(200, { 'content-type': types[path.extname(full)] || 'application/octet-stream' }).end(body)
  } catch {
    res.writeHead(404).end()
  }
}

function forward (req, res) {
  const upstream = http.request(new URL(req.url, target), {
    method: req.method,
    headers: { ...req.headers, host: target.host }
  }, (reply) => {
    res.writeHead(reply.statusCode, reply.headers)
    reply.pipe(res)
  })
  upstream.on('error', (error) => res.writeHead(502).end(`Couldn't reach ${target.origin}: ${error.message}`))
  req.pipe(upstream)
}

http.createServer((req, res) => {
  const { pathname } = new URL(req.url, 'http://localhost')
  if (req.method === 'GET' && (pathname === '/' || pathname.startsWith('/.lambada/'))) return servePage(pathname, res)
  forward(req, res)
}).listen(port, () => {
  console.log(`Scan page at http://localhost:${port}/ -- listing, downloads and deletes go to ${target.origin}`)
})
