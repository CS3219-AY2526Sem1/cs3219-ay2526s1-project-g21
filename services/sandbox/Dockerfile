FROM golang:1.24 AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o sandbox ./cmd/server

FROM gcr.io/distroless/base-debian12:latest
WORKDIR /
COPY --from=builder /app/sandbox /sandbox
ENTRYPOINT ["/sandbox"]
