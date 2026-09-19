import {
  ApplicationConfig,
  inject,
  provideAppInitializer,
  provideZoneChangeDetection
} from '@angular/core';
import { provideHttpClient, withInterceptors } from '@angular/common/http';
import { provideAnimationsAsync } from '@angular/platform-browser/animations/async';
import { provideRouter } from '@angular/router';
import { providePrimeNG } from 'primeng/config';
import { MessageService } from 'primeng/api';
import Aura from '@primeng/themes/aura';

import { routes } from './app.routes';
import { authInterceptor } from './auth/auth.interceptor';
import { AuthService } from './auth/auth.service';

export const appConfig: ApplicationConfig = {
  providers: [
    provideZoneChangeDetection({ eventCoalescing: true }),
    provideRouter(routes),
    provideHttpClient(withInterceptors([authInterceptor])),
    provideAppInitializer(() => inject(AuthService).initialize()),
    provideAnimationsAsync(),
    MessageService,
    providePrimeNG({
      ripple: true,
      theme: {
        preset: Aura,
        options: {
          darkModeSelector: false
        }
      }
    })
  ]
};
