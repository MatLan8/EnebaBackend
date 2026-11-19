ALTER TABLE products ADD COLUMN platform TEXT;

-- Platforms that didn't exist before this migration were all 'official_website'.
UPDATE products p
SET platform = CASE
    WHEN pl.slug IN ('steam', 'xbox_live', 'psn', 'nintendo_eshop', 'ea_app', 'ubisoft', 'epic_games', 'battlenet') THEN pl.slug
    ELSE 'official_website'
END
FROM platforms pl
WHERE pl.id = p.platform_id;

ALTER TABLE products ALTER COLUMN platform SET NOT NULL;
ALTER TABLE products ADD CONSTRAINT products_platform_check CHECK (platform IN (
    'steam', 'xbox_live', 'psn', 'nintendo_eshop', 'ea_app', 'ubisoft', 'epic_games', 'battlenet', 'official_website'
));

-- Also drops idx_products_platform_id and the foreign key.
ALTER TABLE products DROP COLUMN platform_id;

DROP TABLE platforms;
