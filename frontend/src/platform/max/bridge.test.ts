import { beforeEach, describe, expect, it, vi } from 'vitest';
import { getBridge, resetBridge, unavailableBridge } from './bridge';
import { classifyScan } from './codeReader';
import { isSafeExternalUrl } from './links';
import { BridgeError, toBridgeError } from './types';

declare global {
  interface Window {
    WebApp?: unknown;
  }
}

describe('T-FE-MAX: адаптер SDK MAX', () => {
  beforeEach(() => {
    resetBridge();
    delete window.WebApp;
  });

  it('вне MAX возвращает недоступное окружение', async () => {
    const bridge = await getBridge();
    expect(bridge.kind).toBe('unavailable');
    expect(bridge.initData).toBe('');
    await expect(bridge.openLink('https://example.test')).rejects.toBeInstanceOf(BridgeError);
  });

  it('использует window.WebApp, когда он есть', async () => {
    const show = vi.fn();
    const openLink = vi.fn().mockResolvedValue(undefined);
    const notification = vi.fn();
    window.WebApp = {
      initData: 'auth_date=1&hash=abc',
      platform: 'ios',
      version: '26.2.8',
      BackButton: { show, hide: vi.fn(), onClick: vi.fn(), offClick: vi.fn() },
      HapticFeedback: { notificationOccurred: notification },
      openLink,
      openCodeReader: vi.fn().mockResolvedValue('https://example.test/doc'),
    };
    const bridge = await getBridge();
    expect(bridge.kind).toBe('max');
    expect(bridge.platform).toBe('ios');
    expect(bridge.version).toBe('26.2.8');
    expect(bridge.canScanQr).toBe(true);
    bridge.backButton?.show();
    expect(show).toHaveBeenCalled();
    await bridge.openLink('https://example.test');
    expect(openLink).toHaveBeenCalledWith('https://example.test');
    bridge.haptic('success');
    expect(notification).toHaveBeenCalledWith('success');
  });

  it('не вызывает тактильный отклик на desktop и web (F-25)', async () => {
    const notification = vi.fn();
    window.WebApp = {
      initData: 'auth_date=1&hash=abc',
      platform: 'desktop',
      HapticFeedback: { notificationOccurred: notification },
    };
    const bridge = await getBridge();
    bridge.haptic('success');
    expect(notification).not.toHaveBeenCalled();
  });

  it('приводит отказ SDK к BridgeError с кодом', () => {
    expect(toBridgeError({ error: { code: 'client.download_file.request_timeout' } }).code).toBe(
      'client.download_file.request_timeout',
    );
    expect(toBridgeError(new Error('прочее')).code).toBe('unknown');
  });

  it('разбирает результат сканера кода по правилу spec §6', () => {
    expect(classifyScan('https://example.test/doc')).toEqual({ kind: 'url', value: 'https://example.test/doc' });
    expect(classifyScan('АЛ-123')).toEqual({ kind: 'number', value: 'АЛ-123' });
    expect(classifyScan('x'.repeat(200))).toEqual({ kind: 'unknown' });
  });

  it('разрешает только абсолютные https-ссылки', () => {
    expect(isSafeExternalUrl('https://example.test')).toBe(true);
    expect(isSafeExternalUrl('http://example.test')).toBe(false);
    expect(isSafeExternalUrl('javascript:alert(1)')).toBe(false);
    expect(isSafeExternalUrl('/relative')).toBe(false);
  });

  it('недоступное окружение отвечает отказом на все действия', async () => {
    const bridge = unavailableBridge();
    await expect(bridge.downloadFile('https://example.test/f.ics', 'f.ics')).rejects.toBeInstanceOf(BridgeError);
    await expect(bridge.shareMaxContent({ text: 't', link: 'https://example.test' })).rejects.toBeInstanceOf(BridgeError);
    expect(bridge.canScanQr).toBe(false);
  });
});
