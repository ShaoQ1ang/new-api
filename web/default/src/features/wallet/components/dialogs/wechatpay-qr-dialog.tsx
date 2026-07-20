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
import { useEffect, useRef, useState } from 'react'
import { QRCodeSVG } from 'qrcode.react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Spinner } from '@/components/ui/spinner'
import { getOrderStatus, isApiSuccess } from '../../api'
import type { WechatPayOrder } from '../../hooks/use-wechatpay-payment'

interface WechatPayQrDialogProps {
  open: boolean
  order: WechatPayOrder | null
  onOpenChange: (open: boolean) => void
  onPaid: () => Promise<void>
}

export function WechatPayQrDialog({
  open,
  order,
  onOpenChange,
  onPaid,
}: WechatPayQrDialogProps) {
  const { t } = useTranslation()
  const [status, setStatus] = useState<'pending' | 'success' | 'expired'>(
    'pending'
  )
  const paidHandled = useRef(false)

  useEffect(() => {
    if (!open || !order) return

    let disposed = false
    let polling = false
    paidHandled.current = false
    setStatus('pending')

    const poll = async () => {
      if (disposed || polling || paidHandled.current) return
      const expiresAt = order.expires_at * 1000
      if (Date.now() >= expiresAt) {
        setStatus('expired')
      }
      // Keep checking the local order briefly after QR expiry. A payment made
      // at the deadline can still be confirmed by a delayed callback/query.
      if (Date.now() >= expiresAt + 2 * 60 * 1000) {
        return
      }
      polling = true
      try {
        const response = await getOrderStatus(order.trade_no)
        const nextStatus = isApiSuccess(response)
          ? response.data?.status
          : undefined
        if (disposed) return
        if (nextStatus === 'success' && !paidHandled.current) {
          paidHandled.current = true
          setStatus('success')
          toast.success(t('WeChat Pay payment completed'))
          await onPaid()
          if (!disposed) onOpenChange(false)
        } else if (nextStatus === 'expired' || nextStatus === 'failed') {
          setStatus('expired')
        }
      } catch {
        // Transient polling failures are retried without interrupting payment.
      } finally {
        polling = false
      }
    }

    void poll()
    const timer = window.setInterval(() => void poll(), 2500)
    return () => {
      disposed = true
      window.clearInterval(timer)
    }
  }, [open, order, onOpenChange, onPaid, t])

  if (!order) return null

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className='sm:max-w-md'>
        <DialogHeader>
          <DialogTitle>{t('Pay with WeChat Pay')}</DialogTitle>
          <DialogDescription>
            {t(
              'Scan this QR code with WeChat. This page will update automatically.'
            )}
          </DialogDescription>
        </DialogHeader>

        <div className='flex flex-col items-center gap-4 py-2'>
          <div className='rounded-xl border bg-white p-4 shadow-sm'>
            <QRCodeSVG value={order.code_url} size={220} level='M' />
          </div>
          {status === 'pending' && (
            <Badge variant='secondary' className='gap-2'>
              <Spinner className='size-3.5' />
              {t('Waiting for payment')}
            </Badge>
          )}
          {status === 'success' && (
            <Badge className='bg-green-600'>{t('Payment successful')}</Badge>
          )}
          {status === 'expired' && (
            <Alert variant='destructive'>
              <AlertDescription>
                {t(
                  'This payment QR code has expired. Close it and create a new order.'
                )}
              </AlertDescription>
            </Alert>
          )}
          <p className='text-muted-foreground text-center font-mono text-xs break-all'>
            {t('Order number')}: {order.trade_no}
          </p>
        </div>

        <DialogFooter>
          <Button variant='outline' onClick={() => onOpenChange(false)}>
            {t('Close')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
