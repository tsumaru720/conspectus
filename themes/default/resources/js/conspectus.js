// If you change this file, update the timestamp query string in html/_footer.html.

feather.replace()

function setUpMenuItems () {
  const localStorage = window.localStorage

  document.querySelectorAll('[data-save-state]').forEach((el) => {
    const key = `${el.id}_collapsed`
    const state = `${el.id}_state`

    $(el).on('hidden.bs.collapse', () => {
      //Hide
      localStorage.setItem(key, 'true')
      document.getElementById(state).innerHTML = '+'
    })
    $(el).on('shown.bs.collapse', () => {
      //Show
      localStorage.removeItem(key)
      document.getElementById(state).innerHTML = '-'
    })

    if (localStorage.getItem(key)) {
      //Hide
      document.getElementById(state).innerHTML = '+'
    } else {
      //Show
      $(el).collapse('show')
      document.getElementById(state).innerHTML = '-'
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
