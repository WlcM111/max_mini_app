import { BrandMark } from './BrandMark';

/** Заставка на время запуска: проверка данных MAX и загрузка профиля. */
export function SplashView() {
  return (
    <div className="splash" role="status" aria-live="polite">
      <BrandMark size={72} animated />
      <p className="splash__name">Вовремя</p>
      <p className="splash__text">Загружаем сроки…</p>
    </div>
  );
}
