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
  useCallback,
  useEffect,
  useMemo,
  useState,
  type FormEvent,
  type ReactNode,
} from 'react'
import {
  Add01Icon,
  ArrowLeft01Icon,
  ArrowRight01Icon,
  Download04Icon,
  InformationCircleIcon,
  SearchIcon,
} from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { Alert, AlertAction, AlertDescription } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import { Checkbox } from '@/components/ui/checkbox'
import { Input } from '@/components/ui/input'
import {
  Pagination,
  PaginationContent,
  PaginationEllipsis,
  PaginationItem,
} from '@/components/ui/pagination'
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
import { SectionPageLayout } from '@/components/layout'
import { MultiSelect } from '@/components/multi-select'
import { splitSkillHubBatchItems } from '../../../../shared/skill-hub-batch-import.mjs'
import {
  batchDeleteSkillHubSkills,
  batchExportSkillHubSkills,
  listAdminSkillHubSkills,
  listAdminSkillHubSkillsByTags,
  listAdminSkillHubTags,
} from './api'
import { SkillHubBatchUploadDialog } from './batch-upload-dialog'
import type { SkillHubSearch } from './search'
import type { SkillHubSkill, SkillHubTag } from './types'

const DEFAULT_PAGE_SIZE = 20
const PAGE_SIZE_OPTIONS = [10, 20, 50, 100] as const
const PAGE_SIZE_SELECT_ITEMS = PAGE_SIZE_OPTIONS.map((value) => ({
  value: String(value),
  label: value,
}))
const EXPORT_PAGE_SIZE = 100

function getStatusApiValue(statuses: SkillHubSearch['statuses']) {
  const values = (statuses || []).map((status) =>
    status === 'published' ? 1 : 0
  )
  return values.length ? values.join(',') : undefined
}

function getCompactPageNumbers(currentPage: number, totalPages: number) {
  if (totalPages <= 4) {
    return Array.from({ length: totalPages }, (_, index) => index + 1)
  }
  if (currentPage <= 2) return [1, 2, 'right-ellipsis', totalPages]
  if (currentPage >= totalPages - 1) {
    return [1, 'left-ellipsis', totalPages - 1, totalPages]
  }
  return [1, 'left-ellipsis', currentPage, 'right-ellipsis', totalPages]
}

type SkillHubListProps = {
  search: SkillHubSearch
  onSearchChange: (next: SkillHubSearch) => void
  onCreate: () => void
  onEdit: (skillId: string) => void
}

