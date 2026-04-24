import { useState } from 'react';
import { ArrowRight, ShieldCheck, Sparkles, Loader2 } from 'lucide-react';
import { useNavigate } from 'react-router-dom';
import { api } from '../lib/api';

export default function Login() {
  const navigate = useNavigate();
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');

  const [require2FA, setRequire2FA] = useState(false);
  const [twoFACode, setTwoFACode] = useState('');

  const handleLogin = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!username || !password) {
      setError('Please enter both username and password.');
      return;
    }

    setLoading(true);
    setError('');

    try {
      const res = await api.post('/api/user/login', {
        username,
        password,
      });

      const { success, message, data } = res.data;

      if (success) {
        if (data && data.require_2fa) {
          setRequire2FA(true);
        } else {
          localStorage.setItem('user', JSON.stringify(data));
          navigate('/console');
        }
      } else {
        setError(message || 'Login failed.');
      }
    } catch (err: any) {
      setError(err?.response?.data?.message || 'An error occurred during login.');
    } finally {
      setLoading(false);
    }
  };

  const handle2FALogin = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!twoFACode) {
      setError('Please enter your 2FA code.');
      return;
    }

    setLoading(true);
    setError('');

    try {
      const res = await api.post('/api/user/login/2fa', {
        code: twoFACode,
      });

      const { success, message, data } = res.data;

      if (success) {
        localStorage.setItem('user', JSON.stringify(data));
        navigate('/console');
      } else {
        setError(message || '2FA verification failed.');
      }
    } catch (err: any) {
      setError(err?.response?.data?.message || 'An error occurred during 2FA verification.');
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className='min-h-screen bg-slate-950 text-slate-50 selection:bg-sky-500/30 px-6 py-10 lg:px-10 flex items-center justify-center'>
      <div className="absolute inset-0 z-0 overflow-hidden pointer-events-none">
        <div className="absolute -top-[20%] left-[20%] w-[800px] h-[800px] rounded-full bg-sky-600/5 blur-[120px] mix-blend-screen" />
      </div>

      <div className='relative z-10 w-full max-w-5xl mx-auto grid gap-12 lg:grid-cols-[1fr_420px] lg:items-center'>
        <section className='space-y-8 hidden lg:block'>
          <div className='inline-flex items-center gap-2 rounded-full border border-sky-500/30 bg-sky-500/10 px-4 py-2 text-sm font-medium text-sky-300'>
            <ShieldCheck className='h-4 w-4' />
            Secure Access Portal
          </div>
          <div className='space-y-4'>
            <p className='text-sm font-bold uppercase tracking-wider text-slate-400'>QuantumNous / new-api</p>
            <h1 className='text-4xl lg:text-5xl font-bold tracking-tight text-white leading-tight'>
              Sign in to manage your AI infrastructure.
            </h1>
            <p className='text-lg text-slate-400 leading-relaxed max-w-md'>
              Access the console to operate channels, issue tokens, configure routing rules, and monitor your unified gateway telemetry.
            </p>
          </div>
          <div className='grid gap-4 max-w-md'>
            <div className='flex items-start gap-3 rounded-2xl border border-slate-800 bg-slate-900/50 p-4'>
              <div className='mt-0.5 rounded-full bg-emerald-500/20 p-1'>
                <div className='h-2 w-2 rounded-full bg-emerald-400' />
              </div>
              <p className='text-sm text-slate-300'>Connected securely to the real backend login endpoints.</p>
            </div>
          </div>
        </section>

        <section className='rounded-[32px] border border-slate-800 bg-slate-900/60 p-8 shadow-2xl backdrop-blur-xl w-full max-w-md mx-auto'>
          <div className='flex items-center gap-3 mb-8 lg:hidden'>
            <div className='flex items-center justify-center h-10 w-10 rounded-xl bg-gradient-to-br from-sky-400 to-indigo-600 shadow-lg shadow-sky-500/20'>
              <Sparkles className='h-5 w-5 text-white' />
            </div>
            <div>
              <p className='text-xs font-bold uppercase tracking-[0.2em] text-sky-400'>
                QuantumNous
              </p>
              <p className='text-lg font-semibold text-white'>
                new-api
              </p>
            </div>
          </div>

          <div className='hidden lg:flex items-center gap-3 mb-8'>
            <div className='flex items-center justify-center h-12 w-12 rounded-2xl bg-gradient-to-br from-sky-400 to-indigo-600 shadow-lg shadow-sky-500/20'>
              <Sparkles className='h-6 w-6 text-white' />
            </div>
            <div>
              <p className='text-sm font-bold uppercase tracking-[0.2em] text-sky-400'>
                Operator Sign In
              </p>
              <p className='text-slate-400 text-sm'>
                Welcome back to the console
              </p>
            </div>
          </div>

          {!require2FA ? (
            <form onSubmit={handleLogin} className='space-y-5'>
              <div className='space-y-2'>
                <label className='text-sm font-medium text-slate-300'>
                  Account
                </label>
                <input
                  type='text'
                  value={username}
                  onChange={(e) => setUsername(e.target.value)}
                  placeholder='Email or username'
                  className='w-full rounded-xl border border-slate-700 bg-slate-950 px-4 py-3 text-sm text-white placeholder:text-slate-600 focus:border-sky-500 focus:outline-none focus:ring-1 focus:ring-sky-500 transition-colors'
                  required
                />
              </div>
              <div className='space-y-2'>
                <label className='text-sm font-medium text-slate-300'>
                  Password
                </label>
                <input
                  type='password'
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  placeholder='Enter your password'
                  className='w-full rounded-xl border border-slate-700 bg-slate-950 px-4 py-3 text-sm text-white placeholder:text-slate-600 focus:border-sky-500 focus:outline-none focus:ring-1 focus:ring-sky-500 transition-colors'
                  required
                />
              </div>

              {error && (
                <div className='rounded-lg bg-red-500/10 p-3 text-sm text-red-400 border border-red-500/20'>
                  {error}
                </div>
              )}

              <button 
                type='submit' 
                disabled={loading}
                className='w-full inline-flex items-center justify-center rounded-xl bg-gradient-to-r from-sky-500 to-indigo-600 px-4 py-3 text-sm font-semibold text-white shadow-md shadow-sky-500/20 transition-all hover:opacity-90 disabled:opacity-70 disabled:cursor-not-allowed'
              >
                {loading ? <Loader2 className="mr-2 h-4 w-4 animate-spin" /> : null}
                Sign In
              </button>
            </form>
          ) : (
            <form onSubmit={handle2FALogin} className='space-y-5'>
               <div className='space-y-2'>
                <label className='text-sm font-medium text-slate-300'>
                  Two-Factor Authentication
                </label>
                <p className='text-xs text-slate-400 mb-2'>Enter the 6-digit code from your authenticator app, or your 8-digit backup code.</p>
                <input
                  type='text'
                  value={twoFACode}
                  onChange={(e) => setTwoFACode(e.target.value)}
                  placeholder='Verification Code'
                  className='w-full rounded-xl border border-slate-700 bg-slate-950 px-4 py-3 text-sm text-white placeholder:text-slate-600 focus:border-sky-500 focus:outline-none focus:ring-1 focus:ring-sky-500 transition-colors'
                  required
                />
              </div>

              {error && (
                <div className='rounded-lg bg-red-500/10 p-3 text-sm text-red-400 border border-red-500/20'>
                  {error}
                </div>
              )}

              <div className='flex gap-3'>
                <button 
                  type='button'
                  onClick={() => {
                    setRequire2FA(false);
                    setTwoFACode('');
                    setError('');
                  }}
                  disabled={loading}
                  className='w-1/3 inline-flex items-center justify-center rounded-xl border border-slate-700 bg-slate-800 px-4 py-3 text-sm font-medium text-white transition-all hover:bg-slate-700'
                >
                  Back
                </button>
                <button 
                  type='submit' 
                  disabled={loading}
                  className='w-2/3 inline-flex items-center justify-center rounded-xl bg-gradient-to-r from-sky-500 to-indigo-600 px-4 py-3 text-sm font-semibold text-white shadow-md shadow-sky-500/20 transition-all hover:opacity-90 disabled:opacity-70 disabled:cursor-not-allowed'
                >
                  {loading ? <Loader2 className="mr-2 h-4 w-4 animate-spin" /> : null}
                  Verify
                </button>
              </div>
            </form>
          )}

          <a
            href='/'
            className='mt-8 inline-flex items-center gap-2 text-sm font-medium text-slate-400 transition-colors hover:text-white'
          >
            <ArrowRight className='h-4 w-4 rotate-180' />
            Back to Home
          </a>
        </section>
      </div>
    </div>
  );
}