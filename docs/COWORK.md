# Picking up lambada in a new Cowork session

Context for whoever opens this repo cold, with none of the prior conversation history.

## What this is

Lambada is two minimal Go services for a scanner/printer that requires an open relay to e-mail out scans. Run them on a Raspberry Pi to receive scans by e-mail and browse or download them from your home network.

'lambada-mta' is a minimal SMTP server. The scanner emails scans to the Pi over SMTP; lambada-mta receives the message, decodes the attachment, and saves it with an epoch-based filename (e.g. 1779867473.pdf) to attachments/. Files older than 24 hours are cleaned up on each incoming message.

'lambada-web' serves a listing of those files over HTTP: a page with download links, human-readable file sizes, and "time ago" timestamps, plus a GET /scans.json API used by the zouk Mac client. This is a Go port of the scandalous project's Sinatra web server.

Read `README.md` first for the user-facing description and API
contract. This doc is about *how the project got here* and *how to keep
working on it consistently*.
