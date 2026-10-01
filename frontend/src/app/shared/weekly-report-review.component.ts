import { Component, input } from '@angular/core';
import { TagModule } from 'primeng/tag';
import { WeeklyReport, weeklyReportStatusLabels, weeklyReviewStatusLabels } from '../internship/internship.models';

@Component({
  selector: 'app-weekly-report-review',
  imports: [TagModule],
  template: `
    <div class="reviews">
      <p-tag [value]="statusLabels[report().status]" [severity]="report().status === 'APPROVED' ? 'success' : report().status === 'REVISION_REQUESTED' ? 'danger' : 'info'" />
      <div>بررسی سرپرست شرکت: <p-tag [value]="reviewLabels[report().companyReviewStatus]" [severity]="report().companyReviewStatus === 'APPROVED' ? 'success' : report().companyReviewStatus === 'REVISION_REQUESTED' ? 'danger' : 'warn'" /></div>
      <div>بررسی استاد: <p-tag [value]="reviewLabels[report().professorReviewStatus]" [severity]="report().professorReviewStatus === 'APPROVED' ? 'success' : report().professorReviewStatus === 'REVISION_REQUESTED' ? 'danger' : 'warn'" /></div>
      @if (report().companyReviewComment) {
        <div class="feedback" [class.revision]="report().companyReviewStatus === 'REVISION_REQUESTED'"><strong>آخرین نظر سرپرست شرکت</strong><p>{{ report().companyReviewComment }}</p></div>
      }
      @if (report().professorReviewComment) {
        <div class="feedback" [class.revision]="report().professorReviewStatus === 'REVISION_REQUESTED'"><strong>آخرین نظر استاد</strong><p>{{ report().professorReviewComment }}</p></div>
      }
    </div>
  `,
  styles: `
    .reviews { display: grid; gap: .7rem; margin-block: .8rem; }
    .feedback { padding: .8rem; border-radius: .5rem; background: #f4f6f8; }
    .feedback p { white-space: pre-wrap; overflow-wrap: anywhere; margin-bottom: 0; }
    .revision { border-inline-start: 4px solid #dc2626; background: #fff1f2; }
  `
})
export class WeeklyReportReviewComponent {
  readonly report = input.required<WeeklyReport>();
  readonly statusLabels = weeklyReportStatusLabels;
  readonly reviewLabels = weeklyReviewStatusLabels;
}
