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

export const SKILL_HUB_BATCH_LIMITS = Object.freeze({
  maxEntries: 200,
  maxFiles: 5000,
  manifestBytes: 10 * 1024 * 1024,
  zipBytes: 50 * 1024 * 1024,
  iconBytes: 1024 * 1024,
  testcasesBytes: 2 * 1024 * 1024,
  defaultSort: 1_000_000,
})

const skillIDPattern = /^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$/
const evaluationDimensions = ['safety', 'access', 'frontier', 'economy']
const manifestNames = new Set(['manifest.json', 'manifest.jsonl'])
const textDecoder = new TextDecoder('utf-8', { fatal: true })

export class SkillHubBatchValidationError extends Error {
  constructor(code, params = {}) {
    super(code)
    this.name = 'SkillHubBatchValidationError'
    this.code = code
    this.params = params
  }
}

export function createSkillHubBatchOptions() {
  return {
    mode: 'skip',
    published: false,
    recommended: false,
    sortMode: 'fixed',
    fixedSort: SKILL_HUB_BATCH_LIMITS.defaultSort,
    sortStart: SKILL_HUB_BATCH_LIMITS.defaultSort,
    sortStep: 10,
    verifiedMode: 'manifest',
    tagMode: 'manifest',
    commonTags: [],
    overrideOrigin: false,
    origin: '',
    missingIcon: 'retain',
    missingTestcases: 'retain',
    missingEvaluation: 'retain',
    concurrency: 2,
    stopOnError: false,
  }
}

export async function parseSkillHubBatchDirectory(fileList) {
  const selectedFiles = Array.from(fileList || [])
  if (selectedFiles.length === 0) {
    throw issue('Please select a batch upload folder.')
  }
  if (selectedFiles.length > SKILL_HUB_BATCH_LIMITS.maxFiles) {
    throw issue('The selected folder contains too many files (maximum {{max}}).', {
      max: SKILL_HUB_BATCH_LIMITS.maxFiles,
    })
  }

  const normalizedFiles = selectedFiles.map((file) => ({
    file,
    path: normalizeSelectedPath(file.webkitRelativePath || file.name),
  }))
  const rootName = commonSelectedRoot(normalizedFiles.map((item) => item.path))
  const filesByPath = new Map()
  const lowerPaths = new Set()

  for (const item of normalizedFiles) {
    const relativePath = rootName
      ? item.path.slice(rootName.length + 1)
      : item.path
    if (!relativePath) continue
    const lowerPath = relativePath.toLowerCase()
    if (filesByPath.has(relativePath) || lowerPaths.has(lowerPath)) {
      throw issue('The selected folder contains duplicate path "{{path}}".', {
        path: relativePath,
      })
    }
    filesByPath.set(relativePath, item.file)
    lowerPaths.add(lowerPath)
  }

  const manifestPaths = Array.from(filesByPath.keys()).filter(
    (path) => !path.includes('/') && manifestNames.has(path.toLowerCase()),
  )
  if (manifestPaths.length !== 1) {
    throw issue(
      'The batch folder must contain exactly one manifest.json or manifest.jsonl at its root.',
    )
  }

  const manifestPath = manifestPaths[0]
  const manifestFile = filesByPath.get(manifestPath)
  if (!manifestFile || manifestFile.size > SKILL_HUB_BATCH_LIMITS.manifestBytes) {
    throw issue('Manifest cannot exceed {{max}} MB.', {
      max: SKILL_HUB_BATCH_LIMITS.manifestBytes >> 20,
    })
  }
  const manifestText = await readUTF8File(
    manifestFile,
    'Manifest must use UTF-8 encoding.',
  )
  const rawItems = parseManifestText(manifestText, manifestPath)
  if (rawItems.length === 0) {
    throw issue('Manifest must contain at least one skill.')
  }
  if (rawItems.length > SKILL_HUB_BATCH_LIMITS.maxEntries) {
    throw issue('Manifest must contain at most {{max}} skills.', {
      max: SKILL_HUB_BATCH_LIMITS.maxEntries,
    })
  }

  const entries = []
  for (let index = 0; index < rawItems.length; index += 1) {
    entries.push(
      await normalizeDirectoryEntry(rawItems[index], index, filesByPath),
    )
  }
  applyDuplicateIDIssues(entries)

  return {
    rootName,
    manifestPath,
    fileCount: filesByPath.size,
    entries,
  }
}

