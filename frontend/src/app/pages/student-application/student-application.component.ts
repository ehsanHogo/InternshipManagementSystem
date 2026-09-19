import { HttpErrorResponse } from '@angular/common/http';
import { Component, inject, signal } from '@angular/core';
import { FormBuilder, ReactiveFormsModule, Validators } from '@angular/forms';
import { finalize, switchMap } from 'rxjs';
import { ConfirmationService, MessageService } from 'primeng/api';
import { ButtonModule } from 'primeng/button';
import { CardModule } from 'primeng/card';
import { ConfirmDialogModule } from 'primeng/confirmdialog';
import { InputNumberModule } from 'primeng/inputnumber';
import { InputTextModule } from 'primeng/inputtext';
import { RadioButtonModule } from 'primeng/radiobutton';
import { SelectModule } from 'primeng/select';
import { TagModule } from 'primeng/tag';

import {
  Company,
  InternshipCase,
  InternshipCaseStatus,
  InternshipPreference,
  PreferencePayload,
  internshipStatusLabels,
  professorFinalResultLabels
} from '../../internship/internship.models';
import { InternshipService } from '../../internship/internship.service';

@Component({
  selector: 'app-student-application',
  imports: [
    ReactiveFormsModule,
    ButtonModule,
    CardModule,
    ConfirmDialogModule,
    InputNumberModule,
    InputTextModule,
    RadioButtonModule,
    SelectModule,
    TagModule
  ],
  providers: [ConfirmationService],
  templateUrl: './student-application.component.html',
  styleUrl: './student-application.component.scss'
})
export class StudentApplicationComponent {
  private readonly formBuilder = inject(FormBuilder);
  private readonly internshipService = inject(InternshipService);
  private readonly messages = inject(MessageService);
  private readonly confirmation = inject(ConfirmationService);

  readonly internshipCase = signal<InternshipCase | null>(null);
  readonly companies = signal<Company[]>([]);
  readonly loading = signal(true);
  readonly creating = signal(false);
  readonly saving = signal(false);
  readonly preferenceSaving = signal(false);
  readonly editingPreferenceID = signal<number | null>(null);

  readonly applicationForm = this.formBuilder.group({
    passedCredits: this.formBuilder.control<number | null>(null, [Validators.required, Validators.min(0)]),
    mobile: this.formBuilder.nonNullable.control('', Validators.required)
  });

  readonly preferenceForm = this.formBuilder.group({
    priority: this.formBuilder.control<number | null>(null, Validators.required),
    companyType: this.formBuilder.nonNullable.control<'approved' | 'proposed'>('approved'),
    companyId: this.formBuilder.control<number | null>(null),
    proposedCompanyName: this.formBuilder.nonNullable.control(''),
    proposedWebsite: this.formBuilder.nonNullable.control(''),
    proposedPhone: this.formBuilder.nonNullable.control(''),
    proposedEmail: this.formBuilder.nonNullable.control('', Validators.email),
    proposedSupervisorName: this.formBuilder.nonNullable.control(''),
    city: this.formBuilder.nonNullable.control('', Validators.required),
    workField: this.formBuilder.nonNullable.control('', Validators.required)
  });

  constructor() {
    this.loadCompanies();
    this.loadCase();
  }

  get isDraft(): boolean {
    return this.internshipCase()?.status === 'DRAFT';
  }

  get canAddPreference(): boolean {
    return this.isDraft && (this.internshipCase()?.preferences.length ?? 0) < 3;
  }

  statusLabel(status: InternshipCaseStatus): string {
    return internshipStatusLabels[status];
  }

  statusSeverity(status: InternshipCaseStatus): 'secondary' | 'info' | 'success' | 'contrast' {
    if (status === 'PENDING_UNIVERSITY_APPROVAL' || status === 'PENDING_COMPANY_APPROVAL') return 'info';
    if (status === 'COMPANY_APPROVED' || status === 'UNIVERSITY_APPROVED') return 'success';
    if (status === 'ACTIVE') return 'success';
    if (status === 'COMPLETED') return 'contrast';
    return 'secondary';
  }

  finalResultLabel(internshipCase: InternshipCase): string {
    return internshipCase.finalResult ? professorFinalResultLabels[internshipCase.finalResult] : '—';
  }

