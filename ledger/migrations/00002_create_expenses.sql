-- +goose Up
-- +goose StatementBegin
CREATE TABLE expenses (
    id          SERIAL PRIMARY KEY,
    amount      NUMERIC(14,2) NOT NULL CHECK (amount <> 0),
    category    TEXT NOT NULL,
    description TEXT,
    date        DATE NOT NULL
);

CREATE INDEX idx_expenses_category_date ON expenses (category, date);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_expenses_category_date;
DROP TABLE IF EXISTS expenses;
-- +goose StatementEnd
