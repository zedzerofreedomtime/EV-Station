import type { Config } from 'tailwindcss'

export default {
  content: ['./index.html', './src/**/*.{js,ts,jsx,tsx}'],
  theme: {
    extend: {
      colors: {
        ink: '#10254a',
        muted: '#66758f',
        line: '#dbe2ea',
        brand: '#079455',
        canvas: '#f5f7fa',
      },
      boxShadow: { panel: '0 8px 30px rgba(16, 37, 74, 0.06)' },
    },
  },
  plugins: [],
} satisfies Config
