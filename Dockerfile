# syntax=docker/dockerfile:1

# ---- Сборка ----
# mattn/go-sqlite3 использует CGO, поэтому собираем в образе с gcc (Debian)
FROM golang:1.26-bookworm AS build
WORKDIR /src

# Сначала зависимости: слой кэшируется, пока go.mod/go.sum не менялись
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=1 go build -trimpath -ldflags="-s -w" -o /out/bot ./cmd/bot

# Каталог для базы SQLite с владельцем nonroot (uid 65532)
RUN mkdir /data

# ---- Запуск ----
# distroless/base: glibc и сертификаты есть, шелла и пакетного менеджера нет
FROM gcr.io/distroless/base-debian12:nonroot
COPY --from=build /out/bot /app/bot
COPY --from=build --chown=65532:65532 /data /data
WORKDIR /data
USER nonroot:nonroot
ENTRYPOINT ["/app/bot"]
