import { useSyncExternalStore } from 'react';
import { createPortal } from 'react-dom';
import { Icon } from './Icon';

type Tone = 'info' | 'success' | 'error';
interface ToastItem {
  id: number;
  text: string;
  tone: Tone;
  leaving: boolean;
}

let items: ToastItem[] = [];
let sequence = 0;
const listeners = new Set<() => void>();
const emit = () => listeners.forEach((listener) => listener());

/** Короткое уведомление вверху экрана; исчезает само через 3,5 секунды. */
export function toast(text: string, tone: Tone = 'info'): void {
  sequence += 1;
  const id = sequence;
  items = [...items.filter((item) => item.text !== text), { id, text, tone, leaving: false }].slice(-3);
  emit();
  window.setTimeout(() => {
    items = items.map((item) => (item.id === id ? { ...item, leaving: true } : item));
    emit();
    window.setTimeout(() => {
      items = items.filter((item) => item.id !== id);
      emit();
    }, 240);
  }, 3500);
}

function subscribe(listener: () => void): () => void {
  listeners.add(listener);
  return () => {
    listeners.delete(listener);
  };
}

const snapshot = () => items;

export function Toaster() {
  const list = useSyncExternalStore(subscribe, snapshot, snapshot);
  return createPortal(
    <div className="toaster" role="status" aria-live="polite">
      {list.map((item) => (
        <div key={item.id} className={`toast toast--${item.tone}`} data-leaving={item.leaving ? 'true' : 'false'}>
          <Icon name={item.tone === 'success' ? 'check' : item.tone === 'error' ? 'alert' : 'info'} size={18} />
          <span>{item.text}</span>
        </div>
      ))}
    </div>,
    document.body,
  );
}
