FROM golang:alpine AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /go-mcp-server .

FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata
COPY --from=builder /go-mcp-server /usr/local/bin/go-mcp-server
USER 1001:1001
ENTRYPOINT ["go-mcp-server"]
