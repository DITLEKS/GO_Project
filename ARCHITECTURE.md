# Архитектура (тема 8)

Ledger разделён на слои; зависимости направлены внутрь, к домену.

```
gateway/
  main.go                    вызывает ledger.NewService(ctx), defer closeFn(), передаёт сервис в api.NewRouter
  internal/api/              HTTP: DTO, обработчики, middleware; знает только ledger.Service
ledger/
  service.go, budget.go ...  публичный фасад: псевдонимы типов, интерфейс Service, фабрика NewService
  internal/domain/           сущности (Transaction, Budget, отчёт), Validate, контракты хранения
  internal/repository/pg/    реализация контрактов на PostgreSQL (весь SQL здесь)
  internal/service/          бизнес-логика: валидация, правило бюджета, отчёт и его кеш
  internal/app/              сборка: env → sql.Open → Ping → репозитории → сервис
  internal/cache/            клиент Redis для кеша отчётов
  migrations/                миграции Goose (фабрика их не запускает)
```

## Кто от кого зависит

| Пакет | Импортирует | Не импортирует |
|---|---|---|
| internal/domain | только стандартную библиотеку | database/sql, net/http, остальные пакеты проекта |
| internal/repository/pg | domain, database/sql | service, app, Gateway, net/http |
| internal/service | domain | repository/pg, database/sql, net/http |
| internal/app | repository/pg, service, cache, драйвер pgx | Gateway |
| ledger (фасад) | domain, service, app | — |
| gateway | ledger (фасад) | database/sql, pgx, ledger/internal/... |

Фасад нужен из-за правила Go: пакеты `ledger/internal/...` нельзя импортировать из другого модуля, а Gateway — отдельный модуль. Логики в фасаде нет.

## Контракты домена

- `BudgetRepository`: `Upsert`, `GetByCategory`, `List`.
- `ExpenseRepository`: `Add`, `List`, `SumByCategory`, `TotalsByCategory` (для отчёта).
- `CategoryLocker`: `WithCategoryLock` — выполняет функцию атомарно под блокировкой категории. В PostgreSQL это одна SQL-транзакция с advisory lock; транзакция передаётся репозиториям через `context`.
- `ReportCache`: кеш отчётов (Redis), `Purger`: очистка данных для тестов.

## Сервис

`service.Service` — интерфейс для Gateway: `SetBudget`, `ListBudgets`, `AddTransaction`, `ListTransactions`, `ReportSummary`.

`AddTransaction`: валидация сущности → нормализация (trim категории, округление до копейки, дата по умолчанию) → под блокировкой категории: для расхода с бюджетом проверка «потрачено + новая сумма ≤ лимит» (в целых копейках) → вставка → инвалидация кеша отчётов. Ошибки валидации сервис возвращает как `ValidationError`, превышение — как `ErrBudgetExceeded`.

## Инициализация в Gateway

```go
svc, closeFn, err := ledger.NewService(ctx) // env → sql.Open → Ping(ctx) → репозитории → сервис
if err != nil {
    return err // main логирует и завершает работу
}
defer closeFn()
srv := &http.Server{Addr: ":8080", Handler: api.NewRouter(svc)}
```

Фабрика возвращает `(Service, func() error, error)`, миграции не запускает. Обработчики вызывают методы `svc` и переводят ошибки в прежние ответы: `ValidationError` → 400 с тем же текстом, `ErrBudgetExceeded` → 409 `budget exceeded`, остальное → 500 `internal server error`. Пути, DTO и формат `{"error":"…"}` не менялись.

## Тесты

- `ledger/internal/service/service_test.go` — правило бюджета и валидация на фейковых репозиториях, без PostgreSQL и Redis.
- Тесты пакета `ledger` из прошлых тем не переписаны: `testmain_test.go` сопоставляет прежние функции (`SetBudget`, `AddTransaction`, ...) с сервисом из фабрики.
- Тесты Gateway получают сервис через `ledger.NewService`; очистка БД — через `ledger.Resetter` (в `Service` не входит).
