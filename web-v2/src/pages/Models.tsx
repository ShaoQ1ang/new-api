import { useMemo, useState } from 'react';
import { Boxes, Image, MessageSquareText, Search, Video } from 'lucide-react';
import MetricCard from '../components/ui/MetricCard';
import StatePanel from '../components/ui/StatePanel';
import { useAsyncData } from '../hooks/useAsyncData';
import { useI18n } from '../i18n/I18nProvider';
import {
  fetchPlaygroundModels,
  type PricingModelRecord,
} from '../lib/playground';

type ModelKind = 'all' | 'chat' | 'image' | 'video' | 'other';

type ModelRow = {
  id: string;
  name: string;
  kind: Exclude<ModelKind, 'all'>;
  tags: string[];
  hint: string;
};

function detectModelKind(model: PricingModelRecord): ModelRow['kind'] {
  const endpointTypes = model.supported_endpoint_types || [];
  const tagText = `${model.tags || ''} ${model.description || ''} ${model.model_name}`.toLowerCase();

  if (endpointTypes.some((item) => item.includes('video')) || tagText.includes('video')) {
    return 'video';
  }
  if (
    endpointTypes.some((item) => item.includes('image')) ||
    tagText.includes('image') ||
    tagText.includes('vision')
  ) {
    return 'image';
  }
  if (
    endpointTypes.some((item) => item.includes('chat') || item.includes('completion')) ||
    tagText.includes('gpt') ||
    tagText.includes('chat')
  ) {
    return 'chat';
  }
  return 'other';
}

