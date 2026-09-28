import { useState } from 'react';
import { useMutation } from '@tanstack/react-query';
import { messageForError } from '../../api/errors';
import type { CalendarExport } from '../../api/client';
import { downloadCalendar } from '../../platform/max/download';
import { Button } from '../../shared/ui/Button';
import { Icon } from '../../shared/ui/Icon';
import { toast } from '../../shared/ui/Toast';
import { createCalendarExport } from './api';

/** Реестр в Excel: та же одноразовая ссылка, что у календаря, с параметром format=xlsx. */
export function RegistryExportButton({ organizationId }: { organizationId: string }) {
  const [link, setLink] = useState<CalendarExport | null>(null);
  const prepare = useMutation({
    mutationFn: () => createCalendarExport(organizationId),
    onSuccess: (data) => setLink(data),
    onError: (error) => toast(messageForError(error), 'error'),
  });

  const url = link ? `${link.download_url}${link.download_url.includes('?') ? '&' : '?'}format=xlsx` : '';
  const fileName = link ? link.file_name.replace(/^vovremya-/, 'vovremya-reestr-').replace(/\.ics$/i, '.xlsx') : '';

  const download = () => {
    if (!link) return;
    void downloadCalendar(url, fileName).then((outcome) => {
      if (outcome === 'failed') toast('Не удалось открыть файл. Подготовьте его заново.', 'error');
      else toast('Реестр выгружен', 'success');
      setLink(null);
    });
  };

  return (
    <div className="export">
      {link ? (
        <>
          <div className="export__file">
            <Icon name="table" />
            <span>{fileName}</span>
          </div>
          <Button size="l" stretched icon="download" onClick={download}>
            Скачать реестр
          </Button>
        </>
      ) : (
        <Button variant="secondary" size="l" stretched icon="table" loading={prepare.isPending} onClick={() => prepare.mutate()}>
          Выгрузить реестр в Excel
        </Button>
      )}
    </div>
  );
}
