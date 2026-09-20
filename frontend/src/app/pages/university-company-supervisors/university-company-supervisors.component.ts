import { HttpErrorResponse } from '@angular/common/http';
import { Component, inject, signal } from '@angular/core';
import { FormBuilder, ReactiveFormsModule, Validators } from '@angular/forms';
import { finalize, forkJoin } from 'rxjs';
import { MessageService } from 'primeng/api';
import { ButtonModule } from 'primeng/button';
import { DialogModule } from 'primeng/dialog';
import { InputTextModule } from 'primeng/inputtext';
import { SelectModule } from 'primeng/select';
import { TableModule } from 'primeng/table';

import { Company } from '../../internship/internship.models';
import { CompanySupervisor, CreatedAccount } from '../../university-management/university-management.models';
import { UniversityManagementService } from '../../university-management/university-management.service';

@Component({
  selector: 'app-university-company-supervisors',
  imports: [ReactiveFormsModule, ButtonModule, DialogModule, InputTextModule, SelectModule, TableModule],
  templateUrl: './university-company-supervisors.component.html',
  styleUrl: '../workflow-page.scss'
})
export class UniversityCompanySupervisorsComponent {
  private readonly management = inject(UniversityManagementService);
  private readonly messages = inject(MessageService);
  private readonly formBuilder = inject(FormBuilder);

  readonly supervisors = signal<CompanySupervisor[]>([]);
  readonly companies = signal<Company[]>([]);
  readonly loading = signal(true);
  readonly dialogVisible = signal(false);
  readonly credentials = signal<CreatedAccount | null>(null);
  submitting = false;
  readonly form = this.formBuilder.nonNullable.group({
    fullName: ['', Validators.required],
    email: ['', [Validators.required, Validators.email]],
    companyId: [0, Validators.min(1)]
  });

  constructor() { this.load(); }

  openDialog(): void {
    this.form.reset({ fullName: '', email: '', companyId: 0 });
    this.dialogVisible.set(true);
  }

  submit(): void {
    if (this.form.invalid) { this.form.markAllAsTouched(); return; }
    this.submitting = true;
    const raw = this.form.getRawValue();
    this.management.createCompanySupervisor({ fullName: raw.fullName.trim(), email: raw.email.trim(), companyId: raw.companyId })
      .pipe(finalize(() => (this.submitting = false))).subscribe({
        next: (account) => { this.dialogVisible.set(false); this.credentials.set(account); this.load(); },
        error: (error: HttpErrorResponse) => this.showError(error, 'ایجاد حساب سرپرست ناموفق بود.')
      });
  }

  closeCredentials(visible: boolean): void { if (!visible) this.credentials.set(null); }

  copyCredentials(account: CreatedAccount): void {
    void navigator.clipboard.writeText(`نام کاربری: ${account.user.email}\nرمز عبور موقت: ${account.temporaryPassword}`).then(() =>
      this.messages.add({ severity: 'success', summary: 'کپی شد', detail: 'اطلاعات ورود در کلیپ‌بورد قرار گرفت.' })
    );
  }

  private load(): void {
    this.loading.set(true);
    forkJoin({ supervisors: this.management.listCompanySupervisors(), companies: this.management.listCompanies() }).subscribe({
      next: ({ supervisors, companies }) => { this.supervisors.set(supervisors); this.companies.set(companies); this.loading.set(false); },
      error: (error: HttpErrorResponse) => { this.loading.set(false); this.showError(error, 'دریافت اطلاعات ناموفق بود.'); }
    });
  }

  private showError(error: HttpErrorResponse, fallback: string): void {
    this.messages.add({ severity: 'error', summary: 'خطا', detail: error.status === 0 ? 'ارتباط با سرور برقرار نشد.' : (error.error?.error || fallback) });
  }
}
