ALTER TABLE offers ADD COLUMN original_price_cents INTEGER
    CHECK (original_price_cents IS NULL OR original_price_cents > price_cents);

ALTER TABLE products DROP COLUMN base_price_cents;
