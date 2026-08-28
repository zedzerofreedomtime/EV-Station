# Data source policy: free first

The 30-day MVP enables free and open providers before any provider that requires billing. A free allowance is not treated as a free provider when a payment method or billing account is mandatory.

## Enabled without payment

| Provider | Purpose | Credential | Result status | Operational constraints |
|---|---|---|---|---|
| OpenStreetMap Overpass | POIs, mapped charging stations and road-accessibility proxy | None | POI/charger observations verified; road proxy preliminary; no score | Road classes and nearest mapped geometry are not vehicle counts, speeds, congestion or AADT |
| WorldPop Global 2 API | Modelled population within the selected radius | None (1,000 requests/day published for no-key access) | Estimated; no score | Record dataset year, source version, 100m/1km resolution, circular-boundary approximation and retrieval date |
| OpenStreetMap Nominatim | User-triggered address search | None | Preliminary match | No autocomplete, maximum one public request per second, cache results, and require user confirmation |
| OpenStreetMap embedded map | Location preview | None | Context only | Show attribution; never treat the map as cadastral or survey evidence |

## Free but waiting for setup

| Provider | Purpose | Required setup | Status until configured |
|---|---|---|---|
| TomTom | Geocoding, routing, traffic flow and incidents | Free developer API key; no payment card required under the current published free tier | Missing |
| GISTDA Disaster Open API | Recent flood (1/3/7/30 days) and repeated-flood layers | Free account/API key and confirmation of current quota/terms | Missing until `GISTDA_API_KEY` is configured; never infer no flood from a missing response |
| Open Charge Map | Charging station cross-check | Registered API key and provider attribution | Missing |
| data.go.th | Selected Thai government datasets | Dataset-by-dataset licence, date, geographic coverage and schema review | Missing until imported |

## Paid or manual providers: disabled

| Provider | Why it is deferred | User notification required before enabling |
|---|---|---|
| Google Maps Platform | Requires Cloud Billing even where a monthly free usage cap applies | Explain billing, quotas, restricted keys and possible overage before creating keys or enabling billing |
| Commercial historical traffic/count datasets | Free traffic-flow products do not prove vehicle count or AADT | Explain dataset coverage, licence, price and intended deterministic calculation |
| MEA/PEA capacity confirmation | No approved generic public API proves available capacity for a specific plot | Explain that a utility document, site survey or formal response is required |

## Non-negotiable labels

- A provider response is not automatically a score.
- Missing coverage is not proof of absence.
- WorldPop and other modelled datasets are `estimated`, not `verified`.
- Address-geocoding results are `preliminary` until the user confirms the coordinates.
- Traffic speed or congestion is not a traffic count.
- An OSM road-accessibility proxy is not traffic volume, speed, congestion or AADT.
- Electrical capacity stays `missing` unless an actual MEA/PEA source is attached.
- ROI is calculated only from explicit assumptions by deterministic backend logic.
