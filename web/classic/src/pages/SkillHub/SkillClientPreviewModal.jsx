/*
Copyright (C) 2025 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/

import React, { useMemo, useState } from 'react';
import {
  ChevronLeft,
  ChevronRight,
  Database,
  Eye,
  Flag,
  Heart,
  Link2,
  LockKeyhole,
  PlusCircle,
  ShieldCheck,
  Sparkles,
} from 'lucide-react';
import { Button, Modal } from '@douyinfe/semi-ui';
import { useTranslation } from 'react-i18next';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';

const noReferrerPolicy = 'no-referrer';
const radarGridColor = '#dce2eb';
const radarValueColor = '#173b78';

const evaluationDimensions = [
  {
    key: 'safety',
    labelKey: 'S · Safety',
    radarLabelKey: 'Safety',
    background: '#edf9f4',
    color: '#42bd86',
    gradient: 'linear-gradient(90deg, #42aa63 0%, #68cf85 100%)',
    icon: ShieldCheck,
  },
  {
    key: 'access',
    labelKey: 'A · Access',
    radarLabelKey: 'Access',
    background: '#eef4ff',
    color: '#4d7ff5',
    gradient: 'linear-gradient(90deg, #3779f6 0%, #74c5ef 100%)',
    icon: LockKeyhole,
  },
  {
    key: 'frontier',
    labelKey: 'F · Frontier',
    radarLabelKey: 'Frontier',
    background: '#fff7ec',
    color: '#ed9f25',
    gradient: 'linear-gradient(90deg, #f49b2d 0%, #f7d438 100%)',
    icon: Sparkles,
  },
  {
    key: 'economy',
    labelKey: 'E · Economy',
    radarLabelKey: 'Economy',
    background: '#f5f0ff',
    color: '#8b5cf6',
    gradient: 'linear-gradient(90deg, #a04ed8 0%, #c881ed 100%)',
    icon: Database,
  },
];

const SkillClientPreviewModal = ({ form, testcases, updatedAt }) => {
  const { t } = useTranslation();
  const [visible, setVisible] = useState(false);

  return (
    <>
      <Button icon={<Eye size={16} />} onClick={() => setVisible(true)}>
        {t('Preview')}
      </Button>
      <Modal
        title={t('Client preview')}
        visible={visible}
        width='min(1160px, calc(100vw - 32px))'
        footer={null}
        bodyStyle={{ padding: 0 }}
        onCancel={() => setVisible(false)}
      >
        <div className='border-b border-[#e4e5e3] bg-white px-5 py-3 text-sm text-[#71717a]'>
          {t('This preview uses current unsaved form values.')}
        </div>
        <div className='max-h-[calc(100vh-150px)] overflow-y-auto bg-[#f7f7f5] px-4 py-6 sm:px-8 lg:px-10 lg:py-8'>
          <ClientSkillDetailPreview
            form={form}
            testcases={testcases}
            updatedAt={updatedAt}
          />
        </div>
      </Modal>
    </>
  );
};

const ClientSkillDetailPreview = ({ form, testcases, updatedAt }) => {
  const { t } = useTranslation();
  const [tab, setTab] = useState('markdown');
  const sourceUrl = safeExternalUrl(form.originUrl);

  return (
    <div className='mx-auto flex min-h-[620px] max-w-[1040px] flex-col pb-5 font-sans text-[#18181b]'>
      <div className='mb-5 flex shrink-0 flex-wrap items-center justify-between gap-3'>
        <div className='flex h-9 items-center gap-1.5 rounded-lg px-2 text-sm font-semibold text-[#52525b]'>
          <ChevronLeft size={16} />
          {t('Back')}
        </div>
        <div className='flex items-center gap-2'>
          <div className='flex h-9 items-center gap-2 rounded-lg px-3 text-sm font-semibold text-[#3f3f46]'>
            <Heart size={16} />
            {t('Favorite')}
          </div>
          <div className='flex h-9 items-center gap-2 rounded-lg bg-[#20211f] px-3 text-sm font-semibold text-white'>
            <PlusCircle size={16} />
            {t('Install skill')}
          </div>
        </div>
      </div>

      <header className='flex items-start gap-4 px-1 pb-5'>
        <SkillPreviewIcon
          key={form.icon}
          icon={form.icon}
          name={form.name || form.id}
        />
        <div className='min-w-0 flex-1'>
          <h1 className='truncate text-2xl font-bold text-[#20211f]'>
            {form.name.trim() || form.id.trim() || t('Skill')}
          </h1>
          <p className='mt-1 max-w-3xl text-sm leading-6 text-[#52525b]'>
            {form.description.trim() || t('No description available.')}
          </p>
        </div>
      </header>

      <div className='flex h-11 gap-7 border-b border-[#dedfdd]'>
        {[
          { value: 'markdown', label: 'SKILL.md' },
          { value: 'evaluation', label: t('Evaluation report') },
          { value: 'effect', label: t('Effect preview') },
        ].map((item) => (
          <button
            key={item.value}
            type='button'
            className={`relative h-11 flex-none px-0 text-sm font-semibold transition-colors ${
              tab === item.value ? 'text-[#18181b]' : 'text-[#71717a]'
            }`}
            onClick={() => setTab(item.value)}
          >
            {item.label}
            {tab === item.value ? (
              <span className='absolute inset-x-0 bottom-0 h-0.5 rounded bg-[#20211f]' />
            ) : null}
          </button>
        ))}
      </div>

      <div className='grid flex-1 gap-6 pt-5 xl:grid-cols-[minmax(0,1fr)_230px]'>
        <main className='min-w-0'>
          {tab === 'markdown' ? (
            <PreviewContentPanel>
              {form.skillMarkdown.trim() ? (
                <SkillMarkdownPreview text={form.skillMarkdown} />
              ) : (
                <EmptyPreview text={t('No SKILL.md content available.')} />
              )}
            </PreviewContentPanel>
          ) : null}
          {tab === 'evaluation' ? (
            <EvaluationPreview evaluation={form.evaluation} />
          ) : null}
          {tab === 'effect' ? (
            <EffectCasesPreview testcases={testcases} />
          ) : null}
        </main>

        <aside className='h-fit overflow-hidden rounded-xl border border-[#dedfdd] bg-white text-sm shadow-[0_10px_30px_rgba(30,32,30,0.04)]'>
          <PreviewInfoRow
            label={t('Updated at')}
            value={updatedAt ? formatPreviewDate(updatedAt) : '-'}
          />
          <PreviewInfoRow
            label={t('Current version')}
            value={form.version.trim() || '-'}
          />
          <PreviewInfoRow
            label={t('License')}
            value={form.license.trim() || '-'}
          />
          {form.origin.trim() || sourceUrl ? (
            <div className='border-t border-[#e7e8e6] px-4 py-3'>
              {sourceUrl ? (
                <a
                  className='flex items-start gap-2 text-xs font-semibold text-[#52525b] hover:text-[#18181b]'
                  href={sourceUrl}
                  rel='noopener noreferrer'
                  target='_blank'
                >
                  <Link2 className='mt-0.5 shrink-0' size={14} />
                  <span className='min-w-0 flex-1 break-words'>
                    {t('Source')} {form.origin.trim() || sourceUrl}
                  </span>
                </a>
              ) : (
                <div className='flex gap-2 text-xs font-semibold text-[#52525b]'>
                  <Link2 className='mt-0.5 shrink-0' size={14} />
                  <span>
                    {t('Source')} {form.origin.trim()}
                  </span>
                </div>
              )}
            </div>
          ) : null}
          <div className='flex items-center gap-2 border-t border-[#e7e8e6] px-4 py-3 text-xs font-semibold text-[#71717a]'>
            <Flag size={14} />
            {t('Report an issue')}
          </div>
        </aside>
      </div>
    </div>
  );
};

const SkillPreviewIcon = ({ icon, name }) => {
  const [failed, setFailed] = useState(false);
  const safeIcon = safeExternalUrl(icon);
  const fallback = name.trim().slice(0, 1).toUpperCase() || 'S';

  return (
    <div className='grid h-16 w-16 shrink-0 place-items-center overflow-hidden rounded-2xl border border-[#d8ddea] bg-[#eef2fa] text-xl font-bold text-[#536df0] shadow-sm'>
      {safeIcon && !failed ? (
        <img
          src={safeIcon}
          alt=''
          className='h-full w-full object-cover'
          draggable={false}
          referrerPolicy={noReferrerPolicy}
          onError={() => setFailed(true)}
        />
      ) : (
        fallback
      )}
    </div>
  );
};

const PreviewContentPanel = ({ children }) => (
  <section className='min-h-[260px] rounded-xl border border-[#e2e3e1] bg-white p-5 shadow-[0_8px_28px_rgba(30,32,30,0.035)]'>
    {children}
  </section>
);

const EmptyPreview = ({ text }) => (
  <div className='grid min-h-[220px] place-items-center text-sm text-[#a1a1aa]'>
    {text}
  </div>
);

const SkillMarkdownPreview = ({ text }) => {
  const markdown = stripSkillMarkdownFrontmatter(text);

  return (
    <div className='min-w-0 break-words text-sm leading-7 text-[#3f3f46]'>
      <ReactMarkdown
        remarkPlugins={[remarkGfm]}
        components={{
          h1: ({ children }) => (
            <h1 className='mb-4 mt-1 text-2xl font-bold text-[#18181b]'>
              {children}
            </h1>
          ),
          h2: ({ children }) => (
            <h2 className='mb-3 mt-6 text-xl font-bold text-[#18181b]'>
              {children}
            </h2>
          ),
          h3: ({ children }) => (
            <h3 className='mb-2 mt-5 text-lg font-bold text-[#18181b]'>
              {children}
            </h3>
          ),
          p: ({ children }) => <p className='my-3'>{children}</p>,
          ul: ({ children }) => (
            <ul className='my-3 list-disc space-y-1 pl-6'>{children}</ul>
          ),
          ol: ({ children }) => (
            <ol className='my-3 list-decimal space-y-1 pl-6'>{children}</ol>
          ),
          blockquote: ({ children }) => (
            <blockquote className='my-4 border-l-4 border-[#d4d4d8] bg-[#fafafa] px-4 py-1 text-[#52525b]'>
              {children}
            </blockquote>
          ),
          pre: ({ children }) => (
            <pre className='my-4 overflow-x-auto rounded-lg bg-[#18181b] p-4 text-xs leading-6 text-[#f4f4f5]'>
              {children}
            </pre>
          ),
          code: ({ children, className }) => (
            <code
              className={
                className ||
                'rounded bg-black/5 px-1.5 py-0.5 font-mono text-[0.9em]'
              }
            >
              {children}
            </code>
          ),
          a: ({ children, href }) => {
            const safeHref = safeExternalUrl(href);
            return safeHref ? (
              <a
                className='font-medium text-[#2563eb] underline'
                href={safeHref}
                rel='noopener noreferrer'
                target='_blank'
              >
                {children}
              </a>
            ) : (
              <span>{children}</span>
            );
          },
          img: ({ alt }) => (alt ? <span>[{alt}]</span> : null),
          table: ({ children }) => (
            <div className='my-4 overflow-x-auto'>
              <table className='w-full border-collapse text-left'>
                {children}
              </table>
            </div>
          ),
          th: ({ children }) => (
            <th className='border border-[#e4e4e7] bg-[#fafafa] px-3 py-2 font-semibold'>
              {children}
            </th>
          ),
          td: ({ children }) => (
            <td className='border border-[#e4e4e7] px-3 py-2'>{children}</td>
          ),
        }}
      >
        {markdown}
      </ReactMarkdown>
    </div>
  );
};

const EvaluationPreview = ({ evaluation }) => {
  const { t } = useTranslation();
  if (!evaluation) {
    return (
      <PreviewContentPanel>
        <EmptyPreview text={t('No evaluation report available.')} />
      </PreviewContentPanel>
    );
  }

  const scores = evaluationDimensions.map((dimension) =>
    previewScore(evaluation.dimensions[dimension.key].score),
  );
  const average = scores.reduce((sum, score) => sum + score, 0) / scores.length;
  const explicitScore = Number(evaluation.overallScore);
  const overallScore =
    evaluation.overallScore.trim() && Number.isFinite(explicitScore)
      ? Math.max(0, Math.min(5, explicitScore))
      : average;
  const overallRating =
    evaluation.overallRating.trim() || previewRating(overallScore, t);

  return (
    <div className='space-y-6'>
      <section className='grid items-center gap-7 rounded-2xl border border-[#e2e5ea] bg-white p-6 shadow-[0_8px_28px_rgba(30,32,30,0.045)] md:grid-cols-[minmax(280px,1fr)_minmax(0,1.2fr)] lg:px-8 lg:py-7'>
        <EvaluationRadar scores={scores} />
        <div className='min-w-0'>
          <div className='flex items-end gap-2'>
            <span className='text-[52px] font-bold leading-none tracking-[-0.035em] text-[#0f172a]'>
              {overallScore.toFixed(1)}
            </span>
            <span className='pb-1 text-xl font-normal text-[#9aa4b8]'>/ 5</span>
          </div>
          <div className='mt-5 inline-flex rounded-full bg-[#e9edf5] px-3 py-1 text-sm font-bold text-[#143a80]'>
            {t('Overall rating')}: {overallRating}
          </div>
          {evaluation.overallReview.trim() ? (
            <p className='mt-5 whitespace-pre-wrap text-[15px] leading-7 text-[#344054]'>
              {evaluation.overallReview}
            </p>
          ) : null}
        </div>
      </section>

      <div>
        <h2 className='mb-4 text-lg font-bold text-[#101828]'>
          {t('Evaluation details')}
        </h2>
        <section className='divide-y divide-[#e8ebef] rounded-2xl border border-[#e2e5ea] bg-white px-6 shadow-[0_8px_28px_rgba(30,32,30,0.035)] lg:px-8'>
          {evaluationDimensions.map((dimension, index) => {
            const DimensionIcon = dimension.icon;
            const review = evaluation.dimensions[dimension.key].review.trim();

            return (
              <div className='py-6' key={dimension.key}>
                <div className='flex items-center gap-4'>
                  <div
                    className='grid h-10 w-10 shrink-0 place-items-center rounded-full'
                    style={{
                      backgroundColor: dimension.background,
                      color: dimension.color,
                    }}
                  >
                    <DimensionIcon size={20} strokeWidth={2} />
                  </div>
                  <span className='text-[15px] font-bold text-[#101828]'>
                    {t(dimension.labelKey)}
                  </span>
                </div>
                <div className='mt-5 flex items-center gap-5'>
                  <div className='h-2 flex-1 overflow-hidden rounded-full bg-[#f0f2f5]'>
                    <div
                      className='h-full rounded-full'
                      style={{
                        background: dimension.gradient,
                        width: `${scores[index] * 20}%`,
                      }}
                    />
                  </div>
                  <div className='min-w-[54px] text-right text-sm font-bold text-[#344054]'>
                    {scores[index].toFixed(1)}
                    <span className='font-normal text-[#98a2b3]'> / 5</span>
                  </div>
                </div>
                {review ? (
                  <p className='mt-4 whitespace-pre-wrap text-sm leading-6 text-[#475467]'>
                    {review}
                  </p>
                ) : null}
              </div>
            );
          })}
        </section>
      </div>
    </div>
  );
};

const EvaluationRadar = ({ scores }) => {
  const { t } = useTranslation();
  const centerX = 230;
  const centerY = 142;
  const radius = 112;
  const labels = [
    { x: centerX, y: 18, anchor: 'middle' },
    { x: 450, y: 146, anchor: 'end' },
    { x: centerX, y: 280, anchor: 'middle' },
    { x: 10, y: 146, anchor: 'start' },
  ];
  const point = (index, scale) => {
    const angle =
      (-90 + index * (360 / evaluationDimensions.length)) * (Math.PI / 180);
    return {
      x: centerX + Math.cos(angle) * radius * scale,
      y: centerY + Math.sin(angle) * radius * scale,
    };
  };
  const polygon = (scale) =>
    evaluationDimensions
      .map((_, index) => {
        const value = point(index, scale);
        return `${value.x},${value.y}`;
      })
      .join(' ');
  const valuePolygon = evaluationDimensions
    .map((_, index) => {
      const value = point(index, scores[index] / 5);
      return `${value.x},${value.y}`;
    })
    .join(' ');

  return (
    <svg
      aria-label={t('Evaluation report')}
      className='mx-auto h-auto w-full max-w-[460px] overflow-visible'
      role='img'
      viewBox='0 0 460 284'
    >
      {[1, 2, 3, 4, 5].map((level) => (
        <polygon
          fill='none'
          key={level}
          points={polygon(level / 5)}
          stroke={radarGridColor}
          strokeWidth='1'
        />
      ))}
      {evaluationDimensions.map((dimension, index) => {
        const outer = point(index, 1);
        const label = labels[index];
        return (
          <g key={dimension.key}>
            <line
              stroke={radarGridColor}
              x1={centerX}
              x2={outer.x}
              y1={centerY}
              y2={outer.y}
            />
            <text
              fill='#344054'
              fontSize='13'
              fontWeight='500'
              textAnchor={label.anchor}
              x={label.x}
              y={label.y}
            >
              {t(dimension.radarLabelKey)}
            </text>
          </g>
        );
      })}
      <polygon
        fill='rgba(42, 75, 135, 0.16)'
        points={valuePolygon}
        stroke={radarValueColor}
        strokeWidth='2.5'
      />
    </svg>
  );
};

const EffectCasesPreview = ({ testcases }) => {
  const { t } = useTranslation();
  const [caseIndex, setCaseIndex] = useState(0);
  const cases = useMemo(
    () =>
      [...(testcases?.testcases || [])].sort(
        (left, right) => left.sortOrder - right.sortOrder,
      ),
    [testcases],
  );
  const activeIndex = Math.min(caseIndex, Math.max(0, cases.length - 1));
  const activeCase = cases[activeIndex];

  if (!activeCase) {
    return (
      <PreviewContentPanel>
        <EmptyPreview text={t('No effect preview cases available.')} />
      </PreviewContentPanel>
    );
  }

  return (
    <section className='overflow-hidden rounded-xl border border-[#e2e3e1] bg-[#fffaf3] shadow-[0_8px_28px_rgba(30,32,30,0.035)]'>
      <div className='flex flex-wrap items-center justify-between gap-4 border-b border-[#ebe4da] bg-white px-5 py-3'>
        <div className='flex min-w-0 items-center gap-3'>
          <div className='grid h-8 w-8 shrink-0 place-items-center rounded-full bg-pink-50 text-pink-500'>
            <Sparkles size={16} />
          </div>
          <span className='shrink-0 rounded-full bg-blue-50 px-2 py-1 text-xs font-semibold text-blue-600'>
            {t('Use case {{index}}', { index: activeIndex + 1 })}
          </span>
          <span className='truncate text-sm font-bold text-[#18181b]'>
            {activeCase.question}
          </span>
        </div>
        <div className='flex shrink-0 items-center gap-2'>
          <button
            type='button'
            className='grid h-8 w-8 place-items-center rounded-lg text-[#52525b] hover:bg-black/5 disabled:cursor-not-allowed disabled:opacity-30'
            aria-label={t('Previous case')}
            disabled={activeIndex === 0}
            onClick={() => setCaseIndex((current) => Math.max(0, current - 1))}
          >
            <ChevronLeft size={16} />
          </button>
          <span className='w-12 text-center text-xs text-[#a1a1aa]'>
            {activeIndex + 1} / {cases.length}
          </span>
          <button
            type='button'
            className='grid h-8 w-8 place-items-center rounded-lg text-[#52525b] hover:bg-black/5 disabled:cursor-not-allowed disabled:opacity-30'
            aria-label={t('Next case')}
            disabled={activeIndex >= cases.length - 1}
            onClick={() =>
              setCaseIndex((current) => Math.min(cases.length - 1, current + 1))
            }
          >
            <ChevronRight size={16} />
          </button>
        </div>
      </div>
      <div className='p-5 md:p-7'>
        <div className='ml-auto max-w-[76%] rounded-2xl border border-[#e1e2e0] bg-white px-4 py-3 text-sm text-[#27272a] shadow-sm'>
          {activeCase.question}
        </div>
        <div className='mt-5 flex items-start gap-3'>
          <div className='mt-1 grid h-8 w-8 shrink-0 place-items-center rounded-full bg-white text-orange-500 shadow-sm'>
            <Sparkles size={16} />
          </div>
          <div className='min-w-0 flex-1 rounded-2xl border border-[#f0dfc5] bg-[#fffdf9] p-5'>
            <SkillMarkdownPreview text={activeCase.answer} />
          </div>
        </div>
      </div>
    </section>
  );
};

const PreviewInfoRow = ({ label, value }) => (
  <div className='border-b border-[#e7e8e6] px-4 py-3'>
    <div className='text-[10px] font-semibold text-[#a1a1aa]'>{label}</div>
    <div className='mt-1 break-words text-xs font-bold text-[#27272a]'>
      {value}
    </div>
  </div>
);

const previewScore = (value) => {
  const score = Number(value);
  if (!Number.isFinite(score)) return 0;
  return Math.max(0, Math.min(5, score));
};

const previewRating = (score, translate) => {
  if (score >= 4.5) return translate('Excellent');
  if (score >= 4) return translate('Very good');
  if (score >= 3) return translate('Good');
  if (score >= 2) return translate('Fair');
  return translate('Needs improvement');
};

const safeExternalUrl = (value) => {
  if (!value) return undefined;
  try {
    const url = new URL(value);
    if (
      (url.protocol === 'http:' || url.protocol === 'https:') &&
      !url.username &&
      !url.password
    ) {
      return url.toString();
    }
  } catch {
    return undefined;
  }
  return undefined;
};

const stripSkillMarkdownFrontmatter = (markdown) => {
  const content = markdown.startsWith('\uFEFF') ? markdown.slice(1) : markdown;
  const lines = content.split(/\r?\n/);
  if (lines[0]?.trim() !== '---') return content;

  for (let index = 1; index < lines.length; index += 1) {
    const line = lines[index].trim();
    if (line === '---' || line === '...') {
      return lines
        .slice(index + 1)
        .join('\n')
        .replace(/^\s*\n/, '');
    }
  }
  return content;
};

const formatPreviewDate = (value) => {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  const year = date.getFullYear();
  const month = String(date.getMonth() + 1).padStart(2, '0');
  const day = String(date.getDate()).padStart(2, '0');
  return `${year}-${month}-${day}`;
};

export default SkillClientPreviewModal;
