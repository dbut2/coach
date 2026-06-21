FROM golang:alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY ./db ./db
COPY ./web ./web
COPY ./go ./go

RUN go build -o /bin/server ./go

FROM alpine

WORKDIR /app

COPY --from=builder /bin/server /bin/server

ARG PORT=8080
EXPOSE ${PORT}

CMD ["/bin/server"]
