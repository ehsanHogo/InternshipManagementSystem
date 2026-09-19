import { Pipe, PipeTransform } from '@angular/core';

import { formatJalaliDate } from './jalali-date.utils';

@Pipe({
  name: 'jalaliDate',
  standalone: true
})
export class JalaliDatePipe implements PipeTransform {
  transform(value: string | Date | null | undefined, includeTime = false): string {
    if (!value) return '—';
    return formatJalaliDate(value, includeTime);
  }
}
