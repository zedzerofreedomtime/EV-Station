# Data source policy: free first

The 30-day MVP enables free and open providers before any provider that requires billing. A free allowance is not treated as a free provider when a payment method or billing account is mandatory.

## Enabled without payment

| Provider | Purpose | Credential | Result status | Operational constraints |
|---|---|---|---|---|
| OpenStreetMap Overpass | POIs, mapped charging stations and road-accessibility proxy | None | POI/charger observations verified; road proxy preliminary; no score | Road classes and nearest mapped geometry are not vehicle counts, speeds, congestion or AADT |
| Suphan Buri Provincial Office EV charging points | Published charger records within Suphan Buri | None | Verified source-record count; no score | CSV coordinates are extracted from destination links; repeated connector rows are deduplicated by station name and coordinates. Coverage is Suphan Buri only. |
| Saraburi Provincial Energy Office stations API | Published EV-labelled station records within Saraburi | None | Verified source-record count; no score | Coverage is Saraburi only. Records are displayed as published and do not independently prove current operation. |
| WorldPop Global 2 API | Modelled population within the selected radius | None (1,000 requests/day published for no-key access) | Estimated; no score | Record dataset year, source version, 100m/1km resolution, circular-boundary approximation and retrieval date |
| GISTDA Flood Risk Layer (ArcGIS REST) | Published flood-risk polygon overlap within the selected radius | None | Verified layer-overlap observation; no score | A no-overlap response does not prove no flood risk; preserve retrieval date, layer source and coverage limitation |
| Department of Highways AADT + road control sections | Annual average daily traffic on an exactly matched road/control section | None | Verified; no score | AADT CSV year 2568 is matched only where road number and control section also match the public DOH road geometry. It is not a live count at the site entrance; geometry is described by its publisher as a Dec 2022 snapshot. |
| MEA Power Map (via Thailand Power Map) | Published station-area guideline for the configured year and voltage in MEA coverage | None | Preliminary; no score | Point-in-polygon result only. It does not verify remaining capacity, supply capability, connection feasibility or reinforcement requirements for the plot. |
| OpenStreetMap Nominatim | User-triggered address search | None | Preliminary match | No autocomplete, maximum one public request per second, cache results, and require user confirmation |
| OpenStreetMap embedded map | Location preview | None | Context only | Show attribution; never treat the map as cadastral or survey evidence |

## AI advisory (not a data source)

| Provider | Purpose | Credential | Output boundary | Operational constraints |
|---|---|---|---|---|
| Gemini API | Explain an already-completed analysis in Thai or English | `GEMINI_API_KEY` on the API server only | AI-generated explanatory text; not a metric, score, financial calculation or factual provider | The API payload excludes customer name, address, coordinates and example-place lists. Responses are structured JSON, validated, and clearly labelled as AI-assisted. |

## Free but waiting for setup

| Provider | Purpose | Required setup | Status until configured |
|---|---|---|---|
| TomTom | Geocoding, routing, traffic flow and incidents | Free developer API key; no payment card required under the current published free tier | Missing |
| GISTDA Disaster Open API | Recent flood (1/3/7/30 days) and repeated-flood layers | Free account/API key and confirmation of current quota/terms | Prepared separately; never infer no flood from a missing response |
| Open Charge Map | Charging station cross-check | Registered API key and provider attribution | Missing |
| Other data.go.th datasets | Additional provincial charging-station sources | Dataset-by-dataset licence, date, geographic coverage and schema review | Missing until individually imported and tested |

## Paid or manual providers: disabled

| Provider | Why it is deferred | User notification required before enabling |
|---|---|---|
| Google Maps Platform | Requires Cloud Billing even where a monthly free usage cap applies | Explain billing, quotas, restricted keys and possible overage before creating keys or enabling billing |
| Commercial historical traffic/count datasets | Free traffic-flow products do not prove vehicle count or AADT | Explain dataset coverage, licence, price and intended deterministic calculation |
| MEA/PEA capacity confirmation | No approved generic public API proves available capacity for a specific plot | Explain that a utility document, site survey or formal response is required |

## Non-negotiable labels

- A provider response is not automatically a score.
- A combined competitor count is a deduplicated union of connected sources. Source coverage is shown separately and zero returned records never prove absence of competitors.
- Missing coverage is not proof of absence.
- WorldPop and other modelled datasets are `estimated`, not `verified`.
- Address-geocoding results are `preliminary` until the user confirms the coordinates.
- Traffic speed or congestion is not a traffic count.
- An OSM road-accessibility proxy is not traffic volume, speed, congestion or AADT.
- A Department of Highways AADT value is an annual average daily traffic figure for the nearest exactly matched control section, not an observation of vehicles entering the submitted plot.
- A GISTDA flood-risk polygon overlap is not a flood forecast, water-depth estimate, loss estimate or a conclusion that a site has no risk.
- Electrical capacity stays `missing` unless an actual MEA/PEA source is attached.
- A public Thailand/MEA Power Map match is `preliminary`; a formal utility technical response is still required before electrical capacity can be considered verified.
- ROI is calculated only from explicit assumptions by deterministic backend logic.
- Gemini may explain deterministic results and identify checks to complete, but must not calculate or alter any score, ROI, payback, factual metric or source status.
