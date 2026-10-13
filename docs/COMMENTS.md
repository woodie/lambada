# Comments

Rationale that's too long for a comment in the source. Organized by file,
then by what each note is attached to. The source carries at most one short
line at any given spot.

## attachments.go

### `package main` (file comment)
A Go port of scandalous's `lib/scan_files.rb` (the save and cleanup side)
plus the MIME parsing `mta.rb` got from Ruby's `mail` gem. Nothing here knows
about SMTP; `main.go` calls in.

### `attachmentDir`, `maxFileAge` (var)
`attachmentDir` defaults to `./attachments` so a plain build runs from a
checkout with no setup. Under systemd, `LAMBADA_ATTACHMENTS_DIR` points it at
`/srv/lambada/attachments`, the folder nginx serves.

### `scanName` (var)
Cleanup only deletes files lambada itself named (Unix time, ten digits, plus
`.pdf`). The folder is also a general WebDAV share, so uploads, Finder's
hidden files and anything else stay until someone deletes them. An upload
with a scan's name (usually a downloaded scan put back) counts as a scan and
expires 24 hours after it's uploaded; refusing it was more confusing than
useful. Ten digits holds until the year 2286.

## main.go

### `package main` (package doc)
A Go port of scandalous's `mta.rb`: an open relay that keeps attachments and
discards everything else. Open on purpose, since old scanners can't
authenticate; keep it on the home network.

### `init` (func)
`LOG_QUIET` keeps `npm run check`'s `go test` output to pass/fail.
