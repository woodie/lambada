import { humanSize, distanceInTime } from './lib/humane/index.js'

const DAV = 'DAV:'

// Reads a WebDAV PROPFIND (Depth: 1) reply into one entry per file, newest
// first. Folders, including the listed folder itself, are left out.
export function parseListing (xml, { DOMParser } = globalThis) {
  const doc = new DOMParser().parseFromString(xml, 'application/xml')
  if (doc.getElementsByTagName('parsererror').length > 0) throw new Error('unreadable listing')
  const scans = []
  for (const response of Array.from(doc.getElementsByTagNameNS(DAV, 'response'))) {
    if (response.getElementsByTagNameNS(DAV, 'collection').length > 0) continue
    const field = (name) => response.getElementsByTagNameNS(DAV, name)[0]?.textContent.trim() ?? ''
    const href = field('href')
    const modified = new Date(field('getlastmodified'))
    scans.push({
      href,
      name: field('displayname') || decodeURIComponent(href.split('/').pop()),
      size: Number(field('getcontentlength')),
      modified: Number.isNaN(modified.getTime()) ? null : modified
    })
  }
  // Undated entries sort last.
  const time = (scan) => scan.modified?.getTime() ?? 0
  return scans.sort((a, b) => time(b) - time(a))
}

export function age (scan, now) {
  return distanceInTime(scan.modified, now, { whenNil: 'an unknown time' })
}

// One <li> per scan, matching lambada-web's old page: delete button, a link
// showing the size, and how long ago the scan arrived.
export function renderScans (list, scans, now) {
  const doc = list.ownerDocument
  list.replaceChildren(...scans.map((scan) => {
    const remove = doc.createElement('button')
    remove.type = 'button'
    remove.className = 'delete'
    remove.title = `Delete ${scan.name}`
    remove.dataset.href = scan.href
    remove.textContent = '🗑️'

    const link = doc.createElement('a')
    link.href = scan.href
    link.textContent = `📄 ${humanSize(scan.size)}`

    const when = doc.createElement('span')
    when.textContent = age(scan, now)

    const item = doc.createElement('li')
    item.append(remove, link, when)
    return item
  }))
}

// Wires the page up. `env` is the browser's window; tests pass a stand-in.
export function start (env = globalThis) {
  const { document } = env
  const list = document.getElementById('scans')
  const status = document.getElementById('status')
  let scans = []

  const render = () => {
    renderScans(list, scans, new Date())
    if (!status.dataset.error) status.textContent = scans.length === 0 ? 'No scans yet.' : ''
  }

  const load = async () => {
    try {
      const response = await env.fetch('/', { method: 'PROPFIND', headers: { Depth: '1' } })
      if (!response.ok) throw new Error(`status ${response.status}`)
      scans = parseListing(await response.text(), env)
      delete status.dataset.error
    } catch (error) {
      status.dataset.error = 'true'
      status.textContent = `Couldn't load the scans (${error.message}).`
    }
    render()
  }

  list.addEventListener('click', async (event) => {
    const button = event.target.closest('button.delete')
    const scan = button && scans.find((each) => each.href === button.dataset.href)
    if (!scan || !env.confirm(`Delete this scan from ${age(scan, new Date())}?`)) return
    try {
      const response = await env.fetch(scan.href, { method: 'DELETE' })
      // 404: someone else deleted it first, which is what was wanted anyway.
      if (!response.ok && response.status !== 404) throw new Error(`status ${response.status}`)
    } catch {
      env.alert('Delete failed.')
    }
    await load()
  })

  // Keeps "3 minutes ago" current, and catches scans that arrived while the tab was hidden.
  env.setInterval(render, 30_000)
  document.addEventListener('visibilitychange', () => {
    if (document.visibilityState === 'visible') load()
  })

  return load()
}
