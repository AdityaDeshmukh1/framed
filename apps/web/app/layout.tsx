/**
 * Root layout — sets up fonts, React Query provider, and PWA meta tags.
 * Uses Cormorant Garamond (serif), DM Sans (sans), JetBrains Mono (mono).
 */

import type { Metadata } from 'next'
import {
  Cormorant_Garamond,
  DM_Sans,
  JetBrains_Mono,
} from 'next/font/google'
import { ClerkProvider } from '@clerk/nextjs'
import { ReactQueryProvider } from '../lib/providers'
import './globals.css'

const cormorant = Cormorant_Garamond({
  variable: '--font-cormorant',
  subsets: ['latin'],
  weight: ['300', '400', '500', '600', '700'],
  style: ['normal', 'italic'],
  display: 'swap',
})

const dmSans = DM_Sans({
  variable: '--font-dm-sans',
  subsets: ['latin'],
  weight: ['300', '400', '500', '600', '700'],
  display: 'swap',
})

const jetbrainsMono = JetBrains_Mono({
  variable: '--font-jetbrains-mono',
  subsets: ['latin'],
  weight: ['400', '500', '600', '700'],
  display: 'swap',
})

export const metadata: Metadata = {
  title: 'Framed — A film about you',
  description: 'Your cinema. Your character. Your match.',
  manifest: '/manifest.json',
  appleWebApp: {
    capable: true,
    statusBarStyle: 'black-translucent',
    title: 'Framed',
  },
}

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode
}>) {
  return (
    <html
      lang="en"
      className={`${cormorant.variable} ${dmSans.variable} ${jetbrainsMono.variable} h-full antialiased`}
    >
      <head>
        <meta name="theme-color" content="#4A1B0C" />
        <meta name="apple-mobile-web-app-capable" content="yes" />
        <link rel="manifest" href="/manifest.json" />
      </head>
      <body className="min-h-full flex flex-col" style={{ background: 'var(--cream)' }}>
        <ClerkProvider afterSignOutUrl="/sign-in">
          <ReactQueryProvider>{children}</ReactQueryProvider>
        </ClerkProvider>
      </body>
    </html>
  )
}