export function validateSkillHubBatchOptions(options) {
  if (!['skip', 'update', 'fail'].includes(options.mode)) {
    throw issue('Conflict mode must be skip, update, or fail.')
  }
  if (!['fixed', 'sequence'].includes(options.sortMode)) {
    throw issue('Sort mode must be fixed or sequential.')
  }
  for (const [name, value] of [
    ['fixedSort', options.fixedSort],
    ['sortStart', options.sortStart],
    ['sortStep', options.sortStep],
  ]) {
    if (!Number.isSafeInteger(value)) {
      throw issue('{{name}} must be a safe integer.', { name })
    }
  }
  if (!['manifest', 'verified', 'unverified'].includes(options.verifiedMode)) {
    throw issue('Verified mode is invalid.')
  }
  if (!['manifest', 'append', 'replace'].includes(options.tagMode)) {
    throw issue('Tag mode is invalid.')
  }
  normalizeTags(options.commonTags)
  if (Array.from(cleanString(options.origin)).length > 64) {
    throw issue('Source name must be 64 characters or fewer.')
  }
  for (const value of [
    options.missingIcon,
    options.missingTestcases,
    options.missingEvaluation,
  ]) {
    if (!['retain', 'clear'].includes(value)) {
      throw issue('Missing resource policy is invalid.')
    }
  }
  if (
    !Number.isInteger(options.concurrency) ||
    options.concurrency < 1 ||
    options.concurrency > 10
  ) {
    throw issue('Concurrency must be an integer between 1 and 10.')
  }
}

export function resolveSkillHubBatchSort(options, entryIndex) {
  return options.sortMode === 'sequence'
    ? options.sortStart + entryIndex * options.sortStep
    : options.fixedSort
}

export function buildSkillHubBatchPayload(
  entry,
  options,
  existing,
  zipUpload,
  iconUpload,
) {
  validateSkillHubBatchOptions(options)
  const commonTags = normalizeTags(options.commonTags)
  const tags =
    options.tagMode === 'replace'
      ? commonTags
      : options.tagMode === 'append'
        ? mergeTags(entry.tags, commonTags)
        : entry.tags
  const verified =
    options.verifiedMode === 'verified'
      ? true
      : options.verifiedMode === 'unverified'
        ? false
        : entry.verified
  const icon = iconUpload
    ? iconUpload.url
    : existing && options.missingIcon === 'retain'
      ? cleanString(existing.icon)
      : ''
  const testcases = entry.testcasesFile
    ? entry.testcases
    : existing && options.missingTestcases === 'retain'
      ? existing.testcases || null
      : null
  const evaluation = entry.evaluationSpecified
    ? entry.evaluation
    : existing && options.missingEvaluation === 'retain'
      ? existing.evaluation || null
      : null

  return {
    id: entry.id,
    name: entry.name,
    description: entry.description,
    version: entry.version,
    author: entry.author,
    origin: options.overrideOrigin ? cleanString(options.origin) : entry.origin,
    originUrl: entry.originUrl,
    license: entry.license,
    icon,
    tags,
    verified,
    recommended: Boolean(options.recommended),
    published: Boolean(options.published),
    sort: resolveSkillHubBatchSort(options, entry.index),
    evaluation,
    testcases,
    source: {
      type: 'zip',
      url: zipUpload.url,
      ref: zipUpload.object,
      checksum: zipUpload.checksum,
    },
  }
}

export function summarizeSkillHubBatchResults(items) {
  const summary = {
    success: 0,
    skipped: 0,
    failed: 0,
    cancelled: 0,
    unknown: 0,
  }
  for (const item of items) {
    if (item.status === 'success') summary.success += 1
    if (item.status === 'skipped') summary.skipped += 1
    if (item.status === 'failed') summary.failed += 1
    if (item.status === 'cancelled') summary.cancelled += 1
    if (item.status === 'unknown') summary.unknown += 1
  }
  return summary
}

