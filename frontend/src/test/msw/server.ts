import { setupServer } from 'msw/node';
import { handlers } from './handlers';

/** Управляемый сервер контролируемых ответов API для тестов компонентов. */
export const server = setupServer(...handlers);
