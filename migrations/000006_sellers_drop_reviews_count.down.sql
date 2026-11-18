-- Restore the column as it was defined in 000003. DEFAULT 0 lets existing rows
-- satisfy NOT NULL without inventing review counts.
ALTER TABLE sellers ADD COLUMN reviews_count INTEGER NOT NULL DEFAULT 0 CHECK (reviews_count >= 0);
