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

import React from 'react';
import { useTranslation } from 'react-i18next';
import {
  Button,
  Checkbox,
  Collapse,
  Input,
  InputNumber,
  Select,
  Space,
  Switch,
  Tag,
  TextArea,
  Typography,
} from '@douyinfe/semi-ui';
import { Plus, Trash2 } from 'lucide-react';
import {
  combinationValue,
  IMAGE_COUNT_OPTIONS,
  IMAGE_MODES,
  IMAGE_SIZE_OPTIONS,
  normalizeIntegerValues,
  parseCombinationValue,
  setImageModes,
  setVideoModes,
  VIDEO_COMBINATION_OPTIONS,
  VIDEO_DURATION_OPTIONS,
  VIDEO_INPUT_ROLES,
  VIDEO_MODES,
  VIDEO_RATIO_OPTIONS,
  VIDEO_RESOLUTION_OPTIONS,
  videoOutputTemplate,
} from './capabilityConfig';

const { Text } = Typography;

const IMAGE_MODE_LABELS = {
  text_to_image: '文生图',
  image_edit: '图片编辑',
};

const VIDEO_MODE_LABELS = {
  text_to_video: '文生视频',
  first_frame: '首帧生视频',
  first_last_frame: '首尾帧生视频',
  reference: '参考素材生视频',
  video_extension: '视频延长',
  video_edit: '视频编辑',
};

const VIDEO_INPUT_LABELS = {
  general_reference_image: '参考图片',
  general_reference_video: '参考视频',
  general_reference_audio: '参考音频',
  first_frame: '首帧图片',
  last_frame: '尾帧图片',
  driving_audio: '驱动音频',
  first_clip: '起始视频',
  source_video: '源视频',
};

function options(values) {
  return values.map((value) => ({ label: String(value), value }));
}

function mergedOptions(presets, selected) {
  return options([...new Set([...presets, ...(selected || [])])]);
}

function Field({ label, children, hint }) {
  return (
    <label className='flex min-w-0 flex-col gap-2'>
      <Text strong>{label}</Text>
      {children}
      {hint ? (
        <Text type='tertiary' size='small'>
          {hint}
        </Text>
      ) : null}
    </label>
  );
}

function Section({ title, extra, children }) {
  return (
    <section className='border-t border-semi-color-border pt-4 first:border-t-0 first:pt-0'>
      <div className='mb-3 flex min-h-8 items-center justify-between gap-3'>
        <Text strong>{title}</Text>
        {extra}
      </div>
      {children}
    </section>
  );
}

function IntegerMultiSelect({ value, presets, onChange, placeholder }) {
  return (
    <Select
      multiple
      allowCreate
      filter
      showClear
      value={value || []}
      optionList={mergedOptions(presets, value)}
      placeholder={placeholder}
      style={{ width: '100%' }}
      onChange={(next) => onChange(normalizeIntegerValues(next))}
    />
  );
}

function StringMultiSelect({ value, presets, onChange, placeholder }) {
  return (
    <Select
      multiple
      allowCreate
      filter
      showClear
      value={value || []}
      optionList={mergedOptions(presets, value)}
      placeholder={placeholder}
      style={{ width: '100%' }}
      onChange={(next) => onChange(Array.isArray(next) ? next : [])}
    />
  );
}

function TextCapabilityEditor({ config, onChange }) {
  const { t } = useTranslation();
  const text = config.text;
  return (
    <div className='grid grid-cols-1 gap-4 md:grid-cols-2'>
      <Field label={t('最大输出 Token')} hint={t('填 0 表示不额外限制')}>
        <InputNumber
          min={0}
          value={text.max_output_tokens || 0}
          style={{ width: '100%' }}
          onChange={(value) => {
            const next = structuredClone(config);
            next.text.max_output_tokens = Number(value) || 0;
            onChange(next);
          }}
        />
      </Field>
    </div>
  );
}

