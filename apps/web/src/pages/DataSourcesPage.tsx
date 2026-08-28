import { useQuery } from '@tanstack/react-query'
import { AlertTriangle, CheckCircle2, ExternalLink, KeyRound, WalletCards } from 'lucide-react'
import { ErrorState, LoadingState } from '../components/PageState'
import { useI18n } from '../i18n/I18nProvider'
import { api } from '../services/api'
import type { DataSourceCatalogEntry, DataSourceCostModel } from '../types/domain'

const freeModels = new Set<DataSourceCostModel>(['free_no_key', 'free_key', 'free_download'])

export function DataSourcesPage() {
  const { t } = useI18n()
  const sources = useQuery({ queryKey: ['data-sources'], queryFn: api.getDataSources, staleTime: 60 * 60 * 1000 })
  if (sources.isPending) return <LoadingState label={t('Loading data sources…')} />
  if (sources.isError) return <ErrorState message={sources.error.message} />
  const free = sources.data.filter(item => freeModels.has(item.costModel))
  const deferred = sources.data.filter(item => !freeModels.has(item.costModel))

  return <div>
    <h1 className="page-title">{t('Data Sources')}</h1>
    <p className="mt-2 max-w-3xl text-sm leading-6 text-muted">{t('The MVP uses free and open providers first. Paid services remain disabled until the user explicitly approves billing.')}</p>
    <section className="mt-8">
      <div className="flex items-center gap-2"><CheckCircle2 className="text-brand" size={20}/><h2 className="section-title">{t('Free-first providers')}</h2></div>
      <p className="mt-2 text-sm text-muted">{t('Active sources work now. Sources requiring a free key or dataset import are prepared but do not create factual results until configured.')}</p>
      <div className="mt-4 grid gap-4 lg:grid-cols-2">{free.map(source => <SourceCard key={source.id} source={source}/>)}</div>
    </section>
    <section className="mt-10 rounded-xl border border-amber-200 bg-amber-50/60 p-6">
      <div className="flex items-center gap-2"><WalletCards className="text-amber-700" size={20}/><h2 className="section-title">{t('Paid or manually verified sources')}</h2></div>
      <p className="mt-2 text-sm leading-6 text-amber-900/80">{t('These providers are documented separately and are not enabled. The application must notify the user before billing, a contract, or utility verification is required.')}</p>
      <div className="mt-4 grid gap-4 lg:grid-cols-2">{deferred.map(source => <SourceCard key={source.id} source={source}/>)}</div>
    </section>
  </div>
}

function SourceCard({ source }: { source: DataSourceCatalogEntry }) {
  const { t } = useI18n()
  return <article className="rounded-xl border border-line bg-white p-5 shadow-sm">
    <div className="flex flex-wrap items-start justify-between gap-3"><div><h3 className="font-bold text-ink">{source.name}</h3><p className="mt-1 text-xs uppercase tracking-wide text-muted">{source.categories.join(' · ')}</p></div><Availability value={source.availability}/></div>
    <dl className="mt-4 space-y-3 text-sm"><div><dt className="font-semibold text-ink">{t('Cost model')}</dt><dd className="mt-1 text-muted">{t(costLabels[source.costModel])}</dd></div><div><dt className="font-semibold text-ink">{t('Usage')}</dt><dd className="mt-1 leading-5 text-muted">{source.usageNote}</dd></div><div><dt className="font-semibold text-ink">{t('Data quality')}</dt><dd className="mt-1 leading-5 text-muted">{source.dataQualityNote}</dd></div></dl>
    {source.credentialEnvVar ? <p className="mt-4 flex items-center gap-2 rounded-lg bg-slate-50 px-3 py-2 font-mono text-xs text-slate-600"><KeyRound size={14}/>{source.credentialEnvVar}</p> : null}
    <a href={source.referenceUri} target="_blank" rel="noreferrer" className="mt-4 inline-flex items-center gap-1.5 text-sm font-semibold text-brand hover:underline">{t('Official source')}<ExternalLink size={14}/></a>
  </article>
}

const costLabels: Record<DataSourceCostModel, string> = {
  free_no_key: 'Free · no API key', free_key: 'Free allowance · API key required', free_download: 'Free dataset download', paid_billing: 'Billing required', manual_or_contract: 'Manual verification or contract required',
}

function Availability({ value }: { value: DataSourceCatalogEntry['availability'] }) {
  const { t } = useI18n()
  const active = value === 'active'
  const label = { active: 'Active now', requires_key: 'Waiting for free key', planned_import: 'Dataset import planned', deferred_paid: 'Disabled · paid', unavailable: 'Manual verification required' }[value]
  return <span className={`inline-flex items-center gap-1.5 rounded-full px-2.5 py-1 text-xs font-bold ${active ? 'bg-emerald-50 text-emerald-700' : 'bg-slate-100 text-slate-600'}`}>{active ? <CheckCircle2 size={13}/> : <AlertTriangle size={13}/>} {t(label)}</span>
}
