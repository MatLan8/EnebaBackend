-- Discounts are measured against the product's official base price (official store price or a
-- gift card's face value), the way Eneba shows them, so the reference price moves to the product.

-- No default on purpose: every product needs a real base price. Because of that, adding
-- the column fails while products has rows. `go run ./cmd/seed -reset` empties the
-- catalog before it migrates, then seeds every product with its base price.
ALTER TABLE products ADD COLUMN base_price_cents INTEGER NOT NULL CHECK (base_price_cents > 0);

-- Dropping the column also drops its CHECK constraint.
ALTER TABLE offers DROP COLUMN original_price_cents;