function ImageCapabilityEditor({ config, onChange }) {
  const { t } = useTranslation();
  const image = config.image;
  const selectedModes = Object.keys(image.modes || {});

  const updateMode = (modeName, update) => {
    const next = structuredClone(config);
    update(next.image.modes[modeName]);
    onChange(next);
  };

  const updateOutput = (modeName, key, value) => {
    updateMode(modeName, (mode) => {
      mode.output[key] = value;
      if (key === 'sizes' && !value.includes(mode.output.default_size)) {
        mode.output.default_size = value[0] || '';
      }
      if (key === 'counts' && !value.includes(mode.output.default_count)) {
        mode.output.default_count = value[0] || 0;
      }
    });
  };

  return (
    <div className='flex flex-col gap-5'>
      <Section title={t('图片模式')}>
        <Select
          multiple
          value={selectedModes}
          optionList={IMAGE_MODES.map((mode) => ({
            value: mode,
            label: t(IMAGE_MODE_LABELS[mode]),
          }))}
          placeholder={t('至少选择一个图片模式')}
          style={{ width: '100%' }}
          onChange={(value) => onChange(setImageModes(config, value || []))}
        />
      </Section>

      {selectedModes.map((modeName) => {
        const mode = image.modes[modeName];
        const output = mode.output;
        return (
          <Section
            key={modeName}
            title={t(IMAGE_MODE_LABELS[modeName] || modeName)}
            extra={<Tag>{modeName}</Tag>}
          >
            {modeName === 'image_edit' ? (
              <div className='mb-4 grid grid-cols-1 gap-4 md:grid-cols-3'>
                <Field label={t('最少原图数')}>
                  <InputNumber
                    min={1}
                    value={mode.input?.min || 1}
                    style={{ width: '100%' }}
                    onChange={(value) =>
                      updateMode(modeName, (draft) => {
                        draft.input.min = Number(value) || 1;
                        if (draft.input.max < draft.input.min)
                          draft.input.max = draft.input.min;
                      })
                    }
                  />
                </Field>
                <Field label={t('最多原图数')}>
                  <InputNumber
                    min={mode.input?.min || 1}
                    value={mode.input?.max || 1}
                    style={{ width: '100%' }}
                    onChange={(value) =>
                      updateMode(modeName, (draft) => {
                        draft.input.max = Number(value) || draft.input.min;
                      })
                    }
                  />
                </Field>
                <Field label={t('允许的图片格式')}>
                  <StringMultiSelect
                    value={mode.input?.accept || []}
                    presets={['image/png', 'image/jpeg', 'image/webp']}
                    onChange={(value) =>
                      updateMode(modeName, (draft) => {
                        draft.input.accept = value;
                      })
                    }
                  />
                </Field>
              </div>
            ) : null}

            <div className='grid grid-cols-1 gap-4 md:grid-cols-2'>
              <Field
                label={t('可选图片尺寸')}
                hint={t('可选择预设，也可输入自定义像素尺寸')}
              >
                <StringMultiSelect
                  value={output.sizes}
                  presets={IMAGE_SIZE_OPTIONS}
                  placeholder='1024x1024'
                  onChange={(value) => updateOutput(modeName, 'sizes', value)}
                />
              </Field>
              <Field label={t('默认图片尺寸')}>
                <Select
                  value={output.default_size || undefined}
                  optionList={options(output.sizes || [])}
                  style={{ width: '100%' }}
                  onChange={(value) =>
                    updateOutput(modeName, 'default_size', value)
                  }
                />
              </Field>
              <Field label={t('可选生成数量')}>
                <IntegerMultiSelect
                  value={output.counts}
                  presets={IMAGE_COUNT_OPTIONS}
                  onChange={(value) => updateOutput(modeName, 'counts', value)}
                />
              </Field>
              <Field label={t('默认生成数量')}>
                <Select
                  value={output.default_count || undefined}
                  optionList={options(output.counts || [])}
                  style={{ width: '100%' }}
                  onChange={(value) =>
                    updateOutput(modeName, 'default_count', Number(value))
                  }
                />
              </Field>
            </div>
          </Section>
        );
      })}
    </div>
  );
}

