FROM golang:1.16 AS build

WORKDIR /app

COPY go.mod .
COPY go.sum .

RUN go mod download

COPY . .

RUN mkdir bin \
    && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o bin/hath cli/hath/*.go

FROM alpine

RUN apk --no-cache add ca-certificates

WORKDIR /root

COPY --from=build /app/bin .

CMD ["./hath"]