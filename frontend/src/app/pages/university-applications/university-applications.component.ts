import { HttpErrorResponse } from '@angular/common/http';
import { Component, inject, signal } from '@angular/core';
import { RouterLink } from '@angular/router';
import { MessageService } from 'primeng/api';
import { ButtonModule } from 'primeng/button';
import { TableModule } from 'primeng/table';
import { TagModule } from 'primeng/tag';

import { InternshipCase, internshipStatusLabels } from '../../internship/internship.models';
import { InternshipService } from '../../internship/internship.service';
import { JalaliDatePipe } from '../../shared/jalali-date/jalali-date.pipe';
import { PersianDigitsPipe } from '../../shared/persian-digits.pipe';

@Component({
  selector: 'app-university-applications',
  imports: [JalaliDatePipe, PersianDigitsPipe, RouterLink, ButtonModule, TableModule, TagModule],
  templateUrl: './university-applications.component.html',
  styleUrl: '../workflow-page.scss'
})
export class UniversityApplicationsComponent {
  private readonly internshipService = inject(InternshipService);
  private readonly messages = inject(MessageService);

  readonly cases = signal<InternshipCase[]>([]);
  readonly loading = signal(true);

  constructor() {
    this.internshipService.listPendingUniversityReviewCases().subscribe({
      next: (cases) => {
        this.cases.set(cases);
        this.loading.set(false);
      },
      error: (error: HttpErrorResponse) => {
        this.loading.set(false);
        this.messages.add({
          severity: 'error',
          summary: 'خطا',
          detail: error.status === 0 ? 'ارتباط با سرور برقرار نشد.' : 'دریافت پرونده‌های در انتظار بررسی ناموفق بود.'
        });
      }
    });
  }

  readonly statusLabel = internshipStatusLabels.PENDING_UNIVERSITY_REVIEW;
}
