import { HttpErrorResponse } from '@angular/common/http';
import { Component, inject, signal } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { RouterLink } from '@angular/router';
import { MessageService } from 'primeng/api';
import { ButtonModule } from 'primeng/button';
import { SelectModule } from 'primeng/select';
import { TableModule } from 'primeng/table';
import { TagModule } from 'primeng/tag';

import {
  InternshipCase,
  InternshipCaseStatus,
  internshipStatusLabels,
  internshipStatusSeverity
} from '../../internship/internship.models';
import { InternshipService } from '../../internship/internship.service';
import { JalaliDatePipe } from '../../shared/jalali-date/jalali-date.pipe';
import { PersianDigitsPipe } from '../../shared/persian-digits.pipe';

const REVIEW_LIST_STATUSES = [
  'PENDING_UNIVERSITY_REVIEW',
  'PENDING_FINAL_APPROVAL',
  'READY_TO_START',
  'ACTIVE',
  'PASSED',
  'FAILED'
] as const satisfies readonly ReviewListStatus[];

@Component({
  selector: 'app-university-applications',
  imports: [FormsModule, JalaliDatePipe, PersianDigitsPipe, RouterLink, ButtonModule, SelectModule, TableModule, TagModule],
  templateUrl: './university-applications.component.html',
  styleUrl: '../workflow-page.scss'
})
export class UniversityApplicationsComponent {
  private readonly internshipService = inject(InternshipService);
  private readonly messages = inject(MessageService);

  readonly cases = signal<InternshipCase[]>([]);
  readonly loading = signal(true);
  readonly loadFailed = signal(false);

  readonly selectedStatus = signal<ReviewListStatus>('PENDING_UNIVERSITY_REVIEW');
  readonly statusOptions = REVIEW_LIST_STATUSES.map((status) => ({
    label: internshipStatusLabels[status],
    value: status
  }));

  constructor() {
    this.loadCases('PENDING_UNIVERSITY_REVIEW');
  }

  filterChanged(status: ReviewListStatus): void {
    if (status === this.selectedStatus()) {
      return;
    }
    this.loadCases(status);
  }

  loadCases(status: ReviewListStatus): void {
    this.selectedStatus.set(status);
    this.loading.set(true);
    this.loadFailed.set(false);
    const request = status === 'PENDING_FINAL_APPROVAL'
      ? this.internshipService.listPendingFinalApprovalCases()
      : this.internshipService.listUniversityCases(status);
    request.subscribe({
      next: (cases) => {
        this.cases.set(cases);
        this.loading.set(false);
      },
      error: (error: HttpErrorResponse) => {
        this.loading.set(false);
        this.loadFailed.set(true);
        this.messages.add({
          severity: 'error',
          summary: 'خطا',
          detail: error.status === 0 ? 'ارتباط با سرور برقرار نشد.' : 'دریافت پرونده‌های کارآموزی ناموفق بود.'
        });
      }
    });
  }

  readonly internshipStatusSeverity = internshipStatusSeverity;

  caseStatusLabel(status: InternshipCaseStatus): string {
    return internshipStatusLabels[status];
  }
}

type ReviewListStatus = 'PENDING_UNIVERSITY_REVIEW' | 'PENDING_FINAL_APPROVAL' | 'READY_TO_START' | 'ACTIVE' | 'PASSED' | 'FAILED';
