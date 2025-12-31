-- 001_create_tasks.sql
-- Initial migration: create tasks table and supporting objects.
-- This migration is idempotent (uses IF NOT EXISTS) so it can be safely applied
-- against a fresh database. It creates:
--  - pgcrypto extension (for gen_random_uuid())
--  - tasks table
--  - index on completed
--  - trigger to keep updated_at current on UPDATE
--
-- Note: The application currently generates UUIDs in code; the default UUID here
-- is provided as a convenience if DB-side generation is desired.

-- Enable pgcrypto for gen_random_uuid() (safe to run if already present)
CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- Create tasks table
CREATE TABLE IF NOT EXISTS tasks (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  title TEXT NOT NULL,
  description TEXT,
  assignee TEXT,
  completed BOOLEAN NOT NULL DEFAULT FALSE,
  due_date TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Index to speed up queries filtering by completion
CREATE INDEX IF NOT EXISTS idx_tasks_completed ON tasks (completed);

-- Trigger function to update updated_at on row change
CREATE OR REPLACE FUNCTION trg_set_updated_at()
RETURNS TRIGGER AS $$
BEGIN
  IF (TG_OP = 'UPDATE') THEN
    NEW.updated_at = now();
    RETURN NEW;
  END IF;
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Attach trigger to tasks table (only one trigger; safe to recreate)
DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1
    FROM pg_trigger
    WHERE tgname = 'trg_tasks_set_updated_at'
  ) THEN
    CREATE TRIGGER trg_tasks_set_updated_at
      BEFORE UPDATE ON tasks
      FOR EACH ROW
      EXECUTE FUNCTION trg_set_updated_at();
  END IF;
END;
$$;

-- Optional: convenience view with nullable fields materialized as simple columns (not created by default)
-- CREATE VIEW IF NOT EXISTS tasks_view AS
-- SELECT
--   id, title, description, completed, due_date, created_at, updated_at
-- FROM tasks;

-- Down / cleanup (commented out — uncomment if you need to roll back manually)
-- DROP TRIGGER IF EXISTS trg_tasks_set_updated_at ON tasks;
-- DROP FUNCTION IF EXISTS trg_set_updated_at();
-- DROP INDEX IF EXISTS idx_tasks_completed;
-- DROP TABLE IF EXISTS tasks;
