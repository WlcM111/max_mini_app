import { useEffect, useState } from 'react';

/** Совпадение медиазапроса с подпиской на изменения (окно MAX на компьютере меняет размер). */
export function useMediaQuery(query: string): boolean {
  const read = () => typeof window !== 'undefined' && typeof window.matchMedia === 'function' && window.matchMedia(query).matches;
  const [matches, setMatches] = useState(read);
  useEffect(() => {
    if (typeof window.matchMedia !== 'function') return undefined;
    const list = window.matchMedia(query);
    const onChange = () => setMatches(list.matches);
    onChange();
    list.addEventListener?.('change', onChange);
    return () => list.removeEventListener?.('change', onChange);
  }, [query]);
  return matches;
}

/** Размер окна в CSS-пикселях и масштаб экрана — для диагностики раскладки. */
export function useViewport(): { width: number; height: number; dpr: number } {
  const read = () => ({ width: window.innerWidth, height: window.innerHeight, dpr: window.devicePixelRatio || 1 });
  const [size, setSize] = useState(read);
  useEffect(() => {
    const onResize = () => setSize(read());
    window.addEventListener('resize', onResize);
    return () => window.removeEventListener('resize', onResize);
  }, []);
  return size;
}

/** Какая раскладка включена при такой ширине окна (пороги из layout.css). */
export function layoutName(width: number): string {
  if (width >= 1440) return 'широкий монитор: меню и три колонки';
  if (width >= 1024) return 'компьютер: боковое меню';
  if (width >= 820) return 'планшет: две колонки';
  return 'телефон';
}
