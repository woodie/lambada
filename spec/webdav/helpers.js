import { connect } from 'node:net'

// nginx's PROPFIND reply is a small, fixed shape of XML, so a few regular
// expressions read it without pulling in an XML parser just for tests.
export function parseMultistatus (xml) {
  const responses = xml.match(/<D:response>[\s\S]*?<\/D:response>/g) || []
  const field = (block, tag) => (block.match(new RegExp(`<D:${tag}>([^<]*)</D:${tag}>`)) || [])[1]
  return responses.map((block) => ({
    href: field(block, 'href'),
    name: field(block, 'displayname'),
    size: Number(field(block, 'getcontentlength')),
    modified: new Date(field(block, 'getlastmodified')),
    isFolder: block.includes('<D:collection/>')
  }))
}

export async function propfind (url) {
  const response = await fetch(url, { method: 'PROPFIND', headers: { Depth: '1' } })
  return { status: response.status, entries: parseMultistatus(await response.text()) }
}

// Speaks just enough SMTP to hand lambada-mta one message, the way a
// scanner would: a multipart email with the scan as a base64 attachment.
export function emailScan ({ port, filename, content }) {
  const boundary = 'lambada-test-boundary'
  const message = [
    'From: scanner@example.com',
    'To: scans@example.com',
    'Subject: Scan',
    'MIME-Version: 1.0',
    `Content-Type: multipart/mixed; boundary="${boundary}"`,
    '',
    `--${boundary}`,
    'Content-Type: text/plain',
    '',
    'Scanned document attached.',
    `--${boundary}`,
    'Content-Type: application/pdf',
    `Content-Disposition: attachment; filename="${filename}"`,
    'Content-Transfer-Encoding: base64',
    '',
    content.toString('base64').replace(/.{76}/g, '$&\r\n'),
    `--${boundary}--`,
    ''
  ].join('\r\n')

  const commands = ['EHLO scanner.local', 'MAIL FROM:<scanner@example.com>', 'RCPT TO:<scans@example.com>', 'DATA', `${message}\r\n.`, 'QUIT']
  return new Promise((resolve, reject) => {
    const socket = connect(port, '127.0.0.1')
    let buffer = ''
    socket.setEncoding('utf8')
    socket.on('data', (chunk) => {
      buffer += chunk
      const lines = buffer.split('\r\n').filter(Boolean)
      const last = lines[lines.length - 1] || ''
      if (!/^\d{3} /.test(last)) return
      buffer = ''
      if (/^[45]/.test(last)) { socket.destroy(); reject(new Error(`SMTP error: ${last}`)); return }
      const next = commands.shift()
      if (next === undefined) return
      socket.write(`${next}\r\n`)
    })
    socket.on('close', () => resolve())
    socket.on('error', reject)
  })
}
