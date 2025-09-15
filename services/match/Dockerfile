FROM golang:1.22 AS build
WORKDIR /app

COPY go.mod ./
RUN go mod download

COPY . .
RUN go mod tidy

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/app ./cmd/server

FROM gcr.io/distroless/base-debian12
WORKDIR /
COPY --from=build /out/app /app
USER 65532:65532
ENV PORT=8080
EXPOSE 8080
ENTRYPOINT ["/app"]
