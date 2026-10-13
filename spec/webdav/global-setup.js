import { spawn, execFileSync } from 'node:child_process'
import { mkdtempSync, mkdirSync, readFileSync, writeFileSync, rmSync, existsSync } from 'node:fs'
import { connect } from 'node:net'
import { tmpdir, userInfo } from 'node:os'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

// Runs the real service/lambada.nginx.conf (with only its folder paths and
// port swapped) and a real lambada-mta, both pointed at one temp folder.
const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../..')
const nginxPort = Number(process.env.WEBDAV_TEST_PORT || 18080)
const smtpPort = 2525
const davExtModule = process.env.NGINX_DAV_EXT_MODULE || '/usr/lib/nginx/modules/ngx_http_dav_ext_module.so'

function siteConfig (attachmentsDir, wwwDir) {
  const original = readFileSync(path.join(root, 'service/lambada.nginx.conf'), 'utf8')
  const swaps = [
    ['root /srv/lambada/attachments;', `root ${attachmentsDir};`],
    ['alias /srv/lambada/www/;', `alias ${wwwDir}/;`],
    ['listen 80 default_server;', `listen 127.0.0.1:${nginxPort} default_server;`],
    ['listen [::]:80 default_server;', '']
  ]
  return swaps.reduce((config, [from, to]) => {
    if (!config.includes(from)) throw new Error(`service/lambada.nginx.conf no longer contains "${from}"; update spec/webdav/global-setup.js`)
    return config.replace(from, to)
  }, original)
}

function mainConfig (workDir) {
  const runningAsRoot = process.getuid?.() === 0
  return [
    `load_module ${davExtModule};`,
    runningAsRoot ? `user ${userInfo().username};` : '',
    `pid ${workDir}/nginx.pid;`,
    'events {}',
    'http {',
    '  types { text/html html; application/javascript js; application/pdf pdf; }',
    '  access_log off;',
    `  client_body_temp_path ${workDir}/tmp;`,
    `  proxy_temp_path ${workDir}/tmp;`,
    `  fastcgi_temp_path ${workDir}/tmp;`,
    `  uwsgi_temp_path ${workDir}/tmp;`,
    `  scgi_temp_path ${workDir}/tmp;`,
    `  include ${workDir}/site.conf;`,
    '}'
  ].join('\n')
}

function waitForPort (port, timeoutMs = 15000) {
  const deadline = Date.now() + timeoutMs
  return new Promise((resolve, reject) => {
    const attempt = () => {
      const socket = connect(port, '127.0.0.1')
      socket.once('connect', () => { socket.end(); resolve() })
      socket.once('error', () => {
        socket.destroy()
        if (Date.now() > deadline) reject(new Error(`nothing listening on port ${port}`))
        else setTimeout(attempt, 100)
      })
    }
    attempt()
  })
}

export default async function ({ provide }) {
  if (!existsSync(davExtModule)) {
    throw new Error(`nginx's WebDAV listing module isn't at ${davExtModule}; install libnginx-mod-http-dav-ext, use \`make test-webdav\` (Docker), or set NGINX_DAV_EXT_MODULE`)
  }

  const workDir = mkdtempSync(path.join(tmpdir(), 'lambada-webdav-'))
  const attachmentsDir = path.join(workDir, 'attachments')
  const wwwDir = path.join(workDir, 'www')
  mkdirSync(attachmentsDir)
  mkdirSync(wwwDir)
  writeFileSync(path.join(wwwDir, 'index.html'), '<!doctype html><title>Scans</title>')
  mkdirSync(path.join(workDir, 'tmp'))
  writeFileSync(path.join(workDir, 'site.conf'), siteConfig(attachmentsDir, wwwDir))
  writeFileSync(path.join(workDir, 'nginx.conf'), mainConfig(workDir))

  const mtaBin = process.env.LAMBADA_MTA_BIN || path.join(workDir, 'lambada-mta')
  if (!process.env.LAMBADA_MTA_BIN) {
    execFileSync('go', ['build', '-o', mtaBin, './cmd/lambada-mta'], { cwd: root, stdio: 'inherit' })
  }

  const nginx = spawn(process.env.NGINX_BIN || 'nginx', [
    '-p', workDir, '-c', path.join(workDir, 'nginx.conf'), '-e', path.join(workDir, 'error.log'), '-g', 'daemon off;'
  ], { stdio: 'inherit' })
  const mta = spawn(mtaBin, [], {
    env: { ...process.env, LAMBADA_ATTACHMENTS_DIR: attachmentsDir, LOG_QUIET: '1' },
    stdio: 'inherit'
  })

  await Promise.all([waitForPort(nginxPort), waitForPort(smtpPort)])
  provide('webdav', { baseUrl: `http://127.0.0.1:${nginxPort}`, attachmentsDir, smtpPort })

  return () => {
    nginx.kill()
    mta.kill()
    rmSync(workDir, { recursive: true, force: true })
  }
}
