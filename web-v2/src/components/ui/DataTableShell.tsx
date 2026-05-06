import type { ComponentPropsWithoutRef, ReactNode, Ref } from 'react';

type RootProps = {
  children: ReactNode;
  className?: string;
};

type HeaderProps = {
  eyebrow?: ReactNode;
  title?: ReactNode;
  actions?: ReactNode;
  className?: string;
};

type ViewportProps = {
  children: ReactNode;
  className?: string;
  scrollRef?: Ref<HTMLDivElement>;
  onScroll?: ComponentPropsWithoutRef<'div'>['onScroll'];
};

type TableProps = ComponentPropsWithoutRef<'table'>;

type EmptyRowProps = {
  colSpan: number;
  children: ReactNode;
  className?: string;
};

type ScrollbarProps = {
  max: number;
  value: number;
  onChange: (value: number) => void;
  className?: string;
};

type FooterProps = {
  children: ReactNode;
  className?: string;
};

function Root({ children, className }: RootProps) {
  return (
    <section
      className={`overflow-hidden rounded-[28px] border border-slate-200 bg-white shadow-sm ${className || ''}`.trim()}
    >
      {children}
    </section>
  );
}

function Header({ eyebrow, title, actions, className }: HeaderProps) {
  return (
    <div
      className={`flex items-center justify-between border-b border-slate-200 px-5 py-4 ${className || ''}`.trim()}
    >
      <div className='min-h-[56px] max-w-[320px]'>
        {eyebrow ? (
          <p className='text-[11px] font-semibold uppercase tracking-[0.2em] text-slate-400'>
            {eyebrow}
          </p>
        ) : null}
        {title ? (
          <h2 className={`${eyebrow ? 'mt-2' : ''} min-h-[28px] text-xl font-semibold text-slate-950`.trim()}>
            {title}
          </h2>
        ) : null}
      </div>
      {actions ? <div className='flex items-center gap-3'>{actions}</div> : null}
    </div>
  );
}

function Viewport({ children, className, scrollRef, onScroll }: ViewportProps) {
  return (
    <div
      ref={scrollRef}
      onScroll={onScroll}
      className={`no-scrollbar overflow-x-auto ${className || ''}`.trim()}
    >
      {children}
    </div>
  );
}

function Table({ children, className, ...props }: TableProps) {
  return (
    <table className={className} {...props}>
      {children}
    </table>
  );
}

function EmptyRow({ colSpan, children, className }: EmptyRowProps) {
  return (
    <tr>
      <td colSpan={colSpan} className={`px-5 py-10 text-center text-sm text-slate-500 ${className || ''}`.trim()}>
        {children}
      </td>
    </tr>
  );
}

function Scrollbar({ max, value, onChange, className }: ScrollbarProps) {
  if (max <= 0) return null;

  return (
    <div className={`border-t border-slate-200 bg-slate-50/70 px-4 py-3 ${className || ''}`.trim()}>
      <input
        type='range'
        min={0}
        max={Math.max(1, max)}
        value={Math.min(value, Math.max(1, max))}
        onChange={(event) => onChange(Number(event.target.value))}
        className='token-scrollbar w-full'
      />
    </div>
  );
}

function Footer({ children, className }: FooterProps) {
  return <div className={className}>{children}</div>;
}

const DataTableShell = Object.assign(Root, {
  Header,
  Viewport,
  Table,
  EmptyRow,
  Scrollbar,
  Footer,
});

export default DataTableShell;
