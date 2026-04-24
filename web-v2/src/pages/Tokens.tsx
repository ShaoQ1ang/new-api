import { useMemo, useState } from 'react';
import { Gauge, KeyRound, LockKeyhole, Plus, ShieldCheck } from 'lucide-react';
import MetricCard from '../components/ui/MetricCard';
import StatePanel from '../components/ui/StatePanel';
import { useAsyncData } from '../hooks/useAsyncData';
import {
  createToken,
  deleteToken,
  fetchTokenKey,
  fetchTokens,
  updateToken,
  type TokenInput,
} from '../lib/tokens';
import { useI18n } from '../i18n/I18nProvider';

type FilterMode = 'all' | 'active' | 'limited' | 'restricted';

type FormState = {
  id?: number;
  name: string;
  group: string;
  remain_quota: string;
  unlimited_quota: boolean;
  model_limits_enabled: boolean;
  expires_in_days: string;
};

const defaultFormState: FormState = {
  name: '',
  group: 'default',
  remain_quota: '500000',
  unlimited_quota: false,
  model_limits_enabled: false,
  expires_in_days: '',
};

function formatTimestamp(timestamp?: number, neverLabel?: string) {
  if (!timestamp || timestamp <= 0) return neverLabel || '--';
  return new Date(timestamp * 1000).toLocaleDateString('en-US', {
    month: 'short',
    day: 'numeric',
    year: 'numeric',
  });
}

function buildTokenInput(form: FormState): TokenInput {
  const expiresInDays = Number(form.expires_in_days || 0);
  const expiredTime =
    expiresInDays > 0
      ? Math.floor(Date.now() / 1000) + expiresInDays * 24 * 60 * 60
      : -1;

  return {
    id: form.id,
    name: form.name.trim(),
    group: form.group.trim() || 'default',
    remain_quota: form.unlimited_quota ? 0 : Math.max(0, Number(form.remain_quota || 0)),
    unlimited_quota: form.unlimited_quota,
    model_limits_enabled: form.model_limits_enabled,
    expired_time: expiredTime,
  };
}

