document.addEventListener('DOMContentLoaded', function() {
    var statusSelect = document.getElementById('status');
    var dateField = document.getElementById('publish-date-field');
    if (statusSelect && dateField) {
        function toggleDateField() {
            if (statusSelect.value === 'scheduled') {
                dateField.style.display = 'block';
            } else if (statusSelect.value === 'published') {
                dateField.style.display = 'block';
            } else {
                dateField.style.display = 'none';
            }
        }
        statusSelect.addEventListener('change', toggleDateField);
        toggleDateField();
    }
});

function addTag(el) {
    var input = document.getElementById('tags');
    if (!input) return;
    var tagName = el.textContent.trim();
    var current = input.value.trim();
    if (current === '') {
        input.value = tagName;
    } else {
        var tags = current.split(',').map(function(t) { return t.trim(); }).filter(function(t) { return t !== ''; });
        if (tags.indexOf(tagName) === -1) {
            tags.push(tagName);
        }
        input.value = tags.join(', ');
    }
}
