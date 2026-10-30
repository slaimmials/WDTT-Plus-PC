export function bytes(value: number | undefined): string {
  const n = value ?? 0;
  if (n < 1024) return `${n} Б`;
  const units = ['КБ', 'МБ', 'ГБ', 'ТБ'];
  let v = n / 1024;
  let i = 0;
  while (v >= 1024 && i < units.length - 1) {
    v /= 1024;
    i++;
  }
  return `${v.toFixed(v < 10 ? 2 : 1)} ${units[i]}`;
}

export function rate(bytesPerSecond: number): string {
  return `${bytes(Math.max(0, Math.round(bytesPerSecond)))}/с`;
}

export function duration(ms: number): string {
  const total = Math.max(0, Math.floor(ms / 1000));
  const h = Math.floor(total / 3600);
  const m = Math.floor((total % 3600) / 60);
  const s = total % 60;
  const pad = (x: number) => String(x).padStart(2, '0');
  return h > 0 ? `${h}:${pad(m)}:${pad(s)}` : `${pad(m)}:${pad(s)}`;
}

export function dateTime(unixSeconds: number | undefined): string {
  if (!unixSeconds) return '—';
  return new Date(unixSeconds * 1000).toLocaleString('ru-RU', { dateStyle: 'short', timeStyle: 'short' });
}

export function time(ms: number): string {
  return new Date(ms).toLocaleTimeString('ru-RU');
}
