import { HttpErrorResponse } from '@angular/common/http';
import { Component, computed, inject, signal } from '@angular/core';
import { FormBuilder, ReactiveFormsModule, Validators } from '@angular/forms';
import { ActivatedRoute, RouterLink } from '@angular/router';
import { ConfirmationService, MessageService } from 'primeng/api';
import { ButtonModule } from 'primeng/button';
import { CardModule } from 'primeng/card';
import { ConfirmDialogModule } from 'primeng/confirmdialog';
import { DialogModule } from 'primeng/dialog';
import { InputNumberModule } from 'primeng/inputnumber';
import { InputTextModule } from 'primeng/inputtext';
import { SelectModule } from 'primeng/select';
import { TagModule } from 'primeng/tag';
import { TextareaModule } from 'primeng/textarea';

import {
  InternshipCase,
  InternshipCaseStatus,
  InternshipPreference,
  CompanyEvaluation,
  EvaluationRating,
  WeeklyReport,
  evaluationRatingLabels,
  internshipStatusLabels,
  professorFinalResultLabels
} from '../../internship/internship.models';
import { InternshipService } from '../../internship/internship.service';
import { JalaliDatePickerComponent } from '../../shared/jalali-date/jalali-date-picker.component';
import { JalaliDatePipe } from '../../shared/jalali-date/jalali-date.pipe';
import { PersianDigitsPipe } from '../../shared/persian-digits.pipe';

@Component({
  selector: 'app-company-case',
  imports: [JalaliDatePipe, PersianDigitsPipe, JalaliDatePickerComponent, ReactiveFormsModule, RouterLink, ButtonModule, CardModule, ConfirmDialogModule, DialogModule, InputNumberModule, InputTextModule, SelectModule, TagModule, TextareaModule],
  providers: [ConfirmationService],
  templateUrl: './company-case.component.html',
  styleUrl: '../workflow-page.scss'
})
export class CompanyCaseComponent {
  private readonly route = inject(ActivatedRoute);
  private readonly formBuilder = inject(FormBuilder);
  private readonly internshipService = inject(InternshipService);
  private readonly messages = inject(MessageService);
  private readonly confirmation = inject(ConfirmationService);
  private readonly caseID = Number(this.route.snapshot.paramMap.get('id'));

  readonly internshipCase = signal<InternshipCase | null>(null);
  readonly loading = signal(true);
  readonly saving = signal(false);
  readonly reports = signal<WeeklyReport[]>([]);
  readonly evaluation = signal<CompanyEvaluation | null>(null);
  readonly reportsLoading = signal(false);
  readonly evaluationLoading = signal(false);
  readonly confirmingReport = signal(false);
  readonly evaluationSaving = signal(false);
  readonly downloading = signal(false);
  readonly reportDialogVisible = signal(false);
  readonly selectedReport = signal<WeeklyReport | null>(null);
  readonly confirmedReportCount = computed(() => this.reports().filter((report) => report.isConfirmed).length);
  readonly canSubmitEvaluation = computed(() =>
    this.internshipCase()?.status === 'ACTIVE' &&
    this.reports().length === 8 && this.confirmedReportCount() === 8 && this.evaluation() === null
  );

