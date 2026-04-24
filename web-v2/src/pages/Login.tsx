import { useState, type FormEvent } from 'react';
import {
  ArrowRight,
  KeyRound,
  Languages,
  Loader2,
  ShieldCheck,
  Sparkles,
  Workflow,
} from 'lucide-react';
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
  const version = status.data?.version;
  const docsLink = status.data?.docs_link;
  const passkeyEnabled = status.data?.passkey_login;
  const setupMode = status.data?.setup;

  async function handleLogin(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();

    if (!username || !password) {
      setError(t('loginEmptyError'));
      return;
    }

    setLoading(true);
    setError('');

    try {
      const response = await api.post('/api/user/login', {
        username,
        password,
      });
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
      const response = await api.post('/api/user/login/2fa', {
        code: twoFACode,
      });
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
    <div className='min-h-screen overflow-hidden bg-[radial-gradient(circle_at_top_left,_rgba(14,165,233,0.16),_transparent_26%),radial-gradient(circle_at_bottom_right,_rgba(13,148,136,0.18),_transparent_28%),linear-gradient(180deg,_#eff6ff_0%,_#f8fafc_42%,_#ffffff_100%)] px-6 py-8 text-slate-950 selection:bg-sky-200/70 lg:px-10'>
      <div className='pointer-events-none absolute inset-0 z-0 overflow-hidden'>
        <div className='absolute left-[-10%] top-[-8%] h-[28rem] w-[28rem] rounded-full bg-sky-300/25 blur-[120px]' />
        <div className='absolute bottom-[-16%] right-[-8%] h-[28rem] w-[28rem] rounded-full bg-teal-200/30 blur-[120px]' />
      </div>

      <div className='relative z-10 mx-auto flex max-w-7xl items-center justify-between'>
        <div className='flex items-center gap-3'>
          <div className='flex h-11 w-11 items-center justify-center rounded-2xl bg-slate-950 text-white shadow-[0_16px_36px_-22px_rgba(15,23,42,0.7)]'>
            <Workflow className='h-5 w-5' />
          </div>
          <div>
            <p className='text-xs font-bold uppercase tracking-[0.24em] text-slate-500'>
              QuantumNous
            </p>
            <p className='text-lg font-semibold text-slate-950'>{systemName}</p>
          </div>
        </div>

        <div className='hidden items-center gap-3 md:flex'>
          <button
            type='button'
            onClick={() => setLocale(locale === 'en' ? 'zh' : 'en')}
            className='inline-flex items-center gap-2 rounded-full border border-slate-200 bg-white px-4 py-2 text-sm font-medium text-slate-700 shadow-sm transition-colors hover:bg-slate-50'
          >
            <Languages className='h-4 w-4' />
            {t('localeLabel')}
          </button>
          <a
            href='/'
            className='inline-flex items-center gap-2 text-sm font-medium text-slate-600 transition-colors hover:text-slate-950'
          >
            <ArrowRight className='h-4 w-4 rotate-180' />
            {t('loginBackHome')}
          </a>
        </div>
      </div>

      <main className='relative z-10 mx-auto mt-10 grid max-w-7xl gap-10 lg:grid-cols-[1.02fr_0.98fr] lg:items-stretch'>
        <section className='flex flex-col justify-between rounded-[36px] border border-white/80 bg-[linear-gradient(180deg,_rgba(255,255,255,0.78),_rgba(255,255,255,0.58))] p-8 shadow-[0_36px_100px_-54px_rgba(15,23,42,0.4)] backdrop-blur-xl lg:p-10'>
          <div className='space-y-8'>
            <div className='inline-flex items-center gap-2 rounded-full border border-sky-200 bg-white/90 px-4 py-2 text-sm font-medium text-sky-700'>
              <Sparkles className='h-4 w-4' />
              {t('loginEyebrow')}
            </div>

            <div className='space-y-5'>
              <h1 className='max-w-3xl text-4xl font-semibold tracking-[-0.05em] text-slate-950 sm:text-5xl lg:text-6xl lg:leading-[1.02]'>
                {t('loginTitle')}
              </h1>
              <p className='max-w-2xl text-lg leading-8 text-slate-600'>
                {t('loginDescription')}
              </p>
            </div>

            <div className='grid gap-4 md:grid-cols-3'>
              {[t('loginStatusLive'), t('loginStatusSupport'), t('loginStatusI18n')].map(
                (item) => (
                  <div
                    key={item}
                    className='rounded-[24px] border border-white/80 bg-white/82 px-5 py-5 shadow-[0_24px_60px_-42px_rgba(15,23,42,0.35)]'
                  >
                    <ShieldCheck className='h-5 w-5 text-sky-700' />
                    <p className='mt-4 text-sm font-medium leading-6 text-slate-700'>{item}</p>
                  </div>
                ),
              )}
            </div>

            <div className='rounded-[28px] border border-slate-200 bg-slate-950 p-6 text-slate-200 shadow-[inset_0_1px_0_rgba(255,255,255,0.06)]'>
              <div className='flex items-center justify-between border-b border-white/10 pb-4'>
                <p className='font-mono text-sm text-slate-400'>session-overview.json</p>
                <span className='rounded-full bg-white/10 px-3 py-1 text-xs text-slate-300'>
                  {t('loginFormTitle')}
                </span>
              </div>
              <div className='mt-5 space-y-4 text-sm leading-7 text-slate-300'>
                <p>
                  {t('loginMetaVersion')}: {version || 'unknown'}
                </p>
                <p>{passkeyEnabled ? t('loginMetaPasskeyOn') : t('loginMetaPasskeyOff')}</p>
                {setupMode ? <p>{t('loginMetaSetup')}</p> : null}
                {docsLink ? (
                  <a
                    href={docsLink}
                    target='_blank'
                    rel='noreferrer'
                    className='inline-flex items-center gap-2 font-medium text-teal-300 transition-colors hover:text-teal-200'
                  >
                    {t('navDocs')}
                    <ArrowRight className='h-4 w-4' />
                  </a>
                ) : null}
              </div>
            </div>
          </div>

          <div className='mt-8 rounded-[28px] border border-slate-200 bg-white/75 p-6'>
            <p className='text-xs font-medium uppercase tracking-[0.24em] text-slate-500'>
              {t('loginHelpTitle')}
            </p>
            <div className='mt-4 space-y-3'>
              {[t('loginHelpOne'), t('loginHelpTwo'), t('loginHelpThree')].map((item) => (
                <div key={item} className='flex items-start gap-3'>
                  <div className='mt-1 flex h-6 w-6 items-center justify-center rounded-full bg-emerald-50 text-emerald-700'>
                    <div className='h-2.5 w-2.5 rounded-full bg-emerald-500' />
                  </div>
                  <p className='text-sm leading-7 text-slate-600'>{item}</p>
                </div>
              ))}
            </div>
          </div>
        </section>

        <section className='rounded-[36px] border border-slate-200 bg-white/86 p-8 shadow-[0_36px_100px_-54px_rgba(15,23,42,0.4)] backdrop-blur xl:p-10'>
          <div className='flex items-start justify-between gap-4 border-b border-slate-200 pb-6'>
            <div>
              <p className='text-xs font-medium uppercase tracking-[0.24em] text-slate-500'>
                web-v2
              </p>
              <h2 className='mt-3 text-3xl font-semibold tracking-tight text-slate-950'>
                {require2FA ? t('loginTwoFATitle') : t('loginFormTitle')}
              </h2>
              <p className='mt-3 max-w-md text-base leading-7 text-slate-600'>
                {require2FA ? t('loginTwoFADescription') : t('loginFormDescription')}
              </p>
            </div>
            <div className='flex h-12 w-12 items-center justify-center rounded-2xl bg-slate-950 text-white'>
              {require2FA ? (
                <KeyRound className='h-5 w-5' />
              ) : (
                <ShieldCheck className='h-5 w-5' />
              )}
            </div>
          </div>

          {!require2FA ? (
            <form onSubmit={handleLogin} className='mt-8 space-y-5'>
              <div className='space-y-2'>
                <label className='text-sm font-medium text-slate-700'>
                  {t('loginAccountLabel')}
                </label>
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
                <label className='text-sm font-medium text-slate-700'>
                  {t('loginPasswordLabel')}
                </label>
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
                className='inline-flex w-full items-center justify-center rounded-2xl bg-slate-950 px-5 py-3.5 text-sm font-semibold text-white transition-all hover:bg-slate-800 disabled:cursor-not-allowed disabled:opacity-70'
              >
                {loading ? <Loader2 className='mr-2 h-4 w-4 animate-spin' /> : null}
                {loading ? t('loginSubmitting') : t('loginSubmit')}
              </button>
            </form>
          ) : (
            <form onSubmit={handleTwoFactorLogin} className='mt-8 space-y-5'>
              <div className='space-y-2'>
                <label className='text-sm font-medium text-slate-700'>
                  {t('loginTwoFATitle')}
                </label>
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
                  disabled={loading}
                  className='inline-flex w-1/3 items-center justify-center rounded-2xl border border-slate-200 bg-white px-4 py-3.5 text-sm font-semibold text-slate-700 transition-colors hover:bg-slate-50 disabled:cursor-not-allowed disabled:opacity-70'
                >
                  {t('loginBack')}
                </button>
                <button
                  type='submit'
                  disabled={loading}
                  className='inline-flex w-2/3 items-center justify-center rounded-2xl bg-slate-950 px-5 py-3.5 text-sm font-semibold text-white transition-all hover:bg-slate-800 disabled:cursor-not-allowed disabled:opacity-70'
                >
                  {loading ? <Loader2 className='mr-2 h-4 w-4 animate-spin' /> : null}
                  {loading ? t('loginTwoFASubmitting') : t('loginTwoFASubmit')}
                </button>
              </div>
            </form>
          )}

          <div className='mt-8 flex items-center justify-between gap-3 border-t border-slate-200 pt-6'>
            <a
              href='/'
              className='inline-flex items-center gap-2 text-sm font-medium text-slate-600 transition-colors hover:text-slate-950'
            >
              <ArrowRight className='h-4 w-4 rotate-180' />
              {t('loginBackHome')}
            </a>
            <button
              type='button'
              onClick={() => setLocale(locale === 'en' ? 'zh' : 'en')}
              className='inline-flex items-center gap-2 rounded-full border border-slate-200 bg-white px-4 py-2 text-sm font-medium text-slate-700 transition-colors hover:bg-slate-50 md:hidden'
            >
              <Languages className='h-4 w-4' />
              {t('localeLabel')}
            </button>
          </div>
        </section>
      </main>
    </div>
  );
}
