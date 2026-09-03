import { CircleF, GoogleMap, LoadScript, MarkerF } from '@react-google-maps/api'
import { MapPin, Radio } from 'lucide-react'
import { useI18n } from '../i18n/I18nProvider'

interface MapPanelProps { latitude?: number; longitude?: number; radiusMeters?: number; className?: string }

export function MapPanel({ latitude, longitude, radiusMeters = 3000, className = '' }: MapPanelProps) {
  const { t } = useI18n()
  const key = import.meta.env.VITE_GOOGLE_MAPS_BROWSER_API_KEY
  const hasCoordinates = latitude !== undefined && longitude !== undefined
  if (!hasCoordinates) {
    return <div className={`relative grid min-h-72 place-items-center overflow-hidden rounded-xl border border-line bg-[#eef3f7] ${className}`}>
      <div className="absolute inset-0 opacity-45 map-grid" />
      <div className="relative max-w-sm px-8 text-center"><span className="mx-auto grid h-12 w-12 place-items-center rounded-full bg-white text-brand shadow-panel"><MapPin /></span><h3 className="mt-4 font-bold">{t('Map preview unavailable')}</h3><p className="mt-2 text-sm leading-6 text-muted">{t('Search an address or enter valid coordinates to preview this location.')}</p></div>
    </div>
  }
  if (!key) {
    const mapPadding = 1.35
    const deltaLatitude = radiusMeters * mapPadding / 111_320
    const longitudeScale = Math.max(Math.cos(latitude * Math.PI / 180), 0.2)
    const deltaLongitude = radiusMeters * mapPadding / (111_320 * longitudeScale)
    const west = longitude - deltaLongitude
    const south = latitude - deltaLatitude
    const east = longitude + deltaLongitude
    const north = latitude + deltaLatitude
    const bbox = [west, south, east, north].map(value => value.toFixed(6)).join('%2C')
    const marker = `${latitude.toFixed(6)}%2C${longitude.toFixed(6)}`
    const source = `https://www.openstreetmap.org/export/embed.html?bbox=${bbox}&layer=mapnik&marker=${marker}`
    return <div className={`overflow-hidden rounded-xl border border-line bg-white ${className}`}>
      <iframe title={t('Map preview')} src={source} className="block h-full min-h-72 w-full border-0" loading="lazy" />
      <p className="border-t border-line bg-white px-3 py-2 text-xs text-muted">© <a className="font-semibold text-brand hover:underline" href="https://www.openstreetmap.org/copyright" target="_blank" rel="noreferrer">OpenStreetMap contributors</a> · {t('Interactive road map')}</p>
    </div>
  }
  const center = { lat: latitude, lng: longitude }
  return <div className={`overflow-hidden rounded-xl border border-line ${className}`}>
    <LoadScript googleMapsApiKey={key}>
      <GoogleMap center={center} zoom={18} mapContainerStyle={{width:'100%', height:'100%', minHeight:'288px'}} options={{streetViewControl:false,mapTypeControl:false,fullscreenControl:false,mapTypeId:'satellite'}}>
        <MarkerF position={center} />
        <CircleF center={center} radius={radiusMeters} options={{strokeColor:'#079455',strokeOpacity:.85,strokeWeight:2,fillColor:'#079455',fillOpacity:.08}} />
      </GoogleMap>
    </LoadScript>
  </div>
}

export function RadiusLabel({ meters }: { meters: number }) {
  const { t } = useI18n()
  return <span className="inline-flex items-center gap-2 text-sm font-semibold text-ink"><Radio size={16} className="text-brand" />{meters / 1000} {t('km radius')}</span>
}
