// Called from an inline onclick in views/listing.html.tmpl, not from this file.
// eslint-disable-next-line no-unused-vars
function deleteFile (name, confirmMessage) {
  if (!confirm(confirmMessage)) return
  fetch('/download/' + encodeURIComponent(name), { method: 'DELETE' })
    .then(function (res) {
      if (res.ok) {
        location.reload()
      } else {
        alert('Delete failed.')
      }
    })
    .catch(function () {
      alert('Delete failed.')
    })
}
