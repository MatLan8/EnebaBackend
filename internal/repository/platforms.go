package repository

import "eneba-backend/internal/responses"

// Product queries that return a platform join it under alias "pl" and select platformColumns.
const joinPlatform = "platforms pl ON pl.id = p.platform_id"

var platformColumns = []string{"pl.slug", "pl.name", "pl.icon_url"}

func platformResponse(slug, name, iconURL string) responses.PlatformResponse {
	return responses.PlatformResponse{Slug: slug, Name: name, IconURL: iconURL}
}