  priorityLabel(priority: number): string {
    return ['اول', 'دوم', 'سوم'][priority - 1] ?? String(priority);
  }

  createCase(): void {
    this.creating.set(true);
    this.internshipService
      .createCase()
      .pipe(finalize(() => this.creating.set(false)))
      .subscribe({
        next: (internshipCase) => {
          this.setCase(internshipCase);
          this.messages.add({ severity: 'success', summary: 'ایجاد شد', detail: 'پیش‌نویس درخواست کارآموزی ایجاد شد.' });
        },
        error: (error: HttpErrorResponse) => this.showError(error)
      });
  }

  saveApplication(): void {
    if (this.applicationForm.invalid) {
      this.applicationForm.markAllAsTouched();
      return;
    }
    const value = this.applicationForm.getRawValue();
    this.saving.set(true);
    this.internshipService
      .updateCase(value.passedCredits, value.mobile.trim())
      .pipe(finalize(() => this.saving.set(false)))
      .subscribe({
        next: (internshipCase) => {
          this.setCase(internshipCase);
          this.messages.add({ severity: 'success', summary: 'ذخیره شد', detail: 'اطلاعات درخواست ذخیره شد.' });
        },
        error: (error: HttpErrorResponse) => this.showError(error)
      });
  }

  editPreference(preference: InternshipPreference): void {
    this.editingPreferenceID.set(preference.id);
    this.preferenceForm.reset({
      priority: preference.priority,
      companyType: preference.companyId ? 'approved' : 'proposed',
      companyId: preference.companyId ?? null,
      proposedCompanyName: preference.proposedCompanyName ?? '',
      proposedWebsite: preference.proposedWebsite ?? '',
      proposedPhone: preference.proposedPhone ?? '',
      proposedEmail: preference.proposedEmail ?? '',
      proposedSupervisorName: preference.proposedSupervisorName ?? '',
      city: preference.city,
      workField: preference.workField
    });
    document.querySelector('.preference-editor')?.scrollIntoView({ behavior: 'smooth', block: 'start' });
  }

  cancelPreferenceEdit(): void {
    this.editingPreferenceID.set(null);
    this.preferenceForm.reset({
      priority: null,
      companyType: 'approved',
      companyId: null,
      proposedCompanyName: '',
      proposedWebsite: '',
      proposedPhone: '',
      proposedEmail: '',
      proposedSupervisorName: '',
      city: '',
      workField: ''
    });
  }

  savePreference(): void {
    const value = this.preferenceForm.getRawValue();
    const companyIsValid = value.companyType === 'approved' ? value.companyId !== null : value.proposedCompanyName.trim() !== '';
    if (this.preferenceForm.invalid || !companyIsValid || value.priority === null) {
      this.preferenceForm.markAllAsTouched();
      this.messages.add({ severity: 'warn', summary: 'اطلاعات ناقص', detail: 'فیلدهای ضروری اولویت را تکمیل کنید.' });
      return;
    }

    const payload: PreferencePayload = {
      priority: value.priority,
      city: value.city.trim(),
      workField: value.workField.trim()
    };
    if (value.companyType === 'approved') {
      payload.companyId = value.companyId!;
    } else {
      payload.proposedCompanyName = value.proposedCompanyName.trim();
      payload.proposedWebsite = this.optional(value.proposedWebsite);
      payload.proposedPhone = this.optional(value.proposedPhone);
      payload.proposedEmail = this.optional(value.proposedEmail);
      payload.proposedSupervisorName = this.optional(value.proposedSupervisorName);
    }

    const editingID = this.editingPreferenceID();
    const request = editingID === null
      ? this.internshipService.addPreference(payload)
      : this.internshipService.updatePreference(editingID, payload);
    this.preferenceSaving.set(true);
    request
      .pipe(
        switchMap(() => this.internshipService.getCurrentCase()),
        finalize(() => this.preferenceSaving.set(false))
      )
      .subscribe({
        next: (internshipCase) => {
          this.setCase(internshipCase, false);
          this.cancelPreferenceEdit();
          this.messages.add({ severity: 'success', summary: 'ذخیره شد', detail: 'اولویت کارآموزی ذخیره شد.' });
        },
        error: (error: HttpErrorResponse) => this.showError(error)
      });
  }

