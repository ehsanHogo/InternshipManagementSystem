import { HttpErrorResponse } from '@angular/common/http';
import { Component, inject, input, signal } from '@angular/core';
import { MessageService } from 'primeng/api';
import { ButtonModule } from 'primeng/button';
import { CardModule } from 'primeng/card';
import { TagModule } from 'primeng/tag';

import {
  CompanyEvaluation,
  InternshipCase,
  evaluationRatingLabels,
  finalReportStatusLabels,
  finalReportStatusSeverity
} from '../internship/internship.models';
import { InternshipService } from '../internship/internship.service';
import { userErrorMessage } from './http-error-message';
import { JalaliDatePipe } from './jalali-date/jalali-date.pipe';
import { PersianDigitsPipe } from './persian-digits.pipe';
import { WeeklyReportReviewComponent } from './weekly-report-review.component';

type RatingField = keyof Pick<CompanyEvaluation,
  'attendanceRating' | 'participationRating' | 'learningRating' | 'interestRating' |
  'persistenceRating' | 'suggestionRating' | 'resourceUsageRating' |
  'reportQualityRating' | 'projectPerformanceRating'>;

/** Read-only business data for Student history and University case inspection. */
@Component({
  selector: 'app-case-reports',
  imports: [ButtonModule, CardModule, TagModule, JalaliDatePipe, PersianDigitsPipe, WeeklyReportReviewComponent],
  template: `
    @if (item().weeklyReports.length) {
      <p-card header="گزارش‌های هفتگی (فقط خواندنی)">
        @for (report of item().weeklyReports; track report.id) {
          <details>
            <summary>گزارش هفته {{ report.weekNumber | persianDigits }}</summary>
            <p>{{ report.startDate | jalaliDate }} تا {{ report.endDate | jalaliDate }}</p>
            <p class="feedback-text">{{ report.activityDescription }}</p>
            @if (report.submittedAt) { <p>تاریخ ارسال: {{ report.submittedAt | jalaliDate }}</p> }
            <app-weekly-report-review [report]="report" />
          </details>
        }
      </p-card>
    }
    @if (item().companyEvaluation; as evaluation) {
      <p-card header="ارزیابی سرپرست شرکت (فقط خواندنی)">
        <div class="details-grid">
          @for (criterion of criteria; track criterion.key) {
            <div><span>{{ criterion.label }}</span><strong>{{ ratingLabels[evaluation[criterion.key]] }}</strong></div>
          }
          <div><span>روزهای مرخصی</span><strong>{{ evaluation.leaveDays | persianDigits }}</strong></div>
          <div><span>روزهای غیبت</span><strong>{{ evaluation.absenceDays | persianDigits }}</strong></div>
          <div><span>تاریخ ثبت</span><strong>{{ evaluation.submittedAt | jalaliDate }}</strong></div>
        </div>
        @if (evaluation.suggestions) { <p class="feedback-text">{{ evaluation.suggestions }}</p> }
      </p-card>
    }
    @if (item().finalReport; as report) {
      <p-card header="گزارش نهایی (فقط خواندنی)">
        <p-tag [value]="finalLabels[report.status]" [severity]="finalSeverity(report.status)" />
        <div class="details-grid">
          <div><span>نام فایل</span><strong>{{ report.currentFile.originalName }}</strong></div>
          <div><span>تاریخ ارسال</span><strong>{{ report.submittedAt | jalaliDate }}</strong></div>
          @if (report.reviewedAt) { <div><span>تاریخ بررسی</span><strong>{{ report.reviewedAt | jalaliDate }}</strong></div> }
        </div>
        @if (report.reviewComment) {
          <strong>آخرین نظر استاد درباره گزارش نهایی</strong>
          <p class="feedback-text">{{ report.reviewComment }}</p>
        }
        <p-button label="دریافت گزارش نهایی" icon="pi pi-download" severity="secondary"
          [loading]="downloading()" [disabled]="downloading()" (onClick)="downloadFinalReport()" />
      </p-card>
    }
  `,
  styles: `
    :host { display: grid; gap: 1rem; }
    details { padding-block: .8rem; border-bottom: 1px solid #e2e8f0; }
    summary { cursor: pointer; font-weight: 600; }
    .feedback-text { white-space: pre-wrap; overflow-wrap: anywhere; }
    .details-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(12rem, 1fr)); gap: 1rem; margin-block: 1rem; }
    .details-grid div { display: grid; gap: .4rem; }
    .details-grid span { color: #64748b; }
  `
})
export class CaseReportsComponent {
  private readonly internships = inject(InternshipService);
  private readonly messages = inject(MessageService);
  readonly item = input.required<InternshipCase>();
  readonly downloading = signal(false);
  readonly ratingLabels = evaluationRatingLabels;
  readonly finalLabels = finalReportStatusLabels;
  readonly finalSeverity = finalReportStatusSeverity;
  readonly criteria: { key: RatingField; label: string }[] = [
    { key: 'attendanceRating', label: 'حضور و وقت‌شناسی' },
    { key: 'participationRating', label: 'مشارکت و همکاری' },
    { key: 'learningRating', label: 'یادگیری و پیشرفت' },
    { key: 'interestRating', label: 'علاقه‌مندی و انگیزه' },
    { key: 'persistenceRating', label: 'پشتکار و مسئولیت‌پذیری' },
    { key: 'suggestionRating', label: 'ارائه پیشنهادهای سازنده' },
    { key: 'resourceUsageRating', label: 'استفاده از منابع' },
    { key: 'reportQualityRating', label: 'کیفیت گزارش‌دهی' },
    { key: 'projectPerformanceRating', label: 'عملکرد در پروژه' }
  ];

  downloadFinalReport(): void {
    const file = this.item().finalReport?.currentFile;
    if (!file || this.downloading()) return;
    this.downloading.set(true);
    this.internships.downloadFile(file.id).subscribe({
      next: blob => {
        const url = URL.createObjectURL(blob);
        const anchor = document.createElement('a');
        anchor.href = url;
        anchor.download = file.originalName;
        anchor.click();
        URL.revokeObjectURL(url);
        this.downloading.set(false);
      },
      error: (error: HttpErrorResponse) => {
        this.downloading.set(false);
        this.messages.add({ severity: 'error', summary: 'خطا', detail: userErrorMessage(error, 'دریافت گزارش نهایی ناموفق بود.') });
      }
    });
  }
}
