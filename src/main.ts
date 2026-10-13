import { humanSize, distanceInTime } from 'humane-ts'

const DAV = 'DAV:'

// nginx refuses uploads over this (client_max_body_size) and with a scan's
// name, so the page checks first and can say why.
const MAX_UPLOAD = 25 * 1024 * 1024
const SCAN_NAME = /^[0-9]{10}\.pdf$/

export interface Scan {
  href: string
  name: string
  size: number
  modified: Date | null
}

// What the page uses from the browser's window; tests pass a stand-in.
export type Env = Pick<typeof globalThis, 'document' | 'fetch' | 'confirm' | 'alert' | 'setInterval' | 'DOMParser'>

// Reads a WebDAV PROPFIND (Depth: 1) reply into one entry per file, newest
// first. Folders, including the listed folder itself, are left out.
export function parseListing (xml: string, { DOMParser }: Pick<Env, 'DOMParser'> = globalThis): Scan[] {
  const doc = new DOMParser().parseFromString(xml, 'application/xml')
  if (doc.getElementsByTagName('parsererror').length > 0) throw new Error('unreadable listing')
  const scans: Scan[] = []
  for (const response of Array.from(doc.getElementsByTagNameNS(DAV, 'response'))) {
    if (response.getElementsByTagNameNS(DAV, 'collection').length > 0) continue
    const field = (name: string) => response.getElementsByTagNameNS(DAV, name)[0]?.textContent?.trim() ?? ''
    const href = field('href')
    const modified = new Date(field('getlastmodified'))
    scans.push({
      href,
      name: field('displayname') || decodeURIComponent(href.slice(href.lastIndexOf('/') + 1)),
      size: Number(field('getcontentlength')),
      modified: Number.isNaN(modified.getTime()) ? null : modified
    })
  }
  // Undated entries sort last.
  const time = (scan: Scan) => scan.modified?.getTime() ?? 0
  return scans.sort((a, b) => time(b) - time(a))
}

export function age (scan: Scan, now: Date): string {
  return distanceInTime(scan.modified, now, { whenNil: 'an unknown time' })
}

// One <li> per file, like a Finder list with a trash can: delete button, a
// link showing the name, the size, and how long ago the file arrived.
export function renderScans (list: HTMLElement, scans: Scan[], now: Date): void {
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
    link.textContent = `📄 ${scan.name}`

    const size = doc.createElement('span')
    size.className = 'size'
    size.textContent = humanSize(scan.size)

    const when = doc.createElement('span')
    when.className = 'age'
    when.textContent = age(scan, now)

    const item = doc.createElement('li')
    item.append(remove, link, size, when)
    return item
  }))
}

// Wires the page up.
export function start (env: Env = globalThis): Promise<void> {
  const { document } = env
  const list = document.getElementById('scans') as HTMLElement
  const status = document.getElementById('status') as HTMLElement
  const picker = document.getElementById('upload') as HTMLInputElement
  const pickerButton = document.getElementById('upload-button') as HTMLElement
  let scans: Scan[] = []

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
      status.textContent = `Couldn't load the scans (${(error as Error).message}).`
    }
    render()
  }

  list.addEventListener('click', async (event) => {
    const button = (event.target as Element).closest<HTMLElement>('button.delete')
    const scan = button && scans.find((each) => each.href === button.dataset.href)
    if (!scan || !env.confirm(`Delete ${scan.name}?`)) return
    try {
      const response = await env.fetch(scan.href, { method: 'DELETE' })
      // 404: someone else deleted it first, which is what was wanted anyway.
      if (!response.ok && response.status !== 404) throw new Error(`status ${response.status}`)
    } catch {
      env.alert('Delete failed.')
    }
    await load()
  })

  // PUTs each file under its own name, one at a time, then reloads the list.
  const upload = async (files: File[]) => {
    if (files.length === 0) return
    const failed: string[] = []
    for (const file of files) {
      if (SCAN_NAME.test(file.name)) {
        failed.push(`${file.name} (that name is kept for scans)`)
        continue
      }
      if (file.size > MAX_UPLOAD) {
        failed.push(`${file.name} (over 25 MB)`)
        continue
      }
      if (scans.some((each) => each.name === file.name) && !env.confirm(`Replace ${file.name}?`)) continue
      try {
        const response = await env.fetch(`/${encodeURIComponent(file.name)}`, { method: 'PUT', body: file })
        if (!response.ok) throw new Error(`status ${response.status}`)
      } catch (error) {
        failed.push(`${file.name} (${(error as Error).message})`)
      }
    }
    if (failed.length > 0) env.alert(`Couldn't upload ${failed.join(', ')}.`)
    await load()
  }

  pickerButton.addEventListener('click', () => picker.click())
  picker.addEventListener('change', () => {
    upload(Array.from(picker.files ?? []))
    picker.value = ''
  })

  // Drop files anywhere on the page. The outline shows while dragging over it.
  document.addEventListener('dragover', (event) => {
    event.preventDefault()
    document.body.classList.add('dropping')
  })
  document.addEventListener('dragleave', (event) => {
    if (event.relatedTarget === null) document.body.classList.remove('dropping')
  })
  document.addEventListener('drop', (event) => {
    event.preventDefault()
    document.body.classList.remove('dropping')
    upload(Array.from(event.dataTransfer?.files ?? []))
  })

  // Keeps "3 minutes ago" current, and catches scans that arrived while the tab was hidden.
  env.setInterval(render, 30_000)
  document.addEventListener('visibilitychange', () => {
    if (document.visibilityState === 'visible') load()
  })

  return load()
}
