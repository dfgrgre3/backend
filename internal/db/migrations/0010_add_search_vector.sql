-- Migration: Add full-text search support for Subjects
-- Adds tsvector column and GIN index for fast full-text search

ALTER TABLE "Subject" ADD COLUMN IF NOT EXISTS search_vector tsvector;

-- Create GIN index for full-text search
CREATE INDEX IF NOT EXISTS idx_subject_search_vector ON "Subject" USING GIN(search_vector);

-- Populate search_vector for existing records
UPDATE "Subject"
SET search_vector = to_tsvector('simple',
    COALESCE(name, '') || ' ' ||
    COALESCE(name_ar, '') || ' ' ||
    COALESCE(description, '') || ' ' ||
    COALESCE(code, '')
);

-- Create trigger to automatically update search_vector on insert/update
CREATE OR REPLACE FUNCTION update_subject_search_vector()
RETURNS trigger AS $$
BEGIN
    NEW.search_vector := to_tsvector('simple',
        COALESCE(NEW.name, '') || ' ' ||
        COALESCE(NEW.name_ar, '') || ' ' ||
        COALESCE(NEW.description, '') || ' ' ||
        COALESCE(NEW.code, '')
    );
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_subject_search_vector ON "Subject";
CREATE TRIGGER trg_subject_search_vector
    BEFORE INSERT OR UPDATE OF name, name_ar, description, code
    ON "Subject"
    FOR EACH ROW
    EXECUTE FUNCTION update_subject_search_vector();