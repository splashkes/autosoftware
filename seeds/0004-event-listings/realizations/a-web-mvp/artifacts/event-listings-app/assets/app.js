document.addEventListener("click", async (event) => {
  const button = event.target.closest("[data-copy]");
  if (!button) {
    return;
  }

  const rawValue = button.getAttribute("data-copy");
  if (!rawValue) {
    return;
  }
  const value = new URL(rawValue, window.location.href).toString();

  try {
    await navigator.clipboard.writeText(value);
    const original = button.textContent;
    button.textContent = "Copied";
    window.setTimeout(() => {
      button.textContent = original;
    }, 1400);
  } catch (error) {
    window.prompt("Copy this link", value);
  }
});
