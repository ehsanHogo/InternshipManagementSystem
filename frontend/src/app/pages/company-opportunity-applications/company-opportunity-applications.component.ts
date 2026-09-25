import { HttpErrorResponse } from '@angular/common/http';
import { Component, inject, signal } from '@angular/core';
import { ActivatedRoute, RouterLink } from '@angular/router';
import { forkJoin } from 'rxjs';
import { MessageService } from 'primeng/api';
import { ButtonModule } from 'primeng/button';
import { TableModule } from 'primeng/table';
import { TagModule } from 'primeng/tag';

import { ApplicationStatus, CompanyOpportunity, OpportunityApplication, applicationStatusLabels } from '../../opportunity/opportunity.models';
import { OpportunityService } from '../../opportunity/opportunity.service';
import { userErrorMessage } from '../../shared/http-error-message';
import { JalaliDatePipe } from '../../shared/jalali-date/jalali-date.pipe';

@Component({
  selector: 'app-company-opportunity-applications',
  imports: [RouterLink, ButtonModule, TableModule, TagModule, JalaliDatePipe],
  templateUrl: './company-opportunity-applications.component.html',
  styleUrl: './company-opportunity-applications.component.scss'
})
export class CompanyOpportunityApplicationsComponent {
  private readonly route = inject(ActivatedRoute);
  private readonly service = inject(OpportunityService);
  private readonly messages = inject(MessageService);

  readonly opportunity = signal<CompanyOpportunity | null>(null);
  readonly applications = signal<OpportunityApplication[]>([]);
  readonly loading = signal(true);
  readonly loadFailed = signal(false);
  private readonly opportunityId = Number(this.route.snapshot.paramMap.get('id'));

  constructor() { this.load(); }

  load(): void {
    if (!Number.isInteger(this.opportunityId) || this.opportunityId <= 0) { this.loading.set(false); this.loadFailed.set(true); return; }
    this.loading.set(true); this.loadFailed.set(false);
    forkJoin({
      opportunity: this.service.getCompany(this.opportunityId),
      applications: this.service.listCompanyApplications(this.opportunityId)
    }).subscribe({
      next: ({ opportunity, applications }) => {
        this.opportunity.set(opportunity); this.applications.set(applications); this.loading.set(false);
      },
      error: (error: HttpErrorResponse) => {
        this.loading.set(false); this.loadFailed.set(true);
        this.messages.add({ severity: 'error', summary: 'خطا', detail: userErrorMessage(error, 'دریافت متقاضیان فرصت ناموفق بود.') });
      }
    });
  }

  statusLabel(status: ApplicationStatus): string { return applicationStatusLabels[status]; }
  statusSeverity(status: ApplicationStatus): 'success' | 'danger' | 'warn' { return status === 'ACCEPTED' ? 'success' : status === 'REJECTED' ? 'danger' : 'warn'; }
}
