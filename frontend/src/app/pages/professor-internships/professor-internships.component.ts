import { HttpErrorResponse } from '@angular/common/http';
import { Component, inject, signal } from '@angular/core';
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

@Component({
  selector: 'app-professor-internships',
  imports: [RouterLink, ButtonModule, TableModule, TagModule],
  templateUrl: './professor-internships.component.html',
  styleUrl: '../workflow-page.scss'
})
export class ProfessorInternshipsComponent {
  private readonly internshipService = inject(InternshipService);
  private readonly messages = inject(MessageService);

  readonly cases = signal<ProfessorCaseListItem[]>([]);
  readonly loading = signal(true);
  readonly loadFailed = signal(false);

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
    if (item.status === 'PASSED' || item.status === 'FAILED') return 'ارزیابی شده';
    return item.canProfessorComplete ? 'آماده ارزیابی' : 'مدارک ناقص';
  }

  resultLabel(item: ProfessorCaseListItem): string {
    return item.finalResult ? professorFinalResultLabels[item.finalResult] : '—';
  }
}
