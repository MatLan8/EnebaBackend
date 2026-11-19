-- Platforms become a table so each one can have a display name and an icon. Existing slugs are
-- kept, so store URLs like ?platform=steam keep working; 'official_website' is split up below.
CREATE TABLE platforms (
    id       UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    slug     TEXT NOT NULL UNIQUE, -- used in ?platform= filters
    name     TEXT NOT NULL UNIQUE,
    icon_url TEXT NOT NULL
);

INSERT INTO platforms (slug, name, icon_url) VALUES
    ('steam',          'Steam',               'https://vwaezlhgtltnkbrisigc.supabase.co/storage/v1/object/public/Icons/steam.png'),
    ('epic_games',     'Epic Games',          'https://vwaezlhgtltnkbrisigc.supabase.co/storage/v1/object/public/Icons/epic-games.png'),
    ('ea_app',         'EA app',              'https://vwaezlhgtltnkbrisigc.supabase.co/storage/v1/object/public/Icons/ea.png'),
    ('battlenet',      'Battle.net',          'https://vwaezlhgtltnkbrisigc.supabase.co/storage/v1/object/public/Icons/battlenet.png'),
    ('psn',            'PlayStation Network', 'https://vwaezlhgtltnkbrisigc.supabase.co/storage/v1/object/public/Icons/psn.png'),
    ('ubisoft',        'Ubisoft Connect',     'https://vwaezlhgtltnkbrisigc.supabase.co/storage/v1/object/public/Icons/ubisoft.png'),
    ('xbox_live',      'Xbox',                'https://vwaezlhgtltnkbrisigc.supabase.co/storage/v1/object/public/Icons/xbox.png'),
    ('nintendo_eshop', 'Nintendo eShop',      'https://vwaezlhgtltnkbrisigc.supabase.co/storage/v1/object/public/Icons/nintendo.png'),
    ('microsoft',      'Microsoft',           'https://vwaezlhgtltnkbrisigc.supabase.co/storage/v1/object/public/Icons/microsoft.png'),
    ('roblox',         'Roblox',              'https://vwaezlhgtltnkbrisigc.supabase.co/storage/v1/object/public/Icons/roblox.png'),
    ('google_play',    'Google Play',         'https://vwaezlhgtltnkbrisigc.supabase.co/storage/v1/object/public/Icons/google-play.png'),
    ('windows',        'Windows',             'https://vwaezlhgtltnkbrisigc.supabase.co/storage/v1/object/public/Icons/windows.png'),
    ('eset',           'ESET',                'https://vwaezlhgtltnkbrisigc.supabase.co/storage/v1/object/public/Icons/eset.png'),
    ('spotify',        'Spotify',             'https://vwaezlhgtltnkbrisigc.supabase.co/storage/v1/object/public/Icons/spotify.png'),
    ('discord',        'Discord',             'https://vwaezlhgtltnkbrisigc.supabase.co/storage/v1/object/public/Icons/discord.png'),
    ('riot_games',     'Riot Games',          'https://vwaezlhgtltnkbrisigc.supabase.co/storage/v1/object/public/Icons/riot-games.png');

ALTER TABLE products ADD COLUMN platform_id UUID REFERENCES platforms (id);

UPDATE products p
SET platform_id = pl.id
FROM platforms pl
WHERE pl.slug = p.platform;

-- The old 'official_website' products get their real platform by title.
UPDATE products p
SET platform_id = pl.id
FROM (VALUES
    ('Minecraft: Java & Bedrock Edition (PC) Official website Key GLOBAL', 'microsoft'),
    ('Microsoft 365 Personal 12 Months (PC / Mac) Microsoft Key EUROPE',   'microsoft'),
    ('Roblox Gift Card 10 EUR (800 Robux) Key GLOBAL',                     'roblox'),
    ('Google Play Gift Card 25 EUR Key EUROPE',                            'google_play'),
    ('Windows 11 Pro (PC) Microsoft Key GLOBAL',                           'windows'),
    ('ESET NOD32 Antivirus 1 Device 1 Year Key GLOBAL',                    'eset'),
    ('Spotify Premium 1 Month Key EUROPE',                                 'spotify'),
    ('Discord Nitro 1 Month Key GLOBAL',                                   'discord'),
    ('VALORANT - 1000 VP (PC) Riot Key EUROPE',                            'riot_games')
) AS mapping (title, slug)
JOIN platforms pl ON pl.slug = mapping.slug
WHERE p.platform = 'official_website' AND p.title = mapping.title;

-- Fails if any product was left without a platform. The file runs as one implicit transaction,
-- so nothing above is kept then.
ALTER TABLE products ALTER COLUMN platform_id SET NOT NULL;

-- Dropping the column also drops its CHECK constraint.
ALTER TABLE products DROP COLUMN platform;

CREATE INDEX idx_products_platform_id ON products (platform_id);
