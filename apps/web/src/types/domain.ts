export type DataStatus = 'verified' | 'estimated' | 'preliminary' | 'missing'

export interface Site {
  id: string
  name: string
  address?: string
  latitude?: number
  longitude?: number
  landSize: number
  landSizeUnit: 'sqm' | 'rai' | 'ngan' | 'sqwah'
  googleMapsUrl?: string
  notes?: string
  inputStatus: DataStatus
  createdAt: string
  updatedAt: string
}

export interface CreateSiteInput {
  name: string
  address?: string
  latitude?: number
  longitude?: number
  landSize: number
  landSizeUnit: Site['landSizeUnit']
  googleMapsUrl?: string
  notes?: string
}

export interface DataSource {
  name: string
  type: string
  authority?: 'official' | 'modelled' | 'community' | 'customer_supplied' | 'unknown'
  geographicScope?: 'province' | 'district' | 'subdistrict' | 'site_radius' | 'published_station_area' | 'plot'
  siteVerification?: 'verified_at_dataset_scope' | 'modelled_for_site_radius' | 'preliminary_map_lookup' | 'utility_confirmed'
  referenceUri?: string
  datasetVersion?: string
  observedAt?: string
  retrievedAt: string
  methodology?: string
  license?: string
}

export interface Metric {
  id: string
  analysisRunId: string
  type: string
  rawValue?: unknown
  normalizedScore?: number
  status: DataStatus
  source: DataSource
  assumptions: string[]
  createdAt: string
}

export interface FinancialResult {
  initialInvestment: number
  monthlyRevenue: number
  monthlyOperatingCost: number
  monthlyProfit: number
  annualProfit: number
  roiPercentage?: number
  paybackMonths?: number
  assumptions: string[]
}

export interface FranchisePlan {
  code: 'S' | 'M' | 'L'
  name: string
  recommendedAreaSqWah: number
  evChargingStations: number
  investmentMinThb: number
  investmentMaxThb?: number
  investmentUpperReferenceThb?: number
  investmentOpenEnded: boolean
  franchiseFeeThb: number
  locationProfile: string[]
  coreServices: string[]
  sourceStatus: 'user_supplied'
  sourceNote: string
  roiAvailable: false
  missingForRoi: string[]
}

export interface AnalysisRun {
  id: string
  siteId: string
  status: 'pending' | 'running' | 'completed' | 'failed'
  analysisRadiusMeters: number
  overallScore?: number
  assessmentStatus: DataStatus
  recommendation: string
  metrics: Metric[]
  financial?: FinancialResult
  scoring?: {
    version: string
    coveragePercentage: number
    scoredMetricCount: number
    requiredMetricCount: number
    excludedMetrics: string[]
    minimumCoveragePercentage: number
    limitations: string[]
  }
  startedAt: string
  completedAt?: string
  createdAt: string
}

export interface AIAssessment {
  summary: string
  recommendation: string
  strengths: string[]
  risks: string[]
  requiredChecks: string[]
  disclaimer: string
  language: 'th' | 'en'
  model: string
  generatedAt: string
}

export interface GeocodingResult {
  displayName: string
  latitude: number
  longitude: number
  category?: string
  placeType?: string
  status: DataStatus
  source: DataSource
  assumptions: string[]
}

export interface GoogleMapsResolution { inputUrl: string; resolvedUrl: string; latitude: number; longitude: number }

export type DataSourceCostModel = 'free_no_key' | 'free_key' | 'free_download' | 'free_public_map' | 'free_allowance_then_paid' | 'paid_billing' | 'manual_or_contract'

export interface DataSourceCatalogEntry {
  id: string
  name: string
  categories: string[]
  costModel: DataSourceCostModel
  availability: 'active' | 'active_reference' | 'active_selected_imports' | 'requires_key' | 'requires_account_and_key' | 'planned_import' | 'deferred_paid' | 'unavailable'
  credentialEnvVar?: string
  referenceUri: string
  usageNote: string
  dataQualityNote: string
}
