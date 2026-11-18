-- Back to the 000007 shape: cart items owned by users. Cart rows can't be
-- given to a user, so they are deleted.
DELETE FROM cart_items;

ALTER TABLE cart_items DROP COLUMN cart_id;

ALTER TABLE cart_items
    ADD COLUMN user_id UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    ADD CONSTRAINT cart_items_user_id_offer_id_key UNIQUE (user_id, offer_id);

DROP TABLE carts;
