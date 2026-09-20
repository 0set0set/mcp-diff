FROM golang:1.27.1-alpine3.24@sha256:4cb7ac979db5fcc41cae44b2227ba5ab8a51e8807f40d9ba4dee20a0ad960b5b AS build

WORKDIR /src

COPY go.mod ./
COPY . .

ARG TARGETOS=linux
ARG TARGETARCH
ARG VERSION=dev
RUN CGO_ENABLED=0 GOOS="${TARGETOS}" GOARCH="${TARGETARCH}" \
    go build -trimpath -ldflags="-s -w -X main.version=${VERSION}" -o /out/mcp-diff .

FROM scratch

COPY --from=build /out/mcp-diff /mcp-diff

USER 65532:65532
ENTRYPOINT ["/mcp-diff"]
