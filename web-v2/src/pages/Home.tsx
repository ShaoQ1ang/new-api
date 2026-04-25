import { ArrowRight, Languages, Workflow } from 'lucide-react';
import { useStatus } from '../hooks/useStatus';
import { useI18n } from '../i18n/I18nProvider';

export default function Home() {
  const status = useStatus();
  const { locale, setLocale, t } = useI18n();

  const docsLink = status.data?.docs_link;
  const version = status.data?.version;
  const systemName = status.data?.system_name || 'new-api';
  const baseUrl = status.data?.server_address || 'https://your-gateway.example.com';

  return (
    <div className='min-h-screen bg-[#f7f8fa] text-slate-950'>
      <header className='border-b border-slate-200 bg-white'>
        <div className='mx-auto flex max-w-6xl items-center justify-between px-5 py-5 lg:px-8'>
          <div className='flex items-center gap-3'>
            <div className='grid h-10 w-10 place-items-center rounded-2xl bg-slate-950 text-white'>
              <Workflow className='h-4 w-4' />
            </div>
            <p className='text-sm font-semibold text-slate-950'>{systemName}</p>
          </div>

          <div className='flex items-center gap-3'>
            <button
              type='button'
              onClick={() => setLocale(locale === 'en' ? 'zh' : 'en')}
              className='rounded-full border border-slate-200 bg-white px-4 py-2 text-sm font-medium text-slate-700'
            >
              <span className='inline-flex items-center gap-2'>
                <Languages className='h-4 w-4' />
                {t('localeLabel')}
              </span>
            </button>
            <a href='/login' className='text-sm font-medium text-slate-600 hover:text-slate-950'>
              {t('navSignIn')}
            </a>
          </div>
        </div>
      </header>

      <main className='mx-auto max-w-6xl px-5 py-8 lg:px-8 lg:py-12'>
        <section className='rounded-[32px] border border-slate-200 bg-white p-8 shadow-sm lg:p-10'>
          <div className='max-w-3xl'>
            <p className='text-sm font-medium text-slate-500'>{t('heroBadge')}</p>
            <h1 className='mt-4 text-4xl font-semibold tracking-tight text-slate-950 lg:text-5xl'>
              {t('heroTitle')}
            </h1>
            <p className='mt-4 max-w-2xl text-base leading-8 text-slate-600'>
              {t('heroDescription')}
            </p>
          </div>

          <div className='mt-8 flex flex-wrap gap-3'>
            <a href='/console' className='primary-button'>
              {t('heroPrimary')}
              <ArrowRight className='h-4 w-4' />
            </a>
            {docsLink ? (
              <a
                href={docsLink}
                target='_blank'
                rel='noreferrer'
                className='secondary-button'
              >
                {t('heroSecondary')}
              </a>
            ) : null}
          </div>

          <div className='mt-8 grid gap-4 md:grid-cols-3'>
            <div className='rounded-2xl bg-slate-50 p-5'>
              <p className='text-xs uppercase tracking-[0.18em] text-slate-500'>{t('baseUrlLabel')}</p>
              <p className='mt-3 break-all font-mono text-sm text-slate-900'>{baseUrl}</p>
            </div>
            <div className='rounded-2xl bg-slate-50 p-5'>
              <p className='text-xs uppercase tracking-[0.18em] text-slate-500'>{t('metricModels')}</p>
              <p className='mt-3 text-2xl font-semibold text-slate-950'>40+</p>
            </div>
            <div className='rounded-2xl bg-slate-50 p-5'>
              <p className='text-xs uppercase tracking-[0.18em] text-slate-500'>{t('heroVersion')}</p>
              <p className='mt-3 text-2xl font-semibold text-slate-950'>{version || '--'}</p>
            </div>
          </div>
        </section>
      </main>
    </div>
  );
}
