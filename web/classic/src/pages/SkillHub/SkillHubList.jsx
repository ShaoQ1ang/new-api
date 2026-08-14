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

import React, { useEffect, useMemo, useState } from 'react';
import {
  ChevronLeft,
  ChevronRight,
  Download,
  Plus,
  Search,
} from 'lucide-react';
import { useNavigate, useSearchParams } from 'react-router-dom';
import {
  Banner,
  Button,
  Card,
  Checkbox,
  Input,
  Modal,
  Select,
  Space,
  Spin,
  Tag,
  Typography,
} from '@douyinfe/semi-ui';
import { API, showError, showSuccess } from '../../helpers';
import { splitSkillHubBatchItems } from '../../../../shared/skill-hub-batch-import.mjs';
import BatchUploadModal from './BatchUploadModal';

const DEFAULT_PAGE_SIZE = 20;
const PAGE_SIZE_OPTIONS = [10, 20, 50, 100];
const EXPORT_PAGE_SIZE = 100;

const parsePositiveInt = (value, fallback) => {
  const parsed = Number(value);
  return Number.isInteger(parsed) && parsed > 0 ? parsed : fallback;
};

const parsePageSize = (value) => {
  const parsed = Number(value);
  return PAGE_SIZE_OPTIONS.includes(parsed) ? parsed : DEFAULT_PAGE_SIZE;
};

const getCompactPageNumbers = (currentPage, totalPages) => {
  if (totalPages <= 4) {
    return Array.from({ length: totalPages }, (_, index) => index + 1);
  }
  if (currentPage <= 2) return [1, 2, 'right-ellipsis', totalPages];
  if (currentPage >= totalPages - 1) {
    return [1, 'left-ellipsis', totalPages - 1, totalPages];
  }
  return [1, 'left-ellipsis', currentPage, 'right-ellipsis', totalPages];
};

const normalizeTags = (value) => {
  if (Array.isArray(value)) {
    return value.map((item) => String(item).trim()).filter(Boolean);
  }
  return String(value || '')
    .split(/[,，\n]/)
    .map((item) => item.trim())
    .filter(Boolean);
};

const isPublishedSkill = (skill) =>
  Boolean(skill?.published || skill?.status === 1);

const getStatusApiValue = (statuses) => {
  const values = (statuses || []).map((status) =>
    status === 'published' ? 1 : 0,
  );
  return values.length ? values.join(',') : undefined;
};

