import { useQuery } from '@tanstack/react-query'
import { ArrowRight, MapPin, Plus } from 'lucide-react'
import { Link } from 'react-router-dom'
import { DataStatus, DataStatusLegend } from '../components/DataStatus'
import { MapPanel } from '../components/MapPanel'
import { ErrorState, LoadingState } from '../components/PageState'
import { api } from '../services/api'
import { useI18n } from '../i18n/I18nProvider'

export function DashboardPage() {
  const { t } = useI18n()
  const sites = useQuery({ queryKey: ['sites'], queryFn: api.listSites })
  return <div>
    <div className="flex flex-wrap items-center justify-between gap-4"><div><h1 className="page-title">{t('Dashboard')}</h1><p className="mt-2 text-sm text-muted">{t('Candidate site screening and evidence readiness')}</p></div><Link to="/sites/new" className="button-primary"><Plus size={18} />{t('New Site')}</Link></div>
    <section className="mt-8 overflow-hidden rounded-xl border border-line bg-white shadow-panel">
      <div className="flex items-center justify-between border-b border-line px-6 py-5"><div><h2 className="section-title">{t('Work queue')}</h2><p className="mt-1 text-sm text-muted">{t('Sites awaiting review or provider data')}</p></div><span className="text-sm text-muted">{sites.data?.length ?? 0} {t('sites')}</span></div>
      {sites.isLoading && <LoadingState label={t('Loading candidate sites…')} />}
      {sites.isError && <div className="p-6"><ErrorState message={sites.error.message} /></div>}
      {sites.data?.length === 0 && <div className="grid min-h-64 place-items-center px-6 text-center"><div><MapPin className="mx-auto text-brand" size={34} strokeWidth={1.6}/><h3 className="mt-4 text-lg font-bold">{t('No candidate sites yet')}</h3><p className="mt-2 text-sm text-muted">{t('Create the first site to start an evidence-based assessment.')}</p><Link to="/sites/new" className="button-secondary mt-5">{t('Create site')}</Link></div></div>}
      {!!sites.data?.length && <div className="overflow-x-auto"><table className="w-full min-w-[840px] text-left"><thead className="border-b border-line bg-slate-50/60 text-xs uppercase tracking-wide text-muted"><tr><th className="px-6 py-4">Site</th><th className="px-5 py-4">Location</th><th className="px-5 py-4">Data readiness</th><th className="px-5 py-4">Assessment</th><th className="px-5 py-4">Updated</th><th className="px-6 py-4 text-right">Action</th></tr></thead><tbody className="divide-y divide-line">{sites.data.map(site => <tr key={site.id} className="transition hover:bg-slate-50"><td className="px-6 py-5 font-semibold">{site.name}</td><td className="px-5 py-5 text-sm text-muted">{site.address || (site.latitude !== undefined ? `${site.latitude.toFixed(5)}, ${site.longitude?.toFixed(5)}` : 'Not available')}</td><td className="px-5 py-5"><DataStatus status="missing" compact /></td><td className="px-5 py-5"><DataStatus status={site.inputStatus} compact /></td><td className="px-5 py-5 text-sm text-muted">{new Date(site.updatedAt).toLocaleDateString()}</td><td className="px-6 py-5 text-right"><Link to={`/sites/${site.id}`} className="inline-flex items-center gap-2 text-sm font-semibold text-brand hover:underline">Open <ArrowRight size={15}/></Link></td></tr>)}</tbody></table></div>}
    </section>
    {!!sites.data?.length && <section className="mt-6"><div className="mb-3 flex items-center justify-between"><div><h2 className="section-title">{t('Candidate map')}</h2><p className="mt-1 text-sm text-muted">{t('Selected site: ')}{sites.data[0].name}</p></div><Link to={`/sites/${sites.data[0].id}`} className="text-sm font-semibold text-brand hover:underline">{t('Open')} site</Link></div><MapPanel latitude={sites.data[0].latitude} longitude={sites.data[0].longitude} className="min-h-[330px]"/></section>}
    <section className="mt-6 rounded-xl border border-line px-6 py-5"><h2 className="section-title">{t('Data status guide')}</h2><p className="mb-5 mt-1 text-sm text-muted">{t('Every result is labelled by evidence quality. Labels are part of the result, not decoration.')}</p><DataStatusLegend /></section>
  </div>
}
