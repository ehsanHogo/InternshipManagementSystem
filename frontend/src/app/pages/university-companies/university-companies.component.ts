import { HttpErrorResponse } from '@angular/common/http';
import { Component, inject, signal } from '@angular/core';
import { FormBuilder, ReactiveFormsModule, Validators } from '@angular/forms';
import { finalize } from 'rxjs';
import { MessageService } from 'primeng/api';
import { ButtonModule } from 'primeng/button';
import { DialogModule } from 'primeng/dialog';
import { InputTextModule } from 'primeng/inputtext';
import { TableModule } from 'primeng/table';
import { TextareaModule } from 'primeng/textarea';

import { Company } from '../../internship/internship.models';
import { UniversityManagementService } from '../../university-management/university-management.service';

@Component({
  selector: 'app-university-companies',
  imports: [ReactiveFormsModule, ButtonModule, DialogModule, InputTextModule, TableModule, TextareaModule],
  templateUrl: './university-companies.component.html',
  styleUrl: '../workflow-page.scss'
})
export class UniversityCompaniesComponent {
  private readonly management = inject(UniversityManagementService);
  private readonly messages = inject(MessageService);
  private readonly formBuilder = inject(FormBuilder);

  readonly companies = signal<Company[]>([]);
  readonly loading = signal(true);
  readonly dialogVisible = signal(false);
  submitting = false;
  readonly form = this.formBuilder.nonNullable.group({
    name: ['', Validators.required], website: [''], phone: [''],
    email: ['', Validators.email], address: ['']
  });

  constructor() { this.load(); }

  openDialog(): void {
    this.form.reset();
    this.dialogVisible.set(true);
  }

  submit(): void {
    if (this.form.invalid) { this.form.markAllAsTouched(); return; }
    this.submitting = true;
    const raw = this.form.getRawValue();
    this.management.createCompany({
      name: raw.name.trim(), website: raw.website.trim(), phone: raw.phone.trim(),
      email: raw.email.trim(), address: raw.address.trim()
    }).pipe(finalize(() => (this.submitting = false))).subscribe({
      next: () => {
        this.dialogVisible.set(false);
        this.messages.add({ severity: 'success', summary: 'ثبت شد', detail: 'شرکت موردنظر به فهرست شرکت‌های تأییدشده اضافه شد.' });
        this.load();
      },
      error: (error: HttpErrorResponse) => this.showError(error, 'ایجاد شرکت ناموفق بود.')
    });
  }

  private load(): void {
    this.loading.set(true);
    this.management.listCompanies().subscribe({
      next: (companies) => { this.companies.set(companies); this.loading.set(false); },
      error: (error: HttpErrorResponse) => { this.loading.set(false); this.showError(error, 'دریافت شرکت‌ها ناموفق بود.'); }
    });
  }

  private showError(error: HttpErrorResponse, fallback: string): void {
    this.messages.add({ severity: 'error', summary: 'خطا', detail: error.status === 0 ? 'ارتباط با سرور برقرار نشد.' : (error.error?.error || fallback) });
  }
}