  deletePreference(preference: InternshipPreference): void {
    this.internshipService
      .deletePreference(preference.id)
      .pipe(switchMap(() => this.internshipService.getCurrentCase()))
      .subscribe({
        next: (internshipCase) => {
          this.setCase(internshipCase, false);
          if (this.editingPreferenceID() === preference.id) this.cancelPreferenceEdit();
          this.messages.add({ severity: 'success', summary: 'حذف شد', detail: 'اولویت انتخابی حذف شد.' });
        },
        error: (error: HttpErrorResponse) => this.showError(error)
      });
  }

  confirmSubmit(): void {
    const preferenceCount = this.internshipCase()?.preferences.length ?? 0;
    if (this.applicationForm.invalid || preferenceCount < 1 || preferenceCount > 3) {
      this.applicationForm.markAllAsTouched();
      this.messages.add({
        severity: 'warn',
        summary: 'درخواست ناقص است',
        detail: 'اطلاعات درخواست و حداقل یک اولویت کارآموزی را تکمیل کنید.'
      });
      return;
    }
    this.confirmation.confirm({
      header: 'ثبت نهایی درخواست',
      message: 'پس از ثبت نهایی، امکان ویرایش درخواست وجود نخواهد داشت. آیا از ثبت درخواست اطمینان دارید؟',
      icon: 'pi pi-exclamation-triangle',
      acceptLabel: 'بله، ثبت شود',
      rejectLabel: 'انصراف',
      acceptButtonStyleClass: 'p-button-primary',
      rejectButtonStyleClass: 'p-button-text p-button-secondary',
      accept: () => this.submitCase()
    });
  }

  private loadCompanies(): void {
    this.internshipService.listCompanies().subscribe({
      next: (companies) => this.companies.set(companies),
      error: (error: HttpErrorResponse) => this.showError(error)
    });
  }

  private loadCase(): void {
    this.loading.set(true);
    this.internshipService
      .getCurrentCase()
      .pipe(finalize(() => this.loading.set(false)))
      .subscribe({
        next: (internshipCase) => this.setCase(internshipCase),
        error: (error: HttpErrorResponse) => {
          if (error.status === 404) {
            this.internshipCase.set(null);
            return;
          }
          this.showError(error);
        }
      });
  }

  private setCase(internshipCase: InternshipCase, syncApplicationForm = true): void {
    this.internshipCase.set(internshipCase);
    if (!syncApplicationForm) return;
    this.applicationForm.reset({
      passedCredits: internshipCase.passedCredits,
      mobile: internshipCase.mobile ?? ''
    });
    if (internshipCase.status === 'DRAFT') {
      this.applicationForm.enable({ emitEvent: false });
    } else {
      this.applicationForm.disable({ emitEvent: false });
      this.cancelPreferenceEdit();
    }
  }

  private submitCase(): void {
    const value = this.applicationForm.getRawValue();
    this.saving.set(true);
    this.internshipService
      .updateCase(value.passedCredits, value.mobile.trim())
      .pipe(
        switchMap(() => this.internshipService.submitCase()),
        finalize(() => this.saving.set(false))
      )
      .subscribe({
        next: (internshipCase) => {
          this.setCase(internshipCase);
          this.messages.add({
            severity: 'success',
            summary: 'ثبت نهایی انجام شد',
            detail: 'درخواست کارآموزی با موفقیت ثبت شد.'
          });
        },
        error: (error: HttpErrorResponse) => this.showError(error)
      });
  }

  private optional(value: string): string | undefined {
    const trimmed = value.trim();
    return trimmed || undefined;
  }

  private showError(error: HttpErrorResponse): void {
    let detail = 'خطایی غیرمنتظره رخ داد. لطفاً دوباره تلاش کنید.';
    if (error.status === 0) detail = 'ارتباط با سرور برقرار نشد.';
    if (error.status === 400) detail = 'اطلاعات واردشده کامل یا معتبر نیست.';
    if (error.status === 409) detail = 'این عملیات با وضعیت فعلی درخواست یا اولویت‌های موجود سازگار نیست.';
    this.messages.add({ severity: 'error', summary: 'عملیات ناموفق', detail });
  }
}
