-- Runs automatically on first postgres container start via docker-entrypoint-initdb.d.
-- Creates the pgvector extension required for embeddings/RAG.
CREATE EXTENSION IF NOT EXISTS vector;
