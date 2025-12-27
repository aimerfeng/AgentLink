import type { Metadata } from 'next';
import { Inter } from 'next/font/google';
import './globals.css';
import { Providers } from '@/components/providers';

const inter = Inter({ subsets: ['latin'] });

export const metadata: Metadata = {
  title: 'AgentLink - AI Agent Marketplace',
  description: 'Monetize your AI Agents and integrate AI capabilities with one API call',
  keywords: ['AI', 'Agent', 'API', 'Marketplace', 'LLM', 'GPT', 'Claude'],
  authors: [{ name: 'AgentLink' }],
  openGraph: {
    title: 'AgentLink - AI Agent Marketplace',
    description: 'Monetize your AI Agents and integrate AI capabilities with one API call',
    url: 'https://agentlink.io',
    siteName: 'AgentLink',
    type: 'website',
  },
  twitter: {
    card: 'summary_large_image',
    title: 'AgentLink - AI Agent Marketplace',
    description: 'Monetize your AI Agents and integrate AI capabilities with one API call',
  },
};

export default function RootLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <html lang="en" suppressHydrationWarning>
      <body className={inter.className}>
        <Providers>{children}</Providers>
      </body>
    </html>
  );
}
