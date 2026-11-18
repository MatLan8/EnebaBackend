-- The cart that was checked out. The cart is only emptied when Stripe reports
-- the payment (webhook), so the order has to remember which cart to empty.
-- SET NULL: deleting a cart never touches its orders.
ALTER TABLE orders ADD COLUMN cart_id UUID REFERENCES carts (id) ON DELETE SET NULL;
