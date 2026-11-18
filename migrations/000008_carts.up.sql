-- Carts belong to a browser instead of a user, like on Eneba. The cart's id is
-- kept in an HttpOnly cart_token cookie, so a cart works before signing in.
CREATE TABLE carts (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now() -- bumped on every item change
);

-- The existing rows are test data tied to users; they can't be moved to a cart.
DELETE FROM cart_items;

-- Dropping user_id also drops its foreign key and the UNIQUE (user_id, offer_id).
ALTER TABLE cart_items DROP COLUMN user_id;

ALTER TABLE cart_items
    ADD COLUMN cart_id UUID NOT NULL REFERENCES carts (id) ON DELETE CASCADE,
    ADD CONSTRAINT cart_items_cart_id_offer_id_key UNIQUE (cart_id, offer_id); -- one row per offer in a cart
