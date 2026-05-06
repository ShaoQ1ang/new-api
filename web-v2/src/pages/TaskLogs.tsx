import { useMemo, useState } from 'react';
import { CheckCircle2, CircleDashed, Clock3, ExternalLink, Film, PlayCircle, Search, X, XCircle } from 'lucide-react';
import StatePanel from '../components/ui/StatePanel';
import UnifiedPagination from '../components/ui/UnifiedPagination';
import { useAsyncData } from '../hooks/useAsyncData';
import { useI18n } from '../i18n/I18nProvider';
import { fetchTaskLogs } from '../lib/taskLogs';
import type { TaskLogRecord } from '../lib/taskLogs';

function toTimestamp(value: string, fallbackEnd = false) {
  if (!value) return undefined;
  const normalized = fallbackEnd ? `${value}T23:59:59` : `${value}T00:00:00`;
  const timestamp = new Date(normalized).getTime();
  return Number.isNaN(timestamp) ? undefined : Math.floor(timestamp / 1000);
}

function formatDateTime(timestamp?: number) {
  if (!timestamp) return '--';
  return new Date(timestamp * 1000).toLocaleString('en-US', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
    hour12: false,
  });
}

function formatDuration(start?: number, finish?: number) {
  if (!start || !finish || finish <= start) return '--';
  const seconds = finish - start;
  if (seconds < 60) return `${seconds}s`;
  const minutes = Math.floor(seconds / 60);
  const rest = seconds % 60;
  return `${minutes}m ${rest}s`;
}

function getStatusTone(status: string) {
  if (status === 'SUCCESS') return 'bg-emerald-50 text-emerald-700';
  if (status === 'FAILURE') return 'bg-rose-50 text-rose-700';
  if (status === 'IN_PROGRESS') return 'bg-sky-50 text-sky-700';
  if (status === 'QUEUED' || status === 'SUBMITTED') return 'bg-amber-50 text-amber-700';
  return 'bg-slate-100 text-slate-600';
}

function getStatusIcon(status: string) {
  if (status === 'SUCCESS') return CheckCircle2;
  if (status === 'FAILURE') return XCircle;
  if (status === 'IN_PROGRESS') return Film;
  if (status === 'QUEUED' || status === 'SUBMITTED') return Clock3;
  return CircleDashed;
}

function getActionLabel(action: string, t: (key: string) => string) {
  if (action === 'generate') return t('taskActionImageToVideo');
  if (action === 'textGenerate') return t('taskActionTextToVideo');
  if (action === 'firstTailGenerate') return t('taskActionFirstLastFrame');
  if (action === 'referenceGenerate') return t('taskActionReferenceVideo');
  if (action === 'remixGenerate') return t('taskActionRemix');
  if (action === 'MUSIC') return t('taskActionMusic');
  if (action === 'LYRICS') return t('taskActionLyrics');
  return action || '--';
}

function getPlatformLabel(platform: string) {
  if (platform === 'mj') return 'Midjourney';
  if (platform === 'suno') return 'Suno';
  return platform || '--';
}

