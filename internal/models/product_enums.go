package models

// Go has no enum keyword. The usual pattern is a named string type plus
// constants of that type. A ProductType can't be mixed up with a Platform by
// accident, and the value stored in the database is still a plain string.
// These lists must match the CHECK constraints in migrations/000003_products.up.sql.

// ProductType is what kind of item a product is.
type ProductType string

const (
	ProductTypeGame         ProductType = "game"
	ProductTypeDLC          ProductType = "dlc"
	ProductTypeGiftCard     ProductType = "gift_card"
	ProductTypeSubscription ProductType = "subscription"
	ProductTypeSoftware     ProductType = "software"
	ProductTypeTopUp        ProductType = "top_up" // in-game currency, e.g. FC Points
)

// ProductTypes lists every valid ProductType, e.g. for filters in the store.
var ProductTypes = []ProductType{
	ProductTypeGame,
	ProductTypeDLC,
	ProductTypeGiftCard,
	ProductTypeSubscription,
	ProductTypeSoftware,
	ProductTypeTopUp,
}

// Platform is where the key is activated.
type Platform string

const (
	PlatformSteam           Platform = "steam"
	PlatformXboxLive        Platform = "xbox_live"
	PlatformPSN             Platform = "psn"
	PlatformNintendoEShop   Platform = "nintendo_eshop"
	PlatformEAApp           Platform = "ea_app"
	PlatformUbisoft         Platform = "ubisoft"
	PlatformEpicGames       Platform = "epic_games"
	PlatformBattleNet       Platform = "battlenet"
	PlatformOfficialWebsite Platform = "official_website" // redeemed on the publisher's own site, e.g. Minecraft or Windows
)

// Platforms lists every valid Platform.
var Platforms = []Platform{
	PlatformSteam,
	PlatformXboxLive,
	PlatformPSN,
	PlatformNintendoEShop,
	PlatformEAApp,
	PlatformUbisoft,
	PlatformEpicGames,
	PlatformBattleNet,
	PlatformOfficialWebsite,
}

// Region is where the key can be activated.
type Region string

const (
	RegionGlobal       Region = "global"
	RegionEurope       Region = "europe"
	RegionNorthAmerica Region = "north_america"
	RegionUK           Region = "uk"
	RegionLatAm        Region = "latam"
	RegionAsia         Region = "asia"
)

// Regions lists every valid Region.
var Regions = []Region{
	RegionGlobal,
	RegionEurope,
	RegionNorthAmerica,
	RegionUK,
	RegionLatAm,
	RegionAsia,
}
