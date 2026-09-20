import { HttpErrorResponse } from '@angular/common/http';
import { Component, ElementRef, inject, signal, viewChild } from '@angular/core';
import { MessageService } from 'primeng/api';
import { ButtonModule } from 'primeng/button';
import { CardModule } from 'primeng/card';

import { InternshipCase } from '../../internship/internship.models';
import { InternshipService } from '../../internship/internship.service';
import { JalaliDatePipe } from '../../shared/jalali-date/jalali-date.pipe';

@Component({
  selector: 'app-student-final-report',
  imports: [JalaliDatePipe, ButtonModule, CardModule],
  templateUrl: './student-final-report.component.html',
  styleUrls: ['../workflow-page.scss', './student-final-report.component.scss']
})
export class StudentFinalReportComponent {
  private readonly internshipService = inject(InternshipService);
  private readonly messages = inject(MessageService);
  private readonly fileInput = viewChild<ElementRef<HTMLInputElement>>('fileInput');

  readonly internshipCase = signal<InternshipCase | null>(null);
  readonly selectedFile = signal<File | null>(null);
  readonly loading = signal(true);
  readonly uploading = signal(false);
  readonly downloading = signal(false);

  constructor() {
    this.internshipService.getCurrentCase().subscribe({
      next: (internshipCase) => { this.internshipCase.set(internshipCase); this.loading.set(false); },
      error: () => this.loading.set(false)
    });
  }

  chooseFile(): void {
    this.fileInput()?.nativeElement.click();
  }

  fileChanged(event: Event): void {
    const file = (event.target as HTMLInputElement).files?.[0] ?? null;
    if (!file) return;
    if (file.type !== 'application/pdf' || !file.name.toLowerCase().endsWith('.pdf') || file.size > 10 * 1024 * 1024) {
      this.selectedFile.set(null);
      this.messages.add({ severity: 'warn', summary: 'فایل نامعتبر', detail: 'فقط فایل پی‌دی‌اف با حداکثر حجم ۱۰ مگابایت مجاز است.' });
      return;
    }
    this.selectedFile.set(file);
  }

  upload(): void {
    const file = this.selectedFile();
    if (!file) {
      this.messages.add({ severity: 'warn', summary: 'انتخاب فایل', detail: 'ابتدا فایل پی‌دی‌اف گزارش را انتخاب کنید.' });
      return;
    }
    this.uploading.set(true);
    this.internshipService.uploadFinalReport(file).subscribe({
      next: (metadata) => {
        const current = this.internshipCase();
        if (current) this.internshipCase.set({ ...current, finalReport: metadata });
        this.selectedFile.set(null);
        if (this.fileInput()) this.fileInput()!.nativeElement.value = '';
        this.uploading.set(false);
        this.messages.add({ severity: 'success', summary: 'بارگذاری شد', detail: 'گزارش نهایی با موفقیت بارگذاری شد.' });
      },
      error: (error: HttpErrorResponse) => {
        this.uploading.set(false);
        const detail = error.status === 400 ? 'فایل باید پی‌دی‌اف و حداکثر ۱۰ مگابایت باشد.' : 'بارگذاری گزارش نهایی ناموفق بود.';
        this.messages.add({ severity: 'error', summary: 'خطا', detail });
      }
    });
  }

  download(): void {
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
        this.messages.add({ severity: 'error', summary: 'خطا', detail: 'دریافت فایل ناموفق بود.' });
      }
    });
  }
}
