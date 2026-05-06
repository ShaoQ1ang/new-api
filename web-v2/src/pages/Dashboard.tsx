import { useMemo, useState } from 'react';
import { ArrowRight, BarChart3, Clock3, Layers3, Wallet } from 'lucide-react';
import { Link } from 'react-router-dom';
import StatePanel from '../components/ui/StatePanel';
import { useAsyncData } from '../hooks/useAsyncData';
import { fetchDashboardOverview } from '../lib/dashboard';
import { useI18n } from '../i18n/I18nProvider';

type BarPoint = {
  label: string;
  isoDate: string;
  total: number;
  quota: number;
  models: Array<{
    name: string;
    count: number;
    quota: number;
  }>;
};

function buildBarPoints(
  items: Array<{ created_at: number; count?: number; quota?: number; model_name?: string }>,
): BarPoint[] {
  const map = new Map<string, BarPoint>();

  for (const item of items) {
    const date = new Date(item.created_at * 1000);
    const isoDate = date.toISOString().slice(0, 10);
    const label = date.toLocaleDateString('en-US', { month: '2-digit', day: '2-digit' });
    const current = map.get(isoDate) || { label, isoDate, total: 0, quota: 0, models: [] };
    current.total += item.count || 0;
    current.quota += item.quota || 0;
    const modelName = (item as { model_name?: string }).model_name || 'unknown';
    const model = current.models.find((entry) => entry.name === modelName);
    if (model) {
      model.count += item.count || 0;
      model.quota += item.quota || 0;
    } else {
      current.models.push({
        name: modelName,
        count: item.count || 0,
        quota: item.quota || 0,
      });
    }
    map.set(isoDate, current);
  }

  return Array.from(map.values())
    .sort((a, b) => a.isoDate.localeCompare(b.isoDate))
    .slice(-7);
}

function formatCompact(value: number) {
  if (value >= 1000000) return `${(value / 1000000).toFixed(1)}M`;
  if (value >= 1000) return `${(value / 1000).toFixed(1)}K`;
  return String(value);
}

