import { Component, inject, OnInit } from '@angular/core';
import { finalize } from 'rxjs';
import { ButtonModule } from 'primeng/button';
import { CardModule } from 'primeng/card';
import { ProgressSpinnerModule } from 'primeng/progressspinner';
import { TagModule } from 'primeng/tag';

import { HealthService } from './services/health.service';

@Component({
  selector: 'app-root',
  imports: [ButtonModule, CardModule, ProgressSpinnerModule, TagModule],
  templateUrl: './app.component.html',
  styleUrl: './app.component.scss'
})
export class AppComponent implements OnInit {
  private readonly healthService = inject(HealthService);

  loading = true;
  connected = false;

  ngOnInit(): void {
    this.checkServer();
  }

  checkServer(): void {
    this.loading = true;

    this.healthService
      .check()
      .pipe(finalize(() => (this.loading = false)))
      .subscribe({
        next: (response) => {
          this.connected = response.status === 'ok';
        },
        error: () => {
          this.connected = false;
        }
      });
  }
}