export default function Models() {
  const { t } = useI18n();
  const models = useAsyncData(fetchPlaygroundModels, []);
  const [filter, setFilter] = useState<ModelKind>('all');
  const [query, setQuery] = useState('');

  const rows = useMemo<ModelRow[]>(
    () =>
      (models.data || []).map((model) => {
        const kind = detectModelKind(model);
        return {
          id: model.model_name,
          name: model.model_name,
          kind,
          tags: (model.tags || '')
            .split(',')
            .map((item) => item.trim())
            .filter(Boolean),
          hint:
            kind === 'chat'
              ? t('modelsHintChat')
              : kind === 'image'
                ? t('modelsHintImage')
                : kind === 'video'
                  ? t('modelsHintVideo')
                  : t('modelsHintOther'),
        };
      }),
    [models.data, t],
  );

  const filtered = rows.filter((row) => {
    const matchesFilter = filter === 'all' ? true : row.kind === filter;
    const matchesQuery = query
      ? row.name.toLowerCase().includes(query.toLowerCase())
      : true;
    return matchesFilter && matchesQuery;
  });

  const stats = [
    {
      label: t('modelsMetricTotal'),
      value: String(rows.length),
      hint: t('modelsMetricTotalHint'),
      icon: Boxes,
    },
    {
      label: t('modelsMetricChat'),
      value: String(rows.filter((item) => item.kind === 'chat').length),
      hint: t('modelsMetricChatHint'),
      icon: MessageSquareText,
    },
    {
      label: t('modelsMetricImage'),
      value: String(rows.filter((item) => item.kind === 'image').length),
      hint: t('modelsMetricImageHint'),
      icon: Image,
    },
    {
      label: t('modelsMetricVideo'),
      value: String(rows.filter((item) => item.kind === 'video').length),
      hint: t('modelsMetricVideoHint'),
      icon: Video,
    },
  ];

  const filterOptions: Array<[ModelKind, string]> = [
    ['all', t('modelsFilterAll')],
    ['chat', t('modelsFilterChat')],
    ['image', t('modelsFilterImage')],
    ['video', t('modelsFilterVideo')],
    ['other', t('modelsFilterOther')],
  ];

  const kindLabelMap: Record<ModelRow['kind'], string> = {
    chat: t('modelsTypeChat'),
    image: t('modelsTypeImage'),
    video: t('modelsTypeVideo'),
    other: t('modelsTypeOther'),
  };

  return (
    <div className='space-y-5'>
      <section className='rounded-[28px] border border-slate-200 bg-white p-5 shadow-sm'>
        <div className='flex flex-col gap-4 xl:flex-row xl:items-center xl:justify-between'>
          <div>
            <h1 className='text-2xl font-semibold text-slate-950'>{t('modelsNav')}</h1>
            <p className='mt-1 max-w-3xl text-sm text-slate-500'>{t('modelsDescription')}</p>
          </div>
          <label className='inline-flex h-11 min-w-[260px] items-center gap-3 rounded-2xl border border-slate-200 bg-slate-50 px-4 text-sm text-slate-500'>
            <Search className='h-4 w-4 shrink-0' />
            <input
              value={query}
              onChange={(event) => setQuery(event.target.value)}
              placeholder={t('modelsSearchPlaceholder')}
              className='w-full bg-transparent text-slate-700 outline-none placeholder:text-slate-400'
            />
          </label>
        </div>
      </section>

      <section className='grid gap-4 md:grid-cols-2 xl:grid-cols-4'>
        {stats.map((item) => (
          <MetricCard key={item.label} {...item} />
        ))}
      </section>

      <section className='rounded-[28px] border border-slate-200 bg-white p-3 shadow-sm'>
        <div className='flex flex-wrap gap-2'>
          {filterOptions.map(([value, label]) => (
            <button
              key={value}
              type='button'
              onClick={() => setFilter(value)}
              className={
                filter === value
                  ? 'inline-flex h-10 min-w-[88px] items-center justify-center rounded-xl bg-slate-950 px-4 text-sm font-medium text-white'
                  : 'inline-flex h-10 min-w-[88px] items-center justify-center rounded-xl border border-slate-200 bg-white px-4 text-sm font-medium text-slate-600'
              }
            >
              {label}
            </button>
          ))}
        </div>
      </section>

      <StatePanel
        loading={models.loading}
        error={models.error}
        empty={!models.loading && !models.error && filtered.length === 0}
        title={t('modelsEmptyTitle')}
        description={query || filter !== 'all' ? t('modelsEmptyFiltered') : t('modelsEmptyDescription')}
      />

      {filtered.length > 0 ? (
        <section className='grid gap-4 lg:grid-cols-2 2xl:grid-cols-3'>
          {filtered.map((row) => (
            <article
              key={row.id}
              className='rounded-[28px] border border-slate-200 bg-white p-5 shadow-sm transition hover:-translate-y-0.5 hover:shadow-md'
            >
              <div className='flex items-start justify-between gap-4'>
                <div className='min-w-0'>
                  <p className='text-[11px] font-semibold uppercase tracking-[0.2em] text-slate-400'>
                    {kindLabelMap[row.kind]}
                  </p>
                  <h2 className='mt-3 truncate text-lg font-semibold text-slate-950'>{row.name}</h2>
                </div>
                <span className='inline-flex h-8 min-w-[112px] items-center justify-center rounded-full bg-emerald-50 px-3 text-xs font-medium text-emerald-700'>
                  {t('modelsReady')}
                </span>
              </div>
              <p className='mt-4 min-h-[48px] text-sm leading-6 text-slate-500'>{row.hint}</p>
              <div className='mt-5 flex min-h-[36px] flex-wrap gap-2'>
                {row.tags.length > 0 ? (
                  row.tags.slice(0, 4).map((tag) => (
                    <span
                      key={tag}
                      className='inline-flex h-8 items-center rounded-full bg-slate-100 px-3 text-xs font-medium text-slate-600'
                    >
                      {tag}
                    </span>
                  ))
                ) : (
                  <span className='inline-flex h-8 items-center rounded-full bg-slate-100 px-3 text-xs font-medium text-slate-600'>
                    {kindLabelMap[row.kind]}
                  </span>
                )}
              </div>
            </article>
          ))}
        </section>
      ) : null}
    </div>
  );
}
