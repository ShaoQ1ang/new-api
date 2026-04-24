import { Link, Outlet, useLocation } from 'react-router-dom';
import { BarChart3, Key, Layers, LayoutDashboard, Menu, Workflow } from 'lucide-react';
import { clsx } from 'clsx';
import { twMerge } from 'tailwind-merge';
import { useStatus } from '../hooks/useStatus';
import { useI18n } from '../i18n/I18nProvider';

function cn(...inputs: Array<string | boolean | undefined>) {
  return twMerge(clsx(inputs));
}

export default function AppLayout() {
  const location = useLocation();
  const status = useStatus();
  const { locale, setLocale, t } = useI18n();
  const systemName = status.data?.system_name || 'new-api';

  const navigation = [
    { name: 'Overview', href: '/console', icon: LayoutDashboard },
    { name: t('usageNav'), href: '/console/usage', icon: BarChart3 },
    { name: 'Channels', href: '/console/channels', icon: Layers },
    { name: 'Tokens', href: '/console/tokens', icon: Key },
  ];

  return (
    <div className='min-h-screen bg-[#f7f8fa] text-slate-900'>
      <div className='mx-auto flex min-h-screen max-w-[1440px]'>
        <aside className='hidden w-[220px] shrink-0 border-r border-slate-200 bg-white xl:flex xl:flex-col'>
          <div className='flex items-center gap-3 px-6 py-6'>
            <div className='grid h-10 w-10 place-items-center rounded-2xl bg-slate-950 text-white'>
              <Workflow className='h-4 w-4' />
            </div>
            <div className='min-w-0'>
              <p className='truncate text-sm font-semibold text-slate-900'>{systemName}</p>
              <p className='text-xs text-slate-500'>User console</p>
            </div>
          </div>

          <nav className='px-4 py-2'>
            <div className='space-y-1'>
              {navigation.map((item) => {
                const isActive = location.pathname === item.href;
                return (
                  <Link
                    key={item.name}
                    to={item.href}
                    className={cn(
                      isActive
                        ? 'bg-slate-950 text-white'
                        : 'text-slate-600 hover:bg-slate-100 hover:text-slate-950',
                      'flex items-center gap-3 rounded-xl px-4 py-3 text-sm font-medium transition-colors',
                    )}
                  >
                    <item.icon className='h-4 w-4 shrink-0' />
                    {item.name}
                  </Link>
                );
              })}
            </div>
          </nav>
        </aside>

        <div className='flex min-w-0 flex-1 flex-col'>
          <header className='border-b border-slate-200 bg-white'>
            <div className='mx-auto flex max-w-6xl items-center justify-between px-5 py-4 lg:px-8'>
              <div className='flex items-center gap-3'>
                <button className='inline-flex h-10 w-10 items-center justify-center rounded-xl border border-slate-200 bg-white text-slate-600 xl:hidden'>
                  <Menu className='h-4 w-4' />
                </button>
                <div>
                  <p className='text-sm font-semibold text-slate-900'>{systemName}</p>
                  <p className='text-xs text-slate-500'>Simple customer workspace</p>
                </div>
              </div>

              <button
                type='button'
                onClick={() => setLocale(locale === 'en' ? 'zh' : 'en')}
                className='rounded-full border border-slate-200 bg-white px-4 py-2 text-sm font-medium text-slate-700 transition-colors hover:bg-slate-50'
              >
                {t('localeLabel')}
              </button>
            </div>
          </header>

          <main className='flex-1 px-4 py-6 lg:px-8 lg:py-8'>
            <div className='mx-auto max-w-6xl'>
              <Outlet />
            </div>
          </main>
        </div>
      </div>
    </div>
  );
}
