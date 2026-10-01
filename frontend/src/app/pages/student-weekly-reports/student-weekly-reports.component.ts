import { HttpErrorResponse } from '@angular/common/http';
import { Component, inject, signal } from '@angular/core';
import { FormBuilder, ReactiveFormsModule, Validators } from '@angular/forms';
import { ConfirmationService, MessageService } from 'primeng/api';
import { ButtonModule } from 'primeng/button';
import { CardModule } from 'primeng/card';
import { ConfirmDialogModule } from 'primeng/confirmdialog';
import { DialogModule } from 'primeng/dialog';
import { InputNumberModule } from 'primeng/inputnumber';
import { TextareaModule } from 'primeng/textarea';

import { WeeklyReportReviewComponent } from '../../shared/weekly-report-review.component';
import { userErrorMessage } from '../../shared/http-error-message';
import { InternshipCase, WeeklyReport } from '../../internship/internship.models';
import { InternshipService } from '../../internship/internship.service';
import { JalaliDatePickerComponent } from '../../shared/jalali-date/jalali-date-picker.component';
import { JalaliDatePipe } from '../../shared/jalali-date/jalali-date.pipe';
import { PersianDigitsPipe } from '../../shared/persian-digits.pipe';

@Component({
  selector: 'app-student-weekly-reports',
  providers: [ConfirmationService],
  imports: [WeeklyReportReviewComponent, ConfirmDialogModule, ReactiveFormsModule, JalaliDatePickerComponent, JalaliDatePipe, PersianDigitsPipe, ButtonModule, CardModule, DialogModule, InputNumberModule, TextareaModule],
  templateUrl: './student-weekly-reports.component.html',
  styleUrls: ['../workflow-page.scss', './student-weekly-reports.component.scss']
})
export class StudentWeeklyReportsComponent {
  private readonly formBuilder = inject(FormBuilder);
  private readonly internshipService = inject(InternshipService);
  private readonly messages = inject(MessageService);

  private readonly confirmation = inject(ConfirmationService);

  readonly internshipCase = signal<InternshipCase | null>(null);
  readonly reports = signal<WeeklyReport[]>([]);
  readonly loading = signal(true);
  readonly saving = signal(false);
  readonly dialogVisible = signal(false);
  readonly editingReport = signal<WeeklyReport | null>(null);

  readonly weeks = [1, 2, 3, 4, 5, 6, 7, 8];

  readonly reportForm = this.formBuilder.group({
    weekNumber: this.formBuilder.nonNullable.control(1, [Validators.required, Validators.min(1), Validators.max(8)]),
    startDate: this.formBuilder.nonNullable.control('', Validators.required),
    endDate: this.formBuilder.nonNullable.control('', Validators.required),
    activityDescription: this.formBuilder.nonNullable.control('', [Validators.required, Validators.pattern(/\S/)])
  });

  constructor() {
    this.load();
  }

  openCreate(week = this.nextWeek()): void {
    if (this.internshipCase()?.status !== 'ACTIVE') return;
    this.editingReport.set(null);
    this.reportForm.reset({ weekNumber: week, startDate: '', endDate: '', activityDescription: '' });
    this.reportForm.controls.weekNumber.enable();
    this.dialogVisible.set(true);
  }

  reportForWeek(week: number): WeeklyReport | undefined {
    return this.reports().find((report) => report.weekNumber === week);
  }

  openEdit(report: WeeklyReport): void {
    if (!this.canEdit(report)) return;
    this.editingReport.set(report);
    this.reportForm.controls.weekNumber.disable();
    this.reportForm.reset({
      weekNumber: report.weekNumber,
      startDate: report.startDate.slice(0, 10),
      endDate: report.endDate.slice(0, 10),
      activityDescription: report.activityDescription
    });
    this.dialogVisible.set(true);
  }

