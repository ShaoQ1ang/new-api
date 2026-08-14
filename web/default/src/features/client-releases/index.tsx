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
  Add01Icon,
  RefreshIcon,
  Shield02Icon,
} from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { SectionPageLayout } from '@/components/layout'
import { Button } from '@/components/ui/button'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'
import {
  MANAGEMENT_PERMISSION,
  hasManagementPermission,
} from '@/lib/management-permissions'
import { useAuthStore } from '@/stores/auth-store'

import {
  clientReleaseToForm,
  createClientRelease,
  deleteClientRelease,
  discardClientReleaseUpload,
  listAdminClientReleases,
  setClientReleasePublished,
  updateClientRelease,
  uploadClientRelease,
} from './api'
import { ClientReleaseEditor } from './client-release-editor'
import { ClientReleaseTable } from './client-release-table'
import {
  clientReleaseFileMatchesTarget,
  isClientReleaseTargetReady,
  resolvePublicDownloadURL,
} from './release-target'
import type {
  ClientRelease,
  ClientReleaseArch,
  ClientReleaseChannel,
  ClientReleaseForm,
  ClientReleasePlatform,
} from './types'

const platformOptions: ClientReleasePlatform[] = ['windows', 'macos', 'linux']
const versionPattern = /^\d+\.\d+\.\d+$/

type ReleaseListQuery = {
  platform: ClientReleasePlatform
  keyword: string
  arch: ClientReleaseArch | ''
  channel: ClientReleaseChannel | ''
}

type PendingUpload = { ticket: string; object: string } | null