export function createSkillHubBatchReport({
  directory,
  options,
  items,
  startedAt,
  finishedAt = new Date().toISOString(),
}) {
  return {
    startedAt,
    finishedAt,
    manifest: directory.manifestPath,
    mode: options.mode,
    concurrency: options.concurrency,
    overrides: {
      published: options.published,
      recommended: options.recommended,
      sortMode: options.sortMode,
      fixedSort: options.fixedSort,
      sortStart: options.sortStart,
      sortStep: options.sortStep,
      verifiedMode: options.verifiedMode,
      tagMode: options.tagMode,
      commonTags: normalizeTags(options.commonTags),
      overrideOrigin: options.overrideOrigin,
      origin: options.overrideOrigin ? cleanString(options.origin) : '',
      missingIcon: options.missingIcon,
      missingTestcases: options.missingTestcases,
      missingEvaluation: options.missingEvaluation,
      stopOnError: options.stopOnError,
    },
    total: items.length,
    summary: summarizeSkillHubBatchResults(items),
    items: items.map(compactReportItem),
  }
}

export function issueMessage(issueValue, translate) {
  if (!issueValue) return ''
  if (typeof issueValue === 'string') return translate(issueValue)
  return translate(issueValue.code || String(issueValue), issueValue.params || {})
}

function parseManifestText(content, manifestPath) {
  const text = content.replace(/^\uFEFF/, '').trim()
  if (!text) return []
  try {
    if (manifestPath.toLowerCase().endsWith('.jsonl')) {
      return text
        .split(/\r?\n/)
        .map((line) => line.trim())
        .filter(Boolean)
        .map((line) => JSON.parse(line))
    }
    const parsed = JSON.parse(text)
    if (Array.isArray(parsed)) return parsed
    if (parsed && Array.isArray(parsed.skills)) return parsed.skills
    return [parsed]
  } catch (error) {
    throw issue('Manifest contains invalid JSON: {{message}}', {
      message: error instanceof Error ? error.message : String(error),
    })
  }
}

async function normalizeDirectoryEntry(raw, index, filesByPath) {
  const errors = []
  if (!raw || typeof raw !== 'object' || Array.isArray(raw)) {
    return {
      index,
      id: '',
      name: '',
      version: '',
      zipPath: '',
      iconPath: '',
      zipFile: null,
      iconFile: null,
      testcasesFile: null,
      errors: [
        issue('Manifest item {{index}} must be an object.', {
          index: index + 1,
        }),
      ],
    }
  }

  let tags = []
  let verified = false
  let recommended = false
  let sort = 0
  let evaluation = null
  const evaluationSpecified = Object.prototype.hasOwnProperty.call(
    raw,
    'evaluation',
  )
  try {
    tags = normalizeTags(raw.tags)
  } catch (error) {
    errors.push(asIssue(error))
  }
  try {
    verified = normalizeBoolean(raw.verified, 'verified')
    recommended = normalizeBoolean(raw.recommended, 'recommended')
    sort = normalizeSort(raw.sort)
    evaluation = normalizeEvaluation(raw.evaluation ?? null)
  } catch (error) {
    errors.push(asIssue(error))
  }

  const zipReference = firstString(
    raw.zip,
    raw.zipPath,
    raw.package,
    raw.packagePath,
  )
  const iconReference = firstString(raw.icon, raw.iconPath)
  const rawTestcases = Object.prototype.hasOwnProperty.call(raw, 'testcases')
    ? raw.testcases
    : raw.testcasesPath

  let zipPath = ''
  let iconPath = ''
  let testcasesPath = ''
  let zipFile = null
  let iconFile = null
  let testcasesFile = null
  let testcases = null

  try {
    zipPath = resolveDirectoryReference(zipReference)
    zipFile = zipPath ? filesByPath.get(zipPath) || null : null
  } catch (error) {
    errors.push(asIssue(error))
  }
  try {
    iconPath = iconReference ? resolveDirectoryReference(iconReference) : ''
    iconFile = iconPath ? filesByPath.get(iconPath) || null : null
  } catch (error) {
    errors.push(asIssue(error))
  }
  if (rawTestcases !== undefined && rawTestcases !== null && rawTestcases !== '') {
    if (typeof rawTestcases !== 'string') {
      errors.push(issue('testcases must be a local JSON file path or null.'))
    } else {
      try {
        testcasesPath = resolveDirectoryReference(rawTestcases)
        testcasesFile = filesByPath.get(testcasesPath) || null
        if (testcasesFile) {
          testcases = await readAndNormalizeTestcases(testcasesFile)
        }
      } catch (error) {
        errors.push(asIssue(error))
      }
    }
  }

  const entry = {
    index,
    raw,
    id: cleanString(raw.id || raw.skillId),
    name: cleanString(raw.name),
    description: cleanString(raw.description),
    version: cleanString(raw.version) || '1.0.0',
    author: cleanString(raw.author),
    origin: cleanString(raw.origin || raw.sourceName),
    originUrl: cleanString(raw.originUrl || raw.sourceProjectUrl),
    license: cleanString(raw.license),
    tags,
    verified,
    recommended,
    sort,
    evaluation,
    evaluationSpecified,
    testcases,
    zipPath,
    iconPath,
    testcasesPath,
    zipFile,
    iconFile,
    testcasesFile,
    errors,
  }
  validateEntryMetadata(entry)
  await validateEntryFiles(entry)
  return entry
}

