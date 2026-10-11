import { HttpErrorResponse } from '@angular/common/http';
import { Component, computed, inject, signal } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { SelectModule } from 'primeng/select';
import { RouterLink } from '@angular/router';
import { MessageService } from 'primeng/api';
import { ButtonModule } from 'primeng/button';
import { TableModule } from 'primeng/table';
import { TagModule } from 'primeng/tag';

import {
  InternshipCaseStatus,
  ProfessorCaseListItem,
  internshipStatusLabels,
  internshipStatusSeverity,
  professorFinalResultLabels
} from '../../internship/internship.models';
import { InternshipService } from '../../internship/internship.service';
import { statusFilterOptions } from '../../shared/status-filter-options';

@Component({
  selector: 'app-professor-internships',
  imports: [FormsModule, SelectModule, RouterLink, ButtonModule, TableModule, TagModule],
  templateUrl: './professor-internships.component.html',
  styleUrl: '../workflow-page.scss'
})
export class ProfessorInternshipsComponent {
  private readonly internshipService = inject(InternshipService);
  private readonly messages = inject(MessageService);

  readonly cases = signal<ProfessorCaseListItem[]>([]);
  readonly loading = signal(true);
  readonly loadFailed = signal(false);

  readonly listFilter = signal<'ALL' | 'ACTIVE' | 'PASSED' | 'FAILED' | 'CANCELLED'>('ALL');
  readonly filterOptions = statusFilterOptions<'ACTIVE' | 'PASSED' | 'FAILED' | 'CANCELLED'>(internshipStatusLabels,
    ['ACTIVE', 'PASSED', 'FAILED', 'CANCELLED'] as const);
  readonly visibleCases = computed(() => this.listFilter() === 'ALL'
    ? this.cases() : this.cases().filter(item => item.status === this.listFilter()));

  constructor() {
    this.load();
  }

  load(): void {
    this.loading.set(true);
    this.loadFailed.set(false);
    this.internshipService.listProfessorCases().subscribe({
      next: (cases) => {
        this.cases.set(cases);
        this.loading.set(false);
      },
      error: (_error: HttpErrorResponse) => {
        this.loading.set(false);
        this.loadFailed.set(true);
        this.messages.add({ severity: 'error', summary: 'خطا', detail: 'دریافت پرونده‌های دانشجویان ناموفق بود.' });
      }
    });
  }

  readonly internshipStatusSeverity = internshipStatusSeverity;

  statusLabel(status: InternshipCaseStatus): string {
    return internshipStatusLabels[status];
  }

  readinessLabel(item: ProfessorCaseListItem): string {
    if (item.status === 'CANCELLED') return 'لغو شده';
    if (item.status === 'PASSED' || item.status === 'FAILED') return 'ارزیابی شده';
    return item.canProfessorComplete ? 'آماده ارزیابی' : 'مدارک ناقص';
  }

  resultLabel(item: ProfessorCaseListItem): string {
    return item.finalResult ? professorFinalResultLabels[item.finalResult] : '—';
  }
}