export function ClientReleases() {
  const { t } = useTranslation()
  const user = useAuthStore((state) => state.auth.user)
  const canManage = hasManagementPermission(
    user,
    MANAGEMENT_PERMISSION.CLIENT_RELEASES
  )
  const canPublish = hasManagementPermission(
    user,
    MANAGEMENT_PERMISSION.CLIENT_RELEASES_PUBLISH
  )

  const [activePlatform, setActivePlatform] =
    useState<ClientReleasePlatform>('windows')
  const [releases, setReleases] = useState<ClientRelease[]>([])
  const [selectedId, setSelectedId] = useState<number | null>(null)
  const [form, setForm] = useState<ClientReleaseForm>(() =>
    clientReleaseToForm(undefined, 'windows')
  )
  const [keyword, setKeyword] = useState('')
  const [archFilter, setArchFilter] = useState<ClientReleaseArch | ''>('')
  const [channelFilter, setChannelFilter] = useState<ClientReleaseChannel | ''>(
    ''
  )
  const [loading, setLoading] = useState(false)
  const [saving, setSaving] = useState(false)
  const [uploading, setUploading] = useState<'installer' | 'updater' | null>(
    null
  )

  const listAbortRef = useRef<AbortController | null>(null)
  const listRequestRef = useRef(0)
  const savingRef = useRef(false)
  const uploadingRef = useRef(false)
  const formRef = useRef(form)
  const pendingUploadsRef = useRef<{
    installer: PendingUpload
    updater: PendingUpload
  }>({ installer: null, updater: null })
  formRef.current = form

  const selected = useMemo(
    () => releases.find((release) => release.id === selectedId),
    [releases, selectedId]
  )
  const selectedPublished =
    selected?.published === true || selected?.status === 1
  const canEditSelected = canManage && (!selectedPublished || canPublish)
  const canEdit = selected ? canEditSelected : canManage
  const baselineForm = clientReleaseToForm(selected, activePlatform)
  const isDirty = JSON.stringify(form) !== JSON.stringify(baselineForm)
  const busy = saving || uploading !== null

  const loadReleases = useCallback(
    async (query: ReleaseListQuery) => {
      listAbortRef.current?.abort()
      const controller = new AbortController()
      listAbortRef.current = controller
      const requestID = ++listRequestRef.current
      setLoading(true)
      try {
        const payload = await listAdminClientReleases({
          keyword: query.keyword.trim() || undefined,
          platform: query.platform,
          arch: query.arch || undefined,
          channel: query.channel || undefined,
          page_size: 100,
          signal: controller.signal,
        })
        if (controller.signal.aborted || requestID !== listRequestRef.current) {
          return []
        }
        if (!payload.success) {
          throw new Error(
            payload.message || t('Failed to load client releases')
          )
        }
        const items = (payload.data?.items || []).filter(
          (release) => release.platform === query.platform
        )
        setReleases(items)
        return items
      } catch (error) {
        if (controller.signal.aborted) return []
        toast.error(
          error instanceof Error
            ? error.message
            : t('Failed to load client releases')
        )
        return []
      } finally {
        if (requestID === listRequestRef.current) setLoading(false)
      }
    },
    [t]
  )

  useEffect(() => {
    void loadReleases({
      platform: activePlatform,
      keyword: '',
      arch: '',
      channel: '',
    })
  }, [activePlatform, loadReleases])

  useEffect(() => {
    return () => {
      listAbortRef.current?.abort()
      const uploads = Object.values(pendingUploadsRef.current)
        .filter((pending): pending is NonNullable<PendingUpload> =>
          Boolean(pending?.ticket)
        )
        .map((pending) =>
          discardClientReleaseUpload(pending.ticket).catch(() => undefined)
        )
      void Promise.all(uploads)
    }
  }, [])

  function currentListQuery(): ReleaseListQuery {
    return {
      platform: activePlatform,
      keyword,
      arch: archFilter,
      channel: channelFilter,
    }
  }

  function discardPendingUploads() {
    const uploads = pendingUploadsRef.current
    pendingUploadsRef.current = { installer: null, updater: null }
    void Promise.all(
      Object.values(uploads)
        .filter((pending): pending is NonNullable<PendingUpload> =>
          Boolean(pending?.ticket)
        )
        .map((pending) =>
          discardClientReleaseUpload(pending.ticket).catch(() => undefined)
        )
    )
  }

  function replacePendingUpload(
    kind: 'installer' | 'updater',
    uploadTicket?: string,
    objectKey?: string
  ) {
    const previous = pendingUploadsRef.current[kind]
    pendingUploadsRef.current[kind] =
      uploadTicket && objectKey
        ? { ticket: uploadTicket, object: objectKey }
        : null
    if (previous?.ticket && previous.ticket !== uploadTicket) {
      void discardClientReleaseUpload(previous.ticket).catch(() => undefined)
    }
  }

  function confirmDiscardChanges() {
    if (!isDirty) return true
    return window.confirm(t('Discard unsaved changes?'))
  }

  function switchPlatform(platform: ClientReleasePlatform) {
    if (platform === activePlatform) return
    if (busy) {
      toast.error(t('Wait for the current save or upload to finish.'))
      return
    }
    if (!confirmDiscardChanges()) return
    discardPendingUploads()
    listAbortRef.current?.abort()
    setActivePlatform(platform)
    setReleases([])
    setSelectedId(null)
    setForm(clientReleaseToForm(undefined, platform))
    setKeyword('')
    setArchFilter('')
    setChannelFilter('')
  }

  function createDraft() {
    if (busy || !confirmDiscardChanges()) return
    discardPendingUploads()
    setSelectedId(null)
    setForm(clientReleaseToForm(undefined, activePlatform))
  }

  function selectRelease(release: ClientRelease) {
    if (release.platform !== activePlatform) {
      toast.error(t('This release belongs to another platform workspace.'))
      return
    }
    if (release.id === selectedId) return
    if (busy || !confirmDiscardChanges()) return
    discardPendingUploads()
    setSelectedId(release.id)
    setForm(clientReleaseToForm(release, activePlatform))
  }

  function update<K extends keyof ClientReleaseForm>(
    key: K,
    value: ClientReleaseForm[K]
  ) {
    const targetField = key === 'version' || key === 'arch' || key === 'channel'
    if (targetField && form[key] !== value) {
      discardPendingUploads()
      setForm((current) => ({
        ...clearClientReleaseAssets(current),
        [key]: value,
      }))
      return
    }
    setForm((current) => ({ ...current, [key]: value }))
  }

  async function uploadPackage(
    file: File | undefined,
    kind: 'installer' | 'updater'
  ) {
    if (!file || uploadingRef.current) return
    if (!isClientReleaseTargetReady(form)) {
      toast.error(t('Complete the version, architecture, and channel first.'))
      return
    }
    if (!isAllowedPackageFile(file, form.platform, kind)) {
      toast.error(packageFileError(t, form.platform, kind))
      return
    }

    const target = clientReleaseTargetKey(form)
    uploadingRef.current = true
    setUploading(kind)
    try {
      const payload = await uploadClientRelease(file, form)
      if (!payload.success || !payload.data) {
        throw new Error(payload.message || t('Failed to upload package'))
      }
      if (clientReleaseTargetKey(formRef.current) !== target) {
        if (payload.data.uploadTicket) {
          await discardClientReleaseUpload(payload.data.uploadTicket).catch(
            () => undefined
          )
        }
        toast.error(t('Release target changed during upload. Upload again.'))
        return
      }
      replacePendingUpload(kind, payload.data.uploadTicket, payload.data.object)
      setForm((current) => {
        if (kind === 'updater') {
          return {
            ...current,
            updaterFileName: payload.data?.fileName || '',
            updaterObjectKey: payload.data?.object || '',
            updaterSize: payload.data?.size || 0,
            updaterSha256: payload.data?.sha256 || '',
            updaterSha512: payload.data?.sha512 || '',
          }
        }
        return {
          ...current,
          fileName: payload.data?.fileName || '',
          objectKey: payload.data?.object || '',
          size: payload.data?.size || 0,
          sha256: payload.data?.sha256 || '',
          sha512: payload.data?.sha512 || '',
        }
      })
      toast.success(t('Package uploaded to OSS'))
    } catch (error) {
      toast.error(
        error instanceof Error ? error.message : t('Failed to upload package')
      )
    } finally {
      uploadingRef.current = false
      setUploading(null)
    }
  }

  async function findConflictingRelease(nextForm: ClientReleaseForm) {
    const payload = await listAdminClientReleases({
      keyword: nextForm.version,
      platform: activePlatform,
      arch: nextForm.arch || undefined,
      channel: nextForm.channel || undefined,
      page_size: 100,
    })
    if (!payload.success) {
      throw new Error(payload.message || t('Failed to check release version'))
    }
    return (payload.data?.items || []).find(
      (release) =>
        release.id !== selected?.id &&
        release.version.trim() === nextForm.version &&
        release.platform === activePlatform &&
        release.arch === nextForm.arch &&
        release.channel === nextForm.channel
    )
  }

  async function saveRelease() {
    if (savingRef.current) return
    const nextForm = { ...form, version: form.version.trim() }
    const validationError = validateForm(t, nextForm)
    if (validationError) {
      toast.error(validationError)
      return
    }

    savingRef.current = true
    setSaving(true)
    try {
      const conflict = await findConflictingRelease(nextForm)
      if (
        conflict &&
        !window.confirm(
          t(
            'A client version for this platform, arch, and channel already exists. Overwrite it?'
          )
        )
      ) {
        return
      }

      let payload
      if (conflict) {
        payload = await updateClientRelease(conflict.id, activePlatform, {
          ...nextForm,
          revision: conflict.revision,
        })
      } else if (selected) {
        payload = await updateClientRelease(
          selected.id,
          activePlatform,
          nextForm
        )
      } else {
        payload = await createClientRelease(nextForm)
      }
      if (!payload.success || !payload.data) {
        throw new Error(payload.message || t('Failed to save client release'))
      }

      pendingUploadsRef.current = { installer: null, updater: null }
      setSelectedId(payload.data.id)
      setForm(clientReleaseToForm(payload.data, activePlatform))
      toast.success(t('Client release saved'))
      await loadReleases(currentListQuery())
    } catch (error) {
      toast.error(
        error instanceof Error
          ? error.message
          : t('Failed to save client release')
      )
      await loadReleases(currentListQuery())
    } finally {
      savingRef.current = false
      setSaving(false)
    }
  }

  async function togglePublish() {
    if (!selected || savingRef.current) return
    savingRef.current = true
    setSaving(true)
    try {
      const payload = await setClientReleasePublished(
        selected.id,
        activePlatform,
        !selectedPublished
      )
      if (!payload.success || !payload.data) {
        throw new Error(payload.message || t('Failed to update publish status'))
      }
      setForm(clientReleaseToForm(payload.data, activePlatform))
      toast.success(
        selectedPublished
          ? t('Client release unpublished')
          : t('Client release published')
      )
      await loadReleases(currentListQuery())
    } catch (error) {
      toast.error(
        error instanceof Error
          ? error.message
          : t('Failed to update publish status')
      )
    } finally {
      savingRef.current = false
      setSaving(false)
    }
  }

  async function removeRelease() {
    if (!selected || savingRef.current) return
    if (!window.confirm(t('Delete this client release?'))) return
    savingRef.current = true
    setSaving(true)
    try {
      const payload = await deleteClientRelease(selected.id, activePlatform)
      if (!payload.success) {
        throw new Error(payload.message || t('Failed to delete client release'))
      }
      discardPendingUploads()
      setSelectedId(null)
      setForm(clientReleaseToForm(undefined, activePlatform))
      toast.success(t('Client release deleted'))
      await loadReleases(currentListQuery())
    } catch (error) {
      toast.error(
        error instanceof Error
          ? error.message
          : t('Failed to delete client release')
      )
    } finally {
      savingRef.current = false
      setSaving(false)
    }
  }

  async function copyDownloadLink(value: string) {
    const url = resolvePublicDownloadURL(value)
    if (!url) {
      toast.error(t('External download link is unavailable'))
      return
    }
    try {
      await navigator.clipboard.writeText(url)
      toast.success(t('External download link copied'))
    } catch {
      toast.error(t('Failed to copy download link'))
    }
  }

  function cancelEdit() {
    if (busy || !confirmDiscardChanges()) return
    discardPendingUploads()
    setForm(clientReleaseToForm(selected, activePlatform))
  }

  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>
        {t('Client Management')}
      </SectionPageLayout.Title>
      <SectionPageLayout.Actions>
        <Button
          variant='outline'
          disabled={loading}
          onClick={() => void loadReleases(currentListQuery())}
        >
          <HugeiconsIcon icon={RefreshIcon} data-icon='inline-start' />
          {loading ? t('Refreshing') : t('Refresh')}
        </Button>
        {canManage ? (
          <Button disabled={busy} onClick={createDraft}>
            <HugeiconsIcon icon={Add01Icon} data-icon='inline-start' />
            {t('New {{platform}} release', { platform: activePlatform })}
          </Button>
        ) : null}
      </SectionPageLayout.Actions>
      <SectionPageLayout.Content>
        <div className='flex flex-col gap-4'>
          <Tabs
            value={activePlatform}
            onValueChange={(value) =>
              switchPlatform(value as ClientReleasePlatform)
            }
          >
            <div className='flex flex-col gap-3 rounded-xl border px-4 py-3 sm:flex-row sm:items-center sm:justify-between'>
              <TabsList variant='line' className='w-full sm:w-auto'>
                {platformOptions.map((platform) => (
                  <TabsTrigger key={platform} value={platform} disabled={busy}>
                    {platformLabel(platform)}
                  </TabsTrigger>
                ))}
              </TabsList>
              <div className='text-muted-foreground flex items-center gap-2 text-xs'>
                <HugeiconsIcon icon={Shield02Icon} />
                {t(
                  'Current workspace: {{platform}} · Other platform data is hidden',
                  { platform: activePlatform }
                )}
              </div>
            </div>
          </Tabs>

          <div className='grid items-start gap-4 xl:grid-cols-[minmax(0,1fr)_minmax(390px,520px)]'>
            <ClientReleaseTable
              platform={activePlatform}
              releases={releases}
              selectedId={selectedId}
              keyword={keyword}
              archFilter={archFilter}
              channelFilter={channelFilter}
              loading={loading}
              onKeywordChange={setKeyword}
              onArchFilterChange={setArchFilter}
              onChannelFilterChange={setChannelFilter}
              onSearch={() => void loadReleases(currentListQuery())}
              onSelect={selectRelease}
              onCopy={(url) => void copyDownloadLink(url)}
            />
            <ClientReleaseEditor
              selected={selected}
              form={form}
              canEdit={canEdit}
              canPublish={canPublish && Boolean(selected)}
              saving={saving}
              uploading={uploading}
              onUpdate={update}
              onUpload={(file, kind) => void uploadPackage(file, kind)}
              onSave={() => void saveRelease()}
              onDelete={() => void removeRelease()}
              onTogglePublish={() => void togglePublish()}
              onCancel={cancelEdit}
              onCopy={(url) => void copyDownloadLink(url)}
            />
          </div>
        </div>
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}

