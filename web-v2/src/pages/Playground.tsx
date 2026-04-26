import { useEffect, useMemo, useRef, useState, type ChangeEvent, type KeyboardEvent } from 'react';
import {
  ImagePlus,
  Images,
  Loader2,
  Search,
  Send,
  Settings2,
  Sparkles,
  Video,
} from 'lucide-react';
import { useI18n } from '../i18n/I18nProvider';
import { useAsyncData } from '../hooks/useAsyncData';
import {
  createPlaygroundVideo,
  fetchPlaygroundModels,
  fetchPlaygroundTokenKey,
  fetchPlaygroundTokens,
  fetchPlaygroundVideoTask,
  sendPlaygroundChat,
  sendPlaygroundImage,
} from '../lib/playground';

type Mode = 'chat' | 'images' | 'video';
type Message =
  | { role: 'user' | 'assistant'; kind: 'text'; content: string }
  | { role: 'assistant'; kind: 'image'; content: string }
  | { role: 'assistant'; kind: 'video'; content: string }
  | { role: 'assistant'; kind: 'status'; content: string };

type Thread = {
  title: string;
  messages: Message[];
};

export default function Playground() {
  const { t } = useI18n();
  const initialAssistantMessage = useMemo<Message>(
    () => ({
      role: 'assistant',
      kind: 'text',
      content: t('playgroundWelcome'),
    }),
    [t],
  );
  const [mode, setMode] = useState<Mode>('chat');
  const [threads, setThreads] = useState<Thread[]>([
    {
      title: t('playgroundThreadUntitled'),
      messages: [initialAssistantMessage],
    },
  ]);
  const [selectedThread, setSelectedThread] = useState(0);
  const [prompt, setPrompt] = useState('');
  const [selectedModel, setSelectedModel] = useState('');
  const [selectedTokenId, setSelectedTokenId] = useState('');
  const [search, setSearch] = useState('');
  const [referenceImages, setReferenceImages] = useState<string[]>([]);
  const [sending, setSending] = useState(false);
  const fileInputRef = useRef<HTMLInputElement | null>(null);
  const messageEndRef = useRef<HTMLDivElement | null>(null);
  const models = useAsyncData(fetchPlaygroundModels, []);
  const tokens = useAsyncData(fetchPlaygroundTokens, []);

  const activeMessages = useMemo(() => threads[selectedThread]?.messages || [], [threads, selectedThread]);

  const modeModels = useMemo(() => {
    const all = models.data || [];
    const lower = all.map((model) => ({ raw: model, lower: model.toLowerCase() }));
    const imageFiltered = lower.filter(({ lower }) =>
      /(image|img|flux|sdxl|dall|seedream|jimeng|mj|midjourney|gpt-image|recraft)/.test(lower),
    );
    const videoFiltered = lower.filter(({ lower }) =>
      /(video|vidu|kling|wan|seedance|hailuo|jimeng|sora|veo)/.test(lower),
    );

    if (mode === 'images') return (imageFiltered.length ? imageFiltered : lower).map(({ raw }) => raw);
    if (mode === 'video') return (videoFiltered.length ? videoFiltered : lower).map(({ raw }) => raw);
    return all;
  }, [models.data, mode]);

  useEffect(() => {
    if (!modeModels.length) return;
    if (!selectedModel || !modeModels.includes(selectedModel)) {
      setSelectedModel(modeModels[0]);
    }
  }, [modeModels, selectedModel]);

  useEffect(() => {
    if (mode === 'chat') return;
    const availableTokens = tokens.data || [];
    if (!availableTokens.length) {
      setSelectedTokenId('');
      return;
    }
    const nextTokenId = String(availableTokens[0].id);
    if (!selectedTokenId || !availableTokens.some((item) => String(item.id) === selectedTokenId)) {
      setSelectedTokenId(nextTokenId);
    }
  }, [mode, selectedTokenId, tokens.data]);

  useEffect(() => {
    messageEndRef.current?.scrollIntoView({ behavior: 'smooth', block: 'end' });
  }, [activeMessages, sending]);

  const filteredThreads = useMemo(() => {
    const indexedThreads = threads.map((thread, index) => ({ thread, index }));
    if (!search.trim()) return indexedThreads;
    return indexedThreads.filter(({ thread }) => thread.title.toLowerCase().includes(search.trim().toLowerCase()));
  }, [threads, search]);

  const modes = [
    { id: 'chat' as const, label: t('playgroundModeChat'), icon: Sparkles },
    { id: 'images' as const, label: t('playgroundModeImages'), icon: Images },
    { id: 'video' as const, label: t('playgroundModeVideo'), icon: Video },
  ];

  function updateCurrentThread(updater: (thread: Thread) => Thread) {
    setThreads((current) => current.map((thread, index) => (index === selectedThread ? updater(thread) : thread)));
  }

  function createThreadTitle(source: string) {
    return source.trim().slice(0, 18) || t('playgroundThreadUntitled');
  }

  async function handleReferenceChange(event: ChangeEvent<HTMLInputElement>) {
    const files = Array.from(event.target.files || []);
    const nextImages = await Promise.all(
      files.map(
        (file) =>
          new Promise<string>((resolve, reject) => {
            const reader = new FileReader();
            reader.onload = () => resolve(String(reader.result || ''));
            reader.onerror = () => reject(reader.error);
            reader.readAsDataURL(file);
          }),
      ),
    );
    setReferenceImages(nextImages.filter(Boolean).slice(0, 3));
    event.target.value = '';
  }

  async function handleSend() {
    const value = prompt.trim();
    if (!value || !selectedModel || sending) return;

    const userMessage: Message = { role: 'user', kind: 'text', content: value };
    updateCurrentThread((thread) => ({
      ...thread,
      title: thread.title === t('playgroundThreadUntitled') ? createThreadTitle(value) : thread.title,
      messages: [...thread.messages, userMessage],
    }));
    setPrompt('');
    setSending(true);

    try {
      if (mode === 'chat') {
        const reply = await sendPlaygroundChat({
          model: selectedModel,
          prompt: value,
          referenceImages,
        });
        updateCurrentThread((thread) => ({
          ...thread,
          messages: [...thread.messages, { role: 'assistant', kind: 'text', content: reply }],
        }));
      } else if (mode === 'images') {
        const apiKey = await fetchPlaygroundTokenKey(Number(selectedTokenId));
        const imageUrl = await sendPlaygroundImage({
          model: selectedModel,
          prompt: value,
          referenceImages,
          apiKey,
        });
        updateCurrentThread((thread) => ({
          ...thread,
          messages: [...thread.messages, { role: 'assistant', kind: 'image', content: imageUrl }],
        }));
      } else {
        const apiKey = await fetchPlaygroundTokenKey(Number(selectedTokenId));
        updateCurrentThread((thread) => ({
          ...thread,
          messages: [...thread.messages, { role: 'assistant', kind: 'status', content: t('playgroundVideoPending') }],
        }));
        const taskId = await createPlaygroundVideo({
          model: selectedModel,
          prompt: value,
          referenceImages,
          apiKey,
        });

        let attempts = 0;
        let resultUrl = '';
        while (attempts < 20) {
          attempts += 1;
          await new Promise((resolve) => window.setTimeout(resolve, 3000));
          const task = await fetchPlaygroundVideoTask(taskId, apiKey);
          if (task.status === 'succeeded' && task.url) {
            resultUrl = task.url;
            break;
          }
          if (task.status === 'failed') {
            throw new Error(`Video task failed: ${taskId}`);
          }
        }
        if (!resultUrl) {
          throw new Error(`Video task timed out: ${taskId}`);
        }
        updateCurrentThread((thread) => ({
          ...thread,
          messages: [...thread.messages.filter((message) => message.kind !== 'status'), { role: 'assistant', kind: 'video', content: resultUrl }],
        }));
      }
    } catch (error: any) {
      updateCurrentThread((thread) => ({
        ...thread,
        messages: [
          ...thread.messages.filter((message) => message.kind !== 'status'),
          { role: 'assistant', kind: 'status', content: `${t('playgroundErrorPrefix')} ${error?.message || 'Unknown error'}` },
        ],
      }));
    } finally {
      setSending(false);
    }
  }

  function handleNewThread() {
    setThreads((current) => [
      {
        title: t('playgroundThreadUntitled'),
        messages: [initialAssistantMessage],
      },
      ...current,
    ]);
    setSelectedThread(0);
    setPrompt('');
    setReferenceImages([]);
  }

  function handleComposerKeyDown(event: KeyboardEvent<HTMLTextAreaElement>) {
    if (event.key !== 'Enter' || event.shiftKey) return;
    event.preventDefault();
    void handleSend();
  }

  return (
    <section className='overflow-hidden rounded-[30px] border border-slate-200 bg-white shadow-sm'>
      <div className='grid min-h-[760px] xl:grid-cols-[260px_minmax(0,1fr)]'>
        <aside className='flex flex-col border-r border-slate-200 bg-white'>
          <div className='flex items-center justify-between px-4 py-5'>
            <div className='grid h-10 w-10 place-items-center rounded-full bg-[linear-gradient(135deg,#f24bbd_0%,#8c6bff_45%,#3ab8ff_100%)] text-white shadow-[0_10px_30px_-18px_rgba(99,102,241,0.8)]'>
              <Sparkles className='h-4 w-4' />
            </div>
            <button
              type='button'
              className='inline-flex h-9 w-9 items-center justify-center rounded-xl text-slate-400 transition-colors hover:bg-slate-100 hover:text-slate-600'
            >
              <Send className='h-4 w-4 rotate-180' />
            </button>
          </div>

          <div className='px-3 pb-4'>
            <button
              type='button'
              onClick={handleNewThread}
              className='flex h-11 w-full items-center justify-center gap-2 rounded-2xl bg-[linear-gradient(90deg,#ef47c4_0%,#8e62ff_45%,#2bb5ff_100%)] px-4 text-sm font-medium text-white shadow-[0_18px_40px_-24px_rgba(76,145,255,0.7)]'
            >
              <Sparkles className='h-4 w-4' />
              {t('playgroundStartChat')}
            </button>
          </div>

          <div className='px-3'>
            <label className='flex h-10 items-center gap-2 rounded-2xl border border-slate-200 bg-white px-3 text-sm text-slate-400'>
              <Search className='h-4 w-4 shrink-0' />
              <input
                type='text'
                value={search}
                onChange={(event) => setSearch(event.target.value)}
                placeholder={t('playgroundSearchPlaceholder')}
                className='w-full bg-transparent text-sm text-slate-700 outline-none placeholder:text-slate-400'
              />
            </label>
          </div>

          <div className='mt-4 flex-1 overflow-y-auto px-3 pb-6'>
            <p className='px-2 text-[11px] font-semibold uppercase tracking-[0.2em] text-slate-400'>{t('playgroundRecentThreads')}</p>
            <div className='mt-3 space-y-1'>
              {filteredThreads.map(({ thread, index }) => {
                return (
                  <button
                    key={`${thread.title}-${index}`}
                    type='button'
                    onClick={() => setSelectedThread(index)}
                    className={
                      selectedThread === index
                        ? 'flex w-full items-center gap-3 rounded-2xl bg-slate-100 px-3 py-3 text-left'
                        : 'flex w-full items-center gap-3 rounded-2xl px-3 py-3 text-left transition-colors hover:bg-slate-50'
                    }
                  >
                    <div className='grid h-8 w-8 shrink-0 place-items-center rounded-xl border border-slate-200 bg-white text-slate-400'>
                      <Images className='h-4 w-4' />
                    </div>
                    <span className='truncate text-sm text-slate-600'>{thread.title}</span>
                  </button>
                );
              })}
            </div>
          </div>

          <div className='border-t border-slate-200 px-4 py-4'>
            <button
              type='button'
              className='flex items-center gap-3 text-sm font-medium text-slate-500 transition-colors hover:text-slate-900'
            >
              <Settings2 className='h-4 w-4' />
              {t('playgroundSettings')}
            </button>
          </div>
        </aside>

        <div className='flex min-w-0 flex-col bg-white'>
          <div className='flex flex-1 flex-col bg-[radial-gradient(circle_at_top,#fff7fb_0%,#ffffff_22%,#ffffff_100%)] px-5 py-6 lg:px-8 lg:py-8'>
            <div className='mx-auto flex w-full max-w-[860px] flex-1 flex-col'>
              <div className='flex-1 space-y-4 overflow-y-auto pb-6'>
                {activeMessages.map((message, index) => {
                  const isUser = message.role === 'user';
                  return (
                    <div key={index} className={isUser ? 'flex justify-end' : 'flex justify-start'}>
                      {message.kind === 'image' ? (
                        <div className='max-w-[78%] overflow-hidden rounded-[24px] border border-[#f0d8ee] bg-white p-3 shadow-[0_24px_60px_-40px_rgba(236,72,153,0.35)]'>
                          <img src={message.content} alt='Generated result' className='h-auto w-full rounded-[18px] object-cover' />
                        </div>
                      ) : message.kind === 'video' ? (
                        <div className='max-w-[78%] overflow-hidden rounded-[24px] border border-[#f0d8ee] bg-white p-3 shadow-[0_24px_60px_-40px_rgba(236,72,153,0.35)]'>
                          <video src={message.content} controls className='h-auto w-full rounded-[18px]' />
                        </div>
                      ) : (
                        <div
                          className={
                            isUser
                              ? 'max-w-[78%] rounded-[24px] rounded-br-[10px] bg-slate-950 px-5 py-4 text-[15px] leading-7 text-white shadow-[0_18px_40px_-28px_rgba(15,23,42,0.85)]'
                              : message.kind === 'status'
                                ? 'max-w-[78%] rounded-[24px] rounded-bl-[10px] border border-amber-200 bg-amber-50 px-5 py-4 text-[15px] leading-7 text-amber-700'
                                : 'max-w-[78%] rounded-[24px] rounded-bl-[10px] border border-[#f0d8ee] bg-white px-5 py-4 text-[15px] leading-7 text-slate-700 shadow-[0_24px_60px_-40px_rgba(236,72,153,0.35)]'
                          }
                        >
                          {message.content}
                        </div>
                      )}
                    </div>
                  );
                })}
                {models.error ? (
                  <div className='flex justify-start'>
                    <div className='max-w-[78%] rounded-[24px] rounded-bl-[10px] border border-amber-200 bg-amber-50 px-5 py-4 text-[15px] leading-7 text-amber-700'>
                      {t('playgroundErrorPrefix')} {models.error}
                    </div>
                  </div>
                ) : null}
                <div ref={messageEndRef} />
              </div>

              <div className='rounded-[30px] border border-[#f0d5ef] bg-white shadow-[0_24px_80px_-48px_rgba(236,72,153,0.45)]'>
                <div className='min-h-[104px] px-5 py-4'>
                  <textarea
                    value={prompt}
                    onChange={(event) => setPrompt(event.target.value)}
                    onKeyDown={handleComposerKeyDown}
                    placeholder={t('playgroundComposerPlaceholder')}
                    className='h-20 w-full resize-none bg-transparent text-[17px] leading-7 text-slate-700 outline-none placeholder:text-slate-400'
                  />
                </div>

                <div className='border-t border-slate-200 px-5 py-4'>
                  <div className='flex flex-wrap items-center gap-3'>
                    <div className='inline-flex h-9 items-center gap-2 rounded-full border border-slate-200 bg-slate-50 px-3 text-sm text-slate-600'>
                      <Settings2 className='h-4 w-4' />
                      <select
                        value={selectedModel}
                        onChange={(event) => setSelectedModel(event.target.value)}
                        className='bg-transparent pr-2 text-sm text-slate-600 outline-none'
                      >
                        {models.loading ? <option>{t('playgroundLoadingModels')}</option> : null}
                        {!models.loading && !modeModels.length ? <option>{t('playgroundNoModels')}</option> : null}
                        {modeModels.map((model) => (
                          <option key={model} value={model}>
                            {model}
                          </option>
                        ))}
                      </select>
                    </div>

                    {mode !== 'chat' ? (
                      <div className='inline-flex h-9 items-center gap-2 rounded-full border border-slate-200 bg-slate-50 px-3 text-sm text-slate-600'>
                        <Sparkles className='h-4 w-4' />
                        <select
                          value={selectedTokenId}
                          onChange={(event) => setSelectedTokenId(event.target.value)}
                          className='max-w-[220px] bg-transparent pr-2 text-sm text-slate-600 outline-none'
                        >
                          {tokens.loading ? <option value=''>{t('playgroundLoadingTokens')}</option> : null}
                          {!tokens.loading && !tokens.data?.length ? <option value=''>{t('playgroundNoTokens')}</option> : null}
                          {(tokens.data || []).map((token) => (
                            <option key={token.id} value={String(token.id)}>
                              {token.name || `${t('playgroundApiToken')} #${token.id}`}
                            </option>
                          ))}
                        </select>
                      </div>
                    ) : null}
                  </div>

                  <div className='mt-4'>
                    <p className='text-sm font-medium text-slate-600'>{t('playgroundReferenceImages')}</p>
                    <div className='mt-3 flex items-center gap-4'>
                      <button
                        type='button'
                        onClick={() => fileInputRef.current?.click()}
                        className='grid h-14 w-14 place-items-center rounded-2xl border border-slate-200 bg-white text-slate-400 transition-colors hover:border-slate-300 hover:text-slate-600'
                      >
                        <ImagePlus className='h-5 w-5' />
                      </button>
                      <div className='space-y-1'>
                        <p className='text-xs text-slate-400'>{t('playgroundReferenceHint')}</p>
                        {referenceImages.length ? (
                          <div className='flex flex-wrap gap-2'>
                            {referenceImages.map((image, index) => (
                              <img key={index} src={image} alt={`Reference ${index + 1}`} className='h-12 w-12 rounded-xl border border-slate-200 object-cover' />
                            ))}
                          </div>
                        ) : null}
                      </div>
                      <input
                        ref={fileInputRef}
                        type='file'
                        accept='image/png,image/jpeg,image/webp'
                        multiple
                        className='hidden'
                        onChange={handleReferenceChange}
                      />
                    </div>
                  </div>

                  <div className='mt-5 flex items-center justify-between gap-4'>
                    <div className='flex flex-wrap items-center gap-2'>
                      {modes.map((item) => {
                        const active = item.id === mode;
                        return (
                          <button
                            key={item.id}
                            type='button'
                            onClick={() => setMode(item.id)}
                            className={
                              active
                                ? 'inline-flex h-10 items-center gap-2 rounded-full bg-[#fbecfb] px-4 text-sm font-medium text-[#bf3ad1]'
                                : 'inline-flex h-10 items-center gap-2 rounded-full px-3 text-sm text-slate-500 transition-colors hover:bg-slate-50 hover:text-slate-900'
                            }
                          >
                            <item.icon className='h-4 w-4' />
                            {item.label}
                          </button>
                        );
                      })}
                    </div>

                    <button
                      type='button'
                      onClick={handleSend}
                      disabled={sending || !prompt.trim() || !selectedModel || (mode !== 'chat' && !selectedTokenId)}
                      className='grid h-9 w-9 place-items-center rounded-full bg-[linear-gradient(135deg,#f08be6_0%,#b572ff_50%,#7a9cff_100%)] text-white shadow-[0_16px_30px_-20px_rgba(168,85,247,0.85)]'
                    >
                      {sending ? <Loader2 className='h-4 w-4 animate-spin' /> : <Send className='h-4 w-4' />}
                    </button>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </section>
  );
}
