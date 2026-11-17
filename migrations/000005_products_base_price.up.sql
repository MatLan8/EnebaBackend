-- Discounts are measured against the product's official base price, the way Eneba
-- shows them: the price in the official store (Steam, PlayStation Store, Nintendo eShop,
-- Microsoft Store, ...) or the face value of a gift card. A 16.50 EUR offer for a game
-- that costs 29.99 EUR officially shows 29.99 crossed out and -45%.
-- So the reference price moves from each offer to the product.

-- No default on purpose: every product needs a real base price. Because of that, adding
-- the column fails while products has rows. `go run ./cmd/seed -reset` empties the
-- catalog before it migrates, then seeds every product with its base price.
ALTER TABLE products ADD COLUMN base_price_cents INTEGER NOT NULL CHECK (base_price_cents > 0);

-- The per-offer "original price" is replaced by the product's base price.
-- Dropping the column also drops its CHECK constraint.
ALTER TABLE offers DROP COLUMN original_price_cents;
