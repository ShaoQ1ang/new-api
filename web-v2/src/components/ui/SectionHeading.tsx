type SectionHeadingProps = {
  eyebrow?: string;
  title: string;
  description?: string;
  align?: 'left' | 'center';
};

export default function SectionHeading({
  eyebrow,
  title,
  description,
  align = 'left',
}: SectionHeadingProps) {
  return (
    <div className={align === 'center' ? 'text-center' : ''}>
      {eyebrow ? <p className='eyebrow'>{eyebrow}</p> : null}
      <h2 className='section-title'>{title}</h2>
      {description ? <p className='section-description'>{description}</p> : null}
    </div>
  );
}
