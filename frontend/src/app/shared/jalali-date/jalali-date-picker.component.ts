import { Component, ElementRef, HostListener, forwardRef, inject } from '@angular/core';
import { ControlValueAccessor, NG_VALUE_ACCESSOR } from '@angular/forms';
import { jalaaliMonthLength, toGregorian } from 'jalaali-js';

import {
  gregorianDateToJalali,
  jalaliToUtcDate,
  todayInJalali,
  toPersianDigits
} from './jalali-date.utils';

interface CalendarDay {
  day: number;
  today: boolean;
  selected: boolean;
}

const MONTH_NAMES = [
  'فروردین',
  'اردیبهشت',
  'خرداد',
  'تیر',
  'مرداد',
  'شهریور',
  'مهر',
  'آبان',
  'آذر',
  'دی',
  'بهمن',
  'اسفند'
];

@Component({
  selector: 'app-jalali-date-picker',
  standalone: true,
  providers: [
    {
      provide: NG_VALUE_ACCESSOR,
      useExisting: forwardRef(() => JalaliDatePickerComponent),
      multi: true
    }
  ],
  template: `
    <div class="picker">
      <button
        class="date-input"
        type="button"
        [class.placeholder]="!selectedDate"
        [disabled]="disabled"
        [attr.aria-expanded]="open"
        aria-haspopup="dialog"
        (click)="toggle()"
      >
        <span>{{ displayValue }}</span>
        <i class="pi pi-calendar" aria-hidden="true"></i>
      </button>

      @if (open) {
        <div class="calendar" role="dialog" aria-label="انتخاب تاریخ شمسی">
          <div class="calendar-header">
            <button type="button" aria-label="ماه قبل" (click)="previousMonth()"><i class="pi pi-chevron-right"></i></button>
            <strong>{{ monthTitle }}</strong>
            <button type="button" aria-label="ماه بعد" (click)="nextMonth()"><i class="pi pi-chevron-left"></i></button>
          </div>
          <div class="weekdays" aria-hidden="true">
            @for (weekday of weekdays; track weekday) { <span>{{ weekday }}</span> }
          </div>
          <div class="days">
            @for (blank of leadingBlanks; track $index) { <span class="blank"></span> }
            @for (item of days; track item.day) {
              <button
                type="button"
                [class.today]="item.today"
                [class.selected]="item.selected"
                [attr.aria-label]="dayAriaLabel(item.day)"
                [attr.aria-pressed]="item.selected"
                (click)="selectDay(item.day)"
              >{{ persianNumber(item.day) }}</button>
            }
          </div>
          <div class="calendar-footer">
            <button type="button" (click)="selectToday()">امروز</button>
            @if (selectedDate) { <button type="button" class="clear" (click)="clear()">پاک کردن</button> }
          </div>
        </div>
      }
    </div>
  `,
  styles: `
    :host { display: block; width: 100%; }
    .picker { position: relative; width: 100%; }
    button { font: inherit; }
    .date-input {
      display: flex;
      align-items: center;
      justify-content: space-between;
      width: 100%;
      min-height: 2.65rem;
      padding: 0.65rem 0.75rem;
      border: 1px solid #cbd5e1;
      border-radius: 6px;
      color: #1e293b;
      background: #fff;
      cursor: pointer;
      text-align: right;
    }
    .date-input:hover:not(:disabled), .date-input:focus-visible { border-color: #3b82f6; outline: none; }
    .date-input:focus-visible { box-shadow: 0 0 0 0.2rem #bfdbfe; }
    .date-input.placeholder { color: #94a3b8; font-weight: 400; }
    .date-input:disabled { color: #94a3b8; background: #f1f5f9; cursor: default; }
    .date-input i { color: #64748b; }
    .calendar {
      position: absolute;
      z-index: 1000;
      top: calc(100% + 0.4rem);
      right: 0;
      width: min(19rem, calc(100vw - 2rem));
      padding: 0.75rem;
      border: 1px solid #e2e8f0;
      border-radius: 0.8rem;
      background: #fff;
      box-shadow: 0 12px 30px rgb(15 23 42 / 16%);
    }
    .calendar-header, .calendar-footer { display: flex; align-items: center; justify-content: space-between; }
    .calendar-header { margin-bottom: 0.7rem; }
    .calendar-header button, .calendar-footer button {
      border: 0;
      border-radius: 0.45rem;
      color: #2563eb;
      background: transparent;
      cursor: pointer;
    }
    .calendar-header button { width: 2.25rem; height: 2.25rem; }
    .calendar-header button:hover, .calendar-footer button:hover { background: #eff6ff; }
    .calendar-header strong { color: #1e293b; }
    .weekdays, .days { display: grid; grid-template-columns: repeat(7, 1fr); gap: 0.2rem; }
    .weekdays { margin-bottom: 0.25rem; color: #64748b; font-size: 0.75rem; text-align: center; }
    .weekdays span { padding-block: 0.35rem; }
    .days button, .blank { aspect-ratio: 1; }
    .days button {
      border: 0;
      border-radius: 50%;
      color: #334155;
      background: transparent;
      cursor: pointer;
    }
    .days button:hover { background: #eff6ff; }
    .days button.today { border: 1px solid #60a5fa; }
    .days button.selected { color: #fff; background: #2563eb; }
    .calendar-footer { margin-top: 0.65rem; padding-top: 0.55rem; border-top: 1px solid #e2e8f0; }
    .calendar-footer button { padding: 0.4rem 0.65rem; font-size: 0.82rem; }
    .calendar-footer .clear { color: #dc2626; }
    .calendar-footer .clear:hover { background: #fef2f2; }
  `
})
export class JalaliDatePickerComponent implements ControlValueAccessor {
  private readonly element = inject(ElementRef<HTMLElement>);

