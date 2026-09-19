import { HttpErrorResponse } from '@angular/common/http';
import { Component, inject, signal } from '@angular/core';
import { FormBuilder, ReactiveFormsModule, Validators } from '@angular/forms';
import { MessageService } from 'primeng/api';
import { ButtonModule } from 'primeng/button';
import { CardModule } from 'primeng/card';
import { DialogModule } from 'primeng/dialog';
import { InputNumberModule } from 'primeng/inputnumber';
import { TagModule } from 'primeng/tag';
import { TextareaModule } from 'primeng/textarea';

import { InternshipCase, WeeklyReport } from '../../internship/internship.models';
import { InternshipService } from '../../internship/internship.service';
import { JalaliDatePickerComponent } from '../../shared/jalali-date/jalali-date-picker.component';
import { JalaliDatePipe } from '../../shared/jalali-date/jalali-date.pipe';

@Component({
  selector: 'app-student-weekly-reports',
  imports: [ReactiveFormsModule, JalaliDatePickerComponent, JalaliDatePipe, ButtonModule, CardModule, DialogModule, InputNumberModule, TagModule, TextareaModule],
  templateUrl: './student-weekly-reports.component.html',
  styleUrls: ['../workflow-page.scss', './student-weekly-reports.component.scss']
})
export class StudentWeeklyReportsComponent {
  private readonly formBuilder = inject(FormBuilder);
  private readonly internshipService = inject(InternshipService);
  private readonly messages = inject(MessageService);

  readonly internshipCase = signal<InternshipCase | null>(null);
  readonly reports = signal<WeeklyReport[]>([]);
  readonly loading = signal(true);
  readonly saving = signal(false);
  readonly dialogVisible = signal(false);
  readonly editingReport = signal<WeeklyReport | null>(null);

  readonly reportForm = this.formBuilder.group({
    weekNumber: this.formBuilder.nonNullable.control(1, [Validators.required, Validators.min(1), Validators.max(8)]),
    startDate: this.formBuilder.nonNullable.control('', Validators.required),
    endDate: this.formBuilder.nonNullable.control('', Validators.required),
    activityDescription: this.formBuilder.nonNullable.control('', Validators.required)
  });

  constructor() {
    this.load();
  }

  openCreate(): void {
    this.editingReport.set(null);
    this.reportForm.reset({ weekNumber: this.nextWeek(), startDate: '', endDate: '', activityDescription: '' });
    this.dialogVisible.set(true);
  }

  openEdit(report: WeeklyReport): void {
    if (report.isConfirmed) return;
    this.editingReport.set(report);
    this.reportForm.reset({
      weekNumber: report.weekNumber,
      startDate: report.startDate.slice(0, 10),
      endDate: report.endDate.slice(0, 10),
      activityDescription: report.activityDescription
    });
    this.dialogVisible.set(true);
  }

  save(): void {
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
    let detail = 'انجام عملیات ناموفق بود.';
    if (error.status === 409) detail = 'برای این هفته گزارش ثبت شده یا گزارش تأیید شده و قابل ویرایش نیست.';
    if (error.status === 400) detail = 'اطلاعات گزارش معتبر نیست.';
    this.messages.add({ severity: 'error', summary: 'خطا', detail });
  }
}
