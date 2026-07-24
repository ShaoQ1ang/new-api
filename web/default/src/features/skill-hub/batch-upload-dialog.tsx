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
import { useMemo, useRef, useState } from 'react'
import {
  Alert02Icon,
  CheckmarkCircle02Icon,
  Download04Icon,
  FolderUploadIcon,
} from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import {
  Field,
  FieldDescription,
  FieldGroup,
  FieldLabel,
  FieldLegend,
  FieldSet,
} from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import {
  Progress,
  ProgressLabel,
  ProgressValue,
} from '@/components/ui/progress'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group'
import {
  createSkillHubBatchOptions,
  createSkillHubBatchReport,
  issueMessage,
  parseSkillHubBatchDirectory,
  resolveSkillHubBatchSort,
  validateSkillHubBatchOptions,
  type SkillHubBatchDirectory,
  type SkillHubBatchEntry,
  type SkillHubBatchOptions,
} from '../../../../shared/skill-hub-batch-import.mjs'
import {
  commitSkillHubBatchUpload,
  discardSkillHubBatchUploads,
  initSkillHubBatchUpload,
  putSkillHubBatchObject,
} from './api'
import type { SkillHubBatchUploadSpec, SkillHubTag } from './types'

type BatchRowStatus =
  | 'pending'
  | 'validating'
  | 'uploading'
  | 'committing'
  | 'success'
  | 'skipped'
  | 'failed'
  | 'cancelled'
  | 'unknown'

type BatchRowState = {
  index: number
  id: string
  status: BatchRowStatus
  action?: string
  message?: string
  progress: number
}

type ReadyUpload = {
  entry: SkillHubBatchEntry
  zip: SkillHubBatchUploadSpec
  icon?: SkillHubBatchUploadSpec
}

type UploadedItem = ReadyUpload

const commitChunkTargetBytes = 8 * 1024 * 1024

