'use client';

import Link from 'next/link';
import { usePathname } from 'next/navigation';
import { cn } from '@/lib/utils';

const creatorLinks = [
  { href: '/creator', label: 'Overview', icon: '📊' },
  { href: '/creator/agents', label: 'My Agents', icon: '🤖' },
  { href: '/creator/analytics', label: 'Analytics', icon: '📈' },
  { href: '/creator/earnings', label: 'Earnings', icon: '💰' },
];

const developerLinks = [
  { href: '/developer', label: 'Overview', icon: '📊' },
  { href: '/developer/keys', label: 'API Keys', icon: '🔑' },
  { href: '/developer/usage', label: 'Usage', icon: '📈' },
  { href: '/developer/billing', label: 'Billing', icon: '💳' },
];

export function Sidebar() {
  const pathname = usePathname();
  const isCreator = pathname.startsWith('/creator');
  const links = isCreator ? creatorLinks : developerLinks;

  return (
    <aside className="hidden w-64 border-r bg-muted/30 lg:block">
      <nav className="flex flex-col gap-2 p-4">
        <div className="mb-4 px-2 text-sm font-semibold text-muted-foreground">
          {isCreator ? 'Creator Dashboard' : 'Developer Console'}
        </div>
        {links.map((link) => (
          <Link
            key={link.href}
            href={link.href}
            className={cn(
              'flex items-center gap-3 rounded-md px-3 py-2 text-sm transition-colors',
              pathname === link.href
                ? 'bg-primary text-primary-foreground'
                : 'hover:bg-muted'
            )}
          >
            <span>{link.icon}</span>
            {link.label}
          </Link>
        ))}
      </nav>
    </aside>
  );
}
