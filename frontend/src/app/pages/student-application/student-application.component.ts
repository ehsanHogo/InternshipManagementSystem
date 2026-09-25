import { HttpErrorResponse } from '@angular/common/http';
import { Component, inject, signal } from '@angular/core';
import { FormBuilder, ReactiveFormsModule, Validators } from '@angular/forms';
import { RouterLink } from '@angular/router';
import { catchError, finalize, forkJoin, of, switchMap } from 'rxjs';
import { ConfirmationService, MessageService } from 'primeng/api';
import { ButtonModule } from 'primeng/button';
import { CardModule } from 'primeng/card';
import { ConfirmDialogModule } from 'primeng/confirmdialog';
import { InputNumberModule } from 'primeng/inputnumber';
import { InputTextModule } from 'primeng/inputtext';
import { TagModule } from 'primeng/tag';

import {
  AcceptedOpportunityApplication,
  InternshipCase,
  InternshipCaseStatus,
  InternshipPreference,
  internshipStatusLabels,
  professorFinalResultLabels
} from '../../internship/internship.models';
import { InternshipService } from '../../internship/internship.service';
import { userErrorMessage } from '../../shared/http-error-message';
import { JalaliDatePipe } from '../../shared/jalali-date/jalali-date.pipe';
import { PersianDigitsPipe } from '../../shared/persian-digits.pipe';

