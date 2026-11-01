# GO_Project

Дисциплина «Язык Go», темы 1–7. Требуются Go 1.22+, PostgreSQL 16+, Redis 7+.

## Архитектура

`gateway/` — HTTP-сервер :8080, импортирует модуль `ledger` через локальный replace. `ledger/` — валидация и бизнес-логика; **данные только в PostgreSQL**, отчёты кешируются в Redis на 30 секунд. `ledger/migrations/` — миграции Goose, `compose.yml` — локальные зависимости, `.github/workflows/test.yml` — CI с реальными PostgreSQL/Redis.

Ledger пока библиотека, не отдельный HTTP/RPC-сервис. Gateway открывает собственный пул Ledger. `ledger/cmd/server` отдельно проверяет готовность инфраструктуры; Gateway не обращается к нему по сети.

## Запуск и миграции

Из корня:

```bash
docker compose up -d --wait
cp .env.example .env
set -a; . ./.env; set +a
go install -tags='no_clickhouse no_libsql no_mssql no_mysql no_sqlite3 no_vertica no_ydb' github.com/pressly/goose/v3/cmd/goose@v3.24.1
export PATH="$(go env GOPATH)/bin:$PATH"
goose -dir ledger/migrations postgres "$DATABASE_URL" up
(cd gateway && go run .)
```

Без Docker: создайте PostgreSQL-базу `createdb -h localhost -U postgres cashapp`, запустите Redis локально. Применяйте миграции **до** запуска. Проверка/откат:

```bash
goose -dir ledger/migrations postgres "$DATABASE_URL" status
goose -dir ledger/migrations postgres "$DATABASE_URL" down
# Отдельный Ledger, с теми же env:
(cd ledger && go run ./cmd/server)
```

Каждая SQL-миграция содержит обе секции Goose Up/Down. `down` откатывает последнюю, `down-to 0` — все; откат удаляет таблицы/данные. `budgets`: UNIQUE category, NUMERIC(14,2) limit_amount > 0, дополнительный period для совместимости. `expenses`: SERIAL id, NUMERIC(14,2) amount <> 0, DATE NOT NULL, индекс (category,date).

## Переменные окружения

| Переменная | Значение/назначение |
|---|---|
| DATABASE_URL | Приоритетная строка PostgreSQL; пример в .env.example |
| DB_HOST, DB_PORT | localhost, 5432; только если нет DATABASE_URL |
| DB_USER, DB_PASS, DB_NAME | postgres, postgres, cashapp; fallback DSN с экранированием |
| REDIS_ADDR | localhost:6379 |
| REDIS_DB | 0, неотрицательное целое |
| REDIS_PASSWORD | Необязательный пароль |

`.env` автоматически не читается, экспортируйте как выше. Реальные пароли не коммитьте. При старте: sql.Open("pgx"), Ping, проверка схемы, Redis Ping; ошибка завершает приложение. Пул PostgreSQL: MaxOpenConns=10, MaxIdleConns=5, ConnMaxLifetime=30 минут. SQL-операции имеют таймаут. Redis-сбой после старта не блокирует SQL-отчёт (fail-open).

## HTTP API

| Метод | Путь | Ответ |
|---|---|---|
| GET | /ping | 200, {"message":"pong"} |
| POST | /api/budgets | UPSERT, 201 и бюджет |
| GET | /api/budgets | 200, массив по category |
| POST | /api/transactions | 201 и запись с SQL ID; 409 при превышении |
| GET | /api/transactions | 200, массив по date DESC, id DESC |
| GET | /api/reports/summary?from=YYYY-MM-DD&to=YYYY-MM-DD | 200, отчёт за включительный интервал |

Все ответы: `application/json; charset=utf-8`, ошибки `{"error":"…"}`. Валидация/JSON → 400, превышение → 409 `budget exceeded`, SQL → 500 без раскрытия SQL клиенту, неизвестный путь → 404, метод → 405. Пустые списки → []. Middleware логирует метод, путь, длительность.

Прежние пути/DTO сохранены. Gateway минимально адаптирован к `(slice,error)` и точному SQL ID; добавлен ранее отсутствовавший маршрут отчёта.

### Правила

Как в темах 2–6: **отрицательная сумма — расход, положительная — доход**. Доход не уменьшает потраченный бюджет, категория без бюджета не ограничивается. Проверка `SUM(-amount) WHERE amount<0` и INSERT атомарны с advisory lock категории; SetBudget использует тот же lock. Бюджет накопительный, period — информационное поле, не месячный фильтр.

