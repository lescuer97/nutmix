// Core HTMX initialization
import htmx from 'htmx.org';
import 'htmx-ext-preload';
import 'htmx-ext-remove-me';

/**
 * Initialize HTMX and make it globally available
 */
export function initCore() {
  // Make HTMX available globally for use in templates and other scripts
  window.htmx = htmx;
  window.copyLdkText = function copyLdkText(button) {
    if (!button || button.disabled || !navigator.clipboard || !navigator.clipboard.writeText) {
      return;
    }

    var text = button.dataset.copyText || '';
    var defaultText = button.dataset.copyDefaultText || button.textContent || 'Copy';
    var successText = button.dataset.copySuccessText || 'Copied';

    navigator.clipboard.writeText(text).then(function () {
      button.textContent = successText;
      window.setTimeout(function () {
        button.textContent = defaultText;
      }, 1200);
    });
  };
  console.log('HTMX initialized');
}
