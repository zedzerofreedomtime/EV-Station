import { cleanup, render, screen } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it } from 'vitest'
import { DataStatus, DataStatusLegend } from './DataStatus'
import { I18nProvider } from '../i18n/I18nProvider'

afterEach(cleanup)
beforeEach(() => localStorage.setItem('rbc-language', 'en'))

describe('DataStatus', () => {
  it('renders the explicit evidence-quality label', () => {
    render(<I18nProvider><DataStatus status="estimated" /></I18nProvider>)
    expect(screen.getByText('Estimated data')).toBeTruthy()
  })

  it('shows all four required evidence states', () => {
    render(<I18nProvider><DataStatusLegend /></I18nProvider>)
    for (const label of ['Verified data', 'Estimated data', 'Preliminary assessment', 'Missing data']) {
      expect(screen.getByText(label)).toBeTruthy()
    }
  })
})
