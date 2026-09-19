import { toGregorian, toJalaali } from 'jalaali-js';

export interface JalaliDateParts {
  year: number;
  month: number;
  day: number;
}

const TEHRAN_TIME_ZONE = 'Asia/Tehran';
const DATE_ONLY_PATTERN = /^(\d{4})-(\d{2})-(\d{2})$/;

export function gregorianDateToJalali(value: string): JalaliDateParts | null {
  const dateOnlyMatch = DATE_ONLY_PATTERN.exec(value);
  let year: number;
  let month: number;
  let day: number;

  if (dateOnlyMatch) {
    year = Number(dateOnlyMatch[1]);
    month = Number(dateOnlyMatch[2]);
    day = Number(dateOnlyMatch[3]);
  } else {
    const date = new Date(value);
    if (Number.isNaN(date.getTime())) return null;
    const parts = gregorianPartsInTehran(date);
    year = parts.year;
    month = parts.month;
    day = parts.day;
  }

  const jalali = toJalaali(year, month, day);
  return { year: jalali.jy, month: jalali.jm, day: jalali.jd };
}

export function jalaliToUtcDate(year: number, month: number, day: number): string {
  const gregorian = toGregorian(year, month, day);
  return `${gregorian.gy.toString().padStart(4, '0')}-${gregorian.gm.toString().padStart(2, '0')}-${gregorian.gd.toString().padStart(2, '0')}`;
}

export function todayInJalali(): JalaliDateParts {
  const gregorian = gregorianPartsInTehran(new Date());
  const jalali = toJalaali(gregorian.year, gregorian.month, gregorian.day);
  return { year: jalali.jy, month: jalali.jm, day: jalali.jd };
}

export function formatJalaliDate(value: string | Date, includeTime = false): string {
  const rawValue = value instanceof Date ? value.toISOString() : value;
  const jalali = gregorianDateToJalali(rawValue);
  if (!jalali) return '—';

  const date = `${jalali.year.toString().padStart(4, '0')}/${jalali.month.toString().padStart(2, '0')}/${jalali.day.toString().padStart(2, '0')}`;
  if (!includeTime) return toPersianDigits(date);

  const instant = value instanceof Date ? value : new Date(value);
  if (Number.isNaN(instant.getTime())) return toPersianDigits(date);
  const timeParts = new Intl.DateTimeFormat('en-GB', {
    timeZone: TEHRAN_TIME_ZONE,
    hour: '2-digit',
    minute: '2-digit',
    hourCycle: 'h23'
  }).formatToParts(instant);
  const hour = timeParts.find((part) => part.type === 'hour')?.value ?? '00';
  const minute = timeParts.find((part) => part.type === 'minute')?.value ?? '00';
  return toPersianDigits(`${date} ${hour}:${minute}`);
}

export function toPersianDigits(value: string | number): string {
  return String(value).replace(/\d/g, (digit) => '۰۱۲۳۴۵۶۷۸۹'[Number(digit)]);
}

function gregorianPartsInTehran(date: Date): { year: number; month: number; day: number } {
  const parts = new Intl.DateTimeFormat('en-CA', {
    timeZone: TEHRAN_TIME_ZONE,
    year: 'numeric',
    month: '2-digit',
    day: '2-digit'
  }).formatToParts(date);

  return {
    year: Number(parts.find((part) => part.type === 'year')?.value),
    month: Number(parts.find((part) => part.type === 'month')?.value),
    day: Number(parts.find((part) => part.type === 'day')?.value)
  };
}
