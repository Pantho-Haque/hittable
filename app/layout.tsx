import Providers from "@/context/providers";
import type { Metadata } from "next";
import { Geist, Geist_Mono } from "next/font/google";

import "@/styles/app.css";
import "@/styles/theme.css";
import Topbar from "@/components/homepage/TopBar";

const geistSans = Geist({
  variable: "--font-geist-sans",
  subsets: ["latin"],
});

const geistMono = Geist_Mono({
  variable: "--font-geist-mono",
  subsets: ["latin"],
});

export const metadata: Metadata = {
  title: "Hittable | API client for requests that live in your repo",
  description: "Open-source API client for the browser. Sign in with GitHub and your API requests are plain .hit files in your repository: send them, edit them and commit them back. Or open a local folder. No database, no cloud workspace.",
  keywords: ["Hittable", "API client", "HTTP client", "GitHub", "API requests in git", "open source", "Postman alternative", "Insomnia alternative", "terminal API client"],
  authors: [{ name: "Pantho Haque", url: "https://panthohaque.vercel.app" }],
  creator: "Pantho Haque",
  publisher: "Pantho Haque",
  formatDetection: {
    email: false,
    address: false,
    telephone: false,
  },
  metadataBase: new URL('https://hittable.vercel.app'),
  alternates: {
    canonical: '/',
  },
  manifest: '/manifest.json',
  openGraph: {
    title: 'Hittable: API requests that live in your repo',
    description: 'Your API requests as .hit files in your GitHub repo: send, edit and commit them back. Open source.',
    url: 'https://hittable.vercel.app/',
    siteName: 'Hittable',
    images: [
      {
        url: '/assets/images/logo.png',
        width: 334,
        height: 336,
        alt: 'Hittable Logo',
      },
    ],
    locale: 'en_US',
    type: 'website',
  },
  twitter: {
    card: 'summary',
    title: 'Hittable: API requests that live in your repo',
    description: 'Your API requests as .hit files in your GitHub repo: send, edit and commit them back. Open source.',
    images: ['/assets/images/logo.png'],
  },
  robots: {
    index: true,
    follow: true,
    googleBot: {
      index: true,
      follow: true,
      'max-video-preview': -1,
      'max-image-preview': 'large',
      'max-snippet': -1,
    },
  },
};

const jsonLd = {
  '@context': 'https://schema.org',
  '@type': 'SoftwareApplication',
  name: 'Hittable',
  operatingSystem: 'Web, macOS, Linux',
  applicationCategory: 'DeveloperApplication',
  offers: {
    '@type': 'Offer',
    price: '0',
    priceCurrency: 'USD',
  },
  author: {
    '@type': 'Person',
    name: 'Pantho Haque',
    url: 'https://panthohaque.vercel.app',
  },
  description: 'Open-source API client for the browser. Sign in with GitHub and your API requests are plain .hit files in your repository: send them, edit them and commit them back. Or open a local folder. No database, no cloud workspace.',
  isAccessibleForFree: true,
  image: 'https://hittable.vercel.app/assets/images/logo.png',
  url: 'https://hittable.vercel.app/',
};

export default async function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {

  return (
    <html lang="en">
      <body
        className={`${geistSans.variable} ${geistMono.variable} antialiased w-screen h-screen`}
      >
        <script
          type="application/ld+json"
          dangerouslySetInnerHTML={{ __html: JSON.stringify(jsonLd) }}
        />
        <Providers>
          <Topbar />
          {children}
        </Providers>
      </body>
    </html>
  );
}