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

import assert from 'node:assert/strict'
import test from 'node:test'
import {
  buildSkillHubBatchPayload,
  createSkillHubBatchOptions,
  parseSkillHubBatchDirectory,
  resolveSkillHubBatchSort,
  summarizeSkillHubBatchResults,
} from './skill-hub-batch-import.mjs'

function selectedFile(path, content, type = '') {
  const file = new File([content], path.split('/').at(-1), { type })
  Object.defineProperty(file, 'webkitRelativePath', {
    configurable: true,
    value: `batch/${path}`,
  })
  return file
}

const zipBytes = new Uint8Array([0x50, 0x4b, 0x03, 0x04])

test('directory manifest resolves local zip, icon, and testcase files', async () => {
  const directory = await parseSkillHubBatchDirectory([
    selectedFile(
      'manifest.json',
      JSON.stringify([
        {
          id: 'demo',
          name: 'Demo',
          zip: './packages/demo.zip',
          icon: './icons/demo.png',
          testcases: './testcases/demo.json',
        },
      ]),
    ),
    selectedFile('packages/demo.zip', zipBytes, 'application/zip'),
    selectedFile(
      'icons/demo.png',
      new Uint8Array([0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a]),
      'image/png',
    ),
    selectedFile(
      'testcases/demo.json',
      JSON.stringify({
        slug: 'demo',
        testcases: [
          { id: 1, question: 'Question', answer: 'Answer', sortOrder: 0 },
        ],
      }),
      'application/json',
    ),
  ])
  assert.equal(directory.entries.length, 1)
  assert.deepEqual(directory.entries[0].errors, [])
  assert.equal(directory.entries[0].testcases.testcases.length, 1)
})

test('global overrides replace publish, recommendation, sorting, and tags', () => {
  const options = createSkillHubBatchOptions()
  options.published = true
  options.recommended = true
  options.sortMode = 'sequence'
  options.sortStart = 100
  options.sortStep = 5
  options.verifiedMode = 'verified'
  options.tagMode = 'append'
  options.commonTags = ['Official']
  const entry = {
    index: 2,
    id: 'demo',
    name: 'Demo',
    description: '',
    version: '1.0.0',
    author: '',
    origin: '',
    originUrl: '',
    license: '',
    tags: ['Agent'],
    verified: false,
    evaluation: null,
    evaluationSpecified: false,
    testcases: null,
    testcasesFile: null,
  }
  const payload = buildSkillHubBatchPayload(
    entry,
    options,
    null,
    {
      url: 'https://example.com/demo.zip',
      object: 'tmp/demo.zip',
      checksum: 'sha256:demo',
    },
    null,
  )
  assert.equal(resolveSkillHubBatchSort(options, 2), 110)
  assert.equal(payload.published, true)
  assert.equal(payload.recommended, true)
  assert.equal(payload.verified, true)
  assert.deepEqual(payload.tags, ['Agent', 'Official'])
})

test('update retains missing optional resources by default', () => {
  const options = createSkillHubBatchOptions()
  const entry = {
    index: 0,
    id: 'demo',
    name: 'Demo',
    description: '',
    version: '1.0.0',
    author: '',
    origin: '',
    originUrl: '',
    license: '',
    tags: [],
    verified: false,
    evaluation: null,
    evaluationSpecified: false,
    testcases: null,
    testcasesFile: null,
  }
  const existing = {
    icon: 'https://example.com/icon.png',
    evaluation: { dimensions: {} },
    testcases: { slug: 'demo', testcases: [] },
  }
  const payload = buildSkillHubBatchPayload(
    entry,
    options,
    existing,
    {
      url: 'https://example.com/demo.zip',
      object: 'tmp/demo.zip',
      checksum: 'sha256:demo',
    },
    null,
  )
  assert.equal(payload.icon, existing.icon)
  assert.deepEqual(payload.evaluation, existing.evaluation)
  assert.deepEqual(payload.testcases, existing.testcases)
})

test('unsafe and URL references are rejected before upload', async () => {
  const directory = await parseSkillHubBatchDirectory([
    selectedFile(
      'manifest.json',
      JSON.stringify([
        {
          id: 'demo',
          name: 'Demo',
          zip: 'https://example.com/demo.zip',
        },
      ]),
    ),
  ])
  assert.match(
    directory.entries[0].errors.map((error) => error.code).join(';'),
    /local paths/,
  )
})

test('result summary keeps ambiguous commits visible', () => {
  assert.deepEqual(
    summarizeSkillHubBatchResults([
      { status: 'success' },
      { status: 'unknown' },
      { status: 'failed' },
    ]),
    {
      success: 1,
      skipped: 0,
      failed: 1,
      cancelled: 0,
      unknown: 1,
    },
  )
})

test('empty manifests are rejected before upload', async () => {
  await assert.rejects(
    parseSkillHubBatchDirectory([
      selectedFile('manifest.json', JSON.stringify([])),
    ]),
    /at least one skill/,
  )
})
