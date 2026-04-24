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
    <article className='panel-card p-5'>
      <div className='flex items-start justify-between gap-4'>
        <div>
          <p className='text-xs uppercase tracking-[0.24em] text-slate-500'>
            {label}
          </p>
          <p className='mt-3 text-3xl font-semibold text-slate-950'>{value}</p>
          <p className='mt-2 text-sm text-slate-600'>{hint}</p>
        </div>
        <div className='icon-chip'>
          <Icon className='h-5 w-5' />
        </div>
      </div>
    </article>
  );
}
