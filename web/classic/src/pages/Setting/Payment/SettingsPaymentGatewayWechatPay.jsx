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
import React, { useCallback, useEffect, useRef, useState } from 'react';
import {
  Banner,
  Button,
  Col,
  Form,
  Popconfirm,
  Row,
  Spin,
  Tag,
} from '@douyinfe/semi-ui';
import { useTranslation } from 'react-i18next';
import { API, showError, showSuccess } from '../../../helpers';

const emptySettings = {
  enabled: false,
  app_id: '',
  mch_id: '',
  merchant_cert_serial_no: '',
  merchant_private_key: '',
  merchant_private_key_configured: false,
  api_v3_key: '',
  api_v3_key_configured: false,
  public_key_id: '',
  public_key: '',
  public_key_configured: false,
  notify_url: '',
  resolved_notify_url: '',
  min_topup: 1,
  max_topup: 4000,
  order_expire_minutes: 10,
  pending_order_count: 0,
  option_crypt_key_configured: false,
};

export default function SettingsPaymentGatewayWechatPay(props) {
  const { t } = useTranslation();
  const [inputs, setInputs] = useState(emptySettings);
  const [loading, setLoading] = useState(true);
  const [loaded, setLoaded] = useState(false);
  const formApiRef = useRef(null);
  const sectionTitle = props.hideSectionTitle
    ? undefined
    : t('微信支付 Native 设置');

  const loadSettings = useCallback(async () => {
    setLoading(true);
    setLoaded(false);
    try {
      const response = await API.get('/api/wechatpay/admin/settings');
      if (!response.data?.success || !response.data?.data) {
        showError(response.data?.message || t('获取微信支付配置失败'));
        return;
      }
      const next = { ...emptySettings, ...response.data.data };
      setInputs(next);
      formApiRef.current?.setValues(next);
      setLoaded(true);
    } catch (_error) {
      showError(t('获取微信支付配置失败'));
    } finally {
      setLoading(false);
    }
  }, [t]);

  useEffect(() => {
    void loadSettings();
  }, [loadSettings]);

  const handleChange = (values) =>
    setInputs((current) => ({ ...current, ...values }));

  const saveSettings = async () => {
    if (!loaded) return;
    setLoading(true);
    try {
      const payload = {
        enabled: !!inputs.enabled,
        app_id: inputs.app_id || '',
        mch_id: inputs.mch_id || '',
        merchant_cert_serial_no: inputs.merchant_cert_serial_no || '',
        public_key_id: inputs.public_key_id || '',
        notify_url: inputs.notify_url || '',
        min_topup: Number(inputs.min_topup),
        max_topup: Number(inputs.max_topup),
        order_expire_minutes: Number(inputs.order_expire_minutes),
      };
      if ((inputs.merchant_private_key || '').trim()) {
        payload.merchant_private_key = inputs.merchant_private_key;
      }
      if (inputs.api_v3_key) payload.api_v3_key = inputs.api_v3_key;
      if ((inputs.public_key || '').trim()) {
        payload.public_key = inputs.public_key;
      }

      const response = await API.put('/api/wechatpay/admin/settings', payload);
      if (!response.data?.success) {
        showError(response.data?.message || t('保存微信支付配置失败'));
        return;
      }
      showSuccess(t('微信支付配置已保存'));
      const sanitized = {
        ...inputs,
        merchant_private_key: '',
        api_v3_key: '',
        public_key: '',
      };
      setInputs(sanitized);
      formApiRef.current?.setValues(sanitized);
      await loadSettings();
    } catch (_error) {
      showError(t('保存微信支付配置失败'));
    } finally {
      setLoading(false);
    }
  };

  const clearSecret = async (name) => {
    setLoading(true);
    try {
      const response = await API.put('/api/wechatpay/admin/settings', {
        enabled: false,
        clear_secrets: [name],
        force_clear_secrets: Number(inputs.pending_order_count || 0) > 0,
      });
      if (!response.data?.success) {
        showError(response.data?.message || t('清空凭据失败'));
        return;
      }
      showSuccess(t('凭据已清空，微信支付已停用'));
      const sanitized = {
        ...inputs,
        merchant_private_key: '',
        api_v3_key: '',
        public_key: '',
      };
      setInputs(sanitized);
      formApiRef.current?.setValues(sanitized);
      await loadSettings();
    } catch (_error) {
      showError(t('清空凭据失败'));
    } finally {
      setLoading(false);
    }
  };

  const credentialLabel = (configured) => (
    <Tag color={configured ? 'green' : 'grey'}>
      {configured ? t('已配置') : t('未配置')}
    </Tag>
  );

  return (
    <Spin spinning={loading}>
      <Form
        initValues={inputs}
        onValueChange={handleChange}
        getFormApi={(api) => (formApiRef.current = api)}
      >
        <Form.Section text={sectionTitle}>
          {!inputs.option_crypt_key_configured && (
            <Banner
              type='danger'
              closeIcon={null}
              description={t(
                '保存密钥或启用微信支付前，请先在服务端设置环境变量 OPTION_CRYPT_KEY。',
              )}
              style={{ marginBottom: 12 }}
            />
          )}
          <Banner
            type='info'
            closeIcon={null}
            description={
              <>
                {t('充值回调地址')}：
                <code>
                  {inputs.resolved_notify_url || '/api/wechatpay/notify'}
                </code>
                <br />
                {t('关闭支付入口不会关闭在途订单回调和后台补单。')}
              </>
            }
            style={{ marginBottom: 12 }}
          />
          <Banner
            type='warning'
            closeIcon={null}
            description={t('当前有 {{count}} 笔微信支付待处理订单。', {
              count: inputs.pending_order_count || 0,
            })}
            style={{ marginBottom: 16 }}
          />
          <Banner
            type='warning'
            closeIcon={null}
            description={t(
              'WeChat Pay API v3 has no independent sandbox. Use restricted test users and low-value real payments; do not load-test production payment APIs.',
            )}
            style={{ marginBottom: 16 }}
          />

          <Row gutter={16}>
            <Col xs={24} md={6}>
              <Form.Switch
                field='enabled'
                label={t('启用微信支付 Native')}
                checkedText='｜'
                uncheckedText='〇'
              />
            </Col>
            <Col xs={24} md={6}>
              <Form.InputNumber
                field='min_topup'
                label={t('最低充值数量')}
                min={1}
                precision={0}
              />
            </Col>
            <Col xs={24} md={6}>
              <Form.InputNumber
                field='max_topup'
                label={t('Maximum top-up amount')}
                min={Number(inputs.min_topup || 1)}
                max={4000}
                precision={0}
                extraText={t('The server hard limit is 4000 billing units.')}
              />
            </Col>
            <Col xs={24} md={6}>
              <Form.InputNumber
                field='order_expire_minutes'
                label={t('订单有效期（分钟）')}
                min={1}
                max={120}
                precision={0}
              />
            </Col>
          </Row>

          <Row gutter={16} style={{ marginTop: 16 }}>
            <Col xs={24} md={8}>
              <Form.Input field='app_id' label={t('AppID')} />
            </Col>
            <Col xs={24} md={8}>
              <Form.Input field='mch_id' label={t('商户号')} />
            </Col>
            <Col xs={24} md={8}>
              <Form.Input
                field='merchant_cert_serial_no'
                label={t('商户证书序列号')}
              />
            </Col>
          </Row>

          <Row gutter={16} style={{ marginTop: 16 }}>
            <Col xs={24} md={12}>
              <Form.Input
                field='public_key_id'
                label={t('微信支付公钥 ID')}
                placeholder='PUB_KEY_ID_...'
              />
            </Col>
            <Col xs={24} md={12}>
              <Form.Input
                field='notify_url'
                label={t('自定义回调地址（可选）')}
                placeholder='https://example.com/api/wechatpay/notify'
                extraText={t('必须为不带查询参数的公网 HTTPS 地址')}
              />
            </Col>
          </Row>

          <Row gutter={16} style={{ marginTop: 16 }}>
            <Col xs={24} md={12}>
              <Form.TextArea
                field='merchant_private_key'
                label={
                  <span>
                    {t('商户私钥（PKCS#8 PEM）')}{' '}
                    {credentialLabel(inputs.merchant_private_key_configured)}
                  </span>
                }
                placeholder={t('留空表示保持现有凭据不变')}
                autosize={{ minRows: 6, maxRows: 12 }}
              />
              {inputs.merchant_private_key_configured && (
                <Popconfirm
                  title={t('确认清空商户私钥？')}
                  content={t(
                    'Clearing credentials disables new orders and prevents callbacks and background reconciliation. Reconcile {{count}} pending orders first. Continue only for emergency key revocation.',
                    { count: inputs.pending_order_count || 0 },
                  )}
                  onConfirm={() => clearSecret('merchant_private_key')}
                >
                  <Button type='danger' theme='light'>
                    {t('清空商户私钥')}
                  </Button>
                </Popconfirm>
              )}
            </Col>
            <Col xs={24} md={12}>
              <Form.TextArea
                field='public_key'
                label={
                  <span>
                    {t('微信支付公钥（PEM）')}{' '}
                    {credentialLabel(inputs.public_key_configured)}
                  </span>
                }
                placeholder={t('留空表示保持现有凭据不变')}
                autosize={{ minRows: 6, maxRows: 12 }}
              />
              {inputs.public_key_configured && (
                <Popconfirm
                  title={t('确认清空微信支付公钥？')}
                  content={t(
                    'Clearing credentials disables new orders and prevents callbacks and background reconciliation. Reconcile {{count}} pending orders first. Continue only for emergency key revocation.',
                    { count: inputs.pending_order_count || 0 },
                  )}
                  onConfirm={() => clearSecret('public_key')}
                >
                  <Button type='danger' theme='light'>
                    {t('清空微信支付公钥')}
                  </Button>
                </Popconfirm>
              )}
            </Col>
          </Row>

          <Row gutter={16} style={{ marginTop: 16 }}>
            <Col xs={24} md={12}>
              <Form.Input
                field='api_v3_key'
                mode='password'
                autoComplete='new-password'
                label={
                  <span>
                    {t('APIv3 密钥（32 字节）')}{' '}
                    {credentialLabel(inputs.api_v3_key_configured)}
                  </span>
                }
                placeholder={t('留空表示保持现有凭据不变')}
              />
              {inputs.api_v3_key_configured && (
                <Popconfirm
                  title={t('确认清空 APIv3 密钥？')}
                  content={t(
                    'Clearing credentials disables new orders and prevents callbacks and background reconciliation. Reconcile {{count}} pending orders first. Continue only for emergency key revocation.',
                    { count: inputs.pending_order_count || 0 },
                  )}
                  onConfirm={() => clearSecret('api_v3_key')}
                >
                  <Button type='danger' theme='light'>
                    {t('清空 APIv3 密钥')}
                  </Button>
                </Popconfirm>
              )}
            </Col>
          </Row>

          <Button
            style={{ marginTop: 24 }}
            onClick={saveSettings}
            disabled={!loaded || loading}
          >
            {t('保存微信支付设置')}
          </Button>
        </Form.Section>
      </Form>
    </Spin>
  );
}
