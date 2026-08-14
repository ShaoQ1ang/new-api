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
  CheckmarkCircle02Icon,
  CopyLinkIcon,
  Delete02Icon,
  Download04Icon,
  FolderUploadIcon,
  SaveIcon,
} from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useRef } from 'react'
import { useTranslation } from 'react-i18next'

import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button, buttonVariants } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import {
  Field,
  FieldDescription,
  FieldError,
  FieldGroup,
  FieldLabel,
  FieldSet,
  FieldTitle,
} from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import { NativeSelect, NativeSelectOption } from '@/components/ui/native-select'
import { Separator } from '@/components/ui/separator'
import { Switch } from '@/components/ui/switch'
import { Textarea } from '@/components/ui/textarea'
import { cn } from '@/lib/utils'

import {
  clientReleaseExpectedFilePattern,
  clientReleaseFileMatchesTarget,
  isClientReleaseTargetReady,
  resolvePublicDownloadURL,
} from './release-target'
import type {
  ClientRelease,
  ClientReleaseArch,
  ClientReleaseChannel,
  ClientReleaseForm,
} from './types'

type UploadKind = 'installer' | 'updater'

type ClientReleaseEditorProps = {
  selected?: ClientRelease
  form: ClientReleaseForm
  canEdit: boolean
  canPublish: boolean
  saving: boolean
  uploading: UploadKind | null
  onUpdate: <K extends keyof ClientReleaseForm>(
    key: K,
    value: ClientReleaseForm[K]
  ) => void
  onUpload: (file: File | undefined, kind: UploadKind) => void
  onSave: () => void
  onDelete: () => void
  onTogglePublish: () => void
  onCancel: () => void
  onCopy: (url: string) => void
}

const archOptions: ClientReleaseArch[] = ['x64', 'arm64', 'ia32', 'universal']
const channelOptions: ClientReleaseChannel[] = ['stable', 'beta']

