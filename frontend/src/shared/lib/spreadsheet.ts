/** Чтение таблиц .xlsx и .csv в браузере без сторонних библиотек (CSP запрещает внешние скрипты). */
export type SheetRows = string[][];

const utf8 = new TextDecoder();

async function inflateRaw(data: ArrayBuffer): Promise<ArrayBuffer> {
  const stream = new Blob([data]).stream().pipeThrough(new DecompressionStream('deflate-raw'));
  return new Response(stream).arrayBuffer();
}

// Минимальный разбор ZIP: центральный каталог, методы 0 (store) и 8 (deflate).
async function readZip(buffer: ArrayBuffer, wanted: (name: string) => boolean): Promise<Map<string, string>> {
  const view = new DataView(buffer);
  let eocd = -1;
  for (let i = buffer.byteLength - 22; i >= Math.max(0, buffer.byteLength - 65_557); i -= 1) {
    if (view.getUint32(i, true) === 0x06054b50) {
      eocd = i;
      break;
    }
  }
  if (eocd < 0) throw new Error('Файл повреждён или это не .xlsx');
  const count = view.getUint16(eocd + 10, true);
  let offset = view.getUint32(eocd + 16, true);
  const out = new Map<string, string>();
  for (let n = 0; n < count && offset + 46 <= buffer.byteLength; n += 1) {
    if (view.getUint32(offset, true) !== 0x02014b50) break;
    const method = view.getUint16(offset + 10, true);
    const size = view.getUint32(offset + 20, true);
    const nameLength = view.getUint16(offset + 28, true);
    const extraLength = view.getUint16(offset + 30, true);
    const commentLength = view.getUint16(offset + 32, true);
    const local = view.getUint32(offset + 42, true);
    const name = utf8.decode(new Uint8Array(buffer, offset + 46, nameLength));
    offset += 46 + nameLength + extraLength + commentLength;
    if (!wanted(name)) continue;
    const start = local + 30 + view.getUint16(local + 26, true) + view.getUint16(local + 28, true);
    const raw = buffer.slice(start, start + size);
    const data = method === 0 ? raw : method === 8 ? await inflateRaw(raw) : null;
    if (data) out.set(name, utf8.decode(data));
  }
  return out;
}

const byTag = (node: Document | Element, tag: string): Element[] => Array.from(node.getElementsByTagNameNS('*', tag));

function columnIndex(ref: string): number {
  let index = 0;
  for (const letter of ref.replace(/[0-9]/g, '').toUpperCase()) index = index * 26 + (letter.charCodeAt(0) - 64);
  return index - 1;
}

async function readXlsx(buffer: ArrayBuffer): Promise<SheetRows> {
  const parser = new DOMParser();
  const xml = (text: string) => parser.parseFromString(text, 'application/xml');
  const meta = await readZip(buffer, (name) =>
    ['xl/workbook.xml', 'xl/_rels/workbook.xml.rels', 'xl/sharedStrings.xml'].includes(name),
  );
  // Берём первый лист книги по связям workbook.xml.rels.
  let sheetPath = 'xl/worksheets/sheet1.xml';
  const workbook = meta.get('xl/workbook.xml');
  const rels = meta.get('xl/_rels/workbook.xml.rels');
  if (workbook && rels) {
    const first = byTag(xml(workbook), 'sheet')[0];
    const relId = first?.getAttribute('r:id');
    const target = byTag(xml(rels), 'Relationship')
      .find((rel) => rel.getAttribute('Id') === relId)
      ?.getAttribute('Target');
    if (target) sheetPath = target.startsWith('/') ? target.slice(1) : `xl/${target.replace(/^\.\//, '')}`;
  }
  const sheet = (await readZip(buffer, (name) => name === sheetPath)).get(sheetPath);
  if (!sheet) throw new Error('В файле не найден лист с данными');
  const shared = meta.get('xl/sharedStrings.xml');
  const strings = shared ? byTag(xml(shared), 'si').map((si) => byTag(si, 't').map((t) => t.textContent ?? '').join('')) : [];
  const rows: SheetRows = [];
  for (const row of byTag(xml(sheet), 'row')) {
    const cells: string[] = [];
    for (const cell of byTag(row, 'c')) {
      const ref = cell.getAttribute('r');
      const index = ref ? columnIndex(ref) : cells.length;
      const type = cell.getAttribute('t');
      const raw = byTag(cell, 'v')[0]?.textContent ?? '';
      let value = raw;
      if (type === 's') value = strings[Number(raw)] ?? '';
      else if (type === 'inlineStr') value = byTag(cell, 't').map((t) => t.textContent ?? '').join('');
      else if (type === 'b') value = raw === '1' ? 'да' : 'нет';
      while (cells.length < index) cells.push('');
      cells[index] = value.trim();
    }
    rows.push(cells);
  }
  return rows.filter((cells) => cells.some((value) => value !== ''));
}

function decodeText(buffer: ArrayBuffer): string {
  try {
    return new TextDecoder('utf-8', { fatal: true }).decode(buffer).replace(/^\uFEFF/, '');
  } catch {
    return new TextDecoder('windows-1251').decode(buffer);
  }
}

/** CSV из Excel: разделитель ; , или табуляция, кавычки по RFC 4180. */
export function parseCsv(text: string): SheetRows {
  const firstLine = text.split(/\r?\n/, 1)[0] ?? '';
  const delimiter =
    [';', '\t', ',']
      .map((candidate) => [candidate, firstLine.split(candidate).length] as const)
      .sort((a, b) => b[1] - a[1])[0]?.[0] ?? ';';
  const rows: SheetRows = [];
  let row: string[] = [];
  let cell = '';
  let quoted = false;
  for (let i = 0; i < text.length; i += 1) {
    const ch = text.charAt(i);
    if (quoted) {
      if (ch === '"') {
        if (text.charAt(i + 1) === '"') {
          cell += '"';
          i += 1;
        } else quoted = false;
      } else cell += ch;
    } else if (ch === '"') quoted = true;
    else if (ch === delimiter) {
      row.push(cell.trim());
      cell = '';
    } else if (ch === '\n' || ch === '\r') {
      if (ch === '\r' && text.charAt(i + 1) === '\n') i += 1;
      row.push(cell.trim());
      rows.push(row);
      row = [];
      cell = '';
    } else cell += ch;
  }
  if (cell !== '' || row.length > 0) {
    row.push(cell.trim());
    rows.push(row);
  }
  return rows.filter((cells) => cells.some((value) => value !== ''));
}

/** Читает первый лист .xlsx или файл .csv в массив строк. */
export async function readSpreadsheet(file: File): Promise<SheetRows> {
  const buffer = await file.arrayBuffer();
  const head = new Uint8Array(buffer, 0, Math.min(2, buffer.byteLength));
  if (head[0] === 0x50 && head[1] === 0x4b) return readXlsx(buffer);
  if (/\.xls$/i.test(file.name)) throw new Error('Формат .xls не поддерживается — сохраните файл как .xlsx или .csv');
  return parseCsv(decodeText(buffer));
}

/** Буква столбца по номеру: 0 → A, 26 → AA. */
export function columnLetter(index: number): string {
  let value = index + 1;
  let out = '';
  while (value > 0) {
    const rest = (value - 1) % 26;
    out = String.fromCharCode(65 + rest) + out;
    value = Math.floor((value - 1) / 26);
  }
  return out;
}
