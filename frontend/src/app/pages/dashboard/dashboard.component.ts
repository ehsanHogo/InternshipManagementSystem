import { HttpErrorResponse } from '@angular/common/http';
import { Component, inject, signal } from '@angular/core';
import { RouterLink } from '@angular/router';
import { ButtonModule } from 'primeng/button';
import { CardModule } from 'primeng/card';
import { TagModule } from 'primeng/tag';

import { AuthService } from '../../auth/auth.service';
import { InternshipCase, InternshipCaseStatus, internshipStatusLabels } from '../../internship/internship.models';
import { InternshipService } from '../../internship/internship.service';

@Component({
  selector: 'app-dashboard',
  imports: [RouterLink, ButtonModule, CardModule, TagModule],
  templateUrl: './dashboard.component.html',
  styleUrl: './dashboard.component.scss'
})
export class DashboardComponent {
  private readonly auth = inject(AuthService);
  private readonly internshipService = inject(InternshipService);

  readonly user = this.auth.user;
  readonly internshipStatus = signal<InternshipCaseStatus | null>(null);
  readonly internshipCase = signal<InternshipCase | null>(null);
  readonly statusLoading = signal(false);

  constructor() {
    if (this.auth.getCurrentUser()?.role === 'STUDENT') {
      this.statusLoading.set(true);
      this.internshipService.getCurrentCase().subscribe({
        next: (internshipCase) => {
          this.internshipCase.set(internshipCase);
          this.internshipStatus.set(internshipCase.status);
          this.statusLoading.set(false);
        },
        error: (error: HttpErrorResponse) => {
          if (error.status === 404) this.internshipStatus.set(null);
          this.statusLoading.set(false);
        }
      });
    }
  }

  statusLabel(status: InternshipCaseStatus | null): string {
    return status ? internshipStatusLabels[status] : 'درخواستی ثبت نشده است';
  }
}
