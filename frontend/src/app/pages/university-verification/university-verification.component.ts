import { Component, inject, signal } from '@angular/core';
import { RouterLink } from '@angular/router';
import { ButtonModule } from 'primeng/button';
import { finalize } from 'rxjs';
import { AuthService } from '../../auth/auth.service';
import { verificationLabels } from '../../auth/auth.models';
import { UserAccessService } from '../../user-access/user-access.service';
import { JalaliDatePipe } from '../../shared/jalali-date/jalali-date.pipe';
import { userErrorMessage } from '../../shared/http-error-message';
@Component({
 selector: 'app-university-verification',
 imports: [RouterLink, ButtonModule, JalaliDatePipe],
 templateUrl: './university-verification.component.html',
 styleUrl: '../workflow-page.scss'
})
export class UniversityVerificationComponent {
 private readonly auth = inject(AuthService);
 private readonly access = inject(UserAccessService);
 readonly user = this.auth.user;
 readonly labels = verificationLabels;
 readonly busy = signal(false);
 readonly error = signal('');
 refresh(): void {
  if (this.busy()) return;
  this.busy.set(true); this.error.set('');
  this.auth.refreshUser().pipe(finalize(() => this.busy.set(false))).subscribe({ error: error => this.error.set(userErrorMessage(error, 'دریافت وضعیت ناموفق بود.')) });
 }
 resubmit(): void {
  if (this.busy()) return;
  this.busy.set(true); this.error.set('');
  this.access.resubmit().pipe(finalize(() => this.busy.set(false))).subscribe({
   next: user => this.auth.updateCachedUser(user),
   error: error => this.error.set(userErrorMessage(error, 'ارسال مجدد ناموفق بود.'))
  });
 }
}
