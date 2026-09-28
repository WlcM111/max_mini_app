/** Направление последнего перехода: по нему экран выбирает анимацию появления. */
export type NavDirection = 'forward' | 'back' | 'fade' | 'none';

let current: NavDirection = 'fade';

export function setNavDirection(value: NavDirection): void {
  current = value;
}

export function getNavDirection(): NavDirection {
  return current;
}
