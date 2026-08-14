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
import type { ClientReleaseForm } from './types'

const versionPattern = /^\d+\.\d+\.\d+$/

export function isClientReleaseTargetReady(form: ClientReleaseForm) {
  return (
    versionPattern.test(form.version.trim()) &&
    form.arch !== '' &&
    form.channel !== ''
  )
}

export function clientReleaseExpectedFileName(
  form: ClientReleaseForm,
  fileName: string
) {
  const normalizedName = fileName.replaceAll('\\', '/')
  const lastDot = normalizedName.lastIndexOf('.')
  let extension =
    lastDot >= 0 ? normalizedName.slice(lastDot).toLowerCase() : ''
  if (extension === '.appimage') extension = '.AppImage'
  return `Z-UP-Setup-${form.version.trim()}-${form.platform}-${form.arch}-${form.channel}${extension}`
}

export function clientReleaseExpectedFilePattern(form: ClientReleaseForm) {
  return `Z-UP-Setup-${form.version.trim() || '{version}'}-${form.platform}-${form.arch || '{arch}'}-${form.channel || '{channel}'}.{ext}`
}

export function clientReleaseFileMatchesTarget(
  form: ClientReleaseForm,
  fileName: string
) {
  if (!fileName || !isClientReleaseTargetReady(form)) return true
  return fileName === clientReleaseExpectedFileName(form, fileName)
}

export function resolvePublicDownloadURL(value?: string) {
  const trimmed = value?.trim()
  if (!trimmed) return ''
  try {
    const parsed = new URL(trimmed)
    if (parsed.protocol !== 'http:' && parsed.protocol !== 'https:') return ''
    return parsed.toString()
  } catch {
    return ''
  }
}
