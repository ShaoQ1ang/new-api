import { Link, Outlet, useLocation } from 'react-router-dom';
import {
  Bell,
  LayoutDashboard,
  Key,
  Layers,
  Search,
  Settings,
  User,
  Workflow,
} from 'lucide-react';
import { clsx } from 'clsx';
import { twMerge } from 'tailwind-merge';
import { useStatus } from '../hooks/useStatus';

function cn(...inputs: Array<string | boolean | undefined>) {
  return twMerge(clsx(inputs));
}

const navigation = [
  { name: 'Overview', href: '/console', icon: LayoutDashboard },
  { name: 'Channels', href: '/console/channels', icon: Layers },
  { name: 'Tokens', href: '/console/tokens', icon: Key },
  { name: 'Settings', href: '/console/settings', icon: Settings },
];

export default function AppLayout() {
  const location = useLocation();
  const status = useStatus();
  const systemName = status.data?.system_name || 'new-api';

  return (
    <div className='flex min-h-screen bg-[linear-gradient(180deg,_#f8fbff_0%,_#f8fafc_55%,_#f1f5f9_100%)]'>
      <aside className='hidden w-72 shrink-0 border-r border-slate-200/80 bg-white/85 px-5 py-6 backdrop-blur xl:flex xl:flex-col'>
        <div className='flex items-center gap-3 px-2'>
          <div className='icon-chip h-12 w-12 rounded-2xl'>
            <Workflow className='h-5 w-5' />
          </div>
          <div>
            <p className='text-xs uppercase tracking-[0.28em] text-slate-500'>
              new-api
            </p>
            <p className='text-lg font-semibold text-slate-950'>
              {systemName}
            </p>
          </div>
        </div>

        <div className='mt-8 rounded-[28px] border border-slate-200 bg-slate-50/80 p-4'>
          <p className='text-xs uppercase tracking-[0.22em] text-slate-500'>
            Status
          </p>
          <p className='mt-3 text-sm text-slate-700'>
            Greenfield shell for the existing gateway backend.
          </p>
          <div className='mt-4 inline-flex rounded-full bg-emerald-50 px-3 py-1 text-xs font-medium text-emerald-700'>
            {status.loading ? 'Loading status' : 'Phase 1 foundation'}
          </div>
        </div>

        <nav className='mt-8 flex-1 space-y-1'>
          <p className='px-3 text-xs uppercase tracking-[0.22em] text-slate-400'>
            Navigation
          </p>
          <div className='mt-3 space-y-1'>
            {navigation.map((item) => {
              const isActive = location.pathname === item.href;
              return (
                <Link
                  key={item.name}
                  to={item.href}
                  className={cn(
                    isActive
                      ? 'bg-slate-950 text-white shadow-[0_12px_30px_-18px_rgba(15,23,42,0.8)]'
                      : 'text-slate-600 hover:bg-slate-100 hover:text-slate-950',
                    'group flex items-center gap-3 rounded-2xl px-4 py-3 text-sm font-medium transition-all'
                  )}
                >
                  <item.icon
                    className={cn(
                      isActive
                        ? 'text-white'
                        : 'text-slate-400 group-hover:text-slate-700',
                      'h-5 w-5 shrink-0'
                    )}
                    aria-hidden='true'
                  />
                  {item.name}
                </Link>
              );
            })}
          </div>
        </nav>

        <div className='mt-8 rounded-[28px] border border-slate-200 bg-white p-4 shadow-sm'>
          <div className='flex items-center gap-3'>
            <div className='grid h-10 w-10 place-items-center rounded-full bg-slate-100'>
              <User className='h-5 w-5 text-slate-500' />
            </div>
            <div>
              <p className='text-sm font-medium text-slate-900'>Admin</p>
              <p className='text-xs text-slate-500'>Operator workspace</p>
            </div>
          </div>
        </div>
      </aside>

      <div className='flex min-w-0 flex-1 flex-col'>
        <header className='sticky top-0 z-20 border-b border-slate-200/70 bg-white/80 px-6 py-4 backdrop-blur lg:px-8'>
          <div className='mx-auto flex max-w-7xl items-center justify-between gap-4'>
            <div className='flex min-w-0 items-center gap-3'>
              <div className='hidden h-11 min-w-[280px] items-center gap-2 rounded-full border border-slate-200 bg-slate-50 px-4 text-sm text-slate-500 md:flex'>
                <Search className='h-4 w-4' />
                Search actions, routes, and resources
              </div>
            </div>
            <div className='flex items-center gap-3'>
              <button className='icon-button'>
                <Bell className='h-4 w-4' />
              </button>
              <button className='secondary-button'>Switch workspace</button>
            </div>
          </div>
        </header>

        <main className='flex-1 px-6 py-8 lg:px-8'>
          <div className='mx-auto max-w-7xl'>
            <Outlet />
          </div>
        </main>
      </div>
    </div>
  );
}
