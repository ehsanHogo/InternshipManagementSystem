import { HttpErrorResponse } from '@angular/common/http';
import { Component, computed, inject, signal } from '@angular/core';
import { FormBuilder, ReactiveFormsModule, Validators } from '@angular/forms';
import { RouterLink } from '@angular/router';
import { finalize } from 'rxjs';
import { ConfirmationService, MessageService } from 'primeng/api';
import { ButtonModule } from 'primeng/button';
import { ConfirmDialogModule } from 'primeng/confirmdialog';
import { DialogModule } from 'primeng/dialog';
import { InputNumberModule } from 'primeng/inputnumber';
import { SelectModule } from 'primeng/select';
import { TableModule } from 'primeng/table';
import { TagModule } from 'primeng/tag';
import { InternshipTerm, InternshipTermDetail, InternshipTermType, internshipTermLabel } from '../../internship/internship-term.models';
import { InternshipTermService } from '../../internship/internship-term.service';
import { InternshipCaseStatus, internshipStatusLabels, internshipStatusSeverity } from '../../internship/internship.models';
import { userErrorMessage } from '../../shared/http-error-message';
import { JalaliDatePipe } from '../../shared/jalali-date/jalali-date.pipe';
import { PersianDigitsPipe } from '../../shared/persian-digits.pipe';

@Component({
  selector: 'app-university-internship-terms',
  imports: [ReactiveFormsModule, RouterLink, ButtonModule, ConfirmDialogModule, DialogModule,
    InputNumberModule, SelectModule, TableModule, TagModule, JalaliDatePipe, PersianDigitsPipe],
  providers: [ConfirmationService],
  templateUrl: './university-internship-terms.component.html',
  styleUrl: '../workflow-page.scss'
})
export class UniversityInternshipTermsComponent {
  private readonly service = inject(InternshipTermService);
  private readonly messages = inject(MessageService);
  private readonly confirmation = inject(ConfirmationService);
  private readonly formBuilder = inject(FormBuilder);

  readonly terms = signal<InternshipTerm[]>([]);
  readonly currentTerm = computed(() => this.terms().find(term => term.status === 'OPEN'));
  readonly loading = signal(true);
  readonly loadFailed = signal(false);
  readonly saving = signal(false);
  readonly createVisible = signal(false);
  readonly detailVisible = signal(false);
  readonly detailLoading = signal(false);
  readonly detailFailed = signal(false);
  readonly detailID = signal<number | null>(null);
  readonly detail = signal<InternshipTermDetail | null>(null);
  readonly termLabel = internshipTermLabel;
  statusLabel(status: InternshipCaseStatus): string { return internshipStatusLabels[status]; }
  readonly statusSeverity = internshipStatusSeverity;
  readonly types = [
    { label: 'نیمسال اول', value: 'FIRST' },
    { label: 'نیمسال دوم', value: 'SECOND' },
    { label: 'تابستان', value: 'SUMMER' }
  ];
  readonly form = this.formBuilder.group({
    academicYear: this.formBuilder.control<number | null>(null, [Validators.required, Validators.min(1), Validators.max(9999), Validators.pattern(/^\d+$/)]),
    termType: this.formBuilder.control<InternshipTermType | null>(null, Validators.required)
  });

  constructor() { this.load(); }

  load(): void {
    this.loading.set(true);
    this.loadFailed.set(false);
    this.service.list().pipe(finalize(() => this.loading.set(false))).subscribe({
      next: terms => this.terms.set(terms),
      error: (error: HttpErrorResponse) => { this.loadFailed.set(true); this.showError(error); }
    });
  }

  openCreate(): void {
    this.form.reset();
    this.createVisible.set(true);
  }

  create(): void {
    if (this.saving()) return;
    if (this.form.invalid) { this.form.markAllAsTouched(); return; }
    const value = this.form.getRawValue();
    this.saving.set(true);
    this.service.create(value.academicYear!, value.termType!).pipe(finalize(() => this.saving.set(false))).subscribe({
      next: () => {
        this.createVisible.set(false);
        this.messages.add({ severity: 'success', summary: 'ترم باز شد', detail: 'پرونده‌های جدید به‌صورت خودکار به این ترم اختصاص می‌یابند.' });
        this.load();
      },
      error: (error: HttpErrorResponse) => { this.showError(error); if (error.status === 409) this.load(); }
    });
  }

  inspect(id: number): void {
    if (this.detailLoading()) return;
    this.detailID.set(id);
    this.detail.set(null);
    this.detailVisible.set(true);
    this.detailLoading.set(true);
    this.detailFailed.set(false);
    this.service.detail(id).pipe(finalize(() => this.detailLoading.set(false))).subscribe({
      next: detail => this.detail.set(detail),
      error: (error: HttpErrorResponse) => { this.detailFailed.set(true); this.showError(error); }
    });
  }

  confirmClose(term: InternshipTerm): void {
    if (this.saving() || term.status !== 'OPEN') return;
    this.confirmation.confirm({
      header: 'بستن ترم کارآموزی',
      message: 'با بستن این ترم، تمام پرونده‌هایی که هنوز به وضعیت قبول‌شده، مردود یا لغوشده نرسیده‌اند، به‌صورت خودکار لغو خواهند شد. بستن ترم قابل بازگشت نیست. آیا ادامه می‌دهید؟',
      icon: 'pi pi-exclamation-triangle',
      acceptLabel: 'بله، ترم بسته شود', rejectLabel: 'انصراف',
      acceptButtonStyleClass: 'p-button-danger',
      accept: () => this.close(term.id)
    });
  }

  private close(id: number): void {
    if (this.saving()) return;
    this.saving.set(true);
    this.service.close(id).pipe(finalize(() => this.saving.set(false))).subscribe({
      next: () => {
        this.messages.add({ severity: 'success', summary: 'ترم بسته شد', detail: 'تمام پرونده‌های ناتمام این ترم به‌صورت خودکار لغو شدند.' });
        this.load();
        if (this.detailVisible() && this.detailID() === id) this.inspect(id);
      },
      error: (error: HttpErrorResponse) => { this.showError(error); if (error.status === 409) this.load(); }
    });
  }

  private showError(error: HttpErrorResponse): void {
    this.messages.add({ severity: 'error', summary: 'عملیات ناموفق', detail: userErrorMessage(error, 'انجام عملیات ترم کارآموزی امکان‌پذیر نبود.') });
  }
}