export function SkillHubList({
  search,
  onSearchChange,
  onCreate,
  onEdit,
}: SkillHubListProps) {
  const { t } = useTranslation()
  const [skills, setSkills] = useState<SkillHubSkill[]>([])
  const [tagOptions, setTagOptions] = useState<SkillHubTag[]>([])
  const [keywordDraft, setKeywordDraft] = useState(search.q || '')
  const [jumpPageDraft, setJumpPageDraft] = useState('')
  const [checkedIds, setCheckedIds] = useState<string[]>([])
  const [total, setTotal] = useState(0)
  const [loading, setLoading] = useState(false)
  const [refreshKey, setRefreshKey] = useState(0)
  const [batchWorking, setBatchWorking] = useState(false)
  const [exportState, setExportState] = useState<{
    mode: 'selected' | 'all'
    current: number
    total: number
  } | null>(null)

  const selectedTagIds = useMemo(() => search.tags || [], [search.tags])
  const selectedStatuses = useMemo(
    () => search.statuses || [],
    [search.statuses]
  )
  const statusItems = useMemo(
    () => [
      { value: 'published', label: t('Published') },
      { value: 'draft', label: t('Draft') },
    ],
    [t]
  )
  const page = search.page || 1
  const pageSize = search.pageSize || DEFAULT_PAGE_SIZE
  const pageCount = Math.max(1, Math.ceil(total / pageSize))
  const pageNumbers = getCompactPageNumbers(page, pageCount)
  const currentPageIds = useMemo(
    () => skills.map((skill) => skill.id),
    [skills]
  )
  const currentPageDisplayCount = total === 0 ? 0 : currentPageIds.length
  const currentPageSelectedCount = currentPageIds.filter((id) =>
    checkedIds.includes(id)
  ).length
  const currentPageFullySelected =
    currentPageIds.length > 0 &&
    currentPageSelectedCount === currentPageIds.length
  const currentPagePartiallySelected =
    currentPageSelectedCount > 0 && !currentPageFullySelected
  const allFilteredSelected = total > 0 && checkedIds.length === total

  useEffect(() => {
    setKeywordDraft(search.q || '')
  }, [search.q])

  useEffect(() => {
    let cancelled = false

    async function loadTags() {
      try {
        const payload = await listAdminSkillHubTags({ page_size: 500 })
        if (!payload.success) {
          throw new Error(payload.message || t('Failed to load tags'))
        }
        if (!cancelled) setTagOptions(payload.data?.items || [])
      } catch (error) {
        if (!cancelled) {
          toast.error(
            error instanceof Error ? error.message : t('Failed to load tags')
          )
        }
      }
    }

    void loadTags()
    return () => {
      cancelled = true
    }
  }, [t])

  const loadSkills = useCallback(async () => {
    setLoading(true)
    try {
      const params = {
        keyword: search.q?.trim() || undefined,
        recommended: search.recommended || undefined,
        status: getStatusApiValue(selectedStatuses),
        p: page,
        page_size: pageSize,
      }
      const payload = selectedTagIds.length
        ? await listAdminSkillHubSkillsByTags(selectedTagIds, params)
        : await listAdminSkillHubSkills(params)
      if (!payload.success) {
        throw new Error(payload.message || t('Failed to load Skill Hub'))
      }
      const items = payload.data?.items || []
      const nextTotal = payload.data?.total || 0
      const nextPageCount = Math.max(1, Math.ceil(nextTotal / pageSize))
      if (page > nextPageCount) {
        onSearchChange({ ...search, page: nextPageCount })
        return
      }
      setSkills(items)
      setTotal(nextTotal)
    } catch (error) {
      toast.error(
        error instanceof Error ? error.message : t('Failed to load Skill Hub')
      )
    } finally {
      setLoading(false)
    }
  }, [
    onSearchChange,
    page,
    pageSize,
    search,
    selectedStatuses,
    selectedTagIds,
    t,
  ])

  useEffect(() => {
    void loadSkills()
  }, [loadSkills, refreshKey])

  function applyFilters(next: SkillHubSearch) {
    setCheckedIds([])
    onSearchChange({ ...next, page: 1 })
  }

  function submitSearch() {
    const q = keywordDraft.trim()
    if (q === (search.q || '')) return
    applyFilters({ ...search, q })
  }

  function changeTags(values: string[]) {
    const tags = values
      .map(Number)
      .filter((id) => Number.isInteger(id) && id > 0)
    applyFilters({ ...search, tags })
  }

  function changeStatuses(values: string[]) {
    const statuses = values.filter(
      (value): value is 'published' | 'draft' =>
        value === 'published' || value === 'draft'
    )
    if (statuses.join(',') === selectedStatuses.join(',')) return
    applyFilters({ ...search, statuses })
  }

  function goToPage(nextPage: number) {
    const normalizedPage = Math.min(Math.max(nextPage, 1), pageCount)
    setJumpPageDraft('')
    if (normalizedPage === page || loading) return
    onSearchChange({ ...search, page: normalizedPage })
  }

  function submitJumpPage(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    const nextPage = Number(jumpPageDraft)
    if (!Number.isInteger(nextPage) || nextPage < 1) return
    goToPage(nextPage)
  }

  function toggleCurrentPage(checked: boolean) {
    setCheckedIds((current) => {
      if (!checked) {
        return current.filter((id) => !currentPageIds.includes(id))
      }
      return [...new Set([...current, ...currentPageIds])]
    })
  }

  async function fetchFilteredIds() {
    const params = {
      keyword: search.q?.trim() || undefined,
      recommended: search.recommended || undefined,
      status: getStatusApiValue(selectedStatuses),
      page_size: EXPORT_PAGE_SIZE,
    }
    const fetchPage = (nextPage: number) =>
      selectedTagIds.length
        ? listAdminSkillHubSkillsByTags(selectedTagIds, {
            ...params,
            p: nextPage,
          })
        : listAdminSkillHubSkills({ ...params, p: nextPage })
    const firstPayload = await fetchPage(1)
    if (!firstPayload.success) {
      throw new Error(firstPayload.message || t('Failed to load Skill Hub'))
    }
    const firstItems = firstPayload.data?.items || []
    const filteredTotal = firstPayload.data?.total || firstItems.length
    const remainingPayloads = await Promise.all(
      Array.from(
        {
          length: Math.max(0, Math.ceil(filteredTotal / EXPORT_PAGE_SIZE) - 1),
        },
        (_, index) => fetchPage(index + 2)
      )
    )
    const failedPayload = remainingPayloads.find((payload) => !payload.success)
    if (failedPayload) {
      throw new Error(failedPayload.message || t('Failed to load Skill Hub'))
    }
    return [
      ...firstItems,
      ...remainingPayloads.flatMap((payload) => payload.data?.items || []),
    ].map((skill) => skill.id)
  }

  async function selectAllFiltered() {
    setBatchWorking(true)
    try {
      setCheckedIds(await fetchFilteredIds())
    } catch (error) {
      toast.error(
        error instanceof Error
          ? error.message
          : t('Failed to select all skills')
      )
    } finally {
      setBatchWorking(false)
    }
  }

  async function downloadSkillExportBatches(
    ids: string[],
    mode: 'selected' | 'all'
  ) {
    const batches = splitSkillHubBatchItems(ids)
    setExportState({ mode, current: 0, total: batches.length })
    for (let index = 0; index < batches.length; index += 1) {
      setExportState({ mode, current: index + 1, total: batches.length })
      const blob = await batchExportSkillHubSkills(batches[index])
      const url = URL.createObjectURL(blob)
      const link = document.createElement('a')
      link.href = url
      link.download =
        batches.length === 1
          ? 'skill-hub-export.zip'
          : `skill-hub-export-${String(index + 1).padStart(3, '0')}-of-${String(batches.length).padStart(3, '0')}.zip`
      document.body.appendChild(link)
      link.click()
      link.remove()
      window.setTimeout(() => URL.revokeObjectURL(url), 1000)
    }
  }

  async function exportSelected() {
    if (!checkedIds.length) return
    setBatchWorking(true)
    try {
      await downloadSkillExportBatches(checkedIds, 'selected')
      toast.success(
        t('{{count}} skills exported', { count: checkedIds.length })
      )
    } catch (error) {
      toast.error(
        error instanceof Error
          ? error.message
          : t('Failed to export selected skills')
      )
    } finally {
      setExportState(null)
      setBatchWorking(false)
    }
  }

  async function exportAll() {
    setBatchWorking(true)
    setExportState({ mode: 'all', current: 0, total: 0 })
    try {
      const firstPayload = await listAdminSkillHubSkills({
        p: 1,
        page_size: EXPORT_PAGE_SIZE,
      })
      if (!firstPayload.success) {
        throw new Error(firstPayload.message || t('Failed to load Skill Hub'))
      }
      const firstItems = firstPayload.data?.items || []
      const allTotal = firstPayload.data?.total || firstItems.length
      const remainingPayloads = await Promise.all(
        Array.from(
          { length: Math.max(0, Math.ceil(allTotal / EXPORT_PAGE_SIZE) - 1) },
          (_, index) =>
            listAdminSkillHubSkills({
              p: index + 2,
              page_size: EXPORT_PAGE_SIZE,
            })
        )
      )
      const failedPayload = remainingPayloads.find(
        (payload) => !payload.success
      )
      if (failedPayload) {
        throw new Error(failedPayload.message || t('Failed to load Skill Hub'))
      }
      const ids = [
        ...firstItems,
        ...remainingPayloads.flatMap((payload) => payload.data?.items || []),
      ].map((skill) => skill.id)
      await downloadSkillExportBatches(ids, 'all')
      toast.success(t('{{count}} skills exported', { count: ids.length }))
    } catch (error) {
      toast.error(
        error instanceof Error
          ? error.message
          : t('Failed to export all skills')
      )
    } finally {
      setExportState(null)
      setBatchWorking(false)
    }
  }

  async function deleteSelected() {
    if (
      !checkedIds.length ||
      !window.confirm(
        t('Delete {{count}} selected skills?', { count: checkedIds.length })
      )
    ) {
      return
    }
    setBatchWorking(true)
    try {
      const payload = await batchDeleteSkillHubSkills(checkedIds)
      if (!payload.success) {
        throw new Error(
          payload.message || t('Failed to delete selected skills')
        )
      }
      toast.success(
        t('{{count}} skills deleted', {
          count: payload.data?.deleted || checkedIds.length,
        })
      )
      setCheckedIds([])
      setRefreshKey((current) => current + 1)
    } catch (error) {
      toast.error(
        error instanceof Error
          ? error.message
          : t('Failed to delete selected skills')
      )
    } finally {
      setBatchWorking(false)
    }
  }

  let selectionNotice: ReactNode = t(
    'Select all on this page only selects the current {{count}} items. Selections accumulate across pages.',
    { count: currentPageDisplayCount }
  )
  if (currentPageFullySelected) {
    selectionNotice = allFilteredSelected ? (
      t('All {{count}} filtered results are selected.', { count: total })
    ) : (
      <>
        {t('All {{count}} items on this page are selected.', {
          count: currentPageIds.length,
        })}{' '}
        <button
          type='button'
          className='text-primary font-medium underline underline-offset-4'
          disabled={batchWorking}
          onClick={() => void selectAllFiltered()}
        >
          {t('Select all {{count}} filtered results', { count: total })}
        </button>
      </>
    )
  }

  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>{t('Skill management')}</SectionPageLayout.Title>
      <SectionPageLayout.Actions>
        <SkillHubBatchUploadDialog
          tags={tagOptions}
          onComplete={() => setRefreshKey((current) => current + 1)}
        />
        <Button
          variant='outline'
          disabled={batchWorking || loading}
          onClick={() => void exportAll()}
        >
          <HugeiconsIcon icon={Download04Icon} data-icon='inline-start' />
          {exportState?.mode === 'all' && exportState.total
            ? t('Exporting batch {{current}} of {{total}}', exportState)
            : t('Export all')}
        </Button>
        <Button onClick={onCreate}>
          <HugeiconsIcon icon={Add01Icon} data-icon='inline-start' />
          {t('New skill')}
        </Button>
        <Button
          variant='outline'
          disabled={loading}
          onClick={() => setRefreshKey((current) => current + 1)}
        >
          {loading ? t('Refreshing') : t('Refresh')}
        </Button>
      </SectionPageLayout.Actions>
      <SectionPageLayout.Content>
        <div className='text-muted-foreground mb-4 text-sm'>
          {t('Manage skills available to local connectors.')}
        </div>

        <Card>
          <CardContent className='flex flex-col gap-4 pt-6'>
            <div className='flex flex-col gap-3'>
              <div className='grid gap-1.5 sm:grid-cols-[80px_minmax(0,560px)] sm:items-center'>
                <span className='text-sm font-medium'>{t('Search')}</span>
                <div className='flex min-w-0 gap-2'>
                  <Input
                    value={keywordDraft}
                    onChange={(event) => setKeywordDraft(event.target.value)}
                    onKeyDown={(event) => {
                      if (event.key === 'Enter') submitSearch()
                    }}
                    placeholder={t('Search ID, name, or tag')}
                  />
                  <Button variant='outline' onClick={submitSearch}>
                    <HugeiconsIcon icon={SearchIcon} data-icon='inline-start' />
                    {t('Search')}
                  </Button>
                </div>
              </div>
              <div className='grid gap-1.5 sm:grid-cols-[80px_minmax(0,360px)] sm:items-center'>
                <span className='text-sm font-medium'>{t('Status')}</span>
                <MultiSelect
                  options={statusItems}
                  selected={selectedStatuses}
                  onChange={changeStatuses}
                  placeholder={t('All Status')}
                  maxVisibleChips={2}
                />
              </div>
              <div className='grid gap-1.5 sm:grid-cols-[80px_minmax(0,560px)] sm:items-center'>
                <span className='text-sm font-medium'>{t('Tags')}</span>
                <MultiSelect
                  options={tagOptions.map((tag) => ({
                    label: tag.name,
                    value: String(tag.id),
                  }))}
                  selected={selectedTagIds.map(String)}
                  onChange={changeTags}
                  placeholder={t('All tags')}
                  emptyText={t('No tags')}
                  maxVisibleChips={2}
                />
              </div>
              <div className='grid gap-1.5 sm:grid-cols-[80px_auto] sm:items-center'>
                <span className='text-sm font-medium'>
                  {t('Recommendation')}
                </span>
                <div className='flex shrink-0 gap-1 justify-self-start rounded-lg border p-1'>
                  <Button
                    size='sm'
                    variant={search.recommended ? 'ghost' : 'secondary'}
                    onClick={() => {
                      if (search.recommended) {
                        applyFilters({ ...search, recommended: false })
                      }
                    }}
                  >
                    {t('All')}
                  </Button>
                  <Button
                    size='sm'
                    variant={search.recommended ? 'secondary' : 'ghost'}
                    onClick={() => {
                      if (!search.recommended) {
                        applyFilters({ ...search, recommended: true })
                      }
                    }}
                  >
                    {t('Recommended')}
                  </Button>
                </div>
              </div>
            </div>

            <div className='flex flex-wrap items-center gap-2 border-y py-2'>
              <span className='text-muted-foreground text-sm'>
                {t('{{count}} selected', { count: checkedIds.length })}
              </span>
              <Button
                size='sm'
                variant='outline'
                disabled={!checkedIds.length || batchWorking}
                onClick={() => void exportSelected()}
              >
                <HugeiconsIcon icon={Download04Icon} data-icon='inline-start' />
                {exportState?.mode === 'selected' && exportState.total
                  ? t('Exporting batch {{current}} of {{total}}', exportState)
                  : t('Export selected')}
              </Button>
              <Button
                size='sm'
                variant='destructive'
                disabled={!checkedIds.length || batchWorking}
                onClick={() => void deleteSelected()}
              >
                {t('Delete selected')}
              </Button>
            </div>

            <Alert className='border-blue-200 bg-blue-50/80 text-blue-950 dark:border-blue-900 dark:bg-blue-950/40 dark:text-blue-100'>
              <HugeiconsIcon icon={InformationCircleIcon} strokeWidth={2} />
              <AlertDescription className='pr-2 text-blue-900 dark:text-blue-100'>
                {selectionNotice}
              </AlertDescription>
              {checkedIds.length > 0 && (
                <AlertAction>
                  <Button
                    size='sm'
                    variant='ghost'
                    onClick={() => setCheckedIds([])}
                  >
                    {t('Clear selection')}
                  </Button>
                </AlertAction>
              )}
            </Alert>

            <div className='overflow-hidden rounded-lg border'>
              <Table>
                <TableHeader>
                  <TableRow className='bg-muted/40 hover:bg-muted/40'>
                    <TableHead className='w-[190px]'>
                      <div className='flex items-center gap-2 whitespace-nowrap'>
                        <Checkbox
                          aria-label={t('Select current page')}
                          checked={currentPageFullySelected}
                          indeterminate={currentPagePartiallySelected}
                          onCheckedChange={(checked) =>
                            toggleCurrentPage(checked === true)
                          }
                        />
                        <button
                          type='button'
                          className='hover:text-foreground focus-visible:ring-ring cursor-pointer rounded-sm text-left focus-visible:ring-2 focus-visible:outline-none'
                          onClick={() =>
                            toggleCurrentPage(!currentPageFullySelected)
                          }
                        >
                          {t('Select current page ({{count}})', {
                            count: currentPageDisplayCount,
                          })}
                        </button>
                      </div>
                    </TableHead>
                    <TableHead>{t('Skill')}</TableHead>
                    <TableHead className='w-[120px]'>{t('Version')}</TableHead>
                    <TableHead className='w-[140px]'>{t('Status')}</TableHead>
                    <TableHead className='w-[220px]'>{t('Tags')}</TableHead>
                    <TableHead className='w-[100px] text-right'>
                      {t('Actions')}
                    </TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {skills.map((skill) => {
                    const published = skill.published || skill.status === 1
                    const tags = normalizeSkillTags(skill.tags)
                    return (
                      <TableRow key={skill.id}>
                        <TableCell>
                          <Checkbox
                            aria-label={t('Select {{name}}', {
                              name: skill.name,
                            })}
                            checked={checkedIds.includes(skill.id)}
                            onCheckedChange={(checked) =>
                              setCheckedIds((current) =>
                                checked
                                  ? [...new Set([...current, skill.id])]
                                  : current.filter((id) => id !== skill.id)
                              )
                            }
                          />
                        </TableCell>
                        <TableCell>
                          <button
                            type='button'
                            className='max-w-[440px] text-left'
                            onClick={() => onEdit(skill.id)}
                          >
                            <div className='text-primary truncate font-medium hover:underline'>
                              {skill.name}
                            </div>
                            <div className='text-muted-foreground truncate text-xs'>
                              {skill.id}
                              {skill.author ? ` · ${skill.author}` : ''}
                              {skill.origin ? ` · ${skill.origin}` : ''}
                            </div>
                            <div className='text-muted-foreground mt-1 line-clamp-1 text-sm'>
                              {skill.description?.trim() ||
                                t('No description available.')}
                            </div>
                          </button>
                        </TableCell>
                        <TableCell>{skill.version}</TableCell>
                        <TableCell>
                          <div className='flex flex-wrap gap-1'>
                            <Badge variant={published ? 'default' : 'outline'}>
                              {published ? t('Published') : t('Draft')}
                            </Badge>
                            {skill.recommended && (
                              <Badge variant='secondary'>
                                {t('Recommended')}
                              </Badge>
                            )}
                          </div>
                        </TableCell>
                        <TableCell>
                          <div className='flex max-w-[220px] flex-wrap gap-1'>
                            {tags.slice(0, 3).map((tag) => (
                              <Badge key={tag} variant='outline'>
                                {tag}
                              </Badge>
                            ))}
                            {tags.length > 3 && (
                              <span className='text-muted-foreground text-xs'>
                                +{tags.length - 3}
                              </span>
                            )}
                          </div>
                        </TableCell>
                        <TableCell className='text-right'>
                          <Button
                            size='sm'
                            variant='ghost'
                            onClick={() => onEdit(skill.id)}
                          >
                            {t('Edit')}
                          </Button>
                        </TableCell>
                      </TableRow>
                    )
                  })}
                  {!skills.length && (
                    <TableRow>
                      <TableCell
                        colSpan={6}
                        className='text-muted-foreground h-32 text-center'
                      >
                        {loading ? t('Loading...') : t('No skills configured')}
                      </TableCell>
                    </TableRow>
                  )}
                </TableBody>
              </Table>
            </div>

            <div className='flex flex-col gap-3 xl:flex-row xl:items-center xl:justify-between'>
              <span className='text-muted-foreground text-sm'>
                {t('{{total}} results, page {{page}} of {{pageCount}}', {
                  total,
                  page,
                  pageCount,
                })}
              </span>
              <div className='flex flex-wrap items-center gap-3'>
                <div className='flex items-center gap-2'>
                  <span className='text-muted-foreground text-sm whitespace-nowrap'>
                    {t('Rows per page')}
                  </span>
                  <Select
                    items={PAGE_SIZE_SELECT_ITEMS}
                    value={String(pageSize)}
                    onValueChange={(value) =>
                      onSearchChange({
                        ...search,
                        page: 1,
                        pageSize: Number(value) as 10 | 20 | 50 | 100,
                      })
                    }
                  >
                    <SelectTrigger size='sm' className='w-[72px]'>
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent side='top' alignItemWithTrigger={false}>
                      <SelectGroup>
                        {PAGE_SIZE_OPTIONS.map((option) => (
                          <SelectItem key={option} value={String(option)}>
                            {option}
                          </SelectItem>
                        ))}
                      </SelectGroup>
                    </SelectContent>
                  </Select>
                </div>

                <Pagination className='mx-0 w-auto'>
                  <PaginationContent>
                    <PaginationItem>
                      <Button
                        size='icon-sm'
                        variant='outline'
                        aria-label={t('Go to previous page')}
                        disabled={page <= 1 || loading}
                        onClick={() => goToPage(page - 1)}
                      >
                        <HugeiconsIcon icon={ArrowLeft01Icon} />
                      </Button>
                    </PaginationItem>
                    {pageNumbers.map((pageNumber) => (
                      <PaginationItem key={pageNumber}>
                        {typeof pageNumber === 'string' ? (
                          <PaginationEllipsis className='size-7' />
                        ) : (
                          <Button
                            size='icon-sm'
                            variant={
                              pageNumber === page ? 'default' : 'outline'
                            }
                            aria-current={
                              pageNumber === page ? 'page' : undefined
                            }
                            aria-label={t('Go to page {{page}}', {
                              page: pageNumber,
                            })}
                            disabled={loading}
                            onClick={() => goToPage(pageNumber)}
                          >
                            {pageNumber}
                          </Button>
                        )}
                      </PaginationItem>
                    ))}
                    <PaginationItem>
                      <Button
                        size='icon-sm'
                        variant='outline'
                        aria-label={t('Go to next page')}
                        disabled={page >= pageCount || loading}
                        onClick={() => goToPage(page + 1)}
                      >
                        <HugeiconsIcon icon={ArrowRight01Icon} />
                      </Button>
                    </PaginationItem>
                  </PaginationContent>
                </Pagination>

                <form
                  className='flex items-center gap-2'
                  onSubmit={submitJumpPage}
                >
                  <span className='text-muted-foreground text-sm whitespace-nowrap'>
                    {t('Page')}
                  </span>
                  <Input
                    className='h-7 w-16'
                    type='number'
                    inputMode='numeric'
                    min={1}
                    max={pageCount}
                    value={jumpPageDraft}
                    placeholder={String(page)}
                    aria-label={t('Page')}
                    onChange={(event) => setJumpPageDraft(event.target.value)}
                  />
                  <Button
                    size='sm'
                    variant='outline'
                    type='submit'
                    disabled={!jumpPageDraft || loading}
                  >
                    {t('Go')}
                  </Button>
                </form>
              </div>
            </div>
          </CardContent>
        </Card>
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}

function normalizeSkillTags(value: unknown): string[] {
  if (Array.isArray(value)) {
    return value.map((item) => String(item).trim()).filter(Boolean)
  }
  return String(value || '')
    .split(/[,，\n]/)
    .map((item) => item.trim())
    .filter(Boolean)
}
