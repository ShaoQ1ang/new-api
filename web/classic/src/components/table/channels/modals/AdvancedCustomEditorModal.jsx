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
import { useTranslation } from 'react-i18next';
import {
  Banner,
  Button,
  Card,
  Input,
  Modal,
  Radio,
  RadioGroup,
  Select,
  Space,
  Tag,
  TextArea,
  Typography,
} from '@douyinfe/semi-ui';
import {
  IconChevronDown,
  IconChevronUp,
  IconDelete,
  IconPlus,
} from '@douyinfe/semi-icons';
import { showError } from '../../../../helpers';
import {
  ADVANCED_CUSTOM_AUTH_OPTIONS,
  ADVANCED_CUSTOM_CONVERTER_OPTIONS,
  ADVANCED_CUSTOM_INCOMING_PATH_OPTIONS,
  ADVANCED_CUSTOM_TEMPLATES,
  buildAdvancedCustomAuth,
  cloneAdvancedCustomConfig,
  createAdvancedCustomConfig,
  getAdvancedCustomConverterOptions,
  getAdvancedCustomDefaults,
  normalizeAdvancedCustomConfig,
  parseAdvancedCustomConfig,
  stringifyAdvancedCustomConfig,
  validateAdvancedCustomConfig,
} from './advancedCustom';

const { Text } = Typography;

