import { HttpErrorResponse } from '@angular/common/http';
import { userErrorMessage } from '../../shared/http-error-message';
import { Component, ElementRef, inject, signal, viewChild } from '@angular/core';
import { RouterLink } from '@angular/router';
import { MessageService } from 'primeng/api';
import { ButtonModule } from 'primeng/button';
import { CardModule } from 'primeng/card';
import { TagModule } from 'primeng/tag';

import {
  InternshipCase,
  internshipStatusLabels,
  internshipStatusSeverity,
  finalReportStatusLabels,
  finalReportStatusSeverity
} from '../../internship/internship.models';
import { InternshipService } from '../../internship/internship.service';
import { JalaliDatePipe } from '../../shared/jalali-date/jalali-date.pipe';

@Component({
  selector: 'app-student-final-report',
  imports: [RouterLink, JalaliDatePipe, ButtonModule, CardModule, TagModule],
  templateUrl: './student-final-report.component.html',
  styleUrls: ['../workflow-page.scss', './student-final-report.component.scss']
})
export class StudentFinalReportComponent {
  private readonly internshipService = inject(InternshipService);
  private readonly messages = inject(MessageService);
  private readonly fileInput = viewChild<ElementRef<HTMLInputElement>>('fileInput');

  readonly finalReportStatusLabels = finalReportStatusLabels;
  readonly finalReportStatusSeverity = finalReportStatusSeverity;
  readonly internshipStatusLabels = internshipStatusLabels;
  readonly internshipStatusSeverity = internshipStatusSeverity;
  readonly internshipCase = signal<InternshipCase | null>(null);
  readonly selectedFile = signal<File | null>(null);
  readonly loading = signal(true);
  readonly loadFailed = signal(false);
  readonly uploading = signal(false);
  readonly downloading = signal(false);

  constructor() {
    this.load();
  }

  load(): void {
    this.loading.set(true);
    this.loadFailed.set(false);
    this.internshipService.getCurrentCase().subscribe({
      next: (internshipCase) => {
        this.internshipCase.set(internshipCase);
        this.loading.set(false);
      },
      error: (error: HttpErrorResponse) => {
        this.loading.set(false);
        if (error.status === 404) {
          this.internshipCase.set(null);
          return;
        }
        this.loadFailed.set(true);
        this.messages.add({
          severity: 'error',
          summary: 'خطا',
          detail: userErrorMessage(error, 'دریافت اطلاعات کارآموزی ناموفق بود.')
        });
      }
    });
  }

  canUpload(): boolean {
    return this.internshipCase()?.canUploadFinalReport === true;
  }

  chooseFile(): void {
    if (!this.canUpload() || this.uploading()) return;
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
    if (!this.canUpload() || this.uploading()) return;
    const file = this.selectedFile();
    if (!file) {
      this.messages.add({ severity: 'warn', summary: 'انتخاب فایل', detail: 'ابتدا فایل پی‌دی‌اف گزارش را انتخاب کنید.' });
      return;
    }
    this.uploading.set(true);
    this.internshipService.uploadFinalReport(file).subscribe({
      next: (metadata) => {
        const current = this.internshipCase();
        if (current) this.internshipCase.set({ ...current, finalReport: metadata, canUploadFinalReport: false });
        this.selectedFile.set(null);
        if (this.fileInput()) this.fileInput()!.nativeElement.value = '';
        this.uploading.set(false);
        this.messages.add({ severity: 'success', summary: 'بارگذاری شد', detail: 'گزارش نهایی با موفقیت بارگذاری شد.' });
      },
      error: (error: HttpErrorResponse) => {
        this.uploading.set(false);
        const detail = userErrorMessage(error, 'بارگذاری گزارش نهایی ناموفق بود.');
        this.messages.add({ severity: 'error', summary: 'خطا', detail });
        if (error.status === 400 || error.status === 409) this.load();
      }
    });
  }

  download(): void {
    const report = this.internshipCase()?.finalReport;
    if (!report || this.downloading()) return;
    this.downloading.set(true);
    this.internshipService.downloadFile(report.currentFileId).subscribe({
      next: (blob) => {
        const url = URL.createObjectURL(blob);
        const anchor = document.createElement('a');
        anchor.href = url;
        anchor.download = report.currentFile.originalName;
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