export default function Dashboard() {
  const overview = useAsyncData(fetchDashboardOverview, []);
  const { t } = useI18n();
  const [hoveredDay, setHoveredDay] = useState<string | null>(null);

  const items = overview.data?.items || [];
  const topModels = overview.data?.topModels || [];
  const recentItems = overview.data?.recentItems || [];
  const barPoints = useMemo(() => buildBarPoints(items), [items]);
  const maxRequests = Math.max(...barPoints.map((point) => point.total), 1);

  const totalRequests = overview.data?.totalRequests || 0;
  const totalQuota = overview.data?.totalQuota || 0;
  const providerCount = overview.data?.providerCount || 0;
  const activeDays = overview.data?.activeDays || 0;
  const averageRequests = activeDays > 0 ? Math.round(totalRequests / activeDays) : 0;
  const hasTelemetry = items.length > 0 || recentItems.length > 0 || topModels.length > 0;

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

      {hasTelemetry ? (
        <>
          <section className='grid gap-4 xl:grid-cols-[1fr_1fr]'>
            <article className='rounded-[30px] border border-slate-200 bg-white p-5 shadow-sm'>
              <div className='flex items-center justify-between gap-4'>
                <div className='min-h-[56px]'>
                  <p className='text-[11px] font-semibold uppercase tracking-[0.2em] text-slate-400'>{t('dashboardTrafficEyebrow')}</p>
                  <h2 className='mt-2 text-xl font-semibold text-slate-950'>{t('dashboardTrafficTitle')}</h2>
                </div>
                <p className='min-w-[104px] whitespace-nowrap text-right text-sm text-slate-500'>{t('dashboardTrafficWindowValue')}</p>
              </div>

              <div className='mt-5 rounded-[24px] border border-slate-200 bg-slate-50/70 p-4'>
                <div className='grid h-[280px] grid-cols-1 gap-4'>
                  <div className='relative rounded-[20px] bg-white/70 p-4'>
                    <div className='absolute inset-x-4 top-1/4 border-t border-slate-200/70' />
                    <div className='absolute inset-x-4 top-1/2 border-t border-slate-200/70' />
                    <div className='absolute inset-x-4 top-3/4 border-t border-slate-200/70' />

                    <div className='relative flex h-full items-end gap-2'>
                      {barPoints.length > 0 ? (
                        barPoints.map((point) => {
                          const height = Math.max(10, Math.round((point.total / maxRequests) * 100));
                          const isActive = hoveredDay === point.isoDate;

                          return (
                            <button
                              key={point.isoDate}
                              type='button'
                              onMouseEnter={() => setHoveredDay(point.isoDate)}
                              onMouseLeave={() => setHoveredDay(null)}
                              onFocus={() => setHoveredDay(point.isoDate)}
                              onBlur={() => setHoveredDay(null)}
                              className='group flex flex-1 flex-col items-center gap-3 outline-none'
                            >
                              <div className='relative flex h-[180px] w-full items-end justify-center'>
                                <div
                                  className='w-full max-w-[48px] rounded-t-[10px] bg-slate-950 transition-all duration-150 group-hover:bg-slate-800'
                                  style={{ height: `${height}%`, opacity: isActive ? 1 : 0.92 }}
                                />
                                {isActive ? (
                                  <div className='pointer-events-none absolute bottom-full z-20 mb-3 w-[260px] rounded-2xl border border-slate-200 bg-white p-4 text-left shadow-[0_16px_40px_rgba(15,23,42,0.14)]'>
                                    <div className='flex items-start justify-between gap-4'>
                                      <div>
                                        <p className='text-[11px] font-semibold uppercase tracking-[0.16em] text-slate-400'>{point.label}</p>
                                        <p className='mt-2 text-lg font-semibold text-slate-950'>{formatCompact(point.total)} requests</p>
                                      </div>
                                      <p className='text-sm text-slate-500'>${(point.quota / 1000).toFixed(1)}k</p>
                                    </div>
                                    <div className='mt-3 space-y-2'>
                                      {point.models
                                        .slice()
                                        .sort((a, b) => b.count - a.count)
                                        .map((model) => (
                                          <div key={model.name} className='flex items-center justify-between gap-3 text-sm'>
                                            <p className='truncate text-slate-700'>{model.name}</p>
                                            <p className='shrink-0 text-slate-500'>
                                              {model.count} / ${formatCompact(model.quota)}
                                            </p>
                                          </div>
                                        ))}
                                    </div>
                                  </div>
                                ) : null}
                              </div>
                              <div className='text-center'>
                                <p className='text-[11px] font-semibold uppercase tracking-[0.14em] text-slate-500'>{point.label}</p>
                                <p className='mt-1 text-xs text-slate-400'>{formatCompact(point.total)}</p>
                              </div>
                            </button>
                          );
                        })
                      ) : (
                        <div className='flex h-full w-full items-center justify-center text-sm text-slate-500'>
                          {t('dashboardTrafficEmpty')}
                        </div>
                      )}
                    </div>
                  </div>
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
        </>
      ) : (
        <section className='rounded-[30px] border border-slate-200 bg-white px-6 py-10 shadow-sm'>
          <div className='mx-auto flex max-w-xl flex-col items-center text-center'>
            <p className='text-[11px] font-semibold uppercase tracking-[0.2em] text-slate-400'>{t('dashboardEyebrow')}</p>
            <h2 className='mt-3 text-2xl font-semibold text-slate-950'>{t('dashboardLoadingTitle')}</h2>
            <p className='mt-3 max-w-md text-sm leading-6 text-slate-500'>{t('dashboardLoadingDescription')}</p>
          </div>
        </section>
      )}
    </div>
  );
}
