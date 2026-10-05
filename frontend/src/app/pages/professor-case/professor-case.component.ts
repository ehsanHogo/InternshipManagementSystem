import { HttpErrorResponse } from '@angular/common/http';
import { Component, inject, signal } from '@angular/core';
import { FormBuilder, ReactiveFormsModule, Validators } from '@angular/forms';
import { ActivatedRoute, RouterLink } from '@angular/router';
import { ConfirmationService, MessageService } from 'primeng/api';
import { ButtonModule } from 'primeng/button';
import { CardModule } from 'primeng/card';
import { ConfirmDialogModule } from 'primeng/confirmdialog';
import { DialogModule } from 'primeng/dialog';
import { SelectModule } from 'primeng/select';
import { TableModule } from 'primeng/table';
import { TagModule } from 'primeng/tag';
import { TextareaModule } from 'primeng/textarea';

import {
  WeeklyReport,
  finalReportStatusLabels,
  finalReportStatusSeverity,
  EvaluationRating,
  internshipStatusSeverity,
  ProfessorCaseDetail,
  ProfessorCompanyEvaluation,
  ProfessorFinalResult,
  evaluationRatingLabels,
  internshipStatusLabels,
  professorFinalResultLabels
} from '../../internship/internship.models';
import { WeeklyReportReviewComponent } from '../../shared/weekly-report-review.component';
import { InternshipService } from '../../internship/internship.service';
import { internshipTermLabel } from '../../internship/internship-term.models';
import { JalaliDatePipe } from '../../shared/jalali-date/jalali-date.pipe';
import { userErrorMessage } from '../../shared/http-error-message';
import { PersianDigitsPipe } from '../../shared/persian-digits.pipe';

type EvaluationField =
  | 'attendanceRating' | 'participationRating' | 'learningRating'
  | 'interestRating' | 'persistenceRating' | 'suggestionRating'
  | 'resourceUsageRating' | 'reportQualityRating' | 'projectPerformanceRating';

@Component({
  selector: 'app-professor-case',
  imports: [
    WeeklyReportReviewComponent, DialogModule, JalaliDatePipe, PersianDigitsPipe, ReactiveFormsModule, RouterLink, ButtonModule, CardModule,
    ConfirmDialogModule, SelectModule, TableModule, TagModule, TextareaModule
  ],
  providers: [ConfirmationService],
  templateUrl: './professor-case.component.html',
  styleUrl: '../workflow-page.scss'
})
export class ProfessorCaseComponent {
  readonly termLabel = internshipTermLabel;
  private readonly route = inject(ActivatedRoute);
  private readonly formBuilder = inject(FormBuilder);
  private readonly internshipService = inject(InternshipService);
  private readonly messages = inject(MessageService);
  private readonly confirmation = inject(ConfirmationService);
  private readonly caseID = Number(this.route.snapshot.paramMap.get('id'));