@Component({
  selector: 'app-student-application',
  imports: [
    ReactiveFormsModule,
    RouterLink,
    ButtonModule,
    CardModule,
    ConfirmDialogModule,
    InputNumberModule,
    InputTextModule,
    TagModule,
    JalaliDatePipe,
    PersianDigitsPipe
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
  readonly acceptedApplications = signal<AcceptedOpportunityApplication[]>([]);
  readonly loading = signal(true);
  readonly creating = signal(false);
  readonly saving = signal(false);
  readonly preferenceSaving = signal(false);

  readonly applicationForm = this.formBuilder.group({
    passedCredits: this.formBuilder.control<number | null>(null, [Validators.required, Validators.min(0)]),
    mobile: this.formBuilder.nonNullable.control('', Validators.required)
  });

  constructor() {
    this.loadPage();
  }

  get isDraft(): boolean {
    return this.internshipCase()?.status === 'DRAFT';
  }

  get canSubmit(): boolean {
    const count = this.internshipCase()?.preferences.length ?? 0;
    return this.isDraft && this.applicationForm.valid && count >= 1 && count <= 3;
  }

  statusLabel(status: InternshipCaseStatus): string {
    return internshipStatusLabels[status];
  }

  statusSeverity(status: InternshipCaseStatus): 'secondary' | 'info' | 'success' | 'contrast' | 'danger' {
    if (status === 'PENDING_UNIVERSITY_REVIEW' || status === 'PENDING_COMPANY_DETAILS') return 'info';
    if (status === 'PENDING_FINAL_APPROVAL' || status === 'READY_TO_START' || status === 'ACTIVE') return 'success';
    if (status === 'COMPLETED') return 'contrast';
    if (status === 'CANCELLED') return 'danger';
    return 'secondary';
  }

  finalResultLabel(internshipCase: InternshipCase): string {
    return internshipCase.finalResult ? professorFinalResultLabels[internshipCase.finalResult] : '—';
  }

  priorityLabel(priority: number): string {
    return ['۱', '۲', '۳'][priority - 1] ?? String(priority);
  }

  isSelected(applicationID: number): boolean {
    return this.internshipCase()?.preferences.some(
      (preference) => preference.opportunityApplicationId === applicationID
    ) ?? false;
  }

  createCase(): void {
    this.creating.set(true);
    this.internshipService
      .createCase()
      .pipe(finalize(() => this.creating.set(false)))
      .subscribe({
        next: (internshipCase) => {
          this.setCase(internshipCase);
          this.messages.add({ severity: 'success', summary: 'ایجاد شد', detail: 'پیش‌نویس درخواست رسمی ایجاد شد.' });
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
          this.messages.add({ severity: 'success', summary: 'ذخیره شد', detail: 'مشخصات درخواست رسمی ذخیره شد.' });
        },
        error: (error: HttpErrorResponse) => this.showError(error)
      });
  }

  selectApplication(application: AcceptedOpportunityApplication): void {
    const ids = this.selectedApplicationIDs();
    if (ids.length >= 3 || ids.includes(application.id)) return;
    this.replacePreferences([...ids, application.id], 'فرصت پذیرفته‌شده به اولویت‌ها افزوده شد.');
  }

  movePreference(preference: InternshipPreference, offset: -1 | 1): void {
    const preferences = [...(this.internshipCase()?.preferences ?? [])].sort((a, b) => a.priority - b.priority);
    const index = preferences.findIndex((item) => item.id === preference.id);
    const target = index + offset;
    if (index < 0 || target < 0 || target >= preferences.length) return;
    [preferences[index], preferences[target]] = [preferences[target], preferences[index]];
    this.replacePreferences(
      preferences.map((item) => item.opportunityApplicationId),
      'ترتیب اولویت‌ها به‌روزرسانی شد.'
    );
  }

  deletePreference(preference: InternshipPreference): void {
    this.confirmation.confirm({
      header: 'حذف اولویت',
      message: `آیا از حذف اولویت ${this.priorityLabel(preference.priority)} اطمینان دارید؟`,
      icon: 'pi pi-exclamation-triangle',
      acceptLabel: 'بله، حذف شود',
      rejectLabel: 'انصراف',
      acceptButtonStyleClass: 'p-button-danger',
      rejectButtonStyleClass: 'p-button-text p-button-secondary',
      accept: () => this.replacePreferences(
        this.selectedApplicationIDs().filter((id) => id !== preference.opportunityApplicationId),
        'اولویت انتخابی حذف شد.'
      )
    });
  }

  confirmSubmit(): void {
    if (!this.canSubmit) {
      this.applicationForm.markAllAsTouched();
      this.messages.add({
        severity: 'warn',
        summary: 'درخواست ناقص است',
        detail: 'مشخصات درخواست و حداقل یک اولویت پذیرفته‌شده را تکمیل کنید.'
      });
      return;
    }
    this.confirmation.confirm({
      header: 'ارسال درخواست برای آموزش',
      message: 'آیا از ارسال درخواست کارآموزی برای بررسی آموزش مطمئن هستید؟ پس از ارسال، امکان ویرایش اطلاعات و اولویت‌ها تا تعیین تکلیف پرونده وجود نخواهد داشت.',
      icon: 'pi pi-exclamation-triangle',
      acceptLabel: 'بله، ارسال شود',
      rejectLabel: 'انصراف',
      acceptButtonStyleClass: 'p-button-primary',
      rejectButtonStyleClass: 'p-button-text p-button-secondary',
      accept: () => this.submitCase()
    });
  }

  private loadPage(): void {
    this.loading.set(true);
    forkJoin({
      acceptedApplications: this.internshipService.listAcceptedOpportunityApplications(),
      internshipCase: this.internshipService.getCurrentCase().pipe(
        catchError((error: HttpErrorResponse) => {
          if (error.status === 404) return of(null);
          throw error;
        })
      )
    })
      .pipe(finalize(() => this.loading.set(false)))
      .subscribe({
        next: ({ acceptedApplications, internshipCase }) => {
          this.acceptedApplications.set(acceptedApplications);
          if (internshipCase) this.setCase(internshipCase);
        },
        error: (error: HttpErrorResponse) => this.showError(error)
      });
  }

  private replacePreferences(applicationIDs: number[], successMessage: string): void {
    this.preferenceSaving.set(true);
    this.internshipService
      .replacePreferences(applicationIDs)
      .pipe(finalize(() => this.preferenceSaving.set(false)))
      .subscribe({
        next: (internshipCase) => {
          this.setCase(internshipCase, false);
          this.messages.add({ severity: 'success', summary: 'ذخیره شد', detail: successMessage });
        },
        error: (error: HttpErrorResponse) => this.showError(error)
      });
  }

  private selectedApplicationIDs(): number[] {
    return [...(this.internshipCase()?.preferences ?? [])]
      .sort((a, b) => a.priority - b.priority)
      .map((preference) => preference.opportunityApplicationId);
  }

  private setCase(internshipCase: InternshipCase, syncApplicationForm = true): void {
    this.internshipCase.set(internshipCase);
    if (syncApplicationForm) {
      this.applicationForm.reset({
        passedCredits: internshipCase.passedCredits,
        mobile: internshipCase.mobile ?? ''
      });
    }
    if (internshipCase.status === 'DRAFT') {
      this.applicationForm.enable({ emitEvent: false });
    } else {
      this.applicationForm.disable({ emitEvent: false });
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
            summary: 'ارسال شد',
            detail: 'درخواست رسمی برای بررسی آموزش ارسال شد.'
          });
        },
        error: (error: HttpErrorResponse) => this.showError(error)
      });
  }

  private showError(error: HttpErrorResponse): void {
    this.messages.add({
      severity: 'error',
      summary: 'عملیات ناموفق',
      detail: userErrorMessage(error, 'انجام عملیات درخواست رسمی امکان‌پذیر نبود.')
    });
  }
}