  readonly weekdays = ['ش', 'ی', 'د', 'س', 'چ', 'پ', 'ج'];
  open = false;
  disabled = false;
  selectedDate: { year: number; month: number; day: number } | null = null;
  viewYear = todayInJalali().year;
  viewMonth = todayInJalali().month;

  private onChange: (value: string) => void = () => undefined;
  private onTouched: () => void = () => undefined;

  get displayValue(): string {
    if (!this.selectedDate) return 'انتخاب تاریخ';
    const { year, month, day } = this.selectedDate;
    return toPersianDigits(`${year.toString().padStart(4, '0')}/${month.toString().padStart(2, '0')}/${day.toString().padStart(2, '0')}`);
  }

  get monthTitle(): string {
    return `${MONTH_NAMES[this.viewMonth - 1]} ${toPersianDigits(this.viewYear)}`;
  }

  get leadingBlanks(): number[] {
    const firstDay = toGregorian(this.viewYear, this.viewMonth, 1);
    const weekday = new Date(Date.UTC(firstDay.gy, firstDay.gm - 1, firstDay.gd)).getUTCDay();
    return Array.from({ length: (weekday + 1) % 7 }, (_, index) => index);
  }

  get days(): CalendarDay[] {
    const today = todayInJalali();
    return Array.from({ length: jalaaliMonthLength(this.viewYear, this.viewMonth) }, (_, index) => {
      const day = index + 1;
      return {
        day,
        today: today.year === this.viewYear && today.month === this.viewMonth && today.day === day,
        selected: this.selectedDate?.year === this.viewYear && this.selectedDate.month === this.viewMonth && this.selectedDate.day === day
      };
    });
  }

  writeValue(value: string | null): void {
    this.selectedDate = value ? gregorianDateToJalali(value) : null;
    if (this.selectedDate) {
      this.viewYear = this.selectedDate.year;
      this.viewMonth = this.selectedDate.month;
    }
  }

  registerOnChange(fn: (value: string) => void): void {
    this.onChange = fn;
  }

  registerOnTouched(fn: () => void): void {
    this.onTouched = fn;
  }

  setDisabledState(disabled: boolean): void {
    this.disabled = disabled;
    if (disabled) this.open = false;
  }

  toggle(): void {
    if (!this.disabled) this.open = !this.open;
  }

  previousMonth(): void {
    if (this.viewMonth === 1) {
      this.viewMonth = 12;
      this.viewYear--;
    } else {
      this.viewMonth--;
    }
  }

  nextMonth(): void {
    if (this.viewMonth === 12) {
      this.viewMonth = 1;
      this.viewYear++;
    } else {
      this.viewMonth++;
    }
  }

  selectDay(day: number): void {
    this.selectedDate = { year: this.viewYear, month: this.viewMonth, day };
    this.onChange(jalaliToUtcDate(this.viewYear, this.viewMonth, day));
    this.onTouched();
    this.open = false;
  }

  selectToday(): void {
    const today = todayInJalali();
    this.viewYear = today.year;
    this.viewMonth = today.month;
    this.selectDay(today.day);
  }

  clear(): void {
    this.selectedDate = null;
    this.onChange('');
    this.onTouched();
    this.open = false;
  }

  persianNumber(value: number): string {
    return toPersianDigits(value);
  }

  dayAriaLabel(day: number): string {
    return `${toPersianDigits(day)} ${MONTH_NAMES[this.viewMonth - 1]} ${toPersianDigits(this.viewYear)}`;
  }

  @HostListener('document:click', ['$event'])
  closeOnOutsideClick(event: MouseEvent): void {
    if (this.open && !this.element.nativeElement.contains(event.target as Node)) {
      this.open = false;
      this.onTouched();
    }
  }

  @HostListener('document:keydown.escape')
  closeOnEscape(): void {
    if (this.open) {
      this.open = false;
      this.onTouched();
    }
  }
}
