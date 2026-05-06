type UnifiedPaginationProps = {
  page: number;
  pageSize: number;
  totalItems: number;
  totalPages: number;
  summaryTemplate: string;
  pageSizeLabel: string;
  pageSizeOptions: number[];
  onPageChange: (page: number) => void;
  onPageSizeChange: (pageSize: number) => void;
};

function clampPage(page: number, totalPages: number) {
  if (totalPages <= 0) return 1;
  return Math.min(Math.max(1, page), totalPages);
}

function formatSummary(template: string, start: number, end: number, total: number) {
  return template
    .replace('{start}', String(start))
    .replace('{end}', String(end))
    .replace('{total}', String(total));
}

export default function UnifiedPagination({
  page,
  pageSize,
  totalItems,
  totalPages,
  summaryTemplate,
  pageSizeLabel,
  pageSizeOptions,
  onPageChange,
  onPageSizeChange,
}: UnifiedPaginationProps) {
  const safeTotalPages = Math.max(1, totalPages || 1);
  const currentPage = clampPage(page, safeTotalPages);
  const start = totalItems === 0 ? 0 : (currentPage - 1) * pageSize + 1;
  const end = totalItems === 0 ? 0 : Math.min(totalItems, currentPage * pageSize);
  const summary = formatSummary(summaryTemplate, start, end, totalItems);

  const buttonClass =
    'inline-flex h-10 min-w-[44px] items-center justify-center rounded-xl border border-slate-200 bg-white px-3 text-sm font-medium text-slate-700 transition hover:border-slate-300 hover:bg-slate-50 disabled:cursor-not-allowed disabled:opacity-45';

  return (
    <div className='flex flex-col gap-3 border-t border-slate-200 bg-slate-50/80 px-4 py-4 lg:flex-row lg:items-center lg:justify-between'>
      <div className='flex flex-col gap-3 sm:flex-row sm:items-center'>
        <p className='min-w-[150px] whitespace-nowrap text-sm text-slate-500'>{summary}</p>
        <label className='inline-flex min-w-[160px] items-center gap-2 whitespace-nowrap text-sm text-slate-500'>
          <span>{pageSizeLabel}</span>
          <select
            value={pageSize}
            onChange={(event) => onPageSizeChange(Number(event.target.value))}
            className='h-10 min-w-[88px] rounded-xl border border-slate-200 bg-white px-3 text-sm font-medium text-slate-700 outline-none transition focus:border-slate-400'
          >
            {pageSizeOptions.map((option) => (
              <option key={option} value={option}>
                {option}
              </option>
            ))}
          </select>
        </label>
      </div>

      <div className='flex items-center justify-between gap-3 sm:justify-end'>
        <p className='min-w-[84px] whitespace-nowrap text-sm text-slate-500'>
          {currentPage} / {safeTotalPages}
        </p>
        <div className='flex items-center gap-2'>
          <button
            type='button'
            className={buttonClass}
            onClick={() => onPageChange(currentPage - 1)}
            disabled={currentPage <= 1}
          >
            Prev
          </button>
          <button
            type='button'
            className={buttonClass}
            onClick={() => onPageChange(currentPage + 1)}
            disabled={currentPage >= safeTotalPages}
          >
            Next
          </button>
        </div>
      </div>
    </div>
  );
}
