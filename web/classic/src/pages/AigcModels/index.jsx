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

import React, { useCallback, useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import {
  Button,
  Empty,
  Input,
  Modal,
  Pagination,
  Select,
  Space,
  Spin,
  Table,
  Tabs,
  Tag,
  TagInput,
  TextArea,
  Typography,
} from '@douyinfe/semi-ui';
import {
  CheckCircle2,
  CircleOff,
  FileText,
  Image,
  Music2,
  Pencil,
  Plus,
  RefreshCw,
  Rocket,
  Trash2,
  Video,
} from 'lucide-react';
import { API, showError, showSuccess, timestamp2string } from '../../helpers';
import { useIsMobile } from '../../hooks/common/useIsMobile';
import CapabilityEditor from './CapabilityEditor';
import {
  configTemplate,
  profileConfigAssignments,
  syncMusicModelCapabilities,
} from './capabilityConfig';

const { Text, Title } = Typography;

const MODEL_TYPES = [
  { value: 'text', label: '对话', icon: FileText, color: 'blue' },
  { value: 'image', label: '图片', icon: Image, color: 'green' },
  { value: 'video', label: '视频', icon: Video, color: 'violet' },
  { value: 'music', label: '音乐', icon: Music2, color: 'orange' },
];

const STATUS_META = {
  0: { label: '草稿', color: 'grey' },
  1: { label: '已发布', color: 'green' },
  2: { label: '已停用', color: 'red' },
};

const EMPTY_PROFILE = {
  public_model_id: '',
  display_name: '',
  model_type: 'text',
  description: '',
  groups: ['default'],
  config_version: 0,
};

function upstreamIDs(profile) {
  try {
    return [
      ...new Set(
        profileConfigAssignments(profile.model_type, profile.config)
          .map((item) => item.upstream)
          .filter(Boolean),
      ),
    ];
  } catch {
    return [];
  }
}

function apiMessage(response, fallback) {
  return response?.data?.message || response?.data?.error?.message || fallback;
}

function AigcProfileEditor({
  visible,
  profile,
  upstreamModels,
  onCancel,
  onSaved,
}) {
  const { t } = useTranslation();
  const isMobile = useIsMobile();
  const editing = Boolean(profile?.id);
  const [form, setForm] = useState(EMPTY_PROFILE);
  const [config, setConfig] = useState(() => configTemplate('text'));
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    if (!visible) return;
    const next = profile
      ? { ...EMPTY_PROFILE, ...profile }
      : { ...EMPTY_PROFILE };
    const nextConfig = profile?.config || configTemplate(next.model_type);
    setForm({ ...next, groups: next.groups || [] });
    setConfig(structuredClone(nextConfig));
  }, [profile, visible]);

  const assignments = useMemo(
    () => profileConfigAssignments(form.model_type, config),
    [form.model_type, config],
  );
  const availableUpstreamModels = Array.isArray(upstreamModels)
    ? upstreamModels
    : [];

  const updateAssignment = (mode, upstream) => {
    const next = structuredClone(config);
    if (form.model_type === 'text') {
      next.text.upstream_model_id = upstream;
    } else if (form.model_type === 'music') {
      setConfig(syncMusicModelCapabilities(next, upstream));
      return;
    } else {
      next[form.model_type].modes[mode].upstream_model_id = upstream;
    }
    setConfig(next);
  };

  const changeType = (modelType) => {
    setForm((current) => ({ ...current, model_type: modelType }));
    setConfig(configTemplate(modelType));
  };

  const save = async () => {
    const publicModelID = String(form.public_model_id || '').trim();
    const displayName = String(form.display_name || '').trim();
    if (!publicModelID || !displayName) {
      showError(t('请填写公共模型 ID 和显示名称'));
      return;
    }
    setSaving(true);
    try {
      const payload = {
        public_model_id: publicModelID,
        display_name: displayName,
        model_type: form.model_type,
        description: String(form.description || '').trim(),
        groups: Array.isArray(form.groups) ? form.groups : [],
        config,
        ...(editing ? { config_version: form.config_version } : {}),
      };
      const response = editing
        ? await API.put(`/api/aigc/models/${profile.id}`, payload)
        : await API.post('/api/aigc/models', payload);
      if (!response.data.success) {
        showError(apiMessage(response, t('保存失败')));
        return;
      }
      showSuccess(editing ? t('AIGC 模型已更新') : t('AIGC 模型已创建'));
      onSaved();
    } catch (error) {
      showError(
        error.response?.data?.message || error.message || t('保存失败'),
      );
    } finally {
      setSaving(false);
    }
  };

  return (
    <Modal
      title={editing ? t('编辑 AIGC 模型') : t('创建 AIGC 模型')}
      visible={visible}
      width={isMobile ? 'calc(100vw - 16px)' : 920}
      onCancel={onCancel}
      onOk={save}
      confirmLoading={saving}
      okText={t('保存')}
      cancelText={t('取消')}
      bodyStyle={{ maxHeight: '72vh', overflowY: 'auto' }}
    >
      <div className='grid grid-cols-1 gap-4 md:grid-cols-2'>
        <label className='flex flex-col gap-2'>
          <Text strong>{t('公共模型 ID')}</Text>
          <Input
            value={form.public_model_id}
            disabled={editing}
            placeholder='company/model-name'
            onChange={(value) =>
              setForm((current) => ({ ...current, public_model_id: value }))
            }
          />
        </label>
        <label className='flex flex-col gap-2'>
          <Text strong>{t('显示名称')}</Text>
          <Input
            value={form.display_name}
            onChange={(value) =>
              setForm((current) => ({ ...current, display_name: value }))
            }
          />
        </label>
        <label className='flex flex-col gap-2'>
          <Text strong>{t('模型类型')}</Text>
          <Select
            value={form.model_type}
            disabled={editing}
            onChange={changeType}
          >
            {MODEL_TYPES.map((type) => (
              <Select.Option key={type.value} value={type.value}>
                {t(type.label)}
              </Select.Option>
            ))}
          </Select>
        </label>
        <label className='flex flex-col gap-2'>
          <Text strong>{t('可用分组')}</Text>
          <TagInput
            value={form.groups}
            placeholder={t('输入分组后回车')}
            onChange={(groups) =>
              setForm((current) => ({ ...current, groups }))
            }
          />
        </label>
      </div>

      <label className='mt-4 flex flex-col gap-2'>
        <Text strong>{t('描述')}</Text>
        <TextArea
          value={form.description}
          autosize={{ minRows: 2, maxRows: 4 }}
          onChange={(value) =>
            setForm((current) => ({ ...current, description: value }))
          }
        />
      </label>

      <div className='mt-6 border-t border-semi-color-border pt-5'>
        <div className='mb-3 flex items-center justify-between gap-3'>
          <div>
            <Text strong>{t('模型分配')}</Text>
            <div>
              <Text type='tertiary' size='small'>
                {t('每个业务模式选择实际执行的上游模型')}
              </Text>
            </div>
          </div>
          <Tag color='blue'>
            {assignments.length} {t('个路由')}
          </Tag>
        </div>
        {assignments.length ? (
          <div className='grid grid-cols-1 gap-3 md:grid-cols-2'>
            {assignments.map((assignment) => (
              <label key={assignment.mode} className='flex flex-col gap-2'>
                <Text code>{assignment.mode}</Text>
                <Select
                  value={assignment.upstream || undefined}
                  filter
                  remote={false}
                  showClear
                  placeholder={t('选择上游模型')}
                  style={{ width: '100%' }}
                  onChange={(value) =>
                    updateAssignment(assignment.mode, value || '')
                  }
                >
                  {availableUpstreamModels.map((model) => (
                    <Select.Option key={model.id} value={model.id}>
                      {model.id} ({model.channel_count} {t('个渠道')})
                    </Select.Option>
                  ))}
                </Select>
              </label>
            ))}
          </div>
        ) : (
          <Text type='danger'>{t('当前配置没有可分配的业务模式')}</Text>
        )}
      </div>

      <div className='mt-6 border-t border-semi-color-border pt-5'>
        <div className='mb-4'>
          <div>
            <Text strong>{t('能力配置')}</Text>
            <div>
              <Text type='tertiary' size='small'>
                {t('定义模式、输入限制和输出规格')}
              </Text>
            </div>
          </div>
        </div>
        <CapabilityEditor
          modelType={form.model_type}
          config={config}
          upstreamModels={availableUpstreamModels}
          onChange={setConfig}
        />
      </div>
    </Modal>
  );
}

