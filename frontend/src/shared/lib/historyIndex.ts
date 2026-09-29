/**
 * Позиция экрана в истории маршрутизатора в памяти (react-router): 0 — первый экран запуска.
 * Нужна, чтобы «Назад» с первого экрана (диплинк, экран после замены) вёл на главную,
 * а не в пустоту: navigate(-1) на позиции 0 ничего не делает.
 */
export interface HistoryIndex {
  key: string;
  index: number;
  known: Map<string, number>;
}

export function startHistory(key: string): HistoryIndex {
  return { key, index: 0, known: new Map([[key, 0]]) };
}

/** Учитывает переход: PUSH — вперёд, REPLACE — та же позиция, POP — позиция записи по ключу. */
export function trackHistory(history: HistoryIndex, action: string, key: string): HistoryIndex {
  if (key === history.key) return history;
  const known = new Map(history.known);
  let index: number;
  if (action === 'PUSH') {
    index = history.index + 1;
    // Записи «впереди» вытеснены новым экраном.
    for (const [entry, position] of known) if (position >= index) known.delete(entry);
  } else if (action === 'REPLACE') {
    index = history.index;
    known.delete(history.key);
  } else {
    index = known.get(key) ?? Math.max(0, history.index - 1);
  }
  known.set(key, index);
  return { key, index, known };
}
