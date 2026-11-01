package ledger

import (
	"context"
	"log"

	"github.com/DITLEKS/GO_Project/ledger/internal/cache"
	"github.com/DITLEKS/GO_Project/ledger/internal/db"
)

// InitFromEnv подключает Ledger к PostgreSQL (DATABASE_URL или DB_HOST/DB_PORT/DB_USER/DB_PASS/DB_NAME)
// и Redis (REDIS_ADDR, REDIS_DB, REDIS_PASSWORD) и переключает хранилище на PostgreSQL.
//
// Ошибка подключения к PostgreSQL — фатальна (вызывающий код завершает приложение).
// Redis — ускоритель: при его недоступности пишется предупреждение, Ledger работает без кэша.
// Возвращает функцию закрытия соединений.
func InitFromEnv(ctx context.Context) (func(), error) {
	dsn := db.DSNFromEnv()
	conn, err := db.Open(ctx, dsn)
	if err != nil {
		return nil, err
	}
	log.Printf("Ledger: подключение к PostgreSQL установлено (%s)", db.Redact(dsn))

	dbConn = conn
	store = &pgStore{db: conn}

	rc, err := cache.NewFromEnv(ctx)
	if err != nil {
		log.Printf("Ledger: предупреждение: %v — работаем без кэша", err)
	} else {
		cacheClient = rc
		log.Printf("Ledger: подключение к Redis установлено (%s)", rc.Addr())
	}

	return func() {
		if cacheClient != nil {
			_ = cacheClient.Close()
		}
		if dbConn != nil {
			_ = dbConn.Close()
		}
		Reset()
	}, nil
}
