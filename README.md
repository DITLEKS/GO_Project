# GO_Project

Дисциплина «Язык Go», темы 1–9. Требуются Go 1.22+, PostgreSQL 16+, Redis 7+.

## Архитектура

Два модуля: `gateway/` — HTTP-сервер :8080, `ledger/` — доменная логика и хранение. Gateway импортирует модуль `ledger` через локальный replace. **Данные только в PostgreSQL**, отчёты кешируются в Redis на 30 секунд. `ledger/migrations/` — миграции Goose, `compose.yml` — локальные зависимости, `.github/workflows/test.yml` — CI с реальными PostgreSQL/Redis.

С темы 8 Ledger разделён на слои, зависимости направлены внутрь, к домену:

```
gateway/
  main.go                    получает сервис из фабрики и передаёт его в api.NewRouter
  internal/api/              HTTP: DTO, обработчики, middleware; знает только ledger.Service
ledger/
  *.go                       публичный фасад: псевдонимы типов, интерфейс Service, фабрика NewService
  internal/domain/           сущности, Validate, контракты хранения (без SQL и HTTP)
  internal/repository/pg/    реализация контрактов на PostgreSQL (весь SQL здесь)
  internal/service/          бизнес-логика: валидация, правило бюджета, отчёт и его кеш
  internal/app/              сборка: env → sql.Open → Ping → репозитории → сервис
  internal/cache/            клиент Redis для кеша отчётов
  migrations/                миграции Goose
```

- домен не зависит от репозиториев и HTTP;
- репозитории зависят от домена, но не от Gateway;
- сервис зависит от домена, но не от реализации хранения;
- слой сборки (`internal/app`) знает реализации и собирает сервис;
- Gateway зависит от интерфейса сервиса и фабрики; `database/sql`, pgx и `repository/*` не импортирует.

Фасад в корне `ledger/` нужен из-за правила Go: пакеты `ledger/internal/...` нельзя импортировать из другого модуля. Логики в фасаде нет. Подробности — в [ARCHITECTURE.md](ARCHITECTURE.md).

### Инициализация сервиса в Gateway

```go
svc, closeFn, err := ledger.NewService(ctx) // env → sql.Open → Ping(ctx) → репозитории → сервис
if err != nil {
    return err // main логирует ошибку и завершает работу
}
defer closeFn()
srv := &http.Server{Addr: ":8080", Handler: api.NewRouter(svc)}
```

Фабрика возвращает `(Service, func() error, error)` и не запускает миграции. Обработчики вызывают методы `svc`: `SetBudget`, `ListBudgets`, `AddTransaction`, `ListTransactions`, `ReportSummary`.

Ledger пока библиотека, не отдельный HTTP/RPC-сервис. `ledger/cmd/server` отдельно проверяет готовность инфраструктуры; Gateway не обращается к нему по сети.

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
| GATEWAY_REQUEST_TIMEOUT | Таймаут одного API-запроса, по умолчанию 2s; см. «Таймауты запросов» |

`.env` автоматически не читается, экспортируйте как выше. Реальные пароли не коммитьте. Переменные БД и Redis читает фабрика Ledger (`internal/app`), таймаут запроса — Gateway. При старте: sql.Open("pgx"), Ping, проверка схемы, Redis Ping; ошибка завершает приложение. Пул PostgreSQL: MaxOpenConns=10, MaxIdleConns=5, ConnMaxLifetime=30 минут. Redis-сбой после старта не блокирует SQL-отчёт (fail-open).

## HTTP API

| Метод | Путь | Ответ |
|---|---|---|
| GET | /ping | 200, {"message":"pong"} |
| POST | /api/budgets | UPSERT, 201 и бюджет |
| GET | /api/budgets | 200, массив по category |
| POST | /api/transactions | 201 и запись с SQL ID; 409 при превышении |
| GET | /api/transactions | 200, массив по date DESC, id DESC |
| GET | /api/reports/summary?from=YYYY-MM-DD&to=YYYY-MM-DD | 200, отчёт за включительный интервал |

