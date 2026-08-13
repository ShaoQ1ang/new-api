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
  readSkillHubTestcasesFile,
  resolveSkillHubTestcases,
  resolveSkillHubBatchSort,
  splitSkillHubCommitItems,
  splitSkillHubBatchItems,
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

test('large transfers are split into bounded batches without losing order', () => {
  const items = Array.from({ length: 1000 }, (_, index) => index)
  const batches = splitSkillHubBatchItems(items)

  assert.equal(batches.length, 10)
  assert.ok(batches.every((batch) => batch.length === 100))
  assert.deepEqual(batches.flat(), items)
})

test('commit requests settle large batches incrementally', () => {
  const items = Array.from({ length: 12 }, (_, index) => ({ index }))
  const batches = splitSkillHubCommitItems(items)

  assert.deepEqual(
    batches.map((batch) => batch.length),
    [10, 2],
  )
  assert.deepEqual(batches.flat(), items)

  const byteLimited = splitSkillHubCommitItems(items.slice(0, 3), 100, 12)
  assert.deepEqual(
    byteLimited.map((batch) => batch.length),
    [1, 1, 1],
  )
})

test('directory parsing accepts one thousand skills and rejects larger manifests', async () => {
  const manifest = Array.from({ length: 1000 }, (_, index) => ({
    id: `skill-${index}`,
    name: `Skill ${index}`,
    zip: `./packages/skill-${index}.zip`,
  }))
  const files = [
    selectedFile('manifest.json', JSON.stringify(manifest)),
    ...manifest.map((item) =>
      selectedFile(item.zip.slice(2), zipBytes, 'application/zip'),
    ),
  ]

  const directory = await parseSkillHubBatchDirectory(files)
  assert.equal(directory.entries.length, 1000)
  assert.ok(directory.entries.every((entry) => entry.errors.length === 0))

  await assert.rejects(
    parseSkillHubBatchDirectory([
      selectedFile(
        'manifest.json',
        JSON.stringify([
          ...manifest,
          { id: 'skill-1000', name: 'Skill 1000', zip: './skill-1000.zip' },
        ]),
      ),
    ]),
    /at most 1000/,
  )
})

test('single test case upload reads the currently selected file', async () => {
  const tomato = new File(
    [
      JSON.stringify({
        slug: 'g113593',
        testcases: [
          {
            id: 1,
            question: '我想写一本悬疑推理的番茄小说，帮我开始吧',
            answer: '旧案例',
            sortOrder: 0,
          },
        ],
      }),
    ],
    'tomato.json',
    { type: 'application/json' },
  )
  const medicalAI = new File(
    [
      JSON.stringify({
        slug: 'g173482',
        testcases: [
          {
            id: 1,
            question:
              '我需要创建一篇关于“人工智能在医疗领域的应用”的文章，请帮我完成完整流程',
            answer: '新案例',
            sortOrder: 0,
          },
        ],
      }),
    ],
    '1.json',
    { type: 'application/json' },
  )

  const first = await readSkillHubTestcasesFile(tomato)
  const second = await readSkillHubTestcasesFile(medicalAI)

  assert.equal(first.slug, 'g113593')
  assert.equal(second.slug, 'g173482')
  assert.match(second.testcases[0].question, /人工智能在医疗领域/)
  assert.doesNotMatch(second.testcases[0].question, /番茄小说/)
})

test('pending test case upload overrides stale saved cases', () => {
  const saved = {
    slug: 'g113593',
    testcases: [
      { id: 1, question: '番茄小说 1', answer: '旧案例 1', sortOrder: 0 },
      { id: 2, question: '番茄小说 2', answer: '旧案例 2', sortOrder: 1 },
      { id: 3, question: '番茄小说 3', answer: '旧案例 3', sortOrder: 2 },
    ],
  }
  const uploaded = {
    slug: 'g173482',
    testcases: [
      {
        id: 1,
        question: '人工智能在医疗领域的应用',
        answer: '新案例',
        sortOrder: 0,
      },
    ],
  }

  const effective = resolveSkillHubTestcases(saved, {
    active: true,
    value: uploaded,
  })

  assert.equal(effective.slug, 'g173482')
  assert.equal(effective.testcases.length, 1)
})

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
