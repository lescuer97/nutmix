// Chart.js module for proofs time-series visualization
import {
  Chart,
  LineController,
  LineElement,
  PointElement,
  LinearScale,
  TimeScale,
  Title,
  Tooltip,
  Legend,
  Filler
} from 'chart.js';
import 'chartjs-adapter-date-fns';

// Register Chart.js components
Chart.register(
  LineController,
  LineElement,
  PointElement,
  LinearScale,
  TimeScale,
  Title,
  Tooltip,
  Legend,
  Filler
);

// Color scheme matching the design system
const COLORS = {
  cyan: '#00d9b1',
  cyanLight: 'rgba(0, 217, 177, 0.1)',
  purple: '#8b5cf6',
  purpleLight: 'rgba(139, 92, 246, 0.1)',
  gridColor: '#2d333b',
  textColor: '#8b939f',
  textPrimary: '#ffffff'
};

// Store chart instance for updates
let proofsChartInstance = null;

/**
 * Initialize the proofs chart
 * @param {HTMLCanvasElement} canvas - The canvas element to render the chart
 * @param {Array} data - Array of {timestamp, totalAmount, count} objects
 */
export function initProofsChart(canvas, data) {
  if (!canvas || !data) {
    console.warn('Chart initialization skipped: missing canvas or data');
    return null;
  }

  // Transform data for Chart.js
  const chartData = data.map(point => ({
    x: new Date(point.timestamp * 1000), // Convert Unix timestamp to Date
    amount: point.totalAmount,
    count: point.count
  }));

  const chart = new Chart(canvas, {
    type: 'line',
    data: {
      datasets: [
        {
          label: 'Sats Value',
          data: chartData.map(d => ({ x: d.x, y: d.amount })),
          borderColor: COLORS.purple,
          backgroundColor: COLORS.purpleLight,
          borderWidth: 2,
          fill: true,
          tension: 0.3,
          pointRadius: 3,
          pointHoverRadius: 6,
          pointBackgroundColor: COLORS.purple,
          pointBorderColor: COLORS.purple,
          yAxisID: 'y'
        },
        {
          label: 'Proof Count',
          data: chartData.map(d => ({ x: d.x, y: d.count })),
          borderColor: COLORS.cyan,
          backgroundColor: COLORS.cyanLight,
          borderWidth: 2,
          fill: true,
          tension: 0.3,
          pointRadius: 3,
          pointHoverRadius: 6,
          pointBackgroundColor: COLORS.cyan,
          pointBorderColor: COLORS.cyan,
          yAxisID: 'y1'
        }
      ]
    },
    options: {
      responsive: true,
      maintainAspectRatio: false,
      interaction: {
        mode: 'index',
        intersect: false
      },
      plugins: {
        legend: {
          display: true,
          position: 'top',
          labels: {
            color: COLORS.textPrimary,
            usePointStyle: true,
            padding: 20,
            font: {
              family: "'Inter', sans-serif",
              size: 12
            }
          }
        },
        tooltip: {
          backgroundColor: '#161b22',
          titleColor: COLORS.textPrimary,
          bodyColor: COLORS.textColor,
          borderColor: COLORS.gridColor,
          borderWidth: 1,
          padding: 12,
          displayColors: true,
          callbacks: {
            title: function(tooltipItems) {
              const date = tooltipItems[0].parsed.x;
              return new Date(date).toLocaleString();
            },
            label: function(context) {
              let label = context.dataset.label || '';
              if (label) {
                label += ': ';
              }
              if (context.parsed.y !== null) {
                if (context.dataset.label === 'Sats Value') {
                  label += context.parsed.y.toLocaleString() + ' sats';
                } else {
                  label += context.parsed.y.toLocaleString() + ' proofs';
                }
              }
              return label;
            }
          }
        }
      },
      scales: {
        x: {
          type: 'time',
          time: {
            displayFormats: {
              hour: 'MMM d, HH:mm',
              day: 'MMM d',
              week: 'MMM d',
              month: 'MMM yyyy'
            },
            tooltipFormat: 'PPpp'
          },
          title: {
            display: true,
            text: 'Time',
            color: COLORS.textColor,
            font: {
              family: "'Inter', sans-serif",
              size: 12,
              weight: '500'
            }
          },
          grid: {
            color: COLORS.gridColor,
            drawBorder: false
          },
          ticks: {
            color: COLORS.textColor,
            font: {
              family: "'Inter', sans-serif",
              size: 11
            },
            maxRotation: 0,
            autoSkip: true,
            maxTicksLimit: 8
          }
        },
        y: {
          type: 'linear',
          display: true,
          position: 'left',
          title: {
            display: true,
            text: 'Sats Value',
            color: COLORS.purple,
            font: {
              family: "'Inter', sans-serif",
              size: 12,
              weight: '500'
            }
          },
          grid: {
            color: COLORS.gridColor,
            drawBorder: false
          },
          ticks: {
            color: COLORS.purple,
            font: {
              family: "'Inter', sans-serif",
              size: 11
            },
            callback: function(value) {
              return value.toLocaleString();
            }
          },
          beginAtZero: true
        },
        y1: {
          type: 'linear',
          display: true,
          position: 'right',
          title: {
            display: true,
            text: 'Proof Count',
            color: COLORS.cyan,
            font: {
              family: "'Inter', sans-serif",
              size: 12,
              weight: '500'
            }
          },
          grid: {
            drawOnChartArea: false // Only show grid lines for the left axis
          },
          ticks: {
            color: COLORS.cyan,
            font: {
              family: "'Inter', sans-serif",
              size: 11
            },
            callback: function(value) {
              return value.toLocaleString();
            }
          },
          beginAtZero: true
        }
      }
    }
  });

  return chart;
}

