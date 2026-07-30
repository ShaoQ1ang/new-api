/*
Copyright (C) 2023-2026 QuantumNous

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
} from 'lucide-react'
import { useMemo, useState, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from '@/components/ui/dialog'
import { Markdown } from '@/components/ui/markdown'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'

import type {
  SkillHubEvaluationForm,
  SkillHubForm,
  SkillHubTestcases,
} from './types'

type SkillClientPreviewDialogProps = {
  form: SkillHubForm
  testcases: SkillHubTestcases | null
  updatedAt?: string
}

type EvaluationDimensionKey = keyof SkillHubEvaluationForm['dimensions']

const EVALUATION_DIMENSIONS: Array<{
  key: EvaluationDimensionKey
  labelKey: string
  radarLabelKey: string
  background: string
  color: string
  gradient: string
  icon: typeof ShieldCheck
}> = [
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
]

export function SkillClientPreviewDialog(props: SkillClientPreviewDialogProps) {
  const { t } = useTranslation()

  return (
    <Dialog>
      <DialogTrigger render={<Button type='button' variant='outline' />}>
        <Eye className='h-4 w-4' />
        {t('Preview')}
      </DialogTrigger>
      <DialogContent className='flex h-[min(920px,calc(100vh-2rem))] max-h-[calc(100vh-2rem)] flex-col gap-0 overflow-hidden bg-white p-0 sm:max-w-[1120px]'>
        <DialogHeader className='shrink-0 border-b border-[#e4e5e3] px-5 py-4 pr-14 text-left'>
          <DialogTitle>{t('Client preview')}</DialogTitle>
          <DialogDescription>
            {t('This preview uses current unsaved form values.')}
          </DialogDescription>
        </DialogHeader>
        <div className='min-h-0 flex-1 overflow-y-auto bg-[#f7f7f5] px-4 py-6 sm:px-8 lg:px-10 lg:py-8'>
          <ClientSkillDetailPreview
            form={props.form}
            testcases={props.testcases}
            updatedAt={props.updatedAt}
          />
        </div>
      </DialogContent>
    </Dialog>
  )
}

function ClientSkillDetailPreview(props: SkillClientPreviewDialogProps) {
  const { t } = useTranslation()
  const sourceUrl = safeExternalUrl(props.form.originUrl)

  return (
    <div className='mx-auto flex min-h-full max-w-[1040px] flex-col pb-5 font-sans text-zinc-950'>
      <div className='mb-5 flex shrink-0 flex-wrap items-center justify-between gap-3'>
        <div className='flex h-9 items-center gap-1.5 rounded-lg px-2 text-sm font-semibold text-zinc-600'>
          <ChevronLeft className='h-4 w-4' />
          {t('Back')}
        </div>
        <div className='flex items-center gap-2'>
          <div className='flex h-9 items-center gap-2 rounded-lg px-3 text-sm font-semibold text-zinc-700'>
            <Heart className='h-4 w-4' />
            {t('Favorite')}
          </div>
          <div className='flex h-9 items-center gap-2 rounded-lg bg-[#20211f] px-3 text-sm font-semibold text-white'>
            <PlusCircle className='h-4 w-4' />
            {t('Install skill')}
          </div>
        </div>
      </div>

      <header className='flex items-start gap-4 px-1 pb-5'>
        <SkillPreviewIcon
          key={props.form.icon}
          icon={props.form.icon}
          name={props.form.name || props.form.id}
        />
        <div className='min-w-0 flex-1'>
          <h1 className='truncate text-2xl font-bold text-[#20211f]'>
            {props.form.name.trim() || props.form.id.trim() || t('New skill')}
          </h1>
          <p className='mt-1 max-w-3xl text-sm leading-6 text-zinc-600'>
            {props.form.description.trim() || t('No description available.')}
          </p>
        </div>
      </header>

      <Tabs defaultValue='markdown' className='gap-0'>
        <div className='border-b border-[#dedfdd]'>
          <TabsList variant='line' className='h-11 gap-7 rounded-none p-0'>
            <TabsTrigger
              value='markdown'
              className='h-11 flex-none rounded-none px-0 text-sm font-semibold text-zinc-500 data-active:text-zinc-950'
            >
              SKILL.md
            </TabsTrigger>
            <TabsTrigger
              value='evaluation'
              className='h-11 flex-none rounded-none px-0 text-sm font-semibold text-zinc-500 data-active:text-zinc-950'
            >
              {t('Evaluation report')}
            </TabsTrigger>
            <TabsTrigger
              value='effect'
              className='h-11 flex-none rounded-none px-0 text-sm font-semibold text-zinc-500 data-active:text-zinc-950'
            >
              {t('Effect preview')}
            </TabsTrigger>
          </TabsList>
        </div>

        <div className='grid flex-1 gap-6 pt-5 xl:grid-cols-[minmax(0,1fr)_230px]'>
          <main className='min-w-0'>
            <TabsContent value='markdown'>
              <PreviewContentPanel>
                {props.form.skillMarkdown.trim() ? (
                  <SkillMarkdownPreview text={props.form.skillMarkdown} />
                ) : (
                  <EmptyPreview text={t('No SKILL.md content available.')} />
                )}
              </PreviewContentPanel>
            </TabsContent>
            <TabsContent value='evaluation'>
              <EvaluationPreview evaluation={props.form.evaluation} />
            </TabsContent>
            <TabsContent value='effect'>
              <EffectCasesPreview testcases={props.testcases} />
            </TabsContent>
          </main>

          <aside className='h-fit overflow-hidden rounded-xl border border-[#dedfdd] bg-white text-sm shadow-[0_10px_30px_rgba(30,32,30,0.04)]'>
            <PreviewInfoRow
              label={t('Updated at')}
              value={props.updatedAt ? formatPreviewDate(props.updatedAt) : '-'}
            />
            <PreviewInfoRow
              label={t('Current version')}
              value={props.form.version.trim() || '-'}
            />
            <PreviewInfoRow
              label={t('License')}
              value={props.form.license.trim() || '-'}
            />
            {(props.form.origin.trim() || sourceUrl) && (
              <div className='border-t border-[#e7e8e6] px-4 py-3'>
                {sourceUrl ? (
                  <a
                    className='flex items-start gap-2 text-xs font-semibold text-zinc-600 hover:text-zinc-950'
                    href={sourceUrl}
                    rel='noopener noreferrer'
                    target='_blank'
                  >
                    <Link2 className='mt-0.5 h-3.5 w-3.5 shrink-0' />
                    <span className='min-w-0 flex-1 break-words'>
                      {t('Source')} {props.form.origin.trim() || sourceUrl}
                    </span>
                  </a>
                ) : (
                  <div className='flex gap-2 text-xs font-semibold text-zinc-600'>
                    <Link2 className='mt-0.5 h-3.5 w-3.5 shrink-0' />
                    <span>
                      {t('Source')} {props.form.origin.trim()}
                    </span>
                  </div>
                )}
              </div>
            )}
            <div className='flex items-center gap-2 border-t border-[#e7e8e6] px-4 py-3 text-xs font-semibold text-zinc-500'>
              <Flag className='h-3.5 w-3.5' />
              {t('Report an issue')}
            </div>
          </aside>
        </div>
      </Tabs>
    </div>
  )
}

function SkillPreviewIcon(props: { icon: string; name: string }) {
  const [failed, setFailed] = useState(false)
  const safeIcon = safeExternalUrl(props.icon)
  const fallback = props.name.trim().slice(0, 1).toUpperCase() || 'S'

  return (
    <div className='grid h-16 w-16 shrink-0 place-items-center overflow-hidden rounded-2xl border border-[#d8ddea] bg-[#eef2fa] text-xl font-bold text-[#536df0] shadow-sm'>
      {safeIcon && !failed ? (
        <img
          src={safeIcon}
          alt=''
          className='h-full w-full object-cover'
          draggable={false}
          referrerPolicy='no-referrer'
          onError={() => setFailed(true)}
        />
      ) : (
        fallback
      )}
    </div>
  )
}

function PreviewContentPanel(props: { children: ReactNode }) {
  return (
    <section className='min-h-[260px] rounded-xl border border-[#e2e3e1] bg-white p-5 shadow-[0_8px_28px_rgba(30,32,30,0.035)]'>
      {props.children}
    </section>
  )
}

function EmptyPreview(props: { text: string }) {
  return (
    <div className='grid min-h-[220px] place-items-center text-sm text-zinc-400'>
      {props.text}
    </div>
  )
}

function SkillMarkdownPreview(props: { text: string }) {
  const markdown = blockMarkdownImages(
    stripSkillMarkdownFrontmatter(props.text)
  )

  return (
    <Markdown className='min-w-0 break-words !text-zinc-800 [&_a]:!text-zinc-950 [&_blockquote]:!bg-zinc-50 [&_code]:!bg-black/5 [&_h1]:!text-zinc-950 [&_h2]:!text-zinc-950 [&_h3]:!text-zinc-950 [&_li]:!text-zinc-800 [&_p]:!text-zinc-800 [&_pre]:!bg-zinc-950 [&_pre_code]:!bg-transparent [&_pre_code]:!text-zinc-100'>
      {markdown}
    </Markdown>
  )
}

function EvaluationPreview(props: {
  evaluation: SkillHubEvaluationForm | null
}) {
  const { t } = useTranslation()
  const evaluation = props.evaluation
  if (!evaluation) {
    return (
      <PreviewContentPanel>
        <EmptyPreview text={t('No evaluation report available.')} />
      </PreviewContentPanel>
    )
  }

  const scores = EVALUATION_DIMENSIONS.map((dimension) =>
    previewScore(evaluation.dimensions[dimension.key].score)
  )
  const average = scores.reduce((sum, score) => sum + score, 0) / scores.length
  const explicitScore = Number(evaluation.overallScore)
  const overallScore =
    evaluation.overallScore.trim() && Number.isFinite(explicitScore)
      ? Math.max(0, Math.min(5, explicitScore))
      : average
  const overallRating =
    evaluation.overallRating.trim() || previewRating(overallScore, t)

  return (
    <div className='space-y-6'>
      <section className='grid items-center gap-7 rounded-2xl border border-[#e2e5ea] bg-white p-6 shadow-[0_8px_28px_rgba(30,32,30,0.045)] md:grid-cols-[minmax(280px,1fr)_minmax(0,1.2fr)] lg:px-8 lg:py-7'>
        <EvaluationRadar scores={scores} />
        <div className='min-w-0'>
          <div className='flex items-end gap-2'>
            <span className='text-[52px] leading-none font-bold tracking-[-0.035em] text-[#0f172a]'>
              {overallScore.toFixed(1)}
            </span>
            <span className='pb-1 text-xl font-normal text-[#9aa4b8]'>/ 5</span>
          </div>
          <div className='mt-5 inline-flex rounded-full bg-[#e9edf5] px-3 py-1 text-sm font-bold text-[#143a80]'>
            {t('Overall rating')}: {overallRating}
          </div>
          {evaluation.overallReview.trim() && (
            <p className='mt-5 text-[15px] leading-7 whitespace-pre-wrap text-[#344054]'>
              {evaluation.overallReview}
            </p>
          )}
        </div>
      </section>

      <div>
        <h2 className='mb-4 text-lg font-bold text-[#101828]'>
          {t('Evaluation details')}
        </h2>
        <section className='divide-y divide-[#e8ebef] rounded-2xl border border-[#e2e5ea] bg-white px-6 shadow-[0_8px_28px_rgba(30,32,30,0.035)] lg:px-8'>
          {EVALUATION_DIMENSIONS.map((dimension, index) => {
            const DimensionIcon = dimension.icon
            const review = evaluation.dimensions[dimension.key].review.trim()

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
                    <DimensionIcon className='h-5 w-5' strokeWidth={2} />
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
                {review && (
                  <p className='mt-4 text-sm leading-6 whitespace-pre-wrap text-[#475467]'>
                    {review}
                  </p>
                )}
              </div>
            )
          })}
        </section>
      </div>
    </div>
  )
}

function EvaluationRadar(props: { scores: number[] }) {
  const { t } = useTranslation()
  const centerX = 230
  const centerY = 142
  const radius = 112
  const labels = [
    { x: centerX, y: 18, anchor: 'middle' },
    { x: 450, y: 146, anchor: 'end' },
    { x: centerX, y: 280, anchor: 'middle' },
    { x: 10, y: 146, anchor: 'start' },
  ] as const
  const point = (index: number, scale: number) => {
    const angle =
      (-90 + index * (360 / EVALUATION_DIMENSIONS.length)) * (Math.PI / 180)
    return {
      x: centerX + Math.cos(angle) * radius * scale,
      y: centerY + Math.sin(angle) * radius * scale,
    }
  }
  const polygon = (scale: number) =>
    EVALUATION_DIMENSIONS.map((_, index) => {
      const value = point(index, scale)
      return `${value.x},${value.y}`
    }).join(' ')
  const valuePolygon = EVALUATION_DIMENSIONS.map((_, index) => {
    const value = point(index, props.scores[index] / 5)
    return `${value.x},${value.y}`
  }).join(' ')

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
          stroke='#dce2eb'
          strokeWidth='1'
        />
      ))}
      {EVALUATION_DIMENSIONS.map((dimension, index) => {
        const outer = point(index, 1)
        const label = labels[index]
        return (
          <g key={dimension.key}>
            <line
              stroke='#dce2eb'
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
        )
      })}
      <polygon
        fill='rgba(42, 75, 135, 0.16)'
        points={valuePolygon}
        stroke='#173b78'
        strokeWidth='2.5'
      />
    </svg>
  )
}

function EffectCasesPreview(props: { testcases: SkillHubTestcases | null }) {
  const { t } = useTranslation()
  const [caseIndex, setCaseIndex] = useState(0)
  const cases = useMemo(
    () =>
      [...(props.testcases?.testcases || [])].sort(
        (left, right) => left.sortOrder - right.sortOrder
      ),
    [props.testcases]
  )
  const activeIndex = Math.min(caseIndex, Math.max(0, cases.length - 1))
  const activeCase = cases[activeIndex]

  if (!activeCase) {
    return (
      <PreviewContentPanel>
        <EmptyPreview text={t('No effect preview cases available.')} />
      </PreviewContentPanel>
    )
  }

  return (
    <section className='overflow-hidden rounded-xl border border-[#e2e3e1] bg-[#fffaf3] shadow-[0_8px_28px_rgba(30,32,30,0.035)]'>
      <div className='flex flex-wrap items-center justify-between gap-4 border-b border-[#ebe4da] bg-white px-5 py-3'>
        <div className='flex min-w-0 items-center gap-3'>
          <div className='grid h-8 w-8 shrink-0 place-items-center rounded-full bg-pink-50 text-pink-500'>
            <Sparkles className='h-4 w-4' />
          </div>
          <span className='shrink-0 rounded-full bg-blue-50 px-2 py-1 text-xs font-semibold text-blue-600'>
            {t('Use case {{index}}', { index: activeIndex + 1 })}
          </span>
          <span className='truncate text-sm font-bold text-zinc-900'>
            {activeCase.question}
          </span>
        </div>
        <div className='flex shrink-0 items-center gap-2'>
          <Button
            type='button'
            size='icon-sm'
            variant='ghost'
            aria-label={t('Previous case')}
            disabled={activeIndex === 0}
            onClick={() => setCaseIndex((current) => Math.max(0, current - 1))}
          >
            <ChevronLeft className='h-4 w-4' />
          </Button>
          <span className='w-12 text-center text-xs text-zinc-400'>
            {activeIndex + 1} / {cases.length}
          </span>
          <Button
            type='button'
            size='icon-sm'
            variant='ghost'
            aria-label={t('Next case')}
            disabled={activeIndex >= cases.length - 1}
            onClick={() =>
              setCaseIndex((current) => Math.min(cases.length - 1, current + 1))
            }
          >
            <ChevronRight className='h-4 w-4' />
          </Button>
        </div>
      </div>
      <div className='p-5 md:p-7'>
        <div className='ml-auto max-w-[76%] rounded-2xl border border-[#e1e2e0] bg-white px-4 py-3 text-sm text-zinc-800 shadow-sm'>
          {activeCase.question}
        </div>
        <div className='mt-5 flex items-start gap-3'>
          <div className='mt-1 grid h-8 w-8 shrink-0 place-items-center rounded-full bg-white text-orange-500 shadow-sm'>
            <Sparkles className='h-4 w-4' />
          </div>
          <div className='min-w-0 flex-1 rounded-2xl border border-[#f0dfc5] bg-[#fffdf9] p-5'>
            <SkillMarkdownPreview text={activeCase.answer} />
          </div>
        </div>
      </div>
    </section>
  )
}

function PreviewInfoRow(props: { label: string; value: string }) {
  return (
    <div className='border-b border-[#e7e8e6] px-4 py-3'>
      <div className='text-[10px] font-semibold text-zinc-400'>
        {props.label}
      </div>
      <div className='mt-1 text-xs font-bold break-words text-zinc-800'>
        {props.value}
      </div>
    </div>
  )
}

function previewScore(value: string) {
  const score = Number(value)
  if (!Number.isFinite(score)) return 0
  return Math.max(0, Math.min(5, score))
}

function previewRating(score: number, translate: (key: string) => string) {
  if (score >= 4.5) return translate('Excellent')
  if (score >= 4) return translate('Very good')
  if (score >= 3) return translate('Good')
  if (score >= 2) return translate('Fair')
  return translate('Needs improvement')
}

function safeExternalUrl(value: string | undefined) {
  if (!value) return undefined
  try {
    const url = new URL(value)
    if (
      (url.protocol === 'http:' || url.protocol === 'https:') &&
      !url.username &&
      !url.password
    ) {
      return url.toString()
    }
  } catch {
    return undefined
  }
  return undefined
}

function stripSkillMarkdownFrontmatter(markdown: string) {
  const content = markdown.startsWith('\uFEFF') ? markdown.slice(1) : markdown
  const lines = content.split(/\r?\n/)
  if (lines[0]?.trim() !== '---') return content

  for (let index = 1; index < lines.length; index += 1) {
    const line = lines[index].trim()
    if (line === '---' || line === '...') {
      return lines
        .slice(index + 1)
        .join('\n')
        .replace(/^\s*\n/, '')
    }
  }
  return content
}

function blockMarkdownImages(markdown: string) {
  return markdown
    .replaceAll(/<img\b[^>]*>/gi, '')
    .replaceAll(
      /!\[([^\]]*)\](?:\((?:\\.|[^)])*\)|\[[^\]]*\])/g,
      (_, alt: string) => (alt ? `[${alt}]` : '')
    )
}

function formatPreviewDate(value: string) {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  const year = date.getFullYear()
  const month = String(date.getMonth() + 1).padStart(2, '0')
  const day = String(date.getDate()).padStart(2, '0')
  return `${year}-${month}-${day}`
}
