package repository

import sq "github.com/Masterminds/squirrel"

// availableKeyCondition is the one stock rule: not sold and not reserved (an expired reservation
// counts as free). Every stock count and check uses it, so they always agree. Expects alias "k".
const availableKeyCondition = "k.sold_at IS NULL AND (k.reserved_until IS NULL OR k.reserved_until < now())"

// Keys reserved by the cart's own pending order also count for that cart: checking out again
// cancels that order before reserving, so they are freed first. Takes the cart ID as its argument.
const availableForCartCondition = "(" + availableKeyCondition + ` OR (k.sold_at IS NULL AND EXISTS (
	SELECT 1 FROM order_items oi JOIN orders po ON po.id = oi.order_id
	WHERE oi.digital_key_id = k.id AND po.cart_id = ? AND po.status = 'pending'
)))`

const availableForCartCount = "(SELECT COUNT(*) FROM digital_keys k WHERE k.offer_id = o.id AND " + availableForCartCondition + ")"

const availableKeyExists = "EXISTS (SELECT 1 FROM digital_keys k WHERE k.offer_id = o.id AND " + availableKeyCondition + ")"

var hasAvailableKey = sq.Expr(availableKeyExists)
