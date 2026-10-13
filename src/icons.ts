// File-type icons and the trash can, from Bootstrap Icons (MIT, see
// public/bootstrap-icons-LICENSE.txt). Vite inlines each SVG into main.js.
import csv from 'bootstrap-icons/icons/filetype-csv.svg?raw'
import doc from 'bootstrap-icons/icons/filetype-doc.svg?raw'
import docx from 'bootstrap-icons/icons/filetype-docx.svg?raw'
import gif from 'bootstrap-icons/icons/filetype-gif.svg?raw'
import heic from 'bootstrap-icons/icons/filetype-heic.svg?raw'
import jpg from 'bootstrap-icons/icons/filetype-jpg.svg?raw'
import pdf from 'bootstrap-icons/icons/filetype-pdf.svg?raw'
import png from 'bootstrap-icons/icons/filetype-png.svg?raw'
import ppt from 'bootstrap-icons/icons/filetype-ppt.svg?raw'
import pptx from 'bootstrap-icons/icons/filetype-pptx.svg?raw'
import txt from 'bootstrap-icons/icons/filetype-txt.svg?raw'
import xls from 'bootstrap-icons/icons/filetype-xls.svg?raw'
import xlsx from 'bootstrap-icons/icons/filetype-xlsx.svg?raw'
import fileRaw from 'bootstrap-icons/icons/file-earmark.svg?raw'
import trashRaw from 'bootstrap-icons/icons/trash3.svg?raw'

// The SVG files are indented; whitespace between tags would show up as text.
const tidy = (svg: string) => svg.replace(/>\s+</g, '><').trim()

export const trash = tidy(trashRaw)

export interface FileIcon {
  kind: string // pdf, image, doc, slides, sheet or file; style.css colors each
  svg: string
}

const icons: Record<string, FileIcon> = {
  pdf: { kind: 'pdf', svg: tidy(pdf) },
  png: { kind: 'image', svg: tidy(png) },
  jpg: { kind: 'image', svg: tidy(jpg) },
  jpeg: { kind: 'image', svg: tidy(jpg) },
  gif: { kind: 'image', svg: tidy(gif) },
  heic: { kind: 'image', svg: tidy(heic) },
  doc: { kind: 'doc', svg: tidy(doc) },
  docx: { kind: 'doc', svg: tidy(docx) },
  txt: { kind: 'doc', svg: tidy(txt) },
  ppt: { kind: 'slides', svg: tidy(ppt) },
  pptx: { kind: 'slides', svg: tidy(pptx) },
  xls: { kind: 'sheet', svg: tidy(xls) },
  xlsx: { kind: 'sheet', svg: tidy(xlsx) },
  csv: { kind: 'sheet', svg: tidy(csv) }
}

// The icon for a file name, by its extension (any case); a plain page otherwise.
export function fileIcon (name: string): FileIcon {
  const dot = name.lastIndexOf('.')
  const extension = dot > 0 ? name.slice(dot + 1).toLowerCase() : ''
  return icons[extension] ?? { kind: 'file', svg: tidy(fileRaw) }
}
