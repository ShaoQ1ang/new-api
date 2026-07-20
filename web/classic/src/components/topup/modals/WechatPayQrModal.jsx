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
import React, { useEffect, useRef, useState } from 'react';
import { Banner, Modal, Spin, Tag, Typography } from '@douyinfe/semi-ui';
import { QRCodeSVG } from 'qrcode.react';
import { API, showSuccess } from '../../../helpers';

const { Text } = Typography;

export default function WechatPayQrModal({
  t,
  visible,
  order,
  onCancel,
  onPaid,
}) {
  const [status, setStatus] = useState('pending');
  const paidHandledRef = useRef(false);

  useEffect(() => {
    if (!visible || !order) return undefined;

    let disposed = false;
    let polling = false;
    paidHandledRef.current = false;
    setStatus('pending');

    const poll = async () => {
      if (disposed || polling || paidHandledRef.current) return;
      const expiresAt = Number(order.expires_at) * 1000;
      if (Date.now() >= expiresAt) {
        setStatus('expired');
      }
      // Keep checking local state briefly: a payment made at the deadline can
      // still be confirmed by a delayed callback or the reconciliation task.
      if (Date.now() >= expiresAt + 2 * 60 * 1000) {
        return;
      }
      polling = true;
      try {
        const response = await API.get('/api/user/order/status', {
          params: { trade_no: order.trade_no, type: 'topup' },
        });
        const nextStatus = response.data?.success
          ? response.data?.data?.status
          : undefined;
        if (disposed) return;
        if (nextStatus === 'success' && !paidHandledRef.current) {
          paidHandledRef.current = true;
          setStatus('success');
          showSuccess(t('WeChat Pay payment completed'));
          await onPaid();
          if (!disposed) onCancel();
        } else if (nextStatus === 'expired' || nextStatus === 'failed') {
          setStatus('expired');
        }
      } catch (_error) {
        // Retry transient polling errors while the QR code remains valid.
      } finally {
        polling = false;
      }
    };

    void poll();
    const timer = window.setInterval(() => void poll(), 2500);
    return () => {
      disposed = true;
      window.clearInterval(timer);
    };
  }, [visible, order, onCancel, onPaid, t]);

  return (
    <Modal
      title={t('WeChat Pay')}
      visible={visible}
      onCancel={onCancel}
      footer={null}
      maskClosable={false}
      centered
      size='small'
    >
      {order && (
        <div className='flex flex-col items-center gap-4 py-3'>
          <Text type='tertiary'>
            {t(
              'Scan this QR code with WeChat. This page will update automatically.',
            )}
          </Text>
          <div className='rounded-xl border bg-white p-4'>
            <QRCodeSVG value={order.code_url} size={220} level='M' />
          </div>
          {status === 'pending' && (
            <Tag color='green' prefixIcon={<Spin size='small' />}>
              {t('Waiting for payment')}
            </Tag>
          )}
          {status === 'success' && (
            <Tag color='green'>{t('Payment successful')}</Tag>
          )}
          {status === 'expired' && (
            <Banner
              type='danger'
              closeIcon={null}
              description={t(
                'This payment QR code has expired. Close it and create a new order.',
              )}
            />
          )}
          <Text type='tertiary' size='small' className='break-all font-mono'>
            {t('Order number')}: {order.trade_no}
          </Text>
        </div>
      )}
    </Modal>
  );
}
