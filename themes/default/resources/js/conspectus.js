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

    })
});