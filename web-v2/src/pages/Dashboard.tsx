import {
  ArrowUpRight,
  BarChart3,
  KeyRound,
  Layers3,
  Sparkles,
  SunMedium,
} from 'lucide-react';
import { Link } from 'react-router-dom';
import MetricCard from '../components/ui/MetricCard';
import StatePanel from '../components/ui/StatePanel';
import { useAsyncData } from '../hooks/useAsyncData';
import { fetchDashboardOverview } from '../lib/dashboard';
import { useI18n } from '../i18n/I18nProvider';

export default function Dashboard() {
  const overview = useAsyncData(fetchDashboardOverview, []);
  const { t } = useI18n();

  const stats = [
    {
      label: t('dashboardMetricModels'),
      value: overview.data ? String(overview.data.providerCount || 0) : '--',
      hint: t('dashboardMetricModelsHint'),
      icon: Layers3,
    },
    {
      label: t('dashboardMetricRequests'),
      value: overview.data ? overview.data.totalRequests.toLocaleString() : '--',
      hint: t('dashboardMetricRequestsHint'),
      icon: KeyRound,
    },
    {
      label: t('dashboardMetricQuota'),
      value: overview.data ? overview.data.totalQuota.toLocaleString() : '--',
      hint: t('dashboardMetricQuotaHint'),
      icon: BarChart3,
    },
    {
      label: t('dashboardMetricDays'),
      value: overview.data ? String(overview.data.activeDays || 0) : '--',
      hint: t('dashboardMetricDaysHint'),
      icon: SunMedium,
    },
  ];

  const topModels = overview.data?.topModels || [];
  const hasData = Boolean(overview.data?.items.length);

  return (
    <div className='space-y-8'>
      <section className='hero-console'>
        <div className='max-w-3xl space-y-5'>
          <p className='eyebrow'>{t('dashboardEyebrow')}</p>
          <h1 className='page-hero-title'>{t('dashboardTitle')}</h1>
          <p className='page-hero-description'>{t('dashboardDescription')}</p>
          <div className='flex flex-wrap gap-3'>
            <Link to='/console/channels' className='primary-button'>
              {t('dashboardPrimary')}
            </Link>
            <Link to='/console/tokens' className='secondary-button'>
              {t('dashboardSecondary')}
            </Link>
          </div>
        </div>

        <div className='hero-console-panel'>
          <div className='flex items-center justify-between text-sm text-slate-500'>
            <span>{t('dashboardPosture')}</span>
            <span className='inline-flex items-center gap-2 rounded-full bg-emerald-50 px-3 py-1 text-emerald-700'>
              <Sparkles className='h-4 w-4' />
              {t('dashboardHealthy')}
            </span>
          </div>
          <div className='mt-6 space-y-4'>
            {[t('dashboardActionOne'), t('dashboardActionTwo'), t('dashboardActionThree')].map(
              (action) => (
                <div
                  key={action}
                  className='flex items-center justify-between rounded-2xl border border-slate-200 bg-white px-4 py-3 text-sm text-slate-700'
                >
                  <span>{action}</span>
                  <ArrowUpRight className='h-4 w-4 text-slate-400' />
                </div>
              ),
            )}
          </div>
        </div>
      </section>

      <section className='grid gap-4 md:grid-cols-2 xl:grid-cols-4'>
        {stats.map((item) => (
          <MetricCard key={item.label} {...item} />
        ))}
      </section>

      <StatePanel
        loading={overview.loading}
        error={overview.error}
        empty={!overview.loading && !overview.error && !hasData}
        title={t('dashboardLoadingTitle')}
        description={
          hasData ? t('dashboardLoadingDescription') : t('dashboardTrafficEmpty')
        }
      />

      <section className='grid gap-4 xl:grid-cols-[1.45fr_1fr]'>
        <article className='panel-card p-6'>
          <div className='flex items-start justify-between gap-4'>
            <div>
              <p className='eyebrow'>{t('dashboardTrafficEyebrow')}</p>
              <h2 className='text-2xl font-semibold text-slate-950'>
                {t('dashboardTrafficTitle')}
              </h2>
            </div>
            <span className='rounded-full bg-slate-100 px-3 py-1 text-sm text-slate-600'>
              {t('dashboardTrafficWindowValue')}
            </span>
          </div>

          <div className='mt-6 grid gap-4 md:grid-cols-3'>
            <div className='rounded-[24px] border border-slate-200 bg-slate-50/80 p-5'>
              <p className='text-xs uppercase tracking-[0.24em] text-slate-500'>
                {t('dashboardTrafficRequests')}
              </p>
              <p className='mt-3 text-3xl font-semibold text-slate-950'>
                {overview.data ? overview.data.totalRequests.toLocaleString() : '--'}
              </p>
            </div>
            <div className='rounded-[24px] border border-slate-200 bg-slate-50/80 p-5'>
              <p className='text-xs uppercase tracking-[0.24em] text-slate-500'>
                {t('dashboardTrafficQuota')}
              </p>
              <p className='mt-3 text-3xl font-semibold text-slate-950'>
                {overview.data ? overview.data.totalQuota.toLocaleString() : '--'}
              </p>
            </div>
            <div className='rounded-[24px] border border-slate-200 bg-slate-50/80 p-5'>
              <p className='text-xs uppercase tracking-[0.24em] text-slate-500'>
                {t('dashboardTrafficActiveDays')}
              </p>
              <p className='mt-3 text-3xl font-semibold text-slate-950'>
                {overview.data ? overview.data.activeDays : '--'}
              </p>
            </div>
          </div>

          <div className='mt-6 rounded-[28px] border border-dashed border-slate-300 bg-[radial-gradient(circle_at_top,_rgba(20,184,166,0.10),_transparent_52%),linear-gradient(180deg,_#ffffff,_#f8fafc)] p-6'>
            <div className='grid gap-4 md:grid-cols-2'>
              <div>
                <p className='text-xs uppercase tracking-[0.24em] text-slate-500'>
                  {t('dashboardTrafficWindow')}
                </p>
                <p className='mt-3 text-lg font-semibold text-slate-900'>
                  {t('dashboardTrafficWindowValue')}
                </p>
                <p className='mt-2 text-sm leading-7 text-slate-600'>
                  {t('dashboardMetricRequestsHint')}
                </p>
              </div>
              <div>
                <p className='text-xs uppercase tracking-[0.24em] text-slate-500'>
                  {t('dashboardSignalEyebrow')}
                </p>
                <p className='mt-3 text-lg font-semibold text-slate-900'>
                  {t('dashboardSignalOneTitle')}
                </p>
                <p className='mt-2 text-sm leading-7 text-slate-600'>
                  {t('dashboardSignalOneDescription')}
                </p>
              </div>
            </div>
          </div>
        </article>

        <article className='panel-card p-6'>
          <p className='eyebrow'>{t('dashboardTopModelsEyebrow')}</p>
          <h2 className='text-2xl font-semibold text-slate-950'>
            {t('dashboardTopModelsTitle')}
          </h2>
          <div className='mt-6 space-y-4'>
            {topModels.length > 0 ? (
              topModels.map((model) => (
                <div
                  key={model.name}
                  className='rounded-2xl border border-slate-200 bg-slate-50/80 p-4'
                >
                  <div className='flex items-center justify-between gap-4'>
                    <p className='font-medium text-slate-900'>{model.name}</p>
                    <span className='text-sm text-slate-500'>
                      {Math.round(model.share * 100)}% {t('dashboardTopModelsShare')}
                    </span>
                  </div>
                  <div className='mt-3 h-2 overflow-hidden rounded-full bg-slate-200'>
                    <div
                      className='h-full rounded-full bg-gradient-to-r from-teal-500 to-sky-500'
                      style={{ width: `${Math.max(model.share * 100, 8)}%` }}
                    />
                  </div>
                  <p className='mt-3 text-sm text-slate-600'>
                    {model.requests.toLocaleString()} {t('dashboardTopModelsRequests')}
                  </p>
                </div>
              ))
            ) : (
              <div className='rounded-2xl border border-dashed border-slate-300 bg-slate-50/70 p-5 text-sm leading-7 text-slate-500'>
                {t('dashboardTopModelsEmpty')}
              </div>
            )}
          </div>
        </article>
      </section>

      <section className='grid gap-4 lg:grid-cols-3'>
        {[
          {
            title: t('dashboardSignalOneTitle'),
            description: t('dashboardSignalOneDescription'),
          },
          {
            title: t('dashboardSignalTwoTitle'),
            description: t('dashboardSignalTwoDescription'),
          },
          {
            title: t('dashboardSignalThreeTitle'),
            description: t('dashboardSignalThreeDescription'),
          },
        ].map((item) => (
          <article key={item.title} className='panel-card p-6'>
            <p className='text-lg font-semibold text-slate-950'>{item.title}</p>
            <p className='mt-3 text-sm leading-7 text-slate-600'>{item.description}</p>
          </article>
        ))}
      </section>
    </div>
  );
}
