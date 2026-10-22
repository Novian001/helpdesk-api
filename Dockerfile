# Build stage
FROM golang:1.23-alpine AS build

WORKDIR /src

# Download dependencies first so the layer is cached between code changes.
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# CGO is not needed: the binary is fully static.
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/helpdesk-api .

# Runtime stage
FROM alpine:3.20

RUN adduser -D -u 10001 appuser && apk add --no-cache ca-certificates tzdata

WORKDIR /app
COPY --from=build /out/helpdesk-api /app/helpdesk-api

USER appuser
EXPOSE 8080

ENTRYPOINT ["/app/helpdesk-api"]