function VideoInputEditor({ mode, onChange }) {
  const { t } = useTranslation();
  const inputs = mode.inputs || {};
  const combination = mode.combination || {};
  const enabledRoles = VIDEO_INPUT_ROLES.filter((role) => inputs[role]);

  const updateInputs = (update) => {
    const next = structuredClone(mode);
    if (!next.inputs) next.inputs = {};
    update(next.inputs);
    onChange(next);
  };

  return (
    <div className='flex flex-col gap-4'>
      <Field label={t('输入素材类型')}>
        <Select
          multiple
          value={enabledRoles}
          optionList={VIDEO_INPUT_ROLES.map((role) => ({
            value: role,
            label: t(VIDEO_INPUT_LABELS[role]),
          }))}
          style={{ width: '100%' }}
          onChange={(roles) =>
            updateInputs((draft) => {
              for (const role of VIDEO_INPUT_ROLES) {
                if (roles.includes(role)) {
                  if (!draft[role]) draft[role] = { min: 1, max: 1 };
                } else delete draft[role];
              }
            })
          }
        />
      </Field>

      {enabledRoles.length ? (
        <div className='grid grid-cols-1 gap-3 md:grid-cols-2'>
          {enabledRoles.map((role) => (
            <div
              key={role}
              className='grid grid-cols-[minmax(0,1fr)_88px_88px] items-end gap-2 border-b border-semi-color-border pb-3'
            >
              <Text>{t(VIDEO_INPUT_LABELS[role])}</Text>
              <Field label={t('最少')}>
                <InputNumber
                  min={0}
                  value={inputs[role].min}
                  style={{ width: '100%' }}
                  onChange={(value) =>
                    updateInputs((draft) => {
                      draft[role].min = Number(value) || 0;
                    })
                  }
                />
              </Field>
              <Field label={t('最多')}>
                <InputNumber
                  min={1}
                  value={inputs[role].max}
                  style={{ width: '100%' }}
                  onChange={(value) =>
                    updateInputs((draft) => {
                      draft[role].max = Number(value) || 1;
                    })
                  }
                />
              </Field>
            </div>
          ))}
        </div>
      ) : null}

      <div className='grid grid-cols-1 gap-4 md:grid-cols-3'>
        <Field label={t('组合策略')}>
          <Select
            value={combination.strategy || 'any'}
            optionList={[
              { value: 'any', label: t('任意组合') },
              { value: 'allowlist', label: t('仅允许指定组合') },
            ]}
            style={{ width: '100%' }}
            onChange={(value) => {
              const next = structuredClone(mode);
              if (!next.combination) next.combination = {};
              next.combination.strategy = value;
              onChange(next);
            }}
          />
        </Field>
        <Field label={t('素材总数下限')}>
          <InputNumber
            min={0}
            value={combination.min_total || 0}
            style={{ width: '100%' }}
            onChange={(value) => {
              const next = structuredClone(mode);
              if (!next.combination) next.combination = {};
              next.combination.min_total = Number(value) || 0;
              onChange(next);
            }}
          />
        </Field>
        <Field label={t('素材总数上限')}>
          <InputNumber
            min={0}
            value={combination.max_total || 0}
            style={{ width: '100%' }}
            onChange={(value) => {
              const next = structuredClone(mode);
              if (!next.combination) next.combination = {};
              next.combination.max_total = Number(value) || 0;
              onChange(next);
            }}
          />
        </Field>
      </div>

      {combination.strategy === 'allowlist' ? (
        <Field label={t('允许的素材组合')}>
          <Select
            multiple
            value={(combination.allowed_combinations || []).map(
              combinationValue,
            )}
            optionList={VIDEO_COMBINATION_OPTIONS.map((value) => ({
              value: combinationValue(value),
              label: value.map((item) => t(item)).join(' + '),
            }))}
            style={{ width: '100%' }}
            onChange={(values) => {
              const next = structuredClone(mode);
              if (!next.combination) next.combination = {};
              next.combination.allowed_combinations = values.map(
                parseCombinationValue,
              );
              onChange(next);
            }}
          />
        </Field>
      ) : null}

      <Space wrap>
        <Checkbox
          checked={combination.exclude_audio_from_total || false}
          onChange={(event) => {
            const next = structuredClone(mode);
            if (!next.combination) next.combination = {};
            next.combination.exclude_audio_from_total = event.target.checked;
            onChange(next);
          }}
        >
          {t('素材总数不计音频')}
        </Checkbox>
        <Field label={t('包含视频时最大输出时长')}>
          <InputNumber
            min={0}
            suffix={t('秒')}
            value={inputs.max_duration_with_video || 0}
            onChange={(value) =>
              updateInputs((draft) => {
                draft.max_duration_with_video = Number(value) || 0;
              })
            }
          />
        </Field>
      </Space>
    </div>
  );
}

