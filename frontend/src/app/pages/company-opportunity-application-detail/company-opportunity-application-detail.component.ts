import { HttpErrorResponse } from '@angular/common/http';
import { Component, inject, signal } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { ActivatedRoute, RouterLink } from '@angular/router';
import { finalize } from 'rxjs';
import { ConfirmationService, MessageService } from 'primeng/api';
import { ButtonModule } from 'primeng/button';
import { CardModule } from 'primeng/card';
import { ConfirmDialogModule } from 'primeng/confirmdialog';
import { TagModule } from 'primeng/tag';
import { TextareaModule } from 'primeng/textarea';

import { ApplicationStatus, OpportunityApplication, applicationStatusLabels } from '../../opportunity/opportunity.models';
import { OpportunityService } from '../../opportunity/opportunity.service';
import { userErrorMessage } from '../../shared/http-error-message';
import { JalaliDatePipe } from '../../shared/jalali-date/jalali-date.pipe';

@Component({
  selector: 'app-company-opportunity-application-detail',
  imports: [FormsModule, RouterLink, ButtonModule, CardModule, ConfirmDialogModule, TagModule, TextareaModule, JalaliDatePipe],
  providers: [ConfirmationService],
  templateUrl: './company-opportunity-application-detail.component.html',
  styleUrl: './company-opportunity-application-detail.component.scss'
})
export class CompanyOpportunityApplicationDetailComponent {
  private readonly route = inject(ActivatedRoute);
  private readonly service = inject(OpportunityService);
  private readonly messages = inject(MessageService);
  private readonly confirmation = inject(ConfirmationService);

  readonly application = signal<OpportunityApplication | null>(null);
  readonly loading = signal(true);
  readonly loadFailed = signal(false);
  readonly reviewing = signal(false);
  readonly downloading = signal(false);
  companyComment = '';
  private readonly applicationId = Number(this.route.snapshot.paramMap.get('id'));

  constructor() { this.load(); }

  load(): void {
    if (!Number.isInteger(this.applicationId) || this.applicationId <= 0) { this.loading.set(false); this.loadFailed.set(true); return; }
    this.service.getCompanyApplication(this.applicationId).subscribe({
      next: (application) => {
        this.application.set(application); this.companyComment = application.companyComment ?? ''; this.loading.set(false);
      },
      error: (error: HttpErrorResponse) => {
        this.loading.set(false); this.loadFailed.set(true);
        this.messages.add({ severity: 'error', summary: 'خطا', detail: userErrorMessage(error, 'دریافت جزئیات درخواست ناموفق بود.') });
      }
    });
  }

  confirmReview(decision: 'accept' | 'reject'): void {
    if (this.application()?.status !== 'PENDING' || this.reviewing()) return;
    const accepting = decision === 'accept';
    this.confirmation.confirm({
      header: accepting ? 'پذیرش درخواست' : 'رد درخواست',
      message: `${accepting ? 'آیا از پذیرش این درخواست مطمئن هستید؟' : 'آیا از رد این درخواست مطمئن هستید؟'} این تصمیم در حال حاضر قابل تغییر نیست.`,
      icon: accepting ? 'pi pi-check-circle' : 'pi pi-times-circle',
      acceptLabel: accepting ? 'پذیرش نهایی' : 'رد نهایی',
      rejectLabel: 'انصراف',
      acceptButtonStyleClass: accepting ? '' : 'p-button-danger',
      accept: () => this.review(decision)
    });
  }

  download(): void {
    const resume = this.application()?.resume;
    if (!resume || this.downloading()) return;
    this.downloading.set(true);
    this.service.downloadResume(resume.id).pipe(finalize(() => this.downloading.set(false))).subscribe({
      next: (blob) => this.saveBlob(blob, resume.originalName),
      error: (error: HttpErrorResponse) => this.messages.add({ severity: 'error', summary: 'خطا', detail: userErrorMessage(error, 'دریافت رزومه ناموفق بود.') })
    });
  }

  statusLabel(status: ApplicationStatus): string { return applicationStatusLabels[status]; }
  statusSeverity(status: ApplicationStatus): 'success' | 'danger' | 'warn' { return status === 'ACCEPTED' ? 'success' : status === 'REJECTED' ? 'danger' : 'warn'; }

  private review(decision: 'accept' | 'reject'): void {
    this.reviewing.set(true);
    this.service.reviewApplication(this.applicationId, decision, this.companyComment).pipe(finalize(() => this.reviewing.set(false))).subscribe({
      next: (application) => {
        this.application.set(application); this.companyComment = application.companyComment ?? '';
        this.messages.add({ severity: 'success', summary: 'ثبت شد', detail: decision === 'accept' ? 'درخواست دانشجو پذیرفته شد.' : 'درخواست دانشجو رد شد.' });
      },
      error: (error: HttpErrorResponse) => this.messages.add({ severity: 'error', summary: 'خطا', detail: userErrorMessage(error, 'ثبت تصمیم شرکت ناموفق بود.') })
    });
  }

  private saveBlob(blob: Blob, filename: string): void {
    const url = URL.createObjectURL(blob);
    const link = document.createElement('a');
    link.href = url; link.download = filename; link.click();
    URL.revokeObjectURL(url);
  }
}
