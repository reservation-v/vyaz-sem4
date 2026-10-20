# Многоступенчатая сборка: компиляция в golang-образе, запуск в чистом alpine.
# Статика и шаблоны встроены в бинарник через embed - в образ попадает один файл.

FROM golang:1.24-alpine AS build

WORKDIR /src

COPY go.mod go.sum ./
COPY db ./db
COPY docs/docs.html ./docs/docs.html
COPY docs/embed.go ./docs/embed.go
COPY cmd ./cmd
COPY internal ./internal

RUN go mod download && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/coworking-server ./cmd/server

FROM alpine:3.20

# ca-certificates — для будущих внешних запросов (RSS в лабе 4).
RUN apk add --no-cache ca-certificates && adduser -D -u 10001 app

USER app
COPY --from=build /out/coworking-server /usr/local/bin/coworking-server

ENV HTTP_ADDR=:8081
ENV DATABASE_URL=""
EXPOSE 8081

ENTRYPOINT ["/usr/local/bin/coworking-server"]