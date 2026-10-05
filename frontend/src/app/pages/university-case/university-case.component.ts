import { CompanyRatingComponent } from '../../shared/company-rating.component';
import { HttpErrorResponse } from '@angular/common/http';
import { Component, inject, signal } from '@angular/core';
import { FormBuilder, ReactiveFormsModule, Validators } from '@angular/forms';
import { ActivatedRoute, RouterLink } from '@angular/router';
import { finalize } from 'rxjs';
import { ConfirmationService, MessageService } from 'primeng/api';
import { ButtonModule } from 'primeng/button';
import { CardModule } from 'primeng/card';
import { ConfirmDialogModule } from 'primeng/confirmdialog';
import { DialogModule } from 'primeng/dialog';
import { InputTextModule } from 'primeng/inputtext';
import { TagModule } from 'primeng/tag';
import { TextareaModule } from 'primeng/textarea';

import {
  UniversityInternshipCase,
  InternshipCaseStatus,
  InternshipPreference,
  internshipStatusLabels,
  internshipStatusSeverity,
  professorFinalResultLabels
} from '../../internship/internship.models';
import { CaseReportsComponent } from '../../shared/case-reports.component';
import { InternshipService } from '../../internship/internship.service';
import { internshipTermLabel } from '../../internship/internship-term.models';
import { applicationStatusLabels, applicationStatusSeverity } from '../../opportunity/opportunity.models';
import { JalaliDatePickerComponent } from '../../shared/jalali-date/jalali-date-picker.component';
import { JalaliDatePipe } from '../../shared/jalali-date/jalali-date.pipe';
import { externalHref } from '../../shared/external-url';
import { userErrorMessage } from '../../shared/http-error-message';
import { PersianDigitsPipe } from '../../shared/persian-digits.pipe';

@Component({
  selector: 'app-university-case',
  imports: [
    CompanyRatingComponent,
    CaseReportsComponent,
    JalaliDatePipe,
    PersianDigitsPipe,
    JalaliDatePickerComponent,
    ReactiveFormsModule,
    RouterLink,
    ButtonModule,
    CardModule,
    ConfirmDialogModule,
    DialogModule,
    InputTextModule,
    TagModule,
    TextareaModule
  ],
  providers: [ConfirmationService],
  templateUrl: './university-case.component.html',
  styleUrl: '../workflow-page.scss'
})
export class UniversityCaseComponent {
  readonly termLabel = internshipTermLabel;
  private readonly route = inject(ActivatedRoute);
  private readonly formBuilder = inject(FormBuilder);
  private readonly internshipService = inject(InternshipService);
  private readonly messages = inject(MessageService);
  private readonly confirmation = inject(ConfirmationService);
  private readonly caseID = Number(this.route.snapshot.paramMap.get('id'));

  readonly internshipCase = signal<UniversityInternshipCase | null>(null);
  readonly loading = signal(true);
  readonly loadFailed = signal(false);
  readonly saving = signal(false);
  readonly cancelDialogVisible = signal(false);
  readonly correctionDialogVisible = signal(false);

  readonly correctionForm = this.formBuilder.group({
    comment: this.formBuilder.nonNullable.control('', [Validators.required, Validators.pattern(/\S/)])
  });

  readonly reviewForm = this.formBuilder.group({
    preferenceId: this.formBuilder.control<number | null>(null, Validators.required),
    letterNumber: this.formBuilder.nonNullable.control('', [Validators.required, Validators.pattern(/\S/)]),
    letterDate: this.formBuilder.nonNullable.control('', Validators.required)
  });

  readonly cancellationForm = this.formBuilder.group({
    comment: this.formBuilder.nonNullable.control('', [Validators.required, Validators.pattern(/\S/)])
  });

  readonly externalHref = externalHref;

  constructor() {
    this.loadCase();
  }

  readonly internshipStatusSeverity = internshipStatusSeverity;
  readonly applicationStatusLabels = applicationStatusLabels;
  readonly applicationStatusSeverity = applicationStatusSeverity;

  readonly professorFinalResultLabels = professorFinalResultLabels;

  statusLabel(status: InternshipCaseStatus): string {
    return internshipStatusLabels[status];
  }

  selectPreference(preferenceID: number): void {
    this.reviewForm.controls.preferenceId.setValue(preferenceID);
    this.reviewForm.controls.preferenceId.markAsTouched();
  }