const AdvancedCustomEditorModal = ({ visible, value, onCancel, onSave }) => {
  const { t } = useTranslation();
  const [mode, setMode] = useState('visual');
  const [template, setTemplate] = useState(ADVANCED_CUSTOM_TEMPLATES[0].value);
  const [config, setConfig] = useState(createAdvancedCustomConfig());
  const [jsonText, setJsonText] = useState('');
  const [jsonError, setJsonError] = useState('');

  useEffect(() => {
    if (!visible) return;
    const nextConfig =
      parseAdvancedCustomConfig(value) || createAdvancedCustomConfig();
    setConfig(nextConfig);
    setJsonText(stringifyAdvancedCustomConfig(nextConfig));
    setJsonError('');
    setMode('visual');
  }, [visible, value]);

  const routes = useMemo(
    () => normalizeAdvancedCustomConfig(config).advanced_routes,
    [config],
  );
  const validationError = useMemo(
    () => validateAdvancedCustomConfig(config),
    [config],
  );

  const replaceRoutes = (nextRoutes) =>
    setConfig({ advanced_routes: nextRoutes });
  const updateRoute = (index, patch) => {
    const nextRoutes = [...routes];
    nextRoutes[index] = { ...nextRoutes[index], ...patch };
    replaceRoutes(nextRoutes);
  };

  const addRoute = () => {
    replaceRoutes([
      ...routes,
      {
        incoming_path: '/v1/chat/completions',
        upstream_path: '/v1/chat/completions',
        converter: 'none',
      },
    ]);
  };

  const removeRoute = (index) => {
    replaceRoutes(routes.filter((_, routeIndex) => routeIndex !== index));
  };

  const moveRoute = (index, offset) => {
    const target = index + offset;
    if (target < 0 || target >= routes.length) return;
    const nextRoutes = [...routes];
    [nextRoutes[index], nextRoutes[target]] = [
      nextRoutes[target],
      nextRoutes[index],
    ];
    replaceRoutes(nextRoutes);
  };

  const parseJson = () => {
    const parsed = parseAdvancedCustomConfig(jsonText);
    if (!parsed) {
      setJsonError(t('Invalid JSON'));
      return null;
    }
    const error = validateAdvancedCustomConfig(parsed);
    if (error) {
      setJsonError(
        `${error.routeIndex === undefined ? '' : `${t('Route')} ${error.routeIndex + 1}: `}${t(error.message)}`,
      );
      return null;
    }
    setJsonError('');
    return parsed;
  };

  const switchMode = (nextMode) => {
    if (nextMode === mode) return;
    if (nextMode === 'json') {
      setJsonText(stringifyAdvancedCustomConfig(config));
      setJsonError('');
      setMode('json');
      return;
    }
    const parsed = parseJson();
    if (!parsed) return;
    setConfig(parsed);
    setMode('visual');
  };

  const applyTemplate = (append) => {
    const selected = ADVANCED_CUSTOM_TEMPLATES.find(
      (item) => item.value === template,
    );
    if (!selected) return;
    const templateRoutes = cloneAdvancedCustomConfig({
      advanced_routes: selected.routes,
    }).advanced_routes;
    let baseRoutes = [];
    if (append) {
      if (mode === 'json') {
        const parsed = parseJson();
        if (!parsed) return;
        baseRoutes = parsed.advanced_routes;
      } else {
        baseRoutes = routes;
      }
    }
    const nextConfig = { advanced_routes: [...baseRoutes, ...templateRoutes] };
    setConfig(nextConfig);
    setJsonText(stringifyAdvancedCustomConfig(nextConfig));
    setJsonError('');
  };

  const save = () => {
    const nextConfig = mode === 'json' ? parseJson() : config;
    if (!nextConfig) return;
    const error = validateAdvancedCustomConfig(nextConfig);
    if (error) {
      showError(
        `${error.routeIndex === undefined ? '' : `${t('Route')} ${error.routeIndex + 1}: `}${t(error.message)}`,
      );
      return;
    }
    onSave(stringifyAdvancedCustomConfig(nextConfig));
  };

  return (
    <Modal
      title={t('Advanced Custom Routes')}
      visible={visible}
      onCancel={onCancel}
      onOk={save}
      okText={t('Save changes')}
      cancelText={t('Cancel')}
      width={980}
      bodyStyle={{ maxHeight: '72vh', overflowY: 'auto' }}
      centered
    >
      <div className='space-y-4'>
        <div className='flex flex-wrap items-center gap-2'>
          <Text type='tertiary'>{t('Mode')}</Text>
          <RadioGroup
            value={mode}
            type='button'
            onChange={(event) => switchMode(event.target.value)}
          >
            <Radio value='visual'>{t('Visual')}</Radio>
            <Radio value='json'>{t('JSON Text')}</Radio>
          </RadioGroup>
          <Select
            value={template}
            optionList={ADVANCED_CUSTOM_TEMPLATES.map((item) => ({
              value: item.value,
              label: t(item.label),
            }))}
            onChange={setTemplate}
            style={{ minWidth: 260, flex: 1 }}
          />
          <Button onClick={() => applyTemplate(false)}>
            {t('Fill Template')}
          </Button>
          <Button theme='borderless' onClick={() => applyTemplate(true)}>
            {t('Append Template')}
          </Button>
        </div>

        {mode === 'json' ? (
          <div>
            <TextArea
              value={jsonText}
              onChange={(nextValue) => {
                setJsonText(nextValue);
                setJsonError('');
              }}
              autosize={{ minRows: 20, maxRows: 32 }}
              style={{ fontFamily: 'monospace' }}
            />
            {jsonError && <Text type='danger'>{jsonError}</Text>}
          </div>
        ) : (
          <>
            {validationError && (
              <Banner
                type='warning'
                closeIcon={null}
                description={`${
                  validationError.routeIndex === undefined
                    ? ''
                    : `${t('Route')} ${validationError.routeIndex + 1}: `
                }${t(validationError.message)}`}
              />
            )}
            {routes.map((currentRoute, index) => (
              <RouteEditor
                key={`${index}-${currentRoute.incoming_path}`}
                route={currentRoute}
                index={index}
                routeCount={routes.length}
                onChange={(patch) => updateRoute(index, patch)}
                onMove={(offset) => moveRoute(index, offset)}
                onRemove={() => removeRoute(index)}
              />
            ))}
            <Button icon={<IconPlus />} onClick={addRoute} block>
              {t('Add route')}
            </Button>
          </>
        )}
      </div>
    </Modal>
  );
};

