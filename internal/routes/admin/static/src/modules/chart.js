// Chart.js module for time-series visualization
import 'chartjs-adapter-date-fns';
import Chart from 'chart.js/auto'


// Color scheme matching the design system
const COLORS = {
  cyan: '#00d9b1',
  cyanLight: 'rgba(0, 217, 177, 0.1)',
  purple: '#8b5cf6',
  purpleLight: 'rgba(139, 92, 246, 0.1)',
  green: '#22c55e',
  greenLight: 'rgba(34, 197, 94, 0.15)',
  red: '#ef4444',
  redLight: 'rgba(239, 68, 68, 0.15)',
  gridColor: '#2d333b',
  textColor: '#8b939f',
  textPrimary: '#ffffff'
};

// Store chart instances
 const chartInstances = {
  proofs: null,
  blindSigs: null,
  ln: null
};

// Helper to read chart data from a canvas data attribute first, then fallback
// to a paired script tag (legacy). Returns { canvas, data }.
 function getChartContext({ canvasId, dataElementId }) {
  const canvas = document.getElementById(canvasId);
  if (!canvas) {
    return { canvas: null, data: null };
  }

  let data = null;

  const attrData = canvas.getAttribute('data-chart');
  if (attrData) {
    try {
      data = JSON.parse(attrData);
    } catch (error) {
      console.error(`Failed to parse data-chart for ${canvasId}:`, error);
    }
  }

  if (!data && dataElementId) {
    const dataElement = document.getElementById(dataElementId);
    if (dataElement) {
      try {
        data = JSON.parse(dataElement.textContent);
      } catch (error) {
        console.error(`Failed to parse legacy data element for ${canvasId}:`, error);
      }
    }
  }

  return { canvas, data };
}

/**
 * Create chart configuration
 * @param {Array} data - Array of {timestamp, totalAmount, count} objects
 * @param {string} countLabel - Label for the count axis (e.g., 'Proof Count' or 'Signature Count')
 */
 function createChartConfig(data, countLabel) {
  // Transform data for Chart.js
  const chartData = data.map(point => ({
    x: new Date(point.timestamp * 1000), // Convert Unix timestamp to Date
    amount: point.totalAmount,
    count: point.count
  }));

  return {
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
          label: countLabel,
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
                  label += context.parsed.y.toLocaleString();
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
            text: countLabel,
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
  };
}

/**
 * Initialize a chart
 * @param {HTMLCanvasElement} canvas - The canvas element to render the chart
 * @param {Array} data - Array of {timestamp, totalAmount, count} objects
 * @param {string} countLabel - Label for the count axis
 */
 function initChart(canvas, data, countLabel) {
  if (!canvas || !Array.isArray(data)) {
    console.warn('Chart initialization skipped: missing canvas or data');
    return null;
  }

  const config = createChartConfig(data, countLabel);
  const chart = new Chart(canvas, config);
  return chart;
}




/**
 * Initialize or reinitialize the blind sigs chart from the current DOM
 */
 function initializeBlindSigsChartFromDOM() {
  const { canvas, data } = getChartContext({
    canvasId: 'blindSigsChart',
    dataElementId: 'blindSigsChartData'
  });

  if (!canvas || !data) {
    return;
  }

  if (chartInstances.blindSigs) {
    chartInstances.blindSigs.destroy();
    chartInstances.blindSigs = null;
  }

  chartInstances.blindSigs = initChart(canvas, data, 'Signature Count');
  if (chartInstances.blindSigs) {
    console.log('Blind sigs chart initialized with', data.length, 'data points');
  }
}

/**
 * Create chart configuration for mint/melt (LN) chart
 * @param {Array} data - Array of {timestamp, mintAmount, meltAmount, mintCount, meltCount} objects
 */
