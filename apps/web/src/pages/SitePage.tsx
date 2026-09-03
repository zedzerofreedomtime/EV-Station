import { useMutation, useQuery } from '@tanstack/react-query'
import { ArrowLeft, Database, MapPin, Play } from 'lucide-react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import { DataStatus } from '../components/DataStatus'
import { MapPanel, RadiusLabel } from '../components/MapPanel'
import { ErrorState, LoadingState } from '../components/PageState'
import { useI18n } from '../i18n/I18nProvider'
import { api } from '../services/api'

export function SitePage() {
  const { t } = useI18n()
  const { id = '' } = useParams()
  const navigate = useNavigate()
  const site = useQuery({ queryKey: ['site', id], queryFn: () => api.getSite(id), enabled: Boolean(id) })
  const analysis = useMutation({ mutationFn: () => api.runAnalysis(id, 3000), onSuccess: run => navigate(`/analysis/${run.id}`) })
  if (site.isLoading) return <LoadingState label={t('Loading site…')} />
  if (site.isError || !site.data) return <ErrorState error={site.error} message="error.SITE_NOT_FOUND" />
  const value = site.data
  return <div><Link to="/" className="inline-flex items-center gap-2 text-sm font-semibold text-muted hover:text-ink"><ArrowLeft size={16}/>{t('Dashboard')}</Link><div className="mt-5 flex flex-wrap items-start justify-between gap-5"><div><h1 className="page-title">{value.name}</h1><div className="mt-3 flex flex-wrap items-center gap-4"><DataStatus status={value.inputStatus}/><span className="text-sm text-muted">{t('Customer-supplied inputs have not been independently verified.')}</span></div></div><button className="button-primary" onClick={() => analysis.mutate()} disabled={analysis.isPending}><Play size={17}/>{t(analysis.isPending ? 'Running…' : 'Run Analysis')}</button></div>
    <div className="mt-8 grid gap-6 xl:grid-cols-[0.72fr_1.28fr]"><section className="rounded-xl border border-line p-6 shadow-panel"><h2 className="section-title">{t('Site record')}</h2><dl className="mt-6 space-y-5"><Detail icon={MapPin} label={t('Location')} value={value.address || `${value.latitude}, ${value.longitude}`}/><Detail icon={Database} label={t('Land size')} value={`${value.landSize.toLocaleString()} ${t(value.landSizeUnit)}`}/></dl><div className="mt-7 border-t border-line pt-5"><h3 className="text-sm font-bold">{t('Input provenance')}</h3><p className="mt-2 text-sm leading-6 text-muted">{t('Source: User supplied')}<br/>{t('Status: Preliminary')}<br/>{t('Verification: Not completed')}</p></div></section><section><div className="mb-3 flex items-center justify-between"><h2 className="section-title">{t('Map')}</h2><RadiusLabel meters={3000}/></div><MapPanel latitude={value.latitude} longitude={value.longitude} className="min-h-[480px]"/></section></div>
    {analysis.isError && <div className="mt-6"><ErrorState error={analysis.error}/></div>}
  </div>
}

function Detail({ icon: Icon, label, value }: { icon: typeof MapPin; label: string; value: string }) { return <div className="flex gap-3"><Icon size={18} className="mt-0.5 text-brand"/><div><dt className="text-xs font-bold uppercase tracking-wide text-muted">{label}</dt><dd className="mt-1 text-sm font-medium">{value}</dd></div></div> }
