// Time controls module for log filtering
/**
 * Initialize time-based controls for log filtering
 */
export function initTimeControls() {
  // check for click on button for age of logs

  /**
   * @type NodeListOf<HTMLButtonElement>
   * */
  const buttons = document.querySelectorAll(".time-button");

  document.querySelector(".time-select")?.addEventListener("click", (evt) => {
    // turn all time buttons off by removing class
    if (buttons) {
      for (let i = 0; i < buttons.length; i++) {
        const element = buttons[i];

        element.classList.remove("selected");
      }
    }

    evt.target?.classList.add("selected");
    window.htmx.trigger(".summary-table", "reload", { time: evt.target?.value });
    window.htmx.trigger(".log-table", "reload", { time: evt.target?.value });
    window.htmx.trigger(".mint-melt-table ", "reload", {
      time: evt.target?.value,
    });
  });
}
