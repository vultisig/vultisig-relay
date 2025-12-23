FROM golang:1.21-alpine AS builder

RUN apk add --no-cache make gcc musl-dev linux-headers git

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o relay ./cmd/router

FROM alpine:latest

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

COPY --from=builder /app/relay /app/relay