Validate ничего не логирует; вызывается перед записью. Деньги округляются до копейки, NUMERIC(14,2); ноль/NaN/Inf, суммы вне диапазона и округляющиеся в ноль отклоняются. Пустая дата становится сегодняшней UTC-датой; непустая — строго YYYY-MM-DD.

### Проверка cURL

```bash
# 201
curl -i -X POST localhost:8080/api/budgets -H 'Content-Type: application/json' -d '{"category":"еда","limit":5000}'
# 201
curl -i -X POST localhost:8080/api/transactions -H 'Content-Type: application/json' -d '{"amount":-3000,"category":"еда","description":"продукты","date":"2026-11-01"}'
# 409, вставки нет: 3000+2500 > 5000
curl -i -X POST localhost:8080/api/transactions -H 'Content-Type: application/json' -d '{"amount":-2500,"category":"еда","date":"2026-11-02"}'
curl -i localhost:8080/api/transactions
curl -i localhost:8080/api/budgets
# cache miss, затем cache hit (в консоли сервера)
curl -i 'localhost:8080/api/reports/summary?from=2026-11-01&to=2026-11-30'
curl -i 'localhost:8080/api/reports/summary?from=2026-11-01&to=2026-11-30'
```

Отчёт содержит from, to, income, expenses (положительный объём расходов), balance, categories (знаковые суммы). Redis-ключ: `report:summary:<from>:<to>:v<generation>`, TTL 30 секунд. Запись транзакции меняет поколение всех интервалов; старые ключи истекают по TTL. При сбое инвалидации устаревание ограничено TTL. Разным PostgreSQL-базам/тестам нужен отдельный REDIS_DB. Бюджеты не кешируются.

### Перезапуск

Сохраните ответы GET, остановите Gateway (Ctrl+C), снова запустите `go run .` из gateway, повторите GET: строки и ID останутся. Перезапуск Ledger/`docker compose restart` также сохраняет данные. **docker compose down -v удаляет том и данные**. Reset при старте не вызывается.

## Тесты и покрытие

Независимые table-driven unit-тесты используют t.Run/t.Parallel. SQL/HTTP-тесты — с настоящими PostgreSQL/Redis, без параллельного Reset. Проверяются валидация, UPSERT, 409 без вставки, сортировка, ID, отчёты, TTL, cache hit/invalidation, переподключение, конкурентное соблюдение лимита.

**Reset() удаляет ВСЕ строки и сбрасывает ID. Только отдельная тестовая БД!** Без TEST_DATABASE_URL интеграционные тесты явно пропускаются; CI всегда включает их.

```bash
# Из корня, зависимости уже запущены:
createdb -h localhost -U postgres cashapp_test
export TEST_DATABASE_URL='postgres://postgres:postgres@localhost:5432/cashapp_test?sslmode=disable'
export REDIS_ADDR=localhost:6379 REDIS_DB=15
goose -dir ledger/migrations postgres "$TEST_DATABASE_URL" up
# Последовательно, т.к. тесты очищают одну БД:
for module in ledger gateway; do
  (
    cd "$module"
    go vet ./...
    go build ./...
    go test -count=1 -race -cover -coverprofile=cover.out $(go list ./... | grep -v /cmd/)
    go tool cover -func=cover.out | tail -n 1
    go tool cover -html=cover.out -o cover.html
  ) || exit 1
done
```

Замер темы 7 на Go 1.22.12, PostgreSQL 18, Redis 8 (CI: PostgreSQL 16, Redis 7):

| Область | Покрытие statements |
|---|---:|
| ledger | 84.9% |
| ledger/internal/cache | 68.2% |
| Модуль Ledger без cmd/ | 83.6% |
| gateway/internal/api | 86.6% |
| Весь Gateway с main.go | 73.0% |

`go vet`, сборка и race-тесты прошли. Дополнительно проверены Goose up/down-to 0/up, HTTP 201/201/409, cache miss → hit и **настоящий перезапуск процесса Gateway** с сохранением GET/ID. CI пересчитывает покрытие на каждом push/PR и публикует cover.out/cover.html; его свежие результаты приоритетнее локального замера. Старые 100%/91.6% темы 6 относились к реализации в памяти, не SQL-версии. cmd/ исключён из покрытия, но собирается; main.go Gateway включён в итог.

Прежние задания: корневой `go run . arg1 arg2` (тема 1), `ledger.LoadBudgets(io.Reader)` (I/O), Validatable/CheckValid (ООП), Gateway DTO/middleware/REST, тесты. Демо: `(cd ledger && go run ./cmd/demo)` — **записывает примеры в БД**, только для учебной базы. Подробности — в README сервисов.
