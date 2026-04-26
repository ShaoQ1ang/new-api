import { useMemo } from 'react';
import { ArrowRight, BarChart3, Clock3, Layers3, Wallet } from 'lucide-react';
import { Link } from 'react-router-dom';
import StatePanel from '../components/ui/StatePanel';
import { useAsyncData } from '../hooks/useAsyncData';
import { fetchDashboardOverview } from '../lib/dashboard';
import { useI18n } from '../i18n/I18nProvider';

type TrendPoint = {
  label: string;
  total: number;
};

function buildTrendPoints(items: Array<{ created_at: number; count?: number }>): TrendPoint[] {
  const map = new Map<string, number>();

  for (const item of items) {
    const label = new Date(item.created_at * 1000).toISOString().slice(5, 10);
    map.set(label, (map.get(label) || 0) + (item.count || 0));
  }

  return Array.from(map.entries())
    .map(([label, total]) => ({ label, total }))
    .sort((a, b) => a.label.localeCompare(b.label))
    .slice(-7);
}

function buildLinePath(points: TrendPoint[], width: number, height: number) {
  if (!points.length) return '';
  const maxValue = Math.max(...points.map((point) => point.total), 1);
  const stepX = points.length > 1 ? width / (points.length - 1) : width;

  return points
    .map((point, index) => {
      const x = index * stepX;
      const y = height - (point.total / maxValue) * height;
      return `${index === 0 ? 'M' : 'L'} ${x} ${y}`;
    })
    .join(' ');
}

function formatCompact(value: number) {
  if (value >= 1000000) return `${(value / 1000000).toFixed(1)}M`;
  if (value >= 1000) return `${(value / 1000).toFixed(1)}K`;
  return String(value);
}

