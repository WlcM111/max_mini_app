/** Сжимает фото перед отправкой: JPEG до 1600 px по большей стороне (обычно 250–500 КБ). */
export async function prepareImage(file: File, maxSide = 1600): Promise<{ base64: string; dataUrl: string }> {
  const bitmap = await createImageBitmap(file, { imageOrientation: 'from-image' });
  const scale = Math.min(1, maxSide / Math.max(bitmap.width, bitmap.height));
  const canvas = document.createElement('canvas');
  canvas.width = Math.max(1, Math.round(bitmap.width * scale));
  canvas.height = Math.max(1, Math.round(bitmap.height * scale));
  const context = canvas.getContext('2d');
  if (!context) throw new Error('Не удалось обработать фото');
  context.drawImage(bitmap, 0, 0, canvas.width, canvas.height);
  bitmap.close();
  const dataUrl = canvas.toDataURL('image/jpeg', 0.85);
  return { dataUrl, base64: dataUrl.slice(dataUrl.indexOf(',') + 1) };
}