  readonly finalReportStatusLabels = finalReportStatusLabels;
  readonly finalReportStatusSeverity = finalReportStatusSeverity;
  readonly internshipStatusSeverity = internshipStatusSeverity;
  readonly finalReviewAction = signal<'approve' | 'request-revision' | null>(null);
  readonly finalReviewForm = this.formBuilder.group({ comment: this.formBuilder.nonNullable.control('') });
  readonly finalReviewing = signal(false);
  readonly internshipCase = signal<ProfessorCaseDetail | null>(null);
  readonly loading = signal(true);
  readonly loadFailed = signal(false);
  readonly submitting = signal(false);
  readonly downloading = signal(false);
  readonly selectedReport = signal<WeeklyReport | null>(null);
  readonly reviewing = signal(false);
  readonly reviewForm = this.formBuilder.group({ comment: this.formBuilder.nonNullable.control('') });
  readonly resultOptions: { label: string; value: ProfessorFinalResult }[] = [
    { label: 'عالی', value: 'EXCELLENT' },
    { label: 'خوب', value: 'GOOD' },
    { label: 'مردود', value: 'FAILED' }
  ];
  readonly criteria: { key: EvaluationField; label: string }[] = [
    { key: 'attendanceRating', label: 'حضور و وقت‌شناسی' },
    { key: 'participationRating', label: 'مشارکت و همکاری' },
    { key: 'learningRating', label: 'یادگیری و پیشرفت' },
    { key: 'interestRating', label: 'علاقه‌مندی و انگیزه' },
    { key: 'persistenceRating', label: 'پشتکار و مسئولیت‌پذیری' },
    { key: 'suggestionRating', label: 'ارائه پیشنهادهای سازنده' },
    { key: 'resourceUsageRating', label: 'استفاده از منابع' },
    { key: 'reportQualityRating', label: 'کیفیت گزارش‌دهی' },
    { key: 'projectPerformanceRating', label: 'عملکرد در پروژه' }
  ];
  readonly evaluationForm = this.formBuilder.group({
    result: this.formBuilder.control<ProfessorFinalResult | null>(null, Validators.required),
    comment: this.formBuilder.nonNullable.control('', Validators.maxLength(2000))
  });

  constructor() {
    this.loadCase();
  }

  statusLabel(item: ProfessorCaseDetail): string {
    return internshipStatusLabels[item.internship.status];
  }

  resultLabel(result?: ProfessorFinalResult): string {
    return result ? professorFinalResultLabels[result] : '—';
  }

  ratingLabel(evaluation: ProfessorCompanyEvaluation, key: EvaluationField): string {
    return evaluationRatingLabels[evaluation[key] as EvaluationRating];
  }

  canReviewReport(report: WeeklyReport): boolean {
    return this.internshipCase()?.internship.status === 'ACTIVE' && report.status === 'SUBMITTED' && report.professorReviewStatus === 'PENDING';
  }

  openReportReview(report: WeeklyReport): void {
    if (!this.canReviewReport(report)) return;
    this.reviewForm.reset({ comment: '' });
    this.selectedReport.set(report);
  }

  reviewReport(action: 'approve' | 'request-revision'): void {
    const report = this.selectedReport();
    if (!report || !this.canReviewReport(report) || this.reviewing()) return;
    const comment = this.reviewForm.getRawValue().comment.trim();
    if (action === 'request-revision' && !comment) {
      this.messages.add({ severity: 'warn', summary: 'نظر الزامی', detail: 'برای درخواست اصلاح، توضیح غیرخالی وارد کنید.' });
      return;
    }
    this.reviewing.set(true);
    this.internshipService.reviewProfessorWeeklyReport(this.caseID, report.id, action, comment || undefined).subscribe({
      next: () => {
        this.reviewing.set(false);
        this.selectedReport.set(null);
        this.loadCase();
        this.messages.add({ severity: 'success', summary: 'ثبت شد', detail: 'نظر شما درباره گزارش ثبت شد.' });
      },
      error: (error: HttpErrorResponse) => { this.reviewing.set(false); this.showError(error); }
    });
  }

  canReviewFinalReport(): boolean {
    const item = this.internshipCase();
    return item?.internship.status === 'ACTIVE' && item.finalReport?.status === 'SUBMITTED';
  }

  openFinalReview(action: 'approve' | 'request-revision'): void {
    if (!this.canReviewFinalReport()) return;
    this.finalReviewForm.reset({ comment: '' });
    this.finalReviewAction.set(action);
  }

  submitFinalReview(): void {
    const action = this.finalReviewAction();
    if (!action || !this.canReviewFinalReport() || this.finalReviewing()) return;
    const comment = this.finalReviewForm.getRawValue().comment.trim();
    if (action === 'request-revision' && !comment) {
      this.messages.add({ severity: 'warn', summary: 'نظر الزامی', detail: 'توضیحات اصلاحات مورد نیاز نباید خالی باشد.' });
      return;
    }
    this.finalReviewing.set(true);
    this.internshipService.reviewFinalReport(this.caseID, action, comment || undefined).subscribe({
      next: () => {
        this.finalReviewing.set(false);
        this.finalReviewAction.set(null);
        this.loadCase();
        this.messages.add({ severity: 'success', summary: 'ثبت شد', detail: 'بررسی گزارش نهایی ثبت شد.' });
      },
      error: (error: HttpErrorResponse) => { this.finalReviewing.set(false); this.showError(error); }
    });
  }

