-- +goose Up
CREATE TABLE swap_request (
    Amount INTEGER,
    Id TEXT,
    Destination TEXT,
    State TEXT,
    Type TEXT
);


-- +goose Down
DROP TABLE IF EXISTS swap_request;
