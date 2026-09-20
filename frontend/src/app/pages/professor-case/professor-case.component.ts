import { HttpErrorResponse } from '@angular/common/http';
import { Component, inject, signal } from '@angular/core';
import { FormBuilder, ReactiveFormsModule, Validators } from '@angular/forms';
import { ActivatedRoute, RouterLink } from '@angular/router';
import { ConfirmationService, MessageService } from 'primeng/api';
import { ButtonModule } from 'primeng/button';
import { CardModule } from 'primeng/card';
import { ConfirmDialogModule } from 'primeng/confirmdialog';
import { SelectModule } from 'primeng/select';
import { TableModule } from 'primeng/table';
import { TagModule } from 'primeng/tag';
import { TextareaModule } from 'primeng/textarea';

import {
  EvaluationRating,
  ProfessorCaseDetail,
  ProfessorCompanyEvaluation,
  ProfessorFinalResult,
  evaluationRatingLabels,
  internshipStatusLabels,
  professorFinalResultLabels
} from '../../internship/internship.models';
import { InternshipService } from '../../internship/internship.service';
import { JalaliDatePipe } from '../../shared/jalali-date/jalali-date.pipe';
import { userErrorMessage } from '../../shared/http-error-message';

type EvaluationField =
  | 'attendanceRating' | 'participationRating' | 'learningRating'
  | 'interestRating' | 'persistenceRating' | 'suggestionRating'
  | 'resourceUsageRating' | 'reportQualityRating' | 'projectPerformanceRating';

@Component({
  selector: 'app-professor-case',
  imports: [
    JalaliDatePipe, ReactiveFormsModule, RouterLink, ButtonModule, CardModule,
    ConfirmDialogModule, SelectModule, TableModule, TagModule, TextareaModule
  ],
  providers: [ConfirmationService],
  templateUrl: './professor-case.component.html',
  styleUrl: '../workflow-page.scss'
})
export class ProfessorCaseComponent {
  private readonly route = inject(ActivatedRoute);
  private readonly formBuilder = inject(FormBuilder);
  private readonly internshipService = inject(InternshipService);
  private readonly messages = inject(MessageService);
  private readonly confirmation = inject(ConfirmationService);
  private readonly caseID = Number(this.route.snapshot.paramMap.get('id'));

  readonly internshipCase = signal<ProfessorCaseDetail | null>(null);
  readonly loading = signal(true);
  readonly submitting = signal(false);
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

  confirmCompletion(): void {
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
      header: 'تکمیل پرونده کارآموزی',
      message: 'با ثبت ارزیابی نهایی، پرونده کارآموزی تکمیل خواهد شد و در نسخه فعلی امکان ویرایش مجدد نتیجه وجود ندارد. آیا ادامه می‌دهید؟',
      acceptLabel: 'بله، ثبت شود',
      rejectLabel: 'انصراف',
      icon: 'pi pi-exclamation-triangle',
      accept: () => this.completeCase()
    });
  }

  downloadFinalReport(): void {
    const file = this.internshipCase()?.finalReport;
    if (!file) return;
    this.internshipService.downloadFile(file.id).subscribe({
      next: (blob) => {
        const url = URL.createObjectURL(blob);
        const anchor = document.createElement('a');
        anchor.href = url;
        anchor.download = file.originalName;
        anchor.click();
        URL.revokeObjectURL(url);
      },
      error: () => this.messages.add({ severity: 'error', summary: 'خطا', detail: 'دانلود گزارش نهایی ناموفق بود.' })
    });
  }

  private loadCase(): void {
    this.loading.set(true);
    this.internshipService.getProfessorCase(this.caseID).subscribe({
      next: (item) => {
        this.internshipCase.set(item);
        this.loading.set(false);
        if (item.canProfessorComplete) this.evaluationForm.enable();
        else this.evaluationForm.disable();
      },
      error: (error: HttpErrorResponse) => {
        this.loading.set(false);
        this.showError(error);
      }
    });
  }

  private completeCase(): void {
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
    if (error.status === 409) detail = userErrorMessage(error, 'مدارک پرونده کامل نیست یا پرونده قبلاً تکمیل شده است.');
    this.messages.add({ severity: 'error', summary: 'خطا', detail });
  }
}
