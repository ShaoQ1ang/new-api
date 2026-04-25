import type { LucideIcon } from 'lucide-react';

type MetricCardProps = {
  label: string;
  value: string;
  hint: string;
  icon: LucideIcon;
};

export default function MetricCard({
  label,
  value,
  hint,
  icon: Icon,
}: MetricCardProps) {
  return (
    <article className='panel-card min-h-[128px] p-5'>
      <div className='flex items-start justify-between gap-4'>
        <div className='min-w-0 flex-1'>
          <p className='min-h-[32px] text-xs uppercase tracking-[0.18em] text-slate-500'>
            {label}
          </p>
          <p className='mt-3 whitespace-nowrap text-3xl font-semibold text-slate-950'>{value}</p>
          {hint ? <p className='mt-2 line-clamp-2 text-sm text-slate-600'>{hint}</p> : null}
        </div>
        <div className='icon-chip'>
          <Icon className='h-5 w-5' />
        </div>
      </div>
    </article>
  );
}
