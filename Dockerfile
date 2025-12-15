# ---- Stage 0 ----
# Builds tool binaries
FROM golang:1.24-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git musl-dev dos2unix

WORKDIR /opt
COPY . /opt

# Run build steps
RUN GOBIN=$PWD/bin go install -v ./cmd/...

# ---- Stage 1 ----
# Final runtime stage.
FROM alpine

ENV PS_DATABASE_MIGRATIONS_DIR=/opt/migrations
ENV PS_HTTP_BIND=0.0.0.0:8080
ENV PS_HTTP_METRICS_BIND=0.0.0.0:8081
ENV PS_HTTP_PPROF_BIND=0.0.0.0:8082
ENV PS_HOMESERVER_SIGNING_KEY_PATH=/data/signing.key
ENV PS_HOMESERVER_EVENT_SIGNING_KEY_PATH=/data/event_signing.key

COPY --from=builder /opt/bin/app /usr/local/bin/
COPY --from=builder /opt/bin/gen_signing_keys /usr/local/bin/
COPY --from=builder /opt/migrations /opt/migrations

CMD /usr/local/bin/app
VOLUME ["/data"]
EXPOSE 8080
