import { HttpErrorResponse } from '@angular/common/http';
import { Component, computed, inject, signal } from '@angular/core';
import { FormBuilder, ReactiveFormsModule, Validators } from '@angular/forms';
import { finalize, forkJoin } from 'rxjs';
import { ConfirmationService, MessageService } from 'primeng/api';
import { ConfirmDialogModule } from 'primeng/confirmdialog';
import { ButtonModule } from 'primeng/button';
import { DialogModule } from 'primeng/dialog';
import { InputTextModule } from 'primeng/inputtext';
import { TableModule } from 'primeng/table';
import { TagModule } from 'primeng/tag';
import { TextareaModule } from 'primeng/textarea';

import { Company } from '../../internship/internship.models';
import { UniversityManagementService } from '../../university-management/university-management.service';
import { userErrorMessage } from '../../shared/http-error-message';
import { CompanyApprovalService } from '../../company/company-approval.service';
import { ApprovalCompanyDetail, EligibleCompany } from '../../company/company-approval.models';
import { JalaliDatePipe } from '../../shared/jalali-date/jalali-date.pipe';
import { PersianDigitsPipe } from '../../shared/persian-digits.pipe';

@Component({
  selector: 'app-university-companies',
  imports: [ReactiveFormsModule, ButtonModule, DialogModule, InputTextModule, TableModule, TagModule, TextareaModule, ConfirmDialogModule, JalaliDatePipe, PersianDigitsPipe],
  providers: [ConfirmationService],
  templateUrl: './university-companies.component.html',
  styleUrl: '../workflow-page.scss'
})
export class UniversityCompaniesComponent {
  private readonly management = inject(UniversityManagementService);
  private readonly messages = inject(MessageService);
  private readonly formBuilder = inject(FormBuilder);
  private readonly approval = inject(CompanyApprovalService);
  private readonly confirmation = inject(ConfirmationService);

  readonly companies = signal<Company[]>([]);
  readonly eligible = signal<EligibleCompany[]>([]);
  readonly tab = signal<'eligible' | 'approved'>('eligible');
  readonly visibleCompanies = computed(() => this.tab() === 'eligible' ? this.eligible() : this.companies());
  readonly detail = signal<ApprovalCompanyDetail | null>(null);
  readonly detailVisible = signal(false);
  readonly detailLoading = signal(false);
  readonly detailFailed = signal(false);
  readonly detailId = signal<number | null>(null);
  readonly approvingId = signal<number | null>(null);
  readonly loading = signal(true);
  readonly loadFailed = signal(false);
  readonly dialogVisible = signal(false);
  submitting = false;
  readonly form = this.formBuilder.nonNullable.group({
    name: ['', Validators.required], nationalId: ['', Validators.required], economicCode: ['', Validators.required],
    website: [''], phone: [''],
    email: ['', Validators.email], address: ['']
  });

  constructor() { this.load(); }

  openDialog(): void {
    this.form.reset();
    this.dialogVisible.set(true);
  }

  submit(): void {
    if (this.form.invalid) { this.form.markAllAsTouched(); return; }
    this.submitting = true;
    const raw = this.form.getRawValue();
    this.management.createCompany({
      name: raw.name.trim(), nationalId: raw.nationalId.trim(), economicCode: raw.economicCode.trim(), website: raw.website.trim(), phone: raw.phone.trim(),
      email: raw.email.trim(), address: raw.address.trim()
    }).pipe(finalize(() => (this.submitting = false))).subscribe({
      next: () => {
        this.dialogVisible.set(false);
        this.messages.add({ severity: 'success', summary: 'ثبت شد', detail: 'شرکت موردنظر به فهرست شرکت‌های تأییدشده اضافه شد.' });
        this.load();
      },
      error: (error: HttpErrorResponse) => this.showError(error, 'ایجاد شرکت ناموفق بود.')
    });
  }

  load(): void {
    this.loading.set(true);
    this.loadFailed.set(false);
    forkJoin({ eligible: this.approval.listEligible(), approved: this.approval.listApproved() }).subscribe({
      next: ({ eligible, approved }) => {
        this.eligible.set(eligible);
        this.companies.set(approved);
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
