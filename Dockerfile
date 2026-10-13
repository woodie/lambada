# The web side of lambada, set up the way the Pi runs it: nginx WebDAV
# (service/lambada.nginx.conf) on port 80, serving the scans and the scan
# page. Debian bookworm, with the same nginx packages as Raspberry Pi OS.
# lambada itself (the MTA) isn't included; scans come from a folder you mount. For
# working on the page on a Mac; see `make docker-run`.
FROM debian:bookworm-slim
RUN apt-get update \
 && apt-get install -y --no-install-recommends nginx libnginx-mod-http-dav-ext \
 && rm -rf /var/lib/apt/lists/* /etc/nginx/sites-enabled/default
COPY service/lambada.nginx.conf /etc/nginx/sites-enabled/lambada
COPY public /srv/lambada/public
# Like the Pi: the scans folder is open to nginx's www-data user so deletes work.
RUN mkdir -p /srv/lambada/attachments && chmod o+w /srv/lambada/attachments
EXPOSE 80
CMD ["nginx", "-g", "daemon off;"]
