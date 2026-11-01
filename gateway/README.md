# Gateway: HTTP поверх SQL Ledger

Прежние маршруты и DTO сохранены; Gateway импортирует Ledger через локальный replace и открывает свой SQL-пул. Отдельного сетевого Ledger пока нет. Добавлен GET /api/reports/summary, которого раньше не было.

Из корня после установки Goose (см. корневой README):

```bash
docker compose up -d --wait
cp .env.example .env
set -a; . ./.env; set +a
goose -dir ledger/migrations postgres "$DATABASE_URL" up
(cd gateway && go run .)
# Откат последней миграции: goose -dir ledger/migrations postgres "$DATABASE_URL" down
```

Без Docker создайте БД `createdb -h localhost -U postgres cashapp`, запустите Redis. DATABASE_URL приоритетна, fallback DB_HOST=localhost DB_PORT=5432 DB_USER=postgres DB_PASS=postgres DB_NAME=cashapp. REDIS_ADDR=localhost:6379, REDIS_DB=0, REDIS_PASSWORD опциональна. `.env` экспортируется вручную. Миграции применяются до запуска; ошибка подключения останавливает процесс.

```bash
# 201, затем 201 и SQL ID
curl -i -X POST localhost:8080/api/budgets -H 'Content-Type: application/json' -d '{"category":"еда","limit":5000}'
curl -i -X POST localhost:8080/api/transactions -H 'Content-Type: application/json' -d '{"amount":-3000,"category":"еда","date":"2026-11-01"}'
# 409 budget exceeded, вставки нет
curl -i -X POST localhost:8080/api/transactions -H 'Content-Type: application/json' -d '{"amount":-2500,"category":"еда","date":"2026-11-02"}'
curl -i localhost:8080/api/transactions
curl -i localhost:8080/api/budgets
# cache miss, затем cache hit в логе; TTL 30 секунд
curl -i 'localhost:8080/api/reports/summary?from=2026-11-01&to=2026-11-30'
curl -i 'localhost:8080/api/reports/summary?from=2026-11-01&to=2026-11-30'
```

Валидация → 400, лимит → 409, SQL-ошибка → 500, все ответы JSON с application/json; charset=utf-8. Расходы отрицательные, как в предыдущих заданиях. После Ctrl+C и `go run .` повторите GET: записи сохраняются в PostgreSQL. Docker использует постоянный том; down -v удаляет данные.

Для тестов задайте отдельную TEST_DATABASE_URL и REDIS_DB=15, примените миграции, затем из gateway: `go vet ./...`, `go test -count=1 -race -cover -coverprofile=cover.out ./...`, `go tool cover -html=cover.out -o cover.html`. Reset очищает тестовую БД. Без TEST_DATABASE_URL интеграционные сценарии пропускаются; в CI они включены. Проценты и полный порядок запуска — в корневом README.
