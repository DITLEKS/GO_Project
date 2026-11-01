package ledger

// Reset возвращает Ledger к пустому хранилищу в памяти (без бюджетов по умолчанию) и отключает кэш.
// Предназначена для изоляции тестов: вызывайте в начале теста или в t.Cleanup.
// Соединения с PostgreSQL/Redis не закрывает (это делает функция, возвращённая InitFromEnv), базу данных не очищает.
func Reset() {
	store = newMemoryStore(false)
	dbConn = nil
	cacheClient = nil
}
