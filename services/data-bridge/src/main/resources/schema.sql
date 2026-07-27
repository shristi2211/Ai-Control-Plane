-- ============================================
--  Data Bridge — Schema Initialization (MVP)
--  Production: migrate to Flyway / Liquibase
-- ============================================

-- Enable pgvector extension
CREATE EXTENSION IF NOT EXISTS vector;

-- ── Read-only user for Text-to-SQL (defense in depth) ──
DO $$ BEGIN
  IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'ai_reader') THEN
    CREATE ROLE ai_reader WITH LOGIN PASSWORD 'readonly';
  END IF;
END $$;

-- ── Mock Sales Table ──
CREATE TABLE IF NOT EXISTS company_sales (
    id SERIAL PRIMARY KEY,
    product VARCHAR(100) NOT NULL,
    region VARCHAR(50) NOT NULL,
    amount DECIMAL(12,2) NOT NULL,
    sale_date DATE NOT NULL,
    salesperson VARCHAR(100) NOT NULL
);

-- Seed mock data (only if table is empty)
INSERT INTO company_sales (product, region, amount, sale_date, salesperson)
SELECT * FROM (VALUES
    ('Enterprise AI Suite',   'North', 150000.00, DATE '2026-01-15', 'Rahul Sharma'),
    ('Data Analytics Pro',    'South',  85000.00, DATE '2026-01-22', 'Priya Patel'),
    ('Cloud Security Plus',   'East',  120000.00, DATE '2026-01-28', 'Amit Verma'),
    ('ML Pipeline Toolkit',   'West',   95000.00, DATE '2026-02-05', 'Sneha Gupta'),
    ('Enterprise AI Suite',   'North', 175000.00, DATE '2026-02-10', 'Rahul Sharma'),
    ('Data Analytics Pro',    'East',   65000.00, DATE '2026-02-14', 'Vikram Singh'),
    ('Cloud Security Plus',   'South', 140000.00, DATE '2026-02-18', 'Priya Patel'),
    ('Automation Hub',        'West',  110000.00, DATE '2026-02-22', 'Neha Joshi'),
    ('ML Pipeline Toolkit',   'North',  78000.00, DATE '2026-03-01', 'Amit Verma'),
    ('Enterprise AI Suite',   'South', 200000.00, DATE '2026-03-05', 'Sneha Gupta'),
    ('Cloud Security Plus',   'West',   92000.00, DATE '2026-03-10', 'Vikram Singh'),
    ('Automation Hub',        'East',  130000.00, DATE '2026-03-12', 'Neha Joshi'),
    ('Data Analytics Pro',    'North', 105000.00, DATE '2026-03-15', 'Rahul Sharma'),
    ('ML Pipeline Toolkit',   'South',  88000.00, DATE '2026-03-17', 'Priya Patel')
) AS v(product, region, amount, sale_date, salesperson)
WHERE NOT EXISTS (SELECT 1 FROM company_sales LIMIT 1);

-- Grant read access to ai_reader
GRANT SELECT ON ALL TABLES IN SCHEMA public TO ai_reader;

-- ── Embeddings table for RAG (pgvector) ──
CREATE TABLE IF NOT EXISTS embeddings (
    embedding_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    embedding vector(384),
    text TEXT,
    metadata JSON
);

-- HNSW index for 10x faster similarity search at scale
CREATE INDEX IF NOT EXISTS embeddings_hnsw_idx
    ON embeddings USING hnsw (embedding vector_cosine_ops);