const SkillHubList = () => {
  const navigate = useNavigate();
  const [searchParams, setSearchParams] = useSearchParams();
  const keyword = searchParams.get('q') || '';
  const selectedTagIds = (searchParams.get('tags') || '')
    .split(',')
    .map(Number)
    .filter((id) => Number.isInteger(id) && id > 0);
  const recommendedOnly = searchParams.get('recommended') === '1';
  const statusParam = searchParams.get('status') || '';
  const selectedStatuses = statusParam
    .split(',')
    .filter((status) => ['published', 'draft'].includes(status));
  const page = parsePositiveInt(searchParams.get('page'), 1);
  const pageSize = parsePageSize(searchParams.get('pageSize'));
  const [keywordDraft, setKeywordDraft] = useState(keyword);
  const [jumpPageDraft, setJumpPageDraft] = useState('');
  const [skills, setSkills] = useState([]);
  const [tagOptions, setTagOptions] = useState([]);
  const [checkedIds, setCheckedIds] = useState([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(false);
  const [refreshKey, setRefreshKey] = useState(0);
  const [batchWorking, setBatchWorking] = useState(false);
  const [exportState, setExportState] = useState(null);

  const currentPageIds = useMemo(
    () => skills.map((skill) => skill.id),
    [skills],
  );
  const currentPageDisplayCount = total === 0 ? 0 : currentPageIds.length;
  const currentPageSelectedCount = currentPageIds.filter((id) =>
    checkedIds.includes(id),
  ).length;
  const currentPageFullySelected =
    currentPageIds.length > 0 &&
    currentPageSelectedCount === currentPageIds.length;
  const currentPagePartiallySelected =
    currentPageSelectedCount > 0 && !currentPageFullySelected;
  const allFilteredSelected = total > 0 && checkedIds.length === total;

  useEffect(() => {
    setKeywordDraft(keyword);
  }, [keyword]);

  const updateSearch = (next, clearSelection = false) => {
    if (clearSelection) setCheckedIds([]);
    const params = new URLSearchParams();
    if (next.q) params.set('q', next.q);
    if (next.tags?.length) params.set('tags', next.tags.join(','));
    if (next.statuses?.length) params.set('status', next.statuses.join(','));
    if (next.recommended) params.set('recommended', '1');
    if ((next.page || 1) > 1) params.set('page', String(next.page));
    if ((next.pageSize || DEFAULT_PAGE_SIZE) !== DEFAULT_PAGE_SIZE) {
      params.set('pageSize', String(next.pageSize));
    }
    setSearchParams(params);
  };

  const currentSearch = () => ({
    q: keyword,
    tags: selectedTagIds,
    statuses: selectedStatuses,
    recommended: recommendedOnly,
    page,
    pageSize,
  });

  useEffect(() => {
    let cancelled = false;
    const loadTags = async () => {
      try {
        const res = await API.get('/api/admin/skill-hub/tags', {
          params: { page_size: 500 },
        });
        if (!res.data.success) {
          showError(res.data.message || '标签加载失败');
          return;
        }
        if (!cancelled) setTagOptions(res.data.data?.items || []);
      } catch (error) {
        if (!cancelled) showError(error.message || '标签加载失败');
      }
    };
    loadTags();
    return () => {
      cancelled = true;
    };
  }, []);

  useEffect(() => {
    let cancelled = false;
    const loadSkills = async () => {
      setLoading(true);
      try {
        const params = {
          keyword: keyword || undefined,
          recommended: recommendedOnly || undefined,
          status: getStatusApiValue(selectedStatuses),
          p: page,
          page_size: pageSize,
        };
        const res = selectedTagIds.length
          ? await API.get('/api/admin/skill-hub/tags/skills', {
              params: { ...params, tag_ids: selectedTagIds.join(',') },
            })
          : await API.get('/api/admin/skill-hub/skills', { params });
        if (!res.data.success) {
          showError(res.data.message || 'Skill 列表加载失败');
          return;
        }
        if (cancelled) return;
        const items = res.data.data?.items || [];
        const nextTotal = res.data.data?.total || 0;
        const pageCount = Math.max(1, Math.ceil(nextTotal / pageSize));
        if (page > pageCount) {
          updateSearch({ ...currentSearch(), page: pageCount });
          return;
        }
        setSkills(items);
        setTotal(nextTotal);
      } catch (error) {
        if (!cancelled) showError(error.message || 'Skill 列表加载失败');
      } finally {
        if (!cancelled) setLoading(false);
      }
    };
    loadSkills();
    return () => {
      cancelled = true;
    };
  }, [
    keyword,
    searchParams.get('tags'),
    statusParam,
    recommendedOnly,
    page,
    pageSize,
    refreshKey,
  ]);

  const submitSearch = () => {
    const q = keywordDraft.trim();
    if (q === keyword) return;
    updateSearch({ ...currentSearch(), q, page: 1 }, true);
  };

  const changeTags = (values) => {
    const tags = (values || []).map(Number).filter(Number.isInteger);
    updateSearch({ ...currentSearch(), tags, page: 1 }, true);
  };

  const changeStatuses = (values) => {
    const statuses = (values || []).filter((value) =>
      ['published', 'draft'].includes(value),
    );
    if (statuses.join(',') === selectedStatuses.join(',')) return;
    updateSearch({ ...currentSearch(), statuses, page: 1 }, true);
  };

  const pageCount = Math.max(1, Math.ceil(total / pageSize));
  const pageNumbers = getCompactPageNumbers(page, pageCount);

  const goToPage = (nextPage) => {
    const normalizedPage = Math.min(Math.max(nextPage, 1), pageCount);
    setJumpPageDraft('');
    if (normalizedPage === page || loading) return;
    updateSearch({ ...currentSearch(), page: normalizedPage });
  };

  const submitJumpPage = () => {
    const nextPage = Number(jumpPageDraft);
    if (!Number.isInteger(nextPage) || nextPage < 1) return;
    goToPage(nextPage);
  };

  const openEditor = (skillId) => {
    const suffix = searchParams.toString();
    navigate(
      `/console/skill-hub/${encodeURIComponent(skillId)}${suffix ? `?${suffix}` : ''}`,
    );
  };

  const openCreate = () => {
    const suffix = searchParams.toString();
    navigate(`/console/skill-hub/new${suffix ? `?${suffix}` : ''}`);
  };

  const toggleCurrentPage = (checked) => {
    setCheckedIds((current) =>
      checked
        ? [...new Set([...current, ...currentPageIds])]
        : current.filter((id) => !currentPageIds.includes(id)),
    );
  };

  const fetchFilteredIds = async () => {
    const params = {
      keyword: keyword || undefined,
      recommended: recommendedOnly || undefined,
      status: getStatusApiValue(selectedStatuses),
      page_size: EXPORT_PAGE_SIZE,
    };
    const fetchPage = (nextPage) =>
      selectedTagIds.length
        ? API.get('/api/admin/skill-hub/tags/skills', {
            params: {
              ...params,
              p: nextPage,
              tag_ids: selectedTagIds.join(','),
            },
          })
        : API.get('/api/admin/skill-hub/skills', {
            params: { ...params, p: nextPage },
          });
    const firstResponse = await fetchPage(1);
    if (!firstResponse.data.success) {
      throw new Error(firstResponse.data.message || 'Skill 列表加载失败');
    }
    const firstItems = firstResponse.data.data?.items || [];
    const filteredTotal = firstResponse.data.data?.total || firstItems.length;
    const remainingResponses = await Promise.all(
      Array.from(
        {
          length: Math.max(0, Math.ceil(filteredTotal / EXPORT_PAGE_SIZE) - 1),
        },
        (_, index) => fetchPage(index + 2),
      ),
    );
    const failedResponse = remainingResponses.find(
      (response) => !response.data.success,
    );
    if (failedResponse) {
      throw new Error(failedResponse.data.message || 'Skill 列表加载失败');
    }
    return [
      ...firstItems,
      ...remainingResponses.flatMap(
        (response) => response.data.data?.items || [],
      ),
    ].map((skill) => skill.id);
  };

  const selectAllFiltered = async () => {
    setBatchWorking(true);
    try {
      setCheckedIds(await fetchFilteredIds());
    } catch (error) {
      showError(error.message || '全选筛选结果失败');
    } finally {
      setBatchWorking(false);
    }
  };

  const downloadSkillExportBatches = async (ids, mode) => {
    const batches = splitSkillHubBatchItems(ids);
    setExportState({ mode, current: 0, total: batches.length });
    for (let index = 0; index < batches.length; index += 1) {
      setExportState({ mode, current: index + 1, total: batches.length });
      const res = await API.post(
        '/api/admin/skill-hub/skills/batch-export',
        { ids: batches[index] },
        { responseType: 'blob' },
      );
      const url = URL.createObjectURL(res.data);
      const link = document.createElement('a');
      link.href = url;
      link.download =
        batches.length === 1
          ? 'skill-hub-export.zip'
          : `skill-hub-export-${String(index + 1).padStart(3, '0')}-of-${String(batches.length).padStart(3, '0')}.zip`;
      document.body.appendChild(link);
      link.click();
      link.remove();
      window.setTimeout(() => URL.revokeObjectURL(url), 1000);
    }
  };

  const exportSelected = async () => {
    if (!checkedIds.length) return;
    setBatchWorking(true);
    try {
      await downloadSkillExportBatches(checkedIds, 'selected');
      showSuccess(`已导出 ${checkedIds.length} 个 Skill`);
    } catch (error) {
      showError(error.message || '批量导出失败');
    } finally {
      setExportState(null);
      setBatchWorking(false);
    }
  };

  const exportAll = async () => {
    setBatchWorking(true);
    setExportState({ mode: 'all', current: 0, total: 0 });
    try {
      const firstResponse = await API.get('/api/admin/skill-hub/skills', {
        params: { p: 1, page_size: EXPORT_PAGE_SIZE },
      });
      if (!firstResponse.data.success) {
        throw new Error(firstResponse.data.message || 'Skill 列表加载失败');
      }
      const firstItems = firstResponse.data.data?.items || [];
      const allTotal = firstResponse.data.data?.total || firstItems.length;
      const remainingResponses = await Promise.all(
        Array.from(
          {
            length: Math.max(0, Math.ceil(allTotal / EXPORT_PAGE_SIZE) - 1),
          },
          (_, index) =>
            API.get('/api/admin/skill-hub/skills', {
              params: { p: index + 2, page_size: EXPORT_PAGE_SIZE },
            }),
        ),
      );
      const failedResponse = remainingResponses.find(
        (response) => !response.data.success,
      );
      if (failedResponse) {
        throw new Error(failedResponse.data.message || 'Skill 列表加载失败');
      }
      const ids = [
        ...firstItems,
        ...remainingResponses.flatMap(
          (response) => response.data.data?.items || [],
        ),
      ].map((skill) => skill.id);
      await downloadSkillExportBatches(ids, 'all');
      showSuccess(`已导出 ${ids.length} 个 Skill`);
    } catch (error) {
      showError(error.message || '全部导出失败');
    } finally {
      setExportState(null);
      setBatchWorking(false);
    }
  };

  const deleteSelected = () => {
    if (!checkedIds.length) return;
    Modal.confirm({
      title: '批量删除 Skill',
      content: `确认删除选中的 ${checkedIds.length} 个 Skill？`,
      okType: 'danger',
      onOk: async () => {
        setBatchWorking(true);
        try {
          const res = await API.post(
            '/api/admin/skill-hub/skills/batch-delete',
            { ids: checkedIds },
          );
          if (!res.data.success) {
            showError(res.data.message || '批量删除失败');
            return;
          }
          showSuccess(
            `已删除 ${res.data.data?.deleted || checkedIds.length} 个 Skill`,
          );
          setCheckedIds([]);
          setRefreshKey((current) => current + 1);
        } finally {
          setBatchWorking(false);
        }
      },
    });
  };

  return (
    <div className='px-4 py-6 pb-8'>
      <div className='mx-auto flex max-w-7xl flex-col gap-4'>
        <div className='flex flex-wrap items-center justify-between gap-3'>
          <div>
            <Typography.Title heading={3} className='!mb-1'>
              技能管理
            </Typography.Title>
            <Typography.Text type='tertiary'>
              管理可被本地连接器安装的
              Skill，列表支持搜索、标签筛选和跨页批量选择。
            </Typography.Text>
          </div>
          <Space wrap>
            <BatchUploadModal
              tagOptions={tagOptions}
              onComplete={() => setRefreshKey((current) => current + 1)}
            />
            <Button
              icon={<Download size={16} />}
              disabled={batchWorking || loading}
              onClick={exportAll}
            >
              {exportState?.mode === 'all' && exportState.total
                ? `正在导出第 ${exportState.current}/${exportState.total} 批`
                : '全部导出'}
            </Button>
            <Button
              type='primary'
              icon={<Plus size={16} />}
              onClick={openCreate}
            >
              新建
            </Button>
            <Button
              loading={loading}
              onClick={() => setRefreshKey((current) => current + 1)}
            >
              刷新
            </Button>
          </Space>
        </div>

        <Card>
          <div className='mb-4 flex flex-col gap-3'>
            <div className='grid gap-1.5 sm:grid-cols-[80px_minmax(0,560px)] sm:items-center'>
              <span className='text-sm font-medium'>搜索</span>
              <div className='flex min-w-0 gap-2'>
                <Input
                  prefix={<Search size={16} />}
                  placeholder='搜索 ID / 名称 / 标签'
                  value={keywordDraft}
                  onChange={setKeywordDraft}
                  onEnterPress={submitSearch}
                />
                <Button type='primary' onClick={submitSearch}>
                  搜索
                </Button>
              </div>
            </div>
            <div className='grid gap-1.5 sm:grid-cols-[80px_minmax(0,360px)] sm:items-center'>
              <span className='text-sm font-medium'>状态</span>
              <Select
                multiple
                value={selectedStatuses}
                onChange={changeStatuses}
                placeholder='全部状态'
                maxTagCount={2}
                style={{ width: '100%' }}
              >
                <Select.Option value='published'>已发布</Select.Option>
                <Select.Option value='draft'>草稿</Select.Option>
              </Select>
            </div>
            <div className='grid gap-1.5 sm:grid-cols-[80px_minmax(0,560px)] sm:items-center'>
              <span className='text-sm font-medium'>标签</span>
              <Select
                multiple
                filter
                value={selectedTagIds}
                onChange={changeTags}
                placeholder='全部标签'
                maxTagCount={2}
                style={{ width: '100%' }}
              >
                {tagOptions.map((tag) => (
                  <Select.Option key={tag.id} value={tag.id}>
                    {tag.name}
                  </Select.Option>
                ))}
              </Select>
            </div>
            <div className='grid gap-1.5 sm:grid-cols-[80px_auto] sm:items-center'>
              <span className='text-sm font-medium'>推荐</span>
              <div className='flex shrink-0 gap-1 justify-self-start rounded border border-semi-color-border p-1'>
                <Button
                  size='small'
                  type={recommendedOnly ? 'tertiary' : 'primary'}
                  onClick={() => {
                    if (recommendedOnly) {
                      updateSearch(
                        { ...currentSearch(), recommended: false, page: 1 },
                        true,
                      );
                    }
                  }}
                >
                  全部
                </Button>
                <Button
                  size='small'
                  type={recommendedOnly ? 'primary' : 'tertiary'}
                  onClick={() => {
                    if (!recommendedOnly) {
                      updateSearch(
                        { ...currentSearch(), recommended: true, page: 1 },
                        true,
                      );
                    }
                  }}
                >
                  推荐
                </Button>
              </div>
            </div>
          </div>

          <div className='mb-3 flex flex-wrap items-center gap-2 border-y border-semi-color-border py-2'>
            <Typography.Text type='tertiary'>
              已选择 {checkedIds.length} 项
            </Typography.Text>
            <Button
              size='small'
              icon={<Download size={14} />}
              disabled={!checkedIds.length || batchWorking}
              onClick={exportSelected}
            >
              {exportState?.mode === 'selected' && exportState.total
                ? `正在导出第 ${exportState.current}/${exportState.total} 批`
                : '导出选中'}
            </Button>
            <Button
              size='small'
              type='danger'
              disabled={!checkedIds.length || batchWorking}
              onClick={deleteSelected}
            >
              批量删除
            </Button>
          </div>

          <Banner
            type='info'
            closeIcon={null}
            description={
              <div className='flex flex-wrap items-center justify-between gap-2'>
                <span>
                  {currentPageFullySelected ? (
                    allFilteredSelected ? (
                      `已选择全部 ${total} 条筛选结果。`
                    ) : (
                      <>
                        已选择本页 {currentPageIds.length} 项。{' '}
                        <button
                          type='button'
                          className='font-medium text-semi-color-primary underline'
                          disabled={batchWorking}
                          onClick={selectAllFiltered}
                        >
                          选择全部 {total} 条筛选结果
                        </button>
                      </>
                    )
                  ) : (
                    `「全选本页」仅选择当前页 ${currentPageDisplayCount} 条，跨页勾选会累计。`
                  )}
                </span>
                {checkedIds.length > 0 && (
                  <Button
                    size='small'
                    type='tertiary'
                    onClick={() => setCheckedIds([])}
                  >
                    清空选择
                  </Button>
                )}
              </div>
            }
            className='mb-4'
          />

          <Spin spinning={loading}>
            <div className='overflow-x-auto rounded border border-semi-color-border'>
              <table className='w-full min-w-[940px] border-collapse text-sm'>
                <thead className='bg-semi-color-fill-0 text-left text-semi-color-text-2'>
                  <tr>
                    <th className='w-[190px] border-b border-semi-color-border px-4 py-3 font-medium'>
                      <div className='flex items-center gap-2 whitespace-nowrap'>
                        <Checkbox
                          checked={currentPageFullySelected}
                          indeterminate={currentPagePartiallySelected}
                          onChange={(event) =>
                            toggleCurrentPage(event.target.checked)
                          }
                        />
                        <button
                          type='button'
                          className='cursor-pointer rounded-sm bg-transparent text-left hover:text-semi-color-primary focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-semi-color-primary'
                          onClick={() =>
                            toggleCurrentPage(!currentPageFullySelected)
                          }
                        >
                          全选本页（{currentPageDisplayCount}）
                        </button>
                      </div>
                    </th>
                    <th className='border-b border-semi-color-border px-4 py-3 font-medium'>
                      Skill
                    </th>
                    <th className='w-[120px] border-b border-semi-color-border px-4 py-3 font-medium'>
                      版本
                    </th>
                    <th className='w-[150px] border-b border-semi-color-border px-4 py-3 font-medium'>
                      状态
                    </th>
                    <th className='w-[220px] border-b border-semi-color-border px-4 py-3 font-medium'>
                      标签
                    </th>
                    <th className='w-[90px] border-b border-semi-color-border px-4 py-3 text-right font-medium'>
                      操作
                    </th>
                  </tr>
                </thead>
                <tbody>
                  {skills.map((skill) => {
                    const tags = normalizeTags(skill.tags);
                    return (
                      <tr
                        key={skill.id}
                        className='border-b border-semi-color-border last:border-b-0 hover:bg-semi-color-fill-0'
                      >
                        <td className='px-4 py-3 align-middle'>
                          <Checkbox
                            checked={checkedIds.includes(skill.id)}
                            onChange={(event) =>
                              setCheckedIds((current) =>
                                event.target.checked
                                  ? [...new Set([...current, skill.id])]
                                  : current.filter((id) => id !== skill.id),
                              )
                            }
                          />
                        </td>
                        <td className='px-4 py-3 align-middle'>
                          <button
                            type='button'
                            className='max-w-[440px] text-left'
                            onClick={() => openEditor(skill.id)}
                          >
                            <div className='truncate font-semibold text-semi-color-primary hover:underline'>
                              {skill.name}
                            </div>
                            <div className='truncate text-xs text-semi-color-text-2'>
                              {skill.id}
                              {skill.author ? ` · ${skill.author}` : ''}
                              {skill.origin ? ` · ${skill.origin}` : ''}
                            </div>
                            <div className='mt-1 line-clamp-1 text-semi-color-text-1'>
                              {skill.description || '暂无描述'}
                            </div>
                          </button>
                        </td>
                        <td className='px-4 py-3 align-middle'>
                          {skill.version}
                        </td>
                        <td className='px-4 py-3 align-middle'>
                          <Space spacing={4} wrap>
                            <Tag
                              color={isPublishedSkill(skill) ? 'green' : 'grey'}
                            >
                              {isPublishedSkill(skill) ? '已发布' : '草稿'}
                            </Tag>
                            {skill.recommended && (
                              <Tag color='violet'>推荐</Tag>
                            )}
                          </Space>
                        </td>
                        <td className='px-4 py-3 align-middle'>
                          <Space spacing={4} wrap>
                            {tags.slice(0, 3).map((tag) => (
                              <Tag key={tag}>{tag}</Tag>
                            ))}
                            {tags.length > 3 && (
                              <Typography.Text type='tertiary'>
                                +{tags.length - 3}
                              </Typography.Text>
                            )}
                          </Space>
                        </td>
                        <td className='px-4 py-3 text-right align-middle'>
                          <Button
                            size='small'
                            type='tertiary'
                            onClick={() => openEditor(skill.id)}
                          >
                            编辑
                          </Button>
                        </td>
                      </tr>
                    );
                  })}
                  {!skills.length && (
                    <tr>
                      <td
                        colSpan={6}
                        className='h-32 text-center text-semi-color-text-2'
                      >
                        {loading ? '加载中…' : '暂无 Skill'}
                      </td>
                    </tr>
                  )}
                </tbody>
              </table>
            </div>
          </Spin>

          <div className='mt-4 flex flex-col gap-3 xl:flex-row xl:items-center xl:justify-between'>
            <Typography.Text type='tertiary'>
              共 {total} 条，第 {page} / {pageCount} 页
            </Typography.Text>
            <div className='flex flex-wrap items-center gap-3'>
              <div className='flex items-center gap-2'>
                <Typography.Text type='tertiary'>每页</Typography.Text>
                <Select
                  size='small'
                  value={pageSize}
                  style={{ width: 92 }}
                  onChange={(value) =>
                    updateSearch({
                      ...currentSearch(),
                      page: 1,
                      pageSize: Number(value),
                    })
                  }
                >
                  {PAGE_SIZE_OPTIONS.map((option) => (
                    <Select.Option key={option} value={option}>
                      {option} 条
                    </Select.Option>
                  ))}
                </Select>
              </div>

              <div className='flex items-center gap-1'>
                <Button
                  size='small'
                  type='tertiary'
                  icon={<ChevronLeft size={16} />}
                  aria-label='上一页'
                  disabled={page <= 1 || loading}
                  onClick={() => goToPage(page - 1)}
                />
                {pageNumbers.map((pageNumber) =>
                  typeof pageNumber === 'string' ? (
                    <Typography.Text
                      key={pageNumber}
                      type='tertiary'
                      className='px-1'
                    >
                      ...
                    </Typography.Text>
                  ) : (
                    <Button
                      key={pageNumber}
                      size='small'
                      type={pageNumber === page ? 'primary' : 'tertiary'}
                      aria-current={pageNumber === page ? 'page' : undefined}
                      aria-label={`前往第 ${pageNumber} 页`}
                      disabled={loading}
                      onClick={() => goToPage(pageNumber)}
                    >
                      {pageNumber}
                    </Button>
                  ),
                )}
                <Button
                  size='small'
                  type='tertiary'
                  icon={<ChevronRight size={16} />}
                  aria-label='下一页'
                  disabled={page >= pageCount || loading}
                  onClick={() => goToPage(page + 1)}
                />
              </div>

              <div className='flex items-center gap-2'>
                <Typography.Text type='tertiary'>跳至</Typography.Text>
                <Input
                  size='small'
                  type='number'
                  min={1}
                  max={pageCount}
                  value={jumpPageDraft}
                  placeholder={String(page)}
                  aria-label='跳转页码'
                  style={{ width: 62 }}
                  onChange={setJumpPageDraft}
                  onEnterPress={submitJumpPage}
                />
                <Typography.Text type='tertiary'>页</Typography.Text>
                <Button
                  size='small'
                  disabled={!jumpPageDraft || loading}
                  onClick={submitJumpPage}
                >
                  确定
                </Button>
              </div>
            </div>
          </div>
        </Card>
      </div>
    </div>
  );
};

export default SkillHubList;
