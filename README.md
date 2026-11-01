# GO_Project

Домашние задания по дисциплине **«Язык Go»**. Требуется Go 1.22 или новее.

## Структура репозитория

```text
.
├── main.go        # ДЗ 1: имя пользователя, CLI-аргументы, версия Go
├── go.mod
├── gateway/       # HTTP-шлюз (отдельный Go-модуль, импортирует ledger)
│   ├── go.mod             # require + replace на ../ledger
│   ├── main.go            # запуск сервера на :8080
│   └── internal/api/
│       ├── dto.go, handlers.go, middleware.go, router.go
│       ├── handlers_test.go   # HTTP-интеграционные тесты (httptest)
│       └── dto_test.go        # параллельные unit-тесты DTO и middleware
└── ledger/        # бизнес-логика (отдельный Go-модуль, пакет ledger)
    ├── transaction.go       # Transaction, Validate, AddTransaction, ListTransactions
    ├── budget.go            # Budget, Validate, SetBudget, LoadBudgets, проверка лимита
    ├── validatable.go      # интерфейс Validatable, CheckValid
    ├── reset.go             # Reset() — очистка состояния для тестов
    ├── budgets.json         # бюджеты, загружаемые демо-программой
    ├── ledger_test.go       # table-driven unit-тесты домена
    ├── *_test.go
    └── cmd/demo/main.go     # демонстрационные сценарии
```

## ДЗ 1. Введение в Go

```bash
go run . arg1 arg2
```

## ДЗ 2. Функции: Gateway и Ledger

Gateway — HTTP-сервер на порту 8080. Ledger — структура `Transaction` (ID, Amount, Category, Description, Date), хранилище в памяти, функции `AddTransaction` и `ListTransactions`.

## ДЗ 3. Ввод-вывод (I/O). Обработка ошибок: бюджеты в Ledger

- **`Budget`** — категория (`Category`), лимит (`Limit`), необязательный период (`Period`). JSON-теги: `category`, `limit`, `period`.
- **Хранилище бюджетов** — `map[string]Budget`, ключ — категория. Изначально: «Продукты» — 5000, «Транспорт» — 2000.
- **`SetBudget(b Budget) error`** — добавляет или обновляет бюджет; ошибка при пустой категории или лимите ≤0.
- **`AddTransaction(tx Transaction) error`** — если расходы по категории вместе с новой транзакцией превышают `Limit`, возвращается `ErrBudgetExceeded` (`"budget exceeded"`), транзакция не сохраняется. Расходы — отрицательные суммы; доходы и категории без бюджета не ограничиваются.
- **`SpentByCategory(category string) float64`** — сумма расходов по категории.
- **`LoadBudgets(r io.Reader) error`** — читает JSON-массив бюджетов и добавляет через `SetBudget`; понятные ошибки для nil-reader, ошибки чтения, пустого источника, невалидного JSON и невалидного бюджета.

## ДЗ 4. ООП: интерфейс Validatable

- `Validatable` — интерфейс с одним методом `Validate() error`; `CheckValid(v Validatable) error` работает с любым типом, реализующим интерфейс.
- `Transaction.Validate()` — сумма не равна 0 (отрицательная допустима: это расход), категория не пустая, непустая дата — `ГГГГ-ММ-ДД`.
- `Budget.Validate()` — лимит > 0, категория не пустая.
- `AddTransaction` и `SetBudget` сначала вызывают `Validate()`. Сами `Validate()` ничего не логируют.

### Запуск демо Ledger

```bash
cd ledger
go run ./cmd/demo
```

Запускать нужно из каталога `ledger`, т.к. `budgets.json` открывается по относительному пути. Демо печатает загрузку бюджетов, ошибки I/O, блок «Проверка через интерфейс Validatable», добавление транзакций (с отказами по бюджету) и итоги по категориям.

## ДЗ 5. Веб-приложения: REST API в Gateway

Gateway — отдельный Go-модуль; модуль `ledger` подключен через `require` + `replace ... => ../ledger` в `gateway/go.mod`. Хранилище — в памяти (при перезапуске сервера данные сбрасываются, кроме двух бюджетов по умолчанию).

### Запуск

```bash
cd gateway
go run .
```

Сервер слушает `:8080`. Каждый запрос логируется в консоль (метод, путь, длительность). Все ответы имеют `Content-Type: application/json; charset=utf-8`.

### Эндпоинты

