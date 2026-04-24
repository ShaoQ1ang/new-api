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

type FilterMode = 'all' | 'active' | 'inactive' | 'tagged';

export default function Channels() {
  const channels = useAsyncData(fetchChannels, []);
  const { t } = useI18n();
  const [filter, setFilter] = useState<FilterMode>('all');

  const normalizedChannels = useMemo(() => {
    return (channels.data || []).map((channel) => {
      const models = (channel.models || '')
        .split(',')
        .map((item) => item.trim())
        .filter(Boolean);

      const numericType =
        typeof channel.type === 'number'
          ? channel.type
          : Number.parseInt(String(channel.type || ''), 10);
      const typeName =
        channelTypeMap[numericType] ||
        `${t('channelsUnknownType')} ${Number.isFinite(numericType) ? numericType : 'N/A'}`;

      return {
        id: channel.id,
        name: channel.name || `Channel #${channel.id}`,
        typeName,
        statusText:
          channel.status === 1 ? t('channelsStatusActive') : t('channelsStatusInactive'),
        active: channel.status === 1,
        group: channel.group || 'default',
        latency: channel.response_time || 0,
        priority:
          typeof channel.priority === 'number' ? channel.priority : channel.priority || 0,
        tag: channel.tag || '',
        models,
      };
    });
  }, [channels.data, t]);

  const filteredChannels = normalizedChannels.filter((channel) => {
    if (filter === 'active') return channel.active;
    if (filter === 'inactive') return !channel.active;
    if (filter === 'tagged') return Boolean(channel.tag);
    return true;
  });

  const activeCount = normalizedChannels.filter((channel) => channel.active).length;
  const groupCount = new Set(normalizedChannels.map((channel) => channel.group)).size;
  const taggedCount = normalizedChannels.filter((channel) => channel.tag).length;
  const averageLatency =
    normalizedChannels.length > 0
      ? Math.round(
          normalizedChannels.reduce((sum, channel) => sum + channel.latency, 0) /
            normalizedChannels.length,
        )
      : 0;

  const stats = [
    {
      label: t('channelsMetricTotal'),
      value: String(normalizedChannels.length),
      hint: t('channelsMetricTotalHint'),
      icon: Layers3,
    },
    {
      label: t('channelsMetricActive'),
      value: String(activeCount),
      hint: t('channelsMetricActiveHint'),
      icon: RadioTower,
    },
    {
      label: t('channelsMetricGroups'),
      value: String(groupCount),
      hint: t('channelsMetricGroupsHint'),
      icon: Tags,
    },
    {
      label: t('channelsMetricLatency'),
      value: `${averageLatency} ${t('channelsLatencyUnit')}`,
      hint: t('channelsMetricLatencyHint'),
      icon: TimerReset,
    },
  ];

  return (
    <div className='space-y-8'>
      <section className='hero-console'>
        <div className='max-w-3xl space-y-5'>
          <p className='eyebrow'>{t('channelsEyebrow')}</p>
          <h1 className='page-hero-title'>{t('channelsTitle')}</h1>
          <p className='page-hero-description'>{t('channelsDescription')}</p>
        </div>

        <div className='hero-console-panel'>
          <p className='text-xs uppercase tracking-[0.24em] text-slate-500'>
            {t('channelsFiltersTitle')}
          </p>
          <div className='mt-5 flex flex-wrap gap-3'>
            {(
              [
                ['all', t('channelsFilterAll')],
                ['active', t('channelsFilterActive')],
                ['inactive', t('channelsFilterInactive')],
                ['tagged', t('channelsFilterTagged')],
              ] as Array<[FilterMode, string]>
            ).map(([value, label]) => (
              <button
                key={value}
                type='button'
                onClick={() => setFilter(value)}
                className={
                  filter === value
                    ? 'inline-flex rounded-full bg-slate-950 px-4 py-2 text-sm font-medium text-white'
                    : 'inline-flex rounded-full border border-slate-200 bg-white px-4 py-2 text-sm font-medium text-slate-600 transition-colors hover:bg-slate-50'
                }
              >
                {label}
              </button>
            ))}
          </div>
          <div className='mt-6 grid gap-3 text-sm text-slate-600'>
            <div className='rounded-2xl border border-slate-200 bg-white px-4 py-3'>
              {activeCount} / {normalizedChannels.length} {t('channelsFilterActive')}
            </div>
            <div className='rounded-2xl border border-slate-200 bg-white px-4 py-3'>
              {taggedCount} / {normalizedChannels.length} {t('channelsFilterTagged')}
            </div>
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
        empty={!channels.loading && !channels.error && normalizedChannels.length === 0}
        title={t('channelsStatusTitle')}
        description={t('channelsStatusDescription')}
      />

      <section className='grid gap-4 lg:grid-cols-3'>
        {[
          {
            title: t('channelsHighlightOne'),
            description: t('channelsHighlightOneDescription'),
          },
          {
            title: t('channelsHighlightTwo'),
            description: t('channelsHighlightTwoDescription'),
          },
          {
            title: t('channelsHighlightThree'),
            description: t('channelsHighlightThreeDescription'),
          },
        ].map((item) => (
          <article key={item.title} className='panel-card p-6'>
            <p className='text-lg font-semibold text-slate-950'>{item.title}</p>
            <p className='mt-3 text-sm leading-7 text-slate-600'>{item.description}</p>
          </article>
        ))}
      </section>

      <section className='panel-card overflow-hidden'>
        <div className='border-b border-slate-200 px-6 py-5'>
          <p className='eyebrow'>{t('channelsEyebrow')}</p>
          <h2 className='mt-3 text-3xl font-semibold tracking-tight text-slate-950'>
            {t('channelsTableTitle')}
          </h2>
          <p className='mt-3 max-w-3xl text-base leading-7 text-slate-600'>
            {t('channelsTableDescription')}
          </p>
        </div>

        {filteredChannels.length > 0 ? (
          <div className='overflow-x-auto'>
            <table className='min-w-full'>
              <thead>
                <tr className='border-b border-slate-200 bg-slate-50/80 text-left text-xs uppercase tracking-[0.2em] text-slate-500'>
                  <th className='px-6 py-4 font-medium'>{t('channelsColumnChannel')}</th>
                  <th className='px-6 py-4 font-medium'>{t('channelsColumnType')}</th>
                  <th className='px-6 py-4 font-medium'>{t('channelsColumnStatus')}</th>
                  <th className='px-6 py-4 font-medium'>{t('channelsColumnGroup')}</th>
                  <th className='px-6 py-4 font-medium'>{t('channelsColumnLatency')}</th>
                  <th className='px-6 py-4 font-medium'>{t('channelsColumnPriority')}</th>
                  <th className='px-6 py-4 font-medium'>{t('channelsColumnModels')}</th>
                </tr>
              </thead>
              <tbody>
                {filteredChannels.map((channel) => (
                  <tr
                    key={channel.id}
                    className='border-b border-slate-100 text-sm text-slate-700 transition-colors hover:bg-slate-50/80'
                  >
                    <td className='px-6 py-5'>
                      <div className='space-y-2'>
                        <p className='font-medium text-slate-950'>{channel.name}</p>
                        {channel.tag ? (
                          <span className='inline-flex rounded-full bg-sky-50 px-3 py-1 text-xs font-medium text-sky-700'>
                            {channel.tag}
                          </span>
                        ) : null}
                      </div>
                    </td>
                    <td className='px-6 py-5'>{channel.typeName}</td>
                    <td className='px-6 py-5'>
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
                    <td className='px-6 py-5'>{channel.group}</td>
                    <td className='px-6 py-5'>
                      {channel.latency} {t('channelsLatencyUnit')}
                    </td>
                    <td className='px-6 py-5'>{channel.priority}</td>
                    <td className='px-6 py-5'>
                      <div className='flex flex-wrap gap-2'>
                        {channel.models.slice(0, 3).map((model) => (
                          <span
                            key={model}
                            className='inline-flex rounded-full border border-slate-200 bg-white px-3 py-1 text-xs font-medium text-slate-600'
                          >
                            {model}
                          </span>
                        ))}
                        {channel.models.length > 3 ? (
                          <span className='inline-flex rounded-full border border-slate-200 bg-slate-50 px-3 py-1 text-xs font-medium text-slate-500'>
                            +{channel.models.length - 3} {t('channelsModelsMore')}
                          </span>
                        ) : null}
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        ) : (
          <div className='px-6 py-10 text-sm text-slate-500'>{t('channelsEmpty')}</div>
        )}
      </section>
    </div>
  );
}
