# Builder stage
FROM golang:1.26.1-alpine AS builder

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 go build -o /skip-the-line ./cmd/server

# Final stage
FROM gcr.io/distroless/static:nonroot

COPY --from=builder /skip-the-line /skip-the-line

EXPOSE 8080

USER nonroot:nonroot

ENTRYPOINT ["/skip-the-line"]