function createLnChartConfig(data) {
  // Transform data for Chart.js
  const chartData = data.map(point => ({
    x: new Date(point.timestamp * 1000), // Convert Unix timestamp to Date
    mint: point.mintAmount,
    melt: point.meltAmount
  }));

  return {
    type: 'line',
    data: {
      datasets: [
        {
          label: 'Mint (Inflows)',
          data: chartData.map(d => ({ x: d.x, y: d.mint })),
          borderColor: COLORS.green,
          backgroundColor: COLORS.greenLight,
          borderWidth: 2,
          fill: true,
          tension: 0.3,
          pointRadius: 3,
          pointHoverRadius: 6,
          pointBackgroundColor: COLORS.green,
          pointBorderColor: COLORS.green,
          yAxisID: 'y'
        },
        {
          label: 'Melt (Outflows)',
          data: chartData.map(d => ({ x: d.x, y: -d.melt })), // Display melt as negative for visual comparison
          borderColor: COLORS.red,
          backgroundColor: COLORS.redLight,
          borderWidth: 2,
          fill: true,
          tension: 0.3,
          pointRadius: 3,
          pointHoverRadius: 6,
          pointBackgroundColor: COLORS.red,
          pointBorderColor: COLORS.red,
          yAxisID: 'y'
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
                // Show absolute value for melt (since we display as negative)
                const value = Math.abs(context.parsed.y);
                label += value.toLocaleString() + ' sats';
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
            text: 'Sats',
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
            callback: function(value) {
              return value.toLocaleString();
            }
          }
        }
      }
    }
  };
}

/**
 * Initialize or reinitialize the LN (mint/melt) chart from the current DOM
 */
 function initializeLnChartFromDOM() {
  const { canvas, data } = getChartContext({
    canvasId: 'lnChart',
    dataElementId: 'lnChartData'
  });

  if (!canvas || !data) {
    return;
  }

  if (chartInstances.ln) {
    chartInstances.ln.destroy();
    chartInstances.ln = null;
  }

  const config = createLnChartConfig(data);
  chartInstances.ln = new Chart(canvas, config);
}

/**
 * Initialize or reinitialize the proofs chart from the current DOM
 */
 function initializeProofsChartFromDOM() {
  const { canvas, data } = getChartContext({
    canvasId: 'proofsChart',
    dataElementId: 'proofsChartData'
  });

  if (!canvas || !data) {
    return;
  }

  if (chartInstances.proofs) {
    chartInstances.proofs.destroy();
    chartInstances.proofs = null;
  }

  chartInstances.proofs = initChart(canvas, data, 'Proof Count');
  if (chartInstances.proofs) {
  }
}

/**
 * Set up HTMX event listener to reinitialize charts after content swap
 */
function setupHtmxListener() {
  // Listen for HTMX afterSwap event
  document.body.addEventListener('htmx:afterSwap', (event) => {
    const targetId = event.detail.target?.id;
    
    // Proofs chart updates
    if (targetId === 'chart-wrapper' || 
        targetId === 'proofs-chart-placeholder' ||
        targetId === 'proofs-chart-card') {
      setTimeout(initializeProofsChartFromDOM, 50);
    }
    
    // Blind sigs chart updates
    if (targetId === 'blindsigs-chart-wrapper' || 
        targetId === 'blindsigs-chart-placeholder' ||
        targetId === 'blindsigs-chart-card') {
      setTimeout(initializeBlindSigsChartFromDOM, 50);
    }
    
    // LN chart updates
    if (targetId === 'ln-chart-wrapper' || 
        targetId === 'ln-chart-placeholder' ||
        targetId === 'ln-chart-card') {
      setTimeout(initializeLnChartFromDOM, 50);
    }
  });

  // // Also listen for htmx:afterSettle for initial HTMX loads
  document.body.addEventListener('htmx:afterSettle', (event) => {
    const targetId = event.detail.target?.id;
    
    if (targetId === 'proofs-chart-placeholder') {
      setTimeout(initializeProofsChartFromDOM, 50);
    }
    
    if (targetId === 'blindsigs-chart-placeholder') {
      setTimeout(initializeBlindSigsChartFromDOM, 50);
    }
    
    if (targetId === 'ln-chart-placeholder') {
      setTimeout(initializeLnChartFromDOM, 50);
    }
  });
}


setupHtmxListener();