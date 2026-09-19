FROM golang:1.24-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /internship-api ./cmd/api

FROM alpine:3.21

RUN apk add --no-cache ca-certificates
COPY --from=builder /internship-api /usr/local/bin/internship-api

EXPOSE 8080

CMD ["internship-api"]

