import { HttpErrorResponse } from '@angular/common/http';
import { Component, inject, signal } from '@angular/core';
import { FormBuilder, ReactiveFormsModule, Validators } from '@angular/forms';
import { finalize } from 'rxjs';
import { MessageService } from 'primeng/api';
import { ButtonModule } from 'primeng/button';
import { DialogModule } from 'primeng/dialog';
import { InputTextModule } from 'primeng/inputtext';
import { TableModule } from 'primeng/table';
import { TagModule } from 'primeng/tag';
import { TextareaModule } from 'primeng/textarea';

import {
  CompanyOpportunity,
  OpportunityPayload,
  OpportunityStatus,
  opportunityStatusLabels
} from '../../opportunity/opportunity.models';
import { OpportunityService } from '../../opportunity/opportunity.service';
import { userErrorMessage } from '../../shared/http-error-message';
import { JalaliDatePipe } from '../../shared/jalali-date/jalali-date.pipe';

@Component({
  selector: 'app-company-opportunities',
  imports: [
    ReactiveFormsModule,
    ButtonModule,
    DialogModule,
    InputTextModule,
    TableModule,
    TagModule,
    TextareaModule,
    JalaliDatePipe
  ],
  templateUrl: './company-opportunities.component.html',
  styleUrl: './company-opportunities.component.scss'
})
export class CompanyOpportunitiesComponent {
  private readonly opportunitiesService = inject(OpportunityService);
  private readonly messages = inject(MessageService);
  private readonly formBuilder = inject(FormBuilder);

  readonly opportunities = signal<CompanyOpportunity[]>([]);
  readonly loading = signal(true);
  readonly loadFailed = signal(false);
  readonly formVisible = signal(false);
  readonly viewVisible = signal(false);
  readonly closeVisible = signal(false);
  readonly selected = signal<CompanyOpportunity | null>(null);
  readonly editing = signal<CompanyOpportunity | null>(null);
  readonly detailLoading = signal(false);
  readonly closing = signal(false);
  submitting = false;

  readonly form = this.formBuilder.nonNullable.group({
    title: ['', [Validators.required, Validators.maxLength(250)]],
    description: ['', [Validators.required, Validators.maxLength(10000)]],
    workField: ['', [Validators.required, Validators.maxLength(200)]],
    location: ['', [Validators.required, Validators.maxLength(500)]]
  });

  constructor() {
    this.load();
  }

  load(): void {
    this.loading.set(true);
    this.loadFailed.set(false);
    this.opportunitiesService.listCompany().subscribe({
      next: (opportunities) => {
        this.opportunities.set(opportunities);
        this.loading.set(false);
      },
      error: (error: HttpErrorResponse) => {
        this.loading.set(false);
        this.loadFailed.set(true);
        this.showError(error, 'دریافت فرصت‌های کارآموزی ناموفق بود.');
      }
    });
  }

  openCreate(): void {
    this.editing.set(null);
    this.form.reset();
    this.formVisible.set(true);
  }

  openEdit(opportunity: CompanyOpportunity): void {
    this.editing.set(opportunity);
    this.form.reset({
      title: opportunity.title,
      description: opportunity.description,
      workField: opportunity.workField,
      location: opportunity.location
    });
    this.formVisible.set(true);
  }

  openView(opportunity: CompanyOpportunity): void {
    this.selected.set(opportunity);
    this.viewVisible.set(true);
    this.detailLoading.set(true);
    this.opportunitiesService.getCompany(opportunity.id).pipe(finalize(() => this.detailLoading.set(false))).subscribe({
      next: (detail) => this.selected.set(detail),
      error: (error: HttpErrorResponse) => {
        this.viewVisible.set(false);
        this.showError(error, 'دریافت جزئیات فرصت ناموفق بود.');
      }
    });
  }

  submit(): void {
    if (this.submitting) return;
    if (this.form.invalid || this.hasBlankValue()) {
      this.form.markAllAsTouched();
      return;
    }
    const payload = this.payload();
    const editing = this.editing();
    this.submitting = true;
    const request = editing
      ? this.opportunitiesService.update(editing.id, payload)
      : this.opportunitiesService.create(payload);
    request.pipe(finalize(() => (this.submitting = false))).subscribe({
      next: (saved) => {
        this.formVisible.set(false);
        this.upsert(saved);
        this.messages.add({
          severity: 'success',
          summary: 'انجام شد',
          detail: editing ? 'فرصت کارآموزی با موفقیت ویرایش شد.' : 'فرصت کارآموزی با موفقیت ایجاد شد.'
        });
      },
      error: (error: HttpErrorResponse) => this.showError(error, editing ? 'ویرایش فرصت ناموفق بود.' : 'ایجاد فرصت ناموفق بود.')
    });
  }

  askToClose(opportunity: CompanyOpportunity): void {
    this.selected.set(opportunity);
    this.closeVisible.set(true);
  }

  confirmClose(): void {
    const opportunity = this.selected();
    if (!opportunity || this.closing()) return;
    this.closing.set(true);
    this.opportunitiesService.close(opportunity.id).pipe(finalize(() => this.closing.set(false))).subscribe({
      next: (closed) => {
        this.closeVisible.set(false);
        this.upsert(closed);
        this.messages.add({ severity: 'success', summary: 'بسته شد', detail: 'فرصت کارآموزی با موفقیت بسته شد.' });
      },
      error: (error: HttpErrorResponse) => this.showError(error, 'بستن فرصت کارآموزی ناموفق بود.')
    });
  }

  statusLabel(status: OpportunityStatus): string {
    return opportunityStatusLabels[status];
  }

  private payload(): OpportunityPayload {
    const raw = this.form.getRawValue();
    return {
      title: raw.title.trim(),
      description: raw.description.trim(),
      workField: raw.workField.trim(),
      location: raw.location.trim()
    };
  }

  private hasBlankValue(): boolean {
    const raw = this.form.getRawValue();
    return !raw.title.trim() || !raw.description.trim() || !raw.workField.trim() || !raw.location.trim();
  }

  private upsert(saved: CompanyOpportunity): void {
    this.opportunities.update((items) => {
      const index = items.findIndex((item) => item.id === saved.id);
      if (index < 0) return [saved, ...items];
      return items.map((item) => (item.id === saved.id ? saved : item));
    });
    if (this.selected()?.id === saved.id) this.selected.set(saved);
  }

  private showError(error: HttpErrorResponse, fallback: string): void {
    this.messages.add({ severity: 'error', summary: 'خطا', detail: userErrorMessage(error, fallback) });
  }
}
