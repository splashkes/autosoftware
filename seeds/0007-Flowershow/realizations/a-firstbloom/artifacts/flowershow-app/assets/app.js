// Flowershow — minimal JS (HTMX handles most interactivity)

function showTab(name) {
  document.querySelectorAll('.tab-content').forEach(el => el.classList.remove('active'));
  document.querySelectorAll('.tab').forEach(el => el.classList.remove('active'));
  const tab = document.getElementById('tab-' + name);
  if (tab) tab.classList.add('active');
  // Find the clicked button
  document.querySelectorAll('.tab').forEach(el => {
    if (el.textContent.trim().toLowerCase() === name.toLowerCase() ||
        el.getAttribute('onclick')?.includes(name)) {
      el.classList.add('active');
    }
  });
}

// Auto-remove toasts after 4 seconds
document.addEventListener('htmx:afterSettle', function(evt) {
  const toasts = document.querySelectorAll('.toast');
  toasts.forEach(t => {
    setTimeout(() => t.remove(), 4000);
  });
});