export default function Tokens() {
  const tokens = useAsyncData(fetchTokens, []);
  const { t } = useI18n();
  const [filter, setFilter] = useState<FilterMode>('all');
  const [isFormOpen, setIsFormOpen] = useState(false);
  const [isEditing, setIsEditing] = useState(false);
  const [form, setForm] = useState<FormState>(defaultFormState);
  const [submitting, setSubmitting] = useState(false);
  const [actionError, setActionError] = useState('');
  const [copiedId, setCopiedId] = useState<number | null>(null);
  const [busyId, setBusyId] = useState<number | null>(null);

  const rows = useMemo(
    () =>
      (tokens.data || []).map((token) => ({
        id: token.id,
        name: token.name || `Token #${token.id}`,
        key: token.key || '',
        group: token.group || 'default',
        active: token.status === 1,
        remainQuota: token.remain_quota || 0,
        statusText: token.status === 1 ? t('tokensStatusActive') : t('tokensStatusInactive'),
        quotaText: token.unlimited_quota
          ? t('tokensUnlimited')
          : `${(token.remain_quota || 0).toLocaleString()} ${t('tokensQuotaRemaining')}`,
        unlimited: Boolean(token.unlimited_quota),
        restricted: Boolean(token.model_limits_enabled),
        accessMode: token.model_limits_enabled ? t('tokensRestricted') : t('tokensStandard'),
        createdAt: token.created_time,
        createdAtText: formatTimestamp(token.created_time),
        expiresAt: token.expired_time,
        expiresAtText: formatTimestamp(token.expired_time, t('tokensNever')),
      })),
    [tokens.data, t],
  );

  const filtered = rows.filter((token) => {
    if (filter === 'active') return token.active;
    if (filter === 'limited') return !token.unlimited;
    if (filter === 'restricted') return token.restricted;
    return true;
  });

  const stats = [
    {
      label: t('tokensMetricTotal'),
      value: String(rows.length),
      hint: t('tokensMetricTotalHint'),
      icon: KeyRound,
    },
    {
      label: t('tokensMetricActive'),
      value: String(rows.filter((item) => item.active).length),
      hint: t('tokensMetricActiveHint'),
      icon: ShieldCheck,
    },
    {
      label: t('tokensMetricUnlimited'),
      value: String(rows.filter((item) => item.unlimited).length),
      hint: t('tokensMetricUnlimitedHint'),
      icon: Gauge,
    },
    {
      label: t('tokensMetricRestricted'),
      value: String(rows.filter((item) => item.restricted).length),
      hint: t('tokensMetricRestrictedHint'),
      icon: LockKeyhole,
    },
  ];

  function openCreateForm() {
    setIsEditing(false);
    setForm(defaultFormState);
    setActionError('');
    setIsFormOpen(true);
  }

  function openEditForm(row: (typeof rows)[number]) {
    const expiresInDays =
      row.expiresAt && row.expiresAt > 0
        ? Math.max(1, Math.ceil((row.expiresAt - Math.floor(Date.now() / 1000)) / 86400))
        : '';

    setIsEditing(true);
    setForm({
      id: row.id,
      name: row.name,
      group: row.group,
      remain_quota: row.unlimited ? '0' : String(row.remainQuota),
      unlimited_quota: row.unlimited,
      model_limits_enabled: row.restricted,
      expires_in_days: String(expiresInDays),
    });
    setActionError('');
    setIsFormOpen(true);
  }

  async function handleSubmit() {
    if (!form.name.trim()) {
      setActionError(t('tokensActionError'));
      return;
    }

    setSubmitting(true);
    setActionError('');

    try {
      const payload = buildTokenInput(form);
      if (isEditing) {
        await updateToken(payload);
      } else {
        await createToken(payload);
      }
      setIsFormOpen(false);
      setForm(defaultFormState);
      await tokens.reload();
    } catch (error) {
      setActionError(error instanceof Error ? error.message : t('tokensActionError'));
    } finally {
      setSubmitting(false);
    }
  }

  async function handleDelete(id: number) {
    if (!window.confirm(t('tokensDeleteConfirm'))) return;

    setBusyId(id);
    setActionError('');
    try {
      await deleteToken(id);
      await tokens.reload();
    } catch (error) {
      setActionError(error instanceof Error ? error.message : t('tokensActionError'));
    } finally {
      setBusyId(null);
    }
  }

  async function handleCopy(id: number) {
    setBusyId(id);
    setActionError('');
    try {
      const key = await fetchTokenKey(id);
      await navigator.clipboard.writeText(`sk-${key}`);
      setCopiedId(id);
      window.setTimeout(() => setCopiedId((current) => (current === id ? null : current)), 1600);
    } catch (error) {
      setActionError(error instanceof Error ? error.message : t('tokensActionError'));
    } finally {
      setBusyId(null);
    }
  }

  return (
    <div className='space-y-5'>
      <section className='rounded-[28px] border border-slate-200 bg-white p-5 shadow-sm'>
        <div className='flex flex-col gap-4 lg:flex-row lg:items-center lg:justify-between'>
          <div>
            <h1 className='text-2xl font-semibold text-slate-950'>{t('tokensEyebrow')}</h1>
            <p className='mt-1 text-sm text-slate-500'>{t('tokensDescription')}</p>
          </div>
          <div className='flex flex-wrap items-center gap-3'>
            <div className='inline-flex rounded-xl border border-slate-200 bg-slate-50 p-1'>
              {(
                [
                  ['all', t('tokensFilterAll')],
                  ['active', t('tokensFilterActive')],
                  ['limited', t('tokensFilterLimited')],
                  ['restricted', t('tokensFilterRestricted')],
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

            <button type='button' onClick={openCreateForm} className='primary-button !px-4 !py-2.5'>
              <Plus className='h-4 w-4' />
              {t('tokensAdd')}
            </button>
          </div>
        </div>
      </section>

      {actionError ? (
        <div className='rounded-2xl border border-rose-200 bg-rose-50 px-4 py-3 text-sm text-rose-700'>
          {actionError}
        </div>
      ) : null}

      <section className='grid gap-4 md:grid-cols-2 xl:grid-cols-4'>
        {stats.map((item) => (
          <MetricCard key={item.label} {...item} />
        ))}
      </section>

      <StatePanel
        loading={tokens.loading}
        error={tokens.error}
        empty={!tokens.loading && !tokens.error && rows.length === 0}
        title={t('tokensStatusTitle')}
        description={t('tokensStatusDescription')}
      />

      <section className='overflow-hidden rounded-[28px] border border-slate-200 bg-white shadow-sm'>
        <div className='overflow-x-auto'>
          <table className='min-w-full'>
            <thead>
              <tr className='border-b border-slate-200 text-left text-xs uppercase tracking-[0.18em] text-slate-500'>
                <th className='px-5 py-4 font-medium'>{t('tokensColumnToken')}</th>
                <th className='px-5 py-4 font-medium'>{t('tokensColumnGroup')}</th>
                <th className='px-5 py-4 font-medium'>{t('tokensColumnStatus')}</th>
                <th className='px-5 py-4 font-medium'>{t('tokensColumnQuota')}</th>
                <th className='px-5 py-4 font-medium'>{t('tokensColumnAccess')}</th>
                <th className='px-5 py-4 font-medium'>{t('tokensColumnExpires')}</th>
                <th className='px-5 py-4 font-medium'>{t('tokensColumnActions')}</th>
              </tr>
            </thead>
            <tbody>
              {filtered.length > 0 ? (
                filtered.map((token) => (
                  <tr key={token.id} className='border-b border-slate-100 text-sm text-slate-700'>
                    <td className='px-5 py-4'>
                      <div className='space-y-1'>
                        <p className='font-medium text-slate-950'>{token.name}</p>
                        {token.key ? <p className='font-mono text-xs text-slate-500'>{token.key}</p> : null}
                      </div>
                    </td>
                    <td className='px-5 py-4'>{token.group}</td>
                    <td className='px-5 py-4'>
                      <span
                        className={
                          token.active
                            ? 'inline-flex rounded-full bg-emerald-50 px-3 py-1 text-xs font-medium text-emerald-700'
                            : 'inline-flex rounded-full bg-slate-100 px-3 py-1 text-xs font-medium text-slate-600'
                        }
                      >
                        {token.statusText}
                      </span>
                    </td>
                    <td className='px-5 py-4'>{token.quotaText}</td>
                    <td className='px-5 py-4'>{token.accessMode}</td>
                    <td className='px-5 py-4 text-slate-500'>{token.expiresAtText}</td>
                    <td className='px-5 py-4'>
                      <div className='flex flex-wrap gap-2'>
                        <button
                          type='button'
                          onClick={() => openEditForm(token)}
                          className='secondary-button !rounded-lg !px-3 !py-2'
                        >
                          {t('tokensEdit')}
                        </button>
                        <button
                          type='button'
                          onClick={() => handleCopy(token.id)}
                          disabled={busyId === token.id}
                          className='secondary-button !rounded-lg !px-3 !py-2'
                        >
                          {copiedId === token.id ? t('tokensCopied') : t('tokensCopy')}
                        </button>
                        <button
                          type='button'
                          onClick={() => handleDelete(token.id)}
                          disabled={busyId === token.id}
                          className='secondary-button !rounded-lg !border-rose-200 !px-3 !py-2 !text-rose-600 hover:!bg-rose-50'
                        >
                          {t('tokensDelete')}
                        </button>
                      </div>
                    </td>
                  </tr>
                ))
              ) : (
                <tr>
                  <td colSpan={7} className='px-5 py-10 text-center text-sm text-slate-500'>
                    {t('tokensEmpty')}
                  </td>
                </tr>
              )}
            </tbody>
          </table>
        </div>
      </section>

      {isFormOpen ? (
        <div className='fixed inset-0 z-50 grid place-items-center bg-slate-950/30 p-4'>
          <div className='w-full max-w-lg rounded-[28px] border border-slate-200 bg-white p-6 shadow-2xl'>
            <div className='flex items-start justify-between gap-4'>
              <div>
                <h2 className='text-xl font-semibold text-slate-950'>
                  {isEditing ? t('tokensFormEditTitle') : t('tokensFormCreateTitle')}
                </h2>
              </div>
              <button
                type='button'
                onClick={() => setIsFormOpen(false)}
                className='secondary-button !rounded-lg !px-3 !py-2'
              >
                {t('tokensCancel')}
              </button>
            </div>

            <div className='mt-6 grid gap-4 sm:grid-cols-2'>
              <div className='space-y-2 sm:col-span-2'>
                <label className='text-sm font-medium text-slate-700'>{t('tokensFormName')}</label>
                <input
                  value={form.name}
                  onChange={(event) => setForm((current) => ({ ...current, name: event.target.value }))}
                  placeholder={t('tokensFormNamePlaceholder')}
                  className='input-shell'
                />
              </div>

              <div className='space-y-2'>
                <label className='text-sm font-medium text-slate-700'>{t('tokensFormGroup')}</label>
                <input
                  value={form.group}
                  onChange={(event) => setForm((current) => ({ ...current, group: event.target.value }))}
                  placeholder={t('tokensFormGroupPlaceholder')}
                  className='input-shell'
                />
              </div>

              <div className='space-y-2'>
                <label className='text-sm font-medium text-slate-700'>{t('tokensFormQuota')}</label>
                <input
                  type='number'
                  min='0'
                  value={form.remain_quota}
                  disabled={form.unlimited_quota}
                  onChange={(event) =>
                    setForm((current) => ({ ...current, remain_quota: event.target.value }))
                  }
                  className='input-shell'
                />
              </div>

              <div className='space-y-2'>
                <label className='text-sm font-medium text-slate-700'>{t('tokensFormExpires')}</label>
                <input
                  type='number'
                  min='0'
                  value={form.expires_in_days}
                  placeholder={t('tokensFormNever')}
                  onChange={(event) =>
                    setForm((current) => ({ ...current, expires_in_days: event.target.value }))
                  }
                  className='input-shell'
                />
              </div>
            </div>

            <div className='mt-5 flex flex-wrap gap-5'>
              <label className='inline-flex items-center gap-3 text-sm text-slate-700'>
                <input
                  type='checkbox'
                  checked={form.unlimited_quota}
                  onChange={(event) =>
                    setForm((current) => ({
                      ...current,
                      unlimited_quota: event.target.checked,
                    }))
                  }
                />
                {t('tokensFormUnlimited')}
              </label>

              <label className='inline-flex items-center gap-3 text-sm text-slate-700'>
                <input
                  type='checkbox'
                  checked={form.model_limits_enabled}
                  onChange={(event) =>
                    setForm((current) => ({
                      ...current,
                      model_limits_enabled: event.target.checked,
                    }))
                  }
                />
                {t('tokensFormRestricted')}
              </label>
            </div>

            <div className='mt-6 flex justify-end gap-3'>
              <button
                type='button'
                onClick={() => setIsFormOpen(false)}
                className='secondary-button !rounded-lg !px-4 !py-2.5'
              >
                {t('tokensCancel')}
              </button>
              <button
                type='button'
                onClick={() => handleSubmit()}
                disabled={submitting}
                className='primary-button !px-4 !py-2.5 disabled:opacity-60'
              >
                {isEditing ? t('tokensSave') : t('tokensCreate')}
              </button>
            </div>
          </div>
        </div>
      ) : null}
    </div>
  );
}
