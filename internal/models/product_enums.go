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
)

// ProductTypes lists every valid ProductType, e.g. for filters in the store.
var ProductTypes = []ProductType{
	ProductTypeGame,
	ProductTypeDLC,
	ProductTypeGiftCard,
	ProductTypeSubscription,
}

// Platform is where the key is activated.
type Platform string

const (
	PlatformSteam       Platform = "steam"
	PlatformXbox        Platform = "xbox"
	PlatformPlayStation Platform = "playstation"
	PlatformNintendo    Platform = "nintendo"
	PlatformEA          Platform = "ea"
	PlatformUbisoft     Platform = "ubisoft"
	PlatformEpic        Platform = "epic"
	PlatformGOG         Platform = "gog"
	PlatformBattleNet   Platform = "battlenet"
	PlatformOther       Platform = "other"
)

// Platforms lists every valid Platform.
var Platforms = []Platform{
	PlatformSteam,
	PlatformXbox,
	PlatformPlayStation,
	PlatformNintendo,
	PlatformEA,
	PlatformUbisoft,
	PlatformEpic,
	PlatformGOG,
	PlatformBattleNet,
	PlatformOther,
}

// Region is where the key can be activated.
type Region string

const (
	RegionGlobal Region = "global"
	RegionEU     Region = "eu"
	RegionNA     Region = "na"
	RegionUK     Region = "uk"
	RegionLatAm  Region = "latam"
	RegionAsia   Region = "asia"
)

// Regions lists every valid Region.
var Regions = []Region{
	RegionGlobal,
	RegionEU,
	RegionNA,
	RegionUK,
	RegionLatAm,
	RegionAsia,
}
