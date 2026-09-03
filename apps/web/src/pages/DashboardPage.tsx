import { useMutation, useQueries, useQuery, useQueryClient } from '@tanstack/react-query'
import { ArrowRight, MapPin, Pencil, Plus, Trash2 } from 'lucide-react'
import { useState } from 'react'
import { Link } from 'react-router-dom'
import { DataStatus, DataStatusLegend } from '../components/DataStatus'
import { MapPanel } from '../components/MapPanel'
import { ErrorState, LoadingState } from '../components/PageState'
import { api } from '../services/api'
import { useI18n } from '../i18n/I18nProvider'

export function DashboardPage() {
  const { t, language } = useI18n()
  const queryClient = useQueryClient()
  const [siteToDelete, setSiteToDelete] = useState<{ id: string; name: string } | null>(null)
  const sites = useQuery({ queryKey: ['sites'], queryFn: api.listSites })
  const deleteSite = useMutation({ mutationFn: api.deleteSite, onSuccess: async () => { await queryClient.invalidateQueries({ queryKey: ['sites'] }); setSiteToDelete(null) } })
  const latestAnalyses = useQueries({ queries: (sites.data ?? []).map(site => ({ queryKey: ['latest-analysis', site.id], queryFn: () => api.getLatestAnalysisForSite(site.id), staleTime: 60_000 })) })
  const latestAnalysisFor = (siteId: string) => latestAnalyses[sites.data?.findIndex(site => site.id === siteId) ?? -1]?.data
  const destinationFor = (siteId: string) => {
    const latest = latestAnalysisFor(siteId)
    return latest ? `/analysis/${latest.id}` : `/sites/${siteId}`
  }
  return <div>
    <div className="flex flex-wrap items-center justify-between gap-4"><div><h1 className="page-title">{t('Dashboard')}</h1><p className="mt-2 text-sm text-muted">{t('Candidate site screening and evidence readiness')}</p></div><Link to="/sites/new" className="button-primary"><Plus size={18} />{t('New Site')}</Link></div>
    <section className="mt-8 overflow-hidden rounded-xl border border-line bg-white shadow-panel">
      <div className="flex items-center justify-between border-b border-line px-6 py-5"><div><h2 className="section-title">{t('Work queue')}</h2><p className="mt-1 text-sm text-muted">{t('Sites awaiting review or provider data')}</p></div><span className="text-sm text-muted">{sites.data?.length ?? 0} {t('sites')}</span></div>
      {sites.isLoading && <LoadingState label={t('Loading candidate sites…')} />}
      {sites.isError && <div className="p-6"><ErrorState error={sites.error} /></div>}
      {sites.data?.length === 0 && <div className="grid min-h-64 place-items-center px-6 text-center"><div><MapPin className="mx-auto text-brand" size={34} strokeWidth={1.6}/><h3 className="mt-4 text-lg font-bold">{t('No candidate sites yet')}</h3><p className="mt-2 text-sm text-muted">{t('Create the first site to start an evidence-based assessment.')}</p><Link to="/sites/new" className="button-secondary mt-5">{t('Create site')}</Link></div></div>}
      {!!sites.data?.length && <div className="overflow-x-auto"><table className="w-full min-w-[980px] text-left"><thead className="border-b border-line bg-slate-50/60 text-xs uppercase tracking-wide text-muted"><tr><th className="px-6 py-4">{t('Site')}</th><th className="px-5 py-4">{t('Location')}</th><th className="px-5 py-4">{t('Data readiness')}</th><th className="px-5 py-4">{t('Assessment')}</th><th className="px-5 py-4">{t('Updated')}</th><th className="px-6 py-4 text-right">{t('Action')}</th></tr></thead><tbody className="divide-y divide-line">{sites.data.map(site => { const latest = latestAnalysisFor(site.id); return <tr key={site.id} className="transition hover:bg-slate-50"><td className="px-6 py-5 font-semibold">{site.name}</td><td className="px-5 py-5 text-sm text-muted">{site.address || (site.latitude !== undefined ? `${site.latitude.toFixed(5)}, ${site.longitude?.toFixed(5)}` : t('Not available'))}</td><td className="px-5 py-5"><DataStatus status={latest?.assessmentStatus ?? 'missing'} compact /></td><td className="px-5 py-5"><DataStatus status={latest?.assessmentStatus ?? site.inputStatus} compact /></td><td className="px-5 py-5 text-sm text-muted">{new Date(site.updatedAt).toLocaleDateString(language === 'th' ? 'th-TH' : 'en-US')}</td><td className="px-6 py-5"><div className="flex items-center justify-end gap-3"><Link to={destinationFor(site.id)} className="inline-flex items-center gap-1.5 text-sm font-semibold text-brand hover:underline">{t(latest ? 'View analysis' : 'Open')} <ArrowRight size={15}/></Link><Link to={`/sites/${site.id}/edit`} aria-label={`${t('Edit')}: ${site.name}`} className="inline-flex items-center gap-1.5 text-sm font-semibold text-slate-700 hover:text-brand"><Pencil size={15}/>{t('Edit')}</Link><button type="button" aria-label={`${t('Delete')}: ${site.name}`} className="inline-flex items-center gap-1.5 text-sm font-semibold text-red-600 hover:text-red-700" onClick={() => setSiteToDelete({ id: site.id, name: site.name })}><Trash2 size={15}/>{t('Delete')}</button></div></td></tr> })}</tbody></table></div>}
    </section>
    {!!sites.data?.length && <section className="mt-6"><div className="mb-3 flex items-center justify-between"><div><h2 className="section-title">{t('Candidate map')}</h2><p className="mt-1 text-sm text-muted">{t('Selected site: ')}{sites.data[0].name}</p></div><Link to={destinationFor(sites.data[0].id)} className="text-sm font-semibold text-brand hover:underline">{t(latestAnalysisFor(sites.data[0].id) ? 'View analysis' : 'Open')}</Link></div><MapPanel latitude={sites.data[0].latitude} longitude={sites.data[0].longitude} className="min-h-[330px]"/></section>}
    <section className="mt-6 rounded-xl border border-line px-6 py-5"><h2 className="section-title">{t('Data status guide')}</h2><p className="mb-5 mt-1 text-sm text-muted">{t('Every result is labelled by evidence quality. Labels are part of the result, not decoration.')}</p><DataStatusLegend /></section>
    {siteToDelete && <div className="fixed inset-0 z-50 grid place-items-center bg-slate-950/40 p-4" role="presentation"><section role="dialog" aria-modal="true" aria-labelledby="delete-site-title" className="w-full max-w-md rounded-2xl bg-white p-6 shadow-2xl"><h2 id="delete-site-title" className="text-xl font-bold text-ink">{t('Delete this site?')}</h2><p className="mt-3 text-sm leading-6 text-muted">{t('Deleting this site will also permanently remove its analysis results. This cannot be undone.')}</p><p className="mt-3 rounded-lg bg-slate-50 px-3 py-2 text-sm font-semibold text-ink">{siteToDelete.name}</p>{deleteSite.isError && <p className="mt-3 text-sm text-red-600">{t('Unable to delete this site. Please try again.')}</p>}<div className="mt-6 flex justify-end gap-3"><button type="button" className="button-secondary" disabled={deleteSite.isPending} onClick={() => setSiteToDelete(null)}>{t('Cancel')}</button><button type="button" className="button-danger" disabled={deleteSite.isPending} onClick={() => deleteSite.mutate(siteToDelete.id)}>{deleteSite.isPending ? t('Deleting…') : t('Delete site')}</button></div></section></div>}
  </div>
}
