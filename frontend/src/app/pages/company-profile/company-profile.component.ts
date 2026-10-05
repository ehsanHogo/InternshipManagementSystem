import { HttpErrorResponse } from '@angular/common/http';
import { Component, inject, signal } from '@angular/core';
import { FormBuilder, ReactiveFormsModule, Validators } from '@angular/forms';
import { finalize } from 'rxjs';
import { MessageService } from 'primeng/api';
import { ButtonModule } from 'primeng/button';
import { CardModule } from 'primeng/card';
import { TagModule } from 'primeng/tag';
import { InputTextModule } from 'primeng/inputtext';
import { TextareaModule } from 'primeng/textarea';

import { AuthService } from '../../auth/auth.service';
import { CompanyAccountProfile } from '../../company/company-account.models';
import { CompanyAccountService } from '../../company/company-account.service';
import { registrationStatusLabels } from '../../internship/internship.models';
import { userErrorMessage } from '../../shared/http-error-message';

@Component({
  selector: 'app-company-profile',
  imports: [ReactiveFormsModule, ButtonModule, CardModule, TagModule, InputTextModule, TextareaModule],
  templateUrl: './company-profile.component.html',
  styleUrls: ['./company-profile.component.scss', '../workflow-page.scss']
})
export class CompanyProfileComponent {
  private readonly companyAccount = inject(CompanyAccountService);
  private readonly auth = inject(AuthService);
  private readonly messages = inject(MessageService);
  private readonly fb = inject(FormBuilder);
  readonly profile = signal<CompanyAccountProfile | null>(null);
  readonly loading = signal(true);
  readonly loadFailed = signal(false);
  readonly busy = signal(false);
  readonly labels = registrationStatusLabels;
  readonly form = this.fb.nonNullable.group({
    company: this.fb.nonNullable.group({
      name: ['', [Validators.required, Validators.maxLength(250)]],
      nationalId: ['', [Validators.required, Validators.maxLength(50)]],
      economicCode: ['', [Validators.required, Validators.maxLength(50)]],
      website: ['', Validators.maxLength(500)], phone: ['', [Validators.required, Validators.maxLength(50)]],
      email: ['', [Validators.required, Validators.email, Validators.maxLength(320)]],
      address: ['', [Validators.required, Validators.maxLength(1000)]]
    }),
    supervisor: this.fb.nonNullable.group({
      fullName: ['', [Validators.required, Validators.maxLength(200)]],
      email: ['', [Validators.required, Validators.email, Validators.maxLength(320)]],
      phone: ['', [Validators.required, Validators.maxLength(50)]],
      jobTitle: ['', [Validators.required, Validators.maxLength(200)]]
    })
  });
  constructor() { this.load(); }
  load(): void {
    this.loading.set(true); this.loadFailed.set(false);
    this.companyAccount.getProfile().pipe(finalize(() => this.loading.set(false))).subscribe({
      next: account => this.setAccount(account),
      error: (error: HttpErrorResponse) => { this.loadFailed.set(true); this.showError(error); }
    });
  }
  private setAccount(account: CompanyAccountProfile): void {
    this.profile.set(account);
    this.form.reset({
      company: { name: account.company.name, nationalId: account.company.nationalId, economicCode: account.company.economicCode,
        website: account.company.website ?? '', phone: account.company.phone ?? '', email: account.company.email ?? '', address: account.company.address ?? '' },
      supervisor: { fullName: account.supervisor.fullName, email: account.supervisor.email, phone: account.supervisor.phone ?? '', jobTitle: account.supervisor.jobTitle ?? '' }
    });
    this.auth.refreshUser().subscribe({ error: () => {} });
  }
  save(): void {
    if (this.busy()) return;
    if (this.form.invalid) { this.form.markAllAsTouched(); return; }
    this.busy.set(true);
    this.companyAccount.updateProfile(this.form.getRawValue()).pipe(finalize(() => this.busy.set(false))).subscribe({
      next: account => { this.setAccount(account); this.messages.add({ severity: 'success', summary: 'ذخیره شد', detail: 'اطلاعات ثبت شرکت ذخیره شد.' }); },
      error: (error: HttpErrorResponse) => { this.showError(error); if (error.status === 409) this.load(); }
    });
  }
  resubmit(): void {
    if (this.busy() || this.form.dirty || this.form.invalid) return;
    this.busy.set(true);
    this.companyAccount.resubmit().pipe(finalize(() => this.busy.set(false))).subscribe({
      next: account => { this.setAccount(account); this.messages.add({ severity: 'success', summary: 'ارسال شد', detail: 'ثبت شرکت برای بررسی مجدد ارسال شد.' }); },
      error: (error: HttpErrorResponse) => { this.showError(error); if (error.status === 409) this.load(); }
    });
  }
  private showError(error: HttpErrorResponse): void {
    this.messages.add({ severity: 'error', summary: 'خطا', detail: userErrorMessage(error, 'انجام درخواست ثبت شرکت ناموفق بود.') });
  }
}
