// Small behaviours that would otherwise need inline scripts, which the
// content security policy doesn't allow.

// Desk forms: once htmx has sent a scan, clear the barcode and put the
// cursor back so the next item can be scanned straight away.
document.addEventListener("htmx:afterRequest", (event) => {
  const form = event.target.closest("form[data-scan]");
  if (!form) {
    return;
  }
  const field = form.querySelector("input[name=barcode]");
  if (field) {
    field.value = "";
    field.focus();
  }
});
