-- +goose Up
CREATE TABLE location_operations (
 group_id uuid NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
 request_id uuid NOT NULL,
 fingerprint text NOT NULL,
 result text NOT NULL,
 PRIMARY KEY (group_id, request_id)
);

-- +goose Down
DROP TABLE location_operations;
