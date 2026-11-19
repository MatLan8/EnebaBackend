package models

// These lists must match the CHECK constraints in migrations/000003_products.up.sql.

type ProductType string

const (
	ProductTypeGame         ProductType = "game"
	ProductTypeDLC          ProductType = "dlc"
	ProductTypeGiftCard     ProductType = "gift_card"
	ProductTypeSubscription ProductType = "subscription"
	ProductTypeSoftware     ProductType = "software"
	ProductTypeTopUp        ProductType = "top_up" // in-game currency, e.g. FC Points
)

var ProductTypes = []ProductType{
	ProductTypeGame,
	ProductTypeDLC,
	ProductTypeGiftCard,
	ProductTypeSubscription,
	ProductTypeSoftware,
	ProductTypeTopUp,
}

type Region string

const (
	RegionGlobal       Region = "global"
	RegionEurope       Region = "europe"
	RegionNorthAmerica Region = "north_america"
	RegionUK           Region = "uk"
	RegionLatAm        Region = "latam"
	RegionAsia         Region = "asia"
)

var Regions = []Region{
	RegionGlobal,
	RegionEurope,
	RegionNorthAmerica,
	RegionUK,
	RegionLatAm,
	RegionAsia,
}