  confirmCompletion(): void {
    if (this.submitting() || this.internshipCase()?.internship.status !== 'ACTIVE') return;
    const item = this.internshipCase();
    if (!item?.canProfessorComplete || this.evaluationForm.invalid) {
      this.evaluationForm.markAllAsTouched();
      this.messages.add({
        severity: 'warn', summary: 'ارزیابی غیرفعال است',
        detail: item?.canProfessorComplete ? 'نتیجه نهایی را انتخاب کنید.' : 'ابتدا باید تمام مدارک پرونده تکمیل شوند.'
      });
      return;
    }
    this.confirmation.confirm({
      header: 'ثبت ارزیابی نهایی کارآموزی',
      message: 'با ثبت ارزیابی نهایی، وضعیت پرونده بر اساس نتیجه به قبول شده یا مردود تغییر خواهد کرد و در نسخه فعلی امکان ویرایش مجدد نتیجه وجود ندارد. آیا ادامه می‌دهید؟',
      acceptLabel: 'بله، ثبت شود',
      rejectLabel: 'انصراف',
      icon: 'pi pi-exclamation-triangle',
      accept: () => this.completeCase()
    });
  }

  downloadFinalReport(): void {
    const file = this.internshipCase()?.finalReport?.currentFile;
    if (!file || this.downloading()) return;
    this.downloading.set(true);
    this.internshipService.downloadFile(file.id).subscribe({
      next: (blob) => {
        const url = URL.createObjectURL(blob);
        const anchor = document.createElement('a');
        anchor.href = url;
        anchor.download = file.originalName;
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

  loadCase(): void {
    this.loading.set(true);
    this.loadFailed.set(false);
    this.internshipService.getProfessorCase(this.caseID).subscribe({
      next: (item) => {
        this.internshipCase.set(item);
        this.loading.set(false);
        if (item.canProfessorComplete) this.evaluationForm.enable();
        else this.evaluationForm.disable();
      },
      error: (error: HttpErrorResponse) => {
        this.loading.set(false);
        this.loadFailed.set(true);
        this.internshipCase.set(null);
        this.showError(error);
      }
    });
  }

  private completeCase(): void {
    if (this.submitting() || this.evaluationForm.invalid || this.internshipCase()?.internship.status !== 'ACTIVE' || !this.internshipCase()?.canProfessorComplete) return;
    const value = this.evaluationForm.getRawValue();
    if (!value.result) return;
    this.submitting.set(true);
    this.internshipService.completeProfessorCase(this.caseID, {
      result: value.result,
      comment: value.comment.trim() || undefined
    }).subscribe({
      next: (item) => {
        this.internshipCase.set(item);
        this.evaluationForm.disable();
        this.submitting.set(false);
        this.messages.add({ severity: 'success', summary: 'ثبت شد', detail: 'ارزیابی استاد و نتیجه نهایی با موفقیت ثبت شد.' });
      },
      error: (error: HttpErrorResponse) => {
        this.submitting.set(false);
        this.showError(error);
      }
    });
  }

  private showError(error: HttpErrorResponse): void {
    let detail = 'انجام عملیات ناموفق بود.';
    if (error.status === 0) detail = 'ارتباط با سرور برقرار نشد.';
    if (error.status === 403) detail = 'این پرونده به شما اختصاص نیافته است.';
    if (error.status === 400 || error.status === 409) detail = userErrorMessage(error, 'مدارک پرونده کامل نیست یا پرونده قبلاً ارزیابی شده است.');
    this.messages.add({ severity: 'error', summary: 'خطا', detail });
  }
}
