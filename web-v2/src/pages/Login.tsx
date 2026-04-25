import { useState, type FormEvent } from 'react';
import { ArrowRight, Languages, Loader2, ShieldCheck, Workflow } from 'lucide-react';
import { useNavigate } from 'react-router-dom';
import { api } from '../lib/api';
import { useStatus } from '../hooks/useStatus';
import { useI18n } from '../i18n/I18nProvider';

export default function Login() {
  const navigate = useNavigate();
  const status = useStatus();
  const { locale, setLocale, t } = useI18n();

  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [twoFACode, setTwoFACode] = useState('');
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const [require2FA, setRequire2FA] = useState(false);

  const systemName = status.data?.system_name || 'new-api';

  async function handleLogin(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!username || !password) {
      setError(t('loginEmptyError'));
      return;
    }

    setLoading(true);
    setError('');

    try {
      const response = await api.post('/api/user/login', { username, password });
      const { success, message, data } = response.data;

      if (!success) {
        setError(message || t('loginGenericError'));
        return;
      }

      if (data?.require_2fa) {
        setRequire2FA(true);
        return;
      }

      localStorage.setItem('user', JSON.stringify(data));
      navigate('/console');
    } catch (error: any) {
      setError(error?.response?.data?.message || t('loginGenericError'));
    } finally {
      setLoading(false);
    }
  }

  async function handleTwoFactorLogin(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!twoFACode) {
      setError(t('loginTwoFARequiredError'));
      return;
    }

    setLoading(true);
    setError('');

    try {
      const response = await api.post('/api/user/login/2fa', { code: twoFACode });
      const { success, message, data } = response.data;

      if (!success) {
        setError(message || t('loginTwoFAGenericError'));
        return;
      }

      localStorage.setItem('user', JSON.stringify(data));
      navigate('/console');
    } catch (error: any) {
      setError(error?.response?.data?.message || t('loginTwoFAGenericError'));
    } finally {
      setLoading(false);
    }
  }

  return (
    <div className='min-h-screen bg-[#f7f8fa] text-slate-950'>
      <header className='border-b border-slate-200 bg-white'>
        <div className='mx-auto flex max-w-6xl items-center justify-between px-5 py-5 lg:px-8'>
          <div className='flex items-center gap-3'>
            <div className='grid h-10 w-10 place-items-center rounded-2xl bg-slate-950 text-white'>
              <Workflow className='h-4 w-4' />
            </div>
            <p className='text-sm font-semibold text-slate-950'>{systemName}</p>
          </div>

          <button
            type='button'
            onClick={() => setLocale(locale === 'en' ? 'zh' : 'en')}
            className='rounded-full border border-slate-200 bg-white px-4 py-2 text-sm font-medium text-slate-700'
          >
            <span className='inline-flex items-center gap-2'>
              <Languages className='h-4 w-4' />
              {t('localeLabel')}
            </span>
          </button>
        </div>
      </header>

      <main className='mx-auto max-w-6xl px-5 py-8 lg:px-8 lg:py-12'>
        <div className='mx-auto max-w-md rounded-[32px] border border-slate-200 bg-white p-8 shadow-sm'>
          <div className='flex items-center gap-3'>
            <div className='grid h-10 w-10 place-items-center rounded-2xl bg-slate-950 text-white'>
              <ShieldCheck className='h-4 w-4' />
            </div>
            <div>
              <h1 className='text-2xl font-semibold text-slate-950'>
                {require2FA ? t('loginTwoFATitle') : t('loginFormTitle')}
              </h1>
              <p className='mt-1 text-sm text-slate-500'>
                {require2FA ? t('loginTwoFADescription') : t('loginFormDescription')}
              </p>
            </div>
          </div>

          {!require2FA ? (
            <form onSubmit={handleLogin} className='mt-8 space-y-5'>
              <div className='space-y-2'>
                <label className='text-sm font-medium text-slate-700'>{t('loginAccountLabel')}</label>
                <input
                  type='text'
                  value={username}
                  onChange={(event) => setUsername(event.target.value)}
                  placeholder={t('loginAccountPlaceholder')}
                  className='input-shell'
                  autoComplete='username'
                />
              </div>

              <div className='space-y-2'>
                <label className='text-sm font-medium text-slate-700'>{t('loginPasswordLabel')}</label>
                <input
                  type='password'
                  value={password}
                  onChange={(event) => setPassword(event.target.value)}
                  placeholder={t('loginPasswordPlaceholder')}
                  className='input-shell'
                  autoComplete='current-password'
                />
              </div>

              {error ? (
                <div className='rounded-2xl border border-rose-200 bg-rose-50 px-4 py-3 text-sm text-rose-700'>
                  {error}
                </div>
              ) : null}

              <button
                type='submit'
                disabled={loading}
                className='inline-flex w-full items-center justify-center rounded-xl bg-slate-950 px-5 py-3 text-sm font-semibold text-white disabled:opacity-60'
              >
                {loading ? <Loader2 className='mr-2 h-4 w-4 animate-spin' /> : null}
                {loading ? t('loginSubmitting') : t('loginSubmit')}
              </button>
            </form>
          ) : (
            <form onSubmit={handleTwoFactorLogin} className='mt-8 space-y-5'>
              <div className='space-y-2'>
                <label className='text-sm font-medium text-slate-700'>{t('loginTwoFATitle')}</label>
                <input
                  type='text'
                  value={twoFACode}
                  onChange={(event) => setTwoFACode(event.target.value)}
                  placeholder={t('loginTwoFAPlaceholder')}
                  className='input-shell'
                  autoComplete='one-time-code'
                />
              </div>

              {error ? (
                <div className='rounded-2xl border border-rose-200 bg-rose-50 px-4 py-3 text-sm text-rose-700'>
                  {error}
                </div>
              ) : null}

              <div className='flex gap-3'>
                <button
                  type='button'
                  onClick={() => {
                    setRequire2FA(false);
                    setTwoFACode('');
                    setError('');
                  }}
                  className='secondary-button flex-1'
                >
                  {t('loginBack')}
                </button>
                <button
                  type='submit'
                  disabled={loading}
                  className='inline-flex flex-1 items-center justify-center rounded-xl bg-slate-950 px-5 py-3 text-sm font-semibold text-white disabled:opacity-60'
                >
                  {loading ? <Loader2 className='mr-2 h-4 w-4 animate-spin' /> : null}
                  {loading ? t('loginTwoFASubmitting') : t('loginTwoFASubmit')}
                </button>
              </div>
            </form>
          )}

          <div className='mt-8 border-t border-slate-200 pt-6'>
            <a href='/' className='inline-flex items-center gap-2 text-sm font-medium text-slate-600 hover:text-slate-950'>
              <ArrowRight className='h-4 w-4 rotate-180' />
              {t('loginBackHome')}
            </a>
          </div>
        </div>
      </main>
    </div>
  );
}
