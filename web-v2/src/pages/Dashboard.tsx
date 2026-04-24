import { useAsyncData } from '../hooks/useAsyncData';
import { fetchDashboardOverview } from '../lib/dashboard';
import { useI18n } from '../i18n/I18nProvider';
import MetricCard from '../components/ui/MetricCard';
import StatePanel from '../components/ui/StatePanel';
import { BarChart3, KeyRound, Layers3, SunMedium } from 'lucide-react';

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

  return (
    <div className='space-y-5'>
      <section className='rounded-[28px] border border-slate-200 bg-white p-5 shadow-sm'>
        <h1 className='text-2xl font-semibold text-slate-950'>{t('dashboardEyebrow')}</h1>
        <p className='mt-1 text-sm text-slate-500'>{t('dashboardDescription')}</p>
      </section>

      <section className='grid gap-4 md:grid-cols-2 xl:grid-cols-4'>
        {stats.map((item) => (
          <MetricCard key={item.label} {...item} />
        ))}
      </section>

      <StatePanel
        loading={overview.loading}
        error={overview.error}
        empty={!overview.loading && !overview.error && topModels.length === 0}
        title={t('dashboardLoadingTitle')}
        description={t('dashboardLoadingDescription')}
      />

      <section className='grid gap-4 lg:grid-cols-[1.2fr_0.8fr]'>
        <article className='rounded-[28px] border border-slate-200 bg-white p-5 shadow-sm'>
          <div className='flex items-center justify-between'>
            <h2 className='text-lg font-semibold text-slate-950'>{t('dashboardTrafficTitle')}</h2>
            <span className='rounded-xl bg-slate-50 px-3 py-2 text-xs font-medium text-slate-500'>
              {t('dashboardTrafficWindowValue')}
            </span>
          </div>
          <div className='mt-4 grid gap-3 sm:grid-cols-3'>
            <div className='rounded-2xl bg-slate-50 px-4 py-4'>
              <p className='text-xs uppercase tracking-[0.18em] text-slate-500'>
                {t('dashboardTrafficRequests')}
              </p>
              <p className='mt-2 text-2xl font-semibold text-slate-950'>
                {overview.data ? overview.data.totalRequests.toLocaleString() : '--'}
              </p>
            </div>
            <div className='rounded-2xl bg-slate-50 px-4 py-4'>
              <p className='text-xs uppercase tracking-[0.18em] text-slate-500'>
                {t('dashboardTrafficQuota')}
              </p>
              <p className='mt-2 text-2xl font-semibold text-slate-950'>
                {overview.data ? overview.data.totalQuota.toLocaleString() : '--'}
              </p>
            </div>
            <div className='rounded-2xl bg-slate-50 px-4 py-4'>
              <p className='text-xs uppercase tracking-[0.18em] text-slate-500'>
                {t('dashboardTrafficActiveDays')}
              </p>
              <p className='mt-2 text-2xl font-semibold text-slate-950'>
                {overview.data ? overview.data.activeDays : '--'}
              </p>
            </div>
          </div>
        </article>

        <article className='rounded-[28px] border border-slate-200 bg-white p-5 shadow-sm'>
          <h2 className='text-lg font-semibold text-slate-950'>{t('dashboardTopModelsTitle')}</h2>
          <div className='mt-4 space-y-3'>
            {topModels.length > 0 ? (
              topModels.map((model) => (
                <div key={model.name} className='rounded-2xl bg-slate-50 px-4 py-4'>
                  <div className='flex items-center justify-between gap-4'>
                    <p className='font-medium text-slate-900'>{model.name}</p>
                    <span className='text-sm text-slate-500'>
                      {Math.round(model.share * 100)}%
                    </span>
                  </div>
                  <p className='mt-2 text-sm text-slate-500'>
                    {model.requests.toLocaleString()} {t('dashboardTopModelsRequests')}
                  </p>
                </div>
              ))
            ) : (
              <div className='rounded-2xl bg-slate-50 px-4 py-4 text-sm text-slate-500'>
                {t('dashboardTopModelsEmpty')}
              </div>
            )}
          </div>
        </article>
      </section>
    </div>
  );
}
