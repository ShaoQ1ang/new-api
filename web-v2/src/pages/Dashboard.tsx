import {
  ArrowUpRight,
  BarChart3,
  KeyRound,
  Layers3,
  Sparkles,
  Users,
} from 'lucide-react';
import MetricCard from '../components/ui/MetricCard';
import StatePanel from '../components/ui/StatePanel';
import { useAsyncData } from '../hooks/useAsyncData';
import { fetchDashboardOverview } from '../lib/dashboard';

const quickActions = [
  'Review upstream channel health',
  'Create a scoped token for a new client',
  'Inspect quota and billing anomalies',
];

export default function Dashboard() {
  const overview = useAsyncData(fetchDashboardOverview, []);
  const stats = [
    {
      label: 'Detected models',
      value: overview.data ? String(overview.data.providerCount || 0) : '--',
      hint: 'Distinct models observed in the recent personal usage sample.',
      icon: Layers3,
    },
    {
      label: 'Request volume',
      value: overview.data
        ? overview.data.totalRequests.toLocaleString()
        : '--',
      hint: 'Requests counted from `/api/data/self` over the last 7 days.',
      icon: KeyRound,
    },
    {
      label: 'Consumed quota',
      value: overview.data ? overview.data.totalQuota.toLocaleString() : '--',
      hint: 'Aggregated quota consumption from the existing backend stats API.',
      icon: BarChart3,
    },
    {
      label: 'Phase status',
      value: 'P1',
      hint: 'Greenfield shell with first-round backend wiring in progress.',
      icon: Users,
    },
  ];

  return (
    <div className='space-y-8'>
      <section className='hero-console'>
        <div className='max-w-3xl space-y-5'>
          <p className='eyebrow'>Control Center</p>
          <h1 className='page-hero-title'>
            Operate your AI gateway with clearer routing, visibility, and
            client access controls.
          </h1>
          <p className='page-hero-description'>
            This phase-1 dashboard is the first product shell for `web-v2`.
            It is intentionally cleaner, more directional, and closer to a
            modern AI operations cockpit than the legacy admin surface.
          </p>
          <div className='flex flex-wrap gap-3'>
            <button className='primary-button'>Open channels</button>
            <button className='secondary-button'>Inspect tokens</button>
          </div>
        </div>
        <div className='hero-console-panel'>
          <div className='flex items-center justify-between text-sm text-slate-500'>
            <span>System posture</span>
            <span className='inline-flex items-center gap-2 rounded-full bg-emerald-50 px-3 py-1 text-emerald-700'>
              <Sparkles className='h-4 w-4' />
              Healthy
            </span>
          </div>
          <div className='mt-6 space-y-4'>
            {quickActions.map((action) => (
              <div
                key={action}
                className='flex items-center justify-between rounded-2xl border border-slate-200 bg-white px-4 py-3 text-sm text-slate-700'
              >
                <span>{action}</span>
                <ArrowUpRight className='h-4 w-4 text-slate-400' />
              </div>
            ))}
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
        title='Fetching dashboard telemetry'
        description='The new dashboard is now wired to the current backend overview endpoints and will expand from here.'
      />

      <section className='grid gap-4 xl:grid-cols-[1.5fr_1fr]'>
        <article className='panel-card p-6'>
          <div className='flex items-center justify-between'>
            <div>
              <p className='eyebrow'>Traffic</p>
              <h2 className='text-2xl font-semibold text-slate-950'>
                Usage overview
              </h2>
            </div>
            <span className='rounded-full bg-slate-100 px-3 py-1 text-sm text-slate-600'>
              Placeholder
            </span>
          </div>
          <div className='mt-6 grid h-72 place-items-center rounded-[28px] border border-dashed border-slate-300 bg-[radial-gradient(circle_at_top,_rgba(14,165,233,0.10),_transparent_55%),linear-gradient(180deg,_#ffffff,_#f8fafc)] text-center text-slate-500'>
            <div className='max-w-sm space-y-2'>
              <p className='text-lg font-medium text-slate-700'>
                Chart surface reserved for existing dashboard endpoints
              </p>
              <p className='text-sm'>
                Phase 2 will replace this placeholder with richer trend and
                chart components from the current stats pipeline.
              </p>
            </div>
          </div>
        </article>

        <article className='panel-card p-6'>
          <p className='eyebrow'>Routing</p>
          <h2 className='text-2xl font-semibold text-slate-950'>
            Priority providers
          </h2>
          <div className='mt-6 space-y-4'>
            {[
              ['OpenAI', 'Primary production pool', '42% share'],
              ['Anthropic', 'Failover and premium tier', '27% share'],
              ['Google', 'High-throughput lower-cost mix', '19% share'],
            ].map(([name, desc, share]) => (
              <div
                key={name}
                className='rounded-2xl border border-slate-200 bg-slate-50/80 p-4'
              >
                <div className='flex items-center justify-between'>
                  <p className='font-medium text-slate-900'>{name}</p>
                  <span className='text-sm text-slate-500'>{share}</span>
                </div>
                <p className='mt-2 text-sm text-slate-600'>{desc}</p>
              </div>
            ))}
          </div>
        </article>
      </section>
    </div>
  );
}
