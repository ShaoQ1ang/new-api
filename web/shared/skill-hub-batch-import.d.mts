export type SkillHubBatchIssue = Error & {
  code: string
  params: Record<string, string | number | boolean>
}

export type SkillHubBatchTestcase = {
  id: number
  question: string
  answer: string
  sortOrder: number
}

export type SkillHubBatchTestcases = {
  slug: string
  skill_name?: string
  testcases: SkillHubBatchTestcase[]
}

export type SkillHubBatchEntry = {
  index: number
  raw: Record<string, unknown>
  id: string
  name: string
  description: string
  version: string
  author: string
  origin: string
  originUrl: string
  license: string
  tags: string[]
  verified: boolean
  recommended: boolean
  sort: number
  evaluation: Record<string, unknown> | null
  evaluationSpecified: boolean
  testcases: SkillHubBatchTestcases | null
  zipPath: string
  iconPath: string
  testcasesPath: string
  zipFile: File | null
  iconFile: File | null
  testcasesFile: File | null
  errors: SkillHubBatchIssue[]
}

export type SkillHubBatchDirectory = {
  rootName: string
  manifestPath: string
  fileCount: number
  entries: SkillHubBatchEntry[]
}

export type SkillHubBatchOptions = {
  mode: 'skip' | 'update' | 'fail'
  published: boolean
  recommended: boolean
  sortMode: 'fixed' | 'sequence'
  fixedSort: number
  sortStart: number
  sortStep: number
  verifiedMode: 'manifest' | 'verified' | 'unverified'
  tagMode: 'manifest' | 'append' | 'replace'
  commonTags: string[]
  overrideOrigin: boolean
  origin: string
  missingIcon: 'retain' | 'clear'
  missingTestcases: 'retain' | 'clear'
  missingEvaluation: 'retain' | 'clear'
  concurrency: number
  stopOnError: boolean
}

export const SKILL_HUB_BATCH_LIMITS: {
  maxEntries: number
  maxFiles: number
  manifestBytes: number
  zipBytes: number
  iconBytes: number
  testcasesBytes: number
  defaultSort: number
}

export class SkillHubBatchValidationError extends Error {
  code: string
  params: Record<string, string | number | boolean>
}

export function createSkillHubBatchOptions(): SkillHubBatchOptions
export function parseSkillHubBatchDirectory(
  fileList: FileList | File[],
): Promise<SkillHubBatchDirectory>
export function validateSkillHubBatchOptions(
  options: SkillHubBatchOptions,
): void
export function resolveSkillHubBatchSort(
  options: SkillHubBatchOptions,
  entryIndex: number,
): number
export function buildSkillHubBatchPayload(
  entry: SkillHubBatchEntry,
  options: SkillHubBatchOptions,
  existing: Record<string, unknown> | null,
  zipUpload: Record<string, unknown>,
  iconUpload: Record<string, unknown> | null,
): Record<string, unknown>
export function summarizeSkillHubBatchResults(
  items: Array<{ status: string }>,
): {
  success: number
  skipped: number
  failed: number
  cancelled: number
  unknown: number
}
export function createSkillHubBatchReport(input: {
  directory: SkillHubBatchDirectory
  options: SkillHubBatchOptions
  items: Array<Record<string, unknown>>
  startedAt: string
  finishedAt?: string
}): Record<string, unknown>
export function issueMessage(
  issue:
    | SkillHubBatchIssue
    | (Error & {
        code?: string
        params?: Record<string, string | number | boolean>
      })
    | string,
  translate: (
    key: string,
    params?: Record<string, string | number | boolean>,
  ) => string,
): string
