-- +goose Up
CREATE EXTENSION IF NOT EXISTS vector;

CREATE TABLE document_chunks (
    id SERIAL PRIMARY KEY,
    file_path TEXT NOT NULL,
    content TEXT NOT NULL,
    -- 1536 dimensions match standard OpenAI / Voyage / AWS Titan embeddings
    embedding vector(1536),
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX ON document_chunks USING hnsw (embedding vector_cosine_ops);

-- +goose Down
SELECT 'down SQL query';
