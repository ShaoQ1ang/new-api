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
  CopyLinkIcon,
  Download04Icon,
  SearchIcon,
} from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useTranslation } from 'react-i18next'

import { Badge } from '@/components/ui/badge'
import { Button, buttonVariants } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { NativeSelect, NativeSelectOption } from '@/components/ui/native-select'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import { cn } from '@/lib/utils'

import { resolvePublicDownloadURL } from './release-target'
import type {
  ClientRelease,
  ClientReleaseArch,
  ClientReleaseChannel,
  ClientReleasePlatform,
} from './types'

type ClientReleaseTableProps = {
  platform: ClientReleasePlatform
  releases: ClientRelease[]
  selectedId: number | null
  keyword: string
  archFilter: ClientReleaseArch | ''
  channelFilter: ClientReleaseChannel | ''
  loading: boolean
  onKeywordChange: (value: string) => void
  onArchFilterChange: (value: ClientReleaseArch | '') => void
  onChannelFilterChange: (value: ClientReleaseChannel | '') => void
  onSearch: () => void
  onSelect: (release: ClientRelease) => void
  onCopy: (url: string) => void
}

const archOptions: ClientReleaseArch[] = ['x64', 'arm64', 'ia32', 'universal']
const channelOptions: ClientReleaseChannel[] = ['stable', 'beta']

export function ClientReleaseTable(props: ClientReleaseTableProps) {
  const { t } = useTranslation()

  return (
    <Card className='min-h-[580px] overflow-hidden py-0'>
      <CardHeader className='gap-3 border-b px-4 py-4'>
        <div className='flex items-center justify-between gap-3'>
          <CardTitle>
            {t('{{platform}} releases', { platform: props.platform })}
          </CardTitle>
          <Badge variant='outline'>
            {t('{{count}} records', { count: props.releases.length })}
          </Badge>
        </div>
        <div className='grid gap-2 xl:grid-cols-[minmax(180px,1fr)_130px_120px_auto]'>
          <Input
            value={props.keyword}
            placeholder={t('Search version or file')}
            aria-label={t('Search version or file')}
            onChange={(event) => props.onKeywordChange(event.target.value)}
            onKeyDown={(event) => {
              if (event.key === 'Enter') props.onSearch()
            }}
          />
          <NativeSelect
            value={props.archFilter}
            aria-label={t('Arch')}
            onChange={(event) =>
              props.onArchFilterChange(
                event.target.value as ClientReleaseArch | ''
              )
            }
          >
            <NativeSelectOption value=''>{t('All arches')}</NativeSelectOption>
            {archOptions.map((arch) => (
              <NativeSelectOption key={arch} value={arch}>
                {arch}
              </NativeSelectOption>
            ))}
          </NativeSelect>
          <NativeSelect
            value={props.channelFilter}
            aria-label={t('Channel')}
            onChange={(event) =>
              props.onChannelFilterChange(
                event.target.value as ClientReleaseChannel | ''
              )
            }
          >
            <NativeSelectOption value=''>
              {t('All channels')}
            </NativeSelectOption>
            {channelOptions.map((channel) => (
              <NativeSelectOption key={channel} value={channel}>
                {channel}
              </NativeSelectOption>
            ))}
          </NativeSelect>
          <Button variant='outline' onClick={props.onSearch}>
            <HugeiconsIcon icon={SearchIcon} data-icon='inline-start' />
            {t('Search')}
          </Button>
        </div>
      </CardHeader>
      <CardContent className='p-0'>
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>{t('Version')}</TableHead>
              <TableHead>{t('Arch')}</TableHead>
              <TableHead>{t('Channel')}</TableHead>
              <TableHead>{t('Installer package')}</TableHead>
              {props.platform === 'macos' ? (
                <TableHead>{t('Automatic update package')}</TableHead>
              ) : null}
              <TableHead>{t('Status')}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {props.releases.map((release) => (
              <ReleaseRow
                key={release.id}
                release={release}
                selected={props.selectedId === release.id}
                showUpdater={props.platform === 'macos'}
                onSelect={props.onSelect}
                onCopy={props.onCopy}
              />
            ))}
          </TableBody>
        </Table>
        {!props.releases.length ? (
          <div className='text-muted-foreground p-8 text-center text-sm'>
            {props.loading
              ? t('Loading...')
              : t('No client versions configured')}
          </div>
        ) : null}
      </CardContent>
    </Card>
  )
}

function ReleaseRow(props: {
  release: ClientRelease
  selected: boolean
  showUpdater: boolean
  onSelect: (release: ClientRelease) => void
  onCopy: (url: string) => void
}) {
  const { t } = useTranslation()
  const published =
    props.release.published === true || props.release.status === 1
  const downloadURL = resolvePublicDownloadURL(props.release.downloadUrl)
  const updaterURL = resolvePublicDownloadURL(props.release.updaterDownloadUrl)

  return (
    <TableRow
      data-state={props.selected ? 'selected' : undefined}
      className='cursor-pointer'
      tabIndex={0}
      onClick={() => props.onSelect(props.release)}
      onKeyDown={(event) => {
        if (event.key === 'Enter' || event.key === ' ') {
          event.preventDefault()
          props.onSelect(props.release)
        }
      }}
    >
      <TableCell className='font-medium'>{props.release.version}</TableCell>
      <TableCell>{props.release.arch}</TableCell>
      <TableCell>
        <Badge variant='secondary'>{props.release.channel}</Badge>
      </TableCell>
      <TableCell>
        <DownloadActions
          url={downloadURL}
          onCopy={props.onCopy}
          downloadLabel={t('Download installer')}
          copyLabel={t('Copy installer download link')}
        />
      </TableCell>
      {props.showUpdater ? (
        <TableCell>
          <DownloadActions
            url={updaterURL}
            onCopy={props.onCopy}
            downloadLabel={t('Download updater')}
            copyLabel={t('Copy updater download link')}
          />
        </TableCell>
      ) : null}
      <TableCell>
        <Badge variant={published ? 'default' : 'outline'}>
          {published ? t('Published') : t('Draft')}
        </Badge>
      </TableCell>
    </TableRow>
  )
}

function DownloadActions(props: {
  url: string
  downloadLabel: string
  copyLabel: string
  onCopy: (url: string) => void
}) {
  const { t } = useTranslation()

  if (!props.url) {
    return <span className='text-muted-foreground'>{t('Unavailable')}</span>
  }

  return (
    <div className='flex items-center gap-1'>
      <Tooltip>
        <TooltipTrigger
          render={
            <a
              className={cn(
                buttonVariants({ variant: 'ghost', size: 'icon-sm' })
              )}
              href={props.url}
              target='_blank'
              rel='noreferrer'
              aria-label={props.downloadLabel}
              onClick={(event) => event.stopPropagation()}
            />
          }
        >
          <HugeiconsIcon icon={Download04Icon} />
        </TooltipTrigger>
        <TooltipContent>{props.downloadLabel}</TooltipContent>
      </Tooltip>
      <Tooltip>
        <TooltipTrigger
          render={
            <Button
              variant='ghost'
              size='icon-sm'
              aria-label={props.copyLabel}
              onClick={(event) => {
                event.stopPropagation()
                props.onCopy(props.url)
              }}
            />
          }
        >
          <HugeiconsIcon icon={CopyLinkIcon} />
        </TooltipTrigger>
        <TooltipContent>{props.copyLabel}</TooltipContent>
      </Tooltip>
    </div>
  )
}
