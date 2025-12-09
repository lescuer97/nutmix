// Core HTMX initialization
import htmx from 'htmx.org';
import 'htmx-ext-preload';
import 'htmx-ext-remove-me';

/**
 * Initialize HTMX and make it globally available
 */
export function initCore() {
  // Make HTMX available globally for use in templates and other scripts
document.body.addEventListener('htmx:load', function (evt) {
  htmx.logAll();
});
  window.htmx = htmx;
  console.log('HTMX initialized');
}
