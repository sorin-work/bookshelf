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

// Slips print themselves when opened.
document.addEventListener("DOMContentLoaded", () => {
  if (document.body.hasAttribute("data-print")) {
    window.print();
  }
});

// Forms marked data-confirm ask before submitting. htmx has hx-confirm for
// its own requests; this covers plain forms.
document.addEventListener("submit", (event) => {
  const form = event.target;
  const question = form.getAttribute && form.getAttribute("data-confirm");
  if (question && !window.confirm(question)) {
    event.preventDefault();
  }
});

// Press / anywhere (outside a text field) to jump to the search box.
document.addEventListener("keydown", (event) => {
  if (event.key !== "/" || event.ctrlKey || event.metaKey || event.altKey) {
    return;
  }
  const tag = document.activeElement && document.activeElement.tagName;
  if (tag === "INPUT" || tag === "TEXTAREA" || tag === "SELECT") {
    return;
  }
  const search = document.querySelector("input[type=search]");
  if (search) {
    event.preventDefault();
    search.focus();
    search.select();
  }
});

// Covers live on other sites and sometimes disappear. Hide a broken image
// instead of showing the browser's broken image icon; the placeholder
// behind it shows through.
document.addEventListener("error", (event) => {
  const img = event.target;
  if (img instanceof HTMLImageElement) {
    img.hidden = true;
  }
}, true);
