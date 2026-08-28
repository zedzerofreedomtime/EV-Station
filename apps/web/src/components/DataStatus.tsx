import { AlertTriangle, Check, Circle, HelpCircle } from 'lucide-react'
import type { DataStatus as Status } from '../types/domain'
import { useI18n } from '../i18n/I18nProvider'

const labels: Record<Status, string> = {
  verified: 'Verified data', estimated: 'Estimated data', preliminary: 'Preliminary assessment', missing: 'Missing data',
}

export function DataStatus({ status, compact = false }: { status: Status; compact?: boolean }) {
  const { t } = useI18n()
  const classes = { verified: 'text-emerald-700', estimated: 'text-amber-700', preliminary: 'text-blue-700', missing: 'text-slate-500' }[status]
  const Icon = { verified: Check, estimated: AlertTriangle, preliminary: HelpCircle, missing: Circle }[status]
  return <span className={`inline-flex items-center gap-2 text-sm font-medium ${classes}`}><Icon size={compact ? 14 : 16} strokeWidth={2} />{t(labels[status])}</span>
}

export function DataStatusLegend() {
  const { t } = useI18n()
  return <div className="grid gap-4 border-t border-line pt-5 sm:grid-cols-2 xl:grid-cols-4">
    {(['verified','estimated','preliminary','missing'] as Status[]).map(status => <div key={status}><DataStatus status={status} /><p className="mt-1 pl-6 text-xs text-muted">{t({verified:'Source checked and ready to use',estimated:'Modelled or estimated',preliminary:'Requires further validation',missing:'Not currently available'}[status])}</p></div>)}
  </div>
}
