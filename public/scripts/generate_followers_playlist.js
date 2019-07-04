(function() {
    generatePlaylistSpinner   = $('#generate-playlist-spinner');
    generatePlaylistForm      = $('#generate-playlist-form');
    generatePlaylistButton      = $('#generate-playlist-button');

    document.getElementById('generate-playlist-button').addEventListener('click', function() {
        generatePlaylistForm.validate({
            rules: {
                playlist_name: { required: true }
            }
        });
        console.log(generatePlaylistForm.valid());
        if (generatePlaylistForm.valid()) {
            $.ajax({
                url: '/tool/follow-playlist/generate',
                type: 'POST',
                dataType: 'json',
                contentType: 'application/json;charset=UTF-8',
                data: JSON.stringify(generatePlaylistForm.serializeArray()),
                beforeSend: function() {
                    generatePlaylistSpinner.prop('hidden', false);
                    generatePlaylistButton.prop('disabled', true);

                }
            }).done(function(data) {
                document.getElementById('generate-playlist-status').textContent = data.message;
                generatePlaylistSpinner.prop('hidden', true);
                generatePlaylistButton.prop('disabled', false);
            });
        }
    }, false);
})();
