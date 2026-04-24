type StatePanelProps = {
  loading?: boolean;
  error?: string | null;
  empty?: boolean;
  title: string;
  description: string;
};

export default function StatePanel({
  loading,
  error,
  empty,
  title,
  description,
}: StatePanelProps) {
  const label = loading
    ? 'Loading'
    : error
      ? 'Error'
      : empty
        ? 'Empty'
        : null;

  if (!label) {
    return null;
  }

  return (
    <div className='panel-card grid min-h-[220px] place-items-center p-8 text-center'>
      <div className='max-w-md space-y-3'>
        <p className='eyebrow'>{label}</p>
        <h3 className='text-2xl font-semibold text-slate-950'>{title}</h3>
        <p className='text-sm leading-7 text-slate-600'>
          {error || description}
        </p>
      </div>
    </div>
  );
}
