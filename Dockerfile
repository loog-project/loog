# Runtime image for headless collection / CI. GoReleaser builds the binary and
# provides it in the build context (see dockers: in .goreleaser.yaml), so this
# only needs to copy it into a minimal, non-root base.
FROM gcr.io/distroless/static-debian12:nonroot

COPY loog /usr/local/bin/loog

ENTRYPOINT ["/usr/local/bin/loog"]
