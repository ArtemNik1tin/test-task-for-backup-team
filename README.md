# DNS Manager

Клиент-серверное приложение для управления DNS-серверами через `/etc/resolv.conf`.

## Quick Start

```bash
# Build
go build -o dns-manager-server ./api/cmd/server
go build -o dns-cli ./cmd/client

# Run server (default :8080)
./dns-manager-server

# Use CLI
./dns-cli list
./dns-cli add 8.8.8.8
./dns-cli del 8.8.8.8
./dns-cli --server http://localhost:8080 list
```

## Architecture

Чистая архитектура (Clean Architecture / Hexagonal) с явным разделением слоёв:

```
api/internal/
  domain/       — Entity (Nameserver) + sentinel errors
  usecase/      — Business logic + port interfaces (DNSReader/DNSWriter/DNSManager)
  adapter/
    resolver/   — Outbound adapter: read/write resolv.conf
  controller/
    http/       — Inbound adapter: HTTP handlers + middleware + composition root
  dto/          — Request/response DTOs
```

**Dependency Rule**: все слои зависят только к центру (`domain`). Внешние адаптеры реализуют порты, объявленные в `usecase`.

## Highlights for Reviewers

- **Чистая архитектура** — бизнес-логика (`usecase`) не зависит от фреймворков и адаптеров. Порты объявлены как интерфейсы, HTTP-слой работает через свой `DNSUseCase` интерфейс (DIP)
- **Production-grade logging** — structured JSON-logger (slog) с middleware для трейсинга запросов (method, path, status, duration)
- **Graceful shutdown** — сервер корректно обрабатывает SIGINT/SIGTERM, доводя текущие запросы до завершения
- **Атомарный rewrite** — файл resolv.conf перезаписывается через temp-file + rename, данные не теряются при сбое
- **Конкурентный доступ** — sync.Mutex оборачивает все операции чтения/записи resolv.conf
- **Idempotent add** — повторное добавление существующего NS-сервера не приводит к дубликату
- **Обратная совместимость** — парсер корректно читает комментарии (`#`, `;`), пустые строки, не затрагивает чужие директивы

### Тесты

| Пакет | Тестов | Покрытие |
|-------|--------|----------|
| `domain` | 12 | 100% |
| `usecase` | 10 | 100% |
| `adapter/resolver` | 20 | 83% |
| `controller/http` (unit) | 17 | — |
| `controller/http` (integration) | 8 | — |

Каждый пакет имеет unit-тесты, HTTP-слой дополнительно покрыт интеграционными тестами (полный стек через httptest + temp-файл). Все тесты проходят с `-race`.

### Линтер

```bash
golangci-lint run   # 0 issues, 55 linters enabled
```

## Server Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `LISTEN_ADDR` | `:8080` | Server listen address |
| `RESOLVE_PATH` | `/etc/resolv.conf` | Path to resolv.conf |
| `LOG_LEVEL` | `info` | Log level (debug/info/warn/error) |

## Project Structure

```
.
├── api/
│   ├── cmd/server/          # Server entry point
│   └── internal/
│       ├── adapter/resolver # resolv.conf adapter
│       ├── controller/http  # HTTP handlers + tests
│       ├── domain           # Nameserver entity
│       ├── dto              # Request/response DTOs
│       └── usecase          # Business logic
├── cmd/client/              # CLI client (Cobra)
├── .golangci.yml            # Linter config (55 linters)
├── TASK.md                  # Original task description
└── README.md
```
