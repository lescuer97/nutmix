// Main entry point - imports and initializes all modules in correct order

// 1. Core HTMX setup - must be first
import { initCore } from './modules/core.js';

// 2. Authentication module
import { initAuth } from './modules/auth.js';

// 3. Time controls module
import { initTimeControls } from './modules/time-controls.js';

// 4. Chart module
import { initCharts } from './modules/chart.js';

/**
 * Initialize the application
 * Called when DOM is ready
 */
function initializeApp() {
  // Initialize core (HTMX and extensions)
  initCore();
  
  // Initialize authentication handlers
  initAuth();
  
  // Initialize time controls
  initTimeControls();
  
  // Initialize charts (will only render if chart elements exist on the page)
  initCharts();
  
  console.log('App fully initialized');
}

// Wait for DOM to be ready before initializing
if (document.readyState === 'loading') {
  document.addEventListener('DOMContentLoaded', initializeApp);
} else {
  // DOM is already ready
  initializeApp();
}
