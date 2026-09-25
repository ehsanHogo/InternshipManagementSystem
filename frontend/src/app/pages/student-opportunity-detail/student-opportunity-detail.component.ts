import { HttpErrorResponse } from '@angular/common/http';
import { Component, inject, signal } from '@angular/core';
import { ActivatedRoute, RouterLink } from '@angular/router';
import { finalize } from 'rxjs';
import { MessageService } from 'primeng/api';
import { ButtonModule } from 'primeng/button';
import { CardModule } from 'primeng/card';
import { TagModule } from 'primeng/tag';

import {
  ApplicationStatus,
  StudentOpportunity,
  applicationStatusLabels
} from '../../opportunity/opportunity.models';
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
  readonly selectedResume = signal<File | null>(null);
  readonly loading = signal(true);
  readonly loadFailed = signal(false);
  readonly applying = signal(false);
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

  selectResume(event: Event): void {
    const input = event.target as HTMLInputElement;
    const file = input.files?.[0] ?? null;
    if (!file) {
      this.selectedResume.set(null);
      return;
    }
    if (file.type !== 'application/pdf' || !file.name.toLowerCase().endsWith('.pdf')) {
      input.value = '';
      this.selectedResume.set(null);
      this.messages.add({ severity: 'error', summary: 'فایل نامعتبر', detail: 'رزومه باید با قالب پی‌دی‌اف باشد.' });
      return;
    }
    if (file.size <= 0 || file.size > 5 * 1024 * 1024) {
      input.value = '';
      this.selectedResume.set(null);
      this.messages.add({ severity: 'error', summary: 'حجم نامعتبر', detail: 'حجم رزومه نباید بیشتر از ۵ مگابایت باشد.' });
      return;
    }
    this.selectedResume.set(file);
  }

  apply(): void {
    const resume = this.selectedResume();
    const opportunity = this.opportunity();
    if (!resume || !opportunity?.canApply || this.applying()) return;
    this.applying.set(true);
    this.opportunitiesService.apply(opportunity.id, resume).pipe(finalize(() => this.applying.set(false))).subscribe({
      next: (application) => {
        this.opportunity.update((item) => item ? {
          ...item,
          canApply: false,
          applyRestrictionCode: 'APPLICATION_ALREADY_EXISTS',
          existingApplication: { id: application.id, status: application.status }
        } : item);
        this.selectedResume.set(null);
        this.messages.add({ severity: 'success', summary: 'درخواست ثبت شد', detail: 'درخواست شما با موفقیت برای شرکت ارسال شد.' });
      },
      error: (error: HttpErrorResponse) => {
        this.messages.add({ severity: 'error', summary: 'خطا', detail: userErrorMessage(error, 'ارسال درخواست ناموفق بود.') });
        this.load();
      }
    });
  }

  statusLabel(status: ApplicationStatus): string {
    return applicationStatusLabels[status];
  }

  restrictionMessage(code?: string): string {
    if (code === 'INTERNSHIP_ALREADY_COMPLETED') {
      return 'شما قبلاً دوره کارآموزی خود را با موفقیت تکمیل کرده‌اید و امکان ثبت درخواست جدید ندارید.';
    }
    if (code === 'INTERNSHIP_CASE_ALREADY_IN_PROGRESS') {
      return 'پرونده کارآموزی شما در حال بررسی یا اجرا است و در حال حاضر امکان ارسال درخواست جدید وجود ندارد.';
    }
    if (code === 'OPPORTUNITY_NOT_OPEN') {
      return 'این فرصت کارآموزی بسته شده است و درخواست جدید نمی‌پذیرد.';
    }
    return 'در حال حاضر امکان ارسال درخواست برای این فرصت وجود ندارد.';
  }
}
