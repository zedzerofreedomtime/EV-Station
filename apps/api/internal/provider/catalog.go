package provider

type DataSourceCatalogEntry struct {
	ID               string   `json:"id"`
	Name             string   `json:"name"`
	Categories       []string `json:"categories"`
	CostModel        string   `json:"costModel"`
	Availability     string   `json:"availability"`
	CredentialEnvVar string   `json:"credentialEnvVar,omitempty"`
	ReferenceURI     string   `json:"referenceUri"`
	UsageNote        string   `json:"usageNote"`
	DataQualityNote  string   `json:"dataQualityNote"`
}

func DataSourceCatalog() []DataSourceCatalogEntry {
	return []DataSourceCatalogEntry{
		{ID: "osm-overpass", Name: "OpenStreetMap via Overpass API", Categories: []string{"poi", "competition", "road_accessibility"}, CostModel: "free_no_key", Availability: "active", ReferenceURI: "https://wiki.openstreetmap.org/wiki/Overpass_API", UsageNote: "Public endpoint with Redis caching; intended for modest MVP usage.", DataQualityNote: "Road data is an accessibility proxy only, not traffic volume, speed, congestion, or AADT."},
		{ID: "osm-nominatim", Name: "OpenStreetMap Nominatim", Categories: []string{"geocoding"}, CostModel: "free_no_key", Availability: "active", ReferenceURI: "https://operations.osmfoundation.org/policies/nominatim/", UsageNote: "Manual search only, maximum one public request per second, with cached results.", DataQualityNote: "Matches are preliminary until the user confirms the coordinates."},
		{ID: "osm-map", Name: "OpenStreetMap map preview", Categories: []string{"map"}, CostModel: "free_no_key", Availability: "active", ReferenceURI: "https://www.openstreetmap.org/copyright", UsageNote: "Embedded map preview with visible OpenStreetMap attribution.", DataQualityNote: "The preview is contextual and is not cadastral or survey evidence."},
		{ID: "tomtom", Name: "TomTom APIs", Categories: []string{"map", "geocoding", "routes", "traffic"}, CostModel: "free_key", Availability: "requires_key", CredentialEnvVar: "TOMTOM_API_KEY", ReferenceURI: "https://docs.tomtom.com/pricing", UsageNote: "Free monthly allowance and no card required; integration is deferred until a project key is supplied.", DataQualityNote: "Traffic flow is speed/congestion data, not a verified vehicle count or AADT."},
		{ID: "gistda", Name: "GISTDA Disaster Open API", Categories: []string{"flood"}, CostModel: "free_key", Availability: "requires_key", CredentialEnvVar: "GISTDA_API_KEY", ReferenceURI: "https://disaster.gistda.or.th/services/open-api", UsageNote: "Open flood and repeated-flood layers; account/API key and quota confirmation are required.", DataQualityNote: "Dataset date, spatial coverage and methodology must be preserved per result."},
		{ID: "open-charge-map", Name: "Open Charge Map", Categories: []string{"competition"}, CostModel: "free_key", Availability: "requires_key", CredentialEnvVar: "OPEN_CHARGE_MAP_API_KEY", ReferenceURI: "https://openchargemap.io/develop", UsageNote: "Open community charging-location API with provider attribution.", DataQualityNote: "Coverage and operating status may be incomplete; cross-check against other sources."},
		{ID: "worldpop", Name: "WorldPop Global 2 Population API", Categories: []string{"population"}, CostModel: "free_no_key", Availability: "active", ReferenceURI: "https://api.worldpop.org/v2/", UsageNote: "No-key API aggregation within a generated site-radius polygon; results are cached for 30 days.", DataQualityNote: "Modelled population is labelled estimated, with dataset year, source version, resolution and methodology."},
		{ID: "thai-open-data", Name: "Open Government Data of Thailand", Categories: []string{"population", "poi", "competition", "gis"}, CostModel: "free_no_key", Availability: "planned_import", ReferenceURI: "https://data.go.th/", UsageNote: "Import selected datasets only after checking licence, update date and geographic coverage.", DataQualityNote: "Coverage varies by agency and dataset; local datasets must not be presented as nationwide."},
		{ID: "google-maps", Name: "Google Maps Platform", Categories: []string{"map", "geocoding", "places", "routes"}, CostModel: "paid_billing", Availability: "deferred_paid", CredentialEnvVar: "VITE_GOOGLE_MAPS_BROWSER_API_KEY / GOOGLE_MAPS_SERVER_API_KEY", ReferenceURI: "https://developers.google.com/maps/billing-and-pricing/pricing", UsageNote: "Monthly free usage caps exist, but a billing account and payment method are required.", DataQualityNote: "Use only after user approval, restricted keys, quotas and cost monitoring are configured."},
		{ID: "utility-capacity", Name: "MEA / PEA utility confirmation", Categories: []string{"electrical"}, CostModel: "manual_or_contract", Availability: "unavailable", ReferenceURI: "https://www.mea.or.th/", UsageNote: "No generic public API has been approved for verified site capacity.", DataQualityNote: "Never claim verified grid capacity without an actual utility document or provider response."},
	}
}