Все ответы: `application/json; charset=utf-8`, ошибки `{"error":"…"}`. Валидация/JSON → 400, превышение → 409 `budget exceeded`, таймаут или отмена запроса → 504 `request timeout`, SQL → 500 без раскрытия SQL клиенту, неизвестный путь → 404, метод → 405. Пустые списки → []. Middleware логирует метод, путь, длительность.

Прежние пути/DTO сохранены: обработчики переводят ошибки сервиса в прежние ответы (`ValidationError` → 400 с тем же текстом, `ErrBudgetExceeded` → 409). В теме 9 добавлен только код 504.

### Отчёт по категориям

`GET /api/reports/summary?from=YYYY-MM-DD&to=YYYY-MM-DD` — сводка за период, обе границы включаются.

```bash
curl -i 'localhost:8080/api/reports/summary?from=2026-11-01&to=2026-11-30'
```

```json
{
  "from": "2026-11-01",
  "to": "2026-11-30",
  "income": 50000,
  "expenses": 3789,
  "balance": 46211,
  "categories": [
    {"category": "еда", "total": -3000},
    {"category": "зарплата", "total": 50000},
    {"category": "транспорт", "total": -789}
  ]
}
```

`categories` — суммы по категориям со знаком (расход отрицательный), по алфавиту; `expenses` — положительный объём расходов. Формат ответа остался прежним, как в теме 7: это массив объектов, а не объект вида `{"еда": 1234.56}`.

| Ситуация | Ответ |
|---|---|
| Нет `from`/`to`, дата не в формате YYYY-MM-DD, `from` позже `to` | 400, `{"error":"from/to must be YYYY-MM-DD and from <= to"}` |
| Таймаут или отмена запроса | 504, `{"error":"request timeout"}` |
| Метод не GET | 405, `{"error":"method not allowed"}` |

Как считается отчёт (`ledger/internal/service/report.go`):

1. Сервис получает список категорий за период (`SELECT DISTINCT category`).
2. Для каждой категории запускается отдельная горутина, которая считает суммы этой категории; завершение ожидается через `sync.WaitGroup`.
3. Каждая горутина пишет только в свой элемент заранее созданного среза, общего изменяемого состояния нет. Результаты объединяются в одном месте после `Wait()`.
4. Пока идёт расчёт, фоновая горутина с `time.Ticker` раз в 300 мс пишет в лог `report: calculation in progress`. По завершении расчёта или по отмене контекста тикер останавливается, а сервис дожидается выхода этой горутины.
5. `ctx` из запроса передаётся во все обращения к БД (`QueryContext`/`QueryRowContext`/`ExecContext`), поэтому отмена прерывает SQL-операции. Вызовов без контекста в сервисе и репозиториях нет.

Redis-ключ: `report:summary:<from>:<to>:v<generation>`, TTL 30 секунд. При попадании в кеш расчёт не запускается (в логе `report cache hit`). Запись транзакции меняет поколение всех интервалов; старые ключи истекают по TTL. При сбое инвалидации устаревание ограничено TTL. Разным PostgreSQL-базам/тестам нужен отдельный REDIS_DB. Бюджеты не кешируются.

## Таймауты запросов

Middleware `Timeout` (`gateway/internal/api/middleware.go`) оборачивает все маршруты `/api/*`: для каждого запроса создаётся `context.WithTimeout`, контекст передаётся в обработчик через `*http.Request`, а из него — в сервис и SQL-запросы. `/ping` таймаутом не ограничен.

| Параметр | Значение |
|---|---|
| По умолчанию | 2 секунды (константа `DefaultRequestTimeout`) |
| Как изменить | переменная окружения `GATEWAY_REQUEST_TIMEOUT` в формате Go duration: `500ms`, `3s`, `1m` |
| Некорректное или неположительное значение | игнорируется, используется значение по умолчанию; в лог пишется предупреждение |
| Ответ при превышении | **504 Gateway Timeout**, `{"error":"request timeout"}` |

Тот же ответ 504 возвращается, если запрос отменён по другой причине (`ctx.Done()`). Внутри сервиса у операций есть собственный предел 5 секунд; действует меньший из двух.

Проверка малого таймаута:

```bash
(cd gateway && GATEWAY_REQUEST_TIMEOUT=1ms go run .)
curl -i 'localhost:8080/api/reports/summary?from=2026-11-01&to=2026-11-30'
# HTTP/1.1 504 Gateway Timeout
# {"error":"request timeout"}
```