function validateEntryMetadata(entry) {
  if (!entry.id) {
    entry.errors.push(issue('Skill ID is required.'))
  } else if (!skillIDPattern.test(entry.id)) {
    entry.errors.push(
      issue(
        'Skill ID must start with a letter or number and contain only letters, numbers, dots, underscores, or dashes.',
      ),
    )
  }
  if (!entry.name) {
    entry.errors.push(issue('Skill name is required.'))
  } else if (Array.from(entry.name).length > 100) {
    entry.errors.push(issue('Skill name must be 100 characters or fewer.'))
  }
  if (!entry.version) {
    entry.errors.push(issue('Skill version is required.'))
  }
  if (Array.from(entry.origin).length > 64) {
    entry.errors.push(issue('Source name must be 64 characters or fewer.'))
  }
  if (
    entry.originUrl.length > 2048 ||
    (entry.originUrl && !isHttpURL(entry.originUrl))
  ) {
    entry.errors.push(
      issue(
        'Source URL must be an absolute HTTP or HTTPS URL with at most 2048 characters.',
      ),
    )
  }
  if (Array.from(entry.license).length > 128) {
    entry.errors.push(issue('License must be 128 characters or fewer.'))
  }
  if (!entry.zipPath) {
    entry.errors.push(issue('ZIP path is required.'))
  } else if (!entry.zipFile) {
    entry.errors.push(issue('Referenced file was not found: {{path}}', { path: entry.zipPath }))
  }
  if (entry.iconPath && !entry.iconFile) {
    entry.errors.push(
      issue('Referenced file was not found: {{path}}', { path: entry.iconPath }),
    )
  }
  if (entry.testcasesPath && !entry.testcasesFile) {
    entry.errors.push(
      issue('Referenced file was not found: {{path}}', {
        path: entry.testcasesPath,
      }),
    )
  }
}

async function validateEntryFiles(entry) {
  if (entry.zipFile) {
    if (!entry.zipPath.toLowerCase().endsWith('.zip')) {
      entry.errors.push(issue('Skill package must use the .zip extension.'))
    }
    if (entry.zipFile.size <= 0) {
      entry.errors.push(issue('Skill package cannot be empty.'))
    } else if (entry.zipFile.size > SKILL_HUB_BATCH_LIMITS.zipBytes) {
      entry.errors.push(issue('Skill package must be 50 MB or smaller.'))
    } else if (!(await hasZipHeader(entry.zipFile))) {
      entry.errors.push(issue('Skill package is not a valid ZIP file.'))
    }
  }
  if (entry.iconFile) {
    if (!/\.(png|jpe?g|webp)$/i.test(entry.iconPath)) {
      entry.errors.push(issue('Icon must be PNG, JPG, JPEG, or WebP.'))
    }
    if (entry.iconFile.size <= 0) {
      entry.errors.push(issue('Icon cannot be empty.'))
    } else if (entry.iconFile.size > SKILL_HUB_BATCH_LIMITS.iconBytes) {
      entry.errors.push(issue('Icon must be 1 MB or smaller.'))
    } else if (!(await iconHeaderMatches(entry.iconFile, entry.iconPath))) {
      entry.errors.push(
        issue('Icon content does not match its file extension.'),
      )
    }
  }
}

