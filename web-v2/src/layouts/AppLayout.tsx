import { Link, Outlet, useLocation } from 'react-router-dom';
import { BarChart3, Boxes, Clapperboard, CreditCard, Key, LayoutDashboard, Menu, Sparkles, Workflow } from 'lucide-react';
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

  const pageHeader = (() => {
    if (location.pathname === '/console/usage') {
      return {
        title: t('usageEyebrow'),
        description: t('usageDescription'),
      };
    }

    if (location.pathname === '/console/tokens') {
      return {
        title: t('tokensEyebrow'),
        description: t('tokensDescription'),
      };
    }

    if (location.pathname === '/console/models') {
      return {
        title: t('modelsNav'),
        description: t('modelsDescription'),
      };
    }

    if (location.pathname === '/console/tasklog') {
      return {
        title: t('taskNav'),
        description: t('taskDescription'),
      };
    }

    if (location.pathname === '/console/billing') {
      return {
        title: t('billingNav'),
        description: t('billingDescription'),
      };
    }

    if (location.pathname === '/console/playground') {
      return {
        title: t('playgroundNav'),
        description: t('playgroundDescription'),
      };
    }

    return {
      title: systemName,
      description: '',
    };
  })();

  const navigation = [
    { name: 'Overview', href: '/console', icon: LayoutDashboard },
    { name: t('modelsNav'), href: '/console/models', icon: Boxes },
    { name: t('playgroundNav'), href: '/console/playground', icon: Sparkles },
    { name: t('usageNav'), href: '/console/usage', icon: BarChart3 },
    { name: t('taskNav'), href: '/console/tasklog', icon: Clapperboard },
    { name: t('billingNav'), href: '/console/billing', icon: CreditCard },
    { name: 'API Keys', href: '/console/tokens', icon: Key },
  ];

  return (
    <div className='min-h-screen bg-[#f7f8fa] text-slate-900'>
      <div className='mx-auto flex min-h-screen max-w-[1440px]'>
        <aside className='hidden w-[240px] shrink-0 border-r border-slate-200 bg-white xl:flex xl:flex-col'>
          <div className='flex items-center gap-3 px-6 py-6'>
            <div className='grid h-10 w-10 place-items-center rounded-2xl bg-slate-950 text-white'>
              <Workflow className='h-4 w-4' />
            </div>
            <div className='min-w-0'>
              <p className='truncate text-sm font-semibold text-slate-900'>{systemName}</p>
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
                      'flex h-[48px] items-center gap-3 rounded-xl px-4 py-3 text-sm font-medium transition-colors',
                    )}
                  >
                    <item.icon className='h-4 w-4 shrink-0' />
                    <span className='block w-[136px] truncate'>{item.name}</span>
                  </Link>
                );
              })}
            </div>
          </nav>
        </aside>

        <div className='flex min-w-0 flex-1 flex-col'>
          <header className='border-b border-slate-200 bg-white'>
            <div className='mx-auto flex min-h-[68px] max-w-6xl items-start justify-between px-5 py-3 lg:px-8'>
              <div className='flex items-start gap-3'>
                <button className='inline-flex h-10 w-10 items-center justify-center rounded-xl border border-slate-200 bg-white text-slate-600 xl:hidden'>
                  <Menu className='h-4 w-4' />
                </button>
                <div className='min-w-0 max-w-[720px]'>
                  <p className='min-h-[22px] text-[18px] font-semibold leading-[1.1] text-slate-950'>{pageHeader.title}</p>
                  {pageHeader.description ? (
                    <p className='mt-0.5 min-h-[20px] max-w-[680px] text-sm leading-5 text-slate-600'>{pageHeader.description}</p>
                  ) : null}
                </div>
              </div>

              <button
                type='button'
                onClick={() => setLocale(locale === 'en' ? 'zh' : 'en')}
                className='h-9 w-[92px] rounded-full border border-slate-200 bg-white px-4 py-1.5 text-sm font-medium text-slate-700 transition-colors hover:bg-slate-50'
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