  selectedPreference(): InternshipPreference | undefined {
    const preferenceID = this.reviewForm.controls.preferenceId.value;
    return this.internshipCase()?.preferences.find((preference) => preference.id === preferenceID);
  }

  confirmApproval(): void {
    if (this.saving() || this.internshipCase()?.status !== 'PENDING_UNIVERSITY_REVIEW') return;
    if (this.reviewForm.invalid) {
      this.reviewForm.markAllAsTouched();
      this.messages.add({
        severity: 'warn',
        summary: 'اطلاعات ناقص',
        detail: 'یک اولویت، شماره معرفی‌نامه و تاریخ معرفی‌نامه را وارد کنید.'
      });
      return;
    }
    const selected = this.selectedPreference();
    const placement = selected
      ? `${selected.application.opportunity.company.name} — ${selected.application.opportunity.title}`
      : '';
    this.confirmation.confirm({
      header: 'تأیید محل کارآموزی',
      message: `آیا از انتخاب «${placement}» و ثبت معرفی‌نامه اطمینان دارید؟`,
      icon: 'pi pi-check-circle',
      acceptLabel: 'بله، تأیید شود',
      rejectLabel: 'انصراف',
      accept: () => this.approvePlacement()
    });
  }

  openCancellation(): void {
    if (this.saving() || this.internshipCase()?.status !== 'PENDING_UNIVERSITY_REVIEW') return;
    this.cancellationForm.reset({ comment: '' });
    this.cancelDialogVisible.set(true);
  }

  confirmCancellation(): void {
    if (this.saving() || this.internshipCase()?.status !== 'PENDING_UNIVERSITY_REVIEW') return;
    if (this.cancellationForm.invalid) {
      this.cancellationForm.markAllAsTouched();
      return;
    }
    this.confirmation.confirm({
      header: 'تأیید لغو پرونده',
      message: 'کل پرونده رسمی لغو می‌شود و دانشجو می‌تواند درخواست جدیدی ثبت کند. آیا ادامه می‌دهید؟',
      icon: 'pi pi-exclamation-triangle',
      acceptLabel: 'بله، پرونده لغو شود',
      rejectLabel: 'انصراف',
      acceptButtonStyleClass: 'p-button-danger',
      accept: () => this.cancelCase()
    });
  }

  loadCase(): void {
    this.loading.set(true);
    this.loadFailed.set(false);
    this.internshipService.getUniversityCase(this.caseID)
      .pipe(finalize(() => this.loading.set(false)))
      .subscribe({
        next: (internshipCase) => {
          this.internshipCase.set(internshipCase);
          this.syncReviewForm(internshipCase);
        },
        error: (error: HttpErrorResponse) => {
          this.loadFailed.set(true);
          this.internshipCase.set(null);
          this.showError(error);
        }
      });
  }

  confirmFinalApproval(): void {
    if (!this.canReviewPlacement()) return;
    this.confirmation.confirm({
      header: 'تأیید نهایی',
      message: 'با تأیید نهایی، پرونده کارآموزی در سامانه فعال می‌شود. آیا ادامه می‌دهید؟',
      icon: 'pi pi-check-circle',
      acceptLabel: 'بله، تأیید شود',
      rejectLabel: 'انصراف',
      accept: () => this.approvePlacementDetails()
    });
  }

  readonly revisionDialogVisible = signal(false);
  readonly revisionForm = this.formBuilder.group({
    comment: this.formBuilder.nonNullable.control('', [Validators.required, Validators.pattern(/\S/)])
  });

  openRevision(): void {
    if (this.saving() || this.internshipCase()?.status !== 'PENDING_UNIVERSITY_REVIEW') return;
    this.revisionForm.reset({ comment: '' });
    this.revisionDialogVisible.set(true);
  }

  requestRevision(): void {
    if (this.saving() || this.internshipCase()?.status !== 'PENDING_UNIVERSITY_REVIEW') return;
    if (this.revisionForm.invalid) {
      this.revisionForm.markAllAsTouched();
      return;
    }
    this.saving.set(true);
    this.internshipService.requestUniversityRevision(this.caseID, this.revisionForm.controls.comment.value.trim())
      .pipe(finalize(() => this.saving.set(false)))
      .subscribe({
        next: item => {
          this.internshipCase.set(item);
          this.syncReviewForm(item);
          this.revisionDialogVisible.set(false);
          this.messages.add({ severity: 'success', summary: 'درخواست اصلاح ثبت شد', detail: 'پرونده برای اصلاح اولویت‌ها به دانشجو بازگردانده شد.' });
        },
        error: (error: HttpErrorResponse) => this.showError(error)
      });
  }

