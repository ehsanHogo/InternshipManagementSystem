import { Component, input } from '@angular/core';
import { TagModule } from 'primeng/tag';
import {
  WeeklyReport,
  weeklyReportStatusLabels,
  weeklyReportStatusSeverity,
  weeklyReviewStatusLabels,
  weeklyReviewStatusSeverity
} from '../internship/internship.models';

@Component({
  selector: 'app-weekly-report-review',
  imports: [TagModule],
  template: `
    <div class="reviews">
      <div class="status-line">
        <span class="status-label">وضعیت گزارش</span>
        <p-tag
          [value]="statusLabels[report().status]"
          [severity]="weeklyReportStatusSeverity(report().status)"
        />
      </div>
      <div class="status-line">
        <span class="status-label">بررسی سرپرست شرکت</span>
        <p-tag
          [value]="reviewLabels[report().companyReviewStatus]"
          [severity]="weeklyReviewStatusSeverity(report().companyReviewStatus)"
        />
      </div>
      <div class="status-line">
        <span class="status-label">بررسی استاد</span>
        <p-tag
          [value]="reviewLabels[report().professorReviewStatus]"
          [severity]="weeklyReviewStatusSeverity(report().professorReviewStatus)"
        />
      </div>
      @if (report().companyReviewComment) {
        <div class="feedback" [class.revision]="report().companyReviewStatus === 'REVISION_REQUESTED'">
          <strong>آخرین نظر سرپرست شرکت</strong>
          <p>{{ report().companyReviewComment }}</p>
        </div>
      }
      @if (report().professorReviewComment) {
        <div class="feedback" [class.revision]="report().professorReviewStatus === 'REVISION_REQUESTED'">
          <strong>آخرین نظر استاد</strong>
          <p>{{ report().professorReviewComment }}</p>
        </div>
      }
    </div>
  `,
  styles: `
    .reviews {
      display: grid;
      gap: 0.65rem;
      margin-block: 0.8rem;
    }

    .status-line {
      display: flex;
      align-items: center;
      flex-wrap: wrap;
      gap: 0.5rem 0.75rem;
    }

    .status-label {
      color: #64748b;
      font-size: 0.85rem;
      font-weight: 600;
    }

    .feedback {
      padding: 0.8rem;
      border-radius: 0.5rem;
      background: #f4f6f8;
    }

    .feedback p {
      white-space: pre-wrap;
      overflow-wrap: anywhere;
      margin-bottom: 0;
    }

    .revision {
      border-inline-start: 4px solid #dc2626;
      background: #fff1f2;
    }
  `
})
export class WeeklyReportReviewComponent {
  readonly report = input.required<WeeklyReport>();
  readonly statusLabels = weeklyReportStatusLabels;
  readonly reviewLabels = weeklyReviewStatusLabels;
  readonly weeklyReportStatusSeverity = weeklyReportStatusSeverity;
  readonly weeklyReviewStatusSeverity = weeklyReviewStatusSeverity;
}
