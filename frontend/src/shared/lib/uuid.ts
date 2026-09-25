const UUID_V4 = /^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/;

/** Проверяет UUID версии 4 в нижнем регистре (шаблон OpenAPI). */
export function isUuidV4(value: string): boolean {
  return UUID_V4.test(value);
}

/**
 * Генерирует UUID версии 4 для идемпотентных операций создания.
 * Значение создаётся один раз при открытии формы и переиспользуется при повторе.
 */
export function uuidV4(): string {
  const api = globalThis.crypto;
  if (api && typeof api.randomUUID === 'function') return api.randomUUID();
  const bytes = new Uint8Array(16);
  if (api && typeof api.getRandomValues === 'function') api.getRandomValues(bytes);
  else for (let i = 0; i < bytes.length; i += 1) bytes[i] = Math.floor(Math.random() * 256);
  bytes[6] = ((bytes[6] ?? 0) & 0x0f) | 0x40;
  bytes[8] = ((bytes[8] ?? 0) & 0x3f) | 0x80;
  const hex = Array.from(bytes, (b) => b.toString(16).padStart(2, '0')).join('');
  return `${hex.slice(0, 8)}-${hex.slice(8, 12)}-${hex.slice(12, 16)}-${hex.slice(16, 20)}-${hex.slice(20)}`;
}