  openCorrection(): void {
    if (!this.canReviewPlacement()) return;
    this.correctionForm.reset({ comment: '' });
    this.correctionDialogVisible.set(true);
  }

  requestCorrection(): void {
    if (!this.canReviewPlacement()) return;
    if (this.correctionForm.invalid) {
      this.correctionForm.markAllAsTouched();
      return;
    }
    this.saving.set(true);
    this.internshipService.requestPlacementCorrection(this.caseID, this.correctionForm.controls.comment.value.trim())
      .pipe(finalize(() => this.saving.set(false)))
      .subscribe({
        next: (item) => {
          this.internshipCase.set(item);
          this.syncReviewForm(item);
          this.correctionDialogVisible.set(false);
          this.messages.add({ severity: 'success', summary: 'درخواست اصلاح ثبت شد', detail: 'پرونده برای اصلاح اطلاعات محل کارآموزی به شرکت بازگردانده شد.' });
        },
        error: (error: HttpErrorResponse) => this.showError(error)
      });
  }

  private canReviewPlacement(): boolean {
    return !this.saving() && this.internshipCase()?.status === 'PENDING_FINAL_APPROVAL';
  }

  private approvePlacementDetails(): void {
    if (!this.canReviewPlacement()) return;
    this.saving.set(true);
    this.internshipService.approvePlacementDetails(this.caseID)
      .pipe(finalize(() => this.saving.set(false)))
      .subscribe({
        next: (item) => {
          this.internshipCase.set(item);
          this.syncReviewForm(item);
          this.messages.add({ severity: 'success', summary: 'تأیید نهایی شد', detail: 'پرونده کارآموزی با تأیید نهایی فعال شد.' });
        },
        error: (error: HttpErrorResponse) => this.showError(error)
      });
  }

  private approvePlacement(): void {
    if (this.saving() || this.reviewForm.invalid || this.internshipCase()?.status !== 'PENDING_UNIVERSITY_REVIEW') return;
    const value = this.reviewForm.getRawValue();
    this.saving.set(true);
    this.internshipService.approveUniversityPlacement(this.caseID, {
      preferenceId: value.preferenceId!,
      letterNumber: value.letterNumber.trim(),
      letterDate: value.letterDate
    })
      .pipe(finalize(() => this.saving.set(false)))
      .subscribe({
        next: (internshipCase) => {
          this.internshipCase.set(internshipCase);
          this.syncReviewForm(internshipCase);
          this.messages.add({
            severity: 'success',
            summary: 'تأیید شد',
            detail: 'محل کارآموزی و معرفی‌نامه ثبت شد؛ پرونده در انتظار اطلاعات شروع شرکت است.'
          });
        },
        error: (error: HttpErrorResponse) => this.showError(error)
      });
  }

  private cancelCase(): void {
    if (this.saving() || this.cancellationForm.invalid || this.internshipCase()?.status !== 'PENDING_UNIVERSITY_REVIEW') return;
    const comment = this.cancellationForm.controls.comment.value.trim();
    this.saving.set(true);
    this.internshipService.cancelUniversityReview(this.caseID, { comment })
      .pipe(finalize(() => this.saving.set(false)))
      .subscribe({
        next: (internshipCase) => {
          this.cancelDialogVisible.set(false);
          this.internshipCase.set(internshipCase);
          this.syncReviewForm(internshipCase);
          this.messages.add({ severity: 'success', summary: 'لغو شد', detail: 'پرونده کارآموزی لغو شد.' });
        },
        error: (error: HttpErrorResponse) => this.showError(error)
      });
  }

  private syncReviewForm(internshipCase: UniversityInternshipCase): void {
    this.reviewForm.reset({
      preferenceId: internshipCase.selectedPreferenceId ?? null,
      letterNumber: internshipCase.letterNumber ?? '',
      letterDate: internshipCase.letterDate?.slice(0, 10) ?? ''
    });
    if (internshipCase.status === 'PENDING_UNIVERSITY_REVIEW') {
      this.reviewForm.enable({ emitEvent: false });
    } else {
      this.reviewForm.disable({ emitEvent: false });
    }
  }

  private showError(error: HttpErrorResponse): void {
    this.messages.add({
      severity: 'error',
      summary: 'عملیات ناموفق',
      detail: userErrorMessage(error, 'انجام بررسی پرونده امکان‌پذیر نبود.')
    });
  }
}
