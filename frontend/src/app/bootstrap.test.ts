import { describe, expect, it } from 'vitest';
import { chooseInitialPath } from './bootstrap';
import { me, ORG_ID } from '../test/fixtures';

const empty = { ...me, memberships: [] };

describe('T-FE-NAV: выбор первого экрана по цели запуска', () => {
  it('открывает карточку документа по диплинку напоминания', () => {
    expect(chooseInitialPath({ kind: 'document', document_id: 'doc-1' }, me, null)).toBe('/d/doc-1');
  });

  it('открывает дашборд организации', () => {
    expect(chooseInitialPath({ kind: 'organization', organization_id: 'org-1' }, me, null)).toBe('/o/org-1');
  });

  it('открывает экран приглашения', () => {
    expect(chooseInitialPath({ kind: 'invite', invite_token: 'token' }, me, null)).toBe('/invite');
  });

  it('без организаций ведёт в онбординг', () => {
    expect(chooseInitialPath({ kind: 'none' }, empty, null)).toBe('/welcome');
  });

  it('возвращает последнюю открытую организацию', () => {
    expect(chooseInitialPath({ kind: 'none' }, me, ORG_ID)).toBe(`/o/${ORG_ID}`);
  });

  it('игнорирует последнюю организацию, если участие пропало', () => {
    expect(chooseInitialPath({ kind: 'none' }, me, 'другая')).toBe(`/o/${ORG_ID}`);
  });
});
