-- Back to the 000008 shape.
DROP INDEX IF EXISTS digital_keys_available_idx;
ALTER TABLE digital_keys DROP COLUMN reserved_until;

-- Restore order_item_id as defined in 000003. Sold keys get the id of the order
-- item that bought them, so the restored check holds (000003 has no foreign key on it).
ALTER TABLE digital_keys ADD COLUMN order_item_id UUID;
UPDATE digital_keys k
SET order_item_id = oi.id
FROM order_items oi
WHERE oi.digital_key_id = k.id;
ALTER TABLE digital_keys ADD CONSTRAINT digital_keys_check CHECK ((order_item_id IS NULL) = (sold_at IS NULL));
CREATE INDEX idx_digital_keys_unsold ON digital_keys (offer_id) WHERE order_item_id IS NULL;

-- Drop in reverse order of creation, because of the foreign keys.
DROP TABLE IF EXISTS order_items;
DROP TABLE IF EXISTS orders;
