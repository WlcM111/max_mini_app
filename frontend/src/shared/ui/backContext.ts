import { createContext } from 'react';

/** Запасная кнопка «Назад» в шапке, если у клиента MAX нет системной. */
export const BackContext = createContext<(() => void) | null>(null);
