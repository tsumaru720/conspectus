feather.replace()

$('[data-save-state]').each(function() {
  var element = $(this);

  element.on('shown.bs.collapse', function () {
    window.localStorage.setItem(element.attr('id') + "_collapsed", "false");
  })
  element.on('hidden.bs.collapse', function () {
    window.localStorage.setItem(element.attr('id') + "_collapsed", "true");
  })
});

$(function() {
  $('[data-save-state]').each(function() {
    var element = $(this);
    var descr = element.attr('id');
    var state = window.localStorage.getItem(descr + "_collapsed") || "false";

    if (state == "true") {
      //Hidden
      element.collapse('hide');
    } else {
      //Visible
      element.collapse('show');
    }

    // increase transition duration for a nicer animation after initial page load.
    setTimeout(function() {
      element.css('transition-duration','0.35s')
    },10);

  });

  $("input[data-search-top]").on('input', function() {
      var value = $(this).val();
      var searchTop = $(this).data('search-top');

      if (value == "") {
        $(searchTop).find("[data-searchable-value]").show();
      } else {
        var match = new RegExp('^.*' + escapeRegExp(value) + '.*$', 'i');

        $(searchTop).find("[data-searchable-value]").each(function() {
          var show = false;

          for (val in $(this).data('searchable-value').split(" ")) {
            if ($(this).data('searchable-value').match(match)) {
              show = true;
              break;
            }
          }

          if (show) {
            $(this).show();
          } else {
            $(this).hide();
          }
        });
      }
  });

});

function escapeRegExp(str) {
  return str.replace(/[\-\[\]\/\{\}\(\)\*\+\?\.\\\^\$\|]/g, "\\$&");
}