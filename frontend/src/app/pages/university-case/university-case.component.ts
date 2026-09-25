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
  InternshipCase,
  InternshipCaseStatus,
  InternshipPreference,
  internshipStatusLabels
} from '../../internship/internship.models';
import { InternshipService } from '../../internship/internship.service';
import { JalaliDatePickerComponent } from '../../shared/jalali-date/jalali-date-picker.component';
import { JalaliDatePipe } from '../../shared/jalali-date/jalali-date.pipe';
import { userErrorMessage } from '../../shared/http-error-message';
import { PersianDigitsPipe } from '../../shared/persian-digits.pipe';

@Component({
  selector: 'app-university-case',
  imports: [
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
  private readonly route = inject(ActivatedRoute);
  private readonly formBuilder = inject(FormBuilder);
  private readonly internshipService = inject(InternshipService);
  private readonly messages = inject(MessageService);
  private readonly confirmation = inject(ConfirmationService);
  private readonly caseID = Number(this.route.snapshot.paramMap.get('id'));

  readonly internshipCase = signal<InternshipCase | null>(null);
  readonly loading = signal(true);
  readonly saving = signal(false);
  readonly cancelDialogVisible = signal(false);

  readonly reviewForm = this.formBuilder.group({
    preferenceId: this.formBuilder.control<number | null>(null, Validators.required),
    letterNumber: this.formBuilder.nonNullable.control('', [Validators.required, Validators.pattern(/\S/)]),
    letterDate: this.formBuilder.nonNullable.control('', Validators.required)
  });

  readonly cancellationForm = this.formBuilder.group({
    comment: this.formBuilder.nonNullable.control('', [Validators.required, Validators.pattern(/\S/)])
  });

  constructor() {
    this.loadCase();
  }

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
    this.cancellationForm.reset({ comment: '' });
    this.cancelDialogVisible.set(true);
  }

  confirmCancellation(): void {
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

  private loadCase(): void {
    this.loading.set(true);
    this.internshipService.getUniversityCase(this.caseID)
      .pipe(finalize(() => this.loading.set(false)))
      .subscribe({
        next: (internshipCase) => {
          this.internshipCase.set(internshipCase);
          this.syncReviewForm(internshipCase);
        },
        error: (error: HttpErrorResponse) => this.showError(error)
      });
  }

  private approvePlacement(): void {
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

  private syncReviewForm(internshipCase: InternshipCase): void {
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
