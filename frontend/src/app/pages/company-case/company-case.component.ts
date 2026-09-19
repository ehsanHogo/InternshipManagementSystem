import { DatePipe } from '@angular/common';
import { HttpErrorResponse } from '@angular/common/http';
import { Component, inject, signal } from '@angular/core';
import { FormBuilder, ReactiveFormsModule, Validators } from '@angular/forms';
import { ActivatedRoute, RouterLink } from '@angular/router';
import { ConfirmationService, MessageService } from 'primeng/api';
import { ButtonModule } from 'primeng/button';
import { CardModule } from 'primeng/card';
import { ConfirmDialogModule } from 'primeng/confirmdialog';
import { InputTextModule } from 'primeng/inputtext';
import { TagModule } from 'primeng/tag';

import {
  InternshipCase,
  InternshipCaseStatus,
  InternshipPreference,
  internshipStatusLabels
} from '../../internship/internship.models';
import { InternshipService } from '../../internship/internship.service';

@Component({
  selector: 'app-company-case',
  imports: [DatePipe, ReactiveFormsModule, RouterLink, ButtonModule, CardModule, ConfirmDialogModule, InputTextModule, TagModule],
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

  readonly confirmationForm = this.formBuilder.group({
    internshipSubject: this.formBuilder.nonNullable.control('', Validators.required),
    startDate: this.formBuilder.nonNullable.control('', Validators.required),
    workplaceAddress: this.formBuilder.nonNullable.control('', Validators.required),
    workplacePhone: this.formBuilder.nonNullable.control('', Validators.required)
  });

  constructor() {
    this.internshipService.getCompanyCase(this.caseID).subscribe({
      next: (internshipCase) => {
        this.internshipCase.set(internshipCase);
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