function applyDuplicateIDIssues(entries) {
  const seen = new Map()
  for (const entry of entries) {
    if (!entry.id) continue
    if (seen.has(entry.id)) {
      entry.errors.push(
        issue('Duplicate Skill ID; first used by item {{index}}.', {
          index: seen.get(entry.id) + 1,
        }),
      )
      continue
    }
    seen.set(entry.id, entry.index)
  }
}

function normalizeSelectedPath(value) {
  const path = String(value || '')
  if (
    !path ||
    path.includes('\\') ||
    path.startsWith('/') ||
    /^[A-Za-z]:/.test(path)
  ) {
    throw issue('The selected folder contains an unsafe path.')
  }
  const parts = path.split('/')
  if (parts.some((part) => !part || part === '.' || part === '..')) {
    throw issue('The selected folder contains an unsafe path.')
  }
  return parts.join('/')
}

function commonSelectedRoot(paths) {
  if (paths.length === 0) return ''
  const first = paths[0].split('/')[0]
  return paths.every((path) => path.startsWith(`${first}/`)) ? first : ''
}

function resolveDirectoryReference(value) {
  const reference = cleanString(value)
  if (!reference) return ''
  if (isHTTPURLReference(reference)) {
    throw issue('Manifest file references must be local paths, not URLs.')
  }
  if (
    reference.includes('\\') ||
    reference.startsWith('/') ||
    /^[A-Za-z]:/.test(reference)
  ) {
    throw issue('Manifest contains an unsafe file path: {{path}}', {
      path: reference,
    })
  }
  const parts = reference.replace(/^\.\//, '').split('/')
  if (
    parts.length === 0 ||
    parts.some((part) => !part || part === '.' || part === '..')
  ) {
    throw issue('Manifest contains an unsafe file path: {{path}}', {
      path: reference,
    })
  }
  return parts.join('/')
}

async function readAndNormalizeTestcases(file) {
  if (!file.name.toLowerCase().endsWith('.json')) {
    throw issue('testcases file extension must be .json.')
  }
  if (file.size <= 0) {
    throw issue('testcases file cannot be empty.')
  }
  if (file.size > SKILL_HUB_BATCH_LIMITS.testcasesBytes) {
    throw issue('testcases file must be 2 MB or smaller.')
  }
  const content = await readUTF8File(
    file,
    'testcases file must use UTF-8 encoding.',
  )
  let parsed
  try {
    parsed = JSON.parse(content.replace(/^\uFEFF/, ''))
  } catch (error) {
    throw issue('testcases file contains invalid JSON: {{message}}', {
      message: error instanceof Error ? error.message : String(error),
    })
  }
  return normalizeTestcases(parsed)
}

function normalizeTestcases(value) {
  if (!value || typeof value !== 'object' || Array.isArray(value)) {
    throw issue('testcases JSON must be an object.')
  }
  if (typeof value.slug !== 'string') {
    throw issue('testcases.slug must be a string.')
  }
  const slug = value.slug.trim()
  if (Array.from(slug).length > 256) {
    throw issue('testcases.slug must be 256 characters or fewer.')
  }
  if (!Array.isArray(value.testcases)) {
    throw issue('testcases.testcases must be an array.')
  }
  if (value.testcases.length > 50) {
    throw issue('testcases must contain 50 cases or fewer.')
  }
  const testcases = value.testcases.map((item, index) => {
    if (!item || typeof item !== 'object' || Array.isArray(item)) {
      throw issue('Testcase {{index}} must be an object.', { index: index + 1 })
    }
    if (!Number.isSafeInteger(item.id)) {
      throw issue('Testcase {{index}} ID must be a safe integer.', {
        index: index + 1,
      })
    }
    if (!Number.isSafeInteger(item.sortOrder)) {
      throw issue('Testcase {{index}} sortOrder must be a safe integer.', {
        index: index + 1,
      })
    }
    if (typeof item.question !== 'string' || !item.question.trim()) {
      throw issue('Testcase {{index}} question is required.', {
        index: index + 1,
      })
    }
    if (typeof item.answer !== 'string' || !item.answer.trim()) {
      throw issue('Testcase {{index}} answer is required.', {
        index: index + 1,
      })
    }
    const question = item.question.trim()
    const answer = item.answer.trim()
    if (Array.from(question).length > 10000) {
      throw issue('Testcase {{index}} question is too long.', {
        index: index + 1,
      })
    }
    if (Array.from(answer).length > 250000) {
      throw issue('Testcase {{index}} answer is too long.', {
        index: index + 1,
      })
    }
    return {
      id: item.id,
      question,
      answer,
      sortOrder: item.sortOrder,
    }
  })
  return { slug, testcases }
}

function normalizeEvaluation(value) {
  if (value === null) return null
  if (!value || typeof value !== 'object' || Array.isArray(value)) {
    throw issue('evaluation must be an object or null.')
  }
  if (
    !value.dimensions ||
    typeof value.dimensions !== 'object' ||
    Array.isArray(value.dimensions)
  ) {
    throw issue('evaluation.dimensions must be an object.')
  }
  const dimensions = {}
  for (const key of evaluationDimensions) {
    const dimension = value.dimensions[key]
    if (!dimension || typeof dimension !== 'object' || Array.isArray(dimension)) {
      throw issue('Evaluation dimension "{{name}}" is required.', { name: key })
    }
    if (
      typeof dimension.score !== 'number' ||
      !Number.isFinite(dimension.score) ||
      dimension.score < 0 ||
      dimension.score > 5
    ) {
      throw issue('Evaluation dimension "{{name}}" score must be between 0 and 5.', {
        name: key,
      })
    }
    const review = normalizeOptionalString(
      dimension.review,
      `evaluation.dimensions.${key}.review`,
    )
    if (Array.from(review).length > 4000) {
      throw issue('Evaluation dimension "{{name}}" review is too long.', {
        name: key,
      })
    }
    dimensions[key] = { score: dimension.score, review }
  }
  if (
    value.overallScore !== undefined &&
    value.overallScore !== null &&
    (typeof value.overallScore !== 'number' ||
      !Number.isFinite(value.overallScore) ||
      value.overallScore < 0 ||
      value.overallScore > 5)
  ) {
    throw issue('evaluation.overallScore must be between 0 and 5.')
  }
  const overallRating = normalizeOptionalString(
    value.overallRating,
    'evaluation.overallRating',
  )
  const overallReview = normalizeOptionalString(
    value.overallReview,
    'evaluation.overallReview',
  )
  if (Array.from(overallRating).length > 80) {
    throw issue('evaluation.overallRating must be 80 characters or fewer.')
  }
  if (Array.from(overallReview).length > 8000) {
    throw issue('evaluation.overallReview must be 8000 characters or fewer.')
  }
  return {
    overallScore: value.overallScore ?? null,
    overallRating,
    overallReview,
    dimensions,
  }
}

function normalizeTags(value) {
  if (value === undefined || value === null || value === '') return []
  if (!Array.isArray(value) && typeof value !== 'string') {
    throw issue('tags must be an array of names or a delimited string.')
  }
  const rawValues = Array.isArray(value) ? value : value.split(/[,，\r\n]/)
  const tags = []
  const seen = new Set()
  for (const raw of rawValues) {
    if (typeof raw !== 'string') {
      throw issue('tags must contain string names.')
    }
    const tag = raw.trim()
    if (!tag) continue
    if (/^\d+$/.test(tag)) {
      throw issue('Tag "{{tag}}" looks like an ID; tags must be names.', { tag })
    }
    if (Array.from(tag).length > 40) {
      throw issue('Tag "{{tag}}" must be 40 characters or fewer.', { tag })
    }
    if (/[\\/]/.test(tag)) {
      throw issue('Tag "{{tag}}" cannot contain slashes.', { tag })
    }
    const key = tag.toLowerCase()
    if (seen.has(key)) continue
    seen.add(key)
    tags.push(tag)
  }
  return tags
}

function mergeTags(first, second) {
  return normalizeTags([...(first || []), ...(second || [])])
}

function normalizeSort(value) {
  if (value === undefined || value === null || value === '') return 0
  if (!Number.isSafeInteger(value)) {
    throw issue('sort must be a safe integer.')
  }
  return value
}

function normalizeBoolean(value, name) {
  if (value === undefined || value === null) return false
  if (typeof value !== 'boolean') {
    throw issue('{{name}} must be a boolean.', { name })
  }
  return value
}

function normalizeOptionalString(value, name) {
  if (value === undefined || value === null) return ''
  if (typeof value !== 'string') {
    throw issue('{{name}} must be a string.', { name })
  }
  return value.trim()
}

async function readUTF8File(file, errorCode) {
  try {
    return textDecoder.decode(await file.arrayBuffer())
  } catch {
    throw issue(errorCode)
  }
}

async function hasZipHeader(file) {
  const bytes = new Uint8Array(await file.slice(0, 4).arrayBuffer())
  return bytes.length >= 4 && bytes[0] === 0x50 && bytes[1] === 0x4b
}

async function iconHeaderMatches(file, path) {
  const bytes = new Uint8Array(await file.slice(0, 16).arrayBuffer())
  const lowerPath = path.toLowerCase()
  const isPNG =
    bytes.length >= 8 &&
    bytes[0] === 0x89 &&
    bytes[1] === 0x50 &&
    bytes[2] === 0x4e &&
    bytes[3] === 0x47 &&
    bytes[4] === 0x0d &&
    bytes[5] === 0x0a &&
    bytes[6] === 0x1a &&
    bytes[7] === 0x0a
  const isJPEG =
    bytes.length >= 3 &&
    bytes[0] === 0xff &&
    bytes[1] === 0xd8 &&
    bytes[2] === 0xff
  const isWebP =
    bytes.length >= 12 &&
    String.fromCharCode(...bytes.slice(0, 4)) === 'RIFF' &&
    String.fromCharCode(...bytes.slice(8, 12)) === 'WEBP'
  if (lowerPath.endsWith('.png')) return isPNG
  if (lowerPath.endsWith('.jpg') || lowerPath.endsWith('.jpeg')) return isJPEG
  if (lowerPath.endsWith('.webp')) return isWebP
  return false
}

function isHttpURL(value) {
  try {
    const parsed = new URL(value)
    return (
      (parsed.protocol === 'http:' || parsed.protocol === 'https:') &&
      Boolean(parsed.host) &&
      !parsed.username &&
      !parsed.password
    )
  } catch {
    return false
  }
}

function isHTTPURLReference(value) {
  try {
    const parsed = new URL(value)
    return parsed.protocol === 'http:' || parsed.protocol === 'https:'
  } catch {
    return false
  }
}

function firstString(...values) {
  for (const value of values) {
    if (value !== undefined && value !== null && String(value).trim() !== '') {
      return String(value).trim()
    }
  }
  return ''
}

function cleanString(value) {
  if (value === undefined || value === null) return ''
  return String(value).trim()
}

function issue(code, params = {}) {
  return new SkillHubBatchValidationError(code, params)
}

function asIssue(error) {
  if (error instanceof SkillHubBatchValidationError) return error
  return issue(error instanceof Error ? error.message : String(error))
}

function compactReportItem(item) {
  return {
    index: item.index,
    id: item.id,
    status: item.status,
    action: item.action || '',
    sort: item.sort,
    uploadSort: item.uploadSort,
    zipPath: item.zipPath,
    iconPath: item.iconPath || '',
    testcases: item.testcases,
    zip: item.zip,
    icon: item.icon,
    skill: item.skill,
    error: item.error || '',
  }
}
