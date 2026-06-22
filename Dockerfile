FROM golang:1.26-alpine AS builder

WORKDIR /app/go

COPY go/go.mod go/go.sum ./
RUN go mod download

COPY go/ ./
RUN go build -o /bin/server .

FROM alpine AS latest

WORKDIR /app

COPY --from=builder /bin/server /bin/server
COPY db/migrations /migrations

EXPOSE 8080

CMD ["/bin/server"]
