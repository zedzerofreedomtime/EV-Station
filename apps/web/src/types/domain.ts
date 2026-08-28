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
  startedAt: string
  completedAt?: string
  createdAt: string
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

export type DataSourceCostModel = 'free_no_key' | 'free_key' | 'free_download' | 'paid_billing' | 'manual_or_contract'

export interface DataSourceCatalogEntry {
  id: string
  name: string
  categories: string[]
  costModel: DataSourceCostModel
  availability: 'active' | 'requires_key' | 'planned_import' | 'deferred_paid' | 'unavailable'
  credentialEnvVar?: string
  referenceUri: string
  usageNote: string
  dataQualityNote: string
}