export function ClientReleaseEditor(props: ClientReleaseEditorProps) {
  const { t } = useTranslation()
  const targetReady = isClientReleaseTargetReady(props.form)
  const installerMatches = clientReleaseFileMatchesTarget(
    props.form,
    props.form.fileName
  )
  const updaterMatches = clientReleaseFileMatchesTarget(
    props.form,
    props.form.updaterFileName
  )
  const published =
    props.selected?.published === true || props.selected?.status === 1
  const hasSaveError =
    !targetReady || !props.form.fileName || !installerMatches || !updaterMatches
  let saveErrorMessage = t(
    'The file name must match the selected platform, architecture, and channel.'
  )
  if (!targetReady) {
    saveErrorMessage = t(
      'Complete the version, architecture, and channel first.'
    )
  } else if (!props.form.fileName) {
    saveErrorMessage = t('Upload an installer package first.')
  }

  return (
    <Card className='min-h-[580px] overflow-hidden py-0'>
      <CardHeader className='border-b px-4 py-4'>
        <div className='flex items-start justify-between gap-3'>
          <div className='flex min-w-0 flex-col gap-1'>
            <CardTitle className='truncate'>
              {props.selected
                ? t('Edit {{version}} · {{platform}}', {
                    version: props.selected.version,
                    platform: props.form.platform,
                  })
                : t('Create {{platform}} release', {
                    platform: props.form.platform,
                  })}
            </CardTitle>
            <CardDescription>
              {t('Platform is locked to the current workspace.')}
            </CardDescription>
          </div>
          {props.selected ? (
            <Badge variant={published ? 'default' : 'outline'}>
              {published ? t('Published') : t('Draft')}
            </Badge>
          ) : null}
        </div>
      </CardHeader>

      <CardContent className='max-h-[calc(100vh-14rem)] overflow-y-auto px-4 py-4'>
        <fieldset
          disabled={!props.canEdit || props.saving || props.uploading !== null}
        >
          <div className='flex flex-col gap-5'>
            <FieldSet>
              <FieldTitle>{t('Release target')}</FieldTitle>
              <FieldGroup className='grid gap-3 md:grid-cols-2'>
                <Field>
                  <FieldLabel htmlFor='client-release-platform'>
                    {t('Platform')}
                  </FieldLabel>
                  <Input
                    id='client-release-platform'
                    value={props.form.platform}
                    readOnly
                    disabled
                  />
                </Field>
                <Field
                  data-invalid={!/^\d+\.\d+\.\d+$/.test(props.form.version)}
                >
                  <FieldLabel htmlFor='client-release-version'>
                    {t('Version')}
                  </FieldLabel>
                  <Input
                    id='client-release-version'
                    value={props.form.version}
                    placeholder='1.2.3'
                    aria-invalid={!/^\d+\.\d+\.\d+$/.test(props.form.version)}
                    onChange={(event) =>
                      props.onUpdate('version', event.target.value)
                    }
                  />
                </Field>
                <Field data-invalid={props.form.arch === ''}>
                  <FieldLabel htmlFor='client-release-arch'>
                    {t('Arch')}
                  </FieldLabel>
                  <NativeSelect
                    id='client-release-arch'
                    value={props.form.arch}
                    aria-invalid={props.form.arch === ''}
                    onChange={(event) =>
                      props.onUpdate(
                        'arch',
                        event.target.value as ClientReleaseArch | ''
                      )
                    }
                  >
                    <NativeSelectOption value='' disabled>
                      {t('Select architecture')}
                    </NativeSelectOption>
                    {archOptions.map((arch) => (
                      <NativeSelectOption key={arch} value={arch}>
                        {arch}
                      </NativeSelectOption>
                    ))}
                  </NativeSelect>
                </Field>
                <Field data-invalid={props.form.channel === ''}>
                  <FieldLabel htmlFor='client-release-channel'>
                    {t('Channel')}
                  </FieldLabel>
                  <NativeSelect
                    id='client-release-channel'
                    value={props.form.channel}
                    aria-invalid={props.form.channel === ''}
                    onChange={(event) =>
                      props.onUpdate(
                        'channel',
                        event.target.value as ClientReleaseChannel | ''
                      )
                    }
                  >
                    <NativeSelectOption value='' disabled>
                      {t('Select channel')}
                    </NativeSelectOption>
                    {channelOptions.map((channel) => (
                      <NativeSelectOption key={channel} value={channel}>
                        {channel}
                      </NativeSelectOption>
                    ))}
                  </NativeSelect>
                </Field>
              </FieldGroup>
              <FieldDescription>
                {t('Select architecture and channel before uploading files.')}
              </FieldDescription>
              <div className='bg-muted/50 rounded-lg px-3 py-2 font-mono text-xs break-all'>
                {t('Expected file name')}:{' '}
                {clientReleaseExpectedFilePattern(props.form)}
              </div>
            </FieldSet>

            <Separator />

            <FieldSet>
              <FieldTitle>{t('Release assets')}</FieldTitle>
              <AssetUploadField
                kind='installer'
                title={
                  props.form.platform === 'macos'
                    ? t('DMG manual download package')
                    : t('Installer package')
                }
                description={
                  props.form.platform === 'macos'
                    ? t('Upload the DMG used for manual downloads.')
                    : t('Upload the installer for this release target.')
                }
                accept={installerAccept(props.form.platform)}
                fileName={props.form.fileName}
                size={props.form.size}
                targetReady={targetReady}
                matches={installerMatches}
                uploading={props.uploading === 'installer'}
                onUpload={props.onUpload}
              />
              {props.form.platform === 'macos' ? (
                <AssetUploadField
                  kind='updater'
                  title={t('ZIP automatic update package')}
                  description={t(
                    'Upload the ZIP generated from the same signed and notarized build as the DMG.'
                  )}
                  accept='.zip'
                  fileName={props.form.updaterFileName}
                  size={props.form.updaterSize}
                  targetReady={targetReady}
                  matches={updaterMatches}
                  uploading={props.uploading === 'updater'}
                  onUpload={props.onUpload}
                />
              ) : null}
            </FieldSet>

            <Separator />

            <FieldSet>
              <FieldTitle>{t('Release policy')}</FieldTitle>
              <FieldGroup>
                <Field>
                  <FieldLabel htmlFor='client-release-min-version'>
                    {t('Minimum supported version')}
                  </FieldLabel>
                  <Input
                    id='client-release-min-version'
                    value={props.form.minVersion}
                    placeholder='1.0.0'
                    onChange={(event) =>
                      props.onUpdate('minVersion', event.target.value)
                    }
                  />
                </Field>
                <Field>
                  <FieldLabel htmlFor='client-release-notes'>
                    {t('Release notes')}
                  </FieldLabel>
                  <Textarea
                    id='client-release-notes'
                    value={props.form.releaseNotes}
                    onChange={(event) =>
                      props.onUpdate('releaseNotes', event.target.value)
                    }
                  />
                </Field>
                <Field orientation='horizontal'>
                  <Switch
                    id='client-release-forced'
                    checked={props.form.forced}
                    onCheckedChange={(checked) =>
                      props.onUpdate('forced', checked)
                    }
                  />
                  <FieldLabel htmlFor='client-release-forced'>
                    {t('Force update below minimum version')}
                  </FieldLabel>
                </Field>
              </FieldGroup>
            </FieldSet>
          </div>
        </fieldset>

        {props.selected ? (
          <>
            <Separator className='my-5' />
            <ExternalDownloadLinks
              release={props.selected}
              onCopy={props.onCopy}
            />
          </>
        ) : null}

        {hasSaveError ? (
          <Alert variant='destructive' className='mt-5'>
            <AlertTitle>{t('Cannot save release')}</AlertTitle>
            <AlertDescription>{saveErrorMessage}</AlertDescription>
          </Alert>
        ) : (
          <Alert className='border-success/30 bg-success/5 text-success mt-5'>
            <HugeiconsIcon icon={CheckmarkCircle02Icon} />
            <AlertTitle>{t('Save validation passed')}</AlertTitle>
            <AlertDescription>
              {t('Platform, architecture, channel, and file names match.')}
            </AlertDescription>
          </Alert>
        )}
      </CardContent>

      <CardFooter className='justify-between border-t px-4 py-3'>
        <div className='flex items-center gap-2'>
          {props.selected && props.canEdit ? (
            <Button variant='destructive' onClick={props.onDelete}>
              <HugeiconsIcon icon={Delete02Icon} data-icon='inline-start' />
              {t('Delete')}
            </Button>
          ) : null}
          {props.selected && props.canPublish ? (
            <Button variant='outline' onClick={props.onTogglePublish}>
              {published ? t('Unpublish') : t('Publish')}
            </Button>
          ) : null}
        </div>
        <div className='flex items-center gap-2'>
          <Button variant='outline' onClick={props.onCancel}>
            {t('Cancel')}
          </Button>
          {props.canEdit ? (
            <Button
              disabled={
                hasSaveError || props.saving || props.uploading !== null
              }
              onClick={props.onSave}
            >
              <HugeiconsIcon icon={SaveIcon} data-icon='inline-start' />
              {props.saving ? t('Saving') : t('Save version')}
            </Button>
          ) : null}
        </div>
      </CardFooter>
    </Card>
  )
}