  save(): void {
    if (this.saving() || this.internshipCase()?.status !== 'ACTIVE') return;
    if (this.reportForm.invalid) {
      this.reportForm.markAllAsTouched();
      this.messages.add({ severity: 'warn', summary: 'اطلاعات ناقص', detail: 'تمام فیلدهای گزارش را تکمیل کنید.' });
      return;
    }
    const value = this.reportForm.getRawValue();
    if (value.endDate < value.startDate) {
      this.messages.add({ severity: 'warn', summary: 'تاریخ نامعتبر', detail: 'تاریخ پایان نباید پیش از تاریخ شروع باشد.' });
      return;
    }
    const payload = { ...value, activityDescription: value.activityDescription.trim() };
    const editing = this.editingReport();
    if (!editing && this.reportForWeek(value.weekNumber)) {
      this.messages.add({ severity: 'warn', summary: 'هفته تکراری', detail: 'برای این هفته قبلاً گزارش ثبت شده است.' });
      return;
    }
    this.saving.set(true);
    const request = editing
      ? this.internshipService.updateWeeklyReport(editing.id, payload)
      : this.internshipService.createWeeklyReport(payload);
    request.subscribe({
      next: (report) => {
        const current = this.reports().filter((item) => item.id !== report.id);
        this.reports.set([...current, report].sort((a, b) => a.weekNumber - b.weekNumber));
        this.saving.set(false);
        this.dialogVisible.set(false);
        this.messages.add({ severity: 'success', summary: 'ثبت شد', detail: editing ? 'گزارش با موفقیت ویرایش شد.' : 'گزارش هفتگی با موفقیت ثبت شد.' });
      },
      error: (error: HttpErrorResponse) => {
        this.saving.set(false);
        this.showError(error);
      }
    });
  }

  canEdit(report: WeeklyReport): boolean {
    return this.internshipCase()?.status === 'ACTIVE' && (report.status === 'DRAFT' || report.status === 'REVISION_REQUESTED');
  }

  confirmSubmit(report: WeeklyReport): void {
    if (!this.canEdit(report) || this.saving()) return;
    this.confirmation.confirm({
      header: report.status === 'DRAFT' ? 'ارسال گزارش' : 'ارسال مجدد گزارش',
      message: report.status === 'DRAFT' ? 'آیا از ارسال این گزارش برای بررسی مطمئن هستید؟' : 'آیا از ارسال مجدد گزارش اصلاح‌شده مطمئن هستید؟',
      acceptLabel: 'ارسال', rejectLabel: 'انصراف',
      accept: () => {
        this.saving.set(true);
        this.internshipService.submitWeeklyReport(report.id).subscribe({
          next: (updated) => {
            this.reports.update((items) => items.map((item) => item.id === updated.id ? updated : item));
            this.saving.set(false);
            this.messages.add({ severity: 'success', summary: 'ارسال شد', detail: 'گزارش برای بررسی سرپرست شرکت و استاد ارسال شد.' });
          },
          error: (error: HttpErrorResponse) => { this.saving.set(false); this.showError(error); }
        });
      }
    });
  }

  private load(): void {
    this.internshipService.getCurrentCase().subscribe({
      next: (internshipCase) => {
        this.internshipCase.set(internshipCase);
        if (internshipCase.status !== 'ACTIVE' && internshipCase.status !== 'COMPLETED') {
          this.loading.set(false);
          return;
        }
        this.internshipService.listStudentWeeklyReports().subscribe({
          next: (reports) => { this.reports.set(reports); this.loading.set(false); },
          error: (error: HttpErrorResponse) => { this.loading.set(false); this.showError(error); }
        });
      },
      error: () => this.loading.set(false)
    });
  }

  private nextWeek(): number {
    const used = new Set(this.reports().map((report) => report.weekNumber));
    for (let week = 1; week <= 8; week++) if (!used.has(week)) return week;
    return 8;
  }

  private showError(error: HttpErrorResponse): void {
    this.messages.add({ severity: 'error', summary: 'خطا', detail: userErrorMessage(error, 'انجام عملیات ناموفق بود.') });
  }
}
