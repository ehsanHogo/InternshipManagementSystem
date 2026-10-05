import { DecimalPipe } from '@angular/common';
import { Component, input } from '@angular/core';
import { PersianDigitsPipe } from './persian-digits.pipe';

@Component({
  selector: 'app-company-rating',
  imports: [DecimalPipe, PersianDigitsPipe],
  template: `
    <div class="company-rating">
      <span class="label">امتیاز شرکت از دید کارآموزان قبلی</span>
      @if (average() != null && (count() ?? 0) > 0) {
        <div><span class="stars" aria-hidden="true" dir="ltr">{{ stars() }}</span>
          <strong>{{ average() | number:'1.1-1' | persianDigits }} از ۵</strong></div>
        <small>بر اساس {{ count() | persianDigits }} ارزیابی</small>
      } @else {
        <span>هنوز ارزیابی‌ای ثبت نشده است</span>
      }
    </div>
  `,
  styles: `
    .company-rating { display: grid; gap: .35rem; margin: .8rem 0; }
    .label, small { color: var(--p-text-muted-color); }
    .stars { color: var(--p-orange-500); letter-spacing: .12rem; margin-inline-end: .5rem; }
  `
})
export class CompanyRatingComponent {
  readonly average = input<number | null | undefined>(null);
  readonly count = input<number | undefined>(0);
  stars(): string {
    const filled = Math.round(this.average() ?? 0);
    return '★'.repeat(filled) + '☆'.repeat(5 - filled);
  }
}
