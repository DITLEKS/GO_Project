# Ledger: PostgreSQL + Redis

Хранение бюджетов/транзакций — только PostgreSQL, отчёты — Redis TTL 30 секунд. Ledger импортируется Gateway напрямую, без HTTP/RPC. `cmd/server` проверяет подключение и ждёт сигнала остановки; данные не очищаются при старте/закрытии.

Из корня:

```bash
docker compose up -d --wait
cp .env.example .env
set -a; . ./.env; set +a
go install -tags='no_clickhouse no_libsql no_mssql no_mysql no_sqlite3 no_vertica no_ydb' github.com/pressly/goose/v3/cmd/goose@v3.24.1
export PATH="$(go env GOPATH)/bin:$PATH"
goose -dir ledger/migrations postgres "$DATABASE_URL" up
(cd ledger && go run ./cmd/server)
# Откат последней миграции (удаляет таблицу):
# goose -dir ledger/migrations postgres "$DATABASE_URL" down
```

Без Docker: `createdb -h localhost -U postgres cashapp`, запустите Redis локально. `DATABASE_URL` приоритетна; fallback: DB_HOST=localhost, DB_PORT=5432, DB_USER=postgres, DB_PASS=postgres, DB_NAME=cashapp. REDIS_ADDR=localhost:6379, REDIS_DB=0, REDIS_PASSWORD опциональна. `.env` автоматически не загружается. Пул SQL 10/5, Ping при старте; ошибки подключения завершают процесс.

SetBudget — UPSERT; списки возвращают `(slice,error)`; CreateTransaction возвращает SQL ID; AddTransaction сохраняет прежний error-only API. Отрицательные суммы — расходы, положительные — доходы; доход не уменьшает расходы. Категория без бюджета не ограничивается. Проверка и INSERT атомарны с advisory lock. Суммы — NUMERIC(14,2), даты — DATE; пустая дата становится сегодняшней UTC-датой.

Для HTTP запустите Gateway с теми же env:

```bash
(cd gateway && go run .)
curl -i -X POST localhost:8080/api/budgets -H 'Content-Type: application/json' -d '{"category":"еда","limit":5000}'
curl -i -X POST localhost:8080/api/transactions -H 'Content-Type: application/json' -d '{"amount":-3000,"category":"еда","date":"2026-11-01"}'
curl -i localhost:8080/api/transactions
curl -i localhost:8080/api/budgets
curl -i 'localhost:8080/api/reports/summary?from=2026-11-01&to=2026-11-30'
curl -i 'localhost:8080/api/reports/summary?from=2026-11-01&to=2026-11-30'
```

Повторный отчёт логирует cache hit. Запись меняет поколение кеша, TTL 30 секунд; сбой Redis после старта → чтение SQL. Используйте отдельный REDIS_DB для тестов/разных БД. Перезапуск сохраняет данные; `docker compose down -v` удаляет том.

Полные сценарии, тесты и измеренное покрытие — в корневом README. `Reset() error` удаляет ВСЕ строки; интеграционные тесты требуют отдельную TEST_DATABASE_URL, применённые миграции и REDIS_DB=15. Без TEST_DATABASE_URL интеграционные тесты пропускаются.
