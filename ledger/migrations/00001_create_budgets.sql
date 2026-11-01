-- +goose Up
-- +goose StatementBegin
CREATE TABLE budgets (
    id           SERIAL PRIMARY KEY,
    category     TEXT UNIQUE NOT NULL,
    limit_amount NUMERIC(14,2) NOT NULL CHECK (limit_amount > 0),
    period       TEXT NOT NULL DEFAULT ''
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS budgets;
-- +goose StatementEnd
