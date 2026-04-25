import { useEffect, useMemo, useRef, useState } from 'react';
import { CalendarDays, ChevronLeft, ChevronRight, ChevronsLeft, ChevronsRight, CircleHelp, Clock3, DatabaseZap } from 'lucide-react';
import { createPortal } from 'react-dom';
import StatePanel from '../components/ui/StatePanel';
import { useAsyncData } from '../hooks/useAsyncData';
import { useI18n } from '../i18n/I18nProvider';
import { fetchUsageLogs } from '../lib/usageLogs';
import { useStatus } from '../hooks/useStatus';

type PopoverType = 'tokens' | 'cost';

type PopoverState = {
  type: PopoverType;
  rowId: number;
  style: {
    left: number;
    top: number;
  };
} | null;

type ParsedOther = Record<string, unknown>;

function parseOther(other?: string) {
  if (!other) return {};
  try {
    const parsed = JSON.parse(other) as ParsedOther | null;
    return parsed && typeof parsed === 'object' ? parsed : {};
  } catch {
    return {};
  }
}

function clampNumber(value: unknown) {
  const num = typeof value === 'number' ? value : Number(value || 0);
  return Number.isFinite(num) ? num : 0;
}

function formatTime(timestamp: number) {
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

function toDateTimeLocalValue(date: Date) {
  const offset = date.getTimezoneOffset();
  const local = new Date(date.getTime() - offset * 60 * 1000);
  return local.toISOString().slice(0, 16);
}

function fromDateTimeLocalValue(value: string) {
  if (!value) return null;
  const timestamp = new Date(value).getTime();
  if (Number.isNaN(timestamp)) return null;
  return Math.floor(timestamp / 1000);
}

function startOfDay(date: Date) {
  const next = new Date(date);
  next.setHours(0, 0, 0, 0);
  return next;
}

function endOfDay(date: Date) {
  const next = new Date(date);
  next.setHours(23, 59, 59, 999);
  return next;
}

function addDays(date: Date, amount: number) {
  const next = new Date(date);
  next.setDate(next.getDate() + amount);
  return next;
}

function addMonths(date: Date, amount: number) {
  const next = new Date(date);
  next.setMonth(next.getMonth() + amount, 1);
  return next;
}

function startOfWeek(date: Date) {
  const next = startOfDay(date);
  next.setDate(next.getDate() - next.getDay());
  return next;
}

function startOfMonth(date: Date) {
  return new Date(date.getFullYear(), date.getMonth(), 1);
}

function sameDay(left: Date, right: Date) {
  return (
    left.getFullYear() === right.getFullYear() &&
    left.getMonth() === right.getMonth() &&
    left.getDate() === right.getDate()
  );
}

function isWithinRange(date: Date, start: Date, end: Date) {
  const value = startOfDay(date).getTime();
  return value >= startOfDay(start).getTime() && value <= startOfDay(end).getTime();
}

function formatDateInput(date: Date) {
  return new Intl.DateTimeFormat('en-US', {
    month: '2-digit',
    day: '2-digit',
    year: 'numeric',
  }).format(date);
}

function formatTimeInput(date: Date) {
  return new Intl.DateTimeFormat('en-GB', {
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
    hour12: false,
  }).format(date);
}

function buildMonthGrid(month: Date) {
  const firstDay = new Date(month.getFullYear(), month.getMonth(), 1);
  const startWeekday = firstDay.getDay();
  const daysInMonth = new Date(month.getFullYear(), month.getMonth() + 1, 0).getDate();
  const cells: Array<Date | null> = [];

  for (let index = 0; index < startWeekday; index += 1) {
    cells.push(null);
  }

  for (let day = 1; day <= daysInMonth; day += 1) {
    cells.push(new Date(month.getFullYear(), month.getMonth(), day));
  }

  while (cells.length % 7 !== 0) {
    cells.push(null);
  }

  return cells;
}

function extractTimeValue(value: string) {
  return value.slice(11, 19) || '00:00:00';
}

function mergeDateAndTime(dateValue: string, timeValue: string) {
  return `${dateValue.slice(0, 10)}T${timeValue}`;
}

function buildPresetRange(preset: 'last24' | 'today' | 'last7' | 'thisWeek' | 'last30' | 'thisMonth') {
  const now = new Date();

  if (preset === 'last24') return { start: addDays(now, -1), end: now };
  if (preset === 'today') return { start: startOfDay(now), end: now };
  if (preset === 'last7') return { start: addDays(now, -7), end: now };
  if (preset === 'thisWeek') return { start: startOfWeek(now), end: now };
  if (preset === 'thisMonth') return { start: startOfMonth(now), end: now };
  return { start: addDays(now, -30), end: now };
}

function getPresetRangeValues(preset: 'last24' | 'today' | 'last7' | 'thisWeek' | 'last30' | 'thisMonth') {
  const range = buildPresetRange(preset);
  const startValue = toDateTimeLocalValue(range.start);
  const endValue = toDateTimeLocalValue(range.end);

  return {
    startValue,
    endValue,
    startTime: extractTimeValue(startValue),
    endTime: extractTimeValue(endValue),
  };
}

function formatDuration(seconds: number) {
  if (seconds <= 0) return '0.00s';
  return `${seconds.toFixed(2)}s`;
}

function formatFRT(milliseconds: number) {
  if (milliseconds <= 0) return '--';
  return `${(milliseconds / 1000).toFixed(2)}s`;
}

function formatCompactNumber(value: number) {
  if (value >= 1000000) return `${(value / 1000000).toFixed(2)}M`;
  if (value >= 1000) return `${(value / 1000).toFixed(1)}K`;
  return value.toLocaleString();
}

function formatQuota(
  quota: number,
  quotaPerUnit?: number,
  displayType?: string,
  usdExchangeRate?: number,
  customCurrencySymbol?: string,
  customCurrencyExchangeRate?: number,
) {
  if (!quotaPerUnit || quotaPerUnit <= 0 || displayType === 'TOKENS') {
    return quota.toLocaleString();
  }

  const usdValue = quota / quotaPerUnit;

  if (displayType === 'CNY') {
    return `¥${(usdValue * (usdExchangeRate || 1)).toFixed(4)}`;
  }

  if (displayType === 'CUSTOM') {
    return `${customCurrencySymbol || '¤'}${(
      usdValue * (customCurrencyExchangeRate || 1)
    ).toFixed(4)}`;
  }

  return `$${usdValue.toFixed(6)}`;
}

function quotaToUsd(quota: number, quotaPerUnit?: number) {
  if (!quotaPerUnit || quotaPerUnit <= 0) return 0;
  return quota / quotaPerUnit;
}

function formatUsd(usd: number) {
  const sign = usd > 0 ? '+' : usd < 0 ? '-' : '';
  return `${sign}$${Math.abs(usd).toFixed(6)}`;
}

function getRequestTypeLabel(type: number, content: string, t: (key: string) => string) {
  if (type === 1) {
    return {
      label: t('usageTypeTopUp'),
      detail: t('usageTypeCredit'),
      tone: 'sky',
    };
  }

  if (type === 3) {
    return {
      label: t('usageTypeManage'),
      detail: t('usageTypeOperation'),
      tone: 'amber',
    };
  }

  if (type === 4) {
    return {
      label: t('usageTypeSystem'),
      detail: t('usageTypeAutomation'),
      tone: 'violet',
    };
  }

  if (type === 5) {
    return {
      label: t('usageTypeError'),
      detail: t('usageModeStandard'),
      tone: 'rose',
    };
  }

  if (type === 6) {
    return {
      label: t('usageTypeRefund'),
      detail: t('usageTypeSettlement'),
      tone: 'teal',
    };
  }

  return {
    label: t('usageTypeConsume'),
    detail: content.match(/^操作\s+(.+)$/)?.[1] || t('usageModeStandard'),
    tone: 'lime',
  };
}

function getBillingModeLabel(mode: string, t: (key: string) => string) {
  if (mode === 'subscription') return t('usageBillingSubscription');
  return t('usageBillingWallet');
}

function getReasoningEffort(other: ParsedOther, t: (key: string) => string) {
  return typeof other.reasoning_effort === 'string' && other.reasoning_effort
    ? String(other.reasoning_effort)
    : t('usageReasoningNone');
}

function buildEstimatedCostBreakdown(
  promptTokens: number,
  completionTokens: number,
  cacheTokens: number,
  quota: number,
  other: ParsedOther,
  quotaPerUnit?: number,
) {
  const inputTokens = Math.max(0, promptTokens - cacheTokens);
  const outputTokens = Math.max(0, completionTokens);
  const cacheReadTokens = Math.max(0, cacheTokens);

  const inputRatio = Math.max(0, clampNumber(other.model_ratio));
  const outputRatio = Math.max(0, clampNumber(other.completion_ratio));
  const cacheRatio = Math.max(0, clampNumber(other.cache_ratio));

  const weightedInput = inputTokens * inputRatio;
  const weightedOutput = outputTokens * outputRatio;
  const weightedCache = cacheReadTokens * cacheRatio;
  const totalWeighted = weightedInput + weightedOutput + weightedCache;

  if (totalWeighted <= 0) {
    return {
      inputUsd: 0,
      outputUsd: 0,
      cacheUsd: 0,
      billedUsd: quotaToUsd(quota, quotaPerUnit),
      inputRatio,
      outputRatio,
      cacheRatio,
      preConsumedUsd: 0,
      actualUsd: 0,
      deltaUsd: quotaToUsd(quota, quotaPerUnit),
    };
  }

  const totalUsd = quotaToUsd(quota, quotaPerUnit);
  return {
    inputUsd: totalUsd * (weightedInput / totalWeighted),
    outputUsd: totalUsd * (weightedOutput / totalWeighted),
    cacheUsd: totalUsd * (weightedCache / totalWeighted),
    billedUsd: totalUsd,
    inputRatio,
    outputRatio,
    cacheRatio,
    preConsumedUsd: 0,
    actualUsd: 0,
    deltaUsd: totalUsd,
  };
}

function buildSettlementBreakdown(other: ParsedOther, quotaPerUnit?: number) {
  const preConsumedQuota = clampNumber(other.pre_consumed_quota);
  const actualQuota = clampNumber(other.actual_quota);
  const deltaQuota = clampNumber(other.pre_consumed_quota) - clampNumber(other.actual_quota);

  return {
    inputUsd: 0,
    outputUsd: 0,
    cacheUsd: 0,
    billedUsd: quotaToUsd(deltaQuota, quotaPerUnit),
    inputRatio: 0,
    outputRatio: 0,
    cacheRatio: 0,
    preConsumedUsd: quotaToUsd(preConsumedQuota, quotaPerUnit),
    actualUsd: quotaToUsd(actualQuota, quotaPerUnit),
    deltaUsd: quotaToUsd(deltaQuota, quotaPerUnit),
  };
}

function getUserAgent(other: ParsedOther) {
  return (
    (typeof other.user_agent === 'string' && other.user_agent) ||
    (typeof other['user-agent'] === 'string' && other['user-agent']) ||
    (typeof other.ua === 'string' && other.ua) ||
    ''
  );
}

function getEndpoint(other: ParsedOther) {
  return typeof other.request_path === 'string' && other.request_path
    ? String(other.request_path)
    : '--';
}

function getToneClass(tone: 'sky' | 'amber' | 'violet' | 'teal' | 'lime' | 'rose') {
  if (tone === 'sky') {
    return 'bg-sky-50 text-sky-700';
  }
  if (tone === 'amber') {
    return 'bg-amber-50 text-amber-700';
  }
  if (tone === 'violet') {
    return 'bg-violet-50 text-violet-700';
  }
  if (tone === 'teal') {
    return 'bg-emerald-50 text-emerald-700';
  }
  if (tone === 'rose') {
    return 'bg-rose-50 text-rose-700';
  }
  return 'bg-lime-50 text-lime-700';
}

function formatPaginationSummary(template: string, start: number, end: number, total: number) {
  return template
    .replace('{start}', String(start))
    .replace('{end}', String(end))
    .replace('{total}', String(total));
}

function formatRatio(value: unknown) {
  const num = clampNumber(value);
  return num > 0 ? `${num.toFixed(2)}x` : null;
}

function buildDetailsSummary(
  type: number,
  content: string | undefined,
  other: ParsedOther,
  billingMode: string,
  t: (key: string) => string,
) {
  if (type === 1) {
    return [content || t('usageDetailTopUp'), t('usageTypeCredit')];
  }

  if (type === 3) {
    return [content || t('usageDetailManage'), t('usageTypeOperation')];
  }

  if (type === 4) {
    return [content || t('usageDetailSystem'), t('usageTypeAutomation')];
  }

  if (type === 1 || type === 3 || type === 4 || type === 5) {
    return [content || t('usageDetailUpstreamError')];
  }

  if (type === 6) {
    const taskId = typeof other.task_id === 'string' ? other.task_id : '';
    return [t('usageDetailRefund'), taskId ? `Task ${taskId}` : t('usageDetailSettlement')];
  }

  const lines: string[] = [];
  const groupRatio = formatRatio(other.user_group_ratio) || formatRatio(other.group_ratio);
  const modelRatio = formatRatio(other.model_ratio);
  const completionRatio = formatRatio(other.completion_ratio);

  if (other.is_task === true || other.task_id) {
    lines.push(t('usageDetailAsyncTask'));
  }

  if (billingMode === t('usageBillingSubscription')) {
    lines.push(t('usageDetailSubscription'));
  }

  if (groupRatio) {
    lines.push(`${t('usageDetailGroupRatio')} ${groupRatio}`);
  }

  if (modelRatio || completionRatio) {
    lines.push(
      [modelRatio ? `${t('usageTokenInput')} ${modelRatio}` : '', completionRatio ? `${t('usageTokenOutput')} ${completionRatio}` : '']
        .filter(Boolean)
        .join(' / '),
    );
  }

  if (!lines.length && content) {
    lines.push(content);
  }

  return lines.length ? lines.slice(0, 2) : ['--'];
}

function getPopoverSize(type: PopoverType) {
  return type === 'tokens'
    ? { width: 272, height: 168 }
    : { width: 304, height: 238 };
}

function getPopoverPosition(type: PopoverType, rect: DOMRect) {
  const gap = 10;
  const margin = 12;
  const { width, height } = getPopoverSize(type);
  const viewportWidth = window.innerWidth;
  const viewportHeight = window.innerHeight;

  let left =
    type === 'tokens'
      ? rect.right + gap
      : rect.left - width - gap;

  if (left + width > viewportWidth - margin) {
    left = rect.left - width - gap;
  }
  if (left < margin) {
    left = Math.min(viewportWidth - width - margin, rect.right + gap);
  }

  let top = rect.top - height - gap;
  if (top < margin) {
    top = rect.bottom + gap;
  }
  if (top + height > viewportHeight - margin) {
    top = Math.max(margin, viewportHeight - height - margin);
  }

  return {
    left: Math.max(margin, left),
    top: Math.max(margin, top),
  };
}

export default function UsageLogs() {
  const { t } = useI18n();
  const translate = (key: string) => t(key as never);
  const status = useStatus();
  const defaultCustomRange = getPresetRangeValues('last24');
  const [days, setDays] = useState<1 | 7 | 30>(1);
  const [useCustomRange, setUseCustomRange] = useState(false);
  const [customPreset, setCustomPreset] = useState<'last24' | 'today' | 'last7' | 'thisWeek' | 'last30' | 'thisMonth'>('last24');
  const [customStart, setCustomStart] = useState(defaultCustomRange.startValue);
  const [customEnd, setCustomEnd] = useState(defaultCustomRange.endValue);
  const [customStartTime, setCustomStartTime] = useState(defaultCustomRange.startTime);
  const [customEndTime, setCustomEndTime] = useState(defaultCustomRange.endTime);
  const [appliedDays, setAppliedDays] = useState<1 | 7 | 30>(1);
  const [appliedUseCustomRange, setAppliedUseCustomRange] = useState(false);
  const [appliedCustomStart, setAppliedCustomStart] = useState(defaultCustomRange.startValue);
  const [appliedCustomEnd, setAppliedCustomEnd] = useState(defaultCustomRange.endValue);
  const [isDatePickerOpen, setIsDatePickerOpen] = useState(false);
  const [pickerMonth, setPickerMonth] = useState(() => new Date());
  const [pickerSelectionStep, setPickerSelectionStep] = useState<'start' | 'end'>('start');
  const [tokenName, setTokenName] = useState('');
  const [modelName, setModelName] = useState('');
  const [requestId, setRequestId] = useState('');
  const [logType, setLogType] = useState<0 | 1 | 2 | 3 | 4 | 5 | 6>(0);
  const [appliedTokenName, setAppliedTokenName] = useState('');
  const [appliedModelName, setAppliedModelName] = useState('');
  const [appliedRequestId, setAppliedRequestId] = useState('');
  const [appliedLogType, setAppliedLogType] = useState<0 | 1 | 2 | 3 | 4 | 5 | 6>(0);
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(20);
  const [popover, setPopover] = useState<PopoverState>(null);
  const closeTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const datePickerRef = useRef<HTMLDivElement | null>(null);

  useEffect(() => {
    if (!isDatePickerOpen) return;

    function handlePointerDown(event: MouseEvent) {
      if (!datePickerRef.current) return;
      if (datePickerRef.current.contains(event.target as Node)) return;
      setIsDatePickerOpen(false);
    }

    function handleEscape(event: KeyboardEvent) {
      if (event.key === 'Escape') {
        setIsDatePickerOpen(false);
      }
    }

    document.addEventListener('mousedown', handlePointerDown);
    document.addEventListener('keydown', handleEscape);

    return () => {
      document.removeEventListener('mousedown', handlePointerDown);
      document.removeEventListener('keydown', handleEscape);
    };
  }, [isDatePickerOpen]);

  const usage = useAsyncData(
    () =>
      fetchUsageLogs({
        days:
          appliedUseCustomRange || appliedDays === 1
            ? undefined
            : appliedDays,
        startTimestamp: appliedUseCustomRange
          ? fromDateTimeLocalValue(appliedCustomStart) || undefined
          : appliedDays === 1
            ? Math.floor(startOfDay(new Date()).getTime() / 1000)
            : undefined,
        endTimestamp: appliedUseCustomRange
          ? fromDateTimeLocalValue(appliedCustomEnd) || undefined
          : appliedDays === 1
            ? Math.floor(Date.now() / 1000)
            : undefined,
        tokenName: appliedTokenName,
        modelName: appliedModelName,
        requestId: appliedRequestId,
        logType: appliedLogType,
        page,
        pageSize,
      }),
    [appliedDays, appliedUseCustomRange, appliedCustomStart, appliedCustomEnd, appliedTokenName, appliedModelName, appliedRequestId, appliedLogType, page, pageSize],
  );

  const quotaPerUnit = status.data?.quota_per_unit;
  const quotaDisplayType = status.data?.quota_display_type;
  const usdExchangeRate = status.data?.usd_exchange_rate;
  const customCurrencySymbol = status.data?.custom_currency_symbol;
  const customCurrencyExchangeRate = status.data?.custom_currency_exchange_rate;

  const rows = useMemo(() => {
    return (usage.data?.items || [])
      .filter((item) => item.type >= 1 && item.type <= 6)
      .map((item) => {
        const other = parseOther(item.other);
        const cacheTokens = Math.max(0, clampNumber(other.cache_tokens));
        const firstTokenMs = Math.max(0, clampNumber(other.frt));
        const durationSeconds = Math.max(0, item.use_time || 0);
        const requestType = getRequestTypeLabel(item.type, item.content || '', translate);
        const billingMode = typeof other.billing_source === 'string' ? String(other.billing_source) : 'wallet';
        const isSettlement = item.type === 6;
        const isQuotaCredit = item.type === 1;
        const quotaValue = Math.max(0, item.quota || 0);
        const costBreakdown = isSettlement
          ? buildSettlementBreakdown(other, quotaPerUnit)
          : buildEstimatedCostBreakdown(
              item.prompt_tokens || 0,
              item.completion_tokens || 0,
              cacheTokens,
              quotaValue,
              other,
              quotaPerUnit,
            );

        const spendQuota = isSettlement ? clampNumber(other.pre_consumed_quota) - clampNumber(other.actual_quota) : quotaValue;
        const spendUsd = isSettlement ? costBreakdown.deltaUsd : quotaToUsd(quotaValue, quotaPerUnit);

        return {
          id: item.id,
          tokenName: item.token_name || 'default',
          modelName: item.model_name || '--',
          reasoningEffort: getReasoningEffort(other, translate),
          requestType,
          endpoint: getEndpoint(other),
          billingMode: getBillingModeLabel(billingMode, translate),
          streamType: item.is_stream ? t('usageModeStream') : t('usageModeStandard'),
          inputTokens: Math.max(0, (item.prompt_tokens || 0) - cacheTokens),
          promptTokens: Math.max(0, item.prompt_tokens || 0),
          outputTokens: item.completion_tokens || 0,
          cacheReadTokens: cacheTokens,
          totalTokens: item.type === 2 || item.type === 5 ? (item.prompt_tokens || 0) + (item.completion_tokens || 0) : 0,
          spend: formatQuota(
            Math.max(0, spendQuota),
            quotaPerUnit,
            quotaDisplayType,
            usdExchangeRate,
            customCurrencySymbol,
            customCurrencyExchangeRate,
          ),
          spendUsd,
          isPositiveSpend: isSettlement || isQuotaCredit,
          costBreakdown,
          details: buildDetailsSummary(item.type, item.content, other, getBillingModeLabel(billingMode, translate), translate),
          firstToken: isSettlement ? '--' : formatFRT(firstTokenMs),
          duration: isSettlement ? '--' : formatDuration(durationSeconds),
          time: formatTime(item.created_at),
          userAgent: getUserAgent(other),
        };
      });
  }, [
    usage.data?.items,
    t,
    quotaPerUnit,
    quotaDisplayType,
    usdExchangeRate,
    customCurrencySymbol,
    customCurrencyExchangeRate,
  ]);

  const totalItems = usage.data?.total || 0;
  const totalPages = Math.max(1, Math.ceil(totalItems / pageSize));
  const hasUserAgentColumn = rows.some((row) => row.userAgent);
  const paginationSummary = formatPaginationSummary(
    translate('usagePaginationSummary'),
    totalItems === 0 ? 0 : (page - 1) * pageSize + 1,
    Math.min(page * pageSize, totalItems),
    totalItems,
  );
  const tokenOptions = Array.from(
    new Set(
      (usage.data?.items || [])
        .filter((item) => item.type >= 1 && item.type <= 6)
        .map((item) => item.token_name || 'default'),
    ),
  );
  const logTypeOptions = [
    { value: 0, label: t('usageFilterTypeAll') },
    { value: 1, label: t('usageTypeTopUp') },
    { value: 2, label: t('usageTypeConsume') },
    { value: 3, label: t('usageTypeManage') },
    { value: 4, label: t('usageTypeSystem') },
    { value: 5, label: t('usageTypeError') },
    { value: 6, label: t('usageTypeRefund') },
  ] as const satisfies ReadonlyArray<{ value: 0 | 1 | 2 | 3 | 4 | 5 | 6; label: string }>;

  const summary = {
    matchingRecords: totalItems,
    consumedQuota: usage.data?.stat?.quota || 0,
    rpm: usage.data?.stat?.rpm || 0,
    tpm: usage.data?.stat?.tpm || 0,
  };

  function clearCloseTimer() {
    if (closeTimerRef.current) {
      clearTimeout(closeTimerRef.current);
      closeTimerRef.current = null;
    }
  }

  function scheduleClosePopover() {
    clearCloseTimer();
    closeTimerRef.current = setTimeout(() => setPopover(null), 120);
  }

  function openPopover(type: PopoverType, rowId: number, target: HTMLElement) {
    clearCloseTimer();
    const rect = target.getBoundingClientRect();
    setPopover({
      type,
      rowId,
      style: getPopoverPosition(type, rect),
    });
  }

  function applyFilters() {
    setCustomStart(mergeDateAndTime(customStart, customStartTime));
    setCustomEnd(mergeDateAndTime(customEnd, customEndTime));
    setAppliedDays(days);
    setAppliedUseCustomRange(useCustomRange);
    setAppliedCustomStart(mergeDateAndTime(customStart, customStartTime));
    setAppliedCustomEnd(mergeDateAndTime(customEnd, customEndTime));
    setAppliedTokenName(tokenName);
    setAppliedModelName(modelName);
    setAppliedRequestId(requestId);
    setAppliedLogType(logType);
    setPage(1);
  }

  function setTodayDraftRange() {
    setUseCustomRange(false);
    setDays(1);
    setIsDatePickerOpen(false);
  }

  function applyPresetRange(preset: 'last24' | 'today' | 'last7' | 'thisWeek' | 'last30' | 'thisMonth') {
    const { startValue, endValue, startTime, endTime } = getPresetRangeValues(preset);
    setCustomPreset(preset);
    setCustomStart(startValue);
    setCustomEnd(endValue);
    setCustomStartTime(startTime);
    setCustomEndTime(endTime);
  }

  function handleCustomDateClick(date: Date) {
    const currentStart = new Date(customStart);
    const currentEnd = new Date(customEnd);

    if (pickerSelectionStep === 'start') {
      const nextStart = startOfDay(date);
      const nextEnd = currentEnd.getTime() < nextStart.getTime() ? endOfDay(date) : currentEnd;
      setCustomStart(toDateTimeLocalValue(nextStart));
      setCustomEnd(toDateTimeLocalValue(nextEnd));
      setCustomStartTime(extractTimeValue(toDateTimeLocalValue(nextStart)));
      setCustomEndTime(extractTimeValue(toDateTimeLocalValue(nextEnd)));
      setPickerSelectionStep('end');
      return;
    }

    const nextStart = currentStart.getTime() > date.getTime() ? startOfDay(date) : currentStart;
    const nextEnd = currentStart.getTime() > date.getTime() ? endOfDay(currentStart) : endOfDay(date);
    setCustomStart(toDateTimeLocalValue(nextStart));
    setCustomEnd(toDateTimeLocalValue(nextEnd));
    setCustomStartTime(extractTimeValue(toDateTimeLocalValue(nextStart)));
    setCustomEndTime(extractTimeValue(toDateTimeLocalValue(nextEnd)));
    setPickerSelectionStep('start');
  }

  const customStartDate = new Date(customStart);
  const customEndDate = new Date(customEnd);
  const leftMonth = new Date(pickerMonth.getFullYear(), pickerMonth.getMonth(), 1);
  const rightMonth = addMonths(leftMonth, 1);
  const weekDays = ['Sun', 'Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat'];

  return (
    <div className='space-y-4'>
      <section className='rounded-[24px] border border-slate-200 bg-white p-3 shadow-sm'>
        <div className='space-y-3'>
          <div className='grid gap-2.5 md:grid-cols-2 xl:grid-cols-4'>
            <article className='rounded-[18px] border border-slate-200 bg-slate-50/70 px-3.5 py-2.5'>
              <p className='text-[9px] uppercase tracking-[0.18em] text-slate-400'>{t('usageMatchingRecords')}</p>
              <p className='mt-1 text-[26px] font-semibold leading-none text-slate-950'>{summary.matchingRecords.toLocaleString()}</p>
              <p className='mt-1.5 text-[10px] text-slate-500'>{t('usageMetricQuery')}</p>
            </article>
            <article className='rounded-[18px] border border-slate-200 bg-slate-50/70 px-3.5 py-2.5'>
              <p className='text-[9px] uppercase tracking-[0.18em] text-slate-400'>{t('usageConsumedQuota')}</p>
              <p className='mt-1 text-[26px] font-semibold leading-none text-slate-950'>
                {formatQuota(
                  summary.consumedQuota,
                  quotaPerUnit,
                  quotaDisplayType,
                  usdExchangeRate,
                  customCurrencySymbol,
                  customCurrencyExchangeRate,
                )}
              </p>
              <p className='mt-1.5 text-[10px] text-slate-500'>{t('usageMetricConsumeOnly')}</p>
            </article>
            <article className='rounded-[18px] border border-slate-200 bg-slate-50/70 px-3.5 py-2.5'>
              <p className='text-[9px] uppercase tracking-[0.18em] text-slate-400'>{t('usageCurrentRpm')}</p>
              <p className='mt-1 text-[26px] font-semibold leading-none text-slate-950'>{summary.rpm.toLocaleString()}</p>
              <p className='mt-1.5 text-[10px] text-slate-500'>{t('usageMetricLiveWindow')}</p>
            </article>
            <article className='rounded-[18px] border border-slate-200 bg-slate-50/70 px-3.5 py-2.5'>
              <p className='text-[9px] uppercase tracking-[0.18em] text-slate-400'>{t('usageCurrentTpm')}</p>
              <p className='mt-1 text-[26px] font-semibold leading-none text-slate-950'>{formatCompactNumber(summary.tpm)}</p>
              <p className='mt-1.5 text-[10px] text-slate-500'>{t('usageMetricLiveWindow')}</p>
            </article>
          </div>

          <div className='rounded-[18px] border border-slate-200 bg-gradient-to-b from-slate-50 to-white p-3'>
            <div className='flex flex-col gap-3'>
              <div className='grid gap-3 md:grid-cols-2 xl:grid-cols-[220px_1fr_1fr]'>
                <select
                  value={tokenName}
                  onChange={(event) => {
                    setTokenName(event.target.value);
                    setPage(1);
                  }}
                  aria-label={t('usageFilterToken')}
                  className='input-shell !h-11 !w-full !rounded-2xl !px-4 !py-2 text-sm'
                >
                  <option value=''>{t('usageFilterTokenAll')}</option>
                  {tokenOptions.map((option) => (
                    <option key={option} value={option}>
                      {option}
                    </option>
                  ))}
                </select>

                <input
                  value={modelName}
                  onChange={(event) => {
                    setModelName(event.target.value);
                    setPage(1);
                  }}
                  placeholder={t('usageFilterModelPlaceholder')}
                  aria-label={t('usageFilterModel')}
                  className='input-shell !h-11 !w-full !rounded-2xl !px-4 !py-2 text-sm'
                />

                <input
                  value={requestId}
                  onChange={(event) => {
                    setRequestId(event.target.value);
                    setPage(1);
                  }}
                  placeholder={t('usageFilterRequestIdPlaceholder')}
                  aria-label={t('usageFilterRequestId')}
                  className='input-shell !h-11 !w-full !rounded-2xl !px-4 !py-2 text-sm'
                />
              </div>

              <div className='flex flex-col gap-3 xl:flex-row xl:items-center xl:justify-between'>
                <div className='flex flex-1 flex-wrap items-center gap-2'>
                  <select
                    value={String(logType)}
                    onChange={(event) => {
                      setLogType(Number(event.target.value) as 0 | 1 | 2 | 3 | 4 | 5 | 6);
                      setPage(1);
                    }}
                    aria-label={t('usageFilterType')}
                    className='input-shell !h-11 !w-[160px] !rounded-2xl !px-4 !py-2 text-sm'
                  >
                    {logTypeOptions.map((option) => (
                      <option key={option.value} value={option.value}>
                        {option.label}
                      </option>
                    ))}
                  </select>

                  <div className='grid min-w-[220px] max-w-full grid-cols-2 rounded-2xl border border-slate-200 bg-slate-100/80 p-1'>
                    {([
                      ['today', t('usageRangeToday')],
                      ['custom', t('usageRangeCustom')],
                    ] as const).map(([value, label]) => (
                      <button
                        key={value}
                        type='button'
                        onClick={() => {
                          if (value === 'today') {
                            setTodayDraftRange();
                          } else {
                            setUseCustomRange(true);
                            setIsDatePickerOpen((current) => !current);
                          }
                        }}
                        className={
                          (value === 'today' && !useCustomRange) || (value === 'custom' && useCustomRange)
                            ? 'min-w-[96px] whitespace-nowrap rounded-xl bg-white px-4 py-2.5 text-sm font-medium text-slate-950 shadow-sm'
                            : 'min-w-[96px] whitespace-nowrap rounded-xl px-4 py-2.5 text-sm text-slate-500'
                        }
                      >
                        {label}
                      </button>
                    ))}
                  </div>

                  {useCustomRange ? (
                    <div className='relative' ref={datePickerRef}>
                      <button
                        type='button'
                        onClick={() => setIsDatePickerOpen((current) => !current)}
                        className='inline-flex h-11 min-w-[320px] items-center justify-between rounded-2xl border border-slate-200 bg-white px-4 text-sm text-slate-700'
                      >
                        <span>
                          {formatDateInput(customStartDate)} {formatTimeInput(customStartDate)} - {formatDateInput(customEndDate)} {formatTimeInput(customEndDate)}
                        </span>
                        <span className='text-slate-400'>▾</span>
                      </button>

                      {isDatePickerOpen ? (
                        <div className='absolute left-0 top-[calc(100%+10px)] z-30 w-[min(100vw-48px,620px)] rounded-[18px] border border-slate-200 bg-white p-2.5 shadow-[0_24px_80px_-32px_rgba(15,23,42,0.28)]'>
                          <div className='grid gap-2.5 xl:grid-cols-2'>
                            {[leftMonth, rightMonth].map((month, monthIndex) => (
                              <div key={`${month.getFullYear()}-${month.getMonth()}`}>
                                <div className='mb-2 flex items-center justify-between'>
                                  <div className='flex items-center gap-1'>
                                    <button
                                      type='button'
                                      onClick={() => setPickerMonth((current) => addMonths(current, monthIndex === 0 ? -12 : 12))}
                                      className='inline-flex h-7 w-7 items-center justify-center rounded-full text-slate-500 hover:bg-slate-100'
                                    >
                                      {monthIndex === 0 ? <ChevronsLeft className='h-3.5 w-3.5' /> : <ChevronsRight className='h-3.5 w-3.5' />}
                                    </button>
                                    <button
                                      type='button'
                                      onClick={() => setPickerMonth((current) => addMonths(current, monthIndex === 0 ? -1 : 1))}
                                      className='inline-flex h-7 w-7 items-center justify-center rounded-full text-slate-500 hover:bg-slate-100'
                                    >
                                      {monthIndex === 0 ? <ChevronLeft className='h-3.5 w-3.5' /> : <ChevronRight className='h-3.5 w-3.5' />}
                                    </button>
                                  </div>
                                  <p className='text-[15px] font-semibold text-slate-950'>
                                    {month.toLocaleString('en-US', { month: 'short', year: 'numeric' })}
                                  </p>
                                  <div className='w-7' />
                                </div>
                                <div className='grid grid-cols-7 gap-y-0.5 text-center text-[11px] text-slate-400'>
                                  {weekDays.map((day) => (
                                    <div key={`${monthIndex}-${day}`} className='py-1 font-medium'>
                                      {day}
                                    </div>
                                  ))}
                                  {buildMonthGrid(month).map((date, index) => {
                                    if (!date) {
                                      return <div key={`${monthIndex}-empty-${index}`} className='h-7' />;
                                    }

                                    const isStart = sameDay(date, customStartDate);
                                    const isEnd = sameDay(date, customEndDate);
                                    const inRange = isWithinRange(date, customStartDate, customEndDate);

                                    return (
                                      <button
                                        key={`${monthIndex}-${date.toISOString()}`}
                                        type='button'
                                        onClick={() => handleCustomDateClick(date)}
                                        className={
                                          isStart || isEnd
                                            ? 'mx-auto h-7 w-7 rounded-md bg-blue-600 text-[12px] font-semibold text-white'
                                            : inRange
                                              ? 'mx-auto h-7 w-7 rounded-md bg-blue-50 text-[12px] font-medium text-blue-600'
                                              : 'mx-auto h-7 w-7 rounded-md text-[12px] text-slate-700 hover:bg-slate-100'
                                        }
                                      >
                                        {date.getDate()}
                                      </button>
                                    );
                                  })}
                                </div>
                              </div>
                            ))}
                          </div>

                          <div className='mt-2.5 flex flex-wrap items-center gap-2 border-t border-slate-100 pt-2.5'>
                            <div className='flex items-center gap-1.5 rounded-2xl border border-slate-200 bg-white px-2.5 py-1.5 text-[13px] font-medium text-slate-700'>
                              <CalendarDays className='h-3.5 w-3.5 text-slate-500' />
                              <span>{formatDateInput(customStartDate)}</span>
                              <Clock3 className='h-3.5 w-3.5 text-slate-400' />
                              <input
                                type='time'
                                step={1}
                                value={customStartTime}
                                onChange={(event) => {
                                  setCustomStartTime(event.target.value);
                                  setCustomStart(mergeDateAndTime(customStart, event.target.value));
                                }}
                                className='w-[100px] rounded-lg border border-slate-200 bg-slate-50 px-2 py-1 text-[12px] text-slate-600'
                              />
                            </div>
                            <div className='flex items-center gap-1.5 rounded-2xl border border-slate-200 bg-white px-2.5 py-1.5 text-[13px] font-medium text-slate-700'>
                              <CalendarDays className='h-3.5 w-3.5 text-slate-500' />
                              <span>{formatDateInput(customEndDate)}</span>
                              <Clock3 className='h-3.5 w-3.5 text-slate-400' />
                              <input
                                type='time'
                                step={1}
                                value={customEndTime}
                                onChange={(event) => {
                                  setCustomEndTime(event.target.value);
                                  setCustomEnd(mergeDateAndTime(customEnd, event.target.value));
                                }}
                                className='w-[100px] rounded-lg border border-slate-200 bg-slate-50 px-2 py-1 text-[12px] text-slate-600'
                              />
                            </div>
                          </div>
                          <div className='mt-2.5 flex flex-wrap items-center gap-1.5'>
                              {([
                                ['last24', t('usageRange1d')],
                                ['last7', t('usageRange7d')],
                                ['thisWeek', t('usageRangeThisWeek')],
                                ['last30', t('usageRange30d')],
                                ['thisMonth', t('usageRangeThisMonth')],
                              ] as const).map(([value, label]) => (
                                <button
                                  key={`preset-${value}`}
                                  type='button'
                                  onClick={() => {
                                    applyPresetRange(value);
                                  }}
                                  className={
                                    customPreset === value
                                      ? 'rounded-full bg-slate-950 px-3.5 py-1.5 text-[12px] font-medium text-white'
                                      : 'rounded-full bg-slate-100 px-3.5 py-1.5 text-[12px] font-medium text-slate-600'
                                  }
                                >
                                  {label}
                              </button>
                            ))}
                          </div>
                        </div>
                      ) : null}
                    </div>
                  ) : null}
                </div>

                <div className='flex flex-wrap items-center gap-2 xl:justify-end'>
                    <button
                      type='button'
                      onClick={() => {
                        setTokenName('');
                        setModelName('');
                        setRequestId('');
                        setLogType(0);
                        setAppliedTokenName('');
                        setAppliedModelName('');
                        setAppliedRequestId('');
                        setAppliedLogType(0);
                        setUseCustomRange(false);
                        setDays(1);
                        setAppliedDays(1);
                        setAppliedUseCustomRange(false);
                        setCustomPreset('last24');
                        setCustomStart(defaultCustomRange.startValue);
                        setCustomEnd(defaultCustomRange.endValue);
                        setCustomStartTime(defaultCustomRange.startTime);
                        setCustomEndTime(defaultCustomRange.endTime);
                        setAppliedCustomStart(defaultCustomRange.startValue);
                        setAppliedCustomEnd(defaultCustomRange.endValue);
                        setPage(1);
                      }}
                      className='secondary-button !h-11 !w-[88px] !justify-center !rounded-2xl !px-4 !py-2 font-medium'
                    >
                      {t('usageReset')}
                    </button>
                    <button type='button' onClick={applyFilters} className='primary-button !h-11 !w-[112px] !justify-center whitespace-nowrap !rounded-2xl !px-4 !py-2'>
                      {t('usageQuery')}
                    </button>
                </div>
              </div>
            </div>
          </div>
        </div>
      </section>

      {usage.loading || usage.error ? (
        <StatePanel
          loading={usage.loading}
          error={usage.error}
          empty={false}
          title={t('usageLoadingTitle')}
          description={t('usageLoadingDescription')}
        />
      ) : null}

      <section className='overflow-hidden rounded-[24px] border border-slate-200 bg-white shadow-sm'>
        <div className='overflow-x-auto no-scrollbar bg-white'>
          <table className={`table-fixed ${hasUserAgentColumn ? 'min-w-[2250px]' : 'min-w-[1910px]'}`}>
            <thead>
              <tr className='border-b border-slate-200 text-left text-[11px] uppercase tracking-[0.18em] text-slate-500'>
                <th className='sticky left-0 z-10 w-[168px] whitespace-nowrap rounded-tl-[24px] border-r border-slate-200 bg-white px-4 py-3 font-medium shadow-[12px_0_24px_-18px_rgba(15,23,42,0.18)]'>
                  {t('usageTableToken')}
                </th>
                <th className='w-[148px] whitespace-nowrap px-4 py-3 font-medium'>{t('usageTableModel')}</th>
                <th className='w-[108px] whitespace-nowrap px-4 py-3 font-medium'>{t('usageTableReasoning')}</th>
                <th className='w-[156px] whitespace-nowrap px-4 py-3 font-medium'>{t('usageTableEndpoint')}</th>
                <th className='w-[128px] whitespace-nowrap px-4 py-3 font-medium'>{t('usageTableType')}</th>
                <th className='w-[156px] whitespace-nowrap px-4 py-3 font-medium'>{t('usageTableSpend')}</th>
                <th className='w-[268px] whitespace-nowrap px-4 py-3 font-medium'>{t('usageTableTokens')}</th>
                <th className='w-[116px] whitespace-nowrap px-4 py-3 font-medium'>{t('usageTableBillingMode')}</th>
                <th className='w-[220px] whitespace-nowrap px-4 py-3 font-medium'>{t('usageTableTiming')}</th>
                <th className='w-[176px] whitespace-nowrap px-4 py-3 font-medium'>{t('usageTableDetails')}</th>
                <th className={hasUserAgentColumn ? 'w-[156px] whitespace-nowrap px-4 py-3 font-medium' : 'w-[156px] whitespace-nowrap rounded-tr-[24px] px-4 py-3 font-medium'}>
                  {t('usageTableTime')}
                </th>
                {hasUserAgentColumn ? (
                  <th className='w-[300px] whitespace-nowrap rounded-tr-[24px] px-4 py-3 font-medium'>{t('usageTableUserAgent')}</th>
                ) : null}
              </tr>
            </thead>
            <tbody>
              {rows.length > 0 ? (
                rows.map((row) => (
                  <tr key={row.id} className='h-[84px] border-b border-slate-100 text-sm text-slate-700'>
                    <td className='sticky left-0 z-10 w-[168px] border-r border-slate-100 bg-white px-4 py-3 align-middle shadow-[12px_0_24px_-18px_rgba(15,23,42,0.18)]'>
                      <div className='w-[132px] overflow-hidden'>
                        <p className='truncate font-medium text-slate-950'>{row.tokenName}</p>
                      </div>
                    </td>
                    <td className='px-4 py-3 align-middle'>
                      <div className='w-[116px] overflow-hidden'>
                        <p className='truncate font-medium text-slate-950'>{row.modelName}</p>
                      </div>
                    </td>
                    <td className='px-4 py-3 align-middle text-slate-600'>{row.reasoningEffort}</td>
                    <td className='px-4 py-3 align-middle text-slate-600'>
                      <div className='w-[124px] overflow-hidden'>
                        <p className='truncate'>{row.endpoint}</p>
                      </div>
                    </td>
                    <td className='px-4 py-3 align-middle'>
                      <div className='space-y-1'>
                        <span className={`inline-flex h-6 items-center rounded-md px-2.5 text-xs font-medium ${getToneClass(row.requestType.tone as 'sky' | 'amber' | 'violet' | 'teal' | 'lime' | 'rose')}`}>
                          {row.requestType.label}
                        </span>
                      </div>
                    </td>
                    <td className='px-4 py-3 align-middle'>
                      <div className='relative space-y-1'>
                        <div className='flex items-center gap-2'>
                          <span className='font-semibold text-emerald-600'>{row.spend}</span>
                          <button
                            type='button'
                            onMouseEnter={(event) => openPopover('cost', row.id, event.currentTarget)}
                            onMouseLeave={scheduleClosePopover}
                            className='inline-flex h-6 w-6 items-center justify-center rounded-full border border-slate-200 bg-white text-slate-400 transition-colors hover:text-slate-700'
                          >
                            <CircleHelp className='h-4 w-4' />
                          </button>
                        </div>
                      </div>
                    </td>
                    <td className='px-4 py-3 align-middle'>
                      {row.totalTokens > 0 || row.cacheReadTokens > 0 ? (
                        <div className='relative space-y-1'>
                          <div className='flex items-center gap-3 text-[15px]'>
                            <span className='text-emerald-500'>↓</span>
                            <span className='font-medium text-slate-950'>{row.inputTokens.toLocaleString()}</span>
                            <span className='text-violet-500'>↑</span>
                            <span className='font-medium text-slate-950'>{row.outputTokens.toLocaleString()}</span>
                            <button
                              type='button'
                              onMouseEnter={(event) => openPopover('tokens', row.id, event.currentTarget)}
                              onMouseLeave={scheduleClosePopover}
                              className='inline-flex h-6 w-6 items-center justify-center rounded-full border border-sky-200 bg-sky-50 text-sky-500 transition-colors hover:text-sky-700'
                            >
                              <CircleHelp className='h-4 w-4' />
                            </button>
                          </div>
                          <div className='flex items-center gap-2 text-[15px] text-sky-600'>
                            <DatabaseZap className='h-4 w-4' />
                            <span className='font-medium'>{formatCompactNumber(row.cacheReadTokens)}</span>
                          </div>
                        </div>
                      ) : (
                        <span className='text-sm text-slate-400'>--</span>
                      )}
                    </td>
                    <td className='px-4 py-3 align-middle'>
                      <span className='inline-flex h-6 items-center rounded-md bg-slate-100 px-2.5 text-xs font-medium text-slate-700'>
                        {row.billingMode}
                      </span>
                    </td>
                    <td className='px-4 py-3 align-middle'>
                      {row.requestType.label === t('usageTypeConsume') || row.requestType.label === t('usageTypeError') ? (
                        <div className='flex flex-wrap items-center gap-2'>
                          <span className='inline-flex h-6 items-center rounded-full bg-emerald-50 px-2.5 text-xs font-medium text-emerald-700'>
                            {row.duration}
                          </span>
                          <span className='inline-flex h-6 items-center rounded-full bg-amber-50 px-2.5 text-xs font-medium text-amber-700'>
                            {row.firstToken}
                          </span>
                          <span className='inline-flex h-6 items-center rounded-full bg-blue-50 px-2.5 text-xs font-medium text-blue-700'>
                            {row.streamType.toLowerCase()}
                          </span>
                        </div>
                      ) : row.requestType.label === t('usageTypeRefund') ? (
                        <div className='flex flex-wrap items-center gap-2'>
                          <span className='inline-flex h-6 items-center rounded-full bg-emerald-50 px-2.5 text-xs font-medium text-emerald-700'>
                            {row.duration}
                          </span>
                          <span className='inline-flex h-6 items-center rounded-full bg-amber-50 px-2.5 text-xs font-medium text-amber-700'>
                            {row.firstToken}
                          </span>
                        </div>
                      ) : (
                        <div className='flex flex-wrap items-center gap-2'>
                          <span className='inline-flex h-6 items-center rounded-full bg-slate-100 px-2.5 text-xs font-medium text-slate-600'>
                            {row.requestType.detail}
                          </span>
                        </div>
                      )}
                    </td>
                    <td className='px-4 py-3 align-middle'>
                      <div className='w-[148px] space-y-1 overflow-hidden'>
                        {row.details.map((line) => (
                          <p key={`${row.id}-${line}`} className='truncate text-xs text-slate-500'>
                            {line}
                          </p>
                        ))}
                      </div>
                    </td>
                    <td className='whitespace-nowrap px-4 py-3 align-middle text-slate-500'>{row.time}</td>
                    {hasUserAgentColumn ? (
                      <td className='px-4 py-3 align-middle'>
                        <div className='w-[260px] overflow-hidden text-slate-500'>
                          <p className='line-clamp-2 break-all'>{row.userAgent || '--'}</p>
                        </div>
                      </td>
                    ) : null}
                  </tr>
                ))
              ) : null}
            </tbody>
          </table>
        </div>

        {rows.length === 0 ? (
          <div className='flex h-[280px] items-center justify-center border-t border-slate-100 bg-white text-center text-sm text-slate-500'>
            {t('usageEmpty')}
          </div>
        ) : null}

        <div className='flex flex-col gap-3 border-t border-slate-200 bg-white px-4 py-3 xl:flex-row xl:items-center xl:justify-between'>
          <div className='min-w-[220px] whitespace-nowrap text-sm text-slate-500'>
            {paginationSummary}
          </div>

          <div className='flex items-center gap-2'>
            <label className='whitespace-nowrap text-sm text-slate-500'>{t('usagePaginationPerPage')}</label>
            <select
              value={pageSize}
              onChange={(event) => {
                setPageSize(Number(event.target.value));
                setPage(1);
              }}
              className='input-shell !h-10 !w-[72px] !rounded-xl !px-3 !py-2 text-sm'
            >
              {[20, 50, 100].map((size) => (
                <option key={size} value={size}>
                  {size}
                </option>
              ))}
            </select>
          </div>

          <div className='flex flex-wrap items-center gap-0 overflow-hidden rounded-xl border border-slate-200'>
            <button
              type='button'
              onClick={() => setPage((current) => Math.max(1, current - 1))}
              disabled={page === 1}
              className='inline-flex h-10 w-10 items-center justify-center border-r border-slate-200 bg-white text-sm text-slate-600 disabled:opacity-40'
            >
              ‹
            </button>
            {Array.from({ length: totalPages }, (_, index) => index + 1)
              .filter((pageNumber) => {
                if (totalPages <= 6) return true;
                return pageNumber === 1 || pageNumber === totalPages || Math.abs(pageNumber - page) <= 1;
              })
              .map((pageNumber, index, visiblePages) => {
                const previous = visiblePages[index - 1];
                const needsDots = previous && pageNumber - previous > 1;

                return (
                  <div key={pageNumber} className='contents'>
                    {needsDots ? (
                      <div className='inline-flex h-10 w-10 items-center justify-center border-r border-slate-200 bg-white text-sm text-slate-500'>
                        ...
                      </div>
                    ) : null}
                    <button
                      type='button'
                      onClick={() => setPage(pageNumber)}
                      className={
                        page === pageNumber
                          ? 'inline-flex h-10 w-10 items-center justify-center border-r border-slate-200 bg-emerald-50 text-sm font-medium text-emerald-700'
                          : 'inline-flex h-10 w-10 items-center justify-center border-r border-slate-200 bg-white text-sm text-slate-600'
                      }
                    >
                      {pageNumber}
                    </button>
                  </div>
                );
              })}
            <button
              type='button'
              onClick={() => setPage((current) => Math.min(totalPages, current + 1))}
              disabled={page === totalPages}
              className='inline-flex h-10 w-10 items-center justify-center bg-white text-sm text-slate-600 disabled:opacity-40'
            >
              ›
            </button>
          </div>
        </div>
      </section>
      {popover && typeof document !== 'undefined'
        ? createPortal(
            <div
              className='fixed z-[120] rounded-[18px] border border-slate-700 bg-slate-900 px-4 py-3.5 text-white shadow-[0_20px_60px_-28px_rgba(15,23,42,0.8)]'
              style={{
                left: popover.style.left,
                top: popover.style.top,
                width: popover.type === 'tokens' ? 272 : 304,
              }}
              onMouseEnter={clearCloseTimer}
              onMouseLeave={scheduleClosePopover}
            >
              {(() => {
                const row = rows.find((item) => item.id === popover.rowId);
                if (!row) return null;

                if (popover.type === 'tokens') {
                  return (
                    <div className='space-y-3'>
                      <h3 className='text-[16px] font-semibold leading-none'>Token Breakdown</h3>
                      <div className='flex items-center justify-between'>
                        <span className='text-[14px] text-slate-300'>{t('usageTokenInput')} Tokens</span>
                        <span className='text-[14px] font-medium text-white'>{row.inputTokens.toLocaleString()}</span>
                      </div>
                      <div className='flex items-center justify-between'>
                        <span className='text-[14px] text-slate-300'>{t('usageTokenOutput')} Tokens</span>
                        <span className='text-[14px] font-medium text-white'>{row.outputTokens.toLocaleString()}</span>
                      </div>
                      <div className='flex items-center justify-between'>
                        <span className='text-[14px] text-slate-300'>Cache Read Tokens</span>
                        <span className='text-[14px] font-medium text-white'>{row.cacheReadTokens.toLocaleString()}</span>
                      </div>
                      <div className='border-t border-slate-700 pt-3'>
                        <div className='flex items-center justify-between'>
                          <span className='text-[15px] text-slate-300'>{t('usageTokenTotal')} Tokens</span>
                          <span className='text-[15px] font-semibold text-sky-400'>{row.totalTokens.toLocaleString()}</span>
                        </div>
                      </div>
                    </div>
                  );
                }

                return row.isPositiveSpend ? (
                  <div className='space-y-3'>
                    <h3 className='text-[16px] font-semibold leading-none'>Cost Breakdown</h3>
                    <div className='flex items-center justify-between'>
                      <span className='text-[14px] text-slate-300'>{t('usageSettlementPreConsumed')}</span>
                      <span className='text-[14px] font-medium text-white'>{formatUsd(-row.costBreakdown.preConsumedUsd)}</span>
                    </div>
                    <div className='flex items-center justify-between'>
                      <span className='text-[14px] text-slate-300'>{t('usageSettlementActual')}</span>
                      <span className='text-[14px] font-medium text-white'>{formatUsd(-row.costBreakdown.actualUsd)}</span>
                    </div>
                    <div className='border-t border-slate-700 pt-3'>
                      <div className='flex items-center justify-between'>
                        <span className='text-[15px] text-slate-300'>{t('usageSettlementRefund')}</span>
                        <span className='text-[15px] font-semibold text-emerald-400'>{formatUsd(row.costBreakdown.deltaUsd)}</span>
                      </div>
                    </div>
                  </div>
                ) : (
                  <div className='space-y-3'>
                    <h3 className='text-[16px] font-semibold leading-none'>Cost Breakdown</h3>
                    <div className='flex items-center justify-between'>
                      <span className='text-[14px] text-slate-300'>Input Cost</span>
                      <span className='text-[14px] font-medium text-white'>{formatUsd(row.costBreakdown.inputUsd).replace('+', '')}</span>
                    </div>
                    <div className='flex items-center justify-between'>
                      <span className='text-[14px] text-slate-300'>Output Cost</span>
                      <span className='text-[14px] font-medium text-white'>{formatUsd(row.costBreakdown.outputUsd).replace('+', '')}</span>
                    </div>
                    <div className='flex items-center justify-between'>
                      <span className='text-[14px] text-slate-300'>Input price</span>
                      <span className='text-[14px] font-medium text-sky-400'>${row.costBreakdown.inputRatio.toFixed(4)} / 1M tokens</span>
                    </div>
                    <div className='flex items-center justify-between'>
                      <span className='text-[14px] text-slate-300'>Output price</span>
                      <span className='text-[14px] font-medium text-violet-400'>${row.costBreakdown.outputRatio.toFixed(4)} / 1M tokens</span>
                    </div>
                    <div className='flex items-center justify-between'>
                      <span className='text-[14px] text-slate-300'>Cache Read Cost</span>
                      <span className='text-[14px] font-medium text-white'>{formatUsd(row.costBreakdown.cacheUsd).replace('+', '')}</span>
                    </div>
                    <div className='border-t border-slate-700 pt-3 text-[13px]'>
                      <div className='flex items-center justify-between'>
                        <span className='text-slate-300'>Original</span>
                        <span className='font-medium text-white'>{formatUsd(row.spendUsd).replace('+', '')}</span>
                      </div>
                    </div>
                    <div className='border-t border-slate-700 pt-3'>
                      <div className='flex items-center justify-between'>
                        <span className='text-[15px] text-slate-300'>Billed</span>
                        <span className='text-[15px] font-semibold text-emerald-400'>{row.spend}</span>
                      </div>
                    </div>
                  </div>
                );
              })()}
            </div>,
            document.body,
          )
        : null}
    </div>
  );
}