function clearClientReleaseAssets(form: ClientReleaseForm): ClientReleaseForm {
  return {
    ...form,
    fileName: '',
    objectKey: '',
    size: 0,
    sha256: '',
    sha512: '',
    updaterFileName: '',
    updaterObjectKey: '',
    updaterSize: 0,
    updaterSha256: '',
    updaterSha512: '',
  }
}

function clientReleaseTargetKey(form: ClientReleaseForm) {
  return `${form.version.trim()}\u0000${form.platform}\u0000${form.arch}\u0000${form.channel}`
}

function isAllowedPackageFile(
  file: File,
  platform: ClientReleasePlatform,
  kind: 'installer' | 'updater'
) {
  if (kind === 'updater') return /\.zip$/i.test(file.name)
  if (platform === 'macos') return /\.dmg$/i.test(file.name)
  if (platform === 'windows') return /\.(exe|msi|zip)$/i.test(file.name)
  return /\.(appimage|deb|rpm|zip)$/i.test(file.name)
}

function packageFileError(
  t: ReturnType<typeof useTranslation>['t'],
  platform: ClientReleasePlatform,
  kind: 'installer' | 'updater'
) {
  if (kind === 'updater') {
    return t('The macos automatic update package must be a ZIP file')
  }
  if (platform === 'macos') {
    return t('The macos manual download package must be a DMG file')
  }
  if (platform === 'windows') {
    return t('Windows packages must be EXE, MSI, or ZIP files.')
  }
  return t('Linux packages must be AppImage, DEB, RPM, or ZIP files.')
}

function validateForm(
  t: ReturnType<typeof useTranslation>['t'],
  form: ClientReleaseForm
) {
  if (!versionPattern.test(form.version)) {
    return t('Version must use three numeric segments, such as 1.2.3')
  }
  if (!form.arch) return t('Select architecture before saving.')
  if (!form.channel) return t('Select channel before saving.')
  if (!form.fileName || !form.objectKey || form.size <= 0) {
    return t('Upload an installer package first.')
  }
  if (!clientReleaseFileMatchesTarget(form, form.fileName)) {
    return t(
      'The file name must match the selected platform, architecture, and channel.'
    )
  }
  if (
    form.updaterFileName &&
    !clientReleaseFileMatchesTarget(form, form.updaterFileName)
  ) {
    return t(
      'The file name must match the selected platform, architecture, and channel.'
    )
  }
  if (form.minVersion && !versionPattern.test(form.minVersion.trim())) {
    return t('Minimum version must use three numeric segments, such as 1.2.3')
  }
  return ''
}

function platformLabel(platform: ClientReleasePlatform) {
  if (platform === 'windows') return 'Windows'
  if (platform === 'macos') return 'macos'
  return 'Linux'
}
