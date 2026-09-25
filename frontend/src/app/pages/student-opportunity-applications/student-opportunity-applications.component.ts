import { HttpErrorResponse } from '@angular/common/http';
import { Component, inject, signal } from '@angular/core';
import { RouterLink } from '@angular/router';
import { MessageService } from 'primeng/api';
import { ButtonModule } from 'primeng/button';
import { TableModule } from 'primeng/table';
import { TagModule } from 'primeng/tag';

import { ApplicationStatus, OpportunityApplication, applicationStatusLabels } from '../../opportunity/opportunity.models';
import { OpportunityService } from '../../opportunity/opportunity.service';
import { userErrorMessage } from '../../shared/http-error-message';
import { JalaliDatePipe } from '../../shared/jalali-date/jalali-date.pipe';

@Component({
  selector: 'app-student-opportunity-applications',
  imports: [RouterLink, ButtonModule, TableModule, TagModule, JalaliDatePipe],
  templateUrl: './student-opportunity-applications.component.html',
  styleUrl: './student-opportunity-applications.component.scss'
})
export class StudentOpportunityApplicationsComponent {
  private readonly service = inject(OpportunityService);
  private readonly messages = inject(MessageService);

  readonly applications = signal<OpportunityApplication[]>([]);
  readonly loading = signal(true);
  readonly loadFailed = signal(false);

  constructor() { this.load(); }

  load(): void {
    this.loading.set(true);
    this.loadFailed.set(false);
    this.service.listStudentApplications().subscribe({
      next: (applications) => { this.applications.set(applications); this.loading.set(false); },
      error: (error: HttpErrorResponse) => {
        this.loading.set(false);
        this.loadFailed.set(true);
        this.messages.add({ severity: 'error', summary: 'خطا', detail: userErrorMessage(error, 'دریافت درخواست‌های شما ناموفق بود.') });
      }
    });
  }

  statusLabel(status: ApplicationStatus): string { return applicationStatusLabels[status]; }
  statusSeverity(status: ApplicationStatus): 'success' | 'danger' | 'warn' {
    return status === 'ACCEPTED' ? 'success' : status === 'REJECTED' ? 'danger' : 'warn';
  }
}
