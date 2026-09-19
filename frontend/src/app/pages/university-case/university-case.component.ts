import { DatePipe } from '@angular/common';
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
  InternshipCase,
  InternshipCaseStatus,
  InternshipPreference,
  internshipStatusLabels
} from '../../internship/internship.models';
import { InternshipService } from '../../internship/internship.service';

@Component({
  selector: 'app-university-case',
  imports: [
    DatePipe,
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

  selectPreference(preferenceID: number): void {
    this.reviewForm.controls.preferenceId.setValue(preferenceID);
  }

  sendToCompany(): void {
    if (this.reviewForm.invalid) {
      this.reviewForm.markAllAsTouched();
      this.messages.add({ severity: 'warn', summary: 'اطلاعات ناقص', detail: 'محل کارآموزی، سرپرست شرکت و اطلاعات نامه را وارد کنید.' });
      return;
    }
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
}