  readonly ratingOptions = (Object.entries(evaluationRatingLabels) as [EvaluationRating, string][])
    .map(([value, label]) => ({ value, label }));
  readonly criteria: { key: RatingField; label: string }[] = [
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

  readonly confirmationForm = this.formBuilder.group({
    internshipSubject: this.formBuilder.nonNullable.control('', Validators.required),
    startDate: this.formBuilder.nonNullable.control('', Validators.required),
    workplaceAddress: this.formBuilder.nonNullable.control('', Validators.required),
    workplacePhone: this.formBuilder.nonNullable.control('', Validators.required)
  });

  readonly reportConfirmationForm = this.formBuilder.group({
    comment: this.formBuilder.nonNullable.control('')
  });

  readonly evaluationForm = this.formBuilder.group({
    attendanceRating: this.formBuilder.control<EvaluationRating | null>(null, Validators.required),
    participationRating: this.formBuilder.control<EvaluationRating | null>(null, Validators.required),
    learningRating: this.formBuilder.control<EvaluationRating | null>(null, Validators.required),
    interestRating: this.formBuilder.control<EvaluationRating | null>(null, Validators.required),
    persistenceRating: this.formBuilder.control<EvaluationRating | null>(null, Validators.required),
    suggestionRating: this.formBuilder.control<EvaluationRating | null>(null, Validators.required),
    resourceUsageRating: this.formBuilder.control<EvaluationRating | null>(null, Validators.required),
    reportQualityRating: this.formBuilder.control<EvaluationRating | null>(null, Validators.required),
    projectPerformanceRating: this.formBuilder.control<EvaluationRating | null>(null, Validators.required),
    leaveDays: this.formBuilder.nonNullable.control(0, [Validators.required, Validators.min(0)]),
    absenceDays: this.formBuilder.nonNullable.control(0, [Validators.required, Validators.min(0)]),
    suggestions: this.formBuilder.nonNullable.control('')
  });

  constructor() {
    this.internshipService.getCompanyCase(this.caseID).subscribe({
      next: (internshipCase) => {
        this.internshipCase.set(internshipCase);
        this.loading.set(false);
        if (internshipCase.status === 'ACTIVE' || internshipCase.status === 'COMPLETED') this.loadReportingData();
      },
      error: (error: HttpErrorResponse) => {
        this.loading.set(false);
        this.showError(error);
      }
    });
  }

  ratingLabel(rating: EvaluationRating): string {
    return evaluationRatingLabels[rating];
  }

  finalResultLabel(item: InternshipCase): string {
    return item.finalResult ? professorFinalResultLabels[item.finalResult] : '—';
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

  openReportConfirmation(report: WeeklyReport): void {
    this.selectedReport.set(report);
    this.reportConfirmationForm.reset({ comment: '' });
    this.reportDialogVisible.set(true);
  }

  confirmWeeklyReport(): void {
    const report = this.selectedReport();
    if (!report) return;
    this.confirmingReport.set(true);
    this.internshipService.confirmWeeklyReport(this.caseID, report.id, this.reportConfirmationForm.getRawValue().comment).subscribe({
      next: (confirmed) => {
        this.reports.update((reports) => reports.map((item) => item.id === confirmed.id ? confirmed : item));
        this.confirmingReport.set(false);
        this.reportDialogVisible.set(false);
        this.messages.add({ severity: 'success', summary: 'تأیید شد', detail: 'گزارش با موفقیت تأیید شد.' });
      },
      error: (error: HttpErrorResponse) => { this.confirmingReport.set(false); this.showError(error); }
    });
  }

  confirmEvaluationSubmission(): void {
    if (!this.canSubmitEvaluation()) {
      this.messages.add({ severity: 'warn', summary: 'ارزیابی غیرفعال است', detail: 'ابتدا هر ۸ گزارش هفتگی باید ثبت و تأیید شوند.' });
      return;
    }
    if (this.evaluationForm.invalid) {
      this.evaluationForm.markAllAsTouched();
      this.messages.add({ severity: 'warn', summary: 'اطلاعات ناقص', detail: 'تمام معیارهای ارزیابی را تکمیل کنید.' });
      return;
    }
    this.confirmation.confirm({
      header: 'ثبت ارزیابی نهایی',
      message: 'پس از ثبت ارزیابی نهایی، امکان ویرایش آن در نسخه فعلی وجود ندارد. آیا ادامه می‌دهید؟',
      acceptLabel: 'بله، ثبت شود',
      rejectLabel: 'انصراف',
      accept: () => this.submitEvaluation()
    });
  }

  private loadReportingData(): void {
    this.reportsLoading.set(true);
    this.evaluationLoading.set(true);
    this.internshipService.listCompanyWeeklyReports(this.caseID).subscribe({
      next: (reports) => { this.reports.set(reports); this.reportsLoading.set(false); },
      error: (error: HttpErrorResponse) => { this.reportsLoading.set(false); this.showError(error); }
    });
    this.internshipService.getCompanyEvaluation(this.caseID).subscribe({
      next: (evaluation) => { this.evaluation.set(evaluation); this.evaluationLoading.set(false); },
      error: (error: HttpErrorResponse) => {
        this.evaluationLoading.set(false);
        if (error.status !== 404) this.showError(error);
      }
    });
  }

  private submitEvaluation(): void {
    const value = this.evaluationForm.getRawValue();
    this.evaluationSaving.set(true);
    this.internshipService.createCompanyEvaluation(this.caseID, {
      attendanceRating: value.attendanceRating!, participationRating: value.participationRating!,
      learningRating: value.learningRating!, interestRating: value.interestRating!,
      persistenceRating: value.persistenceRating!, suggestionRating: value.suggestionRating!,
      resourceUsageRating: value.resourceUsageRating!, reportQualityRating: value.reportQualityRating!,
      projectPerformanceRating: value.projectPerformanceRating!, leaveDays: value.leaveDays,
      absenceDays: value.absenceDays, suggestions: value.suggestions.trim() || undefined
    }).subscribe({
      next: (evaluation) => {
        this.evaluation.set(evaluation);
        this.evaluationSaving.set(false);
        this.messages.add({ severity: 'success', summary: 'ثبت شد', detail: 'ارزیابی نهایی با موفقیت ثبت شد.' });
      },
      error: (error: HttpErrorResponse) => { this.evaluationSaving.set(false); this.showError(error); }
    });
  }

  statusLabel(status: InternshipCaseStatus): string {
    return internshipStatusLabels[status];
  }

  preferenceName(preference?: InternshipPreference): string {
    return preference?.company?.name ?? preference?.proposedCompanyName ?? '—';
  }

  confirmSubmission(): void {
    if (this.confirmationForm.invalid) {
      this.confirmationForm.markAllAsTouched();
      this.messages.add({ severity: 'warn', summary: 'اطلاعات ناقص', detail: 'تمام اطلاعات محل کارآموزی را وارد کنید.' });
      return;
    }
    this.confirmation.confirm({
      header: 'تأیید پذیرش دانشجو',
      message: 'با تأیید این فرم، پذیرش دانشجو برای دوره کارآموزی تأیید می‌شود. آیا ادامه می‌دهید؟',
      acceptLabel: 'بله، تأیید شود',
      rejectLabel: 'انصراف',
      accept: () => this.submit()
    });
  }

  private submit(): void {
    const value = this.confirmationForm.getRawValue();
    this.saving.set(true);
    this.internshipService.confirmCompanyCase(this.caseID, {
      internshipSubject: value.internshipSubject.trim(),
      startDate: value.startDate,
      workplaceAddress: value.workplaceAddress.trim(),
      workplacePhone: value.workplacePhone.trim()
    }).subscribe({
      next: (internshipCase) => {
        this.internshipCase.set(internshipCase);
        this.confirmationForm.disable();
        this.saving.set(false);
        this.messages.add({ severity: 'success', summary: 'تأیید شد', detail: 'پذیرش دانشجو با موفقیت تأیید شد.' });
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
    if (error.status === 403) detail = 'این پرونده به حساب شما اختصاص نیافته است.';
    if (error.status === 409) detail = 'این پرونده در وضعیت قابل تأیید نیست.';
    this.messages.add({ severity: 'error', summary: 'خطا', detail });
  }
}

type RatingField =
  | 'attendanceRating' | 'participationRating' | 'learningRating'
  | 'interestRating' | 'persistenceRating' | 'suggestionRating'
  | 'resourceUsageRating' | 'reportQualityRating' | 'projectPerformanceRating';
