// If you change this file, update the timestamp query string in html/_footer.html.

feather.replace()

function setUpMenuItems () {

  document.querySelectorAll('[data-save-state]').forEach((el) => {
    const key = `${el.id}_collapsed`
    const stateEl = document.getElementById(`${el.id}_state`)

    $(el).on('hide.bs.collapse', () => {
      //Hide
      localStorage.setItem(key, 'true')
      stateEl.innerHTML = '+'
    })
    $(el).on('show.bs.collapse', () => {
      //Show
      localStorage.removeItem(key)
      stateEl.innerHTML = '-'
    })

    if (localStorage.getItem(key)) {
      //Hide
      stateEl.innerHTML = '+'
    } else {
      //Show
      $(el).collapse('show')
      stateEl.innerHTML = '-'
    }

    // Increase transition duration for a nicer animation after initial page load.
    setTimeout(() => {
      el.style.transitionDuration = '0.35s'
    }, 10)
  })
}

function setUpSearch () {
  var searchBar = document.getElementById('mainSearch')
  var regexBtn = document.getElementById('regexBtn')

  searchBar.addEventListener('input', searchHandler)
  regexBtn.addEventListener('click', regexToggle)

  if (localStorage.getItem(searchBar.id)) {
    searchBar.value = localStorage.getItem(searchBar.id)
    searchBar.dispatchEvent(new Event('input'))
  }

  if (localStorage.getItem(regexBtn.id)) {
    regexBtn.classList.add("active");
    searchBar.placeholder = "Regex Search..."
  }

}

function regexToggle(e) {
  var regexBtn = e.srcElement
  var searchBar = document.getElementById('mainSearch')

  if (localStorage.getItem(regexBtn.id)) {
    regexBtn.classList.remove("active");
    localStorage.removeItem(regexBtn.id)
    searchBar.placeholder = "Search..."
  } else {
    regexBtn.classList.add("active");
    localStorage.setItem(regexBtn.id, 'active')
    searchBar.placeholder = "Regex Search..."
  }

  searchBar.focus()
  searchBar.dispatchEvent(new Event('input'))
}

function searchHandler(e) {
  var searchBar = e.srcElement
  var regexBtn = document.getElementById('regexBtn')
  var query = searchBar.value.toLowerCase()

  if (query != '') {
    localStorage.setItem(searchBar.id, query)
  } else {
    localStorage.removeItem(searchBar.id)
  }
  var target = document.querySelector(searchBar.getAttribute('data-search-top'))
  target.querySelectorAll('[data-searchable-value]').forEach((el) => {
    const value = el.getAttribute('data-searchable-value').toLowerCase()

    if (localStorage.getItem(regexBtn.id)) {
      try {
        var regex = new RegExp(query);
      } catch(e) {
        var regex = null;
      }

      if (value.match(regex)) {
        $(el).show()
      } else {
        $(el).hide()
      }
    } else {
      if (value.includes(query)) {
        $(el).show()
      } else {
        $(el).hide()
      }
    }




  })
}

const localStorage = window.localStorage

document.addEventListener('DOMContentLoaded', () => {
  setUpSearch()
  setUpMenuItems()
})