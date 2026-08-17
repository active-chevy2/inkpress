FROM golang:1.23-alpine AS builder

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /inkpress ./cmd/inkpress

FROM alpine:3.20

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app

COPY --from=builder /inkpress /app/inkpress
COPY --from=builder /build/web/templates /app/web/templates
COPY --from=builder /build/web/static/css /app/web/static/css
COPY --from=builder /build/web/static/js /app/web/static/js

RUN mkdir -p /app/web/static/uploads

EXPOSE 8080

CMD ["/app/inkpress"]