function AssetUploadField(props: {
  kind: UploadKind
  title: string
  description: string
  accept: string
  fileName: string
  size: number
  targetReady: boolean
  matches: boolean
  uploading: boolean
  onUpload: (file: File | undefined, kind: UploadKind) => void
}) {
  const { t } = useTranslation()
  const inputRef = useRef<HTMLInputElement | null>(null)
  const invalid = Boolean(props.fileName) && !props.matches

  return (
    <Field data-invalid={invalid} data-disabled={!props.targetReady}>
      <div className='flex flex-wrap items-start justify-between gap-3'>
        <div className='flex min-w-0 flex-col gap-1'>
          <FieldLabel>{props.title}</FieldLabel>
          <FieldDescription>{props.description}</FieldDescription>
        </div>
        <input
          ref={inputRef}
          type='file'
          accept={props.accept}
          className='hidden'
          onChange={(event) => {
            props.onUpload(event.target.files?.[0], props.kind)
            event.target.value = ''
          }}
        />
        <Button
          type='button'
          variant='outline'
          disabled={!props.targetReady || props.uploading}
          onClick={() => inputRef.current?.click()}
        >
          <HugeiconsIcon icon={FolderUploadIcon} data-icon='inline-start' />
          {props.uploading ? t('Uploading') : t('Upload or replace')}
        </Button>
      </div>
      <Input
        value={props.fileName}
        placeholder={t('No file uploaded')}
        readOnly
        aria-invalid={invalid}
      />
      {props.fileName ? (
        <div className='flex flex-wrap items-center gap-2'>
          <Badge variant={invalid ? 'destructive' : 'secondary'}>
            {invalid ? t('File name mismatch') : t('File name matches')}
          </Badge>
          <span className='text-muted-foreground text-xs'>
            {formatBytes(props.size)}
          </span>
        </div>
      ) : null}
      {invalid ? (
        <FieldError>
          {t(
            'The file name must match the selected platform, architecture, and channel.'
          )}
        </FieldError>
      ) : null}
    </Field>
  )
}

