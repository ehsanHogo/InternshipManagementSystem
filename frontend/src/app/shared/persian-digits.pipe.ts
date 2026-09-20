import { Pipe, PipeTransform } from '@angular/core';

import { toPersianDigits } from './jalali-date/jalali-date.utils';

@Pipe({
  name: 'persianDigits',
  standalone: true
})
export class PersianDigitsPipe implements PipeTransform {
  transform(value: string | number | null | undefined): string {
    if (value === null || value === undefined) return '—';
    return toPersianDigits(value);
  }
}
