# ---- Build stage ----
FROM golang:1.26-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o /page_pulse .

# ---- Runtime stage ----
FROM alpine:3.22

# The app fetches arbitrary user-supplied https:// URLs, so it needs a CA
# bundle to verify TLS certificates - without this, every https audit
# would fail with "x509: certificate signed by unknown authority".
RUN apk add --no-cache ca-certificates && \
    addgroup -S app && adduser -S app -G app

WORKDIR /app

COPY --from=builder /page_pulse .

USER app

EXPOSE 8000

ENTRYPOINT ["./page_pulse"]
