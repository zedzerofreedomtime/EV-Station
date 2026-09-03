import { AlertCircle, LoaderCircle } from 'lucide-react'
import { useI18n } from '../i18n/I18nProvider'
import { errorMessageKey } from '../services/api'

export function LoadingState({ label = 'Loading…' }: { label?: string }) { const { t } = useI18n(); return <div className="grid min-h-64 place-items-center text-muted"><span className="inline-flex items-center gap-2"><LoaderCircle className="animate-spin" size={18} />{t(label)}</span></div> }
export function ErrorState({ error, message }: { error?: unknown; message?: string }) { const { t } = useI18n(); return <div className="flex min-h-48 items-center justify-center gap-2 rounded-xl border border-red-200 bg-red-50 px-6 text-sm text-red-700"><AlertCircle size={18} />{t(error ? errorMessageKey(error) : message || 'error.REQUEST_FAILED')}</div> }
