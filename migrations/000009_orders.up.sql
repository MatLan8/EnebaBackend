-- Orders are created at checkout. An order starts as pending, becomes paid when
-- Stripe confirms the payment, or cancelled if the payment never completes.
CREATE TABLE orders (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    public_id         TEXT NOT NULL UNIQUE,  -- the order number shown to the buyer
    user_id           UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    status            TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'paid', 'cancelled')),
    total_cents       INTEGER NOT NULL CHECK (total_cents > 0),
    currency          CHAR(3) NOT NULL,
    stripe_session_id TEXT UNIQUE,           -- NULL until the Stripe Checkout session is created
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    paid_at           TIMESTAMPTZ,
    CHECK ((status = 'paid') = (paid_at IS NOT NULL)) -- paid orders have paid_at, other orders don't
);

-- A user's order history, newest first.
CREATE INDEX idx_orders_user_id_created_at ON orders (user_id, created_at DESC);

-- One row per key bought. The price is copied from the offer at checkout, so
-- later price changes don't alter past orders.
CREATE TABLE order_items (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id         UUID NOT NULL REFERENCES orders (id) ON DELETE CASCADE,
    offer_id         UUID NOT NULL REFERENCES offers (id),
    digital_key_id   UUID NOT NULL UNIQUE REFERENCES digital_keys (id), -- a key is sold at most once
    unit_price_cents INTEGER NOT NULL CHECK (unit_price_cents > 0),
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_order_items_order_id ON order_items (order_id);

-- Keys now point the other way: order_items.digital_key_id says which order
-- bought a key, so digital_keys.order_item_id and its check are no longer needed.
ALTER TABLE digital_keys DROP CONSTRAINT digital_keys_check;
DROP INDEX idx_digital_keys_unsold;
ALTER TABLE digital_keys DROP COLUMN order_item_id;

-- A key is held for a checkout in progress until this time. NULL or a past
-- time means it is free. Sold keys have sold_at set.
ALTER TABLE digital_keys ADD COLUMN reserved_until TIMESTAMPTZ;

-- Partial index: only unsold keys. Keeps stock counts and "take a free key" fast.
CREATE INDEX digital_keys_available_idx ON digital_keys (offer_id) WHERE sold_at IS NULL;
