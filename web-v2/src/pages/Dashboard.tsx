import { useMemo } from 'react';
import { ArrowRight, BarChart3, Coins, KeyRound, Layers3, TimerReset, Wallet } from 'lucide-react';
import { Link } from 'react-router-dom';
import MetricCard from '../components/ui/MetricCard';
import StatePanel from '../components/ui/StatePanel';
import { useAsyncData } from '../hooks/useAsyncData';
import { fetchDashboardOverview } from '../lib/dashboard';
import { useI18n } from '../i18n/I18nProvider';

type TrendPoint = {
  label: string;
  total: number;
};

function buildTrendPoints(
  items: Array<{ created_at: number; count?: number }>,
): TrendPoint[] {
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
  if (points.length === 0) return '';
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
  const trendPoints = useMemo(() => buildTrendPoints(items), [items]);
  const trendPath = useMemo(() => buildLinePath(trendPoints, 320, 120), [trendPoints]);

  const totalRequests = overview.data?.totalRequests || 0;
  const totalQuota = overview.data?.totalQuota || 0;
  const providerCount = overview.data?.providerCount || 0;
  const activeDays = overview.data?.activeDays || 0;
  const averageRequests = activeDays > 0 ? Math.round(totalRequests / activeDays) : 0;
  const topModelShare = topModels[0] ? Math.round(topModels[0].share * 100) : 0;

  const stats = [
    {
      label: t('dashboardMetricQuota'),
      value: formatCompact(totalQuota),
      hint: 'Used quota',
      icon: Wallet,
    },
    {
      label: t('dashboardMetricModels'),
      value: String(providerCount),
      hint: 'Active models',
      icon: Layers3,
    },
    {
      label: t('dashboardMetricRequests'),
      value: formatCompact(totalRequests),
      hint: 'Total requests',
      icon: BarChart3,
    },
    {
      label: t('dashboardMetricDays'),
      value: String(activeDays),
      hint: 'Active days',
      icon: TimerReset,
    },
    {
      label: 'Avg / day',
      value: formatCompact(averageRequests),
      hint: 'Request pace',
      icon: Coins,
    },
    {
      label: 'Top share',
      value: `${topModelShare}%`,
      hint: topModels[0]?.name || '—',
      icon: KeyRound,
    },
  ];

  const recentItems = [...items]
    .sort((left, right) => right.created_at - left.created_at)
    .slice(0, 5);

  return (
    <div className='space-y-5'>
      <section className='rounded-[28px] border border-slate-200 bg-white p-5 shadow-sm'>
        <h1 className='text-2xl font-semibold text-slate-950'>{t('dashboardEyebrow')}</h1>
        <p className='mt-1 text-sm text-slate-500'>{t('dashboardDescription')}</p>
      </section>

      <section className='grid gap-4 md:grid-cols-2 xl:grid-cols-3'>
        {stats.map((item) => (
          <MetricCard key={item.label} {...item} />
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
        <article className='rounded-[28px] border border-slate-200 bg-white p-5 shadow-sm'>
          <div className='flex items-center justify-between'>
            <h2 className='text-lg font-semibold text-slate-950'>Model mix</h2>
            <span className='text-sm text-slate-400'>{t('dashboardTrafficWindowValue')}</span>
          </div>

          <div className='mt-6 grid gap-6 lg:grid-cols-[220px_1fr] lg:items-center'>
            <div className='flex items-center justify-center'>
              <div className='relative h-44 w-44 rounded-full bg-[conic-gradient(#4f7cff_0_58%,#7ad3ff_58%_82%,#9b8cff_82%_100%)] p-5'>
                <div className='flex h-full w-full items-center justify-center rounded-full bg-white text-center'>
                  <div>
                    <p className='text-xs uppercase tracking-[0.18em] text-slate-400'>Models</p>
                    <p className='mt-2 text-3xl font-semibold text-slate-950'>{providerCount}</p>
                  </div>
                </div>
              </div>
            </div>

            <div className='space-y-3'>
              {topModels.length > 0 ? (
                topModels.map((model) => (
                  <div key={model.name} className='flex items-center justify-between rounded-2xl bg-slate-50 px-4 py-3'>
                    <div>
                      <p className='font-medium text-slate-950'>{model.name}</p>
                      <p className='text-sm text-slate-500'>{formatCompact(model.requests)} requests</p>
                    </div>
                    <p className='text-sm font-medium text-slate-500'>
                      {Math.round(model.share * 100)}%
                    </p>
                  </div>
                ))
              ) : (
                <div className='rounded-2xl bg-slate-50 px-4 py-6 text-sm text-slate-500'>
                  {t('dashboardTopModelsEmpty')}
                </div>
              )}
            </div>
          </div>
        </article>

        <article className='rounded-[28px] border border-slate-200 bg-white p-5 shadow-sm'>
          <div className='flex items-center justify-between'>
            <h2 className='text-lg font-semibold text-slate-950'>Request trend</h2>
            <span className='text-sm text-slate-400'>{trendPoints.length} points</span>
          </div>

          <div className='mt-6'>
            <svg viewBox='0 0 320 140' className='h-[180px] w-full'>
              <path d='M 0 120 L 320 120' stroke='#e2e8f0' strokeWidth='1' fill='none' />
              <path d='M 0 80 L 320 80' stroke='#eef2f7' strokeWidth='1' fill='none' />
              <path d='M 0 40 L 320 40' stroke='#eef2f7' strokeWidth='1' fill='none' />
              {trendPath ? (
                <path
                  d={trendPath}
                  transform='translate(0 10)'
                  fill='none'
                  stroke='#4f7cff'
                  strokeWidth='3'
                  strokeLinecap='round'
                  strokeLinejoin='round'
                />
              ) : null}
              {trendPoints.map((point, index) => {
                const maxValue = Math.max(...trendPoints.map((item) => item.total), 1);
                const stepX = trendPoints.length > 1 ? 320 / (trendPoints.length - 1) : 320;
                const x = index * stepX;
                const y = 130 - (point.total / maxValue) * 120;
                return <circle key={point.label} cx={x} cy={y} r='4' fill='#4f7cff' />;
              })}
            </svg>
            <div className='mt-2 grid grid-cols-4 gap-2 text-xs text-slate-400'>
              {trendPoints.slice(0, 4).map((point) => (
                <p key={point.label}>{point.label}</p>
              ))}
            </div>
          </div>
        </article>
      </section>

      <section className='grid gap-4 xl:grid-cols-[1.2fr_0.8fr]'>
        <article className='rounded-[28px] border border-slate-200 bg-white p-5 shadow-sm'>
          <div className='flex items-center justify-between'>
            <h2 className='text-lg font-semibold text-slate-950'>Recent usage</h2>
            <span className='text-sm text-slate-400'>{t('dashboardTrafficWindowValue')}</span>
          </div>
          <div className='mt-4 space-y-3'>
            {recentItems.length > 0 ? (
              recentItems.map((item, index) => (
                <div key={`${item.model_name}-${index}`} className='flex items-center justify-between rounded-2xl bg-slate-50 px-4 py-4'>
                  <div>
                    <p className='font-medium text-slate-950'>{item.model_name || 'unknown'}</p>
                    <p className='text-sm text-slate-500'>
                      {new Date(item.created_at * 1000).toLocaleString('en-US', {
                        month: '2-digit',
                        day: '2-digit',
                        hour: '2-digit',
                        minute: '2-digit',
                        hour12: false,
                      })}
                    </p>
                  </div>
                  <div className='text-right'>
                    <p className='font-medium text-emerald-600'>{formatCompact(item.quota || 0)}</p>
                    <p className='text-sm text-slate-500'>{formatCompact(item.count || 0)} requests</p>
                  </div>
                </div>
              ))
            ) : (
              <div className='rounded-2xl bg-slate-50 px-4 py-6 text-sm text-slate-500'>
                No recent activity
              </div>
            )}
          </div>
        </article>

        <article className='rounded-[28px] border border-slate-200 bg-white p-5 shadow-sm'>
          <h2 className='text-lg font-semibold text-slate-950'>Quick actions</h2>
          <div className='mt-4 space-y-3'>
            {[
              { label: 'Open API keys', href: '/console/tokens' },
              { label: 'View usage logs', href: '/console/usage' },
              { label: 'Check channels', href: '/console/channels' },
            ].map((item) => (
              <Link
                key={item.href}
                to={item.href}
                className='flex items-center justify-between rounded-2xl bg-slate-50 px-4 py-4 text-sm font-medium text-slate-700 transition-colors hover:bg-slate-100'
              >
                {item.label}
                <ArrowRight className='h-4 w-4 text-slate-400' />
              </Link>
            ))}
          </div>
        </article>
      </section>
    </div>
  );
}
