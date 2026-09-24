import { HttpErrorResponse } from '@angular/common/http';
import { Component, inject, signal } from '@angular/core';
import { RouterLink } from '@angular/router';
import { MessageService } from 'primeng/api';
import { ButtonModule } from 'primeng/button';
import { CardModule } from 'primeng/card';
import { TagModule } from 'primeng/tag';

import { StudentOpportunity } from '../../opportunity/opportunity.models';
import { OpportunityService } from '../../opportunity/opportunity.service';
import { userErrorMessage } from '../../shared/http-error-message';

@Component({
  selector: 'app-student-opportunities',
  imports: [RouterLink, ButtonModule, CardModule, TagModule],
  templateUrl: './student-opportunities.component.html',
  styleUrl: './student-opportunities.component.scss'
})
export class StudentOpportunitiesComponent {
  private readonly opportunitiesService = inject(OpportunityService);
  private readonly messages = inject(MessageService);

  readonly opportunities = signal<StudentOpportunity[]>([]);
  readonly loading = signal(true);
  readonly loadFailed = signal(false);

  constructor() {
    this.load();
  }

  load(): void {
    this.loading.set(true);
    this.loadFailed.set(false);
    this.opportunitiesService.listStudent().subscribe({
      next: (opportunities) => {
        this.opportunities.set(opportunities);
        this.loading.set(false);
      },
      error: (error: HttpErrorResponse) => {
        this.loading.set(false);
        this.loadFailed.set(true);
        this.messages.add({
          severity: 'error',
          summary: 'خطا',
          detail: userErrorMessage(error, 'دریافت فرصت‌های کارآموزی ناموفق بود.')
        });
      }
    });
  }
}
