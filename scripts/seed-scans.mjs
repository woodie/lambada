// Fills a folder with a few test scans for `make docker-run` (and zouk or
// huck pointed at it): real one-page PDFs, named like lambada names
// them (Unix time + .pdf), dated minutes to days ago, in a range of sizes.
// Only seeds an empty folder (hidden files like .DS_Store don't count), so
// running it again never piles up duplicates.
//
//   node scripts/seed-scans.mjs [folder]     # default ./scans-dev
import { mkdirSync, readdirSync, writeFileSync, utimesSync } from 'node:fs'
import path from 'node:path'

const folder = path.resolve(process.argv[2] || 'scans-dev')
const minute = 60
const scans = [
  { ago: 3 * minute, bytes: 226_000, label: 'Test scan: 3 minutes ago' },
  { ago: 3 * 60 * minute, bytes: 1_450_000, label: 'Test scan: about 3 hours ago' },
  { ago: 2 * 24 * 60 * minute, bytes: 18_000, label: 'Test scan: 2 days ago' },
  { ago: 9 * 24 * 60 * minute, bytes: 640, label: 'Test scan: 9 days ago' }
]

// A minimal valid PDF showing one enormous `letter` (so thumbnails are easy
// to tell apart) over a small `label`, padded with a PDF comment to about
// `bytes`. The cross-reference offsets are computed, so viewers open it.
function pdf (letter, label, bytes) {
  // Helvetica-Bold's A-Z are 556-944 units wide; 722 (A-D) centers well enough.
  const text = `BT /F2 500 Tf ${(612 - 0.722 * 500) / 2} 260 Td (${letter}) Tj ET\n` +
    `BT /F1 24 Tf 72 120 Td (${label}) Tj ET`
  const objects = [
    '<< /Type /Catalog /Pages 2 0 R >>',
    '<< /Type /Pages /Kids [3 0 R] /Count 1 >>',
    '<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Contents 4 0 R ' +
      '/Resources << /Font << /F1 5 0 R /F2 6 0 R >> >> >>',
    `<< /Length ${text.length} >>\nstream\n${text}\nendstream`,
    '<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>',
    '<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica-Bold >>'
  ]
  const header = '%PDF-1.4\n'
  const padding = Math.max(0, bytes - 700)
  let body = header + (padding ? `%${'x'.repeat(padding)}\n` : '')
  const offsets = objects.map((object, i) => {
    const offset = body.length
    body += `${i + 1} 0 obj\n${object}\nendobj\n`
    return offset
  })
  const xref = body.length
  body += `xref\n0 ${objects.length + 1}\n0000000000 65535 f \n` +
    offsets.map((offset) => `${String(offset).padStart(10, '0')} 00000 n \n`).join('') +
    `trailer\n<< /Size ${objects.length + 1} /Root 1 0 R >>\nstartxref\n${xref}\n%%EOF\n`
  return body
}

mkdirSync(folder, { recursive: true })
const existing = readdirSync(folder).filter((name) => !name.startsWith('.'))
if (existing.length > 0) {
  console.log(`${path.relative(process.cwd(), folder)} already has ${existing.length} file(s); not seeding.`)
  process.exit(0)
}
const now = Math.floor(Date.now() / 1000)
scans.forEach(({ ago, bytes, label }, i) => {
  const time = now - ago
  const file = path.join(folder, `${time}.pdf`)
  writeFileSync(file, pdf('ABCD'[i], label, bytes))
  utimesSync(file, time, time)
  console.log(`${path.relative(process.cwd(), file)}  ${'ABCD'[i]}  (${label.replace('Test scan: ', '')})`)
})