export default function AigcModelsPage() {
  const { t } = useTranslation();
  const [items, setItems] = useState([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(20);
  const [modelType, setModelType] = useState('');
  const [status, setStatus] = useState('');
  const [loading, setLoading] = useState(false);
  const [actingID, setActingID] = useState(null);
  const [editorVisible, setEditorVisible] = useState(false);
  const [editing, setEditing] = useState(null);
  const [upstreamModels, setUpstreamModels] = useState([]);

  const loadProfiles = useCallback(async () => {
    setLoading(true);
    try {
      const params = { p: page, page_size: pageSize };
      if (modelType) params.type = modelType;
      if (status !== '') params.status = status;
      const response = await API.get('/api/aigc/models', { params });
      if (!response.data.success) {
        showError(apiMessage(response, t('获取 AIGC 模型失败')));
        return;
      }
      setItems(response.data.data?.items || []);
      setTotal(response.data.data?.total || 0);
    } catch (error) {
      showError(
        error.response?.data?.message ||
          error.message ||
          t('获取 AIGC 模型失败'),
      );
    } finally {
      setLoading(false);
    }
  }, [modelType, page, pageSize, status, t]);

  const loadUpstreams = useCallback(async () => {
    try {
      const response = await API.get('/api/aigc/upstream-models');
      if (response.data.success) {
        setUpstreamModels(
          Array.isArray(response.data.data) ? response.data.data : [],
        );
      }
    } catch (error) {
      showError(
        error.response?.data?.message || error.message || t('获取上游模型失败'),
      );
    }
  }, [t]);

  useEffect(() => {
    loadProfiles();
  }, [loadProfiles]);
  useEffect(() => {
    loadUpstreams();
  }, [loadUpstreams]);

  const runAction = async (profile, action) => {
    setActingID(profile.id);
    try {
      let response;
      if (action === 'validate') {
        response = await API.post(`/api/aigc/models/${profile.id}/validate`);
      } else if (action === 'delete') {
        response = await API.delete(`/api/aigc/models/${profile.id}`, {
          data: { config_version: profile.config_version },
        });
      } else {
        response = await API.post(`/api/aigc/models/${profile.id}/${action}`, {
          config_version: profile.config_version,
        });
      }
      if (!response.data.success) {
        showError(apiMessage(response, t('操作失败')));
        return;
      }
      const success = {
        validate: t('配置校验通过'),
        publish: t('模型已发布'),
        disable: t('模型已停用'),
        delete: t('草稿已删除'),
      };
      showSuccess(success[action]);
      loadProfiles();
    } catch (error) {
      showError(
        error.response?.data?.message || error.message || t('操作失败'),
      );
    } finally {
      setActingID(null);
    }
  };

  const confirmAction = (profile, action) => {
    const copy = {
      publish: [t('发布模型'), t('发布后该模型会进入 AIGC 公共目录。')],
      disable: [t('停用模型'), t('停用后前端将不能再创建该模型的新任务。')],
      delete: [t('删除草稿'), t('该操作不可撤销。')],
    };
    Modal.confirm({
      title: copy[action][0],
      content: copy[action][1],
      okType: action === 'delete' ? 'danger' : 'primary',
      onOk: () => runAction(profile, action),
    });
  };

  const columns = useMemo(
    () => [
      {
        title: t('模型'),
        dataIndex: 'public_model_id',
        render: (_, record) => (
          <div className='min-w-[210px]'>
            <Text strong>{record.display_name}</Text>
            <div>
              <Text type='tertiary' size='small' copyable>
                {record.public_model_id}
              </Text>
            </div>
          </div>
        ),
      },
      {
        title: t('类型'),
        dataIndex: 'model_type',
        width: 100,
        render: (value) => {
          const meta =
            MODEL_TYPES.find((item) => item.value === value) || MODEL_TYPES[0];
          const Icon = meta.icon;
          return (
            <Tag color={meta.color} prefixIcon={<Icon size={14} />}>
              {t(meta.label)}
            </Tag>
          );
        },
      },
      {
        title: t('状态'),
        dataIndex: 'status',
        width: 100,
        render: (value) => {
          const meta = STATUS_META[value] || STATUS_META[0];
          return <Tag color={meta.color}>{t(meta.label)}</Tag>;
        },
      },
      {
        title: t('模型分配'),
        width: 260,
        render: (_, record) => {
          const ids = upstreamIDs(record);
          return ids.length ? (
            <Space spacing={4} wrap>
              {ids.slice(0, 3).map((id) => (
                <Tag key={id}>{id}</Tag>
              ))}
              {ids.length > 3 ? <Tag>+{ids.length - 3}</Tag> : null}
            </Space>
          ) : (
            <Text type='danger'>{t('未分配')}</Text>
          );
        },
      },
      {
        title: t('分组'),
        dataIndex: 'groups',
        width: 150,
        render: (groups) => (
          <Space spacing={4} wrap>
            {(groups || []).map((group) => (
              <Tag key={group}>{group}</Tag>
            ))}
          </Space>
        ),
      },
      {
        title: t('版本'),
        dataIndex: 'config_version',
        width: 80,
        render: (value) => `v${value}`,
      },
      {
        title: t('更新时间'),
        dataIndex: 'updated_time',
        width: 170,
        render: (value) => timestamp2string(value),
      },
      {
        title: t('操作'),
        fixed: 'right',
        width: 300,
        render: (_, record) => (
          <Space spacing={4}>
            <Button
              icon={<Pencil size={15} />}
              onClick={() => {
                setEditing(record);
                setEditorVisible(true);
              }}
            >
              {t('编辑')}
            </Button>
            <Button
              loading={actingID === record.id}
              icon={<CheckCircle2 size={15} />}
              onClick={() => runAction(record, 'validate')}
            >
              {t('校验')}
            </Button>
            {record.status !== 1 ? (
              <Button
                type='primary'
                icon={<Rocket size={15} />}
                onClick={() => confirmAction(record, 'publish')}
              >
                {t('发布')}
              </Button>
            ) : null}
            {record.status === 1 ? (
              <Button
                icon={<CircleOff size={15} />}
                onClick={() => confirmAction(record, 'disable')}
              >
                {t('停用')}
              </Button>
            ) : null}
            {record.status === 0 ? (
              <Button
                type='danger'
                theme='borderless'
                icon={<Trash2 size={15} />}
                onClick={() => confirmAction(record, 'delete')}
                aria-label={t('删除')}
              />
            ) : null}
          </Space>
        ),
      },
    ],
    [actingID, loadProfiles, t],
  );

  return (
    <div className='mt-[60px] px-2 pb-8'>
      <div className='mb-5 flex flex-col justify-between gap-4 lg:flex-row lg:items-end'>
        <div>
          <Title heading={3}>{t('AIGC 模型管理')}</Title>
          <Text type='tertiary'>
            {t('统一管理对话、图片、视频和音乐模型的能力、分配与发布状态')}
          </Text>
        </div>
        <Space>
          <Button
            icon={<RefreshCw size={16} />}
            onClick={() => {
              loadProfiles();
              loadUpstreams();
            }}
          >
            {t('刷新')}
          </Button>
          <Button
            type='primary'
            icon={<Plus size={16} />}
            onClick={() => {
              setEditing(null);
              setEditorVisible(true);
            }}
          >
            {t('创建模型')}
          </Button>
        </Space>
      </div>

      <div className='mb-4 flex flex-col justify-between gap-3 md:flex-row md:items-center'>
        <Tabs
          type='button'
          activeKey={modelType || 'all'}
          onChange={(key) => {
            setModelType(key === 'all' ? '' : key);
            setPage(1);
          }}
        >
          <Tabs.TabPane tab={t('全部')} itemKey='all' />
          {MODEL_TYPES.map((type) => (
            <Tabs.TabPane
              key={type.value}
              tab={t(type.label)}
              itemKey={type.value}
            />
          ))}
        </Tabs>
        <Select
          value={status}
          style={{ width: 150 }}
          onChange={(value) => {
            setStatus(value);
            setPage(1);
          }}
        >
          <Select.Option value=''>{t('全部状态')}</Select.Option>
          <Select.Option value={0}>{t('草稿')}</Select.Option>
          <Select.Option value={1}>{t('已发布')}</Select.Option>
          <Select.Option value={2}>{t('已停用')}</Select.Option>
        </Select>
      </div>

      <Spin spinning={loading}>
        <Table
          rowKey='id'
          columns={columns}
          dataSource={items}
          pagination={false}
          scroll={{ x: 1380 }}
          empty={<Empty description={t('暂无 AIGC 模型')} />}
        />
      </Spin>
      {total > pageSize ? (
        <div className='mt-4 flex justify-end'>
          <Pagination
            currentPage={page}
            pageSize={pageSize}
            total={total}
            showSizeChanger
            pageSizeOpts={[10, 20, 50, 100]}
            onPageChange={setPage}
            onPageSizeChange={(value) => {
              setPageSize(value);
              setPage(1);
            }}
          />
        </div>
      ) : null}

      <AigcProfileEditor
        visible={editorVisible}
        profile={editing}
        upstreamModels={upstreamModels}
        onCancel={() => setEditorVisible(false)}
        onSaved={() => {
          setEditorVisible(false);
          loadProfiles();
        }}
      />
    </div>
  );
}
