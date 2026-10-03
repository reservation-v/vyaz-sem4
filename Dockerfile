# Многоступенчатая сборка: компиляция в golang-образе, запуск в чистом alpine.
# Статика и шаблоны встроены в бинарник через embed — в образ попадает один файл.

FROM golang:1.24-alpine AS build

WORKDIR /src

COPY go.mod ./
COPY cmd ./cmd
COPY internal ./internal

RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/coworking-server ./cmd/server

FROM alpine:3.20

# ca-certificates — для будущих внешних запросов (RSS в лабе 4).
RUN apk add --no-cache ca-certificates && adduser -D -u 10001 app

USER app
COPY --from=build /out/coworking-server /usr/local/bin/coworking-server

ENV HTTP_ADDR=:8081
EXPOSE 8081

ENTRYPOINT ["/usr/local/bin/coworking-server"]