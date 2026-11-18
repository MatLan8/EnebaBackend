package repository

import sq "github.com/Masterminds/squirrel"

// availableKeyCondition is the one stock rule: not sold and not reserved (an expired reservation
// counts as free). Every stock count and check uses it, so they always agree. Expects alias "k".
const availableKeyCondition = "k.sold_at IS NULL AND (k.reserved_until IS NULL OR k.reserved_until < now())"

const availableKeyCount = "(SELECT COUNT(*) FROM digital_keys k WHERE k.offer_id = o.id AND " + availableKeyCondition + ")"

const availableKeyExists = "EXISTS (SELECT 1 FROM digital_keys k WHERE k.offer_id = o.id AND " + availableKeyCondition + ")"

var hasAvailableKey = sq.Expr(availableKeyExists)
