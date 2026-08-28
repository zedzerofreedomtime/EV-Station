import { lazy, Suspense } from 'react'
import { Navigate, Route, Routes } from 'react-router-dom'
import { AppShell } from './components/AppShell'
import { LoadingState } from './components/PageState'
import { useI18n } from './i18n/I18nProvider'

const AnalysisPage = lazy(() => import('./pages/AnalysisPage').then(module => ({ default: module.AnalysisPage })))
const DashboardPage = lazy(() => import('./pages/DashboardPage').then(module => ({ default: module.DashboardPage })))
const NewSitePage = lazy(() => import('./pages/NewSitePage').then(module => ({ default: module.NewSitePage })))
const SitePage = lazy(() => import('./pages/SitePage').then(module => ({ default: module.SitePage })))
const DataSourcesPage = lazy(() => import('./pages/DataSourcesPage').then(module => ({ default: module.DataSourcesPage })))

function Placeholder({ title }: { title: string }) { return <div><h1 className="page-title">{title}</h1><p className="mt-3 text-muted">This module is prepared for a later MVP iteration.</p></div> }

export default function App() {
  const { t } = useI18n()
  return <AppShell><Suspense fallback={<LoadingState label={t('Loading page…')}/> }><Routes>
    <Route path="/" element={<DashboardPage />} />
    <Route path="/sites/new" element={<NewSitePage />} />
    <Route path="/sites/:id" element={<SitePage />} />
    <Route path="/analysis/:id" element={<AnalysisPage />} />
    <Route path="/analyses" element={<Placeholder title={t('Analyses')} />} />
    <Route path="/data-sources" element={<DataSourcesPage />} />
    <Route path="/settings" element={<Placeholder title={t('Settings')} />} />
    <Route path="*" element={<Navigate to="/" replace />} />
  </Routes></Suspense></AppShell>
}