export function SkillHubBatchUploadDialog({
  tags,
  onComplete,
}: {
  tags: SkillHubTag[]
  onComplete: () => void | Promise<void>
}) {
  const { t } = useTranslation()
  const [open, setOpen] = useState(false)
  const [directory, setDirectory] = useState<SkillHubBatchDirectory | null>(
    null
  )
  const [options, setOptions] = useState<SkillHubBatchOptions>(() =>
    createSkillHubBatchOptions()
  )
  const [commonTagsText, setCommonTagsText] = useState('')
  const [rows, setRows] = useState<Record<number, BatchRowState>>({})
  const [working, setWorking] = useState(false)
  const [parsing, setParsing] = useState(false)
  const [startedAt, setStartedAt] = useState('')
  const [finished, setFinished] = useState(false)
  const folderInputRef = useRef<HTMLInputElement | null>(null)
  const abortRef = useRef<AbortController | null>(null)
  const workingRef = useRef(false)

  const entries = directory?.entries || []
  const localErrorCount = entries.reduce(
    (count, entry) => count + entry.errors.length,
    0
  )
  const rowValues = useMemo(
    () => entries.map((entry) => rows[entry.index]).filter(Boolean),
    [entries, rows]
  )
  const terminalCount = rowValues.filter((row) =>
    ['success', 'skipped', 'failed', 'cancelled', 'unknown'].includes(
      row.status
    )
  ).length
  const overallProgress = entries.length
    ? Math.round(
        rowValues.reduce((sum, row) => sum + row.progress, 0) / entries.length
      )
    : 0
  const retryIndexes = rowValues
    .filter((row) => ['failed', 'cancelled'].includes(row.status))
    .map((row) => row.index)
  const highRisk =
    options.published ||
    options.recommended ||
    options.verifiedMode === 'verified' ||
    (options.mode === 'update' &&
      [
        options.missingIcon,
        options.missingTestcases,
        options.missingEvaluation,
      ].includes('clear'))

  function setFolderInput(node: HTMLInputElement | null) {
    folderInputRef.current = node
    node?.setAttribute('webkitdirectory', '')
  }

  async function chooseDirectory(fileList: FileList | null) {
    if (!fileList?.length) return
    setParsing(true)
    try {
      const parsed = await parseSkillHubBatchDirectory(fileList)
      setDirectory(parsed)
      setRows(
        Object.fromEntries(
          parsed.entries.map((entry) => [
            entry.index,
            createRow(entry, entry.errors.length ? 'failed' : 'pending'),
          ])
        )
      )
      setStartedAt('')
      setFinished(false)
    } catch (error) {
      toast.error(
        issueMessage(
          error as Error & {
            code?: string
            params?: Record<string, string | number | boolean>
          },
          t
        )
      )
      setDirectory(null)
      setRows({})
    } finally {
      setParsing(false)
      if (folderInputRef.current) folderInputRef.current.value = ''
    }
  }

  function patchRow(index: number, patch: Partial<BatchRowState>) {
    setRows((current) => ({
      ...current,
      [index]: {
        ...(current[index] || {
          index,
          id: entries.find((entry) => entry.index === index)?.id || '',
          status: 'pending',
          progress: 0,
        }),
        ...patch,
      },
    }))
  }

  async function startUpload(targetIndexes?: number[]) {
    if (!directory || working || workingRef.current || localErrorCount > 0)
      return
    try {
      validateSkillHubBatchOptions(options)
    } catch (error) {
      toast.error(
        issueMessage(
          error as Error & {
            code?: string
            params?: Record<string, string | number | boolean>
          },
          t
        )
      )
      return
    }

    const targets = targetIndexes?.length
      ? entries.filter((entry) => targetIndexes.includes(entry.index))
      : entries
    if (!targets.length) return

    const controller = new AbortController()
    abortRef.current = controller
    const nextStartedAt = new Date().toISOString()
    setStartedAt(nextStartedAt)
    setFinished(false)
    workingRef.current = true
    setWorking(true)
    for (const entry of targets) {
      patchRow(entry.index, {
        status: 'validating',
        action: '',
        message: '',
        progress: 2,
      })
    }

    const cleanupTickets = new Set<string>()
    let successfulCount = 0
    try {
      const initPayload = await initSkillHubBatchUpload({
        mode: options.mode,
        options: commitOptions(options),
        items: targets.map((entry) => ({
          index: entry.index,
          skill: entryToCommitSkill(entry),
          zip: {
            fileName: entry.zipFile?.name || '',
            size: entry.zipFile?.size || 0,
          },
          icon: entry.iconFile
            ? {
                fileName: entry.iconFile.name,
                size: entry.iconFile.size,
              }
            : undefined,
        })),
      })
      if (!initPayload.success || !initPayload.data) {
        throw new Error(
          initPayload.message || t('Failed to initialize batch upload.')
        )
      }

      const ready: ReadyUpload[] = []
      for (const item of initPayload.data.items) {
        const entry = targets.find((target) => target.index === item.index)
        if (!entry) continue
        if (item.status !== 'ready' || !item.zip) {
          patchRow(item.index, {
            status: item.status === 'skipped' ? 'skipped' : 'failed',
            action: item.action,
            message: item.message,
            progress: 100,
          })
          continue
        }
        ready.push({ entry, zip: item.zip, icon: item.icon })
        cleanupTickets.add(item.zip.uploadTicket)
        if (item.icon?.uploadTicket) {
          cleanupTickets.add(item.icon.uploadTicket)
        }
      }

      const uploaded = await uploadReadyItems(
        ready,
        options.concurrency,
        controller,
        options.stopOnError,
        patchRow,
        t
      )
      if (controller.signal.aborted) {
        setRows((current) => {
          const next = { ...current }
          for (const item of ready) {
            const row = current[item.entry.index]
            if (!row || !isTerminal(row.status)) {
              next[item.entry.index] = {
                ...(row || createRow(item.entry, 'pending')),
                status: 'cancelled',
                message: t('Batch upload was cancelled.'),
                progress: 100,
              }
            }
          }
          return next
        })
      }

      if (uploaded.length && !controller.signal.aborted) {
        const commitItems = uploaded.map((item) => ({
          index: item.entry.index,
          skill: entryToCommitSkill(item.entry),
          zipUploadTicket: item.zip.uploadTicket,
          iconUploadTicket: item.icon?.uploadTicket || '',
        }))
        const chunks = splitCommitItems(commitItems)
        for (let chunkIndex = 0; chunkIndex < chunks.length; chunkIndex += 1) {
          if (controller.signal.aborted) {
            for (const item of chunks.slice(chunkIndex).flat()) {
              patchRow(item.index, {
                status: 'cancelled',
                message: t('Batch upload was cancelled.'),
                progress: 100,
              })
            }
            break
          }
          const chunk = chunks[chunkIndex]
          for (const item of chunk) {
            patchRow(item.index, { status: 'committing', progress: 95 })
          }
          try {
            const commitPayload = await commitSkillHubBatchUpload({
              mode: options.mode,
              options: commitOptions(options),
              items: chunk,
            })
            if (!commitPayload.success || !commitPayload.data) {
              const message =
                commitPayload.message || t('Failed to commit batch upload.')
              for (const item of chunks.slice(chunkIndex).flat()) {
                patchRow(item.index, {
                  status: 'failed',
                  message,
                  progress: 100,
                })
              }
              break
            }
            const responseByIndex = new Map(
              commitPayload.data.items.map((item) => [item.index, item])
            )
            for (const item of chunk) {
              const result = responseByIndex.get(item.index)
              if (!result) {
                cleanupTickets.delete(item.zipUploadTicket)
                if (item.iconUploadTicket) {
                  cleanupTickets.delete(item.iconUploadTicket)
                }
                patchRow(item.index, {
                  status: 'unknown',
                  message: t(
                    'Batch commit response is incomplete. Refresh the list before retrying.'
                  ),
                  progress: 100,
                })
                continue
              }
              if (result.status === 'success') {
                cleanupTickets.delete(item.zipUploadTicket)
                if (item.iconUploadTicket) {
                  cleanupTickets.delete(item.iconUploadTicket)
                }
              }
              patchRow(item.index, {
                status:
                  result.status === 'success'
                    ? 'success'
                    : result.status === 'skipped'
                      ? 'skipped'
                      : 'failed',
                action: result.action,
                message: result.message,
                progress: 100,
              })
              if (result.status === 'success') successfulCount += 1
            }
          } catch (error) {
            for (const item of chunk) {
              // The server may have committed the request after the connection
              // was lost. Do not retry or discard these tickets automatically.
              cleanupTickets.delete(item.zipUploadTicket)
              if (item.iconUploadTicket) {
                cleanupTickets.delete(item.iconUploadTicket)
              }
              patchRow(item.index, {
                status: 'unknown',
                message:
                  error instanceof Error
                    ? error.message
                    : t(
                        'Batch commit response was not received. Refresh the list before retrying.'
                      ),
                progress: 100,
              })
            }
            for (const item of chunks.slice(chunkIndex + 1).flat()) {
              patchRow(item.index, {
                status: controller.signal.aborted ? 'cancelled' : 'failed',
                message: controller.signal.aborted
                  ? t('Batch upload was cancelled.')
                  : t('Failed to commit batch upload.'),
                progress: 100,
              })
            }
            break
          }
        }
      }
    } catch (error) {
      const message =
        error instanceof Error
          ? error.message
          : t('Batch upload failed to start.')
      setRows((current) => {
        const next = { ...current }
        for (const entry of targets) {
          const row = current[entry.index]
          if (row && isTerminal(row.status)) continue
          next[entry.index] = {
            ...(row || createRow(entry, 'pending')),
            status: controller.signal.aborted ? 'cancelled' : 'failed',
            message,
            progress: 100,
          }
        }
        return next
      })
      toast.error(message)
    } finally {
      if (cleanupTickets.size) {
        try {
          await discardSkillHubBatchUploads([...cleanupTickets])
        } catch {
          toast.warning(
            t(
              'Some temporary uploads could not be removed immediately and will be cleaned by the OSS lifecycle rule.'
            )
          )
        }
      }
      abortRef.current = null
      workingRef.current = false
      setWorking(false)
      setFinished(true)
      if (successfulCount > 0) {
        try {
          await onComplete()
        } catch {
          toast.warning(t('Failed to load Skill Hub'))
        }
      }
    }
  }

  function cancelUpload() {
    abortRef.current?.abort()
  }

  function downloadReport() {
    if (!directory || !startedAt) return
    const reportItems = entries.map((entry) => {
      const row = rows[entry.index] || createRow(entry, 'pending')
      return {
        index: entry.index,
        id: entry.id,
        status: row.status,
        action: row.action,
        sort: entry.sort,
        uploadSort: resolveSkillHubBatchSort(options, entry.index),
        zipPath: entry.zipPath,
        iconPath: entry.iconPath,
        testcases: entry.testcases
          ? {
              path: entry.testcasesPath,
              slug: entry.testcases.slug,
              count: entry.testcases.testcases.length,
            }
          : undefined,
        error: row.message || '',
      }
    })
    const report = createSkillHubBatchReport({
      directory,
      options,
      items: reportItems,
      startedAt,
    })
    const url = URL.createObjectURL(
      new Blob([`${JSON.stringify(report, null, 2)}\n`], {
        type: 'application/json',
      })
    )
    const link = document.createElement('a')
    link.href = url
    link.download = 'skill-hub-batch-upload-report.json'
    link.click()
    URL.revokeObjectURL(url)
  }

  return (
    <>
      <Button variant='outline' onClick={() => setOpen(true)}>
        <HugeiconsIcon icon={FolderUploadIcon} data-icon='inline-start' />
        {t('Batch upload')}
      </Button>
      <Dialog
        open={open}
        onOpenChange={(nextOpen) => {
          if (!nextOpen && working) return
          setOpen(nextOpen)
        }}
      >
        <DialogContent
          showCloseButton={!working}
          className='flex max-h-[calc(100vh-2rem)] flex-col sm:max-w-6xl'
        >
          <DialogHeader>
            <DialogTitle>{t('Batch upload Skill Hub folder')}</DialogTitle>
            <DialogDescription>
              {t(
                'Select a folder containing manifest.json, packages, optional icons, and optional testcases.'
              )}
            </DialogDescription>
          </DialogHeader>

          <div className='flex min-h-0 flex-1 flex-col gap-4 overflow-y-auto pr-1'>
            <input
              ref={setFolderInput}
              type='file'
              multiple
              hidden
              onChange={(event) => void chooseDirectory(event.target.files)}
            />
            <div className='flex flex-wrap items-center gap-2'>
              <Button
                variant='outline'
                disabled={working || parsing}
                onClick={() => folderInputRef.current?.click()}
              >
                <HugeiconsIcon
                  icon={FolderUploadIcon}
                  data-icon='inline-start'
                />
                {parsing ? t('Validating folder') : t('Select batch folder')}
              </Button>
              {directory && (
                <span className='text-muted-foreground text-sm'>
                  {directory.rootName} ·{' '}
                  {t('{{count}} skills', { count: entries.length })} ·{' '}
                  {t('{{count}} files', { count: directory.fileCount })}
                </span>
              )}
            </div>

            {directory && localErrorCount > 0 && (
              <Alert variant='destructive'>
                <HugeiconsIcon icon={Alert02Icon} />
                <AlertTitle>{t('Folder validation failed')}</AlertTitle>
                <AlertDescription>
                  {t(
                    'Resolve all manifest and file errors before starting the upload.'
                  )}
                </AlertDescription>
              </Alert>
            )}

            {directory && localErrorCount === 0 && (
              <Alert>
                <HugeiconsIcon icon={CheckmarkCircle02Icon} />
                <AlertTitle>{t('Local validation passed')}</AlertTitle>
                <AlertDescription>
                  {t(
                    'The server validates all skill metadata again before issuing upload URLs.'
                  )}
                </AlertDescription>
              </Alert>
            )}

            {directory && (
              <>
                <FieldSet disabled={working}>
                  <FieldLegend>{t('Batch settings')}</FieldLegend>
                  <FieldGroup className='grid gap-4 md:grid-cols-2 xl:grid-cols-3'>
                    <Field>
                      <FieldLabel>{t('Conflict strategy')}</FieldLabel>
                      <OptionSelect
                        value={options.mode}
                        onChange={(value) =>
                          setOptions((current) => ({
                            ...current,
                            mode: value as SkillHubBatchOptions['mode'],
                          }))
                        }
                        options={[
                          ['skip', t('Skip existing')],
                          ['update', t('Update existing')],
                          ['fail', t('Fail existing')],
                        ]}
                      />
                    </Field>
                    <Field>
                      <FieldLabel>{t('Save status')}</FieldLabel>
                      <BooleanToggle
                        value={options.published}
                        falseLabel={t('Save all as draft')}
                        trueLabel={t('Publish all')}
                        onChange={(published) =>
                          setOptions((current) => ({
                            ...current,
                            published,
                          }))
                        }
                      />
                    </Field>
                    <Field>
                      <FieldLabel>{t('Recommendation')}</FieldLabel>
                      <BooleanToggle
                        value={options.recommended}
                        falseLabel={t('Do not recommend any')}
                        trueLabel={t('Recommend all')}
                        onChange={(recommended) =>
                          setOptions((current) => ({
                            ...current,
                            recommended,
                          }))
                        }
                      />
                    </Field>
                    <Field>
                      <FieldLabel>{t('Sort strategy')}</FieldLabel>
                      <OptionSelect
                        value={options.sortMode}
                        onChange={(value) =>
                          setOptions((current) => ({
                            ...current,
                            sortMode: value as SkillHubBatchOptions['sortMode'],
                          }))
                        }
                        options={[
                          ['fixed', t('Force one sort value')],
                          ['sequence', t('Generate sequential sort values')],
                        ]}
                      />
                    </Field>
                    {options.sortMode === 'fixed' ? (
                      <Field>
                        <FieldLabel>{t('Forced sort value')}</FieldLabel>
                        <IntegerInput
                          value={options.fixedSort}
                          onChange={(fixedSort) =>
                            setOptions((current) => ({
                              ...current,
                              fixedSort,
                            }))
                          }
                        />
                      </Field>
                    ) : (
                      <>
                        <Field>
                          <FieldLabel>{t('Sort start')}</FieldLabel>
                          <IntegerInput
                            value={options.sortStart}
                            onChange={(sortStart) =>
                              setOptions((current) => ({
                                ...current,
                                sortStart,
                              }))
                            }
                          />
                        </Field>
                        <Field>
                          <FieldLabel>{t('Sort step')}</FieldLabel>
                          <IntegerInput
                            value={options.sortStep}
                            onChange={(sortStep) =>
                              setOptions((current) => ({
                                ...current,
                                sortStep,
                              }))
                            }
                          />
                        </Field>
                      </>
                    )}
                    <Field>
                      <FieldLabel>{t('Concurrency')}</FieldLabel>
                      <Input
                        type='number'
                        min={1}
                        max={10}
                        value={options.concurrency}
                        onChange={(event) =>
                          setOptions((current) => ({
                            ...current,
                            concurrency: Number(event.target.value),
                          }))
                        }
                      />
                      <FieldDescription>
                        {t('Only OSS PUT uploads run concurrently.')}
                      </FieldDescription>
                    </Field>
                    <Field>
                      <FieldLabel>{t('Error handling')}</FieldLabel>
                      <OptionSelect
                        value={options.stopOnError ? 'stop' : 'continue'}
                        onChange={(value) =>
                          setOptions((current) => ({
                            ...current,
                            stopOnError: value === 'stop',
                          }))
                        }
                        options={[
                          ['continue', t('Continue processing other Skills')],
                          ['stop', t('Stop after the first failure')],
                        ]}
                      />
                    </Field>
                  </FieldGroup>
                </FieldSet>

                <FieldSet disabled={working}>
                  <FieldLegend>{t('Advanced overrides')}</FieldLegend>
                  <FieldGroup className='grid gap-4 md:grid-cols-2 xl:grid-cols-3'>
                    <Field>
                      <FieldLabel>{t('Verified status')}</FieldLabel>
                      <OptionSelect
                        value={options.verifiedMode}
                        onChange={(value) =>
                          setOptions((current) => ({
                            ...current,
                            verifiedMode:
                              value as SkillHubBatchOptions['verifiedMode'],
                          }))
                        }
                        options={[
                          ['manifest', t('Keep manifest value')],
                          ['verified', t('Mark all as verified')],
                          ['unverified', t('Mark all as unverified')],
                        ]}
                      />
                    </Field>
                    <Field>
                      <FieldLabel>{t('Common tag behavior')}</FieldLabel>
                      <OptionSelect
                        value={options.tagMode}
                        onChange={(value) =>
                          setOptions((current) => ({
                            ...current,
                            tagMode: value as SkillHubBatchOptions['tagMode'],
                          }))
                        }
                        options={[
                          ['manifest', t('Keep manifest tags')],
                          ['append', t('Append common tags')],
                          ['replace', t('Replace with common tags')],
                        ]}
                      />
                    </Field>
                    {options.tagMode !== 'manifest' && (
                      <Field>
                        <FieldLabel>{t('Common tags')}</FieldLabel>
                        <Input
                          list='skill-hub-batch-tags'
                          value={commonTagsText}
                          placeholder={t('Separate tags with commas')}
                          onChange={(event) => {
                            setCommonTagsText(event.target.value)
                            setOptions((current) => ({
                              ...current,
                              commonTags: parseCommonTags(event.target.value),
                            }))
                          }}
                        />
                        <datalist id='skill-hub-batch-tags'>
                          {tags.map((tag) => (
                            <option key={tag.id} value={tag.name} />
                          ))}
                        </datalist>
                      </Field>
                    )}
                    <MissingPolicyField
                      label={t('Missing icon on update')}
                      value={options.missingIcon}
                      onChange={(missingIcon) =>
                        setOptions((current) => ({
                          ...current,
                          missingIcon,
                        }))
                      }
                    />
                    <MissingPolicyField
                      label={t('Missing testcases on update')}
                      value={options.missingTestcases}
                      onChange={(missingTestcases) =>
                        setOptions((current) => ({
                          ...current,
                          missingTestcases,
                        }))
                      }
                    />
                    <MissingPolicyField
                      label={t('Missing evaluation on update')}
                      value={options.missingEvaluation}
                      onChange={(missingEvaluation) =>
                        setOptions((current) => ({
                          ...current,
                          missingEvaluation,
                        }))
                      }
                    />
                  </FieldGroup>
                </FieldSet>

                {highRisk && (
                  <Alert>
                    <HugeiconsIcon icon={Alert02Icon} />
                    <AlertTitle>{t('Review high-impact settings')}</AlertTitle>
                    <AlertDescription>
                      {t(
                        'Publishing, recommending, verifying, or clearing existing resources affects every imported skill.'
                      )}
                    </AlertDescription>
                  </Alert>
                )}

                {working || finished ? (
                  <Progress value={overallProgress}>
                    <ProgressLabel>
                      {t('{{done}} of {{total}} completed', {
                        done: terminalCount,
                        total: entries.length,
                      })}
                    </ProgressLabel>
                    <ProgressValue />
                  </Progress>
                ) : null}

                <div className='max-h-80 overflow-y-auto rounded-lg border'>
                  <Table>
                    <TableHeader>
                      <TableRow>
                        <TableHead>#</TableHead>
                        <TableHead>{t('Skill')}</TableHead>
                        <TableHead>{t('Assets')}</TableHead>
                        <TableHead>{t('Sort')}</TableHead>
                        <TableHead>{t('Status')}</TableHead>
                      </TableRow>
                    </TableHeader>
                    <TableBody>
                      {entries.map((entry) => {
                        const row = rows[entry.index]
                        return (
                          <TableRow key={`${entry.index}-${entry.id}`}>
                            <TableCell>{entry.index + 1}</TableCell>
                            <TableCell>
                              <div className='flex max-w-72 flex-col gap-1'>
                                <span className='truncate font-medium'>
                                  {entry.name || entry.id}
                                </span>
                                <span className='text-muted-foreground truncate text-xs'>
                                  {entry.id} · {entry.version}
                                </span>
                                {entry.errors.map((error, errorIndex) => (
                                  <span
                                    key={errorIndex}
                                    className='text-destructive text-xs whitespace-normal'
                                  >
                                    {issueMessage(error, t)}
                                  </span>
                                ))}
                              </div>
                            </TableCell>
                            <TableCell>
                              <span className='text-xs'>
                                ZIP
                                {entry.iconFile ? ' · Icon' : ''}
                                {entry.testcasesFile ? ' · JSON' : ''}
                              </span>
                            </TableCell>
                            <TableCell>
                              <span className='text-xs tabular-nums'>
                                {entry.sort} →{' '}
                                {resolveSkillHubBatchSort(options, entry.index)}
                              </span>
                            </TableCell>
                            <TableCell>
                              <div className='flex max-w-52 flex-col gap-1'>
                                <StatusBadge
                                  status={
                                    row?.status ||
                                    (entry.errors.length ? 'failed' : 'pending')
                                  }
                                  t={t}
                                />
                                {row?.message && (
                                  <span className='text-muted-foreground text-xs whitespace-normal'>
                                    {row.message}
                                  </span>
                                )}
                              </div>
                            </TableCell>
                          </TableRow>
                        )
                      })}
                    </TableBody>
                  </Table>
                </div>
              </>
            )}
          </div>

          <DialogFooter>
            {working ? (
              <Button variant='destructive' onClick={cancelUpload}>
                {t('Cancel batch upload')}
              </Button>
            ) : (
              <Button variant='outline' onClick={() => setOpen(false)}>
                {t('Close')}
              </Button>
            )}
            {finished && startedAt && (
              <Button variant='outline' onClick={downloadReport}>
                <HugeiconsIcon icon={Download04Icon} data-icon='inline-start' />
                {t('Download report')}
              </Button>
            )}
            {finished && retryIndexes.length > 0 && (
              <Button
                variant='outline'
                onClick={() => void startUpload(retryIndexes)}
              >
                {t('Retry failed')}
              </Button>
            )}
            <Button
              disabled={!directory || localErrorCount > 0 || working || parsing}
              onClick={() => void startUpload()}
            >
              <HugeiconsIcon
                icon={CheckmarkCircle02Icon}
                data-icon='inline-start'
              />
              {t('Start batch upload')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  )
}

async function uploadReadyItems(
  ready: ReadyUpload[],
  concurrency: number,
  controller: AbortController,
  stopOnError: boolean,
  patchRow: (index: number, patch: Partial<BatchRowState>) => void,
  t: (key: string, options?: Record<string, unknown>) => string
) {
  const uploaded: UploadedItem[] = []
  let cursor = 0
  let halt = false

  async function worker() {
    while (!halt && !controller.signal.aborted) {
      const currentIndex = cursor
      cursor += 1
      if (currentIndex >= ready.length) return
      const item = ready[currentIndex]
      try {
        patchRow(item.entry.index, {
          status: 'uploading',
          action: '',
          message: '',
          progress: 10,
        })
        await putSkillHubBatchObject(item.zip, item.entry.zipFile!, {
          signal: controller.signal,
          onProgress: (percent) =>
            patchRow(item.entry.index, {
              progress: 10 + Math.round(percent * (item.icon ? 0.55 : 0.8)),
            }),
        })
        if (item.icon && item.entry.iconFile) {
          await putSkillHubBatchObject(item.icon, item.entry.iconFile, {
            signal: controller.signal,
            onProgress: (percent) =>
              patchRow(item.entry.index, {
                progress: 65 + Math.round(percent * 0.25),
              }),
          })
        }
        uploaded.push(item)
        patchRow(item.entry.index, { progress: 90 })
      } catch (error) {
        const aborted =
          controller.signal.aborted ||
          (error instanceof DOMException && error.name === 'AbortError')
        patchRow(item.entry.index, {
          status: aborted ? 'cancelled' : 'failed',
          message:
            error instanceof Error
              ? t(error.message)
              : t('Failed to upload batch item.'),
          progress: 100,
        })
        if (stopOnError && !aborted) {
          halt = true
          controller.abort()
        }
      }
    }
  }

  await Promise.all(
    Array.from(
      { length: Math.min(Math.max(1, concurrency), ready.length || 1) },
      () => worker()
    )
  )
  return uploaded
}

function entryToCommitSkill(entry: SkillHubBatchEntry) {
  return {
    id: entry.id,
    name: entry.name,
    description: entry.description,
    version: entry.version,
    author: entry.author,
    origin: entry.origin,
    originUrl: entry.originUrl,
    license: entry.license,
    icon: '',
    tags: entry.tags,
    verified: entry.verified,
    recommended: entry.recommended,
    published: true,
    sort: entry.sort,
    evaluation: entry.evaluation,
    testcases: entry.testcases,
    source: { type: 'zip' },
  }
}

function commitOptions(options: SkillHubBatchOptions) {
  return {
    published: options.published,
    recommended: options.recommended,
    sortMode: options.sortMode,
    fixedSort: options.fixedSort,
    sortStart: options.sortStart,
    sortStep: options.sortStep,
    verifiedMode: options.verifiedMode,
    tagMode: options.tagMode,
    commonTags: options.commonTags,
    overrideOrigin: false,
    origin: '',
    missingIcon: options.missingIcon,
    missingTestcases: options.missingTestcases,
    missingEvaluation: options.missingEvaluation,
  }
}

function splitCommitItems<T>(items: T[]) {
  const chunks: T[][] = []
  let current: T[] = []
  let currentBytes = 0
  for (const item of items) {
    const bytes = new TextEncoder().encode(JSON.stringify(item)).byteLength
    if (current.length && currentBytes + bytes > commitChunkTargetBytes) {
      chunks.push(current)
      current = []
      currentBytes = 0
    }
    current.push(item)
    currentBytes += bytes
  }
  if (current.length) chunks.push(current)
  return chunks
}

function parseCommonTags(value: string) {
  return value
    .split(/[,，\n]/)
    .map((tag) => tag.trim())
    .filter(Boolean)
}

function createRow(
  entry: SkillHubBatchEntry,
  status: BatchRowStatus
): BatchRowState {
  return {
    index: entry.index,
    id: entry.id,
    status,
    progress: isTerminal(status) ? 100 : 0,
    message: entry.errors.length ? entry.errors[0].code : '',
  }
}

function isTerminal(status: BatchRowStatus) {
  return ['success', 'skipped', 'failed', 'cancelled', 'unknown'].includes(
    status
  )
}

function BooleanToggle({
  value,
  falseLabel,
  trueLabel,
  onChange,
}: {
  value: boolean
  falseLabel: string
  trueLabel: string
  onChange: (value: boolean) => void
}) {
  return (
    <ToggleGroup
      variant='outline'
      value={[value ? 'true' : 'false']}
      onValueChange={(nextValue) => {
        if (nextValue[0]) onChange(nextValue[0] === 'true')
      }}
    >
      <ToggleGroupItem value='false'>{falseLabel}</ToggleGroupItem>
      <ToggleGroupItem value='true'>{trueLabel}</ToggleGroupItem>
    </ToggleGroup>
  )
}

function OptionSelect({
  value,
  options,
  onChange,
}: {
  value: string
  options: Array<[string, string]>
  onChange: (value: string) => void
}) {
  return (
    <Select
      value={value}
      onValueChange={(nextValue) => {
        if (nextValue) onChange(nextValue)
      }}
    >
      <SelectTrigger className='w-full'>
        <SelectValue />
      </SelectTrigger>
      <SelectContent>
        <SelectGroup>
          {options.map(([optionValue, label]) => (
            <SelectItem key={optionValue} value={optionValue}>
              {label}
            </SelectItem>
          ))}
        </SelectGroup>
      </SelectContent>
    </Select>
  )
}

function IntegerInput({
  value,
  onChange,
}: {
  value: number
  onChange: (value: number) => void
}) {
  return (
    <Input
      type='number'
      min={-2147483648}
      max={2147483647}
      value={value}
      onChange={(event) => onChange(Number(event.target.value))}
    />
  )
}

function MissingPolicyField({
  label,
  value,
  onChange,
}: {
  label: string
  value: 'retain' | 'clear'
  onChange: (value: 'retain' | 'clear') => void
}) {
  const { t } = useTranslation()
  return (
    <Field>
      <FieldLabel>{label}</FieldLabel>
      <OptionSelect
        value={value}
        onChange={(nextValue) => onChange(nextValue as 'retain' | 'clear')}
        options={[
          ['retain', t('Retain existing value')],
          ['clear', t('Clear existing value')],
        ]}
      />
    </Field>
  )
}

function StatusBadge({
  status,
  t,
}: {
  status: BatchRowStatus
  t: (key: string) => string
}) {
  const labelByStatus: Record<BatchRowStatus, string> = {
    pending: t('Pending'),
    validating: t('Validating'),
    uploading: t('Uploading'),
    committing: t('Saving'),
    success: t('Success'),
    skipped: t('Skipped'),
    failed: t('Failed'),
    cancelled: t('Cancelled'),
    unknown: t('Needs review'),
  }
  const variant =
    status === 'failed'
      ? 'destructive'
      : status === 'success'
        ? 'default'
        : 'secondary'
  return <Badge variant={variant}>{labelByStatus[status]}</Badge>
}
