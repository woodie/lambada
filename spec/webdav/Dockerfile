# Debian bookworm, like the Pi, with the same nginx WebDAV packages; plus Go
# (to build lambada) and Node (to run the suite). Used by `make test-webdav`.
FROM golang:1.26-bookworm AS go

FROM node:24-bookworm-slim
RUN apt-get update \
 && apt-get install -y --no-install-recommends nginx libnginx-mod-http-dav-ext git ca-certificates \
 && rm -rf /var/lib/apt/lists/*
COPY --from=go /usr/local/go /usr/local/go
ENV PATH=/usr/local/go/bin:$PATH
WORKDIR /lambada
CMD ["sh", "-c", "echo 'Installing npm packages in the container (a minute or two, quietly)...' && npm install --no-audit --no-fund --loglevel=error && echo 'Building lambada and running the WebDAV tests...' && npm run test-webdav"]
