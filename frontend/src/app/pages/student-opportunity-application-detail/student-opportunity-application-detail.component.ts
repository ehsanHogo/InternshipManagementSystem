import { HttpErrorResponse } from '@angular/common/http';
import { Component, inject, signal } from '@angular/core';
import { ActivatedRoute, RouterLink } from '@angular/router';
import { finalize } from 'rxjs';
import { MessageService } from 'primeng/api';
import { ButtonModule } from 'primeng/button';
import { CardModule } from 'primeng/card';
import { TagModule } from 'primeng/tag';

import { ApplicationStatus, OpportunityApplication, applicationStatusLabels } from '../../opportunity/opportunity.models';
import { OpportunityService } from '../../opportunity/opportunity.service';
import { userErrorMessage } from '../../shared/http-error-message';
import { JalaliDatePipe } from '../../shared/jalali-date/jalali-date.pipe';

@Component({
  selector: 'app-student-opportunity-application-detail',
  imports: [RouterLink, ButtonModule, CardModule, TagModule, JalaliDatePipe],
  templateUrl: './student-opportunity-application-detail.component.html',
  styleUrl: './student-opportunity-application-detail.component.scss'
})
export class StudentOpportunityApplicationDetailComponent {
  private readonly route = inject(ActivatedRoute);
  private readonly service = inject(OpportunityService);
  private readonly messages = inject(MessageService);

  readonly application = signal<OpportunityApplication | null>(null);
  readonly loading = signal(true);
  readonly loadFailed = signal(false);
  readonly downloading = signal(false);
  private readonly applicationId = Number(this.route.snapshot.paramMap.get('id'));

  constructor() { this.load(); }

  load(): void {
    if (!Number.isInteger(this.applicationId) || this.applicationId <= 0) { this.loading.set(false); this.loadFailed.set(true); return; }
    this.service.getStudentApplication(this.applicationId).subscribe({
      next: (application) => { this.application.set(application); this.loading.set(false); },
      error: (error: HttpErrorResponse) => {
        this.loading.set(false); this.loadFailed.set(true);
        this.messages.add({ severity: 'error', summary: 'خطا', detail: userErrorMessage(error, 'دریافت جزئیات درخواست ناموفق بود.') });
      }
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

  private saveBlob(blob: Blob, filename: string): void {
    const url = URL.createObjectURL(blob);
    const link = document.createElement('a');
    link.href = url; link.download = filename; link.click();
    URL.revokeObjectURL(url);
  }
}
