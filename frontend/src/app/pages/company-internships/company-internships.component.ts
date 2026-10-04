import { HttpErrorResponse } from '@angular/common/http';
import { Component, computed, inject, signal } from '@angular/core';
import { RouterLink } from '@angular/router';
import { MessageService } from 'primeng/api';
import { ButtonModule } from 'primeng/button';
import { TableModule } from 'primeng/table';
import { TagModule } from 'primeng/tag';

import { CompanyInternshipCase, InternshipCaseStatus, internshipStatusLabels } from '../../internship/internship.models';
import { InternshipService } from '../../internship/internship.service';
import { JalaliDatePipe } from '../../shared/jalali-date/jalali-date.pipe';

@Component({
  selector: 'app-company-internships',
  imports: [JalaliDatePipe, RouterLink, ButtonModule, TableModule, TagModule],
  templateUrl: './company-internships.component.html',
  styleUrl: '../workflow-page.scss'
})
export class CompanyInternshipsComponent {
  private readonly internshipService = inject(InternshipService);
  private readonly messages = inject(MessageService);

  readonly cases = signal<CompanyInternshipCase[]>([]);
  readonly loading = signal(true);
  readonly loadFailed = signal(false);
  readonly pendingOnly = signal(true);
  readonly visibleCases = computed(() => this.pendingOnly()
    ? this.cases().filter((item) => item.status === 'PENDING_COMPANY_DETAILS') : this.cases());

  constructor() {
    this.load();
  }

  load(): void {
    this.loading.set(true);
    this.loadFailed.set(false);
    this.internshipService.listCompanyCases().subscribe({
      next: (cases) => {
        this.cases.set(cases);
        this.loading.set(false);
      },
      error: (_error: HttpErrorResponse) => {
        this.loading.set(false);
        this.loadFailed.set(true);
        this.messages.add({ severity: 'error', summary: 'خطا', detail: 'دریافت پرونده‌های اختصاص‌یافته ناموفق بود.' });
      }
    });
  }

  statusLabel(status: InternshipCaseStatus): string {
    return internshipStatusLabels[status];
  }

  placementName(item: CompanyInternshipCase): string {
    return item.selectedPreference?.application.opportunity.company.name ?? '—';
  }
}
