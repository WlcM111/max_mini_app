import { describe, expect, it } from 'vitest';
import { columnLetter, parseCsv, readSpreadsheet, XLSX_UNSUPPORTED } from './spreadsheet';

// Файл с гарантированным arrayBuffer(): не во всех версиях jsdom у File есть этот метод.
function fileOf(bytes: Uint8Array, name: string): File {
  const file = new File([bytes as BlobPart], name);
  if (typeof file.arrayBuffer !== 'function') {
    Object.defineProperty(file, 'arrayBuffer', { value: async () => bytes.slice().buffer });
  }
  return file;
}

// Минимальный ZIP без сжатия (метод 0) — достаточно для проверки чтения .xlsx.
function storedZip(files: Record<string, string>): Uint8Array {
  const encoder = new TextEncoder();
  const locals: Uint8Array[] = [];
  const centrals: Uint8Array[] = [];
  let offset = 0;
  for (const [name, text] of Object.entries(files)) {
    const nameBytes = encoder.encode(name);
    const data = encoder.encode(text);
    const local = new Uint8Array(30 + nameBytes.length + data.length);
    const lv = new DataView(local.buffer);
    lv.setUint32(0, 0x04034b50, true);
    lv.setUint32(18, data.length, true);
    lv.setUint32(22, data.length, true);
    lv.setUint16(26, nameBytes.length, true);
    local.set(nameBytes, 30);
    local.set(data, 30 + nameBytes.length);
    const central = new Uint8Array(46 + nameBytes.length);
    const cv = new DataView(central.buffer);
    cv.setUint32(0, 0x02014b50, true);
    cv.setUint32(20, data.length, true);
    cv.setUint32(24, data.length, true);
    cv.setUint16(28, nameBytes.length, true);
    cv.setUint32(42, offset, true);
    central.set(nameBytes, 46);
    locals.push(local);
    centrals.push(central);
    offset += local.length;
  }
  const centralSize = centrals.reduce((sum, part) => sum + part.length, 0);
  const end = new Uint8Array(22);
  const ev = new DataView(end.buffer);
  ev.setUint32(0, 0x06054b50, true);
  ev.setUint16(8, centrals.length, true);
  ev.setUint16(10, centrals.length, true);
  ev.setUint32(12, centralSize, true);
  ev.setUint32(16, offset, true);
  const out = new Uint8Array(offset + centralSize + end.length);
  let position = 0;
  for (const part of [...locals, ...centrals, end]) {
    out.set(part, position);
    position += part.length;
  }
  return out;
}

const NS = 'xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"';

describe('T-FE-IMPORT: чтение таблиц .csv и .xlsx', () => {
  it('разбирает CSV с кавычками и выбирает разделитель', () => {
    expect(parseCsv('Название;Действует до\n"Договор ""Альфа""; аренда";31.12.2026\n\n')).toEqual([
      ['Название', 'Действует до'],
      ['Договор "Альфа"; аренда', '31.12.2026'],
    ]);
    expect(parseCsv('Название,Номер\r\nУстав,\r\n')).toEqual([
      ['Название', 'Номер'],
      ['Устав', ''],
    ]);
  });

  it('читает CSV в кодировке Windows-1251', async () => {
    const hex = 'cde0e7e2e0ede8e53bd1f0eeea0ad3f1f2e0e23be1e5f1f1f0eef7edee0a';
    const bytes = new Uint8Array(hex.match(/../g)?.map((pair) => parseInt(pair, 16)) ?? []);
    await expect(readSpreadsheet(fileOf(bytes, 'reestr.csv'))).resolves.toEqual([
      ['Название', 'Срок'],
      ['Устав', 'бессрочно'],
    ]);
  });

  it('читает первый лист .xlsx по связям книги, общие и встроенные строки', async () => {
    const zip = storedZip({
      'xl/workbook.xml': `<workbook ${NS} xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><sheets><sheet name="Реестр" sheetId="1" r:id="rId1"/></sheets></workbook>`,
      'xl/_rels/workbook.xml.rels':
        '<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Target="worksheets/data.xml"/></Relationships>',
      'xl/sharedStrings.xml': `<sst ${NS}><si><t>Название</t></si><si><t>Действует до</t></si><si><r><t>Лицензия </t></r><r><t>на алкоголь</t></r></si></sst>`,
      'xl/worksheets/data.xml':
        `<worksheet ${NS}><sheetData>` +
        '<row r="1"><c r="A1" t="s"><v>0</v></c><c r="C1" t="s"><v>1</v></c></row>' +
        '<row r="2"><c r="A2" t="s"><v>2</v></c><c r="B2" t="inlineStr"><is><t>78РПА0012345</t></is></c><c r="C2"><v>47190</v></c></row>' +
        '<row r="3"><c r="A3" t="b"><v>1</v></c></row>' +
        '</sheetData></worksheet>',
    });
    await expect(readSpreadsheet(fileOf(zip, 'reestr.xlsx'))).resolves.toEqual([
      ['Название', '', 'Действует до'],
      ['Лицензия на алкоголь', '78РПА0012345', '47190'],
      ['да'],
    ]);
  });

  it('без распаковки deflate-raw предлагает сохранить таблицу в CSV', async () => {
    const name = 'xl/workbook.xml';
    const zip = storedZip({ [name]: `<workbook ${NS}/>` });
    // Метод 8 (deflate) в центральном каталоге: чтение дойдёт до распаковки.
    new DataView(zip.buffer).setUint16(zip.length - 22 - 46 - name.length + 10, 8, true);
    const original = globalThis.DecompressionStream;
    Object.defineProperty(globalThis, 'DecompressionStream', { value: undefined, configurable: true, writable: true });
    try {
      await expect(readSpreadsheet(fileOf(zip, 'reestr.xlsx'))).rejects.toThrow(XLSX_UNSUPPORTED);
    } finally {
      Object.defineProperty(globalThis, 'DecompressionStream', { value: original, configurable: true, writable: true });
    }
  });

  it('отклоняет старый формат .xls', async () => {
    await expect(readSpreadsheet(fileOf(new Uint8Array([0xd0, 0xcf, 0x11, 0xe0]), 'old.xls'))).rejects.toThrow(/\.xls не поддерживается/);
  });

  it('называет столбцы буквами Excel', () => {
    expect([0, 25, 26, 701, 702].map(columnLetter)).toEqual(['A', 'Z', 'AA', 'ZZ', 'AAA']);
  });
});