const RouteEditor = ({
  route,
  index,
  routeCount,
  onChange,
  onMove,
  onRemove,
}) => {
  const { t } = useTranslation();
  const converterOptions = getAdvancedCustomConverterOptions(
    route.incoming_path,
  );
  const authMode = route.auth?.type || 'default';
  const models = route.models || [];

  const changeIncomingPath = (incomingPath) => {
    const converterStillAllowed = getAdvancedCustomConverterOptions(
      incomingPath,
    ).some((option) => option.value === route.converter);
    const converter = converterStillAllowed ? route.converter : 'none';
    const defaults = getAdvancedCustomDefaults(converter, incomingPath);
    onChange({
      incoming_path: incomingPath,
      upstream_path: defaults.upstream_path,
      converter,
      auth: defaults.auth,
      ...(incomingPath === '/v1/models' ? { models: [] } : {}),
    });
  };

  const changeConverter = (converter) => {
    const defaults = getAdvancedCustomDefaults(converter, route.incoming_path);
    onChange({ converter, ...defaults });
  };

  return (
    <Card
      title={
        <Space>
          <Text strong>{`${t('Route')} ${index + 1}`}</Text>
          {!models.length && route.incoming_path !== '/v1/models' && (
            <Tag color='blue'>{t('Fallback')}</Tag>
          )}
        </Space>
      }
      headerExtraContent={
        <Space spacing={4}>
          <Button
            icon={<IconChevronUp />}
            theme='borderless'
            disabled={index === 0}
            onClick={() => onMove(-1)}
          />
          <Button
            icon={<IconChevronDown />}
            theme='borderless'
            disabled={index === routeCount - 1}
            onClick={() => onMove(1)}
          />
          <Button
            icon={<IconDelete />}
            type='danger'
            theme='borderless'
            onClick={onRemove}
          />
        </Space>
      }
      bodyStyle={{ paddingTop: 12 }}
    >
      <div className='grid grid-cols-1 md:grid-cols-2 gap-3'>
        <Field label={t('Incoming path')}>
          <Select
            value={route.incoming_path}
            optionList={ADVANCED_CUSTOM_INCOMING_PATH_OPTIONS}
            onChange={changeIncomingPath}
            filter
            style={{ width: '100%' }}
          />
        </Field>
        <Field label={t('Upstream path')}>
          <Input
            value={route.upstream_path}
            onChange={(upstreamPath) =>
              onChange({ upstream_path: upstreamPath })
            }
          />
        </Field>
        <Field label={t('Converter')}>
          <Select
            value={route.converter || 'none'}
            optionList={converterOptions.map((option) => ({
              value: option.value,
              label: t(option.label),
            }))}
            onChange={changeConverter}
            style={{ width: '100%' }}
          />
        </Field>
        <Field label={t('Client model')}>
          <Input
            value={models.join(', ')}
            disabled={route.incoming_path === '/v1/models'}
            placeholder={t('Leave empty for fallback')}
            onChange={(value) =>
              onChange({
                models: [
                  ...new Set(
                    value
                      .split(',')
                      .map((item) => item.trim())
                      .filter(Boolean),
                  ),
                ],
              })
            }
          />
        </Field>
        <Field label={t('Auth')}>
          <Select
            value={authMode}
            optionList={ADVANCED_CUSTOM_AUTH_OPTIONS.map((option) => ({
              value: option.value,
              label: t(option.label),
            }))}
            onChange={(nextMode) =>
              onChange({ auth: buildAdvancedCustomAuth(nextMode, route.auth) })
            }
            style={{ width: '100%' }}
          />
        </Field>
        {(authMode === 'header' || authMode === 'query') && (
          <>
            <Field label={t('Auth name')}>
              <Input
                value={route.auth?.name || ''}
                onChange={(name) => onChange({ auth: { ...route.auth, name } })}
              />
            </Field>
            <Field label={t('Auth value')}>
              <Input
                value={route.auth?.value || ''}
                onChange={(authValue) =>
                  onChange({ auth: { ...route.auth, value: authValue } })
                }
              />
            </Field>
          </>
        )}
      </div>
      <Text type='tertiary' size='small' className='mt-3 block'>
        {t(
          'Use exact model names such as gpt-4o, or regex rules prefixed with re: such as re:^gemini-.',
        )}
      </Text>
    </Card>
  );
};

const Field = ({ label, children }) => (
  <label className='flex min-w-0 flex-col gap-1'>
    <Text size='small' strong>
      {label}
    </Text>
    {children}
  </label>
);

export default AdvancedCustomEditorModal;
