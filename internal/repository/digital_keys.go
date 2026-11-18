package repository

import sq "github.com/Masterminds/squirrel"

// availableKeyCondition is the one rule for a digital key that can still be
// bought: not sold, and not reserved by a checkout in progress (an expired
// reservation counts as free). It expects digital_keys to be aliased as "k".
// Every stock count and "has stock" check uses it, so they always agree.
const availableKeyCondition = "k.sold_at IS NULL AND (k.reserved_until IS NULL OR k.reserved_until < now())"

// availableKeyCount counts the available keys of offer "o", i.e. its stock.
const availableKeyCount = "(SELECT COUNT(*) FROM digital_keys k WHERE k.offer_id = o.id AND " + availableKeyCondition + ")"

// hasAvailableKey limits a query on "offers o" to offers with at least one
// available key. An offer without one can't be bought right now.
var hasAvailableKey = sq.Expr("EXISTS (SELECT 1 FROM digital_keys k WHERE k.offer_id = o.id AND " + availableKeyCondition + ")")