| Метод | Путь | Описание | Успех | Ошибки |
|-------|------|----------|-------|--------|
| GET | `/ping` | проверка живости, `{"message":"pong"}` | 200 | 405 |
| POST | `/api/transactions` | создать транзакцию | 201 + `TransactionResponse` | 400 (валидация/JSON), 409 (`budget exceeded`), 500 |
| GET | `/api/transactions` | список транзакций (`[]`, если пусто) | 200 | — |
| POST | `/api/budgets` | создать или обновить бюджет | 201 + `BudgetResponse` | 400, 500 |
| GET | `/api/budgets` | список бюджетов | 200 | — |

Формат ошибок: `{"error":"описание"}`. Неизвестный путь — 404, недопустимый метод — 405 (тоже в JSON). Лишние поля в теле запроса отклоняются с 400.

Сумма транзакции: положительная — доход, отрицательная — расход (только расходы учитываются в бюджете); `0` запрещён.

### Примеры cURL

Создать транзакцию (201):

```bash
curl -i -X POST http://localhost:8080/api/transactions \
  -H "Content-Type: application/json" \
  -d '{"amount":450,"category":"еда","description":"ланч","date":"2025-09-10"}'
# {"id":1,"amount":450,"category":"еда","description":"ланч","date":"2025-09-10"}
```

Ошибка валидации (400):

```bash
curl -i -X POST http://localhost:8080/api/transactions \
  -H "Content-Type: application/json" \
  -d '{"amount":0,"category":"еда"}'
# {"error":"сумма транзакции не может быть равна нулю или быть некорректным числом"}
```

Список транзакций (200):

```bash
curl -i http://localhost:8080/api/transactions
```

Создать бюджет (201) и превысить его (409):

```bash
curl -i -X POST http://localhost:8080/api/budgets \
  -H "Content-Type: application/json" \
  -d '{"category":"кафе","limit":1000}'
# {"category":"кафе","limit":1000}

curl -i -X POST http://localhost:8080/api/transactions \
  -H "Content-Type: application/json" \
  -d '{"amount":-1500,"category":"кафе","description":"ужин","date":"2025-09-11"}'
# HTTP/1.1 409 Conflict
# {"error":"budget exceeded"}
```

Некорректный бюджет (400) и список бюджетов (200):

```bash
curl -i -X POST http://localhost:8080/api/budgets \
  -H "Content-Type: application/json" \
  -d '{"category":"кафе","limit":-5}'

curl -i http://localhost:8080/api/budgets
```

В Windows PowerShell вместо `\` для переноса строк используйте `` ` ``, а JSON передавайте в одинарных кавычках через `curl.exe`.

## ДЗ 6. Тестирование

### Что покрыто

- **Ledger (`ledger/ledger_test.go`, `validatable_test.go`, `budget_test.go`, `transaction_test.go`)** — table-driven тесты `Transaction.Validate()` и `Budget.Validate()` (валидные и негативные случаи, NaN/Inf, пустая категория, невалидный лимит, дата), бизнес-правило «превышение бюджета → `ErrBudgetExceeded`, транзакция не сохраняется», `Reset()`.
- **Gateway (`gateway/internal/api/handlers_test.go`, `dto_test.go`)** — HTTP-интеграционные тесты через `httptest`: `POST/GET /api/budgets` (201/200, 400 с `{"error":"..."}`), `POST/GET /api/transactions` (201 и последующий 200, 409 `budget exceeded`, 400 на невалидный JSON), 404/405, логирование, заголовок `Content-Type: application/json; charset=utf-8` в каждом ответе.

Изоляция: хранилище `ledger` — глобальные переменные, поэтому тесты с состоянием вызывают `ledger.Reset()` в начале и в `t.Cleanup` и не параллельны. `t.Parallel()` используется там, где общего состояния нет: табличные тесты `Validate()`, преобразования DTO, `Chain`, маршруты 404/405 и `/ping`.

### Запуск тестов и покрытие

`ledger` и `gateway` — отдельные Go-модули, поэтому тесты запускаются в каждом отдельно:

```bash
cd ledger
go test ./... -cover -coverprofile=cover.out
go tool cover -func=cover.out | tail -n 1       # итоговый процент
 go tool cover -html=cover.out -o cover.html       # HTML-отчёт (коммитить не нужно)

cd ../gateway
go test ./... -cover -coverprofile=cover.out
go tool cover -func=cover.out | tail -n 1
go tool cover -html=cover.out -o cover.html
```

Дополнительно полезно прогнать тесты с детектором гонок: `go test -race ./...` (в каждом модуле).

**Покрытие тестами:** ledger — `__%`, gateway — `__%` (заполнить после локального запуска команд выше; команда `cmd/demo` в покрытие не входит).