В логе сервера при этом видно, что расчёт прерван по контексту: `report: calculation interrupted: context deadline exceeded` и, если расчёт успел начаться, `report: heartbeat stopped: context deadline exceeded`. При таймауте 1 мс запрос обычно обрывается ещё до запуска горутин, поэтому второй строки может не быть; на 50 мс и медленной БД она появляется.

### Правила

Как в темах 2–6: **отрицательная сумма — расход, положительная — доход**. Доход не уменьшает потраченный бюджет, категория без бюджета не ограничивается. Правило «потрачено + новая сумма ≤ лимит» проверяет сервис; проверка и INSERT атомарны: они выполняются в одной SQL-транзакции с advisory lock категории, SetBudget использует тот же lock. Бюджет накопительный, period — информационное поле, не месячный фильтр.

Validate ничего не логирует; сервис вызывает его перед записью. Деньги округляются до копейки, NUMERIC(14,2); ноль/NaN/Inf, суммы вне диапазона и округляющиеся в ноль отклоняются. Пустая дата становится сегодняшней UTC-датой; непустая — строго YYYY-MM-DD.

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
# 400: некорректные даты
curl -i 'localhost:8080/api/reports/summary?from=01.11.2026&to=2026-11-30'
```

### Перезапуск

Сохраните ответы GET, остановите Gateway (Ctrl+C), снова запустите `go run .` из gateway, повторите GET: строки и ID останутся. Перезапуск Ledger/`docker compose restart` также сохраняет данные. **docker compose down -v удаляет том и данные**. Reset при старте не вызывается.

## Тесты и покрытие

Независимые table-driven unit-тесты используют t.Run/t.Parallel. Тесты сервиса (`ledger/internal/service`) работают на фейковых репозиториях, без PostgreSQL и Redis: правило бюджета, валидация, конкурентный расчёт отчёта и его прерывание по таймауту контекста. Тесты таймаута Gateway (`gateway/internal/api/timeout_test.go`) используют сервис-заглушку, который ждёт `ctx.Done()`, и проверяют ответ 504 для всех API-маршрутов. SQL/HTTP-тесты — с настоящими PostgreSQL/Redis, без параллельного Reset. Проверяются валидация, UPSERT, 409 без вставки, сортировка, ID, отчёты, TTL, cache hit/invalidation, переподключение, конкурентное соблюдение лимита.

Тесты пакета `ledger` из прошлых тем сохранены: `testmain_test.go` сопоставляет прежние функции (`SetBudget`, `AddTransaction`, ...) с сервисом из фабрики. Тесты Gateway получают сервис через `ledger.NewService`.

**Reset удаляет ВСЕ строки и сбрасывает ID. Только отдельная тестовая БД!** Он не входит в `Service` и доступен тестам через интерфейс `ledger.Resetter`. Без TEST_DATABASE_URL интеграционные тесты явно пропускаются; CI всегда включает их.

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

Замер темы 7 на Go 1.22.12, PostgreSQL 18, Redis 8 (CI: PostgreSQL 16, Redis 7). Он сделан **до** рефакторинга темы 8; после разделения на пакеты цифры изменились, актуальные — в отчёте CI:

| Область | Покрытие statements |
|---|---:|
| ledger | 84.9% |
| ledger/internal/cache | 68.2% |
| Модуль Ledger без cmd/ | 83.6% |
| gateway/internal/api | 86.6% |
| Весь Gateway с main.go | 73.0% |

CI пересчитывает покрытие на каждом push/PR и публикует cover.out/cover.html; его свежие результаты приоритетнее локального замера. Старые 100%/91.6% темы 6 относились к реализации в памяти, не SQL-версии. cmd/ исключён из покрытия, но собирается; main.go Gateway включён в итог.

Прежние задания: корневой `go run . arg1 arg2` (тема 1), `ledger.LoadBudgets(ctx, svc, io.Reader)` (I/O), Validatable/CheckValid (ООП), Gateway DTO/middleware/REST, тесты. Демо: `(cd ledger && go run ./cmd/demo)` — **записывает примеры в БД**, только для учебной базы. Подробности — в README сервисов.
