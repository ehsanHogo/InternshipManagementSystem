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

  constructor() {
    this.internshipService.listProfessorCases().subscribe({
      next: (cases) => {
        this.cases.set(cases);
        this.loading.set(false);
      },
      error: () => {
        this.loading.set(false);
        this.messages.add({ severity: 'error', summary: 'خطا', detail: 'دریافت پرونده‌های دانشجویان ناموفق بود.' });
      }
    });
  }

  statusLabel(status: InternshipCaseStatus): string {
    return internshipStatusLabels[status];
  }

  readinessLabel(item: ProfessorCaseListItem): string {
    if (item.status === 'COMPLETED') return 'ارزیابی شده';
    return item.canProfessorComplete ? 'آماده ارزیابی' : 'مدارک ناقص';
  }

  resultLabel(item: ProfessorCaseListItem): string {
    return item.finalResult ? professorFinalResultLabels[item.finalResult] : '—';
  }
}
