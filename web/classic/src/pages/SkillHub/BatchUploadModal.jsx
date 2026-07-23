/*
Copyright (C) 2025 QuantumNous

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

import React, { useMemo, useRef, useState } from 'react';
import { Button, Checkbox, Input, Modal, Space, Tag } from '@douyinfe/semi-ui';
import {
  createSkillHubBatchOptions,
  createSkillHubBatchReport,
  issueMessage,
  parseSkillHubBatchDirectory,
  resolveSkillHubBatchSort,
  validateSkillHubBatchOptions,
} from '../../../../shared/skill-hub-batch-import.mjs';
import { API, showError, showSuccess } from '../../helpers';

const commitChunkTargetBytes = 8 * 1024 * 1024;
const terminalStatuses = new Set([
  'success',
  'skipped',
  'failed',
  'cancelled',
  'unknown',
]);

const BatchUploadModal = ({ tagOptions, onComplete }) => {
  const [visible, setVisible] = useState(false);
  const [directory, setDirectory] = useState(null);
  const [options, setOptions] = useState(createSkillHubBatchOptions);
  const [commonTagsText, setCommonTagsText] = useState('');
  const [rows, setRows] = useState({});
  const [working, setWorking] = useState(false);
  const [parsing, setParsing] = useState(false);
  const [startedAt, setStartedAt] = useState('');
  const [finished, setFinished] = useState(false);
  const folderInputRef = useRef(null);
  const abortRef = useRef(null);

  const entries = directory?.entries || [];
  const localErrorCount = entries.reduce(
    (count, entry) => count + entry.errors.length,
    0,
  );
  const rowValues = useMemo(
    () => entries.map((entry) => rows[entry.index]).filter(Boolean),
    [entries, rows],
  );
  const terminalCount = rowValues.filter((row) =>
    terminalStatuses.has(row.status),
  ).length;
  const overallProgress = entries.length
    ? Math.round(
        rowValues.reduce((sum, row) => sum + row.progress, 0) / entries.length,
      )
    : 0;
  const retryIndexes = rowValues
    .filter((row) => ['failed', 'cancelled'].includes(row.status))
    .map((row) => row.index);
  const highRisk =
    options.published ||
    options.recommended ||
    options.verifiedMode === 'verified' ||
    (options.mode === 'update' &&
      [
        options.missingIcon,
        options.missingTestcases,
        options.missingEvaluation,
      ].includes('clear'));

  const patchRow = (index, patch) => {
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
    }));
  };

  const setFolderInput = (node) => {
    folderInputRef.current = node;
    node?.setAttribute('webkitdirectory', '');
  };

  const chooseDirectory = async (fileList) => {
    if (!fileList?.length) return;
    setParsing(true);
    try {
      const parsed = await parseSkillHubBatchDirectory(fileList);
      setDirectory(parsed);
      setRows(
        Object.fromEntries(
          parsed.entries.map((entry) => [
            entry.index,
            createRow(entry, entry.errors.length ? 'failed' : 'pending'),
          ]),
        ),
      );
      setStartedAt('');
      setFinished(false);
    } catch (error) {
      showError(issueMessage(error, interpolate));
      setDirectory(null);
      setRows({});
    } finally {
      setParsing(false);
      if (folderInputRef.current) folderInputRef.current.value = '';
    }
  };

  const startUpload = async (targetIndexes) => {
    if (!directory || working || localErrorCount > 0) return;
    try {
      validateSkillHubBatchOptions(options);
    } catch (error) {
      showError(issueMessage(error, interpolate));
      return;
    }

    const targets = targetIndexes?.length
      ? entries.filter((entry) => targetIndexes.includes(entry.index))
      : entries;
    if (!targets.length) return;

    const controller = new AbortController();
    abortRef.current = controller;
    setStartedAt(new Date().toISOString());
    setFinished(false);
    setWorking(true);
    for (const entry of targets) {
      patchRow(entry.index, {
        status: 'initializing',
        action: '',
        message: '',
        progress: 2,
      });
    }

    const cleanupTickets = new Set();
    let successCount = 0;
    try {
      const initRes = await API.post('/api/admin/skill-hub/batch-upload/init', {
        mode: options.mode,
        items: targets.map((entry) => ({
          index: entry.index,
          id: entry.id,
          version: entry.version,
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
      });
      const initPayload = initRes.data;
      if (!initPayload.success || !initPayload.data) {
        throw new Error(initPayload.message || '批量上传初始化失败');
      }

      const ready = [];
      for (const item of initPayload.data.items || []) {
        const entry = targets.find((target) => target.index === item.index);
        if (!entry) continue;
        if (item.status !== 'ready' || !item.zip) {
          patchRow(item.index, {
            status: item.status === 'skipped' ? 'skipped' : 'failed',
            action: item.action,
            message: item.message,
            progress: 100,
          });
          continue;
        }
        ready.push({ entry, zip: item.zip, icon: item.icon });
        cleanupTickets.add(item.zip.uploadTicket);
        if (item.icon?.uploadTicket) {
          cleanupTickets.add(item.icon.uploadTicket);
        }
      }

      const uploaded = await uploadReadyItems(
        ready,
        options.concurrency,
        controller,
        options.stopOnError,
        patchRow,
      );
      if (controller.signal.aborted) {
        setRows((current) => {
          const next = { ...current };
          for (const item of ready) {
            const row = current[item.entry.index];
            if (!row || !terminalStatuses.has(row.status)) {
              next[item.entry.index] = {
                ...(row || createRow(item.entry, 'pending')),
                status: 'cancelled',
                message: '批量上传已取消',
                progress: 100,
              };
            }
          }
          return next;
        });
      }

      if (uploaded.length && !controller.signal.aborted) {
        const commitItems = uploaded.map((item) => ({
          index: item.entry.index,
          skill: entryToCommitSkill(item.entry),
          zipUploadTicket: item.zip.uploadTicket,
          iconUploadTicket: item.icon?.uploadTicket || '',
        }));
        const chunks = splitCommitItems(commitItems);
        for (let chunkIndex = 0; chunkIndex < chunks.length; chunkIndex += 1) {
          if (controller.signal.aborted) {
            for (const item of chunks.slice(chunkIndex).flat()) {
              patchRow(item.index, {
                status: 'cancelled',
                message: '批量上传已取消',
                progress: 100,
              });
            }
            break;
          }
          const chunk = chunks[chunkIndex];
          for (const item of chunk) {
            patchRow(item.index, { status: 'committing', progress: 95 });
          }
          try {
            const commitRes = await API.post(
              '/api/admin/skill-hub/batch-upload/commit',
              {
                mode: options.mode,
                options: commitOptions(options),
                items: chunk,
              },
            );
            const commitPayload = commitRes.data;
            if (!commitPayload.success || !commitPayload.data) {
              const message = commitPayload.message || '批量提交失败';
              for (const item of chunks.slice(chunkIndex).flat()) {
                patchRow(item.index, {
                  status: 'failed',
                  message,
                  progress: 100,
                });
              }
              break;
            }
            const responseByIndex = new Map(
              (commitPayload.data.items || []).map((item) => [
                item.index,
                item,
              ]),
            );
            for (const item of chunk) {
              const result = responseByIndex.get(item.index);
              if (!result) {
                cleanupTickets.delete(item.zipUploadTicket);
                if (item.iconUploadTicket) {
                  cleanupTickets.delete(item.iconUploadTicket);
                }
                patchRow(item.index, {
                  status: 'unknown',
                  message: '提交响应不完整，请刷新列表确认后再操作',
                  progress: 100,
                });
                continue;
              }
              if (result.status === 'success') {
                cleanupTickets.delete(item.zipUploadTicket);
                if (item.iconUploadTicket) {
                  cleanupTickets.delete(item.iconUploadTicket);
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
              });
              if (result.status === 'success') successCount += 1;
            }
          } catch (error) {
            for (const item of chunk) {
              // 服务端可能已完成提交。此处不自动重试或清理，避免重复写入。
              cleanupTickets.delete(item.zipUploadTicket);
              if (item.iconUploadTicket) {
                cleanupTickets.delete(item.iconUploadTicket);
              }
              patchRow(item.index, {
                status: 'unknown',
                message:
                  error instanceof Error
                    ? error.message
                    : '未收到批量提交响应，请刷新列表确认',
                progress: 100,
              });
            }
            for (const item of chunks.slice(chunkIndex + 1).flat()) {
              patchRow(item.index, {
                status: controller.signal.aborted ? 'cancelled' : 'failed',
                message: controller.signal.aborted
                  ? '批量上传已取消'
                  : '批量提交未执行',
                progress: 100,
              });
            }
            break;
          }
        }
      }
    } catch (error) {
      const message =
        error instanceof Error ? error.message : '批量上传启动失败';
      setRows((current) => {
        const next = { ...current };
        for (const entry of targets) {
          const row = current[entry.index];
          if (row && terminalStatuses.has(row.status)) continue;
          next[entry.index] = {
            ...(row || createRow(entry, 'pending')),
            status: controller.signal.aborted ? 'cancelled' : 'failed',
            message,
            progress: 100,
          };
        }
        return next;
      });
      showError(message);
    } finally {
      if (cleanupTickets.size) {
        try {
          await API.post('/api/admin/skill-hub/batch-upload/discard', {
            uploadTickets: [...cleanupTickets],
          });
        } catch {
          showError('部分临时文件未能立即清理，将由 OSS 生命周期规则回收');
        }
      }
      abortRef.current = null;
      setWorking(false);
      setFinished(true);
      if (successCount > 0) {
        try {
          await onComplete();
        } catch {
          showError('上传已保存，但刷新技能列表失败');
        }
        showSuccess(`成功保存 ${successCount} 个 Skill`);
      }
    }
  };

  const downloadReport = () => {
    if (!directory || !startedAt) return;
    const report = createSkillHubBatchReport({
      directory,
      options,
      startedAt,
      items: entries.map((entry) => {
        const row = rows[entry.index] || createRow(entry, 'pending');
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
        };
      }),
    });
    const url = URL.createObjectURL(
      new Blob([`${JSON.stringify(report, null, 2)}\n`], {
        type: 'application/json',
      }),
    );
    const link = document.createElement('a');
    link.href = url;
    link.download = 'skill-hub-batch-upload-report.json';
    link.click();
    URL.revokeObjectURL(url);
  };

  return (
    <>
      <Button onClick={() => setVisible(true)}>批量上传</Button>
      <Modal
        title='批量上传 Skill Hub 文件夹'
        visible={visible}
        width='min(1120px, calc(100vw - 32px))'
        closable={!working}
        maskClosable={!working}
        onCancel={() => {
          if (!working) setVisible(false);
        }}
        footer={
          <Space wrap>
            {working ? (
              <Button type='danger' onClick={() => abortRef.current?.abort()}>
                取消上传
              </Button>
            ) : (
              <Button onClick={() => setVisible(false)}>关闭</Button>
            )}
            {finished && startedAt ? (
              <Button onClick={downloadReport}>下载报告</Button>
            ) : null}
            {finished && retryIndexes.length ? (
              <Button onClick={() => startUpload(retryIndexes)}>
                重试失败项
              </Button>
            ) : null}
            <Button
              type='primary'
              disabled={!directory || localErrorCount > 0 || working || parsing}
              onClick={() => startUpload()}
            >
              开始批量上传
            </Button>
          </Space>
        }
      >
        <div className='max-h-[calc(100vh-210px)] space-y-4 overflow-y-auto pr-2'>
          <input
            ref={setFolderInput}
            type='file'
            multiple
            className='hidden'
            onChange={(event) => chooseDirectory(event.target.files)}
          />
          <Space wrap>
            <Button
              disabled={working || parsing}
              onClick={() => folderInputRef.current?.click()}
            >
              {parsing ? '正在校验文件夹' : '选择批量文件夹'}
            </Button>
            {directory ? (
              <span className='text-sm text-semi-color-text-2'>
                {directory.rootName} · {entries.length} 个 Skill ·{' '}
                {directory.fileCount} 个文件
              </span>
            ) : null}
          </Space>

          {directory && localErrorCount > 0 ? (
            <div className='rounded border border-semi-color-danger bg-semi-color-danger-light-default p-3 text-sm text-semi-color-danger'>
              文件夹校验失败，请先修复 manifest 和关联文件中的全部错误。
            </div>
          ) : null}

          {directory ? (
            <>
              <div className='grid grid-cols-1 gap-3 rounded border border-semi-color-border p-3 md:grid-cols-2 xl:grid-cols-3'>
                <ConfigField label='冲突策略'>
                  <NativeSelect
                    value={options.mode}
                    disabled={working}
                    onChange={(mode) =>
                      setOptions((current) => ({ ...current, mode }))
                    }
                    options={[
                      ['skip', '跳过已有 Skill'],
                      ['update', '更新已有 Skill'],
                      ['fail', '已有 Skill 时报错'],
                    ]}
                  />
                </ConfigField>
                <ConfigField label='保存状态'>
                  <NativeSelect
                    value={String(options.published)}
                    disabled={working}
                    onChange={(value) =>
                      setOptions((current) => ({
                        ...current,
                        published: value === 'true',
                      }))
                    }
                    options={[
                      ['false', '全部保存为草稿'],
                      ['true', '全部发布'],
                    ]}
                  />
                </ConfigField>
                <ConfigField label='是否推荐'>
                  <NativeSelect
                    value={String(options.recommended)}
                    disabled={working}
                    onChange={(value) =>
                      setOptions((current) => ({
                        ...current,
                        recommended: value === 'true',
                      }))
                    }
                    options={[
                      ['false', '全部不推荐'],
                      ['true', '全部推荐'],
                    ]}
                  />
                </ConfigField>
                <ConfigField label='排序策略'>
                  <NativeSelect
                    value={options.sortMode}
                    disabled={working}
                    onChange={(sortMode) =>
                      setOptions((current) => ({ ...current, sortMode }))
                    }
                    options={[
                      ['fixed', '统一强制排序值'],
                      ['sequence', '生成递增排序值'],
                    ]}
                  />
                </ConfigField>
                {options.sortMode === 'fixed' ? (
                  <ConfigField label='统一排序序号'>
                    <NumberInput
                      value={options.fixedSort}
                      disabled={working}
                      onChange={(fixedSort) =>
                        setOptions((current) => ({ ...current, fixedSort }))
                      }
                    />
                  </ConfigField>
                ) : (
                  <>
                    <ConfigField label='排序起始值'>
                      <NumberInput
                        value={options.sortStart}
                        disabled={working}
                        onChange={(sortStart) =>
                          setOptions((current) => ({ ...current, sortStart }))
                        }
                      />
                    </ConfigField>
                    <ConfigField label='排序步长'>
                      <NumberInput
                        value={options.sortStep}
                        disabled={working}
                        onChange={(sortStep) =>
                          setOptions((current) => ({ ...current, sortStep }))
                        }
                      />
                    </ConfigField>
                  </>
                )}
                <ConfigField label='上传并发数（1–10）'>
                  <NumberInput
                    min={1}
                    max={10}
                    value={options.concurrency}
                    disabled={working}
                    onChange={(concurrency) =>
                      setOptions((current) => ({ ...current, concurrency }))
                    }
                  />
                </ConfigField>
                <ConfigField label='验证状态'>
                  <NativeSelect
                    value={options.verifiedMode}
                    disabled={working}
                    onChange={(verifiedMode) =>
                      setOptions((current) => ({ ...current, verifiedMode }))
                    }
                    options={[
                      ['manifest', '使用 manifest 配置'],
                      ['verified', '全部设为已验证'],
                      ['unverified', '全部设为未验证'],
                    ]}
                  />
                </ConfigField>
                <ConfigField label='公共标签策略'>
                  <NativeSelect
                    value={options.tagMode}
                    disabled={working}
                    onChange={(tagMode) =>
                      setOptions((current) => ({ ...current, tagMode }))
                    }
                    options={[
                      ['manifest', '使用 manifest 标签'],
                      ['append', '追加公共标签'],
                      ['replace', '用公共标签替换'],
                    ]}
                  />
                </ConfigField>
                {options.tagMode !== 'manifest' ? (
                  <ConfigField label='公共标签（逗号分隔）'>
                    <Input
                      value={commonTagsText}
                      disabled={working}
                      list='skill-hub-batch-tag-options'
                      onChange={(value) => {
                        setCommonTagsText(value);
                        setOptions((current) => ({
                          ...current,
                          commonTags: splitTags(value),
                        }));
                      }}
                    />
                    <datalist id='skill-hub-batch-tag-options'>
                      {(tagOptions || []).map((tag) => (
                        <option key={tag.id || tag.name} value={tag.name} />
                      ))}
                    </datalist>
                  </ConfigField>
                ) : null}
                <ConfigField label='统一来源'>
                  <div className='space-y-2'>
                    <Checkbox
                      checked={options.overrideOrigin}
                      disabled={working}
                      onChange={(event) =>
                        setOptions((current) => ({
                          ...current,
                          overrideOrigin: event.target.checked,
                        }))
                      }
                    >
                      替换 manifest 中的来源
                    </Checkbox>
                    {options.overrideOrigin ? (
                      <Input
                        maxLength={64}
                        value={options.origin}
                        disabled={working}
                        onChange={(origin) =>
                          setOptions((current) => ({ ...current, origin }))
                        }
                      />
                    ) : null}
                  </div>
                </ConfigField>
                {[
                  ['缺少图标时', 'missingIcon'],
                  ['缺少案例时', 'missingTestcases'],
                  ['缺少评测时', 'missingEvaluation'],
                ].map(([label, key]) => (
                  <ConfigField key={key} label={label}>
                    <NativeSelect
                      value={options[key]}
                      disabled={working}
                      onChange={(value) =>
                        setOptions((current) => ({
                          ...current,
                          [key]: value,
                        }))
                      }
                      options={[
                        ['retain', '更新时保留原值'],
                        ['clear', '更新时清空原值'],
                      ]}
                    />
                  </ConfigField>
                ))}
                <ConfigField label='错误处理'>
                  <Checkbox
                    checked={options.stopOnError}
                    disabled={working}
                    onChange={(event) =>
                      setOptions((current) => ({
                        ...current,
                        stopOnError: event.target.checked,
                      }))
                    }
                  >
                    首个上传错误后停止
                  </Checkbox>
                </ConfigField>
              </div>

              {highRisk ? (
                <div className='rounded border border-semi-color-warning bg-semi-color-warning-light-default p-3 text-sm'>
                  当前配置会批量发布、推荐、标记已验证，或清空已有资源。请在开始前再次核对。
                </div>
              ) : null}

              <div className='flex items-center justify-between text-sm'>
                <span>
                  已处理 {terminalCount} / {entries.length}
                </span>
                <span>总体进度 {overallProgress}%</span>
              </div>
              <div className='h-2 overflow-hidden rounded bg-semi-color-fill-1'>
                <div
                  className='h-full bg-semi-color-primary'
                  style={{ width: `${overallProgress}%` }}
                />
              </div>

              <div className='overflow-x-auto rounded border border-semi-color-border'>
                <table className='w-full min-w-[820px] text-left text-sm'>
                  <thead className='bg-semi-color-fill-0'>
                    <tr>
                      <th className='p-2'>#</th>
                      <th className='p-2'>Skill</th>
                      <th className='p-2'>ZIP</th>
                      <th className='p-2'>覆盖排序</th>
                      <th className='p-2'>状态</th>
                      <th className='p-2'>信息</th>
                    </tr>
                  </thead>
                  <tbody>
                    {entries.map((entry) => {
                      const row =
                        rows[entry.index] || createRow(entry, 'pending');
                      return (
                        <tr
                          key={`${entry.index}-${entry.id}`}
                          className='border-t border-semi-color-border'
                        >
                          <td className='p-2'>{entry.index + 1}</td>
                          <td className='p-2'>
                            <div>{entry.name || '—'}</div>
                            <div className='text-xs text-semi-color-text-2'>
                              {entry.id || '—'} · {entry.version || '—'}
                            </div>
                          </td>
                          <td className='max-w-[220px] truncate p-2'>
                            {entry.zipPath || '—'}
                          </td>
                          <td className='p-2'>
                            {resolveSkillHubBatchSort(options, entry.index)}
                          </td>
                          <td className='p-2'>
                            <StatusTag status={row.status} />
                            {row.progress > 0 && row.progress < 100
                              ? ` ${row.progress}%`
                              : ''}
                          </td>
                          <td className='max-w-[300px] p-2 text-xs'>
                            {entry.errors.length
                              ? entry.errors
                                  .map((error) =>
                                    issueMessage(error, interpolate),
                                  )
                                  .join('；')
                              : row.message || '—'}
                          </td>
                        </tr>
                      );
                    })}
                  </tbody>
                </table>
              </div>
            </>
          ) : (
            <div className='rounded border border-dashed border-semi-color-border p-8 text-center text-semi-color-text-2'>
              请选择包含根目录 manifest.json（或 manifest.jsonl）、packages、
              icons 和 testcases 的文件夹。
            </div>
          )}
        </div>
      </Modal>
    </>
  );
};

async function uploadReadyItems(
  ready,
  concurrency,
  controller,
  stopOnError,
  patchRow,
) {
  const uploaded = [];
  let cursor = 0;
  let halt = false;

  async function worker() {
    while (!halt && !controller.signal.aborted) {
      const currentIndex = cursor;
      cursor += 1;
      if (currentIndex >= ready.length) return;
      const item = ready[currentIndex];
      try {
        patchRow(item.entry.index, {
          status: 'uploading',
          action: '',
          message: '',
          progress: 10,
        });
        await putSignedObject(item.zip, item.entry.zipFile, {
          signal: controller.signal,
          onProgress: (percent) =>
            patchRow(item.entry.index, {
              progress: 10 + Math.round(percent * (item.icon ? 0.55 : 0.8)),
            }),
        });
        if (item.icon && item.entry.iconFile) {
          await putSignedObject(item.icon, item.entry.iconFile, {
            signal: controller.signal,
            onProgress: (percent) =>
              patchRow(item.entry.index, {
                progress: 65 + Math.round(percent * 0.25),
              }),
          });
        }
        uploaded.push(item);
        patchRow(item.entry.index, { progress: 90 });
      } catch (error) {
        const aborted =
          controller.signal.aborted ||
          (error instanceof DOMException && error.name === 'AbortError');
        patchRow(item.entry.index, {
          status: aborted ? 'cancelled' : 'failed',
          message: error instanceof Error ? error.message : '文件上传失败',
          progress: 100,
        });
        if (stopOnError && !aborted) {
          halt = true;
          controller.abort();
        }
      }
    }
  }

  await Promise.all(
    Array.from(
      { length: Math.min(Math.max(1, concurrency), ready.length || 1) },
      () => worker(),
    ),
  );
  return uploaded;
}

function putSignedObject(upload, file, { signal, onProgress }) {
  return new Promise((resolve, reject) => {
    const xhr = new XMLHttpRequest();
    const abort = () => xhr.abort();
    xhr.open(upload.uploadMethod || 'PUT', upload.uploadUrl);
    Object.entries(upload.uploadHeaders || {}).forEach(([key, value]) => {
      if (value) xhr.setRequestHeader(key, value);
    });
    xhr.upload.onprogress = (event) => {
      if (event.lengthComputable && event.total > 0) {
        onProgress?.(Math.round((event.loaded / event.total) * 100));
      }
    };
    xhr.onload = () => {
      signal?.removeEventListener('abort', abort);
      if (xhr.status >= 200 && xhr.status < 300) {
        resolve();
      } else {
        reject(new Error(`OSS 上传失败（HTTP ${xhr.status}）`));
      }
    };
    xhr.onerror = () => {
      signal?.removeEventListener('abort', abort);
      reject(new Error('OSS 上传连接失败'));
    };
    xhr.onabort = () => {
      signal?.removeEventListener('abort', abort);
      reject(new DOMException('上传已取消', 'AbortError'));
    };
    signal?.addEventListener('abort', abort, { once: true });
    if (signal?.aborted) {
      abort();
      return;
    }
    xhr.send(file);
  });
}

function entryToCommitSkill(entry) {
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
  };
}

function commitOptions(options) {
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
    overrideOrigin: options.overrideOrigin,
    origin: options.origin,
    missingIcon: options.missingIcon,
    missingTestcases: options.missingTestcases,
    missingEvaluation: options.missingEvaluation,
  };
}

function splitCommitItems(items) {
  const chunks = [];
  let current = [];
  let currentBytes = 0;
  for (const item of items) {
    const bytes = new TextEncoder().encode(JSON.stringify(item)).byteLength;
    if (current.length && currentBytes + bytes > commitChunkTargetBytes) {
      chunks.push(current);
      current = [];
      currentBytes = 0;
    }
    current.push(item);
    currentBytes += bytes;
  }
  if (current.length) chunks.push(current);
  return chunks;
}

function createRow(entry, status) {
  return {
    index: entry.index,
    id: entry.id,
    status,
    progress: terminalStatuses.has(status) ? 100 : 0,
    message: entry.errors.length ? entry.errors[0].code : '',
  };
}

function splitTags(value) {
  const seen = new Set();
  return String(value || '')
    .split(/[,，\n]/)
    .map((tag) => tag.trim())
    .filter((tag) => {
      const key = tag.toLowerCase();
      if (!tag || seen.has(key)) return false;
      seen.add(key);
      return true;
    });
}

function interpolate(key, params = {}) {
  return String(key).replace(/\{\{(\w+)\}\}/g, (_, name) =>
    String(params[name] ?? ''),
  );
}

function ConfigField({ label, children }) {
  return (
    <label className='flex min-w-0 flex-col gap-1 text-sm'>
      <span className='font-medium'>{label}</span>
      {children}
    </label>
  );
}

function NativeSelect({ value, options, onChange, disabled }) {
  return (
    <select
      className='h-8 w-full rounded border border-semi-color-border bg-semi-color-bg-0 px-2 text-sm'
      value={value}
      disabled={disabled}
      onChange={(event) => onChange(event.target.value)}
    >
      {options.map(([optionValue, label]) => (
        <option key={optionValue} value={optionValue}>
          {label}
        </option>
      ))}
    </select>
  );
}

function NumberInput({
  value,
  onChange,
  min = -2147483648,
  max = 2147483647,
  disabled,
}) {
  return (
    <Input
      type='number'
      min={min}
      max={max}
      value={String(value)}
      disabled={disabled}
      onChange={(nextValue) => onChange(Number(nextValue))}
    />
  );
}

function StatusTag({ status }) {
  const labels = {
    pending: '待处理',
    initializing: '初始化',
    uploading: '上传中',
    committing: '保存中',
    success: '成功',
    skipped: '已跳过',
    failed: '失败',
    cancelled: '已取消',
    unknown: '待确认',
  };
  const colors = {
    success: 'green',
    failed: 'red',
    cancelled: 'grey',
    unknown: 'orange',
    skipped: 'amber',
  };
  return <Tag color={colors[status] || 'blue'}>{labels[status] || status}</Tag>;
}

export default BatchUploadModal;