function VideoOutputEditor({
  spec,
  modes,
  upstreamModels,
  onChange,
  onDelete,
}) {
  const { t } = useTranslation();
  const audio = spec.generate_audio || { supported: false, default: false };
  const update = (key, value) => onChange({ ...spec, [key]: value });

  return (
    <div className='border-t border-semi-color-border pt-4 first:border-t-0 first:pt-0'>
      <div className='mb-3 flex items-center justify-between gap-3'>
        <Input
          value={spec.id}
          prefix={t('规格 ID')}
          style={{ maxWidth: 300 }}
          onChange={(value) => update('id', value)}
        />
        <Button
          type='danger'
          theme='borderless'
          icon={<Trash2 size={16} />}
          aria-label={t('删除输出规格')}
          onClick={onDelete}
        />
      </div>
      <div className='grid grid-cols-1 gap-4 md:grid-cols-2'>
        <Field label={t('适用模式')}>
          <Select
            multiple
            value={spec.modes || []}
            optionList={modes.map((mode) => ({
              value: mode,
              label: t(VIDEO_MODE_LABELS[mode] || mode),
            }))}
            style={{ width: '100%' }}
            onChange={(value) => update('modes', value || [])}
          />
        </Field>
        <Field
          label={t('执行目标覆盖')}
          hint={t('留空时使用模式分配的上游模型')}
        >
          <Select
            showClear
            filter
            value={spec.target?.upstream_model_id || undefined}
            optionList={upstreamModels.map((model) => ({
              value: model.id,
              label: model.id,
            }))}
            style={{ width: '100%' }}
            onChange={(value) =>
              update('target', value ? { upstream_model_id: value } : undefined)
            }
          />
        </Field>
        <Field label={t('可选分辨率')}>
          <StringMultiSelect
            value={spec.resolutions}
            presets={VIDEO_RESOLUTION_OPTIONS}
            onChange={(value) => update('resolutions', value)}
          />
        </Field>
        <Field label={t('可选画面比例')}>
          <StringMultiSelect
            value={spec.aspect_ratios}
            presets={VIDEO_RATIO_OPTIONS}
            onChange={(value) => update('aspect_ratios', value)}
          />
        </Field>
        <Field label={t('可选时长')}>
          <IntegerMultiSelect
            value={spec.durations}
            presets={VIDEO_DURATION_OPTIONS}
            onChange={(value) => update('durations', value)}
          />
        </Field>
        <div className='grid grid-cols-1 gap-3 sm:grid-cols-3'>
          <Field label={t('支持音频')}>
            <Switch
              checked={audio.supported}
              onChange={(checked) =>
                update('generate_audio', {
                  ...audio,
                  supported: checked,
                  default: checked ? audio.default : false,
                })
              }
            />
          </Field>
          <Field label={t('默认生成音频')}>
            <Switch
              disabled={!audio.supported}
              checked={audio.default}
              onChange={(checked) =>
                update('generate_audio', { ...audio, default: checked })
              }
            />
          </Field>
          <Field label={t('允许用户切换')}>
            <Switch
              disabled={!audio.supported}
              checked={audio.configurable || false}
              onChange={(checked) =>
                update('generate_audio', { ...audio, configurable: checked })
              }
            />
          </Field>
        </div>
      </div>
    </div>
  );
}

