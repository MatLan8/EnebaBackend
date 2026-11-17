-- Every product now has a release date (games, DLCs, and also gift cards,
-- subscriptions and software), so the newest/oldest sorts never meet NULLs.
-- Fails if any product still has a NULL release_date: reset the catalog and reseed first.
ALTER TABLE products ALTER COLUMN release_date SET NOT NULL;