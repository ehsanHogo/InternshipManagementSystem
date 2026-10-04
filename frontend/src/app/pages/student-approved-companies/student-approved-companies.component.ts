import { HttpErrorResponse } from '@angular/common/http';
import { Component, inject, signal } from '@angular/core';
import { RouterLink } from '@angular/router';
import { MessageService } from 'primeng/api';
import { ButtonModule } from 'primeng/button';
import { TableModule } from 'primeng/table';
import { finalize } from 'rxjs';

import { CompanyApprovalService } from '../../company/company-approval.service';
import { PublicApprovedCompany } from '../../company/company-approval.models';
import { externalHref } from '../../shared/external-url';
import { userErrorMessage } from '../../shared/http-error-message';

@Component({
  selector: 'app-student-approved-companies',
  imports: [ButtonModule, TableModule, RouterLink],
  templateUrl: './student-approved-companies.component.html',
  styleUrl: '../workflow-page.scss'
})
export class StudentApprovedCompaniesComponent {
  private readonly approval = inject(CompanyApprovalService);
  private readonly messages = inject(MessageService);
  readonly companies = signal<PublicApprovedCompany[]>([]);
  readonly loading = signal(true);
  readonly loadFailed = signal(false);
  readonly externalHref = externalHref;

  constructor() { this.load(); }

  load(): void {
    this.loading.set(true);
    this.loadFailed.set(false);
    this.approval.listStudentApproved().pipe(finalize(() => this.loading.set(false))).subscribe({
      next: (companies) => this.companies.set(companies),
      error: (error: HttpErrorResponse) => {
        this.loadFailed.set(true);
        this.messages.add({ severity: 'error', summary: 'خطا', detail: userErrorMessage(error, 'دریافت شرکت‌های مورد تأیید ناموفق بود.') });
      }
    });
  }
}