function VideoCapabilityEditor({ config, upstreamModels, onChange }) {
  const { t } = useTranslation();
  const video = config.video;
  const modes = Object.keys(video.modes || {});

  const updateMode = (modeName, value) => {
    const next = structuredClone(config);
    next.video.modes[modeName] = value;
    onChange(next);
  };

  const updateSpec = (index, value) => {
    const next = structuredClone(config);
    next.video.output_specs[index] = value;
    onChange(next);
  };

  return (
    <div className='flex flex-col gap-5'>
      <Section title={t('视频模式')}>
        <Select
          multiple
          value={modes}
          optionList={VIDEO_MODES.map((mode) => ({
            value: mode,
            label: t(VIDEO_MODE_LABELS[mode]),
          }))}
          placeholder={t('至少选择一个视频模式')}
          style={{ width: '100%' }}
          onChange={(value) => onChange(setVideoModes(config, value || []))}
        />
      </Section>

      {modes.map((modeName) => (
        <Section
          key={modeName}
          title={t(VIDEO_MODE_LABELS[modeName] || modeName)}
          extra={<Tag>{modeName}</Tag>}
        >
          <VideoInputEditor
            mode={video.modes[modeName]}
            onChange={(value) => updateMode(modeName, value)}
          />
        </Section>
      ))}

      <Section
        title={t('输出规格')}
        extra={
          <Button
            icon={<Plus size={16} />}
            onClick={() => {
              const next = structuredClone(config);
              const spec = videoOutputTemplate(modes.slice(0, 1));
              spec.id = `spec-${next.video.output_specs.length + 1}`;
              next.video.output_specs.push(spec);
              onChange(next);
            }}
          >
            {t('添加规格')}
          </Button>
        }
      >
        <div className='flex flex-col gap-5'>
          {(video.output_specs || []).map((spec, index) => (
            <VideoOutputEditor
              key={`${index}-${spec.id}`}
              spec={spec}
              modes={modes}
              upstreamModels={upstreamModels}
              onChange={(value) => updateSpec(index, value)}
              onDelete={() => {
                const next = structuredClone(config);
                next.video.output_specs.splice(index, 1);
                onChange(next);
              }}
            />
          ))}
        </div>
      </Section>
    </div>
  );
}

function MusicCapabilityEditor({ config, onChange }) {
  const { t } = useTranslation();
  const mode = config.music.modes.text_to_music;
  const instrumental = mode.parameters.instrumental;
  const output = mode.output;

  const updateMode = (update) => {
    const next = structuredClone(config);
    update(next.music.modes.text_to_music);
    onChange(next);
  };

  return (
    <div className='grid grid-cols-1 gap-4 md:grid-cols-2'>
      <div className='grid grid-cols-2 gap-4'>
        <Field label={t('支持纯音乐')}>
          <Switch
            checked={instrumental.supported}
            onChange={(checked) =>
              updateMode((draft) => {
                draft.parameters.instrumental.supported = checked;
                if (!checked) draft.parameters.instrumental.default = false;
              })
            }
          />
        </Field>
        <Field label={t('默认纯音乐')}>
          <Switch
            disabled={!instrumental.supported}
            checked={instrumental.default}
            onChange={(checked) =>
              updateMode((draft) => {
                draft.parameters.instrumental.default = checked;
              })
            }
          />
        </Field>
      </div>
      <div className='grid grid-cols-2 gap-4'>
        <Field label={t('最少曲目数')}>
          <InputNumber
            min={1}
            value={output.min_tracks}
            style={{ width: '100%' }}
            onChange={(value) =>
              updateMode((draft) => {
                draft.output.min_tracks = Number(value) || 1;
                if (draft.output.max_tracks < draft.output.min_tracks)
                  draft.output.max_tracks = draft.output.min_tracks;
              })
            }
          />
        </Field>
        <Field label={t('最多曲目数')}>
          <InputNumber
            min={output.min_tracks}
            value={output.max_tracks}
            style={{ width: '100%' }}
            onChange={(value) =>
              updateMode((draft) => {
                draft.output.max_tracks = Number(value) || output.min_tracks;
              })
            }
          />
        </Field>
      </div>
    </div>
  );
}

export default function CapabilityEditor({
  modelType,
  config,
  upstreamModels,
  onChange,
}) {
  const { t } = useTranslation();
  let editor = null;
  if (modelType === 'text' && config.text) {
    editor = <TextCapabilityEditor config={config} onChange={onChange} />;
  } else if (modelType === 'image' && config.image) {
    editor = <ImageCapabilityEditor config={config} onChange={onChange} />;
  } else if (modelType === 'video' && config.video) {
    editor = (
      <VideoCapabilityEditor
        config={config}
        upstreamModels={upstreamModels}
        onChange={onChange}
      />
    );
  } else if (modelType === 'music' && config.music) {
    editor = <MusicCapabilityEditor config={config} onChange={onChange} />;
  }

  return (
    <div className='flex flex-col gap-5'>
      {editor}
      <Collapse>
        <Collapse.Panel header={t('高级配置预览')} itemKey='raw-config'>
          <TextArea
            readOnly
            value={JSON.stringify(config, null, 2)}
            autosize={{ minRows: 10, maxRows: 20 }}
            style={{
              fontFamily: 'ui-monospace, SFMono-Regular, Menlo, monospace',
            }}
          />
        </Collapse.Panel>
      </Collapse>
    </div>
  );
}
