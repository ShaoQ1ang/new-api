import { useMemo, useState } from 'react';
import { Layers3, RadioTower, Tags, TimerReset } from 'lucide-react';
import MetricCard from '../components/ui/MetricCard';
import StatePanel from '../components/ui/StatePanel';
import { useAsyncData } from '../hooks/useAsyncData';
import { fetchChannels } from '../lib/channels';
import { useI18n } from '../i18n/I18nProvider';

const channelTypeMap: Record<number, string> = {
  1: 'OpenAI',
  3: 'Azure',
  14: 'Anthropic',
  20: 'OpenRouter',
  24: 'Gemini',
  25: 'Moonshot',
  33: 'AWS',
  41: 'VertexAI',
  42: 'Mistral',
  43: 'DeepSeek',
  45: 'VolcEngine',
  48: 'xAI',
  57: 'Codex',
};

type FilterMode = 'all' | 'active' | 'inactive';

export default function Channels() {
  const channels = useAsyncData(fetchChannels, []);
  const { t } = useI18n();
  const [filter, setFilter] = useState<FilterMode>('all');

  const rows = useMemo(
    () =>
      (channels.data || []).map((channel) => {
        const models = (channel.models || '')
          .split(',')
          .map((item) => item.trim())
          .filter(Boolean);

        const numericType =
          typeof channel.type === 'number'
            ? channel.type
            : Number.parseInt(String(channel.type || ''), 10);

        return {
          id: channel.id,
          name: channel.name || `Channel #${channel.id}`,
          typeName:
            channelTypeMap[numericType] ||
            `${t('channelsUnknownType')} ${Number.isFinite(numericType) ? numericType : 'N/A'}`,
          active: channel.status === 1,
          statusText:
            channel.status === 1 ? t('channelsStatusActive') : t('channelsStatusInactive'),
          group: channel.group || 'default',
          latency: channel.response_time || 0,
          models,
        };
      }),
    [channels.data, t],
  );

  const filtered = rows.filter((channel) => {
    if (filter === 'active') return channel.active;
    if (filter === 'inactive') return !channel.active;
    return true;
  });

  const stats = [
    {
      label: t('channelsMetricTotal'),
      value: String(rows.length),
      hint: t('channelsMetricTotalHint'),
      icon: Layers3,
    },
    {
      label: t('channelsMetricActive'),
      value: String(rows.filter((item) => item.active).length),
      hint: t('channelsMetricActiveHint'),
      icon: RadioTower,
    },
    {
      label: t('channelsMetricGroups'),
      value: String(new Set(rows.map((item) => item.group)).size),
      hint: t('channelsMetricGroupsHint'),
      icon: Tags,
    },
    {
      label: t('channelsMetricLatency'),
      value: `${rows.length ? Math.round(rows.reduce((sum, item) => sum + item.latency, 0) / rows.length) : 0} ${t('channelsLatencyUnit')}`,
      hint: t('channelsMetricLatencyHint'),
      icon: TimerReset,
    },
  ];

  return (
    <div className='space-y-5'>
      <section className='rounded-[28px] border border-slate-200 bg-white p-5 shadow-sm'>
        <div className='flex flex-col gap-4 lg:flex-row lg:items-center lg:justify-between'>
          <div>
            <h1 className='text-2xl font-semibold text-slate-950'>{t('channelsEyebrow')}</h1>
            <p className='mt-1 text-sm text-slate-500'>{t('channelsDescription')}</p>
          </div>
          <div className='inline-flex rounded-xl border border-slate-200 bg-slate-50 p-1'>
            {(
              [
                ['all', t('channelsFilterAll')],
                ['active', t('channelsFilterActive')],
                ['inactive', t('channelsFilterInactive')],
              ] as Array<[FilterMode, string]>
            ).map(([value, label]) => (
              <button
                key={value}
                type='button'
                onClick={() => setFilter(value)}
                className={
                  filter === value
                    ? 'rounded-lg bg-white px-4 py-2 text-sm font-medium text-slate-950 shadow-sm'
                    : 'rounded-lg px-4 py-2 text-sm text-slate-500'
                }
              >
                {label}
              </button>
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
        loading={channels.loading}
        error={channels.error}
        empty={!channels.loading && !channels.error && rows.length === 0}
        title={t('channelsStatusTitle')}
        description={t('channelsStatusDescription')}
      />

      <section className='overflow-hidden rounded-[28px] border border-slate-200 bg-white shadow-sm'>
        <div className='overflow-x-auto'>
          <table className='min-w-full'>
            <thead>
              <tr className='border-b border-slate-200 text-left text-xs uppercase tracking-[0.18em] text-slate-500'>
                <th className='px-5 py-4 font-medium'>{t('channelsColumnChannel')}</th>
                <th className='px-5 py-4 font-medium'>{t('channelsColumnType')}</th>
                <th className='px-5 py-4 font-medium'>{t('channelsColumnStatus')}</th>
                <th className='px-5 py-4 font-medium'>{t('channelsColumnGroup')}</th>
                <th className='px-5 py-4 font-medium'>{t('channelsColumnLatency')}</th>
                <th className='px-5 py-4 font-medium'>{t('channelsColumnModels')}</th>
              </tr>
            </thead>
            <tbody>
              {filtered.length > 0 ? (
                filtered.map((channel) => (
                  <tr key={channel.id} className='border-b border-slate-100 text-sm text-slate-700'>
                    <td className='px-5 py-4 font-medium text-slate-950'>{channel.name}</td>
                    <td className='px-5 py-4'>{channel.typeName}</td>
                    <td className='px-5 py-4'>
                      <span
                        className={
                          channel.active
                            ? 'inline-flex rounded-full bg-emerald-50 px-3 py-1 text-xs font-medium text-emerald-700'
                            : 'inline-flex rounded-full bg-slate-100 px-3 py-1 text-xs font-medium text-slate-600'
                        }
                      >
                        {channel.statusText}
                      </span>
                    </td>
                    <td className='px-5 py-4'>{channel.group}</td>
                    <td className='px-5 py-4'>
                      {channel.latency} {t('channelsLatencyUnit')}
                    </td>
                    <td className='px-5 py-4'>
                      <div className='flex flex-wrap gap-2'>
                        {channel.models.slice(0, 2).map((model) => (
                          <span
                            key={model}
                            className='inline-flex rounded-full bg-slate-50 px-3 py-1 text-xs font-medium text-slate-600'
                          >
                            {model}
                          </span>
                        ))}
                      </div>
                    </td>
                  </tr>
                ))
              ) : (
                <tr>
                  <td colSpan={6} className='px-5 py-10 text-center text-sm text-slate-500'>
                    {t('channelsEmpty')}
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
