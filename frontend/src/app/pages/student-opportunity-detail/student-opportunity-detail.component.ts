import { HttpErrorResponse } from '@angular/common/http';
import { Component, inject, signal } from '@angular/core';
import { ActivatedRoute, RouterLink } from '@angular/router';
import { MessageService } from 'primeng/api';
import { ButtonModule } from 'primeng/button';
import { CardModule } from 'primeng/card';
import { TagModule } from 'primeng/tag';

import { StudentOpportunity } from '../../opportunity/opportunity.models';
import { OpportunityService } from '../../opportunity/opportunity.service';
import { userErrorMessage } from '../../shared/http-error-message';

@Component({
  selector: 'app-student-opportunity-detail',
  imports: [RouterLink, ButtonModule, CardModule, TagModule],
  templateUrl: './student-opportunity-detail.component.html',
  styleUrl: './student-opportunity-detail.component.scss'
})
export class StudentOpportunityDetailComponent {
  private readonly route = inject(ActivatedRoute);
  private readonly opportunitiesService = inject(OpportunityService);
  private readonly messages = inject(MessageService);

  readonly opportunity = signal<StudentOpportunity | null>(null);
  readonly loading = signal(true);
  readonly loadFailed = signal(false);
  private readonly opportunityId = Number(this.route.snapshot.paramMap.get('id'));

  constructor() {
    this.load();
  }

  load(): void {
    if (!Number.isInteger(this.opportunityId) || this.opportunityId <= 0) {
      this.loading.set(false);
      this.loadFailed.set(true);
      return;
    }
    this.loading.set(true);
    this.loadFailed.set(false);
    this.opportunitiesService.getStudent(this.opportunityId).subscribe({
      next: (opportunity) => {
        this.opportunity.set(opportunity);
        this.loading.set(false);
      },
      error: (error: HttpErrorResponse) => {
        this.loading.set(false);
        this.loadFailed.set(true);
        this.messages.add({
          severity: 'error',
          summary: 'خطا',
          detail: userErrorMessage(error, 'دریافت جزئیات فرصت کارآموزی ناموفق بود.')
        });
      }
    });
  }
}
