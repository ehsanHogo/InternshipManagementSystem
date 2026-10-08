import { HttpErrorResponse } from '@angular/common/http';
import { Component, inject, signal } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { ActivatedRoute, Router, RouterLink } from '@angular/router';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { distinctUntilChanged, map } from 'rxjs';
import { MessageService } from 'primeng/api';
import { ButtonModule } from 'primeng/button';
import { SelectModule } from 'primeng/select';
import { TableModule } from 'primeng/table';
import { TagModule } from 'primeng/tag';

import {
  InternshipCase,
  InternshipCaseStatus,
  internshipStatusLabels,
  internshipStatusSeverity,
  professorFinalResultLabels
} from '../../internship/internship.models';
import { InternshipService } from '../../internship/internship.service';
import { JalaliDatePipe } from '../../shared/jalali-date/jalali-date.pipe';
import { PersianDigitsPipe } from '../../shared/persian-digits.pipe';

const REVIEW_LIST_STATUSES = [
  'PENDING_UNIVERSITY_REVIEW',
  'PENDING_COMPANY_DETAILS',
  'PENDING_FINAL_APPROVAL',
  'REVISION_REQUESTED',
  'ACTIVE',
  'PASSED',
  'FAILED',
  'CANCELLED'
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
  private readonly route = inject(ActivatedRoute);
  private readonly router = inject(Router);

  readonly cases = signal<InternshipCase[]>([]);
  readonly loading = signal(true);
  readonly loadFailed = signal(false);

  readonly selectedStatus = signal<ReviewListStatus>('PENDING_UNIVERSITY_REVIEW');
  readonly statusOptions = REVIEW_LIST_STATUSES.map((status) => ({
    label: internshipStatusLabels[status],
    value: status
  }));

  constructor() {
    this.route.queryParamMap.pipe(
      map(params => {
        const status = params.get('status');
        return REVIEW_LIST_STATUSES.find(value => value === status) ?? 'PENDING_UNIVERSITY_REVIEW';
      }),
      distinctUntilChanged(),
      takeUntilDestroyed()
    ).subscribe(status => this.loadCases(status));
  }

  filterChanged(status: ReviewListStatus): void {
    if (status === this.selectedStatus()) {
      return;
    }
    void this.router.navigate([], { relativeTo: this.route, queryParams: { status }, queryParamsHandling: 'merge' });
  }

  loadCases(status: ReviewListStatus): void {
    this.selectedStatus.set(status);
    this.loading.set(true);
    this.loadFailed.set(false);
    const request = this.internshipService.listUniversityCases(status);
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

  outcomeLabel(item: InternshipCase): string {
    return item.status === 'CANCELLED'
      ? item.cancellationComment || '—'
      : item.finalResult ? professorFinalResultLabels[item.finalResult] : '—';
  }

  readonly internshipStatusSeverity = internshipStatusSeverity;

  caseStatusLabel(status: InternshipCaseStatus): string {
    return internshipStatusLabels[status];
  }
}

type ReviewListStatus = Exclude<InternshipCaseStatus, 'DRAFT'>;
