import type { AnalysisRun, CreateSiteInput, DataSourceCatalogEntry, FranchisePlan, GeocodingResult, Site } from '../types/domain'

const baseURL = import.meta.env.VITE_API_BASE_URL || 'http://localhost:8080/api/v1'

type Envelope<T> = { data: T }
type ErrorEnvelope = { error?: { code?: string; message?: string } }

export class APIError extends Error {
  constructor(public code: string, message: string, public status: number) { super(message) }
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const response = await fetch(`${baseURL}${path}`, {
    ...init,
    headers: { 'Content-Type': 'application/json', ...init?.headers },
  })
  const body = await response.json().catch(() => ({})) as Envelope<T> & ErrorEnvelope
  if (!response.ok) throw new APIError(body.error?.code || 'REQUEST_FAILED', body.error?.message || 'Request failed.', response.status)
  return body.data
}

export const api = {
  listSites: () => request<Site[]>('/sites'),
  getSite: (id: string) => request<Site>(`/sites/${id}`),
  createSite: (input: CreateSiteInput) => request<Site>('/sites', { method: 'POST', body: JSON.stringify(input) }),
  runAnalysis: (siteId: string, radiusMeters = 3000) => request<AnalysisRun>(`/sites/${siteId}/analyses`, { method: 'POST', body: JSON.stringify({ radiusMeters }) }),
  getAnalysis: (id: string) => request<AnalysisRun>(`/analyses/${id}`),
  searchAddress: (query: string) => request<GeocodingResult[]>(`/geocoding/search?q=${encodeURIComponent(query)}&limit=5`),
  getDataSources: () => request<DataSourceCatalogEntry[]>('/data-sources'),
  getFranchisePlans: () => request<FranchisePlan[]>('/financial/plans'),
}
