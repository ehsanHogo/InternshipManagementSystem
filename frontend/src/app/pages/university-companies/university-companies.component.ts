import { HttpErrorResponse } from '@angular/common/http';
import { Component, computed, inject, signal } from '@angular/core';
import { finalize, forkJoin } from 'rxjs';
import { ConfirmationService, MessageService } from 'primeng/api';
import { ConfirmDialogModule } from 'primeng/confirmdialog';
import { ButtonModule } from 'primeng/button';
import { DialogModule } from 'primeng/dialog';
import { TableModule } from 'primeng/table';
import { TagModule } from 'primeng/tag';

import { Company } from '../../internship/internship.models';
import { UniversityManagementService } from '../../university-management/university-management.service';
import { userErrorMessage } from '../../shared/http-error-message';
import { CompanyApprovalService } from '../../company/company-approval.service';
import { ApprovalCompanyDetail, EligibleCompany } from '../../company/company-approval.models';
import { JalaliDatePipe } from '../../shared/jalali-date/jalali-date.pipe';
import { PersianDigitsPipe } from '../../shared/persian-digits.pipe';

@Component({
  selector: 'app-university-companies',
  imports: [ButtonModule, DialogModule, TableModule, TagModule, ConfirmDialogModule, JalaliDatePipe, PersianDigitsPipe],
  providers: [ConfirmationService],
  templateUrl: './university-companies.component.html',
  styleUrl: '../workflow-page.scss'
})
export class UniversityCompaniesComponent {
  private readonly management = inject(UniversityManagementService);
  private readonly messages = inject(MessageService);
  private readonly approval = inject(CompanyApprovalService);
  private readonly confirmation = inject(ConfirmationService);

  readonly companies = signal<Company[]>([]);
  readonly eligible = signal<EligibleCompany[]>([]);
  readonly allCompanies = signal<Company[]>([]);
  readonly tab = signal<'eligible' | 'approved' | 'all'>('eligible');
  readonly visibleCompanies = computed(() => this.tab() === 'eligible' ? this.eligible() : this.tab() === 'all' ? this.allCompanies() : this.companies());
  readonly detail = signal<ApprovalCompanyDetail | null>(null);
  readonly detailVisible = signal(false);
  readonly detailLoading = signal(false);
  readonly detailFailed = signal(false);
  readonly detailId = signal<number | null>(null);
  readonly approvingId = signal<number | null>(null);
  readonly loading = signal(true);
  readonly loadFailed = signal(false);
  constructor() { this.load(); }

  load(): void {
    this.loading.set(true);
    this.loadFailed.set(false);
    forkJoin({ eligible: this.approval.listEligible(), approved: this.approval.listApproved(), all: this.management.listCompanies() }).subscribe({
      next: ({ eligible, approved, all }) => {
        this.eligible.set(eligible);
        this.companies.set(approved);
        this.allCompanies.set(all);
        this.loading.set(false);
      },
      error: (error: HttpErrorResponse) => {
        this.loading.set(false);
        this.loadFailed.set(true);
        this.showError(error, 'دریافت شرکت‌ها ناموفق بود.');
      }
    });
  }

  review(id: number): void {
    this.detailId.set(id);
    this.detail.set(null);
    this.detailFailed.set(false);
    this.detailLoading.set(true);
    this.detailVisible.set(true);
    this.approval.getCompany(id).pipe(finalize(() => this.detailLoading.set(false))).subscribe({
      next: (detail) => this.detail.set(detail),
      error: (error: HttpErrorResponse) => {
        this.detailFailed.set(true);
        this.showError(error, 'دریافت اطلاعات شرکت ناموفق بود.');
      }
    });
  }

  confirmApproval(company: Company): void {
    if (this.approvingId() !== null) return;
    this.confirmation.confirm({
      header: 'تأیید شرکت',
      message: `آیا از تأیید شرکت «${company.name}» برای فهرست شرکت‌های مورد تأیید دانشکده اطمینان دارید؟`,
      icon: 'pi pi-question-circle',
      acceptLabel: 'تأیید شرکت',
      rejectLabel: 'انصراف',
      accept: () => {
        if (this.approvingId() !== null) return;
        this.approvingId.set(company.id);
        this.approval.approve(company.id).pipe(finalize(() => this.approvingId.set(null))).subscribe({
          next: (approved) => {
            this.eligible.update((companies) => companies.filter((item) => item.id !== approved.id));
            this.companies.update((companies) => [...companies.filter((item) => item.id !== approved.id), approved]
              .sort((a, b) => a.name.localeCompare(b.name, 'fa')));
            this.allCompanies.update(items => items.map(item => item.id === approved.id ? approved : item));
            this.detailVisible.set(false);
            this.messages.add({ severity: 'success', summary: 'تأیید شد', detail: 'شرکت به فهرست شرکت‌های مورد تأیید دانشکده اضافه شد.' });
          },
          error: (error: HttpErrorResponse) => {
            this.showError(error, 'تأیید شرکت ناموفق بود.');
            this.detailVisible.set(false);
            this.load();
          }
        });
      }
    });
  }

  private showError(error: HttpErrorResponse, fallback: string): void {
    this.messages.add({ severity: 'error', summary: 'خطا', detail: userErrorMessage(error, fallback) });
  }
}
