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
ENV PS_METRICS_BIND=0.0.0.0:8081
ENV PS_HOMESERVER_SIGNING_KEY_PATH=/data/signing.key

# We (dangerously) use the same signing key for both event signing and server ops
ENV PS_HOMESERVER_EVENT_SIGNING_KEY_PATH=/data/signing.key

COPY --from=builder /opt/bin/app /usr/local/bin/
COPY --from=builder /opt/migrations /opt/migrations

# These are the main changes from the base Dockerfile:
# - we add the demo CA to the trust store.
# - we add the demo signing key to the image
RUN apk add --no-cache ca-certificates
COPY dev/demo/ca/certs/ca.crt /usr/local/share/ca-certificates/myca.crt
RUN update-ca-certificates
COPY dev/demo/policyserv/demo.signing.key /data/demo.signing.key

CMD /usr/local/bin/app
VOLUME ["/data"]
EXPOSE 8080
