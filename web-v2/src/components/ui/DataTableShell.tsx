import { MoreHorizontal, Search } from 'lucide-react';

type Column = {
  key: string;
  label: string;
};

type DataTableShellProps = {
  title: string;
  description: string;
  actionLabel: string;
  columns: Column[];
  rows: Array<Record<string, string>>;
};

export default function DataTableShell({
  title,
  description,
  actionLabel,
  columns,
  rows,
}: DataTableShellProps) {
  return (
    <section className='space-y-6'>
      <div className='flex flex-col gap-4 lg:flex-row lg:items-end lg:justify-between'>
        <div>
          <p className='eyebrow'>Operations</p>
          <h1 className='page-title'>{title}</h1>
          <p className='page-description'>{description}</p>
        </div>
        <button className='primary-button'>{actionLabel}</button>
      </div>

      <div className='panel-card overflow-hidden'>
        <div className='flex flex-col gap-4 border-b border-slate-200 px-5 py-4 lg:flex-row lg:items-center lg:justify-between'>
          <div className='inline-flex items-center gap-2 rounded-full border border-slate-200 bg-white px-4 py-2 text-sm text-slate-500'>
            <Search className='h-4 w-4' />
            Search, filter, and operate
          </div>
          <div className='flex gap-2 text-sm text-slate-500'>
            <span className='rounded-full bg-slate-100 px-3 py-1'>All</span>
            <span className='rounded-full bg-slate-100 px-3 py-1'>Active</span>
            <span className='rounded-full bg-slate-100 px-3 py-1'>Draft</span>
          </div>
        </div>
        <div className='overflow-x-auto'>
          <table className='min-w-full'>
            <thead>
              <tr className='border-b border-slate-200 bg-slate-50/80 text-left text-xs uppercase tracking-[0.2em] text-slate-500'>
                {columns.map((column) => (
                  <th key={column.key} className='px-5 py-4 font-medium'>
                    {column.label}
                  </th>
                ))}
                <th className='px-5 py-4' />
              </tr>
            </thead>
            <tbody>
              {rows.map((row, rowIndex) => (
                <tr
                  key={`${rowIndex}-${row[columns[0].key]}`}
                  className='border-b border-slate-100 text-sm text-slate-700 transition-colors hover:bg-slate-50/80'
                >
                  {columns.map((column) => (
                    <td key={column.key} className='px-5 py-4'>
                      {column.key === 'status' ? (
                        <span className='inline-flex rounded-full bg-emerald-50 px-3 py-1 text-xs font-medium text-emerald-700'>
                          {row[column.key]}
                        </span>
                      ) : (
                        row[column.key]
                      )}
                    </td>
                  ))}
                  <td className='px-5 py-4 text-right'>
                    <button className='rounded-full p-2 text-slate-400 transition-colors hover:bg-slate-100 hover:text-slate-700'>
                      <MoreHorizontal className='h-4 w-4' />
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>
    </section>
  );
}
