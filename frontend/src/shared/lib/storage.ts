// Безопасные обёртки над хранилищами браузера: в приватном режиме доступ может
// бросать исключение, приложение при этом обязано продолжать работу.

type Kind = 'session' | 'local';

function area(kind: Kind): Storage | null {
  try {
    return kind === 'session' ? window.sessionStorage : window.localStorage;
  } catch {
    return null;
  }
}

export function readStorage(kind: Kind, key: string): string | null {
  try {
    return area(kind)?.getItem(key) ?? null;
  } catch {
    return null;
  }
}

export function writeStorage(kind: Kind, key: string, value: string): void {
  try {
    area(kind)?.setItem(key, value);
  } catch {
    /* хранилище недоступно — работаем только в памяти */
  }
}

export function removeStorage(kind: Kind, key: string): void {
  try {
    area(kind)?.removeItem(key);
  } catch {
    /* см. выше */
  }
}
