// If you change this file, update the timestamp query string in html/_footer.html.

feather.replace()

function setUpMenuItems () {
  const localStorage = window.localStorage

  document.querySelectorAll('[data-save-state]').forEach((el) => {
    const key = `${el.id}_collapsed`
    $(el).on('hidden.bs.collapse', () => localStorage.setItem(key, 'true'))
    $(el).on('shown.bs.collapse', () => localStorage.removeItem(key))

    if (!localStorage.getItem(key)) {
      $(el).collapse('show')
    }

    // Increase transition duration for a nicer animation after initial page load.
    setTimeout(() => {
      el.style.transitionDuration = '0.35s'
    }, 10)
  })
}

function setUpSearch () {
  document.querySelectorAll('input[data-search-top]').forEach((searchBar) => {
    searchBar.addEventListener('input', () => {
      const query = searchBar.value.toLowerCase()
      const target = document.querySelector(searchBar.getAttribute('data-search-top'))
      target.querySelectorAll('[data-searchable-value]').forEach((el) => {
        const value = el.getAttribute('data-searchable-value').toLowerCase()
        if (value.includes(query)) {
          $(el).show()
        } else {
          $(el).hide()
        }
      })
    })
  })
}

document.addEventListener('DOMContentLoaded', () => {
  setUpMenuItems()
  setUpSearch()
})
