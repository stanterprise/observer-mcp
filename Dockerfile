FROM golang:1.25-alpine AS builder
WORKDIR /src
ARG TARGETOS
ARG TARGETARCH
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-amd64} go build -o /out/observer-mcp ./cmd/server

FROM gcr.io/distroless/static:nonroot
COPY --from=builder /out/observer-mcp /usr/local/bin/observer-mcp
ENTRYPOINT ["/usr/local/bin/observer-mcp"]
