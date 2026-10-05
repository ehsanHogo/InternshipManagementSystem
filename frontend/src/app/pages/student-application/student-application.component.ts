import { CompanyRatingComponent } from '../../shared/company-rating.component';
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
  StudentInternshipCase,
  InternshipCaseStatus,
  InternshipPreference,
  internshipStatusLabels,
  internshipStatusSeverity,
  professorFinalResultLabels
} from '../../internship/internship.models';
import { CaseReportsComponent } from '../../shared/case-reports.component';
import { InternshipService } from '../../internship/internship.service';
import { InternshipTermService } from '../../internship/internship-term.service';
import { InternshipTerm, internshipTermLabel } from '../../internship/internship-term.models';
import { userErrorMessage } from '../../shared/http-error-message';
import { JalaliDatePipe } from '../../shared/jalali-date/jalali-date.pipe';
import { PersianDigitsPipe } from '../../shared/persian-digits.pipe';

@Component({
  selector: 'app-student-application',
  imports: [
    CompanyRatingComponent,
    CaseReportsComponent,
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
  private readonly termService = inject(InternshipTermService);
  readonly openTerm = signal<InternshipTerm | null>(null);
  readonly termLabel = internshipTermLabel;
  private readonly messages = inject(MessageService);
  private readonly confirmation = inject(ConfirmationService);

  readonly internshipCase = signal<StudentInternshipCase | null>(null);
  readonly historicalCases = signal<StudentInternshipCase[]>([]);
  readonly viewingHistory = signal(false);
  readonly historyLoading = signal(false);
  private latestCase: StudentInternshipCase | null = null;
  readonly acceptedApplications = signal<AcceptedOpportunityApplication[]>([]);
  readonly loading = signal(true);
  readonly loadFailed = signal(false);
  readonly creating = signal(false);
  readonly saving = signal(false);
  readonly ratingSaving = signal(false);
  readonly selectedRating = signal(0);
  readonly ratingStars = [1, 2, 3, 4, 5];
  readonly preferenceSaving = signal(false);

  readonly applicationForm = this.formBuilder.group({
    passedCredits: this.formBuilder.control<number | null>(null, [Validators.required, Validators.min(0)]),
    mobile: this.formBuilder.nonNullable.control('', Validators.required)
  });

  constructor() {
    this.loadPage();
  }

  reloadPage(): void {
    this.loadPage();
  }

  get hasPassedCase(): boolean {
    return this.historicalCases().some(item => item.status === 'PASSED');
  }

  get canCreateCase(): boolean {
    return !!this.openTerm() && !this.latestCase && !this.hasPassedCase && !this.viewingHistory() &&
      !this.loading() && !this.loadFailed() && !this.historyLoading();
  }

  viewHistoricalCase(item: StudentInternshipCase): void {
    if (this.ratingSaving() || this.historyLoading() || this.saving() || this.preferenceSaving() || this.creating()) return;
    this.historyLoading.set(true);
    this.internshipService.getStudentHistoricalCase(item.id)
      .pipe(finalize(() => this.historyLoading.set(false)))
      .subscribe({
        next: detail => {
          this.viewingHistory.set(true);
          this.setCase(detail);
        },
        error: (error: HttpErrorResponse) => this.showError(error)
      });
  }

  returnToCurrentCase(): void {
    if (this.ratingSaving()) return;
    this.viewingHistory.set(false);
    if (this.latestCase) this.setCase(this.latestCase);
    else this.internshipCase.set(null);
  }

  get isDraft(): boolean {
    return !this.viewingHistory() && this.internshipCase()?.status === 'DRAFT';
  }

  get isRevisionRequested(): boolean {
    return !this.viewingHistory() && this.internshipCase()?.status === 'REVISION_REQUESTED';
  }

  get canEditPreferences(): boolean {
    return this.isDraft || this.isRevisionRequested;
  }

  get canSubmit(): boolean {
    const count = this.internshipCase()?.preferences.length ?? 0;
    const item = this.internshipCase();
    const detailsValid = this.isDraft ? this.applicationForm.valid :
      item?.passedCredits != null && !!item.mobile?.trim();
    return this.canEditPreferences && detailsValid && count >= 1 && count <= 3 &&
      !!item?.preferences.every((preference, index) => preference.priority === index + 1);
  }

  readonly internshipStatusSeverity = internshipStatusSeverity;

  statusLabel(status: InternshipCaseStatus): string {
    return internshipStatusLabels[status];
  }

  finalResultLabel(internshipCase: StudentInternshipCase): string {
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
    if (!this.canCreateCase || this.creating()) return;
    this.creating.set(true);
    this.internshipService
      .createCase()
      .pipe(finalize(() => this.creating.set(false)))
      .subscribe({
        next: (internshipCase) => {
          this.setCase(internshipCase);
          this.messages.add({ severity: 'success', summary: 'ایجاد شد', detail: 'پیش‌نویس درخواست رسمی ایجاد شد.' });
        },
        error: (error: HttpErrorResponse) => {
          this.showError(error);
          if (error.error?.code === 'NO_OPEN_INTERNSHIP_TERM') this.loadPage();
        }
      });
  }

  saveApplication(): void {
    if (!this.isDraft || this.saving() || this.preferenceSaving() || this.historyLoading()) return;
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
    if (this.saving() || this.preferenceSaving() || this.historyLoading()) return;
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

  submitCompanyRating(): void {
    const item = this.internshipCase();
    const rating = this.selectedRating();
    if (!item?.canRateInternship || item.status !== 'PASSED' || rating < 1 || rating > 5 || this.ratingSaving()) return;
    this.ratingSaving.set(true);
    this.internshipService.rateCompany(item.id, rating)
      .pipe(finalize(() => this.ratingSaving.set(false)))
      .subscribe({
        next: detail => {
          this.setCase(detail);
          this.historicalCases.update(items => items.map(past => past.id === detail.id ? detail : past));
          this.messages.add({ severity: 'success', summary: 'ثبت شد', detail: 'امتیاز شما به شرکت ثبت شد و قابل تغییر نیست.' });
        },
        error: (error: HttpErrorResponse) => {
          this.showError(error);
          if (error.error?.code === 'STUDENT_RATING_ALREADY_EXISTS') {
            this.internshipService.getStudentHistoricalCase(item.id).subscribe({
              next: detail => this.setCase(detail),
              error: (refreshError: HttpErrorResponse) => this.showError(refreshError)
            });
          }
        }
      });
  }

  private loadPage(): void {
    this.loading.set(true);
    this.loadFailed.set(false);
    forkJoin({
      openTerm: this.termService.current(),
      historicalCases: this.internshipService.listStudentHistoricalCases(),
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
        next: ({ openTerm, acceptedApplications, internshipCase, historicalCases }) => {
          this.openTerm.set(openTerm);
          this.historicalCases.set(historicalCases);
          this.viewingHistory.set(false);
          this.latestCase = internshipCase;
          this.acceptedApplications.set(acceptedApplications);
          if (internshipCase) {
            this.setCase(internshipCase);
          } else {
            this.internshipCase.set(null);
          }
        },
        error: (error: HttpErrorResponse) => {
          this.loadFailed.set(true);
          this.showError(error);
        }
      });
  }

  private replacePreferences(applicationIDs: number[], successMessage: string): void {
    if (!this.canEditPreferences || this.preferenceSaving() || this.saving() || this.historyLoading()) return;
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

  private setCase(internshipCase: StudentInternshipCase, syncApplicationForm = true): void {
    this.selectedRating.set(0);
    this.internshipCase.set(internshipCase);
    if (!this.viewingHistory()) this.latestCase = internshipCase;
    if (syncApplicationForm) {
      this.applicationForm.reset({
        passedCredits: internshipCase.passedCredits,
        mobile: internshipCase.mobile ?? ''
      });
    }
    if (this.isDraft) {
      this.applicationForm.enable({ emitEvent: false });
    } else {
      this.applicationForm.disable({ emitEvent: false });
    }
  }

  private submitCase(): void {
    if (!this.canSubmit || this.saving() || this.preferenceSaving() || this.historyLoading()) return;
    const value = this.applicationForm.getRawValue();
    this.saving.set(true);
    const submission = this.isDraft
      ? this.internshipService.updateCase(value.passedCredits, value.mobile.trim()).pipe(
          switchMap(() => this.internshipService.submitCase()))
      : this.internshipService.submitCase();
    submission
      .pipe(
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