function ExternalDownloadLinks(props: {
  release: ClientRelease
  onCopy: (url: string) => void
}) {
  const { t } = useTranslation()
  const links = [
    {
      key: 'installer',
      label:
        props.release.platform === 'macos'
          ? t('DMG manual download package')
          : t('Installer package'),
      fileName: props.release.fileName,
      url: resolvePublicDownloadURL(props.release.downloadUrl),
    },
  ]
  if (props.release.platform === 'macos') {
    links.push({
      key: 'updater',
      label: t('ZIP automatic update package'),
      fileName: props.release.updaterFileName || '-',
      url: resolvePublicDownloadURL(props.release.updaterDownloadUrl),
    })
  }
  links.push({
    key: 'manifest',
    label:
      props.release.platform === 'macos'
        ? t('Update manifest latest-mac.yml')
        : t('Update manifest latest.yml'),
    fileName:
      props.release.platform === 'macos' ? 'latest-mac.yml' : 'latest.yml',
    url: resolvePublicDownloadURL(props.release.updateManifestUrl),
  })

  return (
    <FieldSet>
      <FieldTitle>{t('External download links')}</FieldTitle>
      <FieldDescription>
        {t('These links can be opened directly outside the admin console.')}
      </FieldDescription>
      <FieldGroup className='gap-3'>
        {links.map((link) => (
          <div
            key={link.key}
            className='flex flex-col gap-2 rounded-lg border p-3 sm:flex-row sm:items-center sm:justify-between'
          >
            <div className='min-w-0'>
              <div className='text-sm font-medium'>{link.label}</div>
              <div className='text-muted-foreground truncate text-xs'>
                {link.fileName}
              </div>
            </div>
            <div className='flex shrink-0 items-center gap-2'>
              {link.url ? (
                <a
                  className={cn(
                    buttonVariants({ variant: 'outline', size: 'sm' })
                  )}
                  href={link.url}
                  target='_blank'
                  rel='noreferrer'
                >
                  <HugeiconsIcon
                    icon={Download04Icon}
                    data-icon='inline-start'
                  />
                  {link.key === 'manifest' ? t('Open') : t('Download')}
                </a>
              ) : (
                <Button variant='outline' size='sm' disabled>
                  <HugeiconsIcon
                    icon={Download04Icon}
                    data-icon='inline-start'
                  />
                  {link.key === 'manifest' ? t('Open') : t('Download')}
                </Button>
              )}
              <Button
                variant='outline'
                size='sm'
                disabled={!link.url}
                onClick={() => props.onCopy(link.url)}
              >
                <HugeiconsIcon icon={CopyLinkIcon} data-icon='inline-start' />
                {t('Copy link')}
              </Button>
            </div>
          </div>
        ))}
      </FieldGroup>
      {!props.release.published && props.release.status !== 1 ? (
        <FieldDescription>
          {t('External links become available after publishing.')}
        </FieldDescription>
      ) : null}
    </FieldSet>
  )
}

function installerAccept(platform: ClientReleaseForm['platform']) {
  if (platform === 'macos') return '.dmg'
  if (platform === 'windows') return '.exe,.msi,.zip'
  return '.AppImage,.deb,.rpm,.zip'
}

function formatBytes(bytes?: number) {
  if (!bytes || Number.isNaN(bytes)) return '-'
  const units = ['B', 'KB', 'MB', 'GB']
  let value = bytes
  let unit = 0
  while (value >= 1024 && unit < units.length - 1) {
    value /= 1024
    unit += 1
  }
  return `${value.toFixed(unit === 0 ? 0 : 1)} ${units[unit]}`
}
