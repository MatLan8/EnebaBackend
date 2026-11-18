-- The CHECK values below must match the constants in internal/models/product_enums.go.

CREATE TABLE products (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    title        TEXT NOT NULL,
    slug         TEXT NOT NULL UNIQUE, -- used in URLs, e.g. /product/elden-ring-steam-global
    description  TEXT NOT NULL DEFAULT '',
    image_url    TEXT NOT NULL,        -- cover shown on cards and the product page
    type         TEXT NOT NULL CHECK (type IN ('game', 'dlc', 'gift_card', 'subscription', 'software', 'top_up')),
    platform     TEXT NOT NULL CHECK (platform IN ('steam', 'xbox_live', 'psn', 'nintendo_eshop', 'ea_app', 'ubisoft', 'epic_games', 'battlenet', 'official_website')),
    region       TEXT NOT NULL CHECK (region IN ('global', 'europe', 'north_america', 'uk', 'latam', 'asia')),
    developer    TEXT,
    publisher    TEXT,
    release_date DATE,
    sold_count   INTEGER NOT NULL DEFAULT 0 CHECK (sold_count >= 0), -- bumped by checkout; drives best sellers and the popularity sort
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Trigram index for typo-tolerant search on the title (pg_trgm, enabled in 000001).
CREATE INDEX idx_products_title_trgm ON products USING GIN (title gin_trgm_ops);

CREATE TABLE genres (
    id   UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL UNIQUE,
    slug TEXT NOT NULL UNIQUE
);

CREATE TABLE product_genres (
    product_id UUID NOT NULL REFERENCES products (id) ON DELETE CASCADE,
    genre_id   UUID NOT NULL REFERENCES genres (id) ON DELETE CASCADE,
    PRIMARY KEY (product_id, genre_id)
);

-- The primary key already covers lookups by product_id; this one covers "products in genre X".
CREATE INDEX idx_product_genres_genre_id ON product_genres (genre_id);

CREATE TABLE sellers (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name          TEXT NOT NULL UNIQUE,
    logo_url      TEXT,
    rating        NUMERIC(4, 2) NOT NULL DEFAULT 0 CHECK (rating BETWEEN 0 AND 10),
    reviews_count INTEGER NOT NULL DEFAULT 0 CHECK (reviews_count >= 0),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE offers (
    id                   UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    product_id           UUID NOT NULL REFERENCES products (id) ON DELETE CASCADE,
    seller_id            UUID NOT NULL REFERENCES sellers (id) ON DELETE CASCADE,
    price_cents          INTEGER NOT NULL CHECK (price_cents > 0),
    original_price_cents INTEGER CHECK (original_price_cents IS NULL OR original_price_cents > price_cents),
    currency             TEXT NOT NULL DEFAULT 'EUR',
    is_active            BOOLEAN NOT NULL DEFAULT TRUE,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (product_id, seller_id) -- one offer per seller per product
);

-- The UNIQUE constraint already indexes (product_id, seller_id), which covers lookups by product_id.
CREATE INDEX idx_offers_seller_id ON offers (seller_id);

CREATE TABLE digital_keys (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    -- RESTRICT: an offer that still has keys can't be deleted, so sold keys are never lost.
    -- Deactivate the offer (is_active = false) instead.
    offer_id       UUID NOT NULL REFERENCES offers (id) ON DELETE RESTRICT,
    code_encrypted BYTEA NOT NULL,        -- AES-GCM encrypted key, decrypted only for the buyer
    code_hash      TEXT NOT NULL UNIQUE,  -- SHA-256 of the plain key, stops the same key being added twice
    order_item_id  UUID,                  -- NULL = unsold; the foreign key is added with the orders migration
    sold_at        TIMESTAMPTZ,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK ((order_item_id IS NULL) = (sold_at IS NULL)) -- sold keys have both, unsold keys neither
);

-- Partial index: only unsold keys. Keeps stock counts and "take a free key" fast.
CREATE INDEX idx_digital_keys_unsold ON digital_keys (offer_id) WHERE order_item_id IS NULL;