/**
 * Destroy the current chart instance if it exists
 */
function destroyChart() {
  if (proofsChartInstance) {
    proofsChartInstance.destroy();
    proofsChartInstance = null;
  }
}

/**
 * Initialize or reinitialize the chart from the current DOM
 */
function initializeChartFromDOM() {
  const canvas = document.getElementById('proofsChart');
  const dataElement = document.getElementById('proofsChartData');

  if (!canvas || !dataElement) {
    console.warn('Chart elements not found in DOM');
    return;
  }

  // Destroy existing chart if any
  destroyChart();

  try {
    const data = JSON.parse(dataElement.textContent);
    proofsChartInstance = initProofsChart(canvas, data);
    console.log('Chart initialized with', data.length, 'data points');
  } catch (error) {
    console.error('Failed to initialize chart:', error);
  }
}

/**
 * Set up HTMX event listener to reinitialize chart after content swap
 */
function setupHtmxListener() {
  // Listen for HTMX afterSwap event
  document.body.addEventListener('htmx:afterSwap', (event) => {
    const targetId = event.detail.target?.id;
    
    // Reinitialize when chart wrapper is updated (date picker changes)
    // or when the chart card is loaded initially via HTMX
    if (targetId === 'chart-wrapper' || 
        targetId === 'proofs-chart-placeholder' ||
        targetId === 'proofs-chart-card') {
      console.log('Chart content swapped, reinitializing...');
      // Small delay to ensure DOM is fully updated
      setTimeout(initializeChartFromDOM, 50);
    }
  });

  // Also listen for htmx:afterSettle for initial HTMX loads
  document.body.addEventListener('htmx:afterSettle', (event) => {
    const targetId = event.detail.target?.id;
    
    if (targetId === 'proofs-chart-placeholder') {
      console.log('Chart card settled, initializing...');
      setTimeout(initializeChartFromDOM, 50);
    }
  });
}

/**
 * Initialize charts on page load
 */
export function initCharts() {
  // Set up HTMX listener for dynamic updates (do this first)
  setupHtmxListener();

  const canvas = document.getElementById('proofsChart');
  const dataElement = document.getElementById('proofsChartData');

  if (!canvas || !dataElement) {
    // Chart elements not present on this page - they may be loaded via HTMX
    console.log('Chart elements not found on initial load, waiting for HTMX...');
    return;
  }

  // Initialize the chart from current DOM (for pages that have it pre-rendered)
  initializeChartFromDOM();
}

