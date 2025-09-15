# syntax=docker/dockerfile:1
FROM golang:1.22 AS build
WORKDIR /app

# pull what's in go.mod (fast cacheable layer)
COPY go.mod ./
RUN go mod download

# now bring in code and resolve any new imports (adds go.sum entries)
COPY . .
RUN go mod tidy

# build
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/app ./cmd/server

FROM gcr.io/distroless/base-debian12
WORKDIR /
COPY --from=build /out/app /app
USER 65532:65532
ENV PORT=8080
EXPOSE 8080
ENTRYPOINT ["/app"]