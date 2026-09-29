import { describe, expect, it } from 'vitest';
import { startHistory, trackHistory } from './historyIndex';

describe('T-FE-NAV: позиция экрана в истории', () => {
  it('диплинк «Продлил»: после сохранения с заменой экрана назад идти некуда', () => {
    let history = startHistory('default');
    history = trackHistory(history, 'REPLACE', 'card');
    expect(history.index).toBe(0);
  });

  it('обычный путь: переходы вперёд и назад', () => {
    let history = startHistory('default');
    history = trackHistory(history, 'PUSH', 'list');
    history = trackHistory(history, 'PUSH', 'form');
    expect(history.index).toBe(2);
    history = trackHistory(history, 'REPLACE', 'card');
    expect(history.index).toBe(2);
    history = trackHistory(history, 'POP', 'list');
    expect(history.index).toBe(1);
    history = trackHistory(history, 'POP', 'default');
    expect(history.index).toBe(0);
  });

  it('возврат на два шага и новый переход после него', () => {
    let history = startHistory('default');
    history = trackHistory(history, 'PUSH', 'documents');
    history = trackHistory(history, 'PUSH', 'typical');
    history = trackHistory(history, 'PUSH', 'dates');
    history = trackHistory(history, 'POP', 'documents');
    expect(history.index).toBe(1);
    history = trackHistory(history, 'PUSH', 'card');
    expect(history.index).toBe(2);
    expect(history.known.has('dates')).toBe(false);
  });

  it('повторный рендер того же экрана позицию не меняет', () => {
    const history = trackHistory(startHistory('default'), 'PUSH', 'list');
    expect(trackHistory(history, 'PUSH', 'list')).toBe(history);
  });
});
