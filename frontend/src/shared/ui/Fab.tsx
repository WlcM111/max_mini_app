import { useEffect, useState } from 'react';
import { Icon, type IconName } from './Icon';

/** Сворачивает плавающую кнопку при прокрутке вниз и разворачивает при прокрутке вверх. */
function useScrollCollapse(threshold = 96): boolean {
  const [collapsed, setCollapsed] = useState(false);
  useEffect(() => {
    let last = window.scrollY;
    let frame = 0;
    const onScroll = () => {
      if (frame) return;
      frame = window.requestAnimationFrame(() => {
        frame = 0;
        const y = window.scrollY;
        if (y <= threshold || y < last - 6) setCollapsed(false);
        else if (y > last + 6) setCollapsed(true);
        last = y;
      });
    };
    window.addEventListener('scroll', onScroll, { passive: true });
    return () => {
      window.removeEventListener('scroll', onScroll);
      if (frame) window.cancelAnimationFrame(frame);
    };
  }, [threshold]);
  return collapsed;
}

export function Fab({ icon, label, onClick }: { icon: IconName; label: string; onClick: () => void }) {
  const collapsed = useScrollCollapse();
  return (
    <button type="button" className="fab" data-layer="bottom" data-collapsed={collapsed ? 'true' : 'false'} onClick={onClick}>
      <Icon name={icon} size={22} />
      <span className="fab__label">{label}</span>
    </button>
  );
}
