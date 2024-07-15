CREATE TABLE IF NOT EXISTS photos (
    id SERIAL PRIMARY KEY,
    filename TEXT NOT NULL,
    metadata TEXT
);

