import {
  createContext,
  useContext,
  useEffect,
  useMemo,
  useState,
  type ReactNode,
} from 'react';
import { messages, type Locale, type MessageKey } from './messages';

type I18nContextValue = {
  locale: Locale;
  setLocale: (locale: Locale) => void;
  t: (key: MessageKey) => string;
};

const STORAGE_KEY = 'web-v2-locale';

const I18nContext = createContext<I18nContextValue | null>(null);

function getDefaultLocale(): Locale {
  const stored = localStorage.getItem(STORAGE_KEY);
  if (stored === 'en' || stored === 'zh') {
    return stored;
  }

  const browserLanguage = navigator.language.toLowerCase();
  return browserLanguage.startsWith('zh') ? 'zh' : 'en';
}

export function I18nProvider({ children }: { children: ReactNode }) {
  const [locale, setLocaleState] = useState<Locale>('en');

  useEffect(() => {
    setLocaleState(getDefaultLocale());
  }, []);

  const setLocale = (nextLocale: Locale) => {
    setLocaleState(nextLocale);
    localStorage.setItem(STORAGE_KEY, nextLocale);
  };

  const value = useMemo<I18nContextValue>(
    () => ({
      locale,
      setLocale,
      t: (key) => messages[locale][key],
    }),
    [locale],
  );

  return <I18nContext.Provider value={value}>{children}</I18nContext.Provider>;
}

export function useI18n() {
  const context = useContext(I18nContext);
  if (!context) {
    throw new Error('useI18n must be used within I18nProvider');
  }
  return context;
}
