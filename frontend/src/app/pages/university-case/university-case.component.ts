import { HttpErrorResponse } from '@angular/common/http';
import { Component, inject, signal } from '@angular/core';
import { FormBuilder, ReactiveFormsModule, Validators } from '@angular/forms';
import { ActivatedRoute, RouterLink } from '@angular/router';
import { forkJoin } from 'rxjs';
import { ConfirmationService, MessageService } from 'primeng/api';
import { ButtonModule } from 'primeng/button';
import { CardModule } from 'primeng/card';
import { ConfirmDialogModule } from 'primeng/confirmdialog';
import { InputTextModule } from 'primeng/inputtext';
import { SelectModule } from 'primeng/select';
import { TagModule } from 'primeng/tag';

import { User } from '../../auth/auth.models';
import {
  EvaluationRating,
  InternshipCase,
  InternshipCaseStatus,
  InternshipPreference,
  ProfessorFinalResult,
  evaluationRatingLabels,
  internshipStatusLabels,
  professorFinalResultLabels
} from '../../internship/internship.models';
import { InternshipService } from '../../internship/internship.service';
import { JalaliDatePickerComponent } from '../../shared/jalali-date/jalali-date-picker.component';
import { JalaliDatePipe } from '../../shared/jalali-date/jalali-date.pipe';
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
    InputTextModule,
    SelectModule,
    TagModule
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
  readonly supervisors = signal<User[]>([]);
  readonly loading = signal(true);
  readonly saving = signal(false);
  readonly downloading = signal(false);
  readonly criteria: { key: EvaluationField; label: string }[] = [
    { key: 'attendanceRating', label: 'حضور و نظم' },
    { key: 'participationRating', label: 'مشارکت در فعالیت‌ها' },
    { key: 'learningRating', label: 'استعداد و توانایی یادگیری' },
    { key: 'interestRating', label: 'علاقه به یادگیری مطالب علمی و فنی' },
    { key: 'persistenceRating', label: 'پیگیری و پشتکار' },
    { key: 'suggestionRating', label: 'ارزش پیشنهادهای ارائه‌شده' },
    { key: 'resourceUsageRating', label: 'استفاده از امکانات موجود برای افزایش توانایی' },
    { key: 'reportQualityRating', label: 'کیفیت گزارش‌های کارآموزی' },
    { key: 'projectPerformanceRating', label: 'عملکرد در پروژه یا فعالیت محوله' }
  ];

  readonly reviewForm = this.formBuilder.group({
    preferenceId: this.formBuilder.control<number | null>(null, Validators.required),
    companySupervisorId: this.formBuilder.control<number | null>(null, Validators.required),
    letterNumber: this.formBuilder.nonNullable.control('', Validators.required),
    letterDate: this.formBuilder.nonNullable.control('', Validators.required)
  });

  constructor() {
    forkJoin({
      internshipCase: this.internshipService.getUniversityCase(this.caseID),
      supervisors: this.internshipService.listCompanySupervisors()
    }).subscribe({
      next: ({ internshipCase, supervisors }) => {
        this.internshipCase.set(internshipCase);
        this.supervisors.set(supervisors);
        this.syncReviewForm(internshipCase);
        this.loading.set(false);
      },
      error: (error: HttpErrorResponse) => {
        this.loading.set(false);
        this.showError(error);
      }
    });
  }

  statusLabel(status: InternshipCaseStatus): string {
    return internshipStatusLabels[status];
  }

  preferenceName(preference: InternshipPreference): string {
    return preference.company?.name ?? preference.proposedCompanyName ?? '—';
  }

  ratingLabel(rating: EvaluationRating): string {
    return evaluationRatingLabels[rating];
  }

  finalResultLabel(result?: ProfessorFinalResult): string {
    return result ? professorFinalResultLabels[result] : '—';
  }

  downloadFinalReport(): void {
    const report = this.internshipCase()?.finalReport;
    if (!report) return;
    this.downloading.set(true);
    this.internshipService.downloadFile(report.id).subscribe({
      next: (blob) => {
        const url = URL.createObjectURL(blob);
        const anchor = document.createElement('a');
        anchor.href = url;
        anchor.download = report.originalName;
        anchor.click();
        URL.revokeObjectURL(url);
        this.downloading.set(false);
      },
      error: () => {
        this.downloading.set(false);
        this.messages.add({ severity: 'error', summary: 'خطا', detail: 'دانلود گزارش نهایی ناموفق بود.' });
      }
    });
  }

  selectPreference(preferenceID: number): void {
    this.reviewForm.controls.preferenceId.setValue(preferenceID);
  }

  confirmSendToCompany(): void {
    if (this.reviewForm.invalid) {
      this.reviewForm.markAllAsTouched();
      this.messages.add({ severity: 'warn', summary: 'اطلاعات ناقص', detail: 'محل کارآموزی، سرپرست شرکت و اطلاعات نامه را وارد کنید.' });
      return;
    }
    this.confirmation.confirm({
      header: 'ارسال پرونده به شرکت',
      message: 'پس از ارسال، اطلاعات نامه، محل انتخابی و سرپرست شرکت در این مرحله قابل ویرایش نیست. آیا ادامه می‌دهید؟',
      acceptLabel: 'بله، ارسال شود',
      rejectLabel: 'انصراف',
      icon: 'pi pi-exclamation-triangle',
      accept: () => this.sendToCompany()
    });
  }

  private sendToCompany(): void {
    const value = this.reviewForm.getRawValue();
    this.saving.set(true);
    this.internshipService.sendToCompany(this.caseID, {
      preferenceId: value.preferenceId!,
      companySupervisorId: value.companySupervisorId!,
      letterNumber: value.letterNumber.trim(),
      letterDate: value.letterDate
    }).subscribe({
      next: (internshipCase) => {
        this.internshipCase.set(internshipCase);
        this.syncReviewForm(internshipCase);
        this.saving.set(false);
        this.messages.add({ severity: 'success', summary: 'ارسال شد', detail: 'پرونده برای تأیید شرکت ارسال شد.' });
      },
      error: (error: HttpErrorResponse) => {
        this.saving.set(false);
        this.showError(error);
      }
    });
  }

  confirmApprove(): void {
    this.confirmation.confirm({
      header: 'تأیید نهایی آموزش',
      message: 'پذیرش شرکت و اطلاعات محل کارآموزی بررسی شد. آیا پرونده توسط آموزش تأیید شود؟',
      acceptLabel: 'تأیید نهایی',
      rejectLabel: 'انصراف',
      accept: () => this.approve()
    });
  }

  confirmActivate(): void {
    this.confirmation.confirm({
      header: 'فعال‌سازی کارآموزی',
      message: 'تمام تأییدهای لازم انجام شده است. آیا کارآموزی دانشجو فعال شود؟',
      acceptLabel: 'فعال شود',
      rejectLabel: 'انصراف',
      accept: () => this.activate()
    });
  }

  private approve(): void {
    this.runAction(this.internshipService.approveUniversityCase(this.caseID), 'پرونده توسط آموزش تأیید شد.');
  }

  private activate(): void {
    this.runAction(this.internshipService.activateUniversityCase(this.caseID), 'کارآموزی دانشجو با موفقیت فعال شد.');
  }

  private runAction(request: ReturnType<InternshipService['approveUniversityCase']>, detail: string): void {
    this.saving.set(true);
    request.subscribe({
      next: (internshipCase) => {
        this.internshipCase.set(internshipCase);
        this.syncReviewForm(internshipCase);
        this.saving.set(false);
        this.messages.add({ severity: 'success', summary: 'انجام شد', detail });
      },
      error: (error: HttpErrorResponse) => {
        this.saving.set(false);
        this.showError(error);
      }
    });
  }

  private showError(error: HttpErrorResponse): void {
    let detail = 'انجام عملیات ناموفق بود.';
    if (error.status === 0) detail = 'ارتباط با سرور برقرار نشد.';
    if (error.status === 409) detail = 'این عملیات با وضعیت فعلی پرونده سازگار نیست.';
    this.messages.add({ severity: 'error', summary: 'خطا', detail });
  }

  private syncReviewForm(internshipCase: InternshipCase): void {
    this.reviewForm.reset({
      preferenceId: internshipCase.selectedPreferenceId ?? null,
      companySupervisorId: internshipCase.companySupervisorId ?? null,
      letterNumber: internshipCase.letterNumber ?? '',
      letterDate: internshipCase.letterDate?.slice(0, 10) ?? ''
    });
    if (internshipCase.status === 'PENDING_UNIVERSITY_REVIEW') this.reviewForm.enable({ emitEvent: false });
    else this.reviewForm.disable({ emitEvent: false });
  }
}

type EvaluationField =
  | 'attendanceRating' | 'participationRating' | 'learningRating'
  | 'interestRating' | 'persistenceRating' | 'suggestionRating'
  | 'resourceUsageRating' | 'reportQualityRating' | 'projectPerformanceRating';