function isHttpUrl(value?: string) {
  return Boolean(value && /^https?:\/\//.test(value));
}

function getPreviewUrl(row: TaskLogRecord) {
  if (isHttpUrl(row.result_url)) return row.result_url;

  const data = row.data as
    | {
        content?: {
          video_url?: string;
          audio_url?: string;
          image_url?: string;
        };
      }
    | null
    | undefined;

  const next =
    data?.content?.video_url ||
    data?.content?.audio_url ||
    data?.content?.image_url ||
    '';

  return isHttpUrl(next) ? next : '';
}

function getPreviewKind(row: TaskLogRecord) {
  if (row.platform === 'suno' || row.action === 'MUSIC') return 'audio';
  return 'video';
}

export default function TaskLogs() {
  const { t } = useI18n();
  const translate = (key: string) => t(key as never);
  const today = new Date().toISOString().slice(0, 10);
  const [taskId, setTaskId] = useState('');
  const [status, setStatus] = useState('');
  const [platform, setPlatform] = useState('');
  const [startDate, setStartDate] = useState(today);
  const [endDate, setEndDate] = useState(today);
  const [appliedTaskId, setAppliedTaskId] = useState('');
  const [appliedStatus, setAppliedStatus] = useState('');
  const [appliedPlatform, setAppliedPlatform] = useState('');
  const [appliedStartDate, setAppliedStartDate] = useState(today);
  const [appliedEndDate, setAppliedEndDate] = useState(today);
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(20);
  const [previewRow, setPreviewRow] = useState<TaskLogRecord | null>(null);

  const taskLogs = useAsyncData(
    () =>
      fetchTaskLogs({
        page,
        pageSize,
        taskId: appliedTaskId || undefined,
        status: appliedStatus || undefined,
        platform: appliedPlatform || undefined,
        startTimestamp: toTimestamp(appliedStartDate),
        endTimestamp: toTimestamp(appliedEndDate, true),
      }),
    [page, pageSize, appliedTaskId, appliedStatus, appliedPlatform, appliedStartDate, appliedEndDate],
  );

  const rows = taskLogs.data?.items || [];
  const total = taskLogs.data?.total || 0;
  const totalPages = Math.max(1, Math.ceil(total / pageSize));
  const startIndex = total === 0 ? 0 : (page - 1) * pageSize + 1;
  const endIndex = Math.min(total, page * pageSize);

  const metrics = useMemo(() => {
    const success = rows.filter((item) => item.status === 'SUCCESS').length;
    const failed = rows.filter((item) => item.status === 'FAILURE').length;
    const running = rows.filter((item) => ['SUBMITTED', 'QUEUED', 'IN_PROGRESS'].includes(item.status)).length;

    return [
      { label: t('taskMetricTotal'), value: total.toLocaleString(), hint: t('taskMetricTotalHint') },
      { label: t('taskMetricSuccess'), value: success.toLocaleString(), hint: t('taskMetricSuccessHint') },
      { label: t('taskMetricRunning'), value: running.toLocaleString(), hint: t('taskMetricRunningHint') },
      { label: t('taskMetricFailed'), value: failed.toLocaleString(), hint: t('taskMetricFailedHint') },
    ];
  }, [rows, total, t]);

  function applyFilters() {
    setAppliedTaskId(taskId.trim());
    setAppliedStatus(status);
    setAppliedPlatform(platform);
    setAppliedStartDate(startDate);
    setAppliedEndDate(endDate);
    setPage(1);
  }

  function resetFilters() {
    setTaskId('');
    setStatus('');
    setPlatform('');
    setStartDate(today);
    setEndDate(today);
    setAppliedTaskId('');
    setAppliedStatus('');
    setAppliedPlatform('');
    setAppliedStartDate(today);
    setAppliedEndDate(today);
    setPage(1);
  }

  return (
    <section className='space-y-5'>
      <div className='grid gap-4 md:grid-cols-2 xl:grid-cols-4'>
        {metrics.map((item) => (
          <article key={item.label} className='min-h-[132px] rounded-[28px] border border-slate-200 bg-white px-5 py-5 shadow-sm'>
            <p className='min-h-[16px] text-xs font-semibold uppercase tracking-[0.16em] text-slate-400'>{item.label}</p>
            <p className='mt-3 text-[30px] font-semibold tracking-[-0.03em] text-slate-950'>{item.value}</p>
            <p className='mt-2 min-h-[40px] text-sm text-slate-500'>{item.hint}</p>
          </article>
        ))}
      </div>

      <article className='rounded-[30px] border border-slate-200 bg-white p-5 shadow-sm'>
        <div className='flex flex-col gap-3 xl:flex-row xl:items-end xl:justify-between'>
          <div className='min-h-[72px] max-w-[420px]'>
            <p className='text-[11px] font-semibold uppercase tracking-[0.2em] text-slate-400'>{t('taskFiltersEyebrow')}</p>
            <h2 className='mt-2 min-h-[56px] text-xl font-semibold text-slate-950'>{t('taskFiltersTitle')}</h2>
          </div>

          <div className='flex flex-wrap items-center gap-2'>
            <button
              type='button'
              onClick={applyFilters}
              className='inline-flex h-10 min-w-[96px] items-center justify-center gap-2 rounded-full bg-slate-950 px-4 text-sm font-medium text-white'
            >
              <Search className='h-4 w-4' />
              {t('taskQuery')}
            </button>
            <button
              type='button'
              onClick={resetFilters}
              className='inline-flex h-10 min-w-[96px] items-center justify-center rounded-full border border-slate-200 px-4 text-sm font-medium text-slate-600 transition-colors hover:bg-slate-50'
            >
              {t('taskReset')}
            </button>
          </div>
        </div>

        <div className='mt-4 grid gap-3 md:grid-cols-2 xl:grid-cols-5'>
          <label className='flex h-11 items-center gap-2 rounded-2xl border border-slate-200 bg-slate-50 px-3'>
            <input
              value={taskId}
              onChange={(event) => setTaskId(event.target.value)}
              placeholder={t('taskTaskIdPlaceholder')}
              className='w-full bg-transparent text-sm text-slate-700 outline-none placeholder:text-slate-400'
            />
          </label>

          <label className='flex h-11 items-center rounded-2xl border border-slate-200 bg-slate-50 px-3'>
            <select value={status} onChange={(event) => setStatus(event.target.value)} className='w-full bg-transparent text-sm text-slate-700 outline-none'>
              <option value=''>{t('taskStatusAll')}</option>
              <option value='SUBMITTED'>{t('taskStatusSubmitted')}</option>
              <option value='QUEUED'>{t('taskStatusQueued')}</option>
              <option value='IN_PROGRESS'>{t('taskStatusRunning')}</option>
              <option value='SUCCESS'>{t('taskStatusSuccess')}</option>
              <option value='FAILURE'>{t('taskStatusFailure')}</option>
            </select>
          </label>

          <label className='flex h-11 items-center rounded-2xl border border-slate-200 bg-slate-50 px-3'>
            <select value={platform} onChange={(event) => setPlatform(event.target.value)} className='w-full bg-transparent text-sm text-slate-700 outline-none'>
              <option value=''>{t('taskPlatformAll')}</option>
              <option value='mj'>Midjourney</option>
              <option value='suno'>Suno</option>
            </select>
          </label>

          <label className='flex h-11 items-center rounded-2xl border border-slate-200 bg-slate-50 px-3'>
            <input value={startDate} onChange={(event) => setStartDate(event.target.value)} type='date' className='w-full bg-transparent text-sm text-slate-700 outline-none' />
          </label>

          <label className='flex h-11 items-center rounded-2xl border border-slate-200 bg-slate-50 px-3'>
            <input value={endDate} onChange={(event) => setEndDate(event.target.value)} type='date' className='w-full bg-transparent text-sm text-slate-700 outline-none' />
          </label>
        </div>
      </article>

      <article className='overflow-hidden rounded-[30px] border border-slate-200 bg-white shadow-sm'>
        <div className='flex items-center justify-between border-b border-slate-200 px-5 py-4'>
          <div className='min-h-[56px] max-w-[320px]'>
            <p className='text-[11px] font-semibold uppercase tracking-[0.2em] text-slate-400'>{t('taskTableEyebrow')}</p>
            <h2 className='mt-2 min-h-[28px] text-xl font-semibold text-slate-950'>{t('taskTableTitle')}</h2>
          </div>
          <div className='flex items-center gap-3'>
            <label className='flex h-10 min-w-[112px] items-center rounded-full border border-slate-200 px-3 text-sm text-slate-600'>
              <select
                value={pageSize}
                onChange={(event) => {
                  setPageSize(Number(event.target.value));
                  setPage(1);
                }}
                className='w-full bg-transparent outline-none'
              >
                {[10, 20, 50].map((size) => (
                  <option key={size} value={size}>
                    {size} / page
                  </option>
                ))}
              </select>
            </label>
            <p className='min-w-[92px] text-right text-sm text-slate-500'>{t('taskPaginationSummary').replace('{start}', String(startIndex)).replace('{end}', String(endIndex)).replace('{total}', String(total))}</p>
          </div>
        </div>

        <div className='overflow-hidden rounded-b-[30px]'>
        <div className='overflow-x-auto'>
          <table className='min-w-[1380px] divide-y divide-slate-200'>
            <thead className='bg-slate-50'>
              <tr className='text-left text-xs font-semibold uppercase tracking-[0.16em] text-slate-500'>
                <th className='sticky left-0 z-10 rounded-tl-[22px] border-r border-slate-200 bg-slate-50 px-5 py-4 shadow-[12px_0_24px_-18px_rgba(15,23,42,0.18)]'>
                  {t('taskColumnTask')}
                </th>
                <th className='px-5 py-4'>{t('taskColumnType')}</th>
                <th className='px-5 py-4'>{t('taskColumnStatus')}</th>
                <th className='px-5 py-4'>{t('taskColumnSubmitted')}</th>
                <th className='px-5 py-4'>{t('taskColumnFinished')}</th>
                <th className='px-5 py-4'>{t('taskColumnDuration')}</th>
                <th className='sticky right-0 z-10 rounded-tr-[22px] border-l border-slate-200 bg-slate-50 px-5 py-4 shadow-[-12px_0_24px_-18px_rgba(15,23,42,0.18)]'>
                  {t('taskColumnResult')}
                </th>
              </tr>
            </thead>
            <tbody className='divide-y divide-slate-200 bg-white align-top'>
              {rows.map((row) => {
                const StatusIcon = getStatusIcon(row.status);
                return (
                  <tr key={row.id} className='text-sm text-slate-600'>
                    <td className='sticky left-0 z-[1] border-r border-slate-200 bg-white px-5 py-4 shadow-[12px_0_24px_-18px_rgba(15,23,42,0.12)]'>
                      <div className='min-w-[240px]'>
                        <p className='font-medium text-slate-900'>{row.task_id || '--'}</p>
                        <p className='mt-1 text-xs text-slate-500'>{getPlatformLabel(row.platform)}</p>
                      </div>
                    </td>
                    <td className='px-5 py-4'>
                      <div className='min-w-[160px]'>
                        <p className='font-medium text-slate-900'>{getActionLabel(row.action, translate)}</p>
                        <p className='mt-1 text-xs text-slate-500 line-clamp-2'>{row.properties?.origin_model_name || row.properties?.upstream_model_name || '--'}</p>
                      </div>
                    </td>
                    <td className='px-5 py-4'>
                      <div className='min-w-[160px]'>
                        <span className={`inline-flex items-center gap-2 rounded-full px-3 py-1.5 text-xs font-semibold ${getStatusTone(row.status)}`}>
                          <StatusIcon className='h-3.5 w-3.5' />
                          {row.status || '--'}
                        </span>
                        <p className='mt-2 text-xs text-slate-500'>{row.progress || '--'}</p>
                      </div>
                    </td>
                    <td className='px-5 py-4 whitespace-nowrap'>{formatDateTime(row.submit_time)}</td>
                    <td className='px-5 py-4 whitespace-nowrap'>{formatDateTime(row.finish_time)}</td>
                    <td className='px-5 py-4 whitespace-nowrap'>{formatDuration(row.submit_time, row.finish_time)}</td>
                    <td className='sticky right-0 z-[1] border-l border-slate-200 bg-white px-5 py-4 shadow-[-12px_0_24px_-18px_rgba(15,23,42,0.12)]'>
                      <div className='min-w-[260px]'>
                        {getPreviewUrl(row) ? (
                          <div className='flex flex-wrap gap-2'>
                            <button
                              type='button'
                              onClick={() => setPreviewRow(row)}
                              className='inline-flex min-w-[96px] items-center justify-center gap-2 rounded-full bg-slate-950 px-3 py-1.5 text-xs font-semibold text-white transition-opacity hover:opacity-90'
                            >
                              <PlayCircle className='h-3.5 w-3.5' />
                              {t('taskPreviewResult')}
                            </button>
                            <button
                              type='button'
                              onClick={() => window.open(getPreviewUrl(row), '_blank', 'noopener,noreferrer')}
                              className='inline-flex min-w-[112px] items-center justify-center gap-2 rounded-full border border-slate-200 px-3 py-1.5 text-xs font-semibold text-slate-700 transition-colors hover:bg-slate-50'
                            >
                              <ExternalLink className='h-3.5 w-3.5' />
                              {t('taskOpenResult')}
                            </button>
                          </div>
                        ) : null}
                        <p className='mt-2 line-clamp-2 text-xs text-slate-500'>{row.fail_reason || row.properties?.input || '--'}</p>
                      </div>
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
        </div>

        <StatePanel
          loading={taskLogs.loading}
          error={taskLogs.error}
          empty={!taskLogs.loading && !taskLogs.error && rows.length === 0}
          title={t('taskEmptyTitle')}
          description={t('taskEmptyDescription')}
        />

        <UnifiedPagination
          page={page}
          pageSize={pageSize}
          totalItems={total}
          totalPages={totalPages}
          summaryTemplate={t('taskPaginationSummary')}
          pageSizeLabel={t('taskPaginationPerPage')}
          pageSizeOptions={[10, 20, 50, 100]}
          onPageChange={setPage}
          onPageSizeChange={(nextPageSize) => {
            setPageSize(nextPageSize);
            setPage(1);
          }}
        />
      </article>

      {previewRow ? (
        <div className='fixed inset-0 z-50 flex items-center justify-center bg-slate-950/70 p-4'>
          <div className='w-full max-w-5xl rounded-[28px] bg-white shadow-2xl'>
            <div className='flex items-start justify-between border-b border-slate-200 px-5 py-4'>
              <div className='min-w-0'>
                <p className='text-xs font-semibold uppercase tracking-[0.16em] text-slate-400'>{t('taskPreviewEyebrow')}</p>
                <h3 className='mt-2 truncate text-lg font-semibold text-slate-950'>{previewRow.task_id}</h3>
                <p className='mt-1 text-sm text-slate-500'>
                  {getActionLabel(previewRow.action, translate)} · {previewRow.properties?.origin_model_name || previewRow.properties?.upstream_model_name || '--'}
                </p>
              </div>
              <button
                type='button'
                onClick={() => setPreviewRow(null)}
                className='inline-flex h-10 w-10 items-center justify-center rounded-full border border-slate-200 text-slate-500 transition-colors hover:bg-slate-50 hover:text-slate-900'
              >
                <X className='h-4 w-4' />
              </button>
            </div>

            <div className='space-y-4 p-5'>
              <div className='overflow-hidden rounded-[22px] bg-slate-950'>
                {getPreviewKind(previewRow) === 'audio' ? (
                  <div className='flex aspect-video items-center justify-center bg-[radial-gradient(circle_at_top,#1f2937_0%,#020617_75%)] px-6'>
                    <audio src={getPreviewUrl(previewRow)} controls className='w-full max-w-2xl' />
                  </div>
                ) : (
                  <video src={getPreviewUrl(previewRow)} controls className='aspect-video w-full bg-black' />
                )}
              </div>

              <div className='flex flex-wrap items-center gap-3 text-sm text-slate-600'>
                <span>{t('taskColumnSubmitted')}: {formatDateTime(previewRow.submit_time)}</span>
                <span>{t('taskColumnFinished')}: {formatDateTime(previewRow.finish_time)}</span>
                <span>{t('taskColumnDuration')}: {formatDuration(previewRow.submit_time, previewRow.finish_time)}</span>
              </div>

              <p className='text-sm leading-7 text-slate-600'>{previewRow.fail_reason || previewRow.properties?.input || '--'}</p>
            </div>
          </div>
        </div>
      ) : null}
    </section>
  );
}
