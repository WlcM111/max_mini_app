interface Decoded {
  image: CanvasImageSource;
  width: number;
  height: number;
  release: () => void;
}

// createImageBitmap есть не во всех WebView; запасной путь — <img> с data URL (CSP разрешает data:, но не blob:).
async function decode(file: File): Promise<Decoded> {
  if (typeof createImageBitmap === 'function') {
    try {
      const bitmap = await createImageBitmap(file, { imageOrientation: 'from-image' });
      return { image: bitmap, width: bitmap.width, height: bitmap.height, release: () => bitmap.close() };
    } catch {
      /* формат не поддержан этим способом — пробуем через <img> */
    }
  }
  const url = await new Promise<string>((resolve, reject) => {
    const reader = new FileReader();
    reader.onload = () => resolve(String(reader.result));
    reader.onerror = () => reject(new Error('Не удалось прочитать фото'));
    reader.readAsDataURL(file);
  });
  const img = new Image();
  img.src = url;
  await img.decode();
  return { image: img, width: img.naturalWidth, height: img.naturalHeight, release: () => undefined };
}

/** Сжимает фото перед отправкой: JPEG до 1600 px по большей стороне (обычно 250–500 КБ). */
export async function prepareImage(file: File, maxSide = 1600): Promise<{ base64: string; dataUrl: string }> {
  const source = await decode(file);
  try {
    const scale = Math.min(1, maxSide / Math.max(source.width, source.height));
    const canvas = document.createElement('canvas');
    canvas.width = Math.max(1, Math.round(source.width * scale));
    canvas.height = Math.max(1, Math.round(source.height * scale));
    const context = canvas.getContext('2d');
    if (!context) throw new Error('Не удалось обработать фото');
    context.drawImage(source.image, 0, 0, canvas.width, canvas.height);
    const dataUrl = canvas.toDataURL('image/jpeg', 0.85);
    return { dataUrl, base64: dataUrl.slice(dataUrl.indexOf(',') + 1) };
  } finally {
    source.release();
  }
}
