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
  internshipStatusLabels
} from '../../internship/internship.models';
import { InternshipService } from '../../internship/internship.service';
import { JalaliDatePipe } from '../../shared/jalali-date/jalali-date.pipe';

@Component({
  selector: 'app-university-applications',
  imports: [JalaliDatePipe, FormsModule, RouterLink, ButtonModule, SelectModule, TableModule, TagModule],
  templateUrl: './university-applications.component.html',
  styleUrl: '../workflow-page.scss'
})
export class UniversityApplicationsComponent {
  private readonly internshipService = inject(InternshipService);
  private readonly messages = inject(MessageService);

  readonly cases = signal<InternshipCase[]>([]);
  readonly loading = signal(true);
  readonly selectedStatus = signal<InternshipCaseStatus | null>(null);
  readonly statusOptions: { label: string; value: InternshipCaseStatus | null }[] = [
    { label: 'همه وضعیت‌ها', value: null },
    { label: internshipStatusLabels.PENDING_UNIVERSITY_REVIEW, value: 'PENDING_UNIVERSITY_REVIEW' },
    { label: internshipStatusLabels.PENDING_COMPANY_DETAILS, value: 'PENDING_COMPANY_DETAILS' },
    { label: internshipStatusLabels.PENDING_FINAL_APPROVAL, value: 'PENDING_FINAL_APPROVAL' },
    { label: internshipStatusLabels.READY_TO_START, value: 'READY_TO_START' },
    { label: internshipStatusLabels.ACTIVE, value: 'ACTIVE' },
    { label: internshipStatusLabels.COMPLETED, value: 'COMPLETED' }
  ];

  constructor() {
    this.loadCases();
  }

  filterChanged(status: InternshipCaseStatus | null): void {
    this.selectedStatus.set(status);
    this.loadCases();
  }

  statusLabel(status: InternshipCaseStatus): string {
    return internshipStatusLabels[status];
  }

  placementName(item: InternshipCase): string {
    const preference = item.selectedPreference;
    return preference?.application.opportunity.company.name ?? 'هنوز انتخاب نشده';
  }

  private loadCases(): void {
    this.loading.set(true);
    this.internshipService.listUniversityCases(this.selectedStatus() ?? undefined).subscribe({
      next: (cases) => {
        this.cases.set(cases);
        this.loading.set(false);
      },
      error: (error: HttpErrorResponse) => {
        this.loading.set(false);
        this.messages.add({ severity: 'error', summary: 'خطا', detail: error.status === 0 ? 'ارتباط با سرور برقرار نشد.' : 'دریافت پرونده‌ها ناموفق بود.' });
      }
    });
  }
}