export default function Dashboard() {
  const overview = useAsyncData(fetchDashboardOverview, []);
  const { t } = useI18n();

  const items = overview.data?.items || [];
  const topModels = overview.data?.topModels || [];
  const recentItems = overview.data?.recentItems || [];
  const trendPoints = useMemo(() => buildTrendPoints(items), [items]);
  const trendPath = useMemo(() => buildLinePath(trendPoints, 360, 112), [trendPoints]);

  const totalRequests = overview.data?.totalRequests || 0;
  const totalQuota = overview.data?.totalQuota || 0;
  const providerCount = overview.data?.providerCount || 0;
  const activeDays = overview.data?.activeDays || 0;
  const averageRequests = activeDays > 0 ? Math.round(totalRequests / activeDays) : 0;

  const stats = [
    {
      label: t('dashboardMetricRequests'),
      value: formatCompact(totalRequests),
      hint: t('dashboardMetricRequestsHint'),
      icon: BarChart3,
    },
    {
      label: t('dashboardMetricQuota'),
      value: formatCompact(totalQuota),
      hint: t('dashboardMetricQuotaHint'),
      icon: Wallet,
    },
    {
      label: t('dashboardMetricModels'),
      value: String(providerCount),
      hint: t('dashboardMetricModelsHint'),
      icon: Layers3,
    },
    {
      label: t('dashboardMetricDays'),
      value: String(activeDays),
      hint: t('dashboardMetricDaysHint'),
      icon: Clock3,
    },
  ];

  return (
    <div className='space-y-5'>
      <section className='grid gap-4 xl:grid-cols-[1.15fr_0.85fr]'>
        <article className='rounded-[30px] border border-slate-200 bg-white p-5 shadow-sm'>
          <p className='text-[11px] font-semibold uppercase tracking-[0.2em] text-slate-400'>{t('dashboardEyebrow')}</p>
          <div className='mt-2 grid gap-3'>
            <h1 className='min-h-[58px] max-w-[720px] text-[26px] font-semibold tracking-[-0.04em] text-slate-950'>
              {t('dashboardTitle')}
            </h1>
            <p className='min-h-[44px] max-w-[680px] text-sm leading-6 text-slate-600'>{t('dashboardDescription')}</p>
          </div>

          <div className='mt-5 grid gap-3 md:grid-cols-3'>
            <div className='grid min-h-[92px] grid-rows-[auto_1fr] rounded-[20px] border border-slate-200 bg-slate-50/80 px-4 py-4'>
              <p className='text-xs font-semibold uppercase tracking-[0.16em] text-slate-400'>{t('dashboardTrafficWindow')}</p>
              <p className='mt-2 self-end text-lg font-semibold text-slate-950'>{t('dashboardTrafficWindowValue')}</p>
            </div>
            <div className='grid min-h-[92px] grid-rows-[auto_1fr] rounded-[20px] border border-slate-200 bg-slate-50/80 px-4 py-4'>
              <p className='text-xs font-semibold uppercase tracking-[0.16em] text-slate-400'>{t('dashboardAvgLabel')}</p>
              <p className='mt-2 self-end text-lg font-semibold text-slate-950'>{formatCompact(averageRequests)}</p>
            </div>
            <div className='grid min-h-[92px] grid-rows-[auto_1fr] rounded-[20px] border border-slate-200 bg-slate-50/80 px-4 py-4'>
              <p className='text-xs font-semibold uppercase tracking-[0.16em] text-slate-400'>{t('dashboardTopModelLabel')}</p>
              <p className='mt-2 self-end truncate text-lg font-semibold text-slate-950'>{topModels[0]?.name || '--'}</p>
            </div>
          </div>
        </article>

        <article className='rounded-[30px] border border-slate-200 bg-white p-5 shadow-sm'>
          <p className='text-[11px] font-semibold uppercase tracking-[0.2em] text-slate-400'>{t('dashboardQuickEyebrow')}</p>
          <h2 className='mt-2 min-h-[28px] text-xl font-semibold text-slate-950'>{t('dashboardQuickTitle')}</h2>

          <div className='mt-5 grid gap-3 sm:grid-cols-2'>
            {[
              { label: t('dashboardQuickPlayground'), href: '/console/playground' },
              { label: t('dashboardQuickUsage'), href: '/console/usage' },
              { label: t('dashboardQuickTasks'), href: '/console/tasklog' },
              { label: t('dashboardQuickTokens'), href: '/console/tokens' },
            ].map((item) => (
              <Link
                key={item.href}
                to={item.href}
                className='flex min-h-[64px] items-center justify-between gap-3 rounded-2xl border border-slate-200 bg-slate-50 px-4 py-3 text-sm font-medium text-slate-700 transition-colors hover:bg-slate-100'
              >
                <span className='leading-5'>{item.label}</span>
                <ArrowRight className='h-4 w-4 text-slate-400' />
              </Link>
            ))}
          </div>
        </article>
      </section>

      <section className='grid gap-4 md:grid-cols-2 xl:grid-cols-4'>
        {stats.map((item) => (
          <article key={item.label} className='grid min-h-[144px] grid-rows-[auto_auto_1fr] rounded-[28px] border border-slate-200 bg-white px-5 py-5 shadow-sm'>
            <div className='flex items-center justify-between gap-4'>
              <p className='text-xs font-semibold uppercase tracking-[0.16em] text-slate-400'>{item.label}</p>
              <item.icon className='h-4 w-4 text-slate-400' />
            </div>
            <p className='mt-3 text-[30px] font-semibold tracking-[-0.03em] text-slate-950'>{item.value}</p>
            <p className='mt-2 text-sm leading-5 text-slate-500'>{item.hint}</p>
          </article>
        ))}
      </section>

      <StatePanel
        loading={overview.loading}
        error={overview.error}
        empty={!overview.loading && !overview.error && items.length === 0}
        title={t('dashboardLoadingTitle')}
        description={t('dashboardLoadingDescription')}
      />

      <section className='grid gap-4 xl:grid-cols-[1fr_1fr]'>
        <article className='rounded-[30px] border border-slate-200 bg-white p-5 shadow-sm'>
          <div className='flex items-center justify-between gap-4'>
            <div className='min-h-[56px]'>
              <p className='text-[11px] font-semibold uppercase tracking-[0.2em] text-slate-400'>{t('dashboardTrafficEyebrow')}</p>
              <h2 className='mt-2 text-xl font-semibold text-slate-950'>{t('dashboardTrafficTitle')}</h2>
            </div>
            <p className='min-w-[104px] whitespace-nowrap text-right text-sm text-slate-500'>{t('dashboardTrafficWindowValue')}</p>
          </div>

          <div className='mt-5 overflow-hidden rounded-[22px] border border-slate-200 bg-slate-50/70 p-4'>
            <svg viewBox='0 0 360 132' className='h-[176px] w-full'>
              <path d='M 0 112 L 360 112' stroke='#e2e8f0' strokeWidth='1' fill='none' />
              <path d='M 0 76 L 360 76' stroke='#eef2f7' strokeWidth='1' fill='none' />
              <path d='M 0 40 L 360 40' stroke='#eef2f7' strokeWidth='1' fill='none' />
              {trendPath ? (
                <path
                  d={trendPath}
                  transform='translate(0 10)'
                  fill='none'
                  stroke='#0f172a'
                  strokeWidth='3'
                  strokeLinecap='round'
                  strokeLinejoin='round'
                />
              ) : null}
              {trendPoints.map((point, index) => {
                const maxValue = Math.max(...trendPoints.map((item) => item.total), 1);
                const stepX = trendPoints.length > 1 ? 360 / (trendPoints.length - 1) : 360;
                const x = index * stepX;
                const y = 122 - (point.total / maxValue) * 112;
                return <circle key={point.label} cx={x} cy={y} r='4' fill='#0f172a' />;
              })}
            </svg>

            <div className='mt-2 grid grid-cols-4 gap-2 text-xs text-slate-400'>
              {trendPoints.slice(0, 4).map((point) => (
                <p key={point.label}>{point.label}</p>
              ))}
            </div>
          </div>
        </article>

        <article className='rounded-[30px] border border-slate-200 bg-white p-5 shadow-sm'>
          <div className='flex items-center justify-between gap-4'>
            <div className='min-h-[56px]'>
              <p className='text-[11px] font-semibold uppercase tracking-[0.2em] text-slate-400'>{t('dashboardTopModelsEyebrow')}</p>
              <h2 className='mt-2 text-xl font-semibold text-slate-950'>{t('dashboardTopModelsTitle')}</h2>
            </div>
            <p className='min-w-[48px] text-right text-sm text-slate-500'>{providerCount}</p>
          </div>

          <div className='mt-5 space-y-3'>
            {topModels.length > 0 ? (
              topModels.map((model) => (
                <div key={model.name} className='flex min-h-[72px] items-center justify-between gap-3 rounded-2xl border border-slate-200 bg-slate-50 px-4 py-3'>
                  <div className='min-w-0'>
                    <p className='truncate font-medium text-slate-950'>{model.name}</p>
                    <p className='mt-1 text-sm text-slate-500'>
                      {formatCompact(model.requests)} {t('dashboardTopModelsRequests')}
                    </p>
                  </div>
                  <p className='shrink-0 text-sm font-medium text-slate-500'>{Math.round(model.share * 100)}%</p>
                </div>
              ))
            ) : (
              <div className='rounded-2xl border border-slate-200 bg-slate-50 px-4 py-6 text-sm text-slate-500'>
                {t('dashboardTopModelsEmpty')}
              </div>
            )}
          </div>
        </article>
      </section>

      <section className='rounded-[30px] border border-slate-200 bg-white shadow-sm'>
        <div className='flex items-center justify-between border-b border-slate-200 px-5 py-4'>
          <div className='min-h-[56px]'>
            <p className='text-[11px] font-semibold uppercase tracking-[0.2em] text-slate-400'>{t('dashboardRecentEyebrow')}</p>
            <h2 className='mt-2 text-xl font-semibold text-slate-950'>{t('dashboardRecentTitle')}</h2>
          </div>
          <p className='min-w-[104px] whitespace-nowrap text-right text-sm text-slate-500'>{t('dashboardTrafficWindowValue')}</p>
        </div>

        <div className='overflow-x-auto'>
          <table className='min-w-full table-fixed divide-y divide-slate-200'>
            <thead className='bg-slate-50'>
              <tr className='text-left text-xs font-semibold uppercase tracking-[0.16em] text-slate-500'>
                <th className='w-[40%] rounded-tl-[22px] px-5 py-4'>{t('dashboardRecentModel')}</th>
                <th className='w-[26%] px-5 py-4'>{t('dashboardRecentTime')}</th>
                <th className='w-[17%] px-5 py-4'>{t('dashboardRecentRequests')}</th>
                <th className='w-[17%] rounded-tr-[22px] px-5 py-4'>{t('dashboardRecentQuota')}</th>
              </tr>
            </thead>
            <tbody className='divide-y divide-slate-200 bg-white'>
              {recentItems.length > 0 ? (
                recentItems.map((item, index) => (
                  <tr key={`${item.model_name}-${index}`} className='text-sm text-slate-600'>
                    <td className='truncate px-5 py-4 font-medium text-slate-900'>{item.model_name || '--'}</td>
                    <td className='px-5 py-4 whitespace-nowrap'>
                      {new Date(item.created_at * 1000).toLocaleString('en-US', {
                        month: '2-digit',
                        day: '2-digit',
                        hour: '2-digit',
                        minute: '2-digit',
                        hour12: false,
                      })}
                    </td>
                    <td className='px-5 py-4'>{formatCompact(item.count || 0)}</td>
                    <td className='px-5 py-4 font-medium text-slate-900'>{formatCompact(item.quota || 0)}</td>
                  </tr>
                ))
              ) : (
                <tr>
                  <td colSpan={4} className='px-5 py-8 text-center text-sm text-slate-500'>
                    {t('dashboardTrafficEmpty')}
                  </td>
                </tr>
              )}
            </tbody>
          </table>
        </div>
      </section>
    </div>
  );
}
