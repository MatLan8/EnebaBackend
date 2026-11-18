-- Quantity is capped at 10, the same limit the cart API enforces.
CREATE TABLE cart_items (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    offer_id   UUID NOT NULL REFERENCES offers (id) ON DELETE CASCADE,
    quantity   INTEGER NOT NULL CHECK (quantity BETWEEN 1 AND 10),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (user_id, offer_id) -- one row per offer in a user's cart
);
