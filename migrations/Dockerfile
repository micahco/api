ARG GO_VERSION=1.24
FROM golang:${GO_VERSION}-bookworm as builder

WORKDIR /usr/src/app
# pulling from the parent dir
COPY ../go.mod ../go.sum ./
RUN go mod download
RUN go mod download && go mod verify
COPY ../ ./
RUN go build -v -o /migrate ./cmd/migrate

FROM debian:bookworm-slim
WORKDIR /app
COPY --from=builder /migrate /migrate

CMD ["/migrate", "-up"]